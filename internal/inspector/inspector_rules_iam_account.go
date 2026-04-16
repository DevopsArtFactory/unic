package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

const (
	inspectorScannerIAMRootName     = "iam-root-account"
	inspectorScannerIAMWildcardName = "iam-wildcard-policies"

	inspectorRuleIDRootMFADisabled       = "iam-root-mfa-disabled"
	inspectorRuleIDRootAccessKeysPresent = "iam-root-access-keys-present"
	inspectorRuleIDIAMPolicyBroadAccess  = "iam-policy-broad-access"
)

type iamPolicyDocument struct {
	Statement json.RawMessage `json:"Statement"`
}

type iamPolicyStatement struct {
	Effect    string `json:"Effect"`
	Action    any    `json:"Action"`
	NotAction any    `json:"NotAction"`
	Resource  any    `json:"Resource"`
}

type iamPolicyAssessment struct {
	Severity RuleSeverity
	Reasons  []string
}

func init() {
	registerSecurityInspectorScanner(InspectorScanner{
		Name: inspectorScannerIAMRootName,
		Run:  runIAMRootAccountScan,
	})
	registerSecurityInspectorScanner(InspectorScanner{
		Name: inspectorScannerIAMWildcardName,
		Run:  runIAMWildcardPolicyScan,
	})
}

func runIAMRootAccountScan(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
	summary, err := repo.IAMClient.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect IAM account summary: %w", err)
	}

	var findings []SecurityFinding
	if summary.SummaryMap["AccountMFAEnabled"] == 0 {
		findings = append(findings, SecurityFinding{
			RuleID:         inspectorRuleIDRootMFADisabled,
			RuleName:       "Root account MFA disabled",
			Severity:       RuleSeverityCritical,
			ResourceType:   "AWSAccount",
			ResourceID:     "root-account",
			Summary:        "The AWS account root user does not have MFA enabled.",
			Recommendation: "Enable MFA on the root user and reserve root access for break-glass administration only.",
		})
	}
	if summary.SummaryMap["AccountAccessKeysPresent"] > 0 {
		findings = append(findings, SecurityFinding{
			RuleID:         inspectorRuleIDRootAccessKeysPresent,
			RuleName:       "Root account access keys present",
			Severity:       RuleSeverityCritical,
			ResourceType:   "AWSAccount",
			ResourceID:     "root-account",
			Summary:        "The AWS account root user still has active access keys present.",
			Recommendation: "Delete root access keys and use IAM roles or short-lived credentials for programmatic access.",
		})
	}

	return findings, nil
}

func runIAMWildcardPolicyScan(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
	accountDetails, err := loadAccountAuthorizationDetails(ctx, repo.IAMClient)
	if err != nil {
		return nil, err
	}

	managedPolicyDocs := buildManagedPolicyDocumentIndex(accountDetails.Policies)
	findings := collectIAMPolicyFindings(accountDetails.UserDetailList, accountDetails.GroupDetailList, accountDetails.RoleDetailList, managedPolicyDocs)

	sort.Slice(findings, func(i, j int) bool {
		left := normalizedSortKey(findings[i].ResourceID, findings[i].RuleID, findings[i].RuleName)
		right := normalizedSortKey(findings[j].ResourceID, findings[j].RuleID, findings[j].RuleName)
		if left == right {
			return findings[i].Severity.Rank() < findings[j].Severity.Rank()
		}
		return left < right
	})

	return findings, nil
}

func loadAccountAuthorizationDetails(ctx context.Context, client IAMClientAPI) (*iam.GetAccountAuthorizationDetailsOutput, error) {
	result := &iam.GetAccountAuthorizationDetailsOutput{}
	marker := ""

	for {
		input := &iam.GetAccountAuthorizationDetailsInput{
			MaxItems: awssdk.Int32(1000),
		}
		if marker != "" {
			input.Marker = awssdk.String(marker)
		}

		page, err := client.GetAccountAuthorizationDetails(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect IAM policy attachments: %w", err)
		}

		result.UserDetailList = append(result.UserDetailList, page.UserDetailList...)
		result.GroupDetailList = append(result.GroupDetailList, page.GroupDetailList...)
		result.RoleDetailList = append(result.RoleDetailList, page.RoleDetailList...)
		result.Policies = append(result.Policies, page.Policies...)

		if !page.IsTruncated || awssdk.ToString(page.Marker) == "" {
			break
		}
		marker = awssdk.ToString(page.Marker)
	}

	return result, nil
}

func buildManagedPolicyDocumentIndex(policies []iamtypes.ManagedPolicyDetail) map[string]string {
	index := make(map[string]string, len(policies))
	for _, policy := range policies {
		arn := awssdk.ToString(policy.Arn)
		if arn == "" {
			continue
		}
		for _, version := range policy.PolicyVersionList {
			if version.IsDefaultVersion && version.Document != nil {
				index[arn] = awssdk.ToString(version.Document)
				break
			}
		}
	}
	return index
}

func collectIAMPolicyFindings(
	users []iamtypes.UserDetail,
	groups []iamtypes.GroupDetail,
	roles []iamtypes.RoleDetail,
	managedPolicyDocs map[string]string,
) []SecurityFinding {
	var findings []SecurityFinding

	for _, user := range users {
		principal := fmt.Sprintf("user/%s", awssdk.ToString(user.UserName))
		findings = append(findings, collectPrincipalPolicyFindings(principal, user.UserPolicyList, user.AttachedManagedPolicies, managedPolicyDocs)...)
	}
	for _, group := range groups {
		principal := fmt.Sprintf("group/%s", awssdk.ToString(group.GroupName))
		findings = append(findings, collectPrincipalPolicyFindings(principal, group.GroupPolicyList, group.AttachedManagedPolicies, managedPolicyDocs)...)
	}
	for _, role := range roles {
		principal := fmt.Sprintf("role/%s", awssdk.ToString(role.RoleName))
		findings = append(findings, collectPrincipalPolicyFindings(principal, role.RolePolicyList, role.AttachedManagedPolicies, managedPolicyDocs)...)
	}

	return findings
}

func collectPrincipalPolicyFindings(
	principal string,
	inlinePolicies []iamtypes.PolicyDetail,
	attachedPolicies []iamtypes.AttachedPolicy,
	managedPolicyDocs map[string]string,
) []SecurityFinding {
	var findings []SecurityFinding

	for _, policy := range inlinePolicies {
		policyName := awssdk.ToString(policy.PolicyName)
		assessment, err := assessIAMPolicyDocument(awssdk.ToString(policy.PolicyDocument))
		if err != nil || len(assessment.Reasons) == 0 {
			continue
		}
		findings = append(findings, buildIAMPolicyFinding(principal, "inline", policyName, assessment))
	}

	for _, policy := range attachedPolicies {
		policyName := awssdk.ToString(policy.PolicyName)
		document := managedPolicyDocs[awssdk.ToString(policy.PolicyArn)]
		if document == "" {
			continue
		}

		assessment, err := assessIAMPolicyDocument(document)
		if err != nil || len(assessment.Reasons) == 0 {
			continue
		}
		findings = append(findings, buildIAMPolicyFinding(principal, "managed", policyName, assessment))
	}

	return findings
}

func buildIAMPolicyFinding(principal, policyKind, policyName string, assessment iamPolicyAssessment) SecurityFinding {
	resourceID := fmt.Sprintf("%s:%s/%s", principal, policyKind, policyName)
	return SecurityFinding{
		RuleID:       inspectorRuleIDIAMPolicyBroadAccess,
		RuleName:     "Overly permissive IAM policy attachment",
		Severity:     assessment.Severity,
		ResourceType: "IAMPolicyAttachment",
		ResourceID:   resourceID,
		Summary: fmt.Sprintf(
			"%s attaches %s policy %s with %s.",
			principal,
			policyKind,
			policyName,
			strings.Join(assessment.Reasons, ", "),
		),
		Recommendation: "Narrow the policy to the minimum required actions and resources, and avoid wildcard administrative access where possible.",
	}
}

func assessIAMPolicyDocument(document string) (iamPolicyAssessment, error) {
	decoded, err := decodeIAMPolicyDocument(document)
	if err != nil {
		return iamPolicyAssessment{}, err
	}

	statements, err := parseIAMPolicyStatements(decoded)
	if err != nil {
		return iamPolicyAssessment{}, err
	}

	reasons := make(map[string]struct{})
	severity := RuleSeverity("")

	for _, statement := range statements {
		if !strings.EqualFold(statement.Effect, "allow") {
			continue
		}

		actions := normalizeIAMPolicyValues(statement.Action)
		notActions := normalizeIAMPolicyValues(statement.NotAction)
		resources := normalizeIAMPolicyValues(statement.Resource)

		switch {
		case len(notActions) > 0 && hasGlobalWildcardResource(resources):
			reasons["an allow statement uses NotAction with Resource *"] = struct{}{}
			severity = strongerSeverity(severity, RuleSeverityCritical)
		case hasBroadWildcardAction(actions) && hasGlobalWildcardResource(resources):
			reasons["wildcard actions on all resources"] = struct{}{}
			severity = strongerSeverity(severity, RuleSeverityCritical)
		default:
			if hasBroadWildcardAction(actions) {
				reasons["wildcard actions"] = struct{}{}
				severity = strongerSeverity(severity, RuleSeverityHigh)
			}
			if hasGlobalWildcardResource(resources) && hasWriteScopeAction(actions) {
				reasons["write access on Resource *"] = struct{}{}
				severity = strongerSeverity(severity, RuleSeverityHigh)
			}
		}
	}

	if len(reasons) == 0 {
		return iamPolicyAssessment{}, nil
	}

	ordered := make([]string, 0, len(reasons))
	for reason := range reasons {
		ordered = append(ordered, reason)
	}
	sort.Strings(ordered)

	return iamPolicyAssessment{
		Severity: severity,
		Reasons:  ordered,
	}, nil
}

func decodeIAMPolicyDocument(document string) ([]byte, error) {
	if document == "" {
		return nil, fmt.Errorf("empty IAM policy document")
	}

	decoded, err := url.QueryUnescape(document)
	if err != nil {
		decoded = document
	}
	return []byte(decoded), nil
}

func parseIAMPolicyStatements(document []byte) ([]iamPolicyStatement, error) {
	var policy iamPolicyDocument
	if err := json.Unmarshal(document, &policy); err != nil {
		return nil, err
	}

	if len(policy.Statement) == 0 {
		return nil, nil
	}

	if policy.Statement[0] == '[' {
		var statements []iamPolicyStatement
		if err := json.Unmarshal(policy.Statement, &statements); err != nil {
			return nil, err
		}
		return statements, nil
	}

	var statement iamPolicyStatement
	if err := json.Unmarshal(policy.Statement, &statement); err != nil {
		return nil, err
	}
	return []iamPolicyStatement{statement}, nil
}

func normalizeIAMPolicyValues(value any) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return []string{typed}
	case []any:
		values := make([]string, 0, len(typed))
		for _, entry := range typed {
			if text, ok := entry.(string); ok {
				values = append(values, text)
			}
		}
		return values
	default:
		return nil
	}
}

func hasGlobalWildcardResource(resources []string) bool {
	for _, resource := range resources {
		if strings.TrimSpace(resource) == "*" {
			return true
		}
	}
	return false
}

func hasBroadWildcardAction(actions []string) bool {
	for _, action := range actions {
		parts := strings.SplitN(strings.TrimSpace(action), ":", 2)
		if len(parts) == 1 {
			if parts[0] == "*" {
				return true
			}
			continue
		}
		if parts[1] == "*" {
			return true
		}
	}
	return false
}

func hasWriteScopeAction(actions []string) bool {
	for _, action := range actions {
		if action == "" {
			continue
		}
		if hasBroadWildcardAction([]string{action}) {
			return true
		}

		parts := strings.SplitN(action, ":", 2)
		verb := action
		if len(parts) == 2 {
			verb = parts[1]
		}
		if !isReadOnlyIAMVerb(verb) {
			return true
		}
	}
	return false
}

func isReadOnlyIAMVerb(verb string) bool {
	readPrefixes := []string{
		"Get",
		"List",
		"Describe",
		"View",
		"Lookup",
		"Read",
		"Search",
		"BatchGet",
		"Head",
		"Query",
	}

	for _, prefix := range readPrefixes {
		if strings.HasPrefix(verb, prefix) {
			return true
		}
	}
	return false
}

func strongerSeverity(current, candidate RuleSeverity) RuleSeverity {
	if current == "" || candidate.Rank() < current.Rank() {
		return candidate
	}
	return current
}
