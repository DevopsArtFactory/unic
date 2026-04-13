package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	inspectorIAMAccessKeyMediumAgeDays = 90
	inspectorIAMAccessKeyHighAgeDays   = 180
	inspectorSecretRotationAgeDays     = 90
	inspectorSecretRotationHighAgeDays = 2 * inspectorSecretRotationAgeDays
)

func init() {
	registerSecurityInspectorScanner(InspectorScanner{
		Name: "iam-access-key-age",
		Run:  runIAMAccessKeyAgeScan,
	})
	registerSecurityInspectorScanner(InspectorScanner{
		Name: "secrets-rotation-age",
		Run:  runSecretsManagerRotationAgeScan,
	})
}

func runIAMAccessKeyAgeScan(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
	return inspectIAMAccessKeyAges(ctx, repo, time.Now().UTC())
}

func runSecretsManagerRotationAgeScan(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
	return inspectSecretsManagerRotationAges(ctx, repo, time.Now().UTC())
}

func inspectIAMAccessKeyAges(ctx context.Context, repo *AwsRepository, now time.Time) ([]SecurityFinding, error) {
	var findings []SecurityFinding
	marker := ""

	for {
		page, err := repo.ListIAMUserSummariesPage(ctx, marker, 100)
		if err != nil {
			return nil, err
		}

		for _, user := range page.Users {
			keys, err := repo.listUserAccessKeys(ctx, user.UserName)
			if err != nil {
				return nil, err
			}
			for _, key := range keys {
				if finding, ok := buildIAMAccessKeyFinding(user.UserName, key, now); ok {
					findings = append(findings, finding)
				}
			}
		}

		if !page.HasMore || page.NextMarker == "" {
			break
		}
		marker = page.NextMarker
	}

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

func inspectSecretsManagerRotationAges(ctx context.Context, repo *AwsRepository, now time.Time) ([]SecurityFinding, error) {
	secrets, err := repo.ListSecrets(ctx)
	if err != nil {
		return nil, err
	}

	findings := make([]SecurityFinding, 0, len(secrets))
	for _, secret := range secrets {
		if finding, ok := buildSecretsRotationFinding(secret, now); ok {
			findings = append(findings, finding)
		}
	}

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

func buildIAMAccessKeyFinding(userName string, key AccessKey, now time.Time) (SecurityFinding, bool) {
	if !strings.EqualFold(key.Status, "active") || key.CreateDate.IsZero() {
		return SecurityFinding{}, false
	}

	ageDays := int(now.Sub(key.CreateDate).Hours() / 24)
	if ageDays < inspectorIAMAccessKeyMediumAgeDays {
		return SecurityFinding{}, false
	}

	severity := RuleSeverityMedium
	if ageDays >= inspectorSecretRotationHighAgeDays {
		severity = RuleSeverityHigh
	}

	return SecurityFinding{
		RuleID:       "iam-access-key-age",
		RuleName:     "Stale IAM access keys",
		Severity:     severity,
		ResourceType: "IAM Access Key",
		ResourceID:   fmt.Sprintf("%s/%s", userName, key.AccessKeyID),
		Summary: fmt.Sprintf(
			"Access key %s for %s is %d days old.",
			key.AccessKeyID,
			userName,
			ageDays,
		),
		Recommendation: fmt.Sprintf(
			"Rotate or remove access keys older than %d days and prefer short-lived credentials where possible.",
			inspectorIAMAccessKeyMediumAgeDays,
		),
	}, true
}

func buildSecretsRotationFinding(secret Secret, now time.Time) (SecurityFinding, bool) {
	if !secret.RotationEnabled {
		return SecurityFinding{
			RuleID:         "secret-rotation-disabled",
			RuleName:       "Secrets Manager rotation disabled",
			Severity:       RuleSeverityHigh,
			ResourceType:   "Secret",
			ResourceID:     secret.Name,
			Summary:        "Automatic rotation is disabled for this secret.",
			Recommendation: "Enable rotation for secrets that store credentials or other long-lived sensitive material.",
		}, true
	}

	referenceTime := secret.LastRotatedDate
	if referenceTime.IsZero() {
		referenceTime = secret.LastChangedDate
	}
	if referenceTime.IsZero() {
		referenceTime = secret.CreatedDate
	}
	if referenceTime.IsZero() {
		return SecurityFinding{
			RuleID:         "secret-rotation-unknown",
			RuleName:       "Secrets rotation age unavailable",
			Severity:       RuleSeverityMedium,
			ResourceType:   "Secret",
			ResourceID:     secret.Name,
			Summary:        "Rotation is enabled, but no rotation timestamp is available.",
			Recommendation: "Review the secret lifecycle and ensure rotation metadata is populated for ongoing monitoring.",
		}, true
	}

	ageDays := int(now.Sub(referenceTime).Hours() / 24)
	if ageDays < inspectorSecretRotationAgeDays {
		return SecurityFinding{}, false
	}

	severity := RuleSeverityMedium
	if ageDays >= inspectorIAMAccessKeyHighAgeDays {
		severity = RuleSeverityHigh
	}

	return SecurityFinding{
		RuleID:       "secret-rotation-age",
		RuleName:     "Secrets Manager rotation overdue",
		Severity:     severity,
		ResourceType: "Secret",
		ResourceID:   secret.Name,
		Summary: fmt.Sprintf(
			"Secret was last rotated or updated %d days ago.",
			ageDays,
		),
		Recommendation: fmt.Sprintf(
			"Rotate secrets at least every %d days and confirm the secret's rotation schedule is enforced.",
			inspectorSecretRotationAgeDays,
		),
	}, true
}
