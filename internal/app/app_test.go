package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{Profile: "default", Region: "us-east-1"}
}

func TestNewModelNotQuitting(t *testing.T) {
	m := New(testConfig())
	if m.quitting {
		t.Error("new model should not be quitting")
	}
}

func TestNewModelStartsOnServiceList(t *testing.T) {
	m := New(testConfig())
	if m.screen != screenServiceList {
		t.Error("new model should start on service list screen")
	}
}

func TestQuitOnCtrlC(t *testing.T) {
	m := New(testConfig())
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := updated.(Model)
	if !model.quitting {
		t.Error("model should be quitting after ctrl+c")
	}
	if cmd == nil {
		t.Error("expected a quit command")
	}
}

func TestQuitOnQ(t *testing.T) {
	m := New(testConfig())
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model := updated.(Model)
	if !model.quitting {
		t.Error("model should be quitting after 'q' on service list")
	}
	if cmd == nil {
		t.Error("expected a quit command")
	}
}

func TestViewNotEmpty(t *testing.T) {
	m := New(testConfig())
	v := m.View()
	if v == "" {
		t.Error("view should not be empty when not quitting")
	}
}

func TestViewEmptyWhenQuitting(t *testing.T) {
	m := Model{quitting: true}
	v := m.View()
	if v != "" {
		t.Error("view should be empty when quitting")
	}
}

func TestServiceListNavigation(t *testing.T) {
	m := New(testConfig())
	// Press down — should not panic (only one service)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.svcIdx != 0 {
		t.Error("should stay at 0 with only one service")
	}
}

func TestServiceListEnterGoesToFeatures(t *testing.T) {
	m := New(testConfig())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenFeatureList {
		t.Errorf("expected feature list screen, got %d", model.screen)
	}
}

func TestFeatureListEscGoesBack(t *testing.T) {
	m := New(testConfig())
	m.screen = screenFeatureList
	m.features = m.services[0].Features
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenServiceList {
		t.Errorf("expected service list screen, got %d", model.screen)
	}
}
