package inspector

import (
	"context"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

func TestRunSnapshotExposureScan_FindsPublicSnapshots(t *testing.T) {
	mockRDS := &mockRDSClient{
		describeDBSnapshotsFunc: func(context.Context, *rds.DescribeDBSnapshotsInput, ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
			return &rds.DescribeDBSnapshotsOutput{
				DBSnapshots: []rdstypes.DBSnapshot{
					{
						DBSnapshotIdentifier: awssdk.String("db-snap-public"),
						DBInstanceIdentifier: awssdk.String("app-db"),
					},
				},
			}, nil
		},
		describeDBSnapshotAttributesFunc: func(_ context.Context, params *rds.DescribeDBSnapshotAttributesInput, _ ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error) {
			if awssdk.ToString(params.DBSnapshotIdentifier) != "db-snap-public" {
				t.Fatalf("unexpected DB snapshot identifier %q", awssdk.ToString(params.DBSnapshotIdentifier))
			}
			return &rds.DescribeDBSnapshotAttributesOutput{
				DBSnapshotAttributesResult: &rdstypes.DBSnapshotAttributesResult{
					DBSnapshotAttributes: []rdstypes.DBSnapshotAttribute{
						{
							AttributeName:   awssdk.String("restore"),
							AttributeValues: []string{"all"},
						},
					},
				},
			}, nil
		},
		describeDBClusterSnapshotsFunc: func(context.Context, *rds.DescribeDBClusterSnapshotsInput, ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error) {
			return &rds.DescribeDBClusterSnapshotsOutput{
				DBClusterSnapshots: []rdstypes.DBClusterSnapshot{
					{
						DBClusterSnapshotIdentifier: awssdk.String("cluster-snap-public"),
						DBClusterIdentifier:         awssdk.String("aurora-prod"),
					},
				},
			}, nil
		},
		describeDBClusterSnapshotAttrsFunc: func(_ context.Context, params *rds.DescribeDBClusterSnapshotAttributesInput, _ ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error) {
			if awssdk.ToString(params.DBClusterSnapshotIdentifier) != "cluster-snap-public" {
				t.Fatalf("unexpected DB cluster snapshot identifier %q", awssdk.ToString(params.DBClusterSnapshotIdentifier))
			}
			return &rds.DescribeDBClusterSnapshotAttributesOutput{
				DBClusterSnapshotAttributesResult: &rdstypes.DBClusterSnapshotAttributesResult{
					DBClusterSnapshotAttributes: []rdstypes.DBClusterSnapshotAttribute{
						{
							AttributeName:   awssdk.String("restore"),
							AttributeValues: []string{"all"},
						},
					},
				},
			}, nil
		},
	}

	mockEC2 := &mockEC2Client{
		describeSnapshotsFunc: func(context.Context, *ec2.DescribeSnapshotsInput, ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
			return &ec2.DescribeSnapshotsOutput{
				Snapshots: []ec2types.Snapshot{
					{
						SnapshotId: awssdk.String("snap-public"),
						VolumeId:   awssdk.String("vol-123"),
					},
				},
			}, nil
		},
		describeSnapshotAttributeFunc: func(_ context.Context, params *ec2.DescribeSnapshotAttributeInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotAttributeOutput, error) {
			if awssdk.ToString(params.SnapshotId) != "snap-public" {
				t.Fatalf("unexpected EBS snapshot identifier %q", awssdk.ToString(params.SnapshotId))
			}
			return &ec2.DescribeSnapshotAttributeOutput{
				CreateVolumePermissions: []ec2types.CreateVolumePermission{
					{Group: ec2types.PermissionGroup("all")},
				},
			}, nil
		},
	}

	findings, err := runSnapshotExposureScan(context.Background(), &AwsRepository{
		RDSClient: mockRDS,
		EC2Client: mockEC2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 3 {
		t.Fatalf("expected 3 public-snapshot findings, got %d", len(findings))
	}

	expectedRuleIDs := map[string]bool{
		inspectorRuleIDRDSDBSnapshotPublic:      false,
		inspectorRuleIDRDSClusterSnapshotPublic: false,
		inspectorRuleIDEBSSnapshotPublic:        false,
	}
	for _, finding := range findings {
		expectedRuleIDs[finding.RuleID] = true
		if finding.Severity != RuleSeverityHigh {
			t.Fatalf("expected high severity for public snapshot finding, got %+v", finding)
		}
	}
	for ruleID, seen := range expectedRuleIDs {
		if !seen {
			t.Fatalf("missing snapshot finding for %s: %+v", ruleID, findings)
		}
	}
}

func TestRunSnapshotExposureScan_IgnoresPrivateSnapshots(t *testing.T) {
	mockRDS := &mockRDSClient{
		describeDBSnapshotsFunc: func(context.Context, *rds.DescribeDBSnapshotsInput, ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
			return &rds.DescribeDBSnapshotsOutput{
				DBSnapshots: []rdstypes.DBSnapshot{{DBSnapshotIdentifier: awssdk.String("db-snap-private")}},
			}, nil
		},
		describeDBSnapshotAttributesFunc: func(context.Context, *rds.DescribeDBSnapshotAttributesInput, ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error) {
			return &rds.DescribeDBSnapshotAttributesOutput{
				DBSnapshotAttributesResult: &rdstypes.DBSnapshotAttributesResult{
					DBSnapshotAttributes: []rdstypes.DBSnapshotAttribute{
						{
							AttributeName:   awssdk.String("restore"),
							AttributeValues: []string{"123456789012"},
						},
					},
				},
			}, nil
		},
		describeDBClusterSnapshotsFunc: func(context.Context, *rds.DescribeDBClusterSnapshotsInput, ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error) {
			return &rds.DescribeDBClusterSnapshotsOutput{
				DBClusterSnapshots: []rdstypes.DBClusterSnapshot{{DBClusterSnapshotIdentifier: awssdk.String("cluster-snap-private")}},
			}, nil
		},
		describeDBClusterSnapshotAttrsFunc: func(context.Context, *rds.DescribeDBClusterSnapshotAttributesInput, ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error) {
			return &rds.DescribeDBClusterSnapshotAttributesOutput{
				DBClusterSnapshotAttributesResult: &rdstypes.DBClusterSnapshotAttributesResult{
					DBClusterSnapshotAttributes: []rdstypes.DBClusterSnapshotAttribute{
						{
							AttributeName:   awssdk.String("restore"),
							AttributeValues: []string{"123456789012"},
						},
					},
				},
			}, nil
		},
	}

	mockEC2 := &mockEC2Client{
		describeSnapshotsFunc: func(context.Context, *ec2.DescribeSnapshotsInput, ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
			return &ec2.DescribeSnapshotsOutput{
				Snapshots: []ec2types.Snapshot{{SnapshotId: awssdk.String("snap-private")}},
			}, nil
		},
		describeSnapshotAttributeFunc: func(context.Context, *ec2.DescribeSnapshotAttributeInput, ...func(*ec2.Options)) (*ec2.DescribeSnapshotAttributeOutput, error) {
			return &ec2.DescribeSnapshotAttributeOutput{
				CreateVolumePermissions: []ec2types.CreateVolumePermission{
					{UserId: awssdk.String("123456789012")},
				},
			}, nil
		},
	}

	findings, err := runSnapshotExposureScan(context.Background(), &AwsRepository{
		RDSClient: mockRDS,
		EC2Client: mockEC2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings for private snapshots, got %+v", findings)
	}
}
