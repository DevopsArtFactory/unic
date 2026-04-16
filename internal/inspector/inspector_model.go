package inspector

import (
	"fmt"
	"strings"
	"time"

	awsservice "unic/internal/services/aws"
)

type WorkflowKind string

const (
	WorkflowSecurity  WorkflowKind = "security"
	WorkflowChecklist WorkflowKind = "checklist"
)

type Workflow struct {
	Kind        WorkflowKind
	Title       string
	Description string
	Available   bool
	Status      string
}

func Workflows(checklistPath string) []Workflow {
	checklistAvailable := strings.TrimSpace(checklistPath) != ""
	checklistStatus := "ADD FILE"
	checklistDescription := "Pass --checklist <path> to load a YAML checklist for readiness checks across RDS, security groups, and Secrets Manager."
	if checklistAvailable {
		checklistStatus = ""
		checklistDescription = "Run a user-supplied YAML checklist to verify infrastructure readiness across RDS, security groups, and Secrets Manager."
	}

	return []Workflow{
		{
			Kind:        WorkflowSecurity,
			Title:       "Security Inspector",
			Description: "Cross-service security findings for network exposure, identity, storage, logging, and baseline posture.",
			Available:   true,
		},
		{
			Kind:        WorkflowChecklist,
			Title:       "Checklist Inspector",
			Description: checklistDescription,
			Available:   checklistAvailable,
			Status:      checklistStatus,
		},
	}
}

func (w Workflow) StatusLabel() string {
	if w.Available {
		return "READY"
	}
	if w.Status != "" {
		return w.Status
	}
	return "PLANNED"
}

type AwsRepository = awsservice.AwsRepository
type AccessKey = awsservice.AccessKey
type CloudTrailClientAPI = awsservice.CloudTrailClientAPI
type ConfigServiceClientAPI = awsservice.ConfigServiceClientAPI
type EC2ClientAPI = awsservice.EC2ClientAPI
type ElastiCacheClientAPI = awsservice.ElastiCacheClientAPI
type GuardDutyClientAPI = awsservice.GuardDutyClientAPI
type IAMClientAPI = awsservice.IAMClientAPI
type RDSClientAPI = awsservice.RDSClientAPI
type S3Bucket = awsservice.S3Bucket
type Secret = awsservice.Secret
type SecretDetail = awsservice.SecretDetail
type SecurityGroup = awsservice.SecurityGroup
type SecurityGroupRule = awsservice.SecurityGroupRule
type SecretsManagerClientAPI = awsservice.SecretsManagerClientAPI
type RDSInstance = awsservice.RDSInstance

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
