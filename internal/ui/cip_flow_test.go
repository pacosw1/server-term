package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/franciscosainzwilliams/server-term/internal/widget"
)

// promoSpec is the spec of the live promotion 12: verify → build → release,
// with a manual gate on release.
func promoSpec() widget.CIPSpec {
	return widget.CIPSpec{Name: "cip", Stages: []widget.CIPSpecStage{
		{Name: "verify"},
		{Name: "build", Needs: []string{"verify"}},
		{Name: "release", Needs: []string{"build"}, Gates: []widget.CIPGate{{Type: "manual"}}},
	}}
}

// gatedEntry is the live promotion 12: two stages passed, one gated on a
// human. This is the case the flow exists to show.
func gatedEntry() widget.CIPPromotionEntry {
	return widget.CIPPromotionEntry{
		Promotion: widget.CIPPromotion{ID: 12, Repo: "cip", SHA: "337ab0ccaabb", Branch: "main",
			State: "active", CreatedAt: graphNow.Add(-30 * time.Minute)},
		Stages: []widget.CIPStage{
			{PromotionID: 12, Stage: "verify", State: "passed", RunID: 49},
			{PromotionID: 12, Stage: "build", State: "passed", RunID: 50},
			{PromotionID: 12, Stage: "release", State: "gated", RunID: 0, GateIdx: 0,
				GateStartedAt: graphNow.Add(-20 * time.Minute)},
		},
	}
}

// failedEntry is the live promotion 11: verify failed, the rest never ran.
func failedEntry() widget.CIPPromotionEntry {
	return widget.CIPPromotionEntry{
		Promotion: widget.CIPPromotion{ID: 11, Repo: "cip", SHA: "112233445566", Branch: "main",
			State: "failed", CreatedAt: graphNow.Add(-60 * time.Minute)},
		Stages: []widget.CIPStage{
			{PromotionID: 11, Stage: "verify", State: "failed", RunID: 48},
			{PromotionID: 11, Stage: "build", State: "pending"},
			{PromotionID: 11, Stage: "release", State: "pending"},
		},
	}
}

func runningEntry() widget.CIPPromotionEntry {
	e := gatedEntry()
	e.Stages[2] = widget.CIPStage{PromotionID: 12, Stage: "release", State: "running", RunID: 51}
	return e
}

func TestStageFlowShowsEveryStageWithConnectors(t *testing.T) {
	view := cipStageFlowView(gatedEntry(), promoSpec(), graphNow, 0, 100)
	for _, want := range []string{"verify", "build", "release"} {
		if !strings.Contains(view, want) {
			t.Errorf("flow is missing the stage %q", want)
		}
	}
	if !strings.Contains(view, "▶") {
		t.Error("flow draws no connector between the stages")
	}
	if !strings.Contains(view, "✓") {
		t.Error("flow has no check mark for the passed stages")
	}
}

// The stages must read left to right in dependency order.
func TestStageFlowOrdersTheStagesByNeeds(t *testing.T) {
	view := cipStageFlowView(gatedEntry(), promoSpec(), graphNow, 0, 100)
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "verify") && strings.Contains(line, "build") {
			if strings.Index(line, "verify") > strings.Index(line, "build") {
				t.Errorf("verify comes after build on line %q", line)
			}
			return
		}
	}
	t.Error("no line holds both verify and build, so the flow is not horizontal")
}

// A gated stage waits for a person. It must say so, and it must not look
// like a stage that is doing work.
func TestGatedStageShowsTheReasonAndDoesNotLookLikeRunning(t *testing.T) {
	view := cipStageFlowView(gatedEntry(), promoSpec(), graphNow, 0, 100)
	if !strings.Contains(view, "awaiting approval") {
		t.Errorf("the gated stage does not say why it waits:\n%s", view)
	}
	if !strings.Contains(view, cipGatedMark) {
		t.Errorf("the gated stage has no distinct mark, want %q", cipGatedMark)
	}
	for i := 0; i < cipSpinnerFrames; i++ {
		if strings.Contains(view, cipSpinner(i)) {
			t.Errorf("the gated stage shows the running spinner mark %q", cipSpinner(i))
		}
	}
}

// A gate waits on a human, so its mark must not turn.
func TestGatedStageDoesNotAnimate(t *testing.T) {
	first := cipStageFlowView(gatedEntry(), promoSpec(), graphNow, 0, 100)
	second := cipStageFlowView(gatedEntry(), promoSpec(), graphNow, 4, 100)
	if first != second {
		t.Error("the gated stage animates, want it static")
	}
}

// A running stage IS working, so it turns.
func TestRunningStageAnimates(t *testing.T) {
	first := cipStageFlowView(runningEntry(), promoSpec(), graphNow, 0, 100)
	second := cipStageFlowView(runningEntry(), promoSpec(), graphNow, 4, 100)
	if first == second {
		t.Error("the running stage does not animate")
	}
}

// A promotion can end while a stage still says "running": a superseded
// promotion lets the stage in flight finish. The promotion decides the
// animation, not the stage, so an ended promotion must never turn a
// spinner and claim work that nobody watches.
func TestEndedPromotionDoesNotAnimateAStaleRunningStage(t *testing.T) {
	entry := runningEntry()
	entry.Promotion.State = "superseded"
	first := cipStageFlowView(entry, promoSpec(), graphNow, 0, 100)
	second := cipStageFlowView(entry, promoSpec(), graphNow, 4, 100)
	if first != second {
		t.Error("an ended promotion animates a stale running stage, want it static")
	}
	for i := 0; i < cipSpinnerFrames; i++ {
		if strings.Contains(first, cipSpinner(i)) {
			t.Errorf("an ended promotion shows the spinner mark %q", cipSpinner(i))
		}
	}
	// The same stages on a live promotion must animate, or this test would
	// pass for a flow that never animates at all.
	entry.Promotion.State = "active"
	if cipStageFlowView(entry, promoSpec(), graphNow, 0, 100) == cipStageFlowView(entry, promoSpec(), graphNow, 4, 100) {
		t.Error("a live promotion does not animate its running stage")
	}
}

// A promotion that ended never changes again.
func TestFinishedPromotionIsStatic(t *testing.T) {
	first := cipStageFlowView(failedEntry(), promoSpec(), graphNow, 0, 100)
	second := cipStageFlowView(failedEntry(), promoSpec(), graphNow, 4, 100)
	if first != second {
		t.Error("a failed promotion animates, want it static")
	}
	if !strings.Contains(first, "✗") {
		t.Error("the failed stage has no cross mark")
	}
	for _, want := range []string{"build", "release"} {
		if !strings.Contains(first, want) {
			t.Errorf("the failed promotion drops the stage %q", want)
		}
	}
}

// Without a spec the flow still shows every stage. Only the edges and the
// gate reason are missing.
func TestStageFlowWithoutASpecStillShowsTheStages(t *testing.T) {
	view := cipStageFlowView(gatedEntry(), widget.CIPSpec{}, graphNow, 0, 100)
	for _, want := range []string{"verify", "build", "release"} {
		if !strings.Contains(view, want) {
			t.Errorf("flow without a spec is missing %q", want)
		}
	}
}

// A gated stage with no spec yet must still say that it waits, rather than
// look like it is doing nothing.
func TestGatedStageWithoutASpecStillSaysItWaits(t *testing.T) {
	view := cipStageFlowView(gatedEntry(), widget.CIPSpec{}, graphNow, 0, 100)
	if !strings.Contains(view, cipGatedMark) {
		t.Error("the gated stage lost its mark without a spec")
	}
	if !strings.Contains(strings.ToLower(view), "gated") && !strings.Contains(view, "wait") {
		t.Errorf("the gated stage says nothing about waiting:\n%s", view)
	}
}

func TestStageFlowDegradesOnANarrowTerminal(t *testing.T) {
	for _, width := range []int{80, 60, 40, 20} {
		view := cipStageFlowView(gatedEntry(), promoSpec(), graphNow, 0, width)
		for _, want := range []string{"verify", "build", "release"} {
			if !strings.Contains(view, want) {
				t.Errorf("width %d drops the stage %q", width, want)
			}
		}
		for _, line := range strings.Split(view, "\n") {
			if w := lineWidth(line); w > width {
				t.Errorf("width %d produced a %d wide line: %q", width, w, line)
			}
		}
	}
}

// --- model wiring ---

func cipPromoModel() Model {
	m := cipGraphModel()
	m.cipPromotions = widget.CIPPromotionList{
		Name: "cip", At: graphNow,
		Promotions: []widget.CIPPromotionEntry{gatedEntry(), failedEntry()},
	}
	m.cipSpecs = map[int]widget.CIPSpec{12: promoSpec()}
	return m
}

// The promotions come first in the lower list, then the runs. With no
// promotion configured the list is exactly the run list it always was.
func TestPromotionsLeadTheListAndRunsFollow(t *testing.T) {
	m := cipPromoModel()
	rows := m.cipRows()
	if len(rows) != 2+len(m.cip.Runs) {
		t.Fatalf("len(rows) = %d, want 2 promotions plus %d runs", len(rows), len(m.cip.Runs))
	}
	if rows[0].Kind != cipRowPromotion || rows[2].Kind != cipRowRun {
		t.Errorf("rows = %+v, want promotions first then runs", rows)
	}
	bare := cipGraphModel()
	for _, row := range bare.cipRows() {
		if row.Kind != cipRowRun {
			t.Error("a model with no promotion shows a promotion row")
		}
	}
}

// The pinned pane follows the selection: a promotion shows the stage flow,
// a run shows the job graph.
func TestPinnedPaneFollowsTheSelectedRowKind(t *testing.T) {
	m := cipPromoModel()
	m.cipSel = 0
	flow := m.cipView()
	if !strings.Contains(flow, "awaiting approval") {
		t.Error("selecting a promotion does not show the stage flow")
	}
	m.cipSel = 2 // the first run
	graph := m.cipView()
	if strings.Contains(graph, "awaiting approval") {
		t.Error("selecting a run still shows the stage flow")
	}
	if !strings.Contains(graph, "build") {
		t.Error("selecting a run does not show the job graph")
	}
}

func TestEnterOnAPromotionOpensItsStages(t *testing.T) {
	m := cipPromoModel()
	m.cipSel = 0
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if m.cipOpenPromotionID != 12 {
		t.Fatalf("cipOpenPromotionID = %d, want 12", m.cipOpenPromotionID)
	}
	view := m.cipView()
	if !strings.Contains(view, "STAGES") {
		t.Error("the open promotion does not list its stages")
	}
	if strings.Contains(view, "RECENT RUNS") {
		t.Error("the open promotion still shows the run list")
	}
	if !strings.Contains(view, "awaiting approval") {
		t.Error("the open promotion lost the gate reason")
	}
}

// A stage links to its run. Opening the stage must show that run's jobs.
func TestEnterOnAStageOpensItsRun(t *testing.T) {
	m := cipPromoModel()
	m.cipOpenPromotionID, m.cipStageSel = 12, 0 // the verify stage, run 49
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if m.cipOpenID != 49 {
		t.Errorf("cipOpenID = %d, want run 49 of the verify stage", m.cipOpenID)
	}
	if cmd == nil {
		t.Error("opening a stage asked for no run detail")
	}
}

// A gated stage has no run yet. Enter must do nothing rather than open run 0.
func TestEnterOnAStageWithNoRunDoesNothing(t *testing.T) {
	m := cipPromoModel()
	m.cipOpenPromotionID, m.cipStageSel = 12, 2 // the gated release stage
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := model.(Model).cipOpenID; got != 0 {
		t.Errorf("cipOpenID = %d, want 0 for a stage with no run", got)
	}
	// Asking for run 0 would be a read of a run that cannot exist.
	if cmd != nil {
		t.Error("a stage with no run still asked the daemon for a run")
	}
}

func TestArrowKeysMoveTheStageSelectionWhenAPromotionIsOpen(t *testing.T) {
	m := cipPromoModel()
	m.cipOpenPromotionID = 12
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(Model)
	if m.cipStageSel != 1 {
		t.Errorf("cipStageSel = %d, want 1", m.cipStageSel)
	}
	if m.cipSel != 0 {
		t.Error("moving inside a promotion also moved the outer list")
	}
}

// Escape must walk back one level at a time.
func TestEscapeWalksBackOneLevel(t *testing.T) {
	m := cipPromoModel()
	m.cipOpenPromotionID, m.cipOpenID = 12, 49
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(Model)
	if m.cipOpenID != 0 || m.cipOpenPromotionID != 12 {
		t.Fatalf("first escape gave run=%d promotion=%d, want run 0 and promotion 12",
			m.cipOpenID, m.cipOpenPromotionID)
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(Model)
	if m.cipOpenPromotionID != 0 {
		t.Errorf("second escape gave promotion=%d, want 0", m.cipOpenPromotionID)
	}
	if !m.detail || m.detailTab != tabCIP {
		t.Error("escape left the CIP tab too early")
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model.(Model).detail {
		t.Error("the third escape did not leave the detail view")
	}
}

// The spec of a promotion never changes, so it is read once and kept.
func TestTheSpecIsFetchedOnceAndCached(t *testing.T) {
	m := cipPromoModel()
	m.cipSel = 0 // promotion 12, already in the cache
	if cmd := m.fetchCIPFocusSpec(); cmd != nil {
		t.Error("a cached spec was fetched again")
	}
	m.cipSel = 1 // promotion 11, not cached
	if cmd := m.fetchCIPFocusSpec(); cmd == nil {
		t.Error("an uncached spec was not fetched")
	}
}

func TestTheSpecCacheKeepsTheAnswer(t *testing.T) {
	m := cipPromoModel()
	detail := widget.CIPPromotionDetail{
		Promotion: widget.CIPPromotion{ID: 11}, Spec: promoSpec(),
	}
	model, _ := m.Update(cipPromotionMsg(detail))
	m = model.(Model)
	if _, ok := m.cipSpecs[11]; !ok {
		t.Error("the spec answer was not cached")
	}
}

// A failed spec read must not be cached as an empty spec forever, and must
// not hide the promotion.
func TestABrokenSpecReadIsNotCached(t *testing.T) {
	m := cipPromoModel()
	detail := widget.CIPPromotionDetail{Promotion: widget.CIPPromotion{ID: 11}, Error: "boom"}
	model, _ := m.Update(cipPromotionMsg(detail))
	if _, ok := model.(Model).cipSpecs[11]; ok {
		t.Error("a failed spec read was cached")
	}
}

// A promotions read error must be visible, not a blank pane.
func TestPromotionErrorIsVisible(t *testing.T) {
	m := cipPromoModel()
	m.cipPromotions = widget.CIPPromotionList{Error: "cip /promotions: 500 Internal Server Error"}
	m.cipSel = 0
	view := m.cipView()
	if !strings.Contains(view, "500 Internal Server Error") {
		t.Errorf("the promotions error is not visible:\n%s", view)
	}
}

func TestPromotionRowsAreClickable(t *testing.T) {
	m := cipPromoModel()
	y := lineOfPromotion(t, m, 12)
	model, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, Y: y})
	if got := model.(Model).cipOpenPromotionID; got != 12 {
		t.Errorf("the click gave cipOpenPromotionID = %d, want 12", got)
	}
}

func lineOfPromotion(t *testing.T, m Model, id int) int {
	t.Helper()
	for i, line := range strings.Split(m.View(), "\n") {
		if got, ok := cipPromotionIDAtLine(line); ok && got == id {
			return i
		}
	}
	t.Fatalf("no screen row holds promotion %d", id)
	return 0
}

func TestCIPPromotionIDAtLineReadsOnlyAPromotionRow(t *testing.T) {
	id, ok := cipPromotionIDAtLine("   P12   cip   main@337ab0c   active")
	if !ok || id != 12 {
		t.Errorf("got (%d,%v), want (12,true)", id, ok)
	}
	if _, ok := cipPromotionIDAtLine("  #40   padel-bros   running"); ok {
		t.Error("a run row parsed as a promotion row")
	}
	if _, ok := cipPromotionIDAtLine("  cip total   740.9 MiB"); ok {
		t.Error("a storage row parsed as a promotion row")
	}
}

// The list refresh must not poll every promotion's spec.
func TestPromotionTickDoesNotFetchEverySpec(t *testing.T) {
	m := cipPromoModel()
	m.cipSel = 0
	// Promotion 12 is cached, so a refresh of the list asks for nothing.
	model, cmd := m.Update(cipPromotionsMsg(m.cipPromotions))
	if cmd != nil {
		t.Error("a list refresh fetched a spec that was already cached")
	}
	if len(model.(Model).cipPromotions.Promotions) != 2 {
		t.Error("the list refresh lost the promotions")
	}
}
