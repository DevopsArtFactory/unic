package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	uniclog "unic/internal/log"
)

// ListBackupVaults returns vaults in the active region. Completed pages remain
// available when a later page fails.
func (r *AwsRepository) ListBackupVaults(ctx context.Context) ([]BackupVault, []error, error) {
	uniclog.Debug("aws", "ListBackupVaults called")

	var vaults []BackupVault
	var warnings []error
	var nextToken *string
	for {
		out, err := r.BackupClient.ListBackupVaults(ctx, &backup.ListBackupVaultsInput{NextToken: nextToken})
		if err != nil {
			wrapped := fmt.Errorf("failed to list AWS Backup vaults: %w", err)
			if len(vaults) == 0 {
				return nil, nil, wrapped
			}
			warnings = append(warnings, wrapped)
			break
		}
		for _, vault := range out.BackupVaultList {
			vaults = append(vaults, mapBackupVault(vault, r.Region))
		}
		if awssdk.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}

	sort.Slice(vaults, func(i, j int) bool {
		left, right := normalizedSortKey(vaults[i].Name), normalizedSortKey(vaults[j].Name)
		if left != right {
			return left < right
		}
		return vaults[i].ARN < vaults[j].ARN
	})
	return vaults, warnings, nil
}

// GetBackupVaultDetail loads independent recovery point, protected resource,
// and recent failed-job sections. A denied section becomes a warning without
// hiding the others.
func (r *AwsRepository) GetBackupVaultDetail(ctx context.Context, vault BackupVault) (*BackupVaultDetail, []error, error) {
	detail := &BackupVaultDetail{Vault: vault}
	var warnings []error

	recoveryPoints, err := r.listBackupRecoveryPoints(ctx, vault.Name)
	if err != nil {
		warnings = append(warnings, err)
	}
	detail.RecoveryPoints = recoveryPoints
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	resources, err := r.listBackupProtectedResources(ctx, vault.Name)
	if err != nil {
		warnings = append(warnings, err)
	}
	detail.ProtectedResources = resources
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	jobs, err := r.listFailedBackupJobs(ctx, vault.Name)
	if err != nil {
		warnings = append(warnings, err)
	}
	detail.FailedJobs = jobs

	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return detail, warnings, nil
}

func (r *AwsRepository) listBackupRecoveryPoints(ctx context.Context, vaultName string) ([]BackupRecoveryPoint, error) {
	var items []BackupRecoveryPoint
	var nextToken *string
	for {
		out, err := r.BackupClient.ListRecoveryPointsByBackupVault(ctx, &backup.ListRecoveryPointsByBackupVaultInput{
			BackupVaultName: awssdk.String(vaultName), NextToken: nextToken,
		})
		if err != nil {
			sortBackupRecoveryPoints(items)
			return items, fmt.Errorf("failed to list recovery points for backup vault %s: %w", vaultName, err)
		}
		for _, item := range out.RecoveryPoints {
			items = append(items, mapBackupRecoveryPoint(item))
		}
		if awssdk.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}
	sortBackupRecoveryPoints(items)
	return items, nil
}

func (r *AwsRepository) listBackupProtectedResources(ctx context.Context, vaultName string) ([]BackupProtectedResource, error) {
	var items []BackupProtectedResource
	var nextToken *string
	for {
		out, err := r.BackupClient.ListProtectedResourcesByBackupVault(ctx, &backup.ListProtectedResourcesByBackupVaultInput{
			BackupVaultName: awssdk.String(vaultName), NextToken: nextToken,
		})
		if err != nil {
			sortBackupProtectedResources(items)
			return items, fmt.Errorf("failed to list protected resources for backup vault %s: %w", vaultName, err)
		}
		for _, item := range out.Results {
			items = append(items, mapBackupProtectedResource(item))
		}
		if awssdk.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}
	sortBackupProtectedResources(items)
	return items, nil
}

func (r *AwsRepository) listFailedBackupJobs(ctx context.Context, vaultName string) ([]BackupJob, error) {
	var items []BackupJob
	var nextToken *string
	for {
		out, err := r.BackupClient.ListBackupJobs(ctx, &backup.ListBackupJobsInput{
			ByBackupVaultName: awssdk.String(vaultName), NextToken: nextToken,
		})
		if err != nil {
			sortBackupJobs(items)
			return items, fmt.Errorf("failed to list recent jobs for backup vault %s: %w", vaultName, err)
		}
		for _, item := range out.BackupJobs {
			if !backupJobNeedsAttention(string(item.State), awssdk.ToString(item.MessageCategory)) {
				continue
			}
			items = append(items, mapBackupJob(item))
		}
		if awssdk.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}
	sortBackupJobs(items)
	return items, nil
}

func sortBackupProtectedResources(items []BackupProtectedResource) {
	sort.Slice(items, func(i, j int) bool {
		if !items[i].LastBackupAt.Equal(items[j].LastBackupAt) {
			return items[i].LastBackupAt.After(items[j].LastBackupAt)
		}
		left, right := normalizedSortKey(items[i].Name), normalizedSortKey(items[j].Name)
		if left != right {
			return left < right
		}
		return normalizedSortKey(items[i].ARN) < normalizedSortKey(items[j].ARN)
	})
}

func sortBackupJobs(items []BackupJob) {
	sort.SliceStable(items, func(i, j int) bool {
		left, right := backupJobStatusRank(items[i].State), backupJobStatusRank(items[j].State)
		if left != right {
			return left < right
		}
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return normalizedSortKey(items[i].ID) < normalizedSortKey(items[j].ID)
	})
}

func mapBackupVault(item backuptypes.BackupVaultListMember, region string) BackupVault {
	vault := BackupVault{
		ARN:                awssdk.ToString(item.BackupVaultArn),
		Name:               awssdk.ToString(item.BackupVaultName),
		Region:             region,
		State:              string(item.VaultState),
		Type:               string(item.VaultType),
		EncryptionKeyARN:   awssdk.ToString(item.EncryptionKeyArn),
		EncryptionKeyType:  string(item.EncryptionKeyType),
		RecoveryPointCount: item.NumberOfRecoveryPoints,
		Locked:             awssdk.ToBool(item.Locked),
		MinRetentionDays:   awssdk.ToInt64(item.MinRetentionDays),
		MinRetentionKnown:  item.MinRetentionDays != nil,
		MaxRetentionDays:   awssdk.ToInt64(item.MaxRetentionDays),
		MaxRetentionKnown:  item.MaxRetentionDays != nil,
	}
	if item.CreationDate != nil {
		vault.CreatedAt = *item.CreationDate
	}
	if item.LockDate != nil {
		vault.LockDate = *item.LockDate
	}
	return vault
}

func mapBackupRecoveryPoint(item backuptypes.RecoveryPointByBackupVault) BackupRecoveryPoint {
	mapped := BackupRecoveryPoint{
		ARN:            awssdk.ToString(item.RecoveryPointArn),
		ResourceARN:    awssdk.ToString(item.ResourceArn),
		ResourceName:   awssdk.ToString(item.ResourceName),
		ResourceType:   awssdk.ToString(item.ResourceType),
		Status:         string(item.Status),
		StatusMessage:  awssdk.ToString(item.StatusMessage),
		SourceVaultARN: awssdk.ToString(item.SourceBackupVaultArn),
		SizeBytes:      awssdk.ToInt64(item.BackupSizeInBytes),
		SizeBytesKnown: item.BackupSizeInBytes != nil,
		Encrypted:      item.IsEncrypted,
	}
	if item.CreationDate != nil {
		mapped.CreatedAt = *item.CreationDate
	}
	if item.CompletionDate != nil {
		mapped.CompletedAt = *item.CompletionDate
	}
	if item.CalculatedLifecycle != nil {
		if item.CalculatedLifecycle.MoveToColdStorageAt != nil {
			mapped.MoveToColdAt = *item.CalculatedLifecycle.MoveToColdStorageAt
		}
		if item.CalculatedLifecycle.DeleteAt != nil {
			mapped.DeleteAt = *item.CalculatedLifecycle.DeleteAt
		}
	}
	return mapped
}

func mapBackupProtectedResource(item backuptypes.ProtectedResource) BackupProtectedResource {
	mapped := BackupProtectedResource{
		ARN:                  awssdk.ToString(item.ResourceArn),
		Name:                 awssdk.ToString(item.ResourceName),
		Type:                 awssdk.ToString(item.ResourceType),
		LastRecoveryPointARN: awssdk.ToString(item.LastRecoveryPointArn),
	}
	if item.LastBackupTime != nil {
		mapped.LastBackupAt = *item.LastBackupTime
	}
	return mapped
}

func mapBackupJob(item backuptypes.BackupJob) BackupJob {
	mapped := BackupJob{
		ID:              awssdk.ToString(item.BackupJobId),
		ResourceARN:     awssdk.ToString(item.ResourceArn),
		ResourceName:    awssdk.ToString(item.ResourceName),
		ResourceType:    awssdk.ToString(item.ResourceType),
		State:           string(item.State),
		StatusMessage:   awssdk.ToString(item.StatusMessage),
		MessageCategory: awssdk.ToString(item.MessageCategory),
		SizeBytes:       awssdk.ToInt64(item.BackupSizeInBytes),
	}
	if item.CreationDate != nil {
		mapped.CreatedAt = *item.CreationDate
	}
	if item.CompletionDate != nil {
		mapped.CompletedAt = *item.CompletionDate
	}
	return mapped
}

func sortBackupRecoveryPoints(items []BackupRecoveryPoint) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].NeedsAttention() != items[j].NeedsAttention() {
			return items[i].NeedsAttention()
		}
		if !items[i].DeleteAt.Equal(items[j].DeleteAt) {
			if items[i].DeleteAt.IsZero() {
				return false
			}
			if items[j].DeleteAt.IsZero() {
				return true
			}
			return items[i].DeleteAt.Before(items[j].DeleteAt)
		}
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		left, right := normalizedSortKey(items[i].ResourceName), normalizedSortKey(items[j].ResourceName)
		if left != right {
			return left < right
		}
		return normalizedSortKey(items[i].ARN) < normalizedSortKey(items[j].ARN)
	})
}

func backupJobNeedsAttention(state, category string) bool {
	switch strings.ToUpper(state) {
	case "FAILED", "ABORTED", "EXPIRED", "PARTIAL":
		return true
	case "COMPLETED":
		return category != "" && !strings.EqualFold(category, "SUCCESS")
	default:
		return false
	}
}

func backupJobStatusRank(state string) int {
	switch strings.ToUpper(state) {
	case "FAILED":
		return 0
	case "EXPIRED":
		return 1
	case "ABORTED", "PARTIAL":
		return 2
	default:
		return 3
	}
}
