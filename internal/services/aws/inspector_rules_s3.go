package aws

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	s3PublicAllUsersURI           = "http://acs.amazonaws.com/groups/global/AllUsers"
	s3PublicAuthenticatedUsersURI = "http://acs.amazonaws.com/groups/global/AuthenticatedUsers"
)

func init() {
	registerSecurityInspectorScanner(InspectorScanner{
		Name: "s3",
		Run:  runS3InspectorScan,
	})
}

func runS3InspectorScan(ctx context.Context, r *AwsRepository) ([]SecurityFinding, error) {
	buckets, err := r.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list S3 buckets: %w", err)
	}

	findings := make([]SecurityFinding, 0)
	for _, bucket := range buckets {
		findings = append(findings, inspectS3BucketPublicACL(ctx, r, bucket)...)
		findings = append(findings, inspectS3BucketPolicy(ctx, r, bucket)...)
		findings = append(findings, inspectS3BucketVersioning(ctx, r, bucket)...)
	}

	return findings, nil
}

func inspectS3BucketPublicACL(ctx context.Context, r *AwsRepository, bucket S3Bucket) []SecurityFinding {
	output, err := r.S3Client.GetBucketAcl(ctx, &s3.GetBucketAclInput{
		Bucket: awssdk.String(bucket.Name),
	})
	if err != nil || output == nil {
		return nil
	}
	if !bucketAclIsPublic(output) {
		return nil
	}

	return []SecurityFinding{
		newS3InspectorFinding(
			"s3-bucket-public-acl",
			"S3 bucket ACL grants public access",
			RuleSeverityHigh,
			bucket.Name,
			"Bucket ACL grants access to a public S3 group.",
			"Remove the public ACL grant or enable S3 Block Public Access for the bucket.",
		),
	}
}

func inspectS3BucketPolicy(ctx context.Context, r *AwsRepository, bucket S3Bucket) []SecurityFinding {
	output, err := r.S3Client.GetBucketPolicyStatus(ctx, &s3.GetBucketPolicyStatusInput{
		Bucket: awssdk.String(bucket.Name),
	})
	if err != nil || output == nil || output.PolicyStatus == nil {
		return nil
	}
	if !awssdk.ToBool(output.PolicyStatus.IsPublic) {
		return nil
	}

	return []SecurityFinding{
		newS3InspectorFinding(
			"s3-bucket-public-policy",
			"S3 bucket policy allows public access",
			RuleSeverityCritical,
			bucket.Name,
			"Bucket policy is public.",
			"Restrict the bucket policy to trusted principals or enable S3 Block Public Access.",
		),
	}
}

func inspectS3BucketVersioning(ctx context.Context, r *AwsRepository, bucket S3Bucket) []SecurityFinding {
	output, err := r.S3Client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: awssdk.String(bucket.Name),
	})
	if err != nil || output == nil {
		return nil
	}
	if output.Status == s3types.BucketVersioningStatusEnabled {
		return nil
	}

	return []SecurityFinding{
		newS3InspectorFinding(
			"s3-bucket-versioning-disabled",
			"S3 bucket versioning is disabled",
			RuleSeverityMedium,
			bucket.Name,
			"Bucket versioning is not enabled.",
			"Enable bucket versioning to protect against accidental deletion and overwrites.",
		),
	}
}

func bucketAclIsPublic(output *s3.GetBucketAclOutput) bool {
	for _, grant := range output.Grants {
		if grant.Grantee == nil || grant.Grantee.Type != s3types.TypeGroup || grant.Grantee.URI == nil {
			continue
		}
		switch awssdk.ToString(grant.Grantee.URI) {
		case s3PublicAllUsersURI, s3PublicAuthenticatedUsersURI:
			return true
		}
	}
	return false
}

func newS3InspectorFinding(ruleID, ruleName string, severity RuleSeverity, resourceID, summary, recommendation string) SecurityFinding {
	return SecurityFinding{
		RuleID:         ruleID,
		RuleName:       ruleName,
		Severity:       severity,
		ResourceType:   "S3 Bucket",
		ResourceID:     resourceID,
		Summary:        summary,
		Recommendation: recommendation,
	}
}
