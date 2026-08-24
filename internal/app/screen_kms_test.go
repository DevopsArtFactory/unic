package app

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	awsservice "unic/internal/services/aws"
)

func TestKMSKeysLoadedRendersPostureAndDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.kms.HandleMessage(&m, kmsKeysLoadedMsg{keys: []awsservice.KMSKey{
		{ID: "key-1", Aliases: []string{"alias/app"}, State: "Enabled", Manager: "CUSTOMER", RotationEligible: true, RotationKnown: true, RotationEnabled: true},
		{ID: "key-2", Aliases: []string{"alias/asymmetric"}, State: "Enabled", Manager: "CUSTOMER"},
	}})
	view, ok := m.kms.View(m)
	if !ok || !strings.Contains(view, "alias/app") || !strings.Contains(view, "true") || !strings.Contains(view, "alias/asymmetric") || !strings.Contains(view, "n/a") {
		t.Fatalf("unexpected list: %s", view)
	}
	_, _, handled := m.kms.HandleKey(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if !handled || m.screen != screenKMSKeyDetail || m.kms.selected == nil {
		t.Fatalf("expected enter to open key detail, got screen %v", m.screen)
	}
	view, _ = m.kms.View(m)
	for _, want := range []string{"key-1", "alias/app", "Rotation Enabled", "true"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail missing %q: %s", want, view)
		}
	}

	_, _, handled = m.kms.HandleKey(&m, tea.KeyMsg{Type: tea.KeyEsc})
	if !handled || m.screen != screenKMSKeyList || m.kms.selected != nil {
		t.Fatalf("expected esc to return to key list, got screen %v", m.screen)
	}
	_, _, handled = m.kms.HandleKey(&m, tea.KeyMsg{Type: tea.KeyDown})
	if !handled {
		t.Fatal("expected down to select the ineligible key")
	}
	_, _, handled = m.kms.HandleKey(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if !handled || m.screen != screenKMSKeyDetail {
		t.Fatalf("expected enter to reopen key detail, got screen %v", m.screen)
	}
	view, _ = m.kms.View(m)
	if !strings.Contains(view, "alias/asymmetric") || !strings.Contains(view, "Not eligible") {
		t.Fatalf("ineligible detail missing rotation posture: %s", view)
	}
	_, _, handled = m.kms.HandleKey(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !handled || m.screen != screenKMSKeyList || m.kms.selected != nil {
		t.Fatalf("expected q to return to key list, got screen %v", m.screen)
	}
}

func TestKMSPartialWarningKeepsKeyWithUnknownRotation(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.kms.HandleMessage(&m, kmsKeysLoadedMsg{
		keys:     []awsservice.KMSKey{{ID: "key-1", RotationEligible: true}},
		warnings: []error{errors.New("failed to get rotation status for KMS key key-1: access denied")},
	})

	view := stripANSI(m.kms.viewList(m))
	if m.screen != screenKMSKeyList || !strings.Contains(view, "key-1") || !strings.Contains(view, "unknown") || !strings.Contains(view, "access denied") {
		t.Fatalf("expected partial key and warning, got screen %v:\n%s", m.screen, view)
	}
}

func TestKMSKeysLoadedErrorOpensErrorScreen(t *testing.T) {
	m := New(testConfig(), "", "dev")
	updated, _, handled := m.kms.HandleMessage(&m, kmsKeysLoadedMsg{err: errors.New("access denied")})
	got := updated.(Model)
	if !handled || got.screen != screenError || !strings.Contains(got.errMsg, "access denied") {
		t.Fatalf("expected error screen with message, got %v %q", got.screen, got.errMsg)
	}
}

func TestKMSFilterWithNoMatchesRendersEmptyState(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.kms.HandleMessage(&m, kmsKeysLoadedMsg{keys: []awsservice.KMSKey{{ID: "key-1", Aliases: []string{"alias/app"}}}})
	m.storeFilterValue(filterKMSKeys, "missing")
	m.kms.ApplyFilter(&m, filterKMSKeys)

	view, ok := m.kms.View(m)
	if !ok || !strings.Contains(view, "No KMS keys found") {
		t.Fatalf("expected no-match empty state, got: %s", view)
	}
}

func TestKMSHelpScreenTitles(t *testing.T) {
	m := New(testConfig(), "", "dev")
	for screen, want := range map[screen]string{
		screenKMSKeyList:   "KMS Keys",
		screenKMSKeyDetail: "KMS Key Detail",
	} {
		m.screen = screen
		if got := m.helpScreenTitle(); got != want {
			t.Errorf("helpScreenTitle() = %q, want %q", got, want)
		}
	}
}
