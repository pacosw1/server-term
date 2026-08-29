package widget

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

// rerunServer records what the widget sent, and answers like the daemon.
type rerunServer struct {
	*httptest.Server
	path   string
	rawURI string
	method string
	body   string
	auth   string
}

func newRerunServer(t *testing.T, status int, reply string) *rerunServer {
	t.Helper()
	rec := &rerunServer{}
	rec.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.path, rec.method, rec.body = r.URL.Path, r.Method, string(body)
		rec.rawURI = r.RequestURI
		rec.auth = r.Header.Get("Authorization")
		if status != http.StatusOK {
			// The daemon answers a refused action with plain text.
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(reply))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reply))
	}))
	return rec
}

const rerunOK = `{"rerunning":["build","checks"],"run":92,"promotion":29,"stage":"ci"}`

func TestRerunCIPRunReRunsEveryFailedJob(t *testing.T) {
	srv := newRerunServer(t, http.StatusOK, rerunOK)
	defer srv.Close()
	got := RerunCIPRun(context.Background(), config.Widget{Endpoint: srv.URL}, "secret", 92, "")
	if got.Error != "" {
		t.Fatalf("Error = %q, want none", got.Error)
	}
	if srv.method != http.MethodPost || srv.path != "/runs/92/rerun" {
		t.Errorf("sent %s %s, want POST /runs/92/rerun", srv.method, srv.path)
	}
	if srv.auth != "Bearer secret" {
		t.Errorf("Authorization = %q, want the bearer token", srv.auth)
	}
	// With no job named, the daemon must not receive a job field, or it
	// would re-run one job instead of every failed job.
	if strings.Contains(srv.body, "job") {
		t.Errorf("body = %q, want no job field when every failed job re-runs", srv.body)
	}
	if len(got.Rerunning) != 2 || got.Rerunning[0] != "build" {
		t.Errorf("Rerunning = %v, want [build checks]", got.Rerunning)
	}
	if got.Run != 92 || got.Promotion != 29 || got.Stage != "ci" {
		t.Errorf("result = %+v, want run 92, promotion 29, stage ci", got)
	}
}

func TestRerunCIPRunReRunsOneNamedJob(t *testing.T) {
	srv := newRerunServer(t, http.StatusOK, `{"rerunning":["e2e-shard1"],"run":92}`)
	defer srv.Close()
	got := RerunCIPRun(context.Background(), config.Widget{Endpoint: srv.URL}, "secret", 92, "e2e-shard1")
	if got.Error != "" {
		t.Fatalf("Error = %q, want none", got.Error)
	}
	var sent struct {
		Job string `json:"job"`
	}
	if err := json.Unmarshal([]byte(srv.body), &sent); err != nil {
		t.Fatalf("body %q is not JSON: %v", srv.body, err)
	}
	if sent.Job != "e2e-shard1" {
		t.Errorf("job = %q, want e2e-shard1", sent.Job)
	}
	if len(got.Rerunning) != 1 || got.Rerunning[0] != "e2e-shard1" {
		t.Errorf("Rerunning = %v, want [e2e-shard1]", got.Rerunning)
	}
}

// The daemon refuses an action with plain text and a status. The reader
// must see that text, because it says what to do next.
func TestRerunCIPRunShowsTheDaemonsRefusalVerbatim(t *testing.T) {
	for _, test := range []struct {
		status int
		reply  string
	}{
		{http.StatusConflict, "run is still going"},
		{http.StatusNotFound, "run not found"},
		{http.StatusNotFound, "no such job in this run"},
		{http.StatusBadRequest, "the run has no failed job"},
		{http.StatusBadRequest, "the run does not belong to a promotion stage"},
	} {
		srv := newRerunServer(t, test.status, test.reply)
		got := RerunCIPRun(context.Background(), config.Widget{Endpoint: srv.URL}, "secret", 92, "")
		srv.Close()
		if !strings.Contains(got.Error, test.reply) {
			t.Errorf("status %d gave Error = %q, want it to contain %q", test.status, got.Error, test.reply)
		}
		if len(got.Rerunning) != 0 {
			t.Errorf("status %d reported %v as re-running, want none", test.status, got.Rerunning)
		}
	}
}

// A refusal with no body must still say something.
func TestRerunCIPRunHandlesAnEmptyRefusalBody(t *testing.T) {
	srv := newRerunServer(t, http.StatusConflict, "")
	defer srv.Close()
	got := RerunCIPRun(context.Background(), config.Widget{Endpoint: srv.URL}, "secret", 92, "")
	if got.Error == "" {
		t.Error("an empty refusal body produced no error")
	}
}

func TestRerunCIPRunNeverActsWithoutAToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("a re-run was sent without a token: %s", r.URL.Path)
	}))
	defer srv.Close()
	got := RerunCIPRun(context.Background(), config.Widget{Endpoint: srv.URL}, "", 92, "")
	if got.Error == "" {
		t.Error("a missing token produced no error")
	}
}

func TestRerunCIPRunNeverPutsTheTokenInTheError(t *testing.T) {
	srv := newRerunServer(t, http.StatusUnauthorized, "")
	defer srv.Close()
	got := RerunCIPRun(context.Background(), config.Widget{Endpoint: srv.URL}, "super-secret", 92, "")
	if !strings.Contains(got.Error, "token") {
		t.Errorf("Error = %q, want it to name the token", got.Error)
	}
	if strings.Contains(got.Error, "super-secret") {
		t.Errorf("Error = %q, must not contain the token", got.Error)
	}
}

func TestApproveCIPStageSendsTheApprovalAndTheReason(t *testing.T) {
	srv := newRerunServer(t, http.StatusOK, `"approved"`)
	defer srv.Close()
	got := ApproveCIPStage(context.Background(), config.Widget{Endpoint: srv.URL}, "secret",
		12, "release", "paco", "ship it")
	if got.Error != "" || !got.OK {
		t.Fatalf("result = %+v, want OK", got)
	}
	if srv.method != http.MethodPost || srv.path != "/promotions/12/stages/release/approve" {
		t.Errorf("sent %s %s, want POST /promotions/12/stages/release/approve", srv.method, srv.path)
	}
	var sent struct{ By, Reason string }
	if err := json.Unmarshal([]byte(srv.body), &sent); err != nil {
		t.Fatalf("body %q is not JSON: %v", srv.body, err)
	}
	if sent.By != "paco" || sent.Reason != "ship it" {
		t.Errorf("body = %+v, want by paco and the reason", sent)
	}
}

// A stage name with a space or a slash must not break the path.
func TestApproveCIPStageEscapesTheStageName(t *testing.T) {
	srv := newRerunServer(t, http.StatusOK, `"approved"`)
	defer srv.Close()
	ApproveCIPStage(context.Background(), config.Widget{Endpoint: srv.URL}, "secret",
		12, "deploy to prod", "paco", "")
	// The wire must carry an escaped path. Go decodes r.URL.Path again on
	// the server, so the raw request line is what proves it.
	if strings.Contains(srv.rawURI, " ") {
		t.Errorf("raw path = %q, want the stage name escaped", srv.rawURI)
	}
	if !strings.Contains(srv.rawURI, "deploy%20to%20prod") {
		t.Errorf("raw path = %q, want the escaped stage name", srv.rawURI)
	}
	if srv.path != "/promotions/12/stages/deploy to prod/approve" {
		t.Errorf("decoded path = %q, want the daemon to read the real stage name", srv.path)
	}
}

func TestApproveCIPStageShowsTheDaemonsRefusalVerbatim(t *testing.T) {
	srv := newRerunServer(t, http.StatusConflict, "stage is not gated")
	defer srv.Close()
	got := ApproveCIPStage(context.Background(), config.Widget{Endpoint: srv.URL}, "secret",
		12, "release", "paco", "")
	if got.OK {
		t.Error("a refused approval reported OK")
	}
	if !strings.Contains(got.Error, "stage is not gated") {
		t.Errorf("Error = %q, want the daemon's words", got.Error)
	}
}

func TestApproveCIPStageNeverActsWithoutAToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("an approval was sent without a token: %s", r.URL.Path)
	}))
	defer srv.Close()
	got := ApproveCIPStage(context.Background(), config.Widget{Endpoint: srv.URL}, "", 12, "release", "paco", "")
	if got.OK || got.Error == "" {
		t.Error("a missing token approved the stage or produced no error")
	}
}
