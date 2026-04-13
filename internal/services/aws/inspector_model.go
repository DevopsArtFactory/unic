package aws

import (
	"fmt"
	"strings"
	"time"
)

type RuleSeverity string

const (
	RuleSeverityCritical RuleSeverity = "CRITICAL"
	RuleSeverityHigh     RuleSeverity = "HIGH"
	RuleSeverityMedium   RuleSeverity = "MEDIUM"
	RuleSeverityLow      RuleSeverity = "LOW"
)

func (s RuleSeverity) Rank() int {
	switch s {
	case RuleSeverityCritical:
		return 0
	case RuleSeverityHigh:
		return 1
	case RuleSeverityMedium:
		return 2
	case RuleSeverityLow:
		return 3
	default:
		return 4
	}
}

func (s RuleSeverity) Label() string {
	if s == "" {
		return "ALL"
	}
	return string(s)
}

type SecurityFinding struct {
	RuleID         string
	RuleName       string
	Severity       RuleSeverity
	ResourceType   string
	ResourceID     string
	Summary        string
	Recommendation string
}

func (f SecurityFinding) DisplayTitle() string {
	resource := f.ResourceID
	if resource == "" {
		resource = f.ResourceType
	}
	ruleName := f.RuleName
	if ruleName == "" {
		ruleName = f.RuleID
	}
	return fmt.Sprintf("[%s] %s — %s", f.Severity, resource, ruleName)
}

func (f SecurityFinding) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s %s",
		f.RuleID,
		f.RuleName,
		f.Severity,
		f.ResourceType,
		f.ResourceID,
		f.Summary,
		f.Recommendation,
	))
}

func (f SecurityFinding) MatchesSeverity(severity RuleSeverity) bool {
	return severity == "" || f.Severity == severity
}

type SecurityScanReport struct {
	Findings     []SecurityFinding
	ScannerCount int
	Warnings     []string
	ScannedAt    time.Time
}

func (r SecurityScanReport) FindingsForSeverity(severity RuleSeverity) int {
	count := 0
	for _, finding := range r.Findings {
		if finding.MatchesSeverity(severity) {
			count++
		}
	}
	return count
}
