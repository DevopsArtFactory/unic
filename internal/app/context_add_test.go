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
	if len(model.addFields) != 8 {
		t.Fatalf("expected 8 fields for console_login (incl. optional chaining), got %d", len(model.addFields))
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

func TestContextAddSelectsOktaSAMLFields(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenContextAdd
	m.addStep = 0
	m.addValues = map[string]string{}

	for i, authType := range authTypes {
		if authType == "okta_saml" {
			m.addAuthIdx = i
			break
		}
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.addValues["auth_type"] != "okta_saml" {
		t.Fatalf("expected okta_saml selection, got %q", model.addValues["auth_type"])
	}

	keys := make([]string, 0, len(model.addFields))
	required := map[string]bool{}
	for _, field := range model.addFields {
		keys = append(keys, field.key)
		required[field.key] = field.required
	}
	joined := strings.Join(keys, ",")
	for _, want := range []string{"okta_org_url", "okta_app_id", "role_arn"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %s field for okta_saml, got %v", want, keys)
		}
	}
	if !required["okta_org_url"] || !required["okta_app_id"] {
		t.Fatal("expected okta org URL and app ID to be required")
	}
	if required["role_arn"] {
		t.Fatal("expected preferred role ARN to be optional")
	}
	for _, key := range keys {
		if strings.Contains(key, "password") || strings.Contains(key, "secret") || strings.Contains(key, "mfa") {
			t.Fatalf("okta_saml wizard must not collect secrets, got field %q", key)
		}
	}
}

func TestContextAddOffersChainingFieldsForProfileBackedTypes(t *testing.T) {
	for _, authType := range []string{"credential", "console_login"} {
		keys := map[string]bool{}
		required := map[string]bool{}
		for _, field := range fieldsByAuthType[authType] {
			keys[field.key] = true
			required[field.key] = field.required
		}
		for _, want := range []string{"role_arn", "external_id", "mfa_serial"} {
			if !keys[want] {
				t.Fatalf("%s: expected optional %s field for role chaining", authType, want)
			}
			if required[want] {
				t.Fatalf("%s: expected %s to be optional", authType, want)
			}
		}
	}
	keys := map[string]bool{}
	for _, field := range fieldsByAuthType["assume_role"] {
		keys[field.key] = true
	}
	if !keys["mfa_serial"] {
		t.Fatal("assume_role: expected mfa_serial field")
	}
}
