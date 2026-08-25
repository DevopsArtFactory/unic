package aws

import (
	"fmt"
	"strings"
)

// WAFWebACL contains the read-only posture fields rendered by the browser.
type WAFWebACL struct {
	Name              string
	ID                string
	ARN               string
	Description       string
	Scope             string
	Region            string
	DefaultAction     string
	Capacity          int64
	DetailKnown       bool
	Rules             []WAFRule
	LoggingKnown      bool
	LogDestinations   []string
	ResourcesComplete bool
	ResourceARNs      []string
}

// WAFRule is a compact, display-oriented rule description.
type WAFRule struct {
	Name            string
	Priority        int32
	Statement       string
	Action          string
	Managed         bool
	MetricName      string
	MetricsEnabled  bool
	SamplingEnabled bool
}

func (a WAFWebACL) FilterText() string {
	parts := []string{a.Name, a.ID, a.ARN, a.Description, a.Scope, a.Region, a.DefaultAction, a.SignalLabel()}
	parts = append(parts, a.LogDestinations...)
	parts = append(parts, a.ResourceARNs...)
	for _, rule := range a.Rules {
		parts = append(parts, rule.Name, rule.Statement, rule.Action, rule.MetricName)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func (a WAFWebACL) ManagedRuleCount() int {
	count := 0
	for _, rule := range a.Rules {
		if rule.Managed {
			count++
		}
	}
	return count
}

func (a WAFWebACL) LoggingLabel() string {
	if !a.LoggingKnown {
		return "unknown"
	}
	if len(a.LogDestinations) == 0 {
		return "off"
	}
	return "on"
}

func (a WAFWebACL) ResourceCountLabel() string {
	if !a.ResourcesComplete {
		if len(a.ResourceARNs) > 0 {
			return fmt.Sprintf("%d+", len(a.ResourceARNs))
		}
		return "unknown"
	}
	return fmt.Sprintf("%d", len(a.ResourceARNs))
}

// Signals returns informational posture flags, not findings.
func (a WAFWebACL) Signals() []string {
	var signals []string
	if a.LoggingKnown && len(a.LogDestinations) == 0 {
		signals = append(signals, "no-logs")
	}
	if strings.EqualFold(a.DefaultAction, "ALLOW") {
		signals = append(signals, "allow-default")
	}
	if a.ResourcesComplete && len(a.ResourceARNs) == 0 {
		signals = append(signals, "unassociated")
	}
	return signals
}

func (a WAFWebACL) SignalLabel() string {
	if signals := a.Signals(); len(signals) > 0 {
		return strings.Join(signals, ",")
	}
	return "-"
}
