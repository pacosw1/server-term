package widget

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

// cipActionBodyLimit caps the reply this widget reads. The daemon answers an
// action with one short line, so a larger body is a fault, not an answer.
const cipActionBodyLimit = 8 << 10

// CIPRerunResult is the daemon's answer to a re-run request. Error is set
// when the daemon refused, and it holds the daemon's own words.
type CIPRerunResult struct {
	// Rerunning names the jobs that go back to pending.
	Rerunning []string `json:"rerunning"`
	Run       int      `json:"run"`
	Promotion int      `json:"promotion"`
	Stage     string   `json:"stage"`
	Error     string   `json:"-"`
}

// Summary says what the daemon did, for the reader.
func (r CIPRerunResult) Summary() string {
	if len(r.Rerunning) == 0 {
		return "no job runs again"
	}
	return fmt.Sprintf("run #%d runs %s again", r.Run, strings.Join(r.Rerunning, ", "))
}

// CIPApproveResult is the daemon's answer to an approval.
type CIPApproveResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"-"`
}

// RerunCIPRun asks the daemon to run the failed jobs of one run again. This
// is a WRITE: it changes a real pipeline. The caller must confirm with the
// reader first.
//
// An empty job re-runs every failed job of the run. A named job re-runs only
// that one. Jobs that passed keep their results either way.
//
// It fails closed and never invents an answer: a refusal carries the
// daemon's own words, so the reader learns what to do next.
func RerunCIPRun(ctx context.Context, provider config.Widget, token string, id int, job string) CIPRerunResult {
	var result CIPRerunResult
	if strings.TrimSpace(token) == "" {
		result.Error = "no token configured: set token_env or token_file"
		return result
	}
	// Send no job field at all when none is named. An empty field would ask
	// the daemon to re-run a job with no name.
	body := map[string]string{}
	if strings.TrimSpace(job) != "" {
		body["job"] = job
	}
	base := strings.TrimRight(provider.Endpoint, "/")
	if err := cipPost(ctx, fmt.Sprintf("%s/runs/%d/rerun", base, id), token, body, &result); err != nil {
		// Keep no partial answer next to an error: a half-filled result
		// would read as work that started.
		return CIPRerunResult{Error: err.Error()}
	}
	return result
}

// ApproveCIPStage approves one gated stage of one promotion. This is a
// WRITE, and it lets a release go ahead. The caller must confirm with the
// reader first.
//
// The daemon records who approved and why, so both are worth sending. A
// reason is optional for the daemon, but an audit trail without one is much
// less useful.
func ApproveCIPStage(ctx context.Context, provider config.Widget, token string, promotionID int, stage, by, reason string) CIPApproveResult {
	if strings.TrimSpace(token) == "" {
		return CIPApproveResult{Error: "no token configured: set token_env or token_file"}
	}
	body := map[string]string{"by": by, "reason": reason}
	base := strings.TrimRight(provider.Endpoint, "/")
	// Escape the stage name: it comes from the spec, so it can hold a
	// character that would otherwise change the path.
	target := fmt.Sprintf("%s/promotions/%d/stages/%s/approve", base, promotionID, url.PathEscape(stage))
	if err := cipPost(ctx, target, token, body, nil); err != nil {
		return CIPApproveResult{Error: err.Error()}
	}
	return CIPApproveResult{OK: true}
}

// cipPost sends one authenticated write and reads the daemon's answer. It
// decodes a JSON reply into out when out is not nil.
//
// The daemon refuses an action with a status and one line of plain text.
// That text is the whole value of the answer, so it is passed through word
// for word. The error never holds the token.
func cipPost(ctx context.Context, target, token string, body any, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	reply, err := io.ReadAll(io.LimitReader(resp.Body, cipActionBodyLimit))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("cip %s: %s — check the widget token", cipPath(target), resp.Status)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if text := strings.TrimSpace(string(reply)); text != "" {
			return fmt.Errorf("cip refused: %s", text)
		}
		return fmt.Errorf("cip %s: %s", cipPath(target), resp.Status)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(reply, out); err != nil {
		return fmt.Errorf("cip %s: %w", cipPath(target), err)
	}
	return nil
}

// cipActionTimeout is how long a write may take before it is given up. A
// re-run only queues work, so it answers quickly.
const cipActionTimeout = 10 * time.Second
