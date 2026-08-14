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

func TestContextAddKeepsCurrentFieldVisibleOnShortTerminal(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenContextAdd
	m.height = 10
	m.addStep = 1
	m.addFields = fieldsByAuthType["sso"]
	m.addFieldIdx = len(m.addFields) - 1
	m.addInput = "AdministratorAccess"
	m.addValues = map[string]string{"auth_type": "sso"}
	for i := 0; i < m.addFieldIdx; i++ {
		m.addValues[m.addFields[i].key] = "configured"
	}

	view := m.viewContextAdd()
	if !strings.Contains(view, "SSO Role Name") || !strings.Contains(view, "AdministratorAccess") {
		t.Fatalf("expected focused field to remain visible, got %q", view)
	}
	if !strings.Contains(view, "earlier fields") {
		t.Fatalf("expected windowing indicator on short terminal, got %q", view)
	}
}

func TestContextAddDoesNotOfferOktaSAMLYet(t *testing.T) {
	for _, authType := range authTypes {
		if authType == "okta_saml" {
			t.Fatal("okta_saml must stay out of the add-context picker until runtime credential exchange lands")
		}
	}
	fields, ok := fieldsByAuthType["okta_saml"]
	if !ok || len(fields) == 0 {
		t.Fatal("okta_saml field definitions should stay ready for the runtime slice")
	}
	for _, field := range fields {
		if strings.Contains(field.key, "password") || strings.Contains(field.key, "secret") || strings.Contains(field.key, "mfa") {
			t.Fatalf("okta_saml wizard must not collect secrets, got field %q", field.key)
		}
	}
}
