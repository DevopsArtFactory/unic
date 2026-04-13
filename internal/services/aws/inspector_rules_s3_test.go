package aws

import (
	"context"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestRunS3InspectorScan_FindsPublicACLPolicyAndVersioning(t *testing.T) {
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
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(findings))
	}

	if findings[0].RuleID != "s3-bucket-public-acl" || findings[0].Severity != RuleSeverityHigh {
		t.Fatalf("unexpected ACL finding: %+v", findings[0])
	}
	if findings[1].RuleID != "s3-bucket-public-policy" || findings[1].Severity != RuleSeverityCritical {
		t.Fatalf("unexpected policy finding: %+v", findings[1])
	}
	if findings[2].RuleID != "s3-bucket-versioning-disabled" || findings[2].Severity != RuleSeverityMedium {
		t.Fatalf("unexpected versioning finding: %+v", findings[2])
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
