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
	m := New(testConfig(), "")
	if m.quitting {
		t.Error("new model should not be quitting")
	}
}

func TestNewModelStartsOnContextPicker(t *testing.T) {
	m := New(testConfig(), "")
	if m.screen != screenContextPicker {
		t.Error("new model should start on context picker screen")
	}
}

func TestQuitOnCtrlC(t *testing.T) {
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
	m.screen = screenServiceList
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
	m.screen = screenServiceList
	// Press down — should move to index 1 (now 2 services: EC2, VPC)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.svcIdx != 1 {
		t.Errorf("expected svcIdx 1 after pressing j, got %d", model.svcIdx)
	}
}

func TestServiceListEnterGoesToFeatures(t *testing.T) {
	m := New(testConfig(), "")
	m.screen = screenServiceList
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenFeatureList {
		t.Errorf("expected feature list screen, got %d", model.screen)
	}
}

func TestFeatureListEscGoesBack(t *testing.T) {
	m := New(testConfig(), "")
	m.screen = screenFeatureList
	m.features = m.services[0].Features
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenServiceList {
		t.Errorf("expected service list screen, got %d", model.screen)
	}
}
