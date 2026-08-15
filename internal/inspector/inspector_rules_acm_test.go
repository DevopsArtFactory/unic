package inspector

import (
	"context"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
)

type acmInspectorMock struct{ expires time.Time }

func (m acmInspectorMock) ListCertificates(context.Context, *acm.ListCertificatesInput, ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	return &acm.ListCertificatesOutput{CertificateSummaryList: []acmtypes.CertificateSummary{{CertificateArn: awssdk.String("arn:cert")}}}, nil
}

func (m acmInspectorMock) DescribeCertificate(context.Context, *acm.DescribeCertificateInput, ...func(*acm.Options)) (*acm.DescribeCertificateOutput, error) {
	return &acm.DescribeCertificateOutput{Certificate: &acmtypes.CertificateDetail{
		CertificateArn: awssdk.String("arn:cert"), DomainName: awssdk.String("api.example.com"), NotAfter: awssdk.Time(m.expires),
	}}, nil
}

func TestInspectACMExpiryWarnsWithinThirtyDays(t *testing.T) {
	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	findings, err := inspectACMExpiry(context.Background(), &AwsRepository{ACMClient: acmInspectorMock{expires: now.Add(7 * 24 * time.Hour)}}, now, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].RuleID != "acm-certificate-expiry" || findings[0].Severity != RuleSeverityHigh {
		t.Fatalf("unexpected findings: %+v", findings)
	}
}

func TestInspectACMExpiryUsesConfiguredWindow(t *testing.T) {
	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	repo := &AwsRepository{ACMClient: acmInspectorMock{expires: now.Add(45 * 24 * time.Hour)}}

	findings, err := inspectACMExpiry(context.Background(), repo, now, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected configured 60-day window to emit one finding, got %+v", findings)
	}
}
