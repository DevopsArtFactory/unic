package aws

import (
	"fmt"
	"strings"
	"time"
)

// Secret holds essential information about an AWS Secrets Manager secret.
type Secret struct {
	Name             string
	ARN              string
	Description      string
	KMSKeyID         string
	CreatedDate      time.Time
	LastChangedDate  time.Time
	LastRotatedDate  time.Time
	NextRotationDate time.Time
	RotationEnabled  bool
}

// SecretDetail holds the full detail of a secret including its key/value pairs.
type SecretDetail struct {
	Secret
	Values map[string]string // parsed key/value pairs from JSON secret string
	Raw    string            // raw secret string (for non-JSON secrets)
}

// DisplayTitle returns a formatted string for list display.
func (s Secret) DisplayTitle() string {
	if s.Description != "" {
		return fmt.Sprintf("%s — %s", s.Name, s.Description)
	}
	return s.Name
}

// FilterText returns a lowercase string for keyword matching.
func (s Secret) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s", s.Name, s.Description, s.ARN))
}
