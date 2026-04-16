package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	inspectorScannerSnapshotExposureName = "snapshot-exposure"

	inspectorRuleIDRDSDBSnapshotPublic      = "rds-db-snapshot-public"
	inspectorRuleIDRDSClusterSnapshotPublic = "rds-cluster-snapshot-public"
	inspectorRuleIDEBSSnapshotPublic        = "ebs-snapshot-public"
	rdsManualSnapshotType                   = "manual"
)

func init() {
	registerSecurityInspectorScanner(InspectorScanner{
		Name: inspectorScannerSnapshotExposureName,
		Run:  runSnapshotExposureScan,
	})
}

func runSnapshotExposureScan(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
	rdsFindings, err := inspectPublicRDSSnapshots(ctx, repo.RDSClient)
	if err != nil {
		return nil, err
	}

	rdsClusterFindings, err := inspectPublicRDSClusterSnapshots(ctx, repo.RDSClient)
	if err != nil {
		return nil, err
	}

	ebsFindings, err := inspectPublicEBSSnapshots(ctx, repo.EC2Client)
	if err != nil {
		return nil, err
	}

	findings := append(rdsFindings, rdsClusterFindings...)
	findings = append(findings, ebsFindings...)

	sort.Slice(findings, func(i, j int) bool {
		left := normalizedSortKey(findings[i].ResourceID, findings[i].RuleID, findings[i].RuleName)
		right := normalizedSortKey(findings[j].ResourceID, findings[j].RuleID, findings[j].RuleName)
		if left == right {
			return findings[i].Severity.Rank() < findings[j].Severity.Rank()
		}
		return left < right
	})

	return findings, nil
}

func inspectPublicRDSSnapshots(ctx context.Context, client RDSClientAPI) ([]SecurityFinding, error) {
	var findings []SecurityFinding
	marker := ""

	for {
		input := &rds.DescribeDBSnapshotsInput{
			SnapshotType: awssdk.String(rdsManualSnapshotType),
		}
		if marker != "" {
			input.Marker = awssdk.String(marker)
		}

		page, err := client.DescribeDBSnapshots(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect DB snapshots: %w", err)
		}

		for _, snapshot := range page.DBSnapshots {
			snapshotID := awssdk.ToString(snapshot.DBSnapshotIdentifier)
			if snapshotID == "" {
				continue
			}

			attributes, err := client.DescribeDBSnapshotAttributes(ctx, &rds.DescribeDBSnapshotAttributesInput{
				DBSnapshotIdentifier: snapshot.DBSnapshotIdentifier,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to inspect DB snapshot attributes for %s: %w", snapshotID, err)
			}

			if attributes.DBSnapshotAttributesResult == nil || !isPublicRDSRestoreAttribute(attributes.DBSnapshotAttributesResult.DBSnapshotAttributes) {
				continue
			}

			findings = append(findings, SecurityFinding{
				RuleID:       inspectorRuleIDRDSDBSnapshotPublic,
				RuleName:     "RDS DB snapshot publicly shared",
				Severity:     RuleSeverityHigh,
				ResourceType: "RDSSnapshot",
				ResourceID:   snapshotID,
				Summary: fmt.Sprintf(
					"Manual DB snapshot %s from %s is publicly restorable.",
					snapshotID,
					awssdk.ToString(snapshot.DBInstanceIdentifier),
				),
				Recommendation: "Remove the public restore permission from the snapshot and share it only with specific AWS accounts when needed.",
			})
		}

		if awssdk.ToString(page.Marker) == "" {
			break
		}
		marker = awssdk.ToString(page.Marker)
	}

	return findings, nil
}

func inspectPublicRDSClusterSnapshots(ctx context.Context, client RDSClientAPI) ([]SecurityFinding, error) {
	var findings []SecurityFinding
	marker := ""

	for {
		input := &rds.DescribeDBClusterSnapshotsInput{
			SnapshotType: awssdk.String(rdsManualSnapshotType),
		}
		if marker != "" {
			input.Marker = awssdk.String(marker)
		}

		page, err := client.DescribeDBClusterSnapshots(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect DB cluster snapshots: %w", err)
		}

		for _, snapshot := range page.DBClusterSnapshots {
			snapshotID := awssdk.ToString(snapshot.DBClusterSnapshotIdentifier)
			if snapshotID == "" {
				continue
			}

			attributes, err := client.DescribeDBClusterSnapshotAttributes(ctx, &rds.DescribeDBClusterSnapshotAttributesInput{
				DBClusterSnapshotIdentifier: snapshot.DBClusterSnapshotIdentifier,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to inspect DB cluster snapshot attributes for %s: %w", snapshotID, err)
			}

			if attributes.DBClusterSnapshotAttributesResult == nil || !isPublicRDSClusterRestoreAttribute(attributes.DBClusterSnapshotAttributesResult.DBClusterSnapshotAttributes) {
				continue
			}

			findings = append(findings, SecurityFinding{
				RuleID:       inspectorRuleIDRDSClusterSnapshotPublic,
				RuleName:     "RDS cluster snapshot publicly shared",
				Severity:     RuleSeverityHigh,
				ResourceType: "RDSClusterSnapshot",
				ResourceID:   snapshotID,
				Summary: fmt.Sprintf(
					"Manual DB cluster snapshot %s from %s is publicly restorable.",
					snapshotID,
					awssdk.ToString(snapshot.DBClusterIdentifier),
				),
				Recommendation: "Remove the public restore permission from the cluster snapshot and share it only with explicitly authorized AWS accounts.",
			})
		}

		if awssdk.ToString(page.Marker) == "" {
			break
		}
		marker = awssdk.ToString(page.Marker)
	}

	return findings, nil
}

func inspectPublicEBSSnapshots(ctx context.Context, client EC2ClientAPI) ([]SecurityFinding, error) {
	var findings []SecurityFinding
	nextToken := ""

	for {
		input := &ec2.DescribeSnapshotsInput{
			OwnerIds: []string{"self"},
		}
		if nextToken != "" {
			input.NextToken = awssdk.String(nextToken)
		}

		page, err := client.DescribeSnapshots(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect EBS snapshots: %w", err)
		}

		for _, snapshot := range page.Snapshots {
			snapshotID := awssdk.ToString(snapshot.SnapshotId)
			if snapshotID == "" {
				continue
			}

			attributes, err := client.DescribeSnapshotAttribute(ctx, &ec2.DescribeSnapshotAttributeInput{
				Attribute:  ec2types.SnapshotAttributeNameCreateVolumePermission,
				SnapshotId: snapshot.SnapshotId,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to inspect EBS snapshot permissions for %s: %w", snapshotID, err)
			}

			if !isPublicCreateVolumePermission(attributes.CreateVolumePermissions) {
				continue
			}

			findings = append(findings, SecurityFinding{
				RuleID:       inspectorRuleIDEBSSnapshotPublic,
				RuleName:     "EBS snapshot publicly shared",
				Severity:     RuleSeverityHigh,
				ResourceType: "EBSSnapshot",
				ResourceID:   snapshotID,
				Summary: fmt.Sprintf(
					"EBS snapshot %s for volume %s grants public create-volume permissions.",
					snapshotID,
					awssdk.ToString(snapshot.VolumeId),
				),
				Recommendation: "Remove the public create-volume permission and share the snapshot only with explicitly authorized AWS accounts.",
			})
		}

		if awssdk.ToString(page.NextToken) == "" {
			break
		}
		nextToken = awssdk.ToString(page.NextToken)
	}

	return findings, nil
}

func isPublicRDSRestoreAttribute(attributes []rdstypes.DBSnapshotAttribute) bool {
	for _, attribute := range attributes {
		if !strings.EqualFold(awssdk.ToString(attribute.AttributeName), "restore") {
			continue
		}
		for _, value := range attribute.AttributeValues {
			if strings.EqualFold(value, "all") {
				return true
			}
		}
	}
	return false
}

func isPublicRDSClusterRestoreAttribute(attributes []rdstypes.DBClusterSnapshotAttribute) bool {
	for _, attribute := range attributes {
		if !strings.EqualFold(awssdk.ToString(attribute.AttributeName), "restore") {
			continue
		}
		for _, value := range attribute.AttributeValues {
			if strings.EqualFold(value, "all") {
				return true
			}
		}
	}
	return false
}

func isPublicCreateVolumePermission(permissions []ec2types.CreateVolumePermission) bool {
	for _, permission := range permissions {
		if strings.EqualFold(string(permission.Group), "all") {
			return true
		}
	}
	return false
}
