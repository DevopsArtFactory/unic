package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
)

const (
	inspectorScannerElastiCacheValkeyName = "elasticache-valkey-security"

	inspectorRuleIDValkeyTransitEncryptionDisabled = "elasticache-valkey-transit-encryption-disabled"
	inspectorRuleIDValkeyAtRestEncryptionDisabled  = "elasticache-valkey-at-rest-encryption-disabled"
	inspectorRuleIDValkeyBackupsDisabled           = "elasticache-valkey-backups-disabled"
	inspectorRuleIDValkeyBackupRetentionLow        = "elasticache-valkey-backup-retention-low"
	inspectorRuleIDValkeyWeakAccessControl         = "elasticache-valkey-weak-access-control"

	inspectorValkeyMinSnapshotRetentionDays = 7
)

func init() {
	registerSecurityInspectorScanner(InspectorScanner{
		Name: inspectorScannerElastiCacheValkeyName,
		Run:  runElastiCacheValkeySecurityScan,
	})
}

func runElastiCacheValkeySecurityScan(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
	var findings []SecurityFinding
	marker := ""

	for {
		input := &elasticache.DescribeReplicationGroupsInput{
			MaxRecords: awssdk.Int32(100),
		}
		if marker != "" {
			input.Marker = awssdk.String(marker)
		}

		page, err := repo.ElastiCacheClient.DescribeReplicationGroups(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect ElastiCache replication groups: %w", err)
		}

		for _, group := range page.ReplicationGroups {
			if !strings.EqualFold(awssdk.ToString(group.Engine), "valkey") {
				continue
			}
			findings = append(findings, inspectValkeyReplicationGroup(group)...)
		}

		if awssdk.ToString(page.Marker) == "" {
			break
		}
		marker = awssdk.ToString(page.Marker)
	}

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

func inspectValkeyReplicationGroup(group elasticachetypes.ReplicationGroup) []SecurityFinding {
	groupID := awssdk.ToString(group.ReplicationGroupId)
	if groupID == "" {
		groupID = awssdk.ToString(group.ARN)
	}
	var findings []SecurityFinding

	if !awssdk.ToBool(group.TransitEncryptionEnabled) {
		findings = append(findings, SecurityFinding{
			RuleID:         inspectorRuleIDValkeyTransitEncryptionDisabled,
			RuleName:       "Valkey in-transit encryption disabled",
			Severity:       RuleSeverityHigh,
			ResourceType:   "ElastiCacheReplicationGroup",
			ResourceID:     groupID,
			Summary:        fmt.Sprintf("Valkey replication group %s does not have in-transit encryption enabled.", groupID),
			Recommendation: "Enable TLS in transit for Valkey replication groups so client and node traffic is encrypted.",
		})
	}

	if !awssdk.ToBool(group.AtRestEncryptionEnabled) {
		findings = append(findings, SecurityFinding{
			RuleID:         inspectorRuleIDValkeyAtRestEncryptionDisabled,
			RuleName:       "Valkey at-rest encryption disabled",
			Severity:       RuleSeverityHigh,
			ResourceType:   "ElastiCacheReplicationGroup",
			ResourceID:     groupID,
			Summary:        fmt.Sprintf("Valkey replication group %s does not have at-rest encryption enabled.", groupID),
			Recommendation: "Enable at-rest encryption for Valkey replication groups so snapshot and disk-backed data is protected.",
		})
	}

	snapshotRetention := awssdk.ToInt32(group.SnapshotRetentionLimit)
	switch {
	case snapshotRetention <= 0:
		findings = append(findings, SecurityFinding{
			RuleID:         inspectorRuleIDValkeyBackupsDisabled,
			RuleName:       "Valkey automatic backups disabled",
			Severity:       RuleSeverityHigh,
			ResourceType:   "ElastiCacheReplicationGroup",
			ResourceID:     groupID,
			Summary:        fmt.Sprintf("Valkey replication group %s has automatic backups disabled.", groupID),
			Recommendation: "Enable automatic backups and retain snapshots long enough to support recovery expectations.",
		})
	case snapshotRetention < inspectorValkeyMinSnapshotRetentionDays:
		findings = append(findings, SecurityFinding{
			RuleID:         inspectorRuleIDValkeyBackupRetentionLow,
			RuleName:       "Valkey backup retention too low",
			Severity:       RuleSeverityMedium,
			ResourceType:   "ElastiCacheReplicationGroup",
			ResourceID:     groupID,
			Summary:        fmt.Sprintf("Valkey replication group %s retains automatic backups for only %d days.", groupID, snapshotRetention),
			Recommendation: fmt.Sprintf("Increase snapshot retention to at least %d days unless a shorter retention period is explicitly justified.", inspectorValkeyMinSnapshotRetentionDays),
		})
	}

	if len(group.UserGroupIds) == 0 {
		severity := RuleSeverityLow
		summary := fmt.Sprintf("Valkey replication group %s does not use RBAC user groups.", groupID)
		if awssdk.ToBool(group.AuthTokenEnabled) {
			severity = RuleSeverityMedium
			summary = fmt.Sprintf("Valkey replication group %s relies on shared AUTH tokens without RBAC user groups.", groupID)
		}

		findings = append(findings, SecurityFinding{
			RuleID:         inspectorRuleIDValkeyWeakAccessControl,
			RuleName:       "Valkey access control posture is weak",
			Severity:       severity,
			ResourceType:   "ElastiCacheReplicationGroup",
			ResourceID:     groupID,
			Summary:        summary,
			Recommendation: "Prefer RBAC user groups and IAM authentication where supported instead of long-lived shared AUTH credentials.",
		})
	}

	return findings
}
