package widget

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

// runsJSON and storageJSON are the exact shapes the cip daemon returns.
const runsJSON = `[
 {"ID":40,"Pipeline":"padel-bros-shadow","Backend":"docker","Repo":"padel-bros-shadow",
  "SHA":"89e4f2074c1a5b6d","Branch":"main","Trigger":"manual","Stage":"","Status":"running",
  "StartedAt":"2026-08-27T20:55:42.02Z","FinishedAt":"0001-01-01T00:00:00Z"},
 {"ID":39,"Pipeline":"padel-bros","Backend":"docker","Repo":"padel-bros",
  "SHA":"1122334455667788","Branch":"main","Trigger":"push","Stage":"","Status":"failed",
  "StartedAt":"2026-08-27T20:50:00Z","FinishedAt":"2026-08-27T20:52:30Z"},
 {"ID":38,"Pipeline":"padel-bros","Backend":"docker","Repo":"padel-bros",
  "SHA":"aabbccddeeff0011","Branch":"dev","Trigger":"push","Stage":"","Status":"success",
  "StartedAt":"2026-08-27T20:40:00Z","FinishedAt":"2026-08-27T20:41:00Z"}
]`

const storageJSON = `{"repos":[{"name":"padel-bros","cacheBytes":0,"repoBytes":182452224,
 "logBytes":1024,"artifactBytes":2048,"workBytes":0}],
 "cipBytes":776925184,"diskFreeBytes":44778520576,"diskTotalBytes":104996663296}`

// cipServer serves both endpoints and checks the bearer token.
func cipServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing bearer token on %s", r.URL.Path)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/runs":
			_, _ = w.Write([]byte(runsJSON))
		case "/storage":
			_, _ = w.Write([]byte(storageJSON))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestFetchCIPNormalizesRunsAndStorage(t *testing.T) {
	srv := cipServer(t)
	defer srv.Close()
	got := FetchCIP(context.Background(), config.Widget{Name: "cip", Type: "cip", Endpoint: srv.URL}, "secret")
	if got.Error != "" {
		t.Fatalf("Error = %q, want none", got.Error)
	}
	if !got.Healthy {
		t.Errorf("Healthy = false, want true")
	}
	if got.Running != 1 || got.Failed != 1 || got.Succeeded != 1 {
		t.Errorf("running=%d failed=%d succeeded=%d, want 1/1/1", got.Running, got.Failed, got.Succeeded)
	}
	if len(got.Runs) != 3 {
		t.Fatalf("len(Runs) = %d, want 3", len(got.Runs))
	}
	first := got.Runs[0]
	if first.ID != 40 || first.Repo != "padel-bros-shadow" || first.Branch != "main" || first.Status != "running" {
		t.Errorf("first run = %+v, want run 40 of padel-bros-shadow on main", first)
	}
	if got.CIPBytes != 776925184 || got.DiskFreeBytes != 44778520576 || got.DiskTotalBytes != 104996663296 {
		t.Errorf("storage totals = %d/%d/%d, want 776925184/44778520576/104996663296", got.CIPBytes, got.DiskFreeBytes, got.DiskTotalBytes)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("len(Repos) = %d, want 1", len(got.Repos))
	}
	if want := int64(182452224 + 1024 + 2048); got.Repos[0].TotalBytes() != want {
		t.Errorf("repo total = %d, want %d", got.Repos[0].TotalBytes(), want)
	}
}

// The daemon writes an unfinished run as the Go zero time. The widget must
// treat that as "not finished" and never show it as a date.
func TestFetchCIPTreatsZeroTimeAsUnfinished(t *testing.T) {
	srv := cipServer(t)
	defer srv.Close()
	got := FetchCIP(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret")
	if got.Error != "" {
		t.Fatalf("Error = %q, want none", got.Error)
	}
	if got.Runs[0].Finished {
		t.Errorf("run 40 Finished = true, want false")
	}
	if !got.Runs[0].FinishedAt.IsZero() {
		t.Errorf("run 40 FinishedAt = %v, want the zero time", got.Runs[0].FinishedAt)
	}
	if !got.Runs[1].Finished {
		t.Errorf("run 39 Finished = false, want true")
	}
	now := time.Date(2026, 8, 27, 20, 56, 42, 0, time.UTC)
	line := got.Runs[0].Line(now)
	if strings.Contains(line, "0001") || strings.Contains(line, "1-01-01") {
		t.Errorf("Line = %q, must not render the zero time as a date", line)
	}
}

// The daemon writes a variable-length fractional second, for example ".02Z".
// The parse must keep the fraction instead of failing.
func TestFetchCIPParsesVariableFractionalSeconds(t *testing.T) {
	srv := cipServer(t)
	defer srv.Close()
	got := FetchCIP(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret")
	want := time.Date(2026, 8, 27, 20, 55, 42, 20000000, time.UTC)
	if !got.Runs[0].StartedAt.Equal(want) {
		t.Errorf("StartedAt = %v, want %v", got.Runs[0].StartedAt, want)
	}
}

func TestCIPRunDurationUsesTheFinishTimeOrTheElapsedTime(t *testing.T) {
	now := time.Date(2026, 8, 27, 21, 0, 0, 0, time.UTC)
	running := CIPRun{Status: "running", StartedAt: now.Add(-90 * time.Second)}
	if got := running.Duration(now); got != 90*time.Second {
		t.Errorf("running Duration = %v, want 1m30s", got)
	}
	done := CIPRun{Status: "success", Finished: true, StartedAt: now.Add(-10 * time.Minute), FinishedAt: now.Add(-8 * time.Minute)}
	if got := done.Duration(now); got != 2*time.Minute {
		t.Errorf("finished Duration = %v, want 2m", got)
	}
	// A clock skew must not produce a negative duration.
	skewed := CIPRun{Status: "running", StartedAt: now.Add(time.Minute)}
	if got := skewed.Duration(now); got != 0 {
		t.Errorf("skewed Duration = %v, want 0s", got)
	}
}

func TestCIPRunLineShowsRepoStatusBranchAndShortSHA(t *testing.T) {
	now := time.Date(2026, 8, 27, 21, 0, 0, 0, time.UTC)
	run := CIPRun{ID: 40, Repo: "padel-bros", Status: "failed", Branch: "main",
		SHA: "89e4f2074c1a5b6d", Finished: true, StartedAt: now.Add(-3 * time.Minute), FinishedAt: now.Add(-time.Minute)}
	line := run.Line(now)
	for _, want := range []string{"padel-bros", "failed", "main", "89e4f20", "2m"} {
		if !strings.Contains(line, want) {
			t.Errorf("Line = %q, want it to contain %q", line, want)
		}
	}
	if strings.Contains(line, "89e4f2074c1a5b6d") {
		t.Errorf("Line = %q, want the short SHA, not the full SHA", line)
	}
}

// A missing token must never reach the network, and must show an error.
func TestFetchCIPFailsClosedWithoutAToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("request sent without a token: %s", r.URL.Path)
	}))
	defer srv.Close()
	got := FetchCIP(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "")
	if got.Error == "" {
		t.Fatalf("Error = %q, want a missing token error", got.Error)
	}
	if got.Healthy {
		t.Errorf("Healthy = true, want false")
	}
}

func TestFetchCIPFailsClosedOnNon200(t *testing.T) {
	for _, test := range []struct{ name, failPath string }{
		{"runs", "/runs"},
		{"storage", "/storage"},
	} {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == test.failPath {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/runs":
					_, _ = w.Write([]byte(runsJSON))
				case "/storage":
					_, _ = w.Write([]byte(storageJSON))
				}
			}))
			defer srv.Close()
			got := FetchCIP(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret")
			if got.Error == "" {
				t.Fatalf("Error = %q, want a status error", got.Error)
			}
			if got.Healthy {
				t.Errorf("Healthy = true, want false")
			}
		})
	}
}

// A 401 must name the token, because that is the fault the reader must fix.
func TestFetchCIPReportsAnUnauthorizedTokenClearly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	got := FetchCIP(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "wrong")
	if !strings.Contains(got.Error, "token") {
		t.Errorf("Error = %q, want it to name the token", got.Error)
	}
}

func TestFetchCIPFailsClosedOnBrokenJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"not":"an array"`))
	}))
	defer srv.Close()
	got := FetchCIP(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret")
	if got.Error == "" || got.Healthy {
		t.Fatalf("snapshot = %+v, want an error and Healthy false", got)
	}
}

// The error must never contain the token. A log line or a JSON dump of the
// snapshot must stay safe to share.
func TestFetchCIPNeverPutsTheTokenInTheError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	got := FetchCIP(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "super-secret-token")
	if strings.Contains(got.Error, "super-secret-token") {
		t.Errorf("Error = %q, must not contain the token", got.Error)
	}
}

func TestCIPSnapshotSummaryAndStorageLines(t *testing.T) {
	srv := cipServer(t)
	defer srv.Close()
	got := FetchCIP(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret")
	summary := got.SummaryLine()
	for _, want := range []string{"running 1", "failed 1"} {
		if !strings.Contains(summary, want) {
			t.Errorf("SummaryLine = %q, want it to contain %q", summary, want)
		}
	}
	lines := got.StorageLines()
	if len(lines) == 0 {
		t.Fatalf("StorageLines is empty")
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"padel-bros", "GiB", "free"} {
		if !strings.Contains(joined, want) {
			t.Errorf("StorageLines = %q, want it to contain %q", joined, want)
		}
	}
}

// A failed read must show only the error. Zeroed storage rows read as real
// data, so they must not appear next to a fault.
func TestCIPSnapshotShowsNoDataRowsOnAnError(t *testing.T) {
	s := CIPSnapshot{Name: "cip", Error: "cip /runs: 500 Internal Server Error"}
	if got := s.StorageLines(); len(got) != 0 {
		t.Errorf("StorageLines = %q, want none on an error", got)
	}
	if got := s.RunLines(time.Now()); len(got) != 0 {
		t.Errorf("RunLines = %q, want none on an error", got)
	}
	if !strings.Contains(s.SummaryLine(), "cip /runs: 500") {
		t.Errorf("SummaryLine = %q, want it to show the error", s.SummaryLine())
	}
}

// The disk free share is the reason this widget exists: the root partition
// filled up once. A near-full disk must not read as healthy.
func TestFetchCIPIsNotHealthyWhenTheDiskIsNearlyFull(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/runs":
			_, _ = w.Write([]byte(`[]`))
		case "/storage":
			_, _ = w.Write([]byte(`{"repos":[],"cipBytes":1,"diskFreeBytes":3,"diskTotalBytes":100}`))
		}
	}))
	defer srv.Close()
	got := FetchCIP(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret")
	if got.Error != "" {
		t.Fatalf("Error = %q, want none", got.Error)
	}
	if got.Healthy {
		t.Errorf("Healthy = true, want false at 3%% free")
	}
	if got.DiskFreePercent() != 3 {
		t.Errorf("DiskFreePercent = %v, want 3", got.DiskFreePercent())
	}
}

// An empty run list is a healthy idle daemon, not a failure.
func TestFetchCIPIsHealthyWithNoRuns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/runs":
			_, _ = w.Write([]byte(`[]`))
		case "/storage":
			_, _ = w.Write([]byte(`{"repos":[],"cipBytes":0,"diskFreeBytes":50,"diskTotalBytes":100}`))
		}
	}))
	defer srv.Close()
	got := FetchCIP(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret")
	if got.Error != "" || !got.Healthy || len(got.Runs) != 0 {
		t.Fatalf("snapshot = %+v, want a healthy idle daemon", got)
	}
}

// The daemon can hold hundreds of runs. The widget keeps only the newest few
// so the panel stays readable.
func TestFetchCIPKeepsOnlyTheNewestRunsButCountsThemAll(t *testing.T) {
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < maxCIPRuns+5; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"ID":1,"Repo":"r","Status":"failed","StartedAt":"2026-08-27T20:00:00Z","FinishedAt":"2026-08-27T20:01:00Z"}`)
	}
	b.WriteString("]")
	body := b.String()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/runs":
			_, _ = w.Write([]byte(body))
		case "/storage":
			_, _ = w.Write([]byte(`{"repos":[],"cipBytes":0,"diskFreeBytes":50,"diskTotalBytes":100}`))
		}
	}))
	defer srv.Close()
	got := FetchCIP(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret")
	if len(got.Runs) != maxCIPRuns {
		t.Errorf("len(Runs) = %d, want %d", len(got.Runs), maxCIPRuns)
	}
	if got.Failed != maxCIPRuns+5 {
		t.Errorf("Failed = %d, want %d counted over every run", got.Failed, maxCIPRuns+5)
	}
}
