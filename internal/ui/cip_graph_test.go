package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/franciscosainzwilliams/server-term/internal/widget"
)

var graphNow = time.Date(2026, 8, 27, 21, 0, 0, 0, time.UTC)

// runningDetail is a run in flight: one job done, one running, two waiting.
func runningDetail() widget.CIPRunDetail {
	return widget.CIPRunDetail{
		Name: "cip", At: graphNow,
		Run: widget.CIPRun{ID: 40, Repo: "padel-bros", Status: "running", Branch: "main",
			SHA: "89e4f207aabbccdd", StartedAt: graphNow.Add(-2 * time.Minute)},
		Jobs: []widget.CIPJob{
			{Name: "lint", Status: "success", Finished: true, StepsTotal: 2, StepsDone: 2,
				StartedAt: graphNow.Add(-110 * time.Second), FinishedAt: graphNow.Add(-80 * time.Second)},
			{Name: "build", Status: "running", StepsTotal: 3, StepsDone: 1,
				StartedAt: graphNow.Add(-80 * time.Second)},
			{Name: "test", Status: "pending", Needs: []string{"build"}, StepsTotal: 4},
			{Name: "deploy", Status: "pending", Needs: []string{"test", "lint"}, StepsTotal: 2},
		},
	}
}

// finishedDetail is a run that already ended with a failure.
func finishedDetail() widget.CIPRunDetail {
	return widget.CIPRunDetail{
		Name: "cip", At: graphNow,
		Run: widget.CIPRun{ID: 39, Repo: "padel-bros", Status: "failed", Branch: "main",
			SHA: "1122334455667788", Finished: true,
			StartedAt: graphNow.Add(-10 * time.Minute), FinishedAt: graphNow.Add(-8 * time.Minute)},
		Jobs: []widget.CIPJob{
			{Name: "lint", Status: "success", Finished: true, StepsTotal: 2, StepsDone: 2,
				StartedAt: graphNow.Add(-10 * time.Minute), FinishedAt: graphNow.Add(-9 * time.Minute)},
			{Name: "build", Status: "failed", Finished: true, StepsTotal: 3, StepsDone: 2,
				StartedAt: graphNow.Add(-9 * time.Minute), FinishedAt: graphNow.Add(-8 * time.Minute)},
			{Name: "deploy", Status: "skipped", Needs: []string{"build"}, StepsTotal: 2},
		},
	}
}

func TestCIPGraphShowsEveryJobWithItsMark(t *testing.T) {
	view := cipGraphView(runningDetail(), graphNow, 0, 100)
	for _, want := range []string{"lint", "build", "test", "deploy"} {
		if !strings.Contains(view, want) {
			t.Errorf("graph is missing the job %q", want)
		}
	}
	if !strings.Contains(view, "✓") {
		t.Error("graph has no check mark for the finished job")
	}
	if !strings.Contains(view, "1/3") {
		t.Error("graph does not show the step progress of the running job")
	}
	if !strings.Contains(view, "2/4") && !strings.Contains(view, "0/4") {
		t.Error("graph does not show the step total of the pending job")
	}
}

func TestCIPGraphShowsAFailedAndASkippedJob(t *testing.T) {
	view := cipGraphView(finishedDetail(), graphNow, 0, 100)
	if !strings.Contains(view, "✗") {
		t.Error("graph has no cross mark for the failed job")
	}
	if !strings.Contains(view, "deploy") {
		t.Error("graph drops the skipped job")
	}
}

// The graph must lay the jobs out by dependency depth, with a connector
// between the columns.
func TestCIPGraphDrawsColumnsWithConnectors(t *testing.T) {
	view := cipGraphView(runningDetail(), graphNow, 0, 100)
	if !strings.Contains(view, "▶") {
		t.Error("graph draws no connector between the columns")
	}
	// build sits in column 1 and test in column 2, so on the line that holds
	// both, build must come first.
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "build") && strings.Contains(line, "test") {
			if strings.Index(line, "build") > strings.Index(line, "test") {
				t.Errorf("column order is wrong on line %q", line)
			}
			return
		}
	}
}

// A running job animates. The frame counter drives it, so a test can pin it.
func TestCIPGraphSpinnerFollowsTheFrameCounter(t *testing.T) {
	first := cipGraphView(runningDetail(), graphNow, 0, 100)
	second := cipGraphView(runningDetail(), graphNow, 1, 100)
	if first == second {
		t.Error("the graph does not change when the frame counter advances")
	}
	if cipSpinner(0) == cipSpinner(1) {
		t.Error("cipSpinner returns the same mark for two frames")
	}
	if cipSpinner(0) != cipSpinner(cipSpinnerFrames) {
		t.Error("cipSpinner does not repeat after a full cycle")
	}
}

// A finished run is static. A spinner there would suggest work still runs.
func TestCIPGraphHasNoSpinnerOnAFinishedRun(t *testing.T) {
	first := cipGraphView(finishedDetail(), graphNow, 0, 100)
	second := cipGraphView(finishedDetail(), graphNow, 3, 100)
	if first != second {
		t.Error("a finished run animates, want a static graph")
	}
	for i := 0; i < cipSpinnerFrames; i++ {
		if strings.Contains(first, cipSpinner(i)) {
			t.Errorf("a finished run shows the spinner mark %q", cipSpinner(i))
		}
	}
}

// A run can end while a job still says "running", for example when the
// daemon stopped. The run decides the animation, not the job: an ended run
// must never turn a spinner, because that claims work that nobody does.
func TestCIPGraphDoesNotAnimateAStaleRunningJobOfAnEndedRun(t *testing.T) {
	detail := widget.CIPRunDetail{
		Name: "cip", At: graphNow,
		Run: widget.CIPRun{ID: 41, Repo: "padel-bros", Status: "failed", Finished: true,
			StartedAt: graphNow.Add(-5 * time.Minute), FinishedAt: graphNow.Add(-time.Minute)},
		Jobs: []widget.CIPJob{
			{Name: "build", Status: "running", StepsTotal: 3, StepsDone: 1,
				StartedAt: graphNow.Add(-5 * time.Minute)},
		},
	}
	first := cipGraphView(detail, graphNow, 0, 100)
	second := cipGraphView(detail, graphNow, 3, 100)
	if first != second {
		t.Error("an ended run animates a stale running job, want a static graph")
	}
	for i := 0; i < cipSpinnerFrames; i++ {
		if strings.Contains(first, cipSpinner(i)) {
			t.Errorf("an ended run shows the spinner mark %q", cipSpinner(i))
		}
	}
	// A live run with the same job must animate, or the test above would
	// pass for a graph that never animates at all.
	detail.Run.Status, detail.Run.Finished = "running", false
	if cipGraphView(detail, graphNow, 0, 100) == cipGraphView(detail, graphNow, 3, 100) {
		t.Error("a live run does not animate its running job")
	}
}

func TestCIPGraphShowsElapsedForARunningJobAndFinalForADoneJob(t *testing.T) {
	view := cipGraphView(runningDetail(), graphNow, 0, 100)
	if !strings.Contains(view, "30s") {
		t.Error("graph does not show the 30s the lint job took")
	}
	if strings.Contains(view, "0001") {
		t.Error("graph renders the Go zero time")
	}
	// A pending job never started, so it must not claim an elapsed time.
	if strings.Contains(view, "17532") || strings.Contains(view, "h0m0s") {
		t.Errorf("graph shows an age for a job that did not start:\n%s", view)
	}
}

// The panel must stay usable in a narrow terminal.
func TestCIPGraphDegradesOnANarrowTerminal(t *testing.T) {
	for _, width := range []int{80, 60, 40, 20} {
		view := cipGraphView(runningDetail(), graphNow, 0, width)
		for _, want := range []string{"lint", "build", "test", "deploy"} {
			if !strings.Contains(view, want) {
				t.Errorf("width %d drops the job %q", width, want)
			}
		}
		for _, line := range strings.Split(view, "\n") {
			if w := lineWidth(line); w > width {
				t.Errorf("width %d produced a %d wide line: %q", width, w, line)
			}
		}
	}
}

func lineWidth(line string) int {
	return len([]rune(stripANSI(line)))
}

func TestCIPGraphShowsTheErrorState(t *testing.T) {
	view := cipGraphView(widget.CIPRunDetail{Error: "cip /runs/40: 500 Internal Server Error"}, graphNow, 0, 100)
	if !strings.Contains(view, "500 Internal Server Error") {
		t.Errorf("graph = %q, want the error text", view)
	}
}

func TestCIPGraphSaysWhenTheRunHasNoJobs(t *testing.T) {
	view := cipGraphView(widget.CIPRunDetail{Run: widget.CIPRun{ID: 7}, At: graphNow}, graphNow, 0, 100)
	if strings.TrimSpace(view) == "" {
		t.Error("a run with no job draws a blank panel")
	}
	if !strings.Contains(view, "No job") {
		t.Errorf("graph = %q, want it to say that no job exists", view)
	}
}

// --- selection, opening, and the two panes ---

func cipGraphModel() Model {
	m := cipModel(nil, cipWidgetConfig)
	m.detail, m.detailTab = true, tabCIP
	m.cip = widget.CIPSnapshot{
		Name: "cip", At: graphNow, Healthy: true, Running: 1, Failed: 1,
		Runs: []widget.CIPRun{
			{ID: 40, Repo: "padel-bros", Status: "running", Branch: "main", SHA: "89e4f207aabb", StartedAt: graphNow.Add(-2 * time.Minute)},
			{ID: 39, Repo: "padel-bros", Status: "failed", Branch: "main", SHA: "112233445566", Finished: true,
				StartedAt: graphNow.Add(-10 * time.Minute), FinishedAt: graphNow.Add(-8 * time.Minute)},
			{ID: 38, Repo: "other", Status: "success", Branch: "dev", SHA: "aabbccddeeff", Finished: true,
				StartedAt: graphNow.Add(-20 * time.Minute), FinishedAt: graphNow.Add(-19 * time.Minute)},
		},
		DiskFreeBytes: 50, DiskTotalBytes: 100,
	}
	m.cipDetail = runningDetail()
	return m
}

func TestCIPArrowKeysMoveTheSelection(t *testing.T) {
	m := cipGraphModel()
	if m.cipSel != 0 {
		t.Fatalf("cipSel = %d, want 0", m.cipSel)
	}
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(Model)
	if m.cipSel != 1 {
		t.Errorf("after down cipSel = %d, want 1", m.cipSel)
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if got := model.(Model).cipSel; got != 0 {
		t.Errorf("after up cipSel = %d, want 0", got)
	}
}

func TestCIPSelectionStopsAtTheEnds(t *testing.T) {
	m := cipGraphModel()
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if got := model.(Model).cipSel; got != 0 {
		t.Errorf("up at the top gave %d, want 0", got)
	}
	m.cipSel = len(m.cip.Runs) - 1
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := model.(Model).cipSel; got != len(m.cip.Runs)-1 {
		t.Errorf("down at the end gave %d, want %d", got, len(m.cip.Runs)-1)
	}
}

// Moving the selection asks for that run's graph, because the graph above
// always shows the run the reader points at.
func TestMovingTheSelectionFetchesThatRunsGraph(t *testing.T) {
	m := cipGraphModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd == nil {
		t.Fatal("moving the selection asked for no run detail")
	}
}

func TestEnterOpensTheSelectedRun(t *testing.T) {
	m := cipGraphModel()
	m.cipSel = 1
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if m.cipOpenID != 39 {
		t.Errorf("cipOpenID = %d, want run 39", m.cipOpenID)
	}
	if cmd == nil {
		t.Error("opening a run asked for no detail")
	}
}

func TestEscapeClosesTheOpenRunAndKeepsTheTab(t *testing.T) {
	m := cipGraphModel()
	m.cipOpenID = 39
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(Model)
	if m.cipOpenID != 0 {
		t.Errorf("cipOpenID = %d, want 0 after escape", m.cipOpenID)
	}
	if !m.detail || m.detailTab != tabCIP {
		t.Error("escape left the CIP tab instead of closing the run")
	}
}

// With no run open, escape must keep its normal job of leaving the detail
// view. The new tab must not swallow it.
func TestEscapeWithNoOpenRunLeavesTheDetailView(t *testing.T) {
	m := cipGraphModel()
	m.cipOpenID = 0
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model.(Model).detail {
		t.Error("escape did not leave the detail view when no run was open")
	}
}

// The graph is pinned above in both modes, and only the lower pane changes.
func TestTheGraphStaysAboveInBothModes(t *testing.T) {
	m := cipGraphModel()
	list := m.cipView()
	if !strings.Contains(list, "RECENT RUNS") {
		t.Error("the closed view does not show the run list")
	}
	if !strings.Contains(list, "build") {
		t.Error("the closed view does not show the pipeline graph")
	}
	m.cipOpenID = 40
	open := m.cipView()
	if !strings.Contains(open, "build") {
		t.Error("the open view lost the pipeline graph")
	}
	if strings.Contains(open, "RECENT RUNS") {
		t.Error("the open view still shows the run list, want the run detail")
	}
	if !strings.Contains(open, "esc") {
		t.Error("the open view does not say how to go back")
	}
}

func TestTheOpenRunShowsItsJobs(t *testing.T) {
	m := cipGraphModel()
	m.cipOpenID = 40
	view := m.cipView()
	for _, want := range []string{"lint", "build", "test", "deploy"} {
		if !strings.Contains(view, want) {
			t.Errorf("the open run does not list the job %q", want)
		}
	}
}

// The graph must never show one run's jobs under another run's heading.
func TestTheGraphWaitsWhenTheDetailIsForAnotherRun(t *testing.T) {
	m := cipGraphModel()
	m.cipSel = 2 // run 38, while cipDetail holds run 40
	view := m.cipView()
	if strings.Contains(view, "deploy") {
		t.Errorf("the graph shows run 40's jobs while run 38 is selected:\n%s", view)
	}
	if !strings.Contains(view, "Loading") {
		t.Error("the graph does not say that it loads the selected run")
	}
}

// --- mouse ---

// One click opens the run it lands on. Two clicks cannot work here: opening
// a run redraws the graph above the list, so the rows move under the
// pointer between the two clicks.
func TestClickOpensTheRunItLandsOn(t *testing.T) {
	m := cipGraphModel()
	y := lineOfRun(t, m, 39)
	model, cmd := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, Y: y})
	m = model.(Model)
	if m.cipSel != 1 {
		t.Errorf("the click selected row %d, want run 39 at row 1", m.cipSel)
	}
	if m.cipOpenID != 39 {
		t.Errorf("cipOpenID = %d, want the clicked run 39", m.cipOpenID)
	}
	if cmd == nil {
		t.Error("opening by mouse asked for no detail")
	}
}

// A click must open the run under the pointer, not the run that happened to
// be selected before.
func TestClickOpensTheClickedRunNotTheSelectedOne(t *testing.T) {
	m := cipGraphModel()
	m.cipSel = 0
	y := lineOfRun(t, m, 38)
	model, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, Y: y})
	if got := model.(Model).cipOpenID; got != 38 {
		t.Errorf("cipOpenID = %d, want the clicked run 38", got)
	}
}

func TestClickOffARunRowChangesNothing(t *testing.T) {
	m := cipGraphModel()
	model, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, Y: 0})
	got := model.(Model)
	if got.cipSel != m.cipSel || got.cipOpenID != m.cipOpenID {
		t.Error("a click away from the run list changed the selection")
	}
}

// lineOfRun finds the screen row that holds the row for one run id.
func lineOfRun(t *testing.T, m Model, id int) int {
	t.Helper()
	for i, line := range strings.Split(m.View(), "\n") {
		if got, ok := cipRunIDAtLine(line); ok && got == id {
			return i
		}
	}
	t.Fatalf("no screen row holds run %d", id)
	return 0
}

func TestCIPRunIDAtLineReadsOnlyARunRow(t *testing.T) {
	if _, ok := cipRunIDAtLine("  RUN   REPO                     STATUS"); ok {
		t.Error("the header row parsed as a run row")
	}
	if _, ok := cipRunIDAtLine("  cip total                 740.9 MiB"); ok {
		t.Error("a storage row parsed as a run row")
	}
	id, ok := cipRunIDAtLine("  #40   padel-bros    running")
	if !ok || id != 40 {
		t.Errorf("cipRunIDAtLine gave (%d, %v), want (40, true)", id, ok)
	}
	// Styling wraps the row, so the parse must see through the codes.
	styled := errStyle.Render("  #39   padel-bros    failed")
	id, ok = cipRunIDAtLine(styled)
	if !ok || id != 39 {
		t.Errorf("a styled row gave (%d, %v), want (39, true)", id, ok)
	}
}

// --- fetch volume ---

// A finished run never changes, so it must not be polled.
func TestNoRunDetailTickForAFinishedRun(t *testing.T) {
	m := cipGraphModel()
	m.cipOpenID = 39
	m.cipDetail = finishedDetail()
	if cmd := m.nextCIPRunTick(); cmd != nil {
		t.Error("a finished run schedules a refresh, want none")
	}
}

func TestRunDetailTickRunsWhileTheRunIsRunning(t *testing.T) {
	m := cipGraphModel()
	m.cipOpenID = 40
	m.cipDetail = runningDetail()
	if cmd := m.nextCIPRunTick(); cmd == nil {
		t.Error("a running run schedules no refresh, want one")
	}
}

// The focused run is the open one, or the selected one when none is open.
func TestCIPFocusRunIDPrefersTheOpenRun(t *testing.T) {
	m := cipGraphModel()
	if got := m.cipFocusRunID(); got != 40 {
		t.Errorf("focus = %d, want the selected run 40", got)
	}
	m.cipSel = 2
	if got := m.cipFocusRunID(); got != 38 {
		t.Errorf("focus = %d, want the selected run 38", got)
	}
	m.cipOpenID = 39
	if got := m.cipFocusRunID(); got != 39 {
		t.Errorf("focus = %d, want the open run 39", got)
	}
}

func TestCIPFrameAdvancesOnTheFrameTick(t *testing.T) {
	m := cipGraphModel()
	before := m.cipFrame
	model, _ := m.Update(frameMsg(time.Now()))
	if got := model.(Model).cipFrame; got == before {
		t.Error("the frame tick did not advance the spinner counter")
	}
}

// Leaving the widget host must not keep a stale run open.
func TestSwitchingServerClosesTheOpenRun(t *testing.T) {
	m := cipGraphModel()
	m.cipOpenID = 40
	m.detail = false
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = model.(Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := model.(Model).cipOpenID; got != 0 {
		t.Errorf("cipOpenID = %d, want 0 on a server without cip", got)
	}
}
