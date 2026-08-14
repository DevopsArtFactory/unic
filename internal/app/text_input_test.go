package app

import (
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func TestTrimLastRuneIsRuneSafe(t *testing.T) {
	if got := trimLastRune("한글"); got != "한" {
		t.Fatalf("expected one rune removed, got %q", got)
	}
	if got := trimLastRune("a"); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
	if got := trimLastRune(""); got != "" {
		t.Fatalf("expected empty passthrough, got %q", got)
	}
}

func TestAppendKeyRunes(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'글'}}
	if got := appendKeyRunes("한", msg); got != "한글" {
		t.Fatalf("expected appended rune, got %q", got)
	}
	if got := appendKeyRunes("x", tea.KeyMsg{Type: tea.KeyEnter}); got != "x" {
		t.Fatalf("expected non-rune keys ignored, got %q", got)
	}
}

func TestSharedFilterBackspaceIsRuneSafe(t *testing.T) {
	next, _, changed := handleFilterKey("backspace", "한글")
	if !changed || next != "한" || !utf8.ValidString(next) {
		t.Fatalf("expected rune-safe filter backspace, got %q changed=%v", next, changed)
	}
}

func TestRDSConfirmInputSurvivesMultibyteBackspace(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	m.screen = screenRDSConfirm
	m.rds.action = "stop"
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "한글-db"}

	for _, r := range "한글" {
		m.rds.updateConfirm(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.rds.updateConfirm(&m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.rds.confirmInput != "한" || !utf8.ValidString(m.rds.confirmInput) {
		t.Fatalf("expected rune-safe confirm input, got %q", m.rds.confirmInput)
	}
}
