package aws

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
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
			origins := in.CertificateKeyPairOrigins
			if len(origins) != 3 || origins[0] != acmtypes.CertificateKeyPairOriginAwsManaged || origins[1] != acmtypes.CertificateKeyPairOriginAcme || origins[2] != acmtypes.CertificateKeyPairOriginCustomerProvided {
				t.Fatalf("expected all supported certificate origins, got %+v", origins)
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

	certificates, warnings, err := (&AwsRepository{ACMClient: mock, Region: "us-east-1"}).ListCertificates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
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

func TestListCertificatesDescribesConcurrentlyWithLimit(t *testing.T) {
	const certificateCount = 12
	summaries := make([]acmtypes.CertificateSummary, certificateCount)
	for i := range summaries {
		summaries[i].CertificateArn = awssdk.String(fmt.Sprintf("cert-%d", i))
	}

	var active, peak atomic.Int32
	release := make(chan struct{})
	mock := &mockACMClient{
		list: func(context.Context, *acm.ListCertificatesInput, ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
			return &acm.ListCertificatesOutput{CertificateSummaryList: summaries}, nil
		},
		describe: func(ctx context.Context, in *acm.DescribeCertificateInput, _ ...func(*acm.Options)) (*acm.DescribeCertificateOutput, error) {
			current := active.Add(1)
			for previous := peak.Load(); current > previous && !peak.CompareAndSwap(previous, current); previous = peak.Load() {
			}
			if current == 10 {
				close(release)
			}
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			active.Add(-1)
			return &acm.DescribeCertificateOutput{Certificate: &acmtypes.CertificateDetail{
				CertificateArn: in.CertificateArn, DomainName: in.CertificateArn,
			}}, nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	certificates, warnings, err := (&AwsRepository{ACMClient: mock}).ListCertificates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(certificates) != certificateCount || peak.Load() != 10 {
		t.Fatalf("expected %d certificates and 10 concurrent describes, got %d and %d", certificateCount, len(certificates), peak.Load())
	}
}

func TestListCertificatesKeepsSuccessfulDescriptions(t *testing.T) {
	mock := &mockACMClient{
		list: func(context.Context, *acm.ListCertificatesInput, ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
			return &acm.ListCertificatesOutput{CertificateSummaryList: []acmtypes.CertificateSummary{
				{CertificateArn: awssdk.String("denied")},
				{CertificateArn: awssdk.String("visible")},
			}}, nil
		},
		describe: func(_ context.Context, in *acm.DescribeCertificateInput, _ ...func(*acm.Options)) (*acm.DescribeCertificateOutput, error) {
			if awssdk.ToString(in.CertificateArn) == "denied" {
				return nil, fmt.Errorf("access denied")
			}
			return &acm.DescribeCertificateOutput{Certificate: &acmtypes.CertificateDetail{
				CertificateArn: in.CertificateArn,
				DomainName:     awssdk.String("visible.example.com"),
			}}, nil
		},
	}

	certificates, warnings, err := (&AwsRepository{ACMClient: mock}).ListCertificates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(certificates) != 1 || certificates[0].ARN != "visible" {
		t.Fatalf("expected visible certificate to be preserved, got %+v", certificates)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Error(), "failed to describe ACM certificate denied") {
		t.Fatalf("expected denied certificate warning, got %v", warnings)
	}
}
