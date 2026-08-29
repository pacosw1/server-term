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

// specJSON is the real spec of promotion 12 on the live daemon, with the
// job lists shortened. The keys are lower case, unlike the promotion and
// stage rows, which carry Go field names.
const specJSON = `{"name":"cip","version":"0","stages":[` +
	`{"name":"verify","jobs":[{"name":"test"},{"name":"vet"}]},` +
	`{"name":"build","needs":["verify"],"jobs":[{"name":"binary"}]},` +
	`{"name":"release","needs":["build"],"gates":[{"type":"manual"}],"jobs":[{"name":"verify-artifact"}]}]}`

// promotionsJSON is the shape of GET /promotions, newest first.
const promotionsJSON = `[
 {"promotion":{"ID":12,"Repo":"cip","SHA":"337ab0ccaabbccdd","Branch":"main","Trigger":"manual",
   "SpecJSON":"","State":"active","CreatedAt":"2026-08-28T00:30:00Z"},
  "stages":[
   {"PromotionID":12,"Stage":"verify","State":"passed","EligibleAt":"2026-08-28T00:30:01Z","GateIdx":0,
    "RunID":49,"ApprovedBy":"","ApprovedAt":"0001-01-01T00:00:00Z","ApproveReason":"","GateStartedAt":"2026-08-28T00:30:01Z"},
   {"PromotionID":12,"Stage":"build","State":"passed","EligibleAt":"2026-08-28T00:31:00Z","GateIdx":0,
    "RunID":50,"ApprovedBy":"","ApprovedAt":"0001-01-01T00:00:00Z","ApproveReason":"","GateStartedAt":"2026-08-28T00:31:00Z"},
   {"PromotionID":12,"Stage":"release","State":"gated","EligibleAt":"2026-08-28T00:32:06.832Z","GateIdx":0,
    "RunID":0,"ApprovedBy":"","ApprovedAt":"0001-01-01T00:00:00Z","ApproveReason":"","GateStartedAt":"2026-08-28T00:32:06.832Z"}]},
 {"promotion":{"ID":11,"Repo":"cip","SHA":"1122334455667788","Branch":"main","Trigger":"push",
   "SpecJSON":"","State":"failed","CreatedAt":"2026-08-28T00:10:00Z"},
  "stages":[
   {"PromotionID":11,"Stage":"verify","State":"failed","RunID":48,"GateIdx":0,
    "EligibleAt":"2026-08-28T00:10:01Z","ApprovedAt":"0001-01-01T00:00:00Z","GateStartedAt":"0001-01-01T00:00:00Z"},
   {"PromotionID":11,"Stage":"build","State":"pending","RunID":0,"GateIdx":0,
    "EligibleAt":"0001-01-01T00:00:00Z","ApprovedAt":"0001-01-01T00:00:00Z","GateStartedAt":"0001-01-01T00:00:00Z"}]}]`

func promotionServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing bearer token on %s", r.URL.Path)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/promotions":
			_, _ = w.Write([]byte(promotionsJSON))
		case "/promotions/12":
			// The detail carries the spec; the list leaves it empty.
			body := `{"promotion":{"ID":12,"Repo":"cip","SHA":"337ab0ccaabbccdd","Branch":"main",
			 "Trigger":"manual","State":"active","CreatedAt":"2026-08-28T00:30:00Z","SpecJSON":` +
				quoteJSON(specJSON) + `},"stages":[]}`
			_, _ = w.Write([]byte(body))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// quoteJSON embeds a JSON document as a JSON string value.
func quoteJSON(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func TestFetchCIPPromotionsReadsThePromotionsAndStages(t *testing.T) {
	srv := promotionServer(t)
	defer srv.Close()
	got := FetchCIPPromotions(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret")
	if got.Error != "" {
		t.Fatalf("Error = %q, want none", got.Error)
	}
	if len(got.Promotions) != 2 {
		t.Fatalf("len(Promotions) = %d, want 2", len(got.Promotions))
	}
	first := got.Promotions[0]
	if first.Promotion.ID != 12 || first.Promotion.Repo != "cip" || first.Promotion.State != "active" {
		t.Errorf("promotion = %+v, want the active promotion 12 of cip", first.Promotion)
	}
	if len(first.Stages) != 3 {
		t.Fatalf("len(Stages) = %d, want 3", len(first.Stages))
	}
	release := first.Stages[2]
	if release.Stage != "release" || release.State != "gated" {
		t.Errorf("third stage = %+v, want release gated", release)
	}
	if release.RunID != 0 || release.HasRun() {
		t.Error("the gated stage claims a run, want none")
	}
	if first.Stages[0].RunID != 49 || !first.Stages[0].HasRun() {
		t.Errorf("verify RunID = %d, want 49", first.Stages[0].RunID)
	}
	if release.Approved() {
		t.Error("the gated stage reports an approval, want none")
	}
}

func TestFetchCIPPromotionParsesTheSpec(t *testing.T) {
	srv := promotionServer(t)
	defer srv.Close()
	got := FetchCIPPromotion(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret", 12)
	if got.Error != "" {
		t.Fatalf("Error = %q, want none", got.Error)
	}
	if len(got.Spec.Stages) != 3 {
		t.Fatalf("len(Spec.Stages) = %d, want 3", len(got.Spec.Stages))
	}
	if got.Spec.Stages[0].Name != "verify" || len(got.Spec.Stages[0].Needs) != 0 {
		t.Errorf("verify = %+v, want no dependency", got.Spec.Stages[0])
	}
	if len(got.Spec.Stages[1].Needs) != 1 || got.Spec.Stages[1].Needs[0] != "verify" {
		t.Errorf("build Needs = %v, want [verify]", got.Spec.Stages[1].Needs)
	}
	gates := got.Spec.Stages[2].Gates
	if len(gates) != 1 || gates[0].Type != "manual" {
		t.Errorf("release gates = %+v, want one manual gate", gates)
	}
}

// A stage waits for a reason. The reader must see the reason, not only the
// word "gated".
func TestCIPGateDescribeNamesTheReason(t *testing.T) {
	for _, test := range []struct {
		gate CIPGate
		want string
	}{
		{CIPGate{Type: "manual"}, "awaiting approval"},
		{CIPGate{Type: "bake", Minutes: 30}, "bake 30m"},
		{CIPGate{Type: "check", Run: "curl -f https://x"}, "check"},
		{CIPGate{Type: "alarm", Minutes: 15}, "alarm 15m"},
		{CIPGate{Type: "window"}, "time window"},
	} {
		if got := test.gate.Describe(); !strings.Contains(got, test.want) {
			t.Errorf("%s gate says %q, want it to contain %q", test.gate.Type, got, test.want)
		}
	}
}

// A gate type this build does not know must still say something honest,
// rather than claim the stage waits for nothing.
func TestCIPGateDescribeHandlesAnUnknownType(t *testing.T) {
	got := CIPGate{Type: "moonphase"}.Describe()
	if got == "" || !strings.Contains(got, "moonphase") {
		t.Errorf("Describe = %q, want it to name the unknown gate", got)
	}
	if empty := (CIPGate{}).Describe(); empty == "" {
		t.Error("an empty gate describes nothing, want an honest wait message")
	}
}

func TestCIPSpecGateForFindsTheGateAtTheIndex(t *testing.T) {
	srv := promotionServer(t)
	defer srv.Close()
	got := FetchCIPPromotion(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret", 12)
	gate, ok := got.Spec.GateFor("release", 0)
	if !ok || gate.Type != "manual" {
		t.Errorf("GateFor(release,0) = %+v %v, want a manual gate", gate, ok)
	}
	if _, ok := got.Spec.GateFor("verify", 0); ok {
		t.Error("verify has no gate, but GateFor found one")
	}
	if _, ok := got.Spec.GateFor("release", 9); ok {
		t.Error("GateFor accepted an index past the end")
	}
	if _, ok := got.Spec.GateFor("ghost", 0); ok {
		t.Error("GateFor found a gate for a stage that does not exist")
	}
}

func TestCIPStageColumnsLaysOutTheFlowByNeeds(t *testing.T) {
	srv := promotionServer(t)
	defer srv.Close()
	list := FetchCIPPromotions(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret")
	detail := FetchCIPPromotion(context.Background(), config.Widget{Name: "cip", Endpoint: srv.URL}, "secret", 12)
	columns := CIPStageColumns(list.Promotions[0].Stages, detail.Spec)
	if len(columns) != 3 {
		t.Fatalf("len(columns) = %d, want 3 (verify, build, release)", len(columns))
	}
	for i, want := range []string{"verify", "build", "release"} {
		if len(columns[i]) != 1 || columns[i][0].Stage != want {
			t.Errorf("column %d = %v, want %s", i+1, stageNames(columns[i]), want)
		}
	}
}

func stageNames(stages []CIPStage) []string {
	out := make([]string, 0, len(stages))
	for _, s := range stages {
		out = append(out, s.Stage)
	}
	return out
}

// Without a spec there are no edges. Every stage must still appear, in the
// order the daemon gave, so the flow is never empty while the spec loads.
func TestCIPStageColumnsWithoutASpecKeepsEveryStage(t *testing.T) {
	stages := []CIPStage{{Stage: "verify"}, {Stage: "build"}, {Stage: "release"}}
	columns := CIPStageColumns(stages, CIPSpec{})
	count := 0
	for _, col := range columns {
		count += len(col)
	}
	if count != 3 {
		t.Errorf("layout kept %d stages, want 3", count)
	}
}

func TestCIPStageColumnsHandlesNoStages(t *testing.T) {
	if got := CIPStageColumns(nil, CIPSpec{}); len(got) != 0 {
		t.Errorf("CIPStageColumns(nil) = %v, want no columns", got)
	}
}

// Two stages that both wait for one stage must share a column, so a fan-out
// draws as a fan-out.
func TestCIPStageColumnsPutsParallelStagesInOneColumn(t *testing.T) {
	stages := []CIPStage{{Stage: "build"}, {Stage: "beta"}, {Stage: "canary"}}
	spec := CIPSpec{Stages: []CIPSpecStage{
		{Name: "build"},
		{Name: "beta", Needs: []string{"build"}},
		{Name: "canary", Needs: []string{"build"}},
	}}
	columns := CIPStageColumns(stages, spec)
	if len(columns) != 2 {
		t.Fatalf("len(columns) = %d, want 2", len(columns))
	}
	if len(columns[1]) != 2 {
		t.Errorf("column 2 = %v, want beta and canary together", stageNames(columns[1]))
	}
}

func TestFetchCIPPromotionsFailsClosed(t *testing.T) {
	t.Run("no token", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("request sent without a token: %s", r.URL.Path)
		}))
		defer srv.Close()
		if got := FetchCIPPromotions(context.Background(), config.Widget{Endpoint: srv.URL}, ""); got.Error == "" {
			t.Error("a missing token produced no error")
		}
	})
	t.Run("non 200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		if got := FetchCIPPromotions(context.Background(), config.Widget{Endpoint: srv.URL}, "t"); got.Error == "" {
			t.Error("a 500 produced no error")
		}
	})
	t.Run("broken json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[{"promotion":`))
		}))
		defer srv.Close()
		if got := FetchCIPPromotions(context.Background(), config.Widget{Endpoint: srv.URL}, "t"); got.Error == "" {
			t.Error("a broken body produced no error")
		}
	})
}

func TestFetchCIPPromotionFailsClosedAndKeepsTheID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	got := FetchCIPPromotion(context.Background(), config.Widget{Endpoint: srv.URL}, "super-secret", 12)
	if got.Error == "" {
		t.Fatal("a 401 produced no error")
	}
	if !strings.Contains(got.Error, "token") {
		t.Errorf("Error = %q, want it to name the token", got.Error)
	}
	if strings.Contains(got.Error, "super-secret") {
		t.Errorf("Error = %q, must not contain the token", got.Error)
	}
	if got.Promotion.ID != 12 {
		t.Errorf("Promotion.ID = %d, want 12 kept on the error", got.Promotion.ID)
	}
}

// A spec that does not parse must not take the whole promotion down. The
// flow still draws from the stage rows; only the edges are missing.
func TestFetchCIPPromotionSurvivesABrokenSpec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"promotion":{"ID":12,"State":"active","SpecJSON":"{not json"},"stages":[{"Stage":"verify"}]}`))
	}))
	defer srv.Close()
	got := FetchCIPPromotion(context.Background(), config.Widget{Endpoint: srv.URL}, "t", 12)
	if got.Error != "" {
		t.Errorf("Error = %q, want none: a broken spec is not a failed read", got.Error)
	}
	if len(got.Stages) != 1 {
		t.Errorf("len(Stages) = %d, want the stage rows to survive", len(got.Stages))
	}
	if len(got.Spec.Stages) != 0 {
		t.Errorf("Spec.Stages = %v, want it empty after a broken spec", got.Spec.Stages)
	}
}

// An empty SpecJSON is normal: the list endpoint always leaves it empty.
func TestFetchCIPPromotionAcceptsAnEmptySpec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"promotion":{"ID":12,"State":"active","SpecJSON":""},"stages":[]}`))
	}))
	defer srv.Close()
	if got := FetchCIPPromotion(context.Background(), config.Widget{Endpoint: srv.URL}, "t", 12); got.Error != "" {
		t.Errorf("Error = %q, want none for an empty spec", got.Error)
	}
}

func TestCIPStageDurationUsesTheGateClock(t *testing.T) {
	now := time.Date(2026, 8, 28, 1, 0, 0, 0, time.UTC)
	gated := CIPStage{Stage: "release", State: "gated", GateStartedAt: now.Add(-20 * time.Minute)}
	if got := gated.WaitingFor(now); got != 20*time.Minute {
		t.Errorf("WaitingFor = %v, want 20m", got)
	}
	// A stage that never reached its gate has waited no time at all.
	fresh := CIPStage{Stage: "build", State: "pending"}
	if got := fresh.WaitingFor(now); got != 0 {
		t.Errorf("WaitingFor = %v, want 0 before the gate starts", got)
	}
}
