package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/franciscosainzwilliams/server-term/internal/config"
	"github.com/franciscosainzwilliams/server-term/internal/widget"
)

// cipModel builds two servers where only the first one runs the cip daemon
// that the widget endpoint points at.
func cipModel(desktops []config.Desktop, widgets []config.Widget) Model {
	m := New(config.Config{
		Servers: []config.Server{
			{Name: "hetzner", Address: "10.0.0.1"},
			{Name: "office", Address: "10.0.0.2"},
		},
		Widgets:  widgets,
		Desktops: desktops,
	})
	for i := range m.samples {
		m.samples[i].Online, m.samples[i].At = true, time.Now()
	}
	m.width, m.height = 100, 40
	return m
}

var cipWidgetConfig = []config.Widget{{Name: "cip", Type: "cip", Endpoint: "http://10.0.0.1:8080", TokenEnv: "T"}}

func TestCIPTabShowsOnlyOnTheWidgetHost(t *testing.T) {
	m := cipModel(nil, cipWidgetConfig)
	m.detail = true
	if !strings.Contains(m.detailView(), "CIP") {
		t.Fatal("widget host does not show the CIP tab")
	}
	m.cursor = 1
	if strings.Contains(m.detailView(), "CIP") {
		t.Fatal("server without cip shows the CIP tab")
	}
}

func TestCIPTabIsAbsentWithoutAWidget(t *testing.T) {
	m := cipModel(nil, nil)
	m.detail = true
	if strings.Contains(m.detailView(), "CIP") {
		t.Fatal("an inventory with no cip widget shows the CIP tab")
	}
}

func TestCIPKeyIsIgnoredOffTheWidgetHost(t *testing.T) {
	m := cipModel(nil, cipWidgetConfig)
	m.detail, m.cursor = true, 1
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if model.(Model).detailTab == tabCIP {
		t.Fatal("c opened the CIP tab on a server without cip")
	}
}

func TestCIPKeyOpensTheTabOnTheWidgetHost(t *testing.T) {
	m := cipModel(nil, cipWidgetConfig)
	m.detail = true
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if got := model.(Model).detailTab; got != tabCIP {
		t.Fatalf("detailTab = %d, want the CIP tab %d", got, tabCIP)
	}
}

// The c key belonged to the desktop tab first. It must keep opening the
// desktop viewer there, so the new tab does not take an existing shortcut.
func TestCIPKeyLeavesTheDesktopShortcutAlone(t *testing.T) {
	m := cipModel([]config.Desktop{{Name: "d", Host: "10.0.0.1"}}, cipWidgetConfig)
	m.detail, m.detailTab = true, 7
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if got := model.(Model).detailTab; got != 7 {
		t.Fatalf("detailTab = %d, want the desktop tab 7 to stay selected", got)
	}
}

func TestTabCycleSkipsCIPOffTheWidgetHost(t *testing.T) {
	m := cipModel(nil, cipWidgetConfig)
	m.detail, m.cursor = true, 1
	for i := 0; i < 14; i++ {
		model, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = model.(Model)
		if m.detailTab == tabCIP {
			t.Fatal("tab cycle reached the CIP tab on a server without cip")
		}
	}
}

func TestDetailEntryLeavesCIPTabOnAnotherServer(t *testing.T) {
	m := cipModel(nil, cipWidgetConfig)
	m.detailTab = tabCIP
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = model.(Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := model.(Model).detailTab; got == tabCIP {
		t.Fatal("detail view kept the CIP tab on a server without cip")
	}
}

// The tab identity must not depend on which optional tabs are configured.
// A label is a key hint, and the index behind it drives the body. The two
// must agree in every combination, or a tab shows another tab's body.
func TestDetailTabIndicesStayCorrectInEveryCombination(t *testing.T) {
	desktop := []config.Desktop{{Name: "d", Host: "10.0.0.1"}}
	orchestrator := config.Widget{Name: "agents", Type: "orchestrator", Endpoint: "http://10.0.0.1:7844", TokenEnv: "T"}
	cip := cipWidgetConfig[0]
	want := map[string]int{
		"1 CPU": 0, "2 MEMORY": 1, "3 STORAGE": 2, "4 NETWORK": 3, "5 RUNNERS": 4,
		"6 PROCESSES": 5, "7 ACCEL": 6, "8 DESKTOP": 7, "9 SSH": 8, "10 DEVTOOLS": 9,
		"o AGENTS": 10, "c CIP": tabCIP,
	}
	for _, test := range []struct {
		name     string
		desktops []config.Desktop
		widgets  []config.Widget
	}{
		{"bare", nil, nil},
		{"desktop only", desktop, nil},
		{"cip only", nil, []config.Widget{cip}},
		{"orchestrator only", nil, []config.Widget{orchestrator}},
		{"cip and desktop", desktop, []config.Widget{cip}},
		{"everything", desktop, []config.Widget{orchestrator, cip}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := cipModel(test.desktops, test.widgets)
			for _, entry := range m.detailTabs(0) {
				if want[entry.Label] != entry.Index {
					t.Errorf("%q has index %d, want %d", entry.Label, entry.Index, want[entry.Label])
				}
			}
		})
	}
}

// Tab must reach every shown tab and no hidden one, whatever is configured.
func TestTabCycleVisitsEveryShownTabExactlyOnce(t *testing.T) {
	desktop := []config.Desktop{{Name: "d", Host: "10.0.0.1"}}
	orchestrator := config.Widget{Name: "agents", Type: "orchestrator", Endpoint: "http://10.0.0.1:7844", TokenEnv: "T"}
	cip := cipWidgetConfig[0]
	for _, test := range []struct {
		name     string
		desktops []config.Desktop
		widgets  []config.Widget
	}{
		{"bare", nil, nil},
		{"desktop only", desktop, nil},
		{"cip only", nil, []config.Widget{cip}},
		{"cip and desktop", desktop, []config.Widget{cip}},
		{"everything", desktop, []config.Widget{orchestrator, cip}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := cipModel(test.desktops, test.widgets)
			m.detail = true
			shown := map[int]bool{}
			for _, entry := range m.detailTabs(0) {
				shown[entry.Index] = true
			}
			seen := map[int]bool{}
			for i := 0; i < len(shown); i++ {
				model, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
				m = model.(Model)
				if !shown[m.detailTab] {
					t.Fatalf("tab cycle reached hidden tab %d", m.detailTab)
				}
				if seen[m.detailTab] {
					t.Fatalf("tab cycle repeated tab %d before visiting them all", m.detailTab)
				}
				seen[m.detailTab] = true
			}
			if len(seen) != len(shown) {
				t.Errorf("tab cycle visited %d tabs, want all %d", len(seen), len(shown))
			}
		})
	}
}

// A failed read must show the reason. A blank panel reads as healthy.
func TestCIPViewShowsTheError(t *testing.T) {
	m := cipModel(nil, cipWidgetConfig)
	m.detail, m.detailTab = true, tabCIP
	m.cip = widget.CIPSnapshot{Name: "cip", Error: "cip /runs: 500 Internal Server Error"}
	view := m.cipView()
	if !strings.Contains(view, "500 Internal Server Error") {
		t.Errorf("cipView = %q, want the error text", view)
	}
	if !strings.Contains(view, "unavailable") {
		t.Errorf("cipView = %q, want it to say the widget is unavailable", view)
	}
}

func TestCIPViewShowsRunsCountsAndStorage(t *testing.T) {
	now := time.Date(2026, 8, 27, 21, 0, 0, 0, time.UTC)
	m := cipModel(nil, cipWidgetConfig)
	m.detail, m.detailTab = true, tabCIP
	m.cip = widget.CIPSnapshot{
		Name: "cip", At: now, Healthy: true, Running: 1, Failed: 1, Succeeded: 2,
		Runs: []widget.CIPRun{
			{ID: 40, Repo: "padel-bros-shadow", Status: "running", Branch: "main",
				SHA: "89e4f2074c1a5b6d", StartedAt: now.Add(-90 * time.Second)},
			{ID: 39, Repo: "padel-bros", Status: "failed", Branch: "dev",
				SHA: "1122334455667788", Finished: true,
				StartedAt: now.Add(-10 * time.Minute), FinishedAt: now.Add(-8 * time.Minute)},
		},
		Repos:          []widget.CIPRepoStorage{{Name: "padel-bros", RepoBytes: 182452224}},
		CIPBytes:       776925184,
		DiskFreeBytes:  44778520576,
		DiskTotalBytes: 104996663296,
	}
	view := m.cipView()
	for _, want := range []string{
		"padel-bros-shadow", "running", "failed", "main", "89e4f20", "dev",
		"cip total", "filesystem", "GiB",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("cipView is missing %q", want)
		}
	}
	if strings.Contains(view, "0001") {
		t.Errorf("cipView = %q, must not render the zero time", view)
	}
	if strings.Contains(view, "89e4f2074c1a5b6d") {
		t.Errorf("cipView shows the full SHA, want the short SHA")
	}
}

// An idle daemon is not a fault. The panel must say so rather than look
// broken or empty.
func TestCIPViewSaysWhenNoRunExists(t *testing.T) {
	m := cipModel(nil, cipWidgetConfig)
	m.cip = widget.CIPSnapshot{Name: "cip", Healthy: true, DiskFreeBytes: 50, DiskTotalBytes: 100}
	view := m.cipView()
	if !strings.Contains(view, "No pipeline run") {
		t.Errorf("cipView = %q, want it to say that no run exists", view)
	}
	if !strings.Contains(view, "filesystem") {
		t.Errorf("cipView = %q, want the storage rows even with no run", view)
	}
}

// A degraded snapshot must read as degraded, because a nearly full disk is
// the fault that stops the next run.
func TestCIPViewShowsTheDegradedState(t *testing.T) {
	m := cipModel(nil, cipWidgetConfig)
	m.cip = widget.CIPSnapshot{Name: "cip", Healthy: false, DiskFreeBytes: 3, DiskTotalBytes: 100}
	if !strings.Contains(m.cipView(), "DEGRADED") {
		t.Errorf("cipView = %q, want DEGRADED", m.cipView())
	}
}

// The tab refreshes quickly while the reader looks at it, and slowly when
// the reader looks somewhere else.
func TestCIPRefreshIsSlowerWhenTheTabIsNotFocused(t *testing.T) {
	if focused, idle := cipRefresh(true), cipRefresh(false); focused >= idle {
		t.Errorf("focused refresh %v, idle refresh %v, want the focused one to be shorter", focused, idle)
	}
}

func TestCIPTabFocusedOnlyOnTheCIPTab(t *testing.T) {
	m := cipModel(nil, cipWidgetConfig)
	m.detail, m.detailTab = true, tabCIP
	if !m.cipTabFocused() {
		t.Error("cipTabFocused = false on the CIP tab")
	}
	m.detailTab = 0
	if m.cipTabFocused() {
		t.Error("cipTabFocused = true on the CPU tab")
	}
	m.detail, m.detailTab = false, tabCIP
	if m.cipTabFocused() {
		t.Error("cipTabFocused = true in the overview")
	}
}

// The widget must never reach the network without a token, and a token
// failure must land in the snapshot as a visible error.
func TestFetchCIPCommandReportsAMissingTokenFile(t *testing.T) {
	m := cipModel(nil, []config.Widget{{Name: "cip", Type: "cip",
		Endpoint: "http://10.0.0.1:8080", TokenFile: "/no/such/token/file"}})
	cmd := m.fetchCIP()
	if cmd == nil {
		t.Fatal("fetchCIP returned no command")
	}
	msg, ok := cmd().(cipMsg)
	if !ok {
		t.Fatalf("fetchCIP returned %T, want cipMsg", cmd())
	}
	if widget.CIPSnapshot(msg).Error == "" {
		t.Error("a missing token file produced no error in the snapshot")
	}
}

func TestFetchCIPIsNilWithoutAWidget(t *testing.T) {
	if cmd := cipModel(nil, nil).fetchCIP(); cmd != nil {
		t.Error("fetchCIP returned a command with no cip widget configured")
	}
}

// A gate that nobody knows how to approve is a gate nobody approves.
// The pane names the key while a stage waits.
func TestCIPActionPaneOffersTheApproveKeyOnAGatedStage(t *testing.T) {
	m := cipPromoModel()
	m.cipSel = 0

	pane := m.cipActionPane(80)

	if !strings.Contains(pane, "release") {
		t.Errorf("the pane does not name the waiting stage:\n%s", pane)
	}
	if !strings.Contains(pane, "approve") {
		t.Errorf("the pane does not offer the approve key:\n%s", pane)
	}
}

// With nothing gated the pane must not invite an approval.
func TestCIPActionPaneStaysQuietWithoutAGate(t *testing.T) {
	m := cipPromoModel()
	for i := range m.cipPromotions.Promotions {
		for j := range m.cipPromotions.Promotions[i].Stages {
			if m.cipPromotions.Promotions[i].Stages[j].State == "gated" {
				m.cipPromotions.Promotions[i].Stages[j].State = "running"
			}
		}
	}
	m.cipSel = 0

	if strings.Contains(m.cipActionPane(80), "approve") {
		t.Error("the pane invites an approval with no gate waiting")
	}
}
