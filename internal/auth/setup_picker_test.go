package auth

import (
	"strings"
	"testing"
)

func TestPickerStateFiltersAsYouType(t *testing.T) {
	items := []string{"dev", "prod-admin", "staging"}
	matches := func(item, query string) bool {
		return containsFold(item, query)
	}

	state := newPickerState(len(items))
	appendPickerRune(&state, items, matches, 'p')
	if len(state.filtered) != 1 || state.filtered[0] != 1 {
		t.Fatalf("expected only prod-admin after first rune, got %#v", state.filtered)
	}

	appendPickerRune(&state, items, matches, 'r')
	if len(state.filtered) != 1 || state.filtered[0] != 1 {
		t.Fatalf("expected prod-admin to stay selected after second rune, got %#v", state.filtered)
	}
}

func TestPickerStateBackspaceRestoresMatches(t *testing.T) {
	items := []string{"dev", "prod-admin", "prod-readonly"}
	matches := func(item, query string) bool {
		return containsFold(item, query)
	}

	state := newPickerState(len(items))
	appendPickerRune(&state, items, matches, 'p')
	appendPickerRune(&state, items, matches, 'r')
	appendPickerRune(&state, items, matches, 'o')
	if len(state.filtered) != 2 {
		t.Fatalf("expected 2 filtered matches, got %d", len(state.filtered))
	}

	deleteLastPickerRune(&state, items, matches)
	if state.query != "pr" {
		t.Fatalf("expected query to shrink to pr, got %q", state.query)
	}
	if len(state.filtered) != 2 {
		t.Fatalf("expected filtered matches after backspace, got %d", len(state.filtered))
	}
}

func TestPickerStateClearResetsFilter(t *testing.T) {
	items := []string{"dev", "prod", "staging"}
	matches := func(item, query string) bool {
		return containsFold(item, query)
	}

	state := newPickerState(len(items))
	appendPickerRune(&state, items, matches, 's')
	if len(state.filtered) != 1 {
		t.Fatalf("expected 1 filtered item, got %d", len(state.filtered))
	}

	clearPickerQuery(&state, items, matches)
	if state.query != "" {
		t.Fatalf("expected query to clear, got %q", state.query)
	}
	if len(state.filtered) != len(items) {
		t.Fatalf("expected all items after clear, got %d", len(state.filtered))
	}
}

func TestRenderInteractivePickerAlignsNumberedRows(t *testing.T) {
	items := []string{"dev", "prod", "staging"}
	state := pickerState{filtered: []int{0, 1, 2}, cursor: 1}

	var out strings.Builder
	renderInteractivePicker(&out, "contexts", items, func(item string) string { return item }, state)

	rendered := out.String()
	if !strings.Contains(rendered, "  1) dev") {
		t.Fatalf("expected first row to be numbered and aligned, got %q", rendered)
	}
	if !strings.Contains(rendered, "> 2) prod") {
		t.Fatalf("expected selected row marker and numbering, got %q", rendered)
	}
	if !strings.Contains(rendered, "  3) staging") {
		t.Fatalf("expected third row to be numbered and aligned, got %q", rendered)
	}
}

func TestPickerWindowStartsAtTopForInitialCursor(t *testing.T) {
	start, end := pickerWindow(40, 0, 10)
	if start != 0 || end != 10 {
		t.Fatalf("expected top window 0-10, got %d-%d", start, end)
	}
}

func TestPickerWindowCentersAroundCursorWhenPossible(t *testing.T) {
	start, end := pickerWindow(40, 20, 10)
	if start != 15 || end != 25 {
		t.Fatalf("expected centered window 15-25, got %d-%d", start, end)
	}
}
