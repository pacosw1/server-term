package widget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

func TestFetchOrchestratorParsesAGoodResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected request: %s %s", r.URL, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schema_version":1,"at":"2026-08-21T02:52:36.765Z","healthy":true,"mode":"fast","repo":"pacosw1/pitsa-vps","daemon":{"pid":1234,"cpu_percent":0.4,"rss_bytes":21663744,"uptime_seconds":312},"budget":{"hour_usd":0,"day_usd":0,"week_usd":0,"hour_limit_usd":5,"day_limit_usd":7.5,"week_limit_usd":50,"day_remaining_usd":7.5,"pace_note":"spending is within the pace"},"totals":{"input_tokens":24729,"output_tokens":812,"cost_usd":0.039,"live":1,"done":0,"blocked":0},"limits":{"weekly":{"used_percent":83.0,"resets_at":1787331928},"five_hour":{"used_percent":12.0,"resets_at":1787300000},"plan_type":"pro"},"agents":[{"issue":82,"title":"Derive the deploy staging lists from package.json","state":"implementing","cycle":0,"pr_number":null,"branch":"agent/issue-82","elapsed_seconds":128,"input_tokens":24729,"output_tokens":812,"cost_usd":0.039,"pid":5678,"cpu_percent":98.2,"rss_bytes":512000000,"last_error":null,"weekly_percent_used":3.0,"last_activity":"bun test routes/call","activity_age_seconds":4,"turns":41}],"recent":[{"issue":88,"state":"done","pr_number":91,"cost_usd":0.12,"title":"..."}]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Type: "orchestrator", Endpoint: srv.URL}, "secret")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if !got.Healthy || got.Mode != "fast" || got.Repo != "pacosw1/pitsa-vps" {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
	if got.Daemon.PID != 1234 || got.Daemon.CPUPercent != 0.4 || got.Daemon.UptimeSeconds != 312 {
		t.Fatalf("unexpected daemon: %+v", got.Daemon)
	}
	if got.Budget.DayLimitUSD != 7.5 || got.Budget.DayRemainingUSD != 7.5 {
		t.Fatalf("unexpected budget: %+v", got.Budget)
	}
	if got.Totals.Live != 1 || got.Totals.CostUSD != 0.039 {
		t.Fatalf("unexpected totals: %+v", got.Totals)
	}
	if got.Limits == nil || got.Limits.Weekly == nil || got.Limits.Weekly.UsedPercent != 83.0 || got.Limits.Weekly.ResetsAt != 1787331928 {
		t.Fatalf("unexpected weekly limit: %+v", got.Limits)
	}
	if got.Limits.FiveHour == nil || got.Limits.FiveHour.UsedPercent != 12.0 || got.Limits.PlanType != "pro" {
		t.Fatalf("unexpected five_hour limit or plan type: %+v", got.Limits)
	}
	if len(got.Agents) != 1 || got.Agents[0].Issue != 82 || got.Agents[0].State != "implementing" || got.Agents[0].PRNumber != nil {
		t.Fatalf("unexpected agents: %+v", got.Agents)
	}
	a := got.Agents[0]
	if a.WeeklyPercentUsed == nil || *a.WeeklyPercentUsed != 3.0 {
		t.Fatalf("unexpected weekly_percent_used: %+v", a.WeeklyPercentUsed)
	}
	if a.LastActivity == nil || *a.LastActivity != "bun test routes/call" {
		t.Fatalf("unexpected last_activity: %+v", a.LastActivity)
	}
	if a.ActivityAgeSeconds == nil || *a.ActivityAgeSeconds != 4 {
		t.Fatalf("unexpected activity_age_seconds: %+v", a.ActivityAgeSeconds)
	}
	if a.Turns != 41 {
		t.Fatalf("Turns = %d, want 41", a.Turns)
	}
	if len(got.Recent) != 1 || got.Recent[0].Issue != 88 || got.Recent[0].PRNumber == nil || *got.Recent[0].PRNumber != 91 {
		t.Fatalf("unexpected recent: %+v", got.Recent)
	}
	if got.Name != "pitsa-agents" {
		t.Fatalf("Name = %q, want the provider name", got.Name)
	}
}

// A daemon that has not taken a usage reading yet sends limits as null.
// Callers must treat that as "unknown", not as an empty (0%) bar.
func TestFetchOrchestratorHandlesNullLimits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy":true,"limits":null,"agents":[]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if got.Limits != nil {
		t.Fatalf("Limits = %+v, want nil", got.Limits)
	}
}

// Either usage window can be null on its own, independent of the other.
func TestFetchOrchestratorHandlesOneNullUsageWindow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy":true,"limits":{"weekly":null,"five_hour":{"used_percent":12.0,"resets_at":1787300000},"plan_type":"pro"},"agents":[]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if got.Limits == nil {
		t.Fatal("want a non-nil Limits when one window is present")
	}
	if got.Limits.Weekly != nil {
		t.Fatalf("Limits.Weekly = %+v, want nil", got.Limits.Weekly)
	}
	if got.Limits.FiveHour == nil || got.Limits.FiveHour.UsedPercent != 12.0 {
		t.Fatalf("unexpected five_hour limit: %+v", got.Limits.FiveHour)
	}
}

// An agent's weekly_percent_used, last_activity, and activity_age_seconds
// can each be null on their own; a decode must not fail or fake a zero.
func TestFetchOrchestratorHandlesNullAgentActivityFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy":true,"agents":[{"issue":1,"state":"implementing","weekly_percent_used":null,"last_activity":null,"activity_age_seconds":null,"turns":0}]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if len(got.Agents) != 1 {
		t.Fatalf("want one agent, got %+v", got.Agents)
	}
	a := got.Agents[0]
	if a.WeeklyPercentUsed != nil || a.LastActivity != nil || a.ActivityAgeSeconds != nil {
		t.Fatalf("want all three activity fields nil, got %+v", a)
	}
}

func TestFetchOrchestratorReportsANon200Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error == "" {
		t.Fatal("want an error for a non-200 response")
	}
	if got.Healthy {
		t.Fatal("a failed fetch must never read as healthy")
	}
	if len(got.Agents) != 0 {
		t.Fatalf("want no agents on a failed fetch, got %+v", got.Agents)
	}
}

func TestFetchOrchestratorRejectsMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy": true, "agents": [{"issue": "not-a-number"}]`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error == "" {
		t.Fatal("want an error for malformed JSON")
	}
	if got.Healthy {
		t.Fatal("a decode failure must never read as healthy, even if the body claims healthy:true")
	}
	if len(got.Agents) != 0 {
		t.Fatalf("want no agents when decoding fails partway through, got %+v", got.Agents)
	}
}

func TestFetchOrchestratorIgnoresUnknownFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schema_version":1,"healthy":true,"mode":"fast","totals":{"live":2},"agents":[],"future_field":"whatever","daemon":{"pid":1,"future_stat":42}}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if !got.Healthy || got.Mode != "fast" || got.Totals.Live != 2 || got.Daemon.PID != 1 {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
}

// When the daemon does not report spark subagents at all, children must
// decode to nil, not to an empty slice. The two are different facts: "not
// reported" versus "reported, and there were none".
func TestFetchOrchestratorLeavesChildrenNilWhenAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy":true,"agents":[{"issue":1,"state":"implementing"}]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if got.Agents[0].Children != nil {
		t.Fatalf("Children = %+v, want nil when the key is absent", got.Agents[0].Children)
	}
}

// An explicit empty array means the task launched no subagents, which must
// decode to a non-nil, zero-length slice so callers can tell it apart from
// "not reported".
func TestFetchOrchestratorMarksChildrenEmptyWhenTaskLaunchedNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy":true,"agents":[{"issue":1,"state":"implementing","children":[]}]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if got.Agents[0].Children == nil {
		t.Fatal("Children = nil, want a non-nil empty slice for an explicit []")
	}
	if len(got.Agents[0].Children) != 0 {
		t.Fatalf("Children = %+v, want zero entries", got.Agents[0].Children)
	}
}

// One running and one failed child must both decode with their full detail,
// including a nil pid on the child that has not been assigned one yet.
func TestFetchOrchestratorParsesRunningAndFailedChildren(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy":true,"agents":[{"issue":82,"state":"implementing","children":[{"id":"spark-82-1","model":"gpt-5.3-codex-spark","state":"running","task":"update the Dockerfile COPY list","elapsed_seconds":12,"input_tokens":1200,"output_tokens":300,"pid":91234,"exit_code":null},{"id":"spark-82-2","model":"gpt-5.3-codex-spark","state":"failed","task":"regenerate the lockfile","elapsed_seconds":30,"input_tokens":800,"output_tokens":50,"pid":null,"exit_code":1}],"children_running":1,"children_done":0,"children_failed":1}]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	a := got.Agents[0]
	if a.ChildrenRunning != 1 || a.ChildrenDone != 0 || a.ChildrenFailed != 1 {
		t.Fatalf("unexpected children counters: %+v", a)
	}
	if len(a.Children) != 2 {
		t.Fatalf("want 2 children, got %+v", a.Children)
	}
	running, failed := a.Children[0], a.Children[1]
	if running.State != "running" || running.PID == nil || *running.PID != 91234 || running.ExitCode != nil {
		t.Fatalf("unexpected running child: %+v", running)
	}
	if failed.State != "failed" || failed.PID != nil || failed.ExitCode == nil || *failed.ExitCode != 1 {
		t.Fatalf("unexpected failed child: %+v", failed)
	}
	if failed.Task != "regenerate the lockfile" {
		t.Fatalf("Task = %q", failed.Task)
	}
}

// The daemon's overall disk usage is nil when no reading is available, and
// an agent's worktree disk usage is nil when it has not been measured yet.
func TestFetchOrchestratorHandlesNullDiskReadings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy":true,"disk":null,"agents":[{"issue":1,"state":"implementing","worktree":"/home/pitsa/worktrees/issue-1","worktree_disk_bytes":null}]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if got.Disk != nil {
		t.Fatalf("Disk = %+v, want nil", got.Disk)
	}
	if got.Agents[0].Worktree != "/home/pitsa/worktrees/issue-1" {
		t.Fatalf("Worktree = %q", got.Agents[0].Worktree)
	}
	if got.Agents[0].WorktreeDiskBytes != nil {
		t.Fatalf("WorktreeDiskBytes = %v, want nil", got.Agents[0].WorktreeDiskBytes)
	}
}

// Once the daemon reports readings, disk usage and the per-agent worktree
// size decode with their real values.
func TestFetchOrchestratorParsesDiskReadings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy":true,"disk":{"total_bytes":500000000000,"free_bytes":120000000000,"used_bytes":380000000000},"agents":[{"issue":1,"state":"implementing","worktree":"/home/pitsa/worktrees/issue-1","worktree_disk_bytes":734003200}]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if got.Disk == nil || got.Disk.TotalBytes != 500000000000 || got.Disk.UsedBytes != 380000000000 {
		t.Fatalf("unexpected disk: %+v", got.Disk)
	}
	if got.Agents[0].WorktreeDiskBytes == nil || *got.Agents[0].WorktreeDiskBytes != 734003200 {
		t.Fatalf("unexpected worktree disk: %v", got.Agents[0].WorktreeDiskBytes)
	}
}

// Tasks follows the same absent-vs-empty rule as children: nil means the
// daemon does not report a checklist, and a non-nil empty slice means the
// agent tracks a checklist that currently has zero items.
func TestFetchOrchestratorHandlesTaskChecklist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy":true,"agents":[{"issue":1,"state":"implementing"},{"issue":2,"state":"implementing","tasks":[]},{"issue":3,"state":"implementing","tasks":[{"text":"read the existing helpers","done":true},{"text":"write tests","done":false}]}]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if got.Agents[0].Tasks != nil {
		t.Fatalf("Tasks = %+v, want nil when absent", got.Agents[0].Tasks)
	}
	if got.Agents[1].Tasks == nil || len(got.Agents[1].Tasks) != 0 {
		t.Fatalf("Tasks = %+v, want a non-nil empty slice", got.Agents[1].Tasks)
	}
	tasks := got.Agents[2].Tasks
	if len(tasks) != 2 || !tasks[0].Done || tasks[1].Done {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	if tasks[0].Text != "read the existing helpers" {
		t.Fatalf("Text = %q", tasks[0].Text)
	}
}

// A subscription account has no per-call price, so its dollar figure is a
// computed estimate. It must carry the "~" marker and be labeled by plan.
func TestFetchOrchestratorMarksSubscriptionCostAsAnEstimate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy":true,"budget":{"day_usd":4.91,"day_limit_usd":7.5},"auth":{"mode":"subscription","plan_type":"pro","billed":false},"cost_is_estimate":true,"agents":[]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if got.Auth.Mode != "subscription" || got.Auth.PlanType != "pro" || got.Auth.Billed {
		t.Fatalf("unexpected auth: %+v", got.Auth)
	}
	if !got.CostIsEstimate {
		t.Fatal("CostIsEstimate = false, want true for a subscription")
	}
	if !strings.Contains(got.CostText(), "~") {
		t.Fatalf("CostText() = %q, want the estimate marker \"~\"", got.CostText())
	}
	if got.AccountLabel() != "codex pro" {
		t.Fatalf("AccountLabel() = %q, want %q", got.AccountLabel(), "codex pro")
	}
}

// An API key account is real billed money: no "~", no estimate wording.
func TestFetchOrchestratorShowsAPIKeyCostAsReal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy":true,"budget":{"day_usd":4.91,"day_limit_usd":7.5},"auth":{"mode":"api_key","billed":true},"cost_is_estimate":false,"agents":[]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if got.CostIsEstimate {
		t.Fatal("CostIsEstimate = true, want false for an api_key account")
	}
	if strings.Contains(got.CostText(), "~") || strings.Contains(got.CostText(), "est") {
		t.Fatalf("CostText() = %q, want no estimate marker for real billed spend", got.CostText())
	}
	if got.AccountLabel() != "api key" {
		t.Fatalf("AccountLabel() = %q, want %q", got.AccountLabel(), "api key")
	}
}

// An unknown account is treated as billed by the daemon; it must never be
// labeled an estimate, since that would hide real spending.
func TestFetchOrchestratorNeverMarksUnknownAccountAsAnEstimate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"healthy":true,"budget":{"day_usd":4.91,"day_limit_usd":7.5},"auth":{"mode":"unknown","billed":true},"cost_is_estimate":false,"agents":[]}`))
	}))
	defer srv.Close()
	got := FetchOrchestrator(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "t")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if got.CostIsEstimate {
		t.Fatal("CostIsEstimate = true, want false for an unknown account")
	}
	if strings.Contains(got.CostText(), "~") || strings.Contains(got.CostText(), "est") {
		t.Fatalf("CostText() = %q, want no estimate marker for an unknown (billed) account", got.CostText())
	}
	if !strings.Contains(got.CostText(), "billed") {
		t.Fatalf("CostText() = %q, want it to say billed", got.CostText())
	}
}

// A valid mode request must hit POST /api/mode with the bearer token and
// the requested mode, and report the daemon's own ok/mode back.
func TestSetOrchestratorModeSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/mode" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected request: %s %s %s", r.Method, r.URL, r.Header.Get("Authorization"))
		}
		var body struct {
			Mode string `json:"mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Mode != "economy" {
			t.Fatalf("unexpected body: err=%v mode=%q", err, body.Mode)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"mode":"economy"}`))
	}))
	defer srv.Close()
	got := SetOrchestratorMode(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "secret", "economy")
	if !got.OK || got.Mode != "economy" || got.Error != "" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

// An invalid mode gets a 400 with the daemon's own error text; the caller
// must surface that text, not invent its own message.
func TestSetOrchestratorModeSurfacesTheDaemonsErrorOn400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"error":"\"turbo\" is not a mode. Use one of: fast, economy, paused."}`))
	}))
	defer srv.Close()
	got := SetOrchestratorMode(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "secret", "turbo")
	if got.OK {
		t.Fatal("OK = true, want false for a rejected mode")
	}
	want := `"turbo" is not a mode. Use one of: fast, economy, paused.`
	if got.Error != want {
		t.Fatalf("Error = %q, want the daemon's exact text %q", got.Error, want)
	}
}

// A wrong or missing token gets a 401 with no body; that must not be
// mistaken for success or produce a confusing JSON-decode error.
func TestSetOrchestratorModeReportsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	got := SetOrchestratorMode(context.Background(), config.Widget{Name: "pitsa-agents", Endpoint: srv.URL}, "wrong", "fast")
	if got.OK {
		t.Fatal("OK = true, want false for a 401")
	}
	if got.Error == "" {
		t.Fatal("want a non-empty error for a 401")
	}
}
