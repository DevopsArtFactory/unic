package inspector

import (
	"context"
	"fmt"
	"time"
)

const inspectorACMExpiryDays = 30

func init() {
	registerSecurityInspectorScanner(InspectorScanner{Name: "acm-certificate-expiry", Run: runACMExpiryScan})
}

func runACMExpiryScan(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
	return inspectACMExpiry(ctx, repo, time.Now())
}

func inspectACMExpiry(ctx context.Context, repo *AwsRepository, now time.Time) ([]SecurityFinding, error) {
	certificates, err := repo.ListCertificates(ctx)
	if err != nil {
		return nil, err
	}
	var findings []SecurityFinding
	for _, certificate := range certificates {
		if certificate.NotAfter.IsZero() || certificate.DaysToExpiry(now) > inspectorACMExpiryDays {
			continue
		}
		days := certificate.DaysToExpiry(now)
		severity := RuleSeverityMedium
		if days <= 7 {
			severity = RuleSeverityHigh
		}
		findings = append(findings, SecurityFinding{
			RuleID:         "acm-certificate-expiry",
			RuleName:       "ACM certificate expiring soon",
			Severity:       severity,
			ResourceType:   "ACM Certificate",
			ResourceID:     certificate.ARN,
			Summary:        fmt.Sprintf("Certificate for %s expires in %d days.", certificate.DomainName, days),
			Recommendation: "Renew or replace the certificate before expiry and verify dependent resources use the replacement.",
		})
	}
	return findings, nil
}
