package inspector

import (
	"context"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
)

type mockElastiCacheClient struct {
	describeReplicationGroupsFunc func(ctx context.Context, params *elasticache.DescribeReplicationGroupsInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error)
}

func (m *mockElastiCacheClient) DescribeReplicationGroups(ctx context.Context, params *elasticache.DescribeReplicationGroupsInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
	if m.describeReplicationGroupsFunc != nil {
		return m.describeReplicationGroupsFunc(ctx, params, optFns...)
	}
	return &elasticache.DescribeReplicationGroupsOutput{}, nil
}

func TestRunElastiCacheValkeySecurityScan_FindsSecurityGaps(t *testing.T) {
	mock := &mockElastiCacheClient{
		describeReplicationGroupsFunc: func(context.Context, *elasticache.DescribeReplicationGroupsInput, ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
			return &elasticache.DescribeReplicationGroupsOutput{
				ReplicationGroups: []elasticachetypes.ReplicationGroup{
					{
						ReplicationGroupId:       awssdk.String("valkey-prod"),
						Engine:                   awssdk.String("valkey"),
						TransitEncryptionEnabled: awssdk.Bool(false),
						AtRestEncryptionEnabled:  awssdk.Bool(false),
						SnapshotRetentionLimit:   awssdk.Int32(0),
						AuthTokenEnabled:         awssdk.Bool(true),
						UserGroupIds:             nil,
					},
					{
						ReplicationGroupId:       awssdk.String("redis-legacy"),
						Engine:                   awssdk.String("redis"),
						TransitEncryptionEnabled: awssdk.Bool(false),
					},
				},
			}, nil
		},
	}

	findings, err := runElastiCacheValkeySecurityScan(context.Background(), &AwsRepository{
		ElastiCacheClient: mock,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 4 {
		t.Fatalf("expected 4 Valkey findings, got %d", len(findings))
	}

	ruleIDs := map[string]bool{}
	for _, finding := range findings {
		ruleIDs[finding.RuleID] = true
		if finding.ResourceID != "valkey-prod" {
			t.Fatalf("expected only Valkey replication-group findings, got %+v", finding)
		}
	}
	if !ruleIDs[inspectorRuleIDValkeyTransitEncryptionDisabled] ||
		!ruleIDs[inspectorRuleIDValkeyAtRestEncryptionDisabled] ||
		!ruleIDs[inspectorRuleIDValkeyBackupsDisabled] ||
		!ruleIDs[inspectorRuleIDValkeyWeakAccessControl] {
		t.Fatalf("missing expected Valkey findings: %+v", findings)
	}
}

func TestRunElastiCacheValkeySecurityScan_IgnoresCompliantReplicationGroups(t *testing.T) {
	mock := &mockElastiCacheClient{
		describeReplicationGroupsFunc: func(context.Context, *elasticache.DescribeReplicationGroupsInput, ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
			return &elasticache.DescribeReplicationGroupsOutput{
				ReplicationGroups: []elasticachetypes.ReplicationGroup{
					{
						ReplicationGroupId:       awssdk.String("valkey-secure"),
						Engine:                   awssdk.String("valkey"),
						TransitEncryptionEnabled: awssdk.Bool(true),
						AtRestEncryptionEnabled:  awssdk.Bool(true),
						SnapshotRetentionLimit:   awssdk.Int32(inspectorValkeyMinSnapshotRetentionDays),
						UserGroupIds:             []string{"rg-123"},
					},
				},
			}, nil
		},
	}

	findings, err := runElastiCacheValkeySecurityScan(context.Background(), &AwsRepository{
		ElastiCacheClient: mock,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings for compliant Valkey replication group, got %+v", findings)
	}
}
