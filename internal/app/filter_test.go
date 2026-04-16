package app

import (
	"strings"
	"testing"
)

type testFilterItem struct {
	text string
}

func (i testFilterItem) FilterText() string {
	return i.text
}

func TestApplyFilterPrefersStrongerFuzzyMatches(t *testing.T) {
	items := []testFilterItem{
		{text: "my-prod-db"},
		{text: "prod-db"},
		{text: "private-db"},
	}

	filtered := applyFilter(items, "pdb")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 strong fuzzy matches, got %d", len(filtered))
	}
	if filtered[0].text != "prod-db" {
		t.Fatalf("expected strongest match first, got %q", filtered[0].text)
	}
}

func TestRenderHighlightedMatchAddsAnsiWithoutChangingVisibleText(t *testing.T) {
	highlighted := renderHighlightedMatch("prod-db", "pdb")
	if highlighted == "prod-db" {
		t.Fatal("expected highlighted output to include styling")
	}
	if got := stripANSI(highlighted); got != "prod-db" {
		t.Fatalf("expected visible text to stay unchanged, got %q", got)
	}
	if !strings.Contains(highlighted, "\x1b[") {
		t.Fatalf("expected ANSI styling in highlighted output, got %q", highlighted)
	}
}
