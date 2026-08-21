package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/franciscosainzwilliams/server-term/internal/config"
)

// agentsModel builds two servers where only the first one runs the
// orchestrator daemon that the widget endpoint points at.
func agentsModel() Model {
	m := New(config.Config{
		Servers: []config.Server{
			{Name: "hetzner", Address: "10.0.0.1"},
			{Name: "office", Address: "10.0.0.2"},
		},
		Widgets: []config.Widget{{Name: "agents", Type: "orchestrator", Endpoint: "http://10.0.0.1:7844", TokenEnv: "T"}},
	})
	for i := range m.samples {
		m.samples[i].Online, m.samples[i].At = true, time.Now()
	}
	m.width, m.height = 100, 40
	return m
}

func TestAgentsTabShowsOnlyOnTheWidgetHost(t *testing.T) {
	m := agentsModel()
	m.detail = true
	if !strings.Contains(m.detailView(), "AGENTS") {
		t.Fatal("widget host does not show the AGENTS tab")
	}
	m.cursor = 1
	if strings.Contains(m.detailView(), "AGENTS") {
		t.Fatal("server without the orchestrator shows the AGENTS tab")
	}
}

func TestAgentsKeyIsIgnoredOffTheWidgetHost(t *testing.T) {
	m := agentsModel()
	m.detail, m.cursor = true, 1
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if model.(Model).detailTab == 10 {
		t.Fatal("o opened the AGENTS tab on a server without the orchestrator")
	}
}

func TestTabCycleSkipsAgentsOffTheWidgetHost(t *testing.T) {
	m := agentsModel()
	m.detail, m.cursor = true, 1
	for i := 0; i < 12; i++ {
		model, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = model.(Model)
		if m.detailTab == 10 {
			t.Fatal("tab cycle reached the AGENTS tab on a server without the orchestrator")
		}
	}
}

func TestDetailEntryLeavesAgentsTabOnAnotherServer(t *testing.T) {
	m := agentsModel()
	m.detailTab = 10
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = model.(Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := model.(Model).detailTab; got == 10 {
		t.Fatal("detail view kept the AGENTS tab on a server without the orchestrator")
	}
}
