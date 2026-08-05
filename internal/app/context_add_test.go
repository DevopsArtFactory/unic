package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestContextAddShowsConsoleLoginAuthType(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenContextAdd
	m.addStep = 0

	view := m.viewContextAdd()
	if !strings.Contains(view, "console_login") {
		t.Fatalf("expected console_login in auth type selection, got %q", view)
	}
}

func TestContextAddSelectsConsoleLoginFields(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenContextAdd
	m.addStep = 0
	m.addValues = map[string]string{}

	for i, authType := range authTypes {
		if authType == "console_login" {
			m.addAuthIdx = i
			break
		}
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.addValues["auth_type"] != "console_login" {
		t.Fatalf("expected console_login selection, got %q", model.addValues["auth_type"])
	}
	if len(model.addFields) != 5 {
		t.Fatalf("expected 5 fields for console_login, got %d", len(model.addFields))
	}
	if model.addFields[3].key != "regions" || model.addFields[4].key != "profile" {
		t.Fatalf("expected regions and profile fields, got %+v", model.addFields)
	}
}
