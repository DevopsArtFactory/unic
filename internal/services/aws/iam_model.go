package aws

import (
	"fmt"
	"strings"
	"time"
)

// IAMUser holds list-view metadata for an IAM user.
type IAMUser struct {
	UserName         string
	UserID           string
	ARN              string
	Path             string
	CreateDate       time.Time
	PasswordLastUsed time.Time
	LastActivity     time.Time
	MFAEnabled       bool
	AccessKeyCount   int
}

// IAMUserDetail holds the full detail view for an IAM user.
type IAMUserDetail struct {
	IAMUser
	Groups           []string
	AttachedPolicies []string
	AccessKeys       []AccessKey
}

// IAMUserPage represents one paginated slice of IAM users.
type IAMUserPage struct {
	Users      []IAMUser
	NextMarker string
	HasMore    bool
}

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

// DisplayTitle returns a formatted string for list display.
func (u IAMUser) DisplayTitle() string {
	return fmt.Sprintf("%s  created:%s  last:%s  keys:%d  mfa:%t",
		u.UserName, formatIAMDate(u.CreateDate), u.LastActivityDisplay(), u.AccessKeyCount, u.MFAEnabled)
}

// FilterText returns a lowercase string for keyword matching.
func (u IAMUser) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s",
		u.UserName, u.UserID, u.ARN, u.Path))
}

// LastActivityDisplay returns the last activity date or "never".
func (u IAMUser) LastActivityDisplay() string {
	return formatIAMDateOrNever(u.LastActivity)
}

// PasswordLastUsedDisplay returns the console password usage date or "never".
func (u IAMUser) PasswordLastUsedDisplay() string {
	return formatIAMDateOrNever(u.PasswordLastUsed)
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

func formatIAMDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(time.DateOnly)
}

func formatIAMDateOrNever(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format(time.DateOnly)
}
