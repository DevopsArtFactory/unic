package aws

import (
	"context"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
)

type mockACMClient struct {
	list     func(context.Context, *acm.ListCertificatesInput, ...func(*acm.Options)) (*acm.ListCertificatesOutput, error)
	describe func(context.Context, *acm.DescribeCertificateInput, ...func(*acm.Options)) (*acm.DescribeCertificateOutput, error)
}

func (m *mockACMClient) ListCertificates(ctx context.Context, in *acm.ListCertificatesInput, opts ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	return m.list(ctx, in, opts...)
}

func (m *mockACMClient) DescribeCertificate(ctx context.Context, in *acm.DescribeCertificateInput, opts ...func(*acm.Options)) (*acm.DescribeCertificateOutput, error) {
	return m.describe(ctx, in, opts...)
}

func TestListCertificatesMapsAndSortsByExpiry(t *testing.T) {
	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	mock := &mockACMClient{
		list: func(_ context.Context, in *acm.ListCertificatesInput, _ ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
			if in.Includes == nil || len(in.Includes.KeyTypes) != 7 {
				t.Fatalf("expected all supported key types, got %+v", in.Includes)
			}
			return &acm.ListCertificatesOutput{CertificateSummaryList: []acmtypes.CertificateSummary{
				{CertificateArn: awssdk.String("late")}, {CertificateArn: awssdk.String("soon")},
			}}, nil
		},
		describe: func(_ context.Context, in *acm.DescribeCertificateInput, _ ...func(*acm.Options)) (*acm.DescribeCertificateOutput, error) {
			if awssdk.ToString(in.CertificateArn) == "soon" {
				return &acm.DescribeCertificateOutput{Certificate: &acmtypes.CertificateDetail{
					CertificateArn: awssdk.String("soon"), DomainName: awssdk.String("soon.example.com"), Status: acmtypes.CertificateStatusIssued,
					NotAfter: awssdk.Time(now.Add(5 * 24 * time.Hour)), RenewalEligibility: acmtypes.RenewalEligibilityEligible,
					SubjectAlternativeNames: []string{"www.example.com"}, InUseBy: []string{"load-balancer"},
					DomainValidationOptions: []acmtypes.DomainValidation{{DomainName: awssdk.String("soon.example.com"), ValidationMethod: acmtypes.ValidationMethodDns, ValidationStatus: acmtypes.DomainStatusSuccess}},
				}}, nil
			}
			return &acm.DescribeCertificateOutput{Certificate: &acmtypes.CertificateDetail{
				CertificateArn: awssdk.String("late"), DomainName: awssdk.String("late.example.com"), NotAfter: awssdk.Time(now.Add(90 * 24 * time.Hour)),
			}}, nil
		},
	}

	certificates, err := (&AwsRepository{ACMClient: mock, Region: "us-east-1"}).ListCertificates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(certificates) != 2 || certificates[0].ARN != "soon" {
		t.Fatalf("expected soonest expiry first, got %+v", certificates)
	}
	soon := certificates[0]
	if soon.DaysToExpiry(now) != 5 || len(soon.InUseBy) != 1 || len(soon.Validation) != 1 || soon.Region != "us-east-1" {
		t.Fatalf("unexpected mapping: %+v", soon)
	}
}

func TestCertificateDaysToExpiryClampsExpiredCertificate(t *testing.T) {
	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	certificate := ACMCertificate{NotAfter: now.Add(-24 * time.Hour)}
	if got := certificate.DaysToExpiry(now); got != 0 {
		t.Fatalf("expected expired certificate to report zero days, got %d", got)
	}
}
