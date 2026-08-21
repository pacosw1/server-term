package widget

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

// OrchestratorDaemon is the resource usage of the orchestrator process itself.
type OrchestratorDaemon struct {
	PID           int     `json:"pid"`
	CPUPercent    float64 `json:"cpu_percent"`
	RSSBytes      int64   `json:"rss_bytes"`
	UptimeSeconds int64   `json:"uptime_seconds"`
}

// OrchestratorBudget is the spend guardrail the orchestrator enforces.
type OrchestratorBudget struct {
	HourUSD         float64 `json:"hour_usd"`
	DayUSD          float64 `json:"day_usd"`
	WeekUSD         float64 `json:"week_usd"`
	HourLimitUSD    float64 `json:"hour_limit_usd"`
	DayLimitUSD     float64 `json:"day_limit_usd"`
	WeekLimitUSD    float64 `json:"week_limit_usd"`
	DayRemainingUSD float64 `json:"day_remaining_usd"`
	PaceNote        string  `json:"pace_note"`
}

// OrchestratorTotals summarizes all agents the orchestrator is tracking.
type OrchestratorTotals struct {
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	Live         int     `json:"live"`
	Done         int     `json:"done"`
	Blocked      int     `json:"blocked"`
}

// OrchestratorAgent is one running or queued agent task.
type OrchestratorAgent struct {
	Issue              int      `json:"issue"`
	Title              string   `json:"title"`
	State              string   `json:"state"`
	Cycle              int      `json:"cycle"`
	PRNumber           *int     `json:"pr_number"`
	Branch             string   `json:"branch"`
	ElapsedSeconds     int64    `json:"elapsed_seconds"`
	InputTokens        int64    `json:"input_tokens"`
	OutputTokens       int64    `json:"output_tokens"`
	CostUSD            float64  `json:"cost_usd"`
	PID                int      `json:"pid"`
	CPUPercent         float64  `json:"cpu_percent"`
	RSSBytes           int64    `json:"rss_bytes"`
	LastError          string   `json:"last_error"`
	WeeklyPercentUsed  *float64 `json:"weekly_percent_used"`
	LastActivity       *string  `json:"last_activity"`
	ActivityAgeSeconds *int64   `json:"activity_age_seconds"`
	Turns              int      `json:"turns"`
	// Children is nil when the daemon does not report spark subagents yet,
	// and non-nil (possibly empty) once it does. Callers must tell the two
	// apart: nil means "not reported", not "none launched".
	Children        []OrchestratorChild `json:"children"`
	ChildrenRunning int                 `json:"children_running"`
	ChildrenDone    int                 `json:"children_done"`
	ChildrenFailed  int                 `json:"children_failed"`
	// Worktree is the checkout path this agent is working in. It is "" when
	// the daemon does not report it.
	Worktree string `json:"worktree"`
	// WorktreeDiskBytes is the disk this agent's checkout occupies. It is
	// nil when no reading is available, so callers do not show a fake 0.
	WorktreeDiskBytes *int64 `json:"worktree_disk_bytes"`
	// Tasks is nil when the daemon does not report a task checklist for
	// this agent, and non-nil (possibly empty) once it does. Callers must
	// tell the two apart the same way as Children: nil is "not reported",
	// not "no tasks".
	Tasks []OrchestratorTask `json:"tasks"`
}

// OrchestratorTask is one checklist item an agent tracks for itself while it
// works, for example a step a hook reminded it to record.
type OrchestratorTask struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// OrchestratorChild is one spark subagent a task agent launched.
type OrchestratorChild struct {
	ID             string `json:"id"`
	Model          string `json:"model"`
	State          string `json:"state"`
	Task           string `json:"task"`
	ElapsedSeconds int64  `json:"elapsed_seconds"`
	InputTokens    int64  `json:"input_tokens"`
	OutputTokens   int64  `json:"output_tokens"`
	PID            *int   `json:"pid"`
	ExitCode       *int   `json:"exit_code"`
}

// OrchestratorUsageWindow is the share of one subscription plan window
// (for example the weekly or five-hour window) that is already used.
type OrchestratorUsageWindow struct {
	UsedPercent float64 `json:"used_percent"`
	ResetsAt    int64   `json:"resets_at"`
}

// OrchestratorLimits is the subscription plan usage. Either window is nil
// when no reading is available for it; callers must not draw a bar for a
// nil window, since an empty bar reads as "plenty left" instead of unknown.
type OrchestratorLimits struct {
	Weekly   *OrchestratorUsageWindow `json:"weekly"`
	FiveHour *OrchestratorUsageWindow `json:"five_hour"`
	PlanType string                   `json:"plan_type"`
}

// OrchestratorRecent is one agent task that already left the live list.
type OrchestratorRecent struct {
	Issue    int     `json:"issue"`
	State    string  `json:"state"`
	PRNumber *int    `json:"pr_number"`
	CostUSD  float64 `json:"cost_usd"`
	Title    string  `json:"title"`
	// LastError explains why a blocked task stopped. It is the reason a
	// reader opens the history at all, so it must survive decoding.
	LastError string `json:"last_error"`
}

// OrchestratorDisk is the daemon host's overall disk usage. It is nil on
// the snapshot when no reading is available; callers must not draw a bar
// for a nil disk, for the same reason as a nil usage window.
type OrchestratorDisk struct {
	TotalBytes int64 `json:"total_bytes"`
	FreeBytes  int64 `json:"free_bytes"`
	UsedBytes  int64 `json:"used_bytes"`
}

// OrchestratorSnapshot is the stable, provider-neutral subset exposed to
// callers. Unknown fields from the orchestrator daemon are deliberately
// ignored so its status API can evolve independently.
type OrchestratorSnapshot struct {
	SchemaVersion int                  `json:"schema_version"`
	Name          string               `json:"name"`
	At            time.Time            `json:"at"`
	Healthy       bool                 `json:"healthy"`
	Mode          string               `json:"mode"`
	Repo          string               `json:"repo"`
	Daemon        OrchestratorDaemon   `json:"daemon"`
	Budget        OrchestratorBudget   `json:"budget"`
	Totals        OrchestratorTotals   `json:"totals"`
	Agents        []OrchestratorAgent  `json:"agents"`
	Recent        []OrchestratorRecent `json:"recent"`
	Limits        *OrchestratorLimits  `json:"limits"`
	Disk          *OrchestratorDisk    `json:"disk"`
	// Auth says which account pays for the tokens. CostIsEstimate says
	// whether Budget/Totals are a computed estimate (a subscription plan has
	// no per-call price) or real billed spend. A caller must never show a
	// dollar figure as real money when CostIsEstimate is true.
	Auth           OrchestratorAuth `json:"auth"`
	CostIsEstimate bool             `json:"cost_is_estimate"`
	Error          string           `json:"error,omitempty"`
}

// OrchestratorAuth is the account the daemon is spending against. Mode is
// "subscription", "api_key", or "unknown".
type OrchestratorAuth struct {
	Mode     string `json:"mode"`
	PlanType string `json:"plan_type"`
	Billed   bool   `json:"billed"`
}

// FetchOrchestrator reads only the authenticated GET /api/status endpoint. It
// never starts, stops, or steers an agent, and never accepts arbitrary code.
// AccountLabel names the account paying for the tokens, for the CLI summary
// and the TUI header. A subscription shows its plan (or "codex" alone when
// the plan is not reported); an API key account shows "api key"; an unknown
// account still gets an honest label rather than being left blank.
func (s OrchestratorSnapshot) AccountLabel() string {
	switch s.Auth.Mode {
	case "subscription":
		if s.Auth.PlanType != "" {
			return "codex " + s.Auth.PlanType
		}
		return "codex"
	case "api_key":
		return "api key"
	default:
		return "unknown account"
	}
}

// CostText formats the current day's spend against its limit. A dollar
// figure computed from a subscription's token usage is an estimate, not a
// charge, so it is marked with "~" and "est". Real billed spend (an API
// key, or an unknown account the daemon already treats as billed) is shown
// as a plain figure, because a "~" there would hide real money as a guess.
func (s OrchestratorSnapshot) CostText() string {
	if s.CostIsEstimate {
		return fmt.Sprintf("est ~$%.2f/$%.2f day", s.Budget.DayUSD, s.Budget.DayLimitUSD)
	}
	if s.Auth.Mode == "unknown" {
		return fmt.Sprintf("$%.2f/$%.2f day billed", s.Budget.DayUSD, s.Budget.DayLimitUSD)
	}
	return fmt.Sprintf("$%.2f/$%.2f day", s.Budget.DayUSD, s.Budget.DayLimitUSD)
}
func FetchOrchestrator(ctx context.Context, provider config.Widget, token string) OrchestratorSnapshot {
	result := OrchestratorSnapshot{SchemaVersion: 1, Name: provider.Name, At: time.Now()}
	base := strings.TrimRight(provider.Endpoint, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/status", nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Sprintf("orchestrator status: %s", resp.Status)
		return result
	}
	// Decode into a separate value so a decode error partway through the
	// body (for example inside the agents array) cannot leave result
	// holding a half-filled snapshot that reads as healthy.
	var parsed OrchestratorSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		result.Error = err.Error()
		return result
	}
	parsed.Name = provider.Name
	if parsed.At.IsZero() {
		parsed.At = time.Now()
	}
	return parsed
}

// OrchestratorModeResult is the daemon's own answer to a mode change
// request. OK is false on any failure; callers must show Error, not invent
// their own message, and must never assume OK when the daemon did not say
// so.
type OrchestratorModeResult struct {
	OK    bool   `json:"ok"`
	Mode  string `json:"mode"`
	Error string `json:"error"`
}

// SetOrchestratorMode sends the ONLY write this widget performs: a request
// to switch the daemon's run mode to "fast", "economy", or "paused". Every
// mode only reduces work — none can raise fanout, spend, enable autoMerge,
// or change the repository — so a mistaken or hostile call can do no worse
// than quiet the daemon down.
func SetOrchestratorMode(ctx context.Context, provider config.Widget, token, mode string) OrchestratorModeResult {
	base := strings.TrimRight(provider.Endpoint, "/")
	body, err := json.Marshal(struct {
		Mode string `json:"mode"`
	}{Mode: mode})
	if err != nil {
		return OrchestratorModeResult{Error: err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/mode", bytes.NewReader(body))
	if err != nil {
		return OrchestratorModeResult{Error: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return OrchestratorModeResult{Error: err.Error()}
	}
	defer resp.Body.Close()
	// A wrong or missing token gets a 401 with no body; decoding that would
	// only produce a confusing JSON error, so report the real reason.
	if resp.StatusCode == http.StatusUnauthorized {
		return OrchestratorModeResult{Error: "unauthorized: check the widget token"}
	}
	var result OrchestratorModeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return OrchestratorModeResult{Error: fmt.Sprintf("orchestrator mode: %s", resp.Status)}
	}
	// The daemon's own ok/error fields are the source of truth, even on a
	// 200; never override them with an invented message.
	return result
}
