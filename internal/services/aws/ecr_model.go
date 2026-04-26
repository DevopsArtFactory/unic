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
	return fmt.Sprintf("%s  %s  %s  %s%s", i.PrimaryLabel(), shortDigest(i.Digest), pushed, FormatBytes(i.SizeBytes), status)
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
