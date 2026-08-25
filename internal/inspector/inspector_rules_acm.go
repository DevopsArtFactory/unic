package inspector

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const inspectorACMExpiryDays = 30

func init() {
	registerSecurityInspectorScanner(InspectorScanner{Name: "acm-certificate-expiry", RunConfigured: runACMExpiryScan})
}

func runACMExpiryScan(ctx context.Context, repo *AwsRepository, options SecurityScanOptions) ([]SecurityFinding, error) {
	return inspectACMExpiry(ctx, repo, time.Now(), options.ACMExpiryWindowDays)
}

func inspectACMExpiry(ctx context.Context, repo *AwsRepository, now time.Time, expiryWindowDays int) ([]SecurityFinding, error) {
	if expiryWindowDays <= 0 {
		expiryWindowDays = inspectorACMExpiryDays
	}
	certificates, warnings, err := repo.ListCertificates(ctx)
	if err != nil {
		return nil, err
	}
	var findings []SecurityFinding
	for _, certificate := range certificates {
		if certificate.NotAfter.IsZero() || certificate.DaysToExpiry(now) > expiryWindowDays {
			continue
		}
		days := certificate.DaysToExpiry(now)
		severity := RuleSeverityMedium
		if days <= 7 {
			severity = RuleSeverityHigh
		}
		summary := fmt.Sprintf("Certificate for %s expires in %d days.", certificate.DomainName, days)
		if certificate.NotAfter.Before(now) {
			summary = fmt.Sprintf("Certificate for %s has expired.", certificate.DomainName)
		}
		findings = append(findings, SecurityFinding{
			RuleID:         "acm-certificate-expiry",
			RuleName:       "ACM certificate expiring soon",
			Severity:       severity,
			ResourceType:   "ACM Certificate",
			ResourceID:     certificate.ARN,
			Summary:        summary,
			Recommendation: "Renew or replace the certificate before expiry and verify dependent resources use the replacement.",
		})
	}
	return findings, errors.Join(warnings...)
}
