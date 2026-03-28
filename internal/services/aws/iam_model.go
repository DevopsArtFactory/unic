package aws

import (
	"fmt"
	"strings"
	"time"
)

// AccessKey holds information about an IAM access key.
type AccessKey struct {
	AccessKeyID string
	Status      string
	CreateDate  time.Time
	LastUsed    time.Time
	ServiceName string
}

// NewAccessKey holds credentials for a newly created access key.
// The secret is only available at creation time.
type NewAccessKey struct {
	AccessKeyID     string
	SecretAccessKey string
}

// Age returns the number of days since the key was created.
func (k AccessKey) Age() int {
	return int(time.Since(k.CreateDate).Hours() / 24)
}

// IsAged returns true if the key is older than 90 days.
func (k AccessKey) IsAged() bool {
	return k.Age() > 90
}

// DisplayTitle returns a formatted string for list display.
func (k AccessKey) DisplayTitle() string {
	age := k.Age()
	ageStr := fmt.Sprintf("%dd", age)
	if age > 90 {
		ageStr = fmt.Sprintf("%dd ⚠", age)
	}
	lastUsed := "never"
	if !k.LastUsed.IsZero() {
		lastUsed = k.LastUsed.Format(time.DateOnly)
	}
	return fmt.Sprintf("%s  [%s]  age:%s  last:%s", k.AccessKeyID, k.Status, ageStr, lastUsed)
}

// FilterText returns a lowercase string for keyword matching.
func (k AccessKey) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s", k.AccessKeyID, k.Status, k.ServiceName))
}
