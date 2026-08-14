package app

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
)

func TestCommandLifecycleRenewCancelsPreviousGeneration(t *testing.T) {
	lifecycle := newCommandLifecycle()

	first := lifecycle.Current()
	if first.Err() != nil {
		t.Fatal("expected a live initial context")
	}
	firstGen := lifecycle.CurrentGen()
	if gen := lifecycle.Renew(); gen != firstGen+1 {
		t.Fatalf("expected renewal to advance the generation, got %d", gen)
	}
	if first.Err() == nil {
		t.Fatal("expected renewal to cancel the previous generation")
	}
	renewed := lifecycle.Current()
	if renewed.Err() != nil {
		t.Fatal("expected the renewed context to be live")
	}
	if _, ok := renewed.Deadline(); !ok {
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

func TestQueuedCommandCannotMigrateAcrossRenewal(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}

	executed := false
	next, boundCmd := m.startLoading(func() tea.Msg {
		executed = true
		return screenReadyMsg{}
	})
	m = next.(Model)

	// A second load supersedes the first before its goroutine ever ran.
	m.commands.Renew()

	msg := runBatchedUserCmd(t, boundCmd)
	if executed {
		t.Fatal("expected the superseded command body to be skipped entirely")
	}
	if msg != nil {
		t.Fatalf("expected no message from a superseded command, got %#v", msg)
	}
}

func TestQueuedCommandCannotSurviveCancelAll(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}

	executed := false
	next, boundCmd := m.startLoading(func() tea.Msg {
		executed = true
		return screenReadyMsg{}
	})
	m = next.(Model)

	// The user abandons the screen before the command ran.
	m.commands.CancelAll()

	if msg := runBatchedUserCmd(t, boundCmd); executed || msg != nil {
		t.Fatalf("expected abandonment to keep the queued command from running, executed=%v msg=%#v", executed, msg)
	}
}

func TestStaleGenerationMessageIsDroppedAtDelivery(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	m.screen = screenRDSList

	// A command from generation N completes, but the generation moves on
	// before its message is delivered.
	staleGen := m.commands.Renew()
	m.commands.Renew()

	next, _ := m.Update(genBoundMsg{gen: staleGen, msg: cwAlarmsLoadedMsg{}})
	model := next.(Model)
	if model.screen != screenRDSList {
		t.Fatalf("expected stale message to be dropped without touching the model, got screen %v", model.screen)
	}
}

// runBatchedUserCmd extracts and runs the user command from the tea.Batch that
// startLoading returns (the other entry is the spinner tick).
func runBatchedUserCmd(t *testing.T, batched tea.Cmd) tea.Msg {
	t.Helper()
	if batched == nil {
		t.Fatal("expected a command from startLoading")
	}
	msg := batched()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return msg
	}
	var result tea.Msg
	for _, cmd := range batch {
		if cmd == nil {
			continue
		}
		out := cmd()
		if _, isTick := out.(spinner.TickMsg); isTick {
			continue
		}
		result = out
	}
	return result
}
