package aws

import (
	"context"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

func TestRunS3InspectorScan_FindsPublicACLPolicyPublicAccessBlockAndVersioning(t *testing.T) {
	mock := &mockS3Client{
		listBucketsFunc: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{
					{Name: awssdk.String("public-bucket")},
				},
			}, nil
		},
		getBucketAclFunc: func(_ context.Context, in *s3.GetBucketAclInput, _ ...func(*s3.Options)) (*s3.GetBucketAclOutput, error) {
			if awssdk.ToString(in.Bucket) != "public-bucket" {
				t.Fatalf("unexpected bucket for ACL lookup: %q", awssdk.ToString(in.Bucket))
			}
			return &s3.GetBucketAclOutput{
				Grants: []s3types.Grant{
					{
						Grantee: &s3types.Grantee{
							Type: s3types.TypeGroup,
							URI:  awssdk.String(s3PublicAllUsersURI),
						},
						Permission: s3types.PermissionRead,
					},
				},
			}, nil
		},
		getPublicAccessBlockFunc: func(_ context.Context, in *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
			if awssdk.ToString(in.Bucket) != "public-bucket" {
				t.Fatalf("unexpected bucket for public access block lookup: %q", awssdk.ToString(in.Bucket))
			}
			return &s3.GetPublicAccessBlockOutput{
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       awssdk.Bool(false),
					IgnorePublicAcls:      awssdk.Bool(false),
					BlockPublicPolicy:     awssdk.Bool(false),
					RestrictPublicBuckets: awssdk.Bool(false),
				},
			}, nil
		},
		getBucketPolicyStatusFunc: func(_ context.Context, in *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
			if awssdk.ToString(in.Bucket) != "public-bucket" {
				t.Fatalf("unexpected bucket for policy lookup: %q", awssdk.ToString(in.Bucket))
			}
			return &s3.GetBucketPolicyStatusOutput{
				PolicyStatus: &s3types.PolicyStatus{IsPublic: awssdk.Bool(true)},
			}, nil
		},
		getBucketVersioningFunc: func(_ context.Context, in *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
			if awssdk.ToString(in.Bucket) != "public-bucket" {
				t.Fatalf("unexpected bucket for versioning lookup: %q", awssdk.ToString(in.Bucket))
			}
			return &s3.GetBucketVersioningOutput{
				Status: s3types.BucketVersioningStatusSuspended,
			}, nil
		},
	}

	findings, err := runS3InspectorScan(context.Background(), &AwsRepository{S3Client: mock})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 4 {
		t.Fatalf("expected 4 findings, got %d", len(findings))
	}

	if findings[0].RuleID != s3InspectorRuleIDPublicACL || findings[0].Severity != RuleSeverityHigh {
		t.Fatalf("unexpected ACL finding: %+v", findings[0])
	}
	if findings[1].RuleID != s3InspectorRuleIDPublicPolicy || findings[1].Severity != RuleSeverityCritical {
		t.Fatalf("unexpected policy finding: %+v", findings[1])
	}
	if findings[2].RuleID != s3InspectorRuleIDPublicAccess || findings[2].Severity != RuleSeverityHigh {
		t.Fatalf("unexpected public access block finding: %+v", findings[2])
	}
	if findings[3].RuleID != s3InspectorRuleIDVersioning || findings[3].Severity != RuleSeverityMedium {
		t.Fatalf("unexpected versioning finding: %+v", findings[3])
	}
}

func TestRunS3InspectorScan_IgnoresPrivateBuckets(t *testing.T) {
	mock := &mockS3Client{
		listBucketsFunc: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{
					{Name: awssdk.String("private-bucket")},
				},
			}, nil
		},
		getBucketAclFunc: func(_ context.Context, _ *s3.GetBucketAclInput, _ ...func(*s3.Options)) (*s3.GetBucketAclOutput, error) {
			return &s3.GetBucketAclOutput{}, nil
		},
		getPublicAccessBlockFunc: func(_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
			return &s3.GetPublicAccessBlockOutput{
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       awssdk.Bool(true),
					IgnorePublicAcls:      awssdk.Bool(true),
					BlockPublicPolicy:     awssdk.Bool(true),
					RestrictPublicBuckets: awssdk.Bool(true),
				},
			}, nil
		},
		getBucketPolicyStatusFunc: func(_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
			return &s3.GetBucketPolicyStatusOutput{
				PolicyStatus: &s3types.PolicyStatus{IsPublic: awssdk.Bool(false)},
			}, nil
		},
		getBucketVersioningFunc: func(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
			return &s3.GetBucketVersioningOutput{
				Status: s3types.BucketVersioningStatusEnabled,
			}, nil
		},
	}

	findings, err := runS3InspectorScan(context.Background(), &AwsRepository{S3Client: mock})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestInspectS3BucketPublicAccessBlock_FindsMissingConfiguration(t *testing.T) {
	mock := &mockS3Client{
		getPublicAccessBlockFunc: func(_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
			return nil, &smithy.GenericAPIError{
				Code:    "NoSuchPublicAccessBlockConfiguration",
				Message: "bucket has no public access block configuration",
			}
		},
	}

	findings := inspectS3BucketPublicAccessBlock(context.Background(), &AwsRepository{S3Client: mock}, S3Bucket{Name: "bucket-a"})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != s3InspectorRuleIDPublicAccess || findings[0].Severity != RuleSeverityHigh {
		t.Fatalf("unexpected finding: %+v", findings[0])
	}
	for _, want := range []string{"BlockPublicAcls", "IgnorePublicAcls", "BlockPublicPolicy", "RestrictPublicBuckets"} {
		if !strings.Contains(findings[0].Summary, want) {
			t.Fatalf("expected summary to mention %q, got %q", want, findings[0].Summary)
		}
	}
}

func TestInspectS3BucketPublicAccessBlock_IgnoresFullyEnabledBuckets(t *testing.T) {
	mock := &mockS3Client{
		getPublicAccessBlockFunc: func(_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
			return &s3.GetPublicAccessBlockOutput{
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       awssdk.Bool(true),
					IgnorePublicAcls:      awssdk.Bool(true),
					BlockPublicPolicy:     awssdk.Bool(true),
					RestrictPublicBuckets: awssdk.Bool(true),
				},
			}, nil
		},
	}

	findings := inspectS3BucketPublicAccessBlock(context.Background(), &AwsRepository{S3Client: mock}, S3Bucket{Name: "bucket-a"})
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}
