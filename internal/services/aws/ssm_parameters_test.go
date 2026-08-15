package aws

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func TestListParametersMapsAndSorts(t *testing.T) {
	pages := 0
	mock := &mockSSMClient{
		describeParametersFunc: func(_ context.Context, params *ssm.DescribeParametersInput, _ ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
			pages++
			if pages == 1 {
				if params.NextToken != nil {
					t.Fatalf("first page should have no token, got %v", params.NextToken)
				}
				return &ssm.DescribeParametersOutput{
					Parameters: []ssmtypes.ParameterMetadata{
						{
							Name:             aws.String("/app/prod/db-password"),
							Type:             ssmtypes.ParameterTypeSecureString,
							Tier:             ssmtypes.ParameterTierStandard,
							Version:          3,
							KeyId:            aws.String("alias/app-key"),
							Description:      aws.String("prod db password"),
							LastModifiedDate: aws.Time(time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)),
						},
					},
					NextToken: aws.String("page2"),
				}, nil
			}
			if aws.ToString(params.NextToken) != "page2" {
				t.Fatalf("second page should carry token, got %v", params.NextToken)
			}
			return &ssm.DescribeParametersOutput{
				Parameters: []ssmtypes.ParameterMetadata{
					{
						Name: aws.String("/app/dev/api-url"),
						Type: ssmtypes.ParameterTypeString,
						Tier: ssmtypes.ParameterTierStandard,
					},
				},
			}, nil
		},
	}
	repo := &AwsRepository{SSMClient: mock, Region: "us-east-1"}

	parameters, err := repo.ListParameters(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parameters) != 2 {
		t.Fatalf("expected parameters from both pages, got %d", len(parameters))
	}
	if parameters[0].Name != "/app/dev/api-url" || parameters[1].Name != "/app/prod/db-password" {
		t.Fatalf("expected path-sorted order, got %s, %s", parameters[0].Name, parameters[1].Name)
	}
	secure := parameters[1]
	if secure.Type != "SecureString" || !secure.IsSecure() || secure.Version != 3 || secure.KMSKeyID != "alias/app-key" {
		t.Fatalf("unexpected mapping: %+v", secure)
	}
	if secure.Region != "us-east-1" || secure.LastModified.IsZero() {
		t.Fatalf("expected region and last-modified stamped, got %+v", secure)
	}
}

func TestListParametersWrapsError(t *testing.T) {
	mock := &mockSSMClient{
		describeParametersFunc: func(_ context.Context, _ *ssm.DescribeParametersInput, _ ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
			return nil, errors.New("denied")
		},
	}
	repo := &AwsRepository{SSMClient: mock}
	if _, err := repo.ListParameters(context.Background()); err == nil || !strings.Contains(err.Error(), "failed to describe parameters") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestGetParameterValueRequestsDecryption(t *testing.T) {
	mock := &mockSSMClient{
		getParameterFunc: func(_ context.Context, params *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			if aws.ToString(params.Name) != "/app/prod/db-password" {
				t.Fatalf("unexpected parameter name: %s", aws.ToString(params.Name))
			}
			if !aws.ToBool(params.WithDecryption) {
				t.Fatal("expected WithDecryption to be set")
			}
			return &ssm.GetParameterOutput{
				Parameter: &ssmtypes.Parameter{Value: aws.String("s3cret")},
			}, nil
		},
	}
	repo := &AwsRepository{SSMClient: mock}

	value, err := repo.GetParameterValue(context.Background(), "/app/prod/db-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "s3cret" {
		t.Fatalf("unexpected value: %q", value)
	}
}

func TestGetParameterValueWrapsError(t *testing.T) {
	mock := &mockSSMClient{
		getParameterFunc: func(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return nil, errors.New("denied")
		},
	}
	repo := &AwsRepository{SSMClient: mock}
	if _, err := repo.GetParameterValue(context.Background(), "/x"); err == nil || !strings.Contains(err.Error(), "failed to get parameter /x") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestGetParameterValueRejectsMissingValue(t *testing.T) {
	responses := []*ssm.GetParameterOutput{
		nil,
		{},
		{Parameter: &ssmtypes.Parameter{}},
	}
	for _, response := range responses {
		mock := &mockSSMClient{
			getParameterFunc: func(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
				return response, nil
			},
		}
		repo := &AwsRepository{SSMClient: mock}
		if _, err := repo.GetParameterValue(context.Background(), "/x"); err == nil || err.Error() != "parameter /x has no value" {
			t.Fatalf("expected missing-value error, got %v", err)
		}
	}
}
