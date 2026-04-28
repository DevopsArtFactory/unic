package aws

import (
	"fmt"
	"strings"
	"time"
)

const ECRImageStaleAgeDays = 90

// ECRRepository holds repository-level settings useful during operations review.
type ECRRepository struct {
	Name          string
	URI           string
	RegistryID    string
	ARN           string
	ScanOnPush    bool
	TagMutability string
	Encryption    string
}

// DisplayTitle returns a compact list label for an ECR repository.
func (r ECRRepository) DisplayTitle() string {
	return fmt.Sprintf("%s [%s] scan:%s enc:%s",
		r.Name, r.TagMutability, yesNo(r.ScanOnPush), r.Encryption)
}

// FilterText returns a lowercase string for keyword matching.
func (r ECRRepository) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s",
		r.Name, r.URI, r.RegistryID, r.ARN, r.TagMutability, r.Encryption))
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

// ECRImage holds image-level metadata for an ECR repository.
type ECRImage struct {
	RepositoryName string
	Digest         string
	Tags           []string
	PushedAt       time.Time
	SizeBytes      int64
}

// Column widths for the ECR image list. Tuned so a typical row
// (`v1.2.3`, sha256-19, `2026-04-28 12:34`, ` 12.3 MB`, ` [stale]`) fits
// comfortably while a long tag truncates with an ellipsis instead of
// pushing the later columns out of alignment (#195).
const (
	ecrTagColWidth    = 30
	ecrDigestColWidth = 19 // shortDigest already pads/truncates to <= 19
	ecrPushedColWidth = 16 // "2006-01-02 15:04"
	ecrSizeColWidth   = 9
)

func (i ECRImage) DisplayTitle() string {
	pushed := "-"
	if !i.PushedAt.IsZero() {
		pushed = i.PushedAt.Format("2006-01-02 15:04")
	}
	status := ""
	if i.IsUntagged() {
		status = " [untagged]"
	} else if i.IsStale(time.Now()) {
		status = " [stale]"
	}
	return fmt.Sprintf("%s  %-*s  %-*s  %*s%s",
		fitColumn(i.PrimaryLabel(), ecrTagColWidth),
		ecrDigestColWidth, shortDigest(i.Digest),
		ecrPushedColWidth, pushed,
		ecrSizeColWidth, FormatBytes(i.SizeBytes),
		status,
	)
}

// fitColumn left-pads `s` to exactly `width` runes so subsequent columns
// align in the rendered list. Long values are truncated with a single
// trailing ellipsis. Width is measured in runes (not bytes) so multi-byte
// tag characters don't break the layout.
func fitColumn(s string, width int) string {
	if width <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) == width {
		return s
	}
	if len(runes) < width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func (i ECRImage) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s", i.RepositoryName, i.Digest, strings.Join(i.Tags, " ")))
}

func (i ECRImage) TagsText() string {
	if len(i.Tags) == 0 {
		return "(untagged)"
	}
	return strings.Join(i.Tags, ", ")
}

func (i ECRImage) PrimaryLabel() string {
	if len(i.Tags) > 0 {
		return i.Tags[0]
	}
	return "(untagged)"
}

func (i ECRImage) IsUntagged() bool {
	return len(i.Tags) == 0
}

func (i ECRImage) IsStale(now time.Time) bool {
	if i.IsUntagged() {
		return true
	}
	if i.PushedAt.IsZero() {
		return false
	}
	return now.Sub(i.PushedAt) >= ECRImageStaleAgeDays*24*time.Hour
}

func (i ECRImage) CleanupSignal(now time.Time) string {
	if i.IsUntagged() {
		return "untagged"
	}
	if i.IsStale(now) {
		return fmt.Sprintf("older than %d days", ECRImageStaleAgeDays)
	}
	return "current"
}

func (i ECRImage) CopyTagValue() string {
	if len(i.Tags) == 0 {
		return ""
	}
	return i.Tags[0]
}

func shortDigest(digest string) string {
	if digest == "" {
		return "-"
	}
	if len(digest) <= 19 {
		return digest
	}
	if strings.HasPrefix(digest, "sha256:") && len(digest) > 19 {
		return digest[:19]
	}
	return digest[:16]
}
