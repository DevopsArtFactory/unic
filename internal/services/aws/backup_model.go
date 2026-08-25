package aws

import (
	"fmt"
	"strings"
	"time"
)

// BackupVault contains vault metadata rendered by the browser.
type BackupVault struct {
	ARN                string
	Name               string
	Region             string
	State              string
	Type               string
	EncryptionKeyARN   string
	EncryptionKeyType  string
	RecoveryPointCount int64
	Locked             bool
	LockDate           time.Time
	MinRetentionDays   int64
	MinRetentionKnown  bool
	MaxRetentionDays   int64
	MaxRetentionKnown  bool
	CreatedAt          time.Time
}

// FilterText returns searchable vault metadata.
func (v BackupVault) FilterText() string {
	return strings.ToLower(strings.Join([]string{v.ARN, v.Name, v.Region, v.State, v.Type, v.EncryptionKeyARN, v.EncryptionKeyType}, " "))
}

// DisplayTitle returns a column-aligned vault row.
func (v BackupVault) DisplayTitle() string {
	lock := "-"
	if v.Locked {
		lock = "locked"
	}
	return fmt.Sprintf("%-32.32s  %-12.12s  %8d  %-8s  %s", v.Name, valueOrDash(v.State), v.RecoveryPointCount, lock, valueOrDash(v.Type))
}

// BackupVaultDetail contains the recovery-readiness signals for one vault.
type BackupVaultDetail struct {
	Vault              BackupVault
	RecoveryPoints     []BackupRecoveryPoint
	ProtectedResources []BackupProtectedResource
	FailedJobs         []BackupJob
}

// BackupRecoveryPoint contains recovery point metadata used by the detail screen.
type BackupRecoveryPoint struct {
	ARN            string
	ResourceARN    string
	ResourceName   string
	ResourceType   string
	Status         string
	StatusMessage  string
	SourceVaultARN string
	SizeBytes      int64
	SizeBytesKnown bool
	Encrypted      bool
	CreatedAt      time.Time
	CompletedAt    time.Time
	MoveToColdAt   time.Time
	DeleteAt       time.Time
}

// NeedsAttention reports whether the recovery point is not currently usable.
func (r BackupRecoveryPoint) NeedsAttention() bool {
	switch strings.ToUpper(r.Status) {
	case "AVAILABLE", "COMPLETED":
		return false
	default:
		return true
	}
}

// BackupProtectedResource contains the latest backup metadata for one resource.
type BackupProtectedResource struct {
	ARN                  string
	Name                 string
	Type                 string
	LastBackupAt         time.Time
	LastRecoveryPointARN string
}

// BackupJob contains a recent failed or expired backup job.
type BackupJob struct {
	ID            string
	ResourceARN   string
	ResourceName  string
	ResourceType  string
	State         string
	StatusMessage string
	SizeBytes     int64
	CreatedAt     time.Time
	CompletedAt   time.Time
}
