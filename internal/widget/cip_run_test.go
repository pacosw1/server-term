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

const runDetailJSON = `{"run":{"ID":40,"Pipeline":"padel-bros","Backend":"docker","Repo":"padel-bros",
 "SHA":"89e4f207aabbccdd","Branch":"main","Trigger":"push","Status":"running",
 "StartedAt":"2026-08-27T20:55:42.02Z","FinishedAt":"0001-01-01T00:00:00Z"},
 "jobs":[
  {"Name":"lint","Status":"success","LogPath":"/l/lint","Needs":[],
   "StartedAt":"2026-08-27T20:55:43Z","FinishedAt":"2026-08-27T20:56:13Z","StepsTotal":2,"StepsDone":2},
  {"Name":"build","Status":"running","LogPath":"/l/build","Needs":[],
   "StartedAt":"2026-08-27T20:55:43Z","FinishedAt":"0001-01-01T00:00:00Z","StepsTotal":3,"StepsDone":1},
  {"Name":"test","Status":"pending","Needs":["build"],
   "StartedAt":"0001-01-01T00:00:00Z","FinishedAt":"0001-01-01T00:00:00Z","StepsTotal":4,"StepsDone":0},
  {"Name":"deploy","Status":"pending","Needs":["test","lint"],
   "StartedAt":"0001-01-01T00:00:00Z","FinishedAt":"0001-01-01T00:00:00Z","StepsTotal":2,"StepsDone":0}]}`

func runDetailServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing bearer token on %s", r.URL.Path)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/runs/40" {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(runDetailJSON))
	}))
}

func TestFetchCIPRunReadsTheRunAndItsJobs(t *testing.T) {
	srv := runDetailServer(t)
	defer srv.Close()
	got := FetchCIPRun(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret", 40)
	if got.Error != "" {
		t.Fatalf("Error = %q, want none", got.Error)
	}
	if got.Run.ID != 40 || got.Run.Repo != "padel-bros" || got.Run.Status != "running" {
		t.Errorf("Run = %+v, want run 40 of padel-bros running", got.Run)
	}
	if got.Run.Finished {
		t.Error("the run is marked finished, want unfinished")
	}
	if len(got.Jobs) != 4 {
		t.Fatalf("len(Jobs) = %d, want 4", len(got.Jobs))
	}
	lint := got.Jobs[0]
	if lint.Name != "lint" || lint.Status != "success" || !lint.Finished || lint.StepsDone != 2 {
		t.Errorf("lint = %+v, want a finished success with 2 steps done", lint)
	}
	build := got.Jobs[1]
	if build.Finished {
		t.Error("the running build job is marked finished")
	}
	deploy := got.Jobs[3]
	if len(deploy.Needs) != 2 || deploy.Needs[0] != "test" {
		t.Errorf("deploy Needs = %v, want [test lint]", deploy.Needs)
	}
}

// A pending job has no start time. Its duration must be zero, never the age
// of the Go zero time, which would read as two thousand years.
func TestCIPJobDurationIsZeroBeforeAJobStarts(t *testing.T) {
	now := time.Date(2026, 8, 27, 21, 0, 0, 0, time.UTC)
	pending := CIPJob{Name: "test", Status: "pending"}
	if got := pending.Duration(now); got != 0 {
		t.Errorf("pending Duration = %v, want 0", got)
	}
	if pending.Started() {
		t.Error("a pending job reports that it started")
	}
	running := CIPJob{Name: "build", Status: "running", StartedAt: now.Add(-30 * time.Second)}
	if got := running.Duration(now); got != 30*time.Second {
		t.Errorf("running Duration = %v, want 30s", got)
	}
	done := CIPJob{Name: "lint", Status: "success", Finished: true,
		StartedAt: now.Add(-2 * time.Minute), FinishedAt: now.Add(-time.Minute)}
	if got := done.Duration(now); got != time.Minute {
		t.Errorf("finished Duration = %v, want 1m", got)
	}
}

func TestCIPJobColumnsGroupsJobsByDependencyDepth(t *testing.T) {
	srv := runDetailServer(t)
	defer srv.Close()
	got := FetchCIPRun(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret", 40)
	columns := CIPJobColumns(got.Jobs)
	if len(columns) != 3 {
		t.Fatalf("len(columns) = %d, want 3", len(columns))
	}
	if names := jobNames(columns[0]); strings.Join(names, ",") != "lint,build" {
		t.Errorf("column 1 = %v, want lint and build", names)
	}
	if names := jobNames(columns[1]); strings.Join(names, ",") != "test" {
		t.Errorf("column 2 = %v, want test", names)
	}
	// deploy needs test (depth 1) and lint (depth 0), so it sits one past
	// the deepest job it waits for.
	if names := jobNames(columns[2]); strings.Join(names, ",") != "deploy" {
		t.Errorf("column 3 = %v, want deploy", names)
	}
}

func jobNames(jobs []CIPJob) []string {
	out := make([]string, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, j.Name)
	}
	return out
}

// A need that names no known job must not hide the job or crash the layout.
func TestCIPJobColumnsIgnoresAnUnknownDependency(t *testing.T) {
	jobs := []CIPJob{{Name: "build", Needs: []string{"ghost"}}, {Name: "test", Needs: []string{"build"}}}
	columns := CIPJobColumns(jobs)
	if len(columns) != 2 {
		t.Fatalf("len(columns) = %d, want 2", len(columns))
	}
	if jobNames(columns[0])[0] != "build" || jobNames(columns[1])[0] != "test" {
		t.Errorf("columns = %v, want build then test", columns)
	}
}

// A cycle is bad data from the daemon. The layout must still finish and must
// still show every job, because a hidden job is worse than an odd order.
func TestCIPJobColumnsSurvivesACycle(t *testing.T) {
	done := make(chan [][]CIPJob, 1)
	go func() {
		done <- CIPJobColumns([]CIPJob{
			{Name: "a", Needs: []string{"b"}},
			{Name: "b", Needs: []string{"a"}},
			{Name: "c", Needs: []string{}},
		})
	}()
	select {
	case columns := <-done:
		count := 0
		for _, col := range columns {
			count += len(col)
		}
		if count != 3 {
			t.Errorf("layout kept %d jobs, want all 3", count)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("CIPJobColumns did not finish on a cycle")
	}
}

func TestCIPJobColumnsHandlesNoJobs(t *testing.T) {
	if got := CIPJobColumns(nil); len(got) != 0 {
		t.Errorf("CIPJobColumns(nil) = %v, want no columns", got)
	}
}

func TestFetchCIPRunFailsClosedWithoutAToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("request sent without a token: %s", r.URL.Path)
	}))
	defer srv.Close()
	got := FetchCIPRun(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "", 40)
	if got.Error == "" {
		t.Fatal("a missing token produced no error")
	}
}

func TestFetchCIPRunFailsClosedOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	got := FetchCIPRun(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret", 40)
	if got.Error == "" {
		t.Fatal("a 500 produced no error")
	}
}

func TestFetchCIPRunReportsAnUnauthorizedTokenClearly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	got := FetchCIPRun(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "super-secret", 40)
	if !strings.Contains(got.Error, "token") {
		t.Errorf("Error = %q, want it to name the token", got.Error)
	}
	if strings.Contains(got.Error, "super-secret") {
		t.Errorf("Error = %q, must not contain the token", got.Error)
	}
}

func TestFetchCIPRunFailsClosedOnBrokenJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"run":`))
	}))
	defer srv.Close()
	got := FetchCIPRun(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret", 40)
	if got.Error == "" {
		t.Fatal("a broken body produced no error")
	}
}

func TestCIPRunDetailCountsJobsByStatus(t *testing.T) {
	srv := runDetailServer(t)
	defer srv.Close()
	got := FetchCIPRun(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret", 40)
	if n := got.CountByStatus("pending"); n != 2 {
		t.Errorf("pending = %d, want 2", n)
	}
	if n := got.CountByStatus("running"); n != 1 {
		t.Errorf("running = %d, want 1", n)
	}
	if n := got.CountByStatus("success"); n != 1 {
		t.Errorf("success = %d, want 1", n)
	}
}
