package inspector

import (
	"context"
	"strings"
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

func TestInspectACMExpiryDistinguishesExpiredFromExpiringToday(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, want string
		expires    time.Time
	}{
		{name: "expired", expires: now.Add(-time.Hour), want: "has expired"},
		{name: "expiring now", expires: now, want: "expires in 0 days"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := inspectACMExpiry(context.Background(), &AwsRepository{ACMClient: acmInspectorMock{expires: tc.expires}}, now, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 || !strings.Contains(findings[0].Summary, tc.want) {
				t.Fatalf("unexpected findings: %+v", findings)
			}
		})
	}
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
