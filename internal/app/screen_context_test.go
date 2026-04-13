package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
)

func testContexts() []config.ContextInfo {
	return []config.ContextInfo{
		{Name: "dev", Region: "us-east-1", AuthType: "credential"},
		{Name: "prod", Region: "us-west-2", AuthType: "sso", Current: true},
		{Name: "staging", Region: "ap-northeast-2", AuthType: "assume_role"},
	}
}

func TestContextsLoadedSyncsContextTable(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	model := updated.(Model)

	if model.screen != screenContextPicker {
		t.Fatalf("expected context picker screen, got %v", model.screen)
	}
	if got := len(model.contextTable.Rows()); got != 3 {
		t.Fatalf("expected 3 context rows, got %d", got)
	}
	if model.ctxIdx != 1 {
		t.Fatalf("expected current context cursor at index 1, got %d", model.ctxIdx)
	}
	if model.contextTable.Cursor() != 1 {
		t.Fatalf("expected table cursor at index 1, got %d", model.contextTable.Cursor())
	}
	selected := model.contextTable.SelectedRow()
	if len(selected) != 4 || selected[0] != "prod" || selected[3] != "*" {
		t.Fatalf("expected selected row for current context, got %#v", selected)
	}

	view := model.viewContextPicker()
	for _, want := range []string{"NAME", "AUTH TYPE", "CURRENT", "prod"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected context picker view to contain %q, got %q", want, view)
		}
	}
}

func TestContextPickerNavigationUsesTableModel(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	model := updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)

	if model.ctxIdx != 2 {
		t.Fatalf("expected cursor to move to index 2, got %d", model.ctxIdx)
	}
	if model.contextTable.Cursor() != 2 {
		t.Fatalf("expected table cursor to move to index 2, got %d", model.contextTable.Cursor())
	}
}

func TestContextPickerFilterUpdatesTableRows(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	model := updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updated.(Model)
	for _, ch := range []rune("prod") {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(Model)
	}

	if got := len(model.filteredCtxList); got != 1 {
		t.Fatalf("expected 1 filtered context, got %d", got)
	}
	if got := len(model.contextTable.Rows()); got != 1 {
		t.Fatalf("expected table to show 1 filtered row, got %d", got)
	}
	if selected := model.contextTable.SelectedRow(); len(selected) == 0 || selected[0] != "prod" {
		t.Fatalf("expected filtered table row for prod, got %#v", selected)
	}
}

func TestContextPickerEnterUsesSelectedTableRow(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	model := updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if cmd == nil {
		t.Fatal("expected context switch command on enter")
	}
	if model.pendingContextName != "staging" {
		t.Fatalf("expected pending context staging, got %q", model.pendingContextName)
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen after selecting context, got %v", model.screen)
	}
}
