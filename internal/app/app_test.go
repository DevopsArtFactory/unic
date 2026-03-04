package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelNotQuitting(t *testing.T) {
	m := New()
	if m.quitting {
		t.Error("new model should not be quitting")
	}
}

func TestQuitOnQ(t *testing.T) {
	m := New()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model := updated.(Model)
	if !model.quitting {
		t.Error("model should be quitting after 'q'")
	}
	if cmd == nil {
		t.Error("expected a quit command")
	}
}

func TestViewNotEmpty(t *testing.T) {
	m := New()
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
