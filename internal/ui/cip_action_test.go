package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/franciscosainzwilliams/server-term/internal/config"
)

func key(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

// --- re-run ---

// One press only arms the action. A stray key must never re-run a job.
func TestRerunNeedsASecondPressToAct(t *testing.T) {
	m := cipGraphModel()
	m.cipSel = 0 // run 40
	model, cmd := m.Update(key('r'))
	m = model.(Model)
	if m.cipActionBusy {
		t.Fatal("one press started the re-run, want it only armed")
	}
	if !m.cipActionArmed || m.cipAction != cipActionRerun {
		t.Fatalf("action=%q armed=%v, want an armed re-run", m.cipAction, m.cipActionArmed)
	}
	if cmd != nil {
		t.Error("arming the re-run already sent a request")
	}
	if !strings.Contains(m.cipView(), "again") {
		t.Error("the armed re-run does not tell the reader how to confirm")
	}
	model, cmd = m.Update(key('r'))
	m = model.(Model)
	if !m.cipActionBusy || cmd == nil {
		t.Error("the second press did not start the re-run")
	}
	if m.cipActionArmed {
		t.Error("the action stayed armed after it started")
	}
}

// Escape must drop an armed action without acting.
func TestEscapeCancelsAnArmedRerun(t *testing.T) {
	m := cipGraphModel()
	model, _ := m.Update(key('r'))
	m = model.(Model)
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(Model)
	if m.cipActionArmed || m.cipActionBusy {
		t.Error("escape left the action armed or running")
	}
	if cmd != nil {
		t.Error("escape sent a request")
	}
	// The run list must still be open: escape cancelled the action, not the view.
	if !m.detail || m.detailTab != tabCIP {
		t.Error("escape left the CIP tab instead of cancelling the action")
	}
}

// A run row re-runs every failed job. An open run with a job under the
// cursor re-runs only that job.
func TestRerunTargetIsTheRunOrTheFocusedJob(t *testing.T) {
	m := cipGraphModel()
	m.cipSel = 0
	id, job, ok := m.cipRerunTarget()
	if !ok || id != 40 || job != "" {
		t.Errorf("list target = (%d,%q,%v), want run 40 and every failed job", id, job, ok)
	}
	m.cipOpenID = 40
	m.cipJobSel = 1 // the build job
	id, job, ok = m.cipRerunTarget()
	if !ok || id != 40 || job != "build" {
		t.Errorf("open target = (%d,%q,%v), want run 40 and the build job", id, job, ok)
	}
}

// Inside a promotion, a stage row re-runs the failed jobs of its run.
func TestRerunTargetFollowsTheSelectedStage(t *testing.T) {
	m := cipPromoModel()
	m.cipOpenPromotionID, m.cipStageSel = 12, 0 // verify, run 49
	id, job, ok := m.cipRerunTarget()
	if !ok || id != 49 || job != "" {
		t.Errorf("stage target = (%d,%q,%v), want run 49 and every failed job", id, job, ok)
	}
}

// A stage with no run cannot be re-run. The reader must be told, not
// ignored.
func TestRerunSaysWhyWhenThereIsNoTarget(t *testing.T) {
	m := cipPromoModel()
	m.cipOpenPromotionID, m.cipStageSel = 12, 2 // the gated release stage, no run
	model, cmd := m.Update(key('r'))
	m = model.(Model)
	if m.cipActionArmed || cmd != nil {
		t.Error("a stage with no run armed a re-run")
	}
	if m.cipActionMessage == "" {
		t.Error("a re-run with no target said nothing")
	}
}

func TestRerunShowsTheDaemonsRefusal(t *testing.T) {
	m := cipGraphModel()
	m.cipActionBusy, m.cipAction = true, cipActionRerun
	model, _ := m.Update(cipActionMsg{Error: "cip refused: run is still going"})
	m = model.(Model)
	if m.cipActionBusy {
		t.Error("the action stayed busy after the answer")
	}
	if !m.cipActionIsError {
		t.Error("a refusal was not marked as an error")
	}
	if !strings.Contains(m.cipView(), "run is still going") {
		t.Errorf("the view hides the refusal:\n%s", m.cipView())
	}
}

// A success must be visible, and the widget must read the new state.
func TestRerunShowsTheOutcomeAndRefreshes(t *testing.T) {
	m := cipGraphModel()
	m.cipActionBusy, m.cipAction = true, cipActionRerun
	model, cmd := m.Update(cipActionMsg{Message: "run #92 runs build, checks again"})
	m = model.(Model)
	if m.cipActionIsError {
		t.Error("a success was marked as an error")
	}
	if !strings.Contains(m.cipView(), "runs build, checks again") {
		t.Error("the view hides the outcome")
	}
	if cmd == nil {
		t.Error("a finished action did not refresh the widget")
	}
}

// --- approve ---

// Approve is offered only for a gated stage.
func TestApproveOnlyOnAGatedStage(t *testing.T) {
	m := cipPromoModel()
	m.cipOpenPromotionID, m.cipStageSel = 12, 0 // verify, already passed
	model, cmd := m.Update(key('a'))
	m = model.(Model)
	if m.cipReasonInput || cmd != nil {
		t.Error("a passed stage opened an approval")
	}
	if !strings.Contains(m.cipActionMessage, "gated") {
		t.Errorf("message = %q, want it to say only a gated stage can be approved", m.cipActionMessage)
	}
}

func TestApproveAsksForAReasonThenConfirms(t *testing.T) {
	m := cipPromoModel()
	m.cipOpenPromotionID, m.cipStageSel = 12, 2 // the gated release stage
	model, cmd := m.Update(key('a'))
	m = model.(Model)
	if !m.cipReasonInput || m.cipAction != cipActionApprove {
		t.Fatalf("reasonInput=%v action=%q, want the reason prompt", m.cipReasonInput, m.cipAction)
	}
	if cmd != nil {
		t.Error("opening the reason prompt already approved the stage")
	}
	if !strings.Contains(m.cipView(), "reason") {
		t.Error("the prompt does not ask for a reason")
	}
	// The typed text must become the reason, not drive the list.
	for _, r := range "ship it" {
		model, _ = m.Update(key(r))
		m = model.(Model)
	}
	if m.cipReason != "ship it" {
		t.Fatalf("cipReason = %q, want %q", m.cipReason, "ship it")
	}
	if m.cipStageSel != 2 {
		t.Error("typing the reason moved the stage selection")
	}
	// Enter arms the approval; a second enter sends it.
	model, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if !m.cipActionArmed || m.cipActionBusy || cmd != nil {
		t.Fatalf("armed=%v busy=%v, want the approval armed only", m.cipActionArmed, m.cipActionBusy)
	}
	model, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if !m.cipActionBusy || cmd == nil {
		t.Error("the second enter did not send the approval")
	}
}

func TestApproveReasonAcceptsBackspaceAndEscapeCancels(t *testing.T) {
	m := cipPromoModel()
	m.cipOpenPromotionID, m.cipStageSel = 12, 2
	model, _ := m.Update(key('a'))
	m = model.(Model)
	for _, r := range "ho" {
		model, _ = m.Update(key(r))
		m = model.(Model)
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = model.(Model)
	if m.cipReason != "h" {
		t.Errorf("cipReason = %q, want %q after a backspace", m.cipReason, "h")
	}
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(Model)
	if m.cipReasonInput || m.cipAction != "" || cmd != nil {
		t.Error("escape did not cancel the approval")
	}
	if m.cipOpenPromotionID != 12 {
		t.Error("escape closed the promotion instead of the prompt")
	}
}

// The approval names who approved, so the audit trail is useful.
func TestApproveTargetAndApprover(t *testing.T) {
	m := cipPromoModel()
	m.cipOpenPromotionID, m.cipStageSel = 12, 2
	id, stage, ok := m.cipApproveTarget()
	if !ok || id != 12 || stage.Stage != "release" {
		t.Errorf("target = (%d,%q,%v), want promotion 12 stage release", id, stage.Stage, ok)
	}
	if cipApprover() == "" {
		t.Error("cipApprover is empty, want a name for the audit trail")
	}
}

// From the list, approving targets the gated stage of the selected
// promotion, so a reader does not have to open it first.
func TestApproveFromTheListTargetsTheGatedStage(t *testing.T) {
	m := cipPromoModel()
	m.cipSel = 0 // promotion 12, not open
	id, stage, ok := m.cipApproveTarget()
	if !ok || id != 12 || stage.Stage != "release" {
		t.Errorf("target = (%d,%q,%v), want the gated release stage", id, stage.Stage, ok)
	}
}

func TestApproveShowsTheDaemonsRefusal(t *testing.T) {
	m := cipPromoModel()
	m.cipActionBusy, m.cipAction = true, cipActionApprove
	model, _ := m.Update(cipActionMsg{Error: "cip refused: stage is not gated"})
	m = model.(Model)
	if !m.cipActionIsError || m.cipActionBusy {
		t.Error("the refusal was not recorded")
	}
	if !strings.Contains(m.cipView(), "stage is not gated") {
		t.Error("the view hides the refusal")
	}
}

// A second action must not start while one is still going.
func TestNoSecondActionWhileOneIsBusy(t *testing.T) {
	m := cipGraphModel()
	m.cipActionBusy = true
	model, cmd := m.Update(key('r'))
	if model.(Model).cipActionArmed || cmd != nil {
		t.Error("a second action started while one was busy")
	}
}

// The reason prompt must swallow the keys that drive the list, or typing a
// reason would move the selection under the reader.
func TestReasonInputSwallowsTheListKeys(t *testing.T) {
	m := cipPromoModel()
	m.cipOpenPromotionID, m.cipStageSel = 12, 1
	model, _ := m.Update(key('a'))
	m = model.(Model)
	// 'a' is not a gated stage here, so arrange the gated one instead.
	m = cipPromoModel()
	m.cipOpenPromotionID, m.cipStageSel = 12, 2
	model, _ = m.Update(key('a'))
	m = model.(Model)
	for _, r := range "jkra" {
		model, _ = m.Update(key(r))
		m = model.(Model)
	}
	if m.cipReason != "jkra" {
		t.Errorf("cipReason = %q, want the typed text", m.cipReason)
	}
	if m.cipStageSel != 2 || m.cipAction != cipActionApprove {
		t.Error("the typed keys drove the list instead of the prompt")
	}
}

// Moving the selection must drop an armed action, so the confirmation
// cannot apply to something the reader no longer points at.
func TestMovingTheSelectionDisarmsTheAction(t *testing.T) {
	m := cipGraphModel()
	model, _ := m.Update(key('r'))
	m = model.(Model)
	if !m.cipActionArmed {
		t.Fatal("the action did not arm")
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if model.(Model).cipActionArmed {
		t.Error("the armed action survived a move of the selection")
	}
}

// A token that cannot be read must stop the action and say so. It must
// never reach the daemon without one.
func TestRerunFailsClosedWhenTheTokenCannotBeRead(t *testing.T) {
	m := cipGraphModel()
	m.cfg.Widgets = []config.Widget{{Name: "cip", Type: "cip",
		Endpoint: "http://10.0.0.1:8080", TokenFile: "/no/such/token/file"}}
	cmd := m.rerunCIP(40, "")
	if cmd == nil {
		t.Fatal("rerunCIP returned no command")
	}
	msg, ok := cmd().(cipActionMsg)
	if !ok {
		t.Fatalf("rerunCIP returned %T, want cipActionMsg", cmd())
	}
	if msg.Error == "" {
		t.Error("an unreadable token produced no error")
	}
}

func TestApproveFailsClosedWhenTheTokenCannotBeRead(t *testing.T) {
	m := cipPromoModel()
	m.cfg.Widgets = []config.Widget{{Name: "cip", Type: "cip",
		Endpoint: "http://10.0.0.1:8080", TokenFile: "/no/such/token/file"}}
	cmd := m.approveCIPStage(12, "release", "why")
	if cmd == nil {
		t.Fatal("approveCIPStage returned no command")
	}
	msg, ok := cmd().(cipActionMsg)
	if !ok {
		t.Fatalf("approveCIPStage returned %T, want cipActionMsg", cmd())
	}
	if msg.Error == "" {
		t.Error("an unreadable token produced no error")
	}
}
