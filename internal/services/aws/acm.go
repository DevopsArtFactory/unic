package aws

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
)

// ACMCertificate contains the certificate fields used by the browser and inspector.
type ACMCertificate struct {
	ARN                 string
	DomainName          string
	SubjectAlternatives []string
	Status              string
	NotAfter            time.Time
	RenewalEligibility  string
	InUseBy             []string
	Validation          []ACMValidation
	Region              string
}

// ACMValidation describes domain validation state for a certificate.
type ACMValidation struct {
	Domain string
	Method string
	Status string
}

// DaysToExpiry returns whole calendar days until expiry, rounding partial days up.
func (c ACMCertificate) DaysToExpiry(now time.Time) int {
	if c.NotAfter.IsZero() {
		return 0
	}
	return max(int(math.Ceil(c.NotAfter.Sub(now).Hours()/24)), 0)
}

// DisplayTitle returns a column-aligned certificate row.
func (c ACMCertificate) DisplayTitle() string {
	days := "-"
	if !c.NotAfter.IsZero() {
		days = fmt.Sprintf("%dd", c.DaysToExpiry(time.Now()))
	}
	return fmt.Sprintf("%-42.42s %-12s %8s %6d  %s", c.DomainName, c.Status, days, len(c.InUseBy), valueOrDash(c.RenewalEligibility))
}

// FilterText returns searchable certificate metadata.
func (c ACMCertificate) FilterText() string {
	parts := []string{c.DomainName, c.ARN, c.Status, c.RenewalEligibility, c.Region}
	parts = append(parts, c.SubjectAlternatives...)
	parts = append(parts, c.InUseBy...)
	return strings.ToLower(strings.Join(parts, " "))
}

// ListCertificates returns certificate details sorted by earliest expiry.
func (r *AwsRepository) ListCertificates(ctx context.Context) ([]ACMCertificate, error) {
	var certificates []ACMCertificate
	var nextToken *string
	for {
		out, err := r.ACMClient.ListCertificates(ctx, &acm.ListCertificatesInput{
			NextToken: nextToken,
			Includes: &acmtypes.Filters{KeyTypes: []acmtypes.KeyAlgorithm{
				acmtypes.KeyAlgorithmRsa1024, acmtypes.KeyAlgorithmRsa2048, acmtypes.KeyAlgorithmRsa3072, acmtypes.KeyAlgorithmRsa4096,
				acmtypes.KeyAlgorithmEcPrime256v1, acmtypes.KeyAlgorithmEcSecp384r1, acmtypes.KeyAlgorithmEcSecp521r1,
			}},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list ACM certificates: %w", err)
		}
		for _, summary := range out.CertificateSummaryList {
			detailOut, err := r.ACMClient.DescribeCertificate(ctx, &acm.DescribeCertificateInput{CertificateArn: summary.CertificateArn})
			if err != nil {
				return nil, fmt.Errorf("failed to describe ACM certificate %s: %w", derefString(summary.CertificateArn), err)
			}
			if detailOut.Certificate == nil {
				continue
			}
			detail := detailOut.Certificate
			certificate := ACMCertificate{
				ARN:                 derefString(detail.CertificateArn),
				DomainName:          derefString(detail.DomainName),
				SubjectAlternatives: append([]string(nil), detail.SubjectAlternativeNames...),
				Status:              string(detail.Status),
				RenewalEligibility:  string(detail.RenewalEligibility),
				InUseBy:             append([]string(nil), detail.InUseBy...),
				Region:              r.Region,
			}
			if detail.NotAfter != nil {
				certificate.NotAfter = *detail.NotAfter
			}
			for _, validation := range detail.DomainValidationOptions {
				certificate.Validation = append(certificate.Validation, ACMValidation{
					Domain: derefString(validation.DomainName), Method: string(validation.ValidationMethod), Status: string(validation.ValidationStatus),
				})
			}
			certificates = append(certificates, certificate)
		}
		if out.NextToken == nil || derefString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}
	sort.SliceStable(certificates, func(i, j int) bool {
		if certificates[i].NotAfter.Equal(certificates[j].NotAfter) {
			return normalizedSortKey(certificates[i].DomainName) < normalizedSortKey(certificates[j].DomainName)
		}
		if certificates[i].NotAfter.IsZero() {
			return false
		}
		if certificates[j].NotAfter.IsZero() {
			return true
		}
		return certificates[i].NotAfter.Before(certificates[j].NotAfter)
	})
	return certificates, nil
}
