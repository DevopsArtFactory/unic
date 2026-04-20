package aws

import (
	"fmt"
	"strings"
	"time"
)

const BedrockServiceSpecificCredentialName = "bedrock.amazonaws.com"

// BedrockAPIKey holds non-secret metadata for a long-term Bedrock API key.
type BedrockAPIKey struct {
	CredentialID string
	UserName     string
	ServiceName  string
	Alias        string
	Status       string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

// GeneratedBedrockAPIKey includes the one-time secret value returned by IAM.
type GeneratedBedrockAPIKey struct {
	BedrockAPIKey
	Secret string
}

// DisplayTitle returns a formatted string for list display.
func (k BedrockAPIKey) DisplayTitle() string {
	alias := k.Alias
	if alias == "" {
		alias = "-"
	}
	return fmt.Sprintf("%s  user:%s  [%s]  expires:%s",
		alias, k.UserName, k.Status, k.ExpiresDisplay())
}

// FilterText returns a lowercase string for keyword matching.
func (k BedrockAPIKey) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %s",
		k.CredentialID, k.UserName, k.ServiceName, k.Alias, k.Status))
}

// ExpiresDisplay returns the expiration date or "never".
func (k BedrockAPIKey) ExpiresDisplay() string {
	if k.ExpiresAt.IsZero() {
		return "never"
	}
	return k.ExpiresAt.Format(time.DateOnly)
}

// CreatedDisplay returns the creation date or "-".
func (k BedrockAPIKey) CreatedDisplay() string {
	if k.CreatedAt.IsZero() {
		return "-"
	}
	return k.CreatedAt.Format(time.DateOnly)
}

// EnvExport returns shell env output for a generated Bedrock API key.
func (k GeneratedBedrockAPIKey) EnvExport() string {
	return fmt.Sprintf("export AWS_BEARER_TOKEN_BEDROCK=%s", k.Secret)
}
