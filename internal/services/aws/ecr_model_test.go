package aws

import (
	"strings"
	"testing"
	"time"
)

// Regression test for #195. The ECR image list rendered tag/digest/etc.
// as space-padded but variable-width columns, so the digest column
// started at a different position for each tag length and the list was
// hard to scan. After the fix the digest substring lands at the same
// position in every rendered row, regardless of tag length.
func TestECRImageDisplayTitleColumnsAlign(t *testing.T) {
	now := time.Now()
	mk := func(tag string) ECRImage {
		return ECRImage{
			Tags:      []string{tag},
			Digest:    "sha256:abcdef0123456789",
			PushedAt:  now,
			SizeBytes: 1234567,
		}
	}

	short := mk("v1").DisplayTitle()
	medium := mk("v1.2.3-rc.1").DisplayTitle()
	long := mk("very-long-feature-branch-tag-that-overflows-the-column").DisplayTitle()

	// shortDigest output for our digest. Reused as a column-position probe.
	const digest = "sha256:abcdef0123"

	// Compare rune positions, not bytes — the truncated row contains a
	// multi-byte ellipsis ('…' is 3 bytes / 1 rune) so byte indexes
	// disagree even when the rendered column lines up correctly in a
	// terminal. Rune count matches what the user actually sees.
	runePos := func(s, needle string) int {
		idx := strings.Index(s, needle)
		if idx < 0 {
			return -1
		}
		return len([]rune(s[:idx]))
	}
	pShort := runePos(short, digest)
	pMedium := runePos(medium, digest)
	pLong := runePos(long, digest)

	if pShort < 0 || pMedium < 0 || pLong < 0 {
		t.Fatalf("digest substring missing from rendered row:\n  short=%q\n  medium=%q\n  long=%q",
			short, medium, long)
	}
	if pShort != pMedium || pMedium != pLong {
		t.Fatalf("digest column not aligned (short=%d medium=%d long=%d):\n  %s\n  %s\n  %s",
			pShort, pMedium, pLong, short, medium, long)
	}

	// Overflow must truncate via ellipsis, not push later columns right.
	if !strings.Contains(long, "…") {
		t.Fatalf("long tag should truncate with ellipsis, got %q", long)
	}
}

// fitColumn is the helper that does the truncate-or-pad work; pin its
// contract directly so any future width tweak surfaces here.
func TestFitColumn(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"shorter pads with spaces", "abc", 6, "abc   "},
		{"exact length unchanged", "abcdef", 6, "abcdef"},
		{"longer truncates with ellipsis", "abcdefghij", 6, "abcde…"},
		{"unicode tag still respects rune width", "v1.2-α", 6, "v1.2-α"},
		{"width 0 returns input", "abc", 0, "abc"},
		{"width 1 with overflow keeps single rune", "abcde", 1, "a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fitColumn(tc.in, tc.width); got != tc.want {
				t.Fatalf("fitColumn(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
			}
		})
	}
}
