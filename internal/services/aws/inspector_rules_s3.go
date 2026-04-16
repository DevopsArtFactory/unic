package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const (
	s3PublicAllUsersURI           = "http://acs.amazonaws.com/groups/global/AllUsers"
	s3PublicAuthenticatedUsersURI = "http://acs.amazonaws.com/groups/global/AuthenticatedUsers"
	s3InspectorRuleIDPublicACL    = "s3-bucket-public-acl"
	s3InspectorRuleIDPublicPolicy = "s3-bucket-public-policy"
	s3InspectorRuleIDPublicAccess = "s3-bucket-public-access-block-disabled"
	s3InspectorRuleIDVersioning   = "s3-bucket-versioning-disabled"
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
		findings = append(findings, inspectS3BucketPublicAccessBlock(ctx, r, bucket)...)
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
			s3InspectorRuleIDPublicACL,
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
			s3InspectorRuleIDPublicPolicy,
			"S3 bucket policy allows public access",
			RuleSeverityCritical,
			bucket.Name,
			"Bucket policy is public.",
			"Restrict the bucket policy to trusted principals or enable S3 Block Public Access.",
		),
	}
}

func inspectS3BucketPublicAccessBlock(ctx context.Context, r *AwsRepository, bucket S3Bucket) []SecurityFinding {
	output, err := r.S3Client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
		Bucket: awssdk.String(bucket.Name),
	})
	if err != nil && !isMissingBucketPublicAccessBlockConfiguration(err) {
		return nil
	}

	missing := missingBucketPublicAccessBlockSettings(nil)
	if err == nil && output != nil {
		missing = missingBucketPublicAccessBlockSettings(output.PublicAccessBlockConfiguration)
	}
	if len(missing) == 0 {
		return nil
	}

	return []SecurityFinding{
		newS3InspectorFinding(
			s3InspectorRuleIDPublicAccess,
			"S3 bucket Block Public Access is not fully enabled",
			RuleSeverityHigh,
			bucket.Name,
			fmt.Sprintf("Bucket-level Block Public Access is missing settings: %s.", strings.Join(missing, ", ")),
			"Enable all four bucket-level S3 Block Public Access settings: BlockPublicAcls, IgnorePublicAcls, BlockPublicPolicy, RestrictPublicBuckets.",
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
			s3InspectorRuleIDVersioning,
			"S3 bucket versioning is disabled",
			RuleSeverityMedium,
			bucket.Name,
			"Bucket versioning is not enabled.",
			"Enable bucket versioning to protect against accidental deletion and overwrites.",
		),
	}
}

func missingBucketPublicAccessBlockSettings(cfg *s3types.PublicAccessBlockConfiguration) []string {
	missing := make([]string, 0, 4)
	if cfg == nil || !awssdk.ToBool(cfg.BlockPublicAcls) {
		missing = append(missing, "BlockPublicAcls")
	}
	if cfg == nil || !awssdk.ToBool(cfg.IgnorePublicAcls) {
		missing = append(missing, "IgnorePublicAcls")
	}
	if cfg == nil || !awssdk.ToBool(cfg.BlockPublicPolicy) {
		missing = append(missing, "BlockPublicPolicy")
	}
	if cfg == nil || !awssdk.ToBool(cfg.RestrictPublicBuckets) {
		missing = append(missing, "RestrictPublicBuckets")
	}
	return missing
}

func isMissingBucketPublicAccessBlockConfiguration(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchPublicAccessBlockConfiguration" {
		return true
	}

	text := strings.ToLower(err.Error())
	return strings.Contains(text, "nosuchpublicaccessblockconfiguration") ||
		strings.Contains(text, "no such public access block configuration")
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
