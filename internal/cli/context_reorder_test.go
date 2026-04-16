package cli

import (
	"testing"

	"unic/internal/config"
)

func TestMoveContextSwapsWithPrevious(t *testing.T) {
	contexts := []config.ContextInfo{
		{Name: "one"},
		{Name: "two"},
		{Name: "three"},
	}

	updated, cursor := moveContext(contexts, 1, -1)
	if cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", cursor)
	}
	if updated[0].Name != "two" || updated[1].Name != "one" {
		t.Fatalf("unexpected order after move up: %#v", updated)
	}
}

func TestMoveContextStaysWithinBounds(t *testing.T) {
	contexts := []config.ContextInfo{
		{Name: "one"},
		{Name: "two"},
	}

	updated, cursor := moveContext(contexts, 0, -1)
	if cursor != 0 {
		t.Fatalf("expected cursor to stay at 0, got %d", cursor)
	}
	if updated[0].Name != "one" || updated[1].Name != "two" {
		t.Fatalf("unexpected order when moving out of bounds: %#v", updated)
	}
}

func TestContextNamesReturnsCurrentOrder(t *testing.T) {
	contexts := []config.ContextInfo{
		{Name: "alpha"},
		{Name: "beta"},
		{Name: "gamma"},
	}

	names := contextNames(contexts)
	if len(names) != 3 || names[0] != "alpha" || names[1] != "beta" || names[2] != "gamma" {
		t.Fatalf("unexpected names: %#v", names)
	}
}

func TestMoveReorderCursorClampsWithinBounds(t *testing.T) {
	if got := moveReorderCursor(3, 0, -1); got != 0 {
		t.Fatalf("expected top clamp to 0, got %d", got)
	}
	if got := moveReorderCursor(3, 2, 1); got != 2 {
		t.Fatalf("expected bottom clamp to 2, got %d", got)
	}
	if got := moveReorderCursor(3, 1, 1); got != 2 {
		t.Fatalf("expected move to 2, got %d", got)
	}
}
