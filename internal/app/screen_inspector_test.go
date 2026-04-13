package app

import (
	"testing"
	"unicode/utf8"
)

func TestInspectorShortenHandlesUnicodeSafely(t *testing.T) {
	tests := []struct {
		name  string
		value string
		width int
	}{
		{name: "cjk narrow", value: "보안점검", width: 2},
		{name: "emoji narrow", value: "🔒security", width: 3},
		{name: "cjk with ellipsis", value: "보안점검결과", width: 6},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := inspectorShorten(tc.value, tc.width)
			if !utf8.ValidString(got) {
				t.Fatalf("expected valid UTF-8, got %q", got)
			}
		})
	}
}

func TestInspectorShortenUsesEllipsisForWiderColumns(t *testing.T) {
	got := inspectorShorten("security-group-public-ssh", 10)
	if got != "securit..." {
		t.Fatalf("expected truncated value with ellipsis, got %q", got)
	}
}
