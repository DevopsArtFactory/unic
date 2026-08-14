package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
)

func TestCommandLifecycleRenewCancelsPreviousGeneration(t *testing.T) {
	lifecycle := newCommandLifecycle()

	first := lifecycle.Current()
	if first.Err() != nil {
		t.Fatal("expected a live initial context")
	}
	second := lifecycle.Renew()
	if first.Err() == nil {
		t.Fatal("expected renewal to cancel the previous generation")
	}
	if second.Err() != nil {
		t.Fatal("expected the renewed context to be live")
	}
	if _, ok := second.Deadline(); !ok {
		t.Fatal("expected the command context to carry a deadline")
	}
}

func TestCommandLifecycleCurrentRevivesAfterCancel(t *testing.T) {
	lifecycle := newCommandLifecycle()
	lifecycle.Current()
	lifecycle.CancelAll()

	revived := lifecycle.Current()
	if revived.Err() != nil {
		t.Fatal("expected Current to revive a fresh context after cancellation")
	}
}

func TestStartLoadingRenewsAndHomeCancelsInFlightWork(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	m.screen = screenRDSList

	before := m.commands.Current()
	next, _ := m.startLoading(func() tea.Msg { return nil })
	m = next.(Model)
	if before.Err() == nil {
		t.Fatal("expected startLoading to supersede the previous generation")
	}

	inFlight := m.commands.Current()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	m = updated.(Model)
	if m.screen != screenServiceList {
		t.Fatalf("expected home navigation, got %v", m.screen)
	}
	if inFlight.Err() == nil {
		t.Fatal("expected abandoning the screen with H to cancel in-flight work")
	}
}
