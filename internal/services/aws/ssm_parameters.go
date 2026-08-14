package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	uniclog "unic/internal/log"
)

// SSMParameter is one Parameter Store entry's metadata — never its value.
// Values are fetched separately, and decrypted only on an explicit request.
type SSMParameter struct {
	Name         string
	Type         string // String, StringList, SecureString
	Tier         string
	Version      int64
	KMSKeyID     string
	Description  string
	LastModified time.Time
	Region       string
}

// IsSecure reports whether the parameter is an encrypted SecureString.
func (p SSMParameter) IsSecure() bool { return p.Type == "SecureString" }

// DisplayTitle returns a formatted string for list display.
func (p SSMParameter) DisplayTitle() string {
	modified := "-"
	if !p.LastModified.IsZero() {
		modified = p.LastModified.Format("2006-01-02 15:04")
	}
	return fmt.Sprintf("%-56.56s %-13s %-17s %s", p.Name, p.Type, p.Tier, modified)
}

// FilterText returns a lowercase string for keyword matching. Names are
// hierarchical paths (/app/env/key), so path segments match naturally.
func (p SSMParameter) FilterText() string {
	return strings.ToLower(strings.Join([]string{p.Name, p.Type, p.Tier, p.Description}, " "))
}

// ListParameters returns all Parameter Store entries' metadata, sorted by
// path. Values are intentionally not fetched here.
func (r *AwsRepository) ListParameters(ctx context.Context) ([]SSMParameter, error) {
	uniclog.Debug("aws", "ListParameters called")

	var parameters []SSMParameter
	paginator := ssm.NewDescribeParametersPaginator(r.SSMClient, &ssm.DescribeParametersInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe parameters: %w", err)
		}
		for _, p := range page.Parameters {
			parameters = append(parameters, SSMParameter{
				Name:         awssdk.ToString(p.Name),
				Type:         string(p.Type),
				Tier:         string(p.Tier),
				Version:      p.Version,
				KMSKeyID:     awssdk.ToString(p.KeyId),
				Description:  awssdk.ToString(p.Description),
				LastModified: awssdk.ToTime(p.LastModifiedDate),
				Region:       r.Region,
			})
		}
	}

	sort.Slice(parameters, func(i, j int) bool {
		return normalizedSortKey(parameters[i].Name) < normalizedSortKey(parameters[j].Name)
	})
	return parameters, nil
}

// GetParameterValue fetches one parameter's value. SecureString values are
// decrypted; callers must only invoke this on an explicit reveal or copy
// request so decrypted values never load implicitly.
func (r *AwsRepository) GetParameterValue(ctx context.Context, name string) (string, error) {
	uniclog.Info("aws", "GetParameterValue called", "parameter", name)
	out, err := r.SSMClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           awssdk.String(name),
		WithDecryption: awssdk.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get parameter %s: %w", name, err)
	}
	if out.Parameter == nil {
		return "", fmt.Errorf("parameter %s has no value", name)
	}
	return awssdk.ToString(out.Parameter.Value), nil
}
