package aws

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
)

type mockBackupClient struct {
	listVaults    func(context.Context, *backup.ListBackupVaultsInput, ...func(*backup.Options)) (*backup.ListBackupVaultsOutput, error)
	listPoints    func(context.Context, *backup.ListRecoveryPointsByBackupVaultInput, ...func(*backup.Options)) (*backup.ListRecoveryPointsByBackupVaultOutput, error)
	listResources func(context.Context, *backup.ListProtectedResourcesByBackupVaultInput, ...func(*backup.Options)) (*backup.ListProtectedResourcesByBackupVaultOutput, error)
	listJobs      func(context.Context, *backup.ListBackupJobsInput, ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error)
}

func (m *mockBackupClient) ListBackupVaults(ctx context.Context, input *backup.ListBackupVaultsInput, opts ...func(*backup.Options)) (*backup.ListBackupVaultsOutput, error) {
	return m.listVaults(ctx, input, opts...)
}

func (m *mockBackupClient) ListRecoveryPointsByBackupVault(ctx context.Context, input *backup.ListRecoveryPointsByBackupVaultInput, opts ...func(*backup.Options)) (*backup.ListRecoveryPointsByBackupVaultOutput, error) {
	return m.listPoints(ctx, input, opts...)
}

func (m *mockBackupClient) ListProtectedResourcesByBackupVault(ctx context.Context, input *backup.ListProtectedResourcesByBackupVaultInput, opts ...func(*backup.Options)) (*backup.ListProtectedResourcesByBackupVaultOutput, error) {
	return m.listResources(ctx, input, opts...)
}

func (m *mockBackupClient) ListBackupJobs(ctx context.Context, input *backup.ListBackupJobsInput, opts ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
	return m.listJobs(ctx, input, opts...)
}

func TestListBackupVaultsPaginatesMapsAndPreservesCompletedPages(t *testing.T) {
	now := time.Date(2026, 8, 26, 1, 0, 0, 0, time.UTC)
	calls := 0
	client := &mockBackupClient{listVaults: func(_ context.Context, input *backup.ListBackupVaultsInput, _ ...func(*backup.Options)) (*backup.ListBackupVaultsOutput, error) {
		calls++
		if calls == 1 {
			if input.NextToken != nil {
				t.Fatalf("expected no first-page token, got %q", awssdk.ToString(input.NextToken))
			}
			return &backup.ListBackupVaultsOutput{
				NextToken: awssdk.String("page-2"),
				BackupVaultList: []backuptypes.BackupVaultListMember{{
					BackupVaultName: awssdk.String("prod"), BackupVaultArn: awssdk.String("arn:prod"),
					CreationDate: awssdk.Time(now), EncryptionKeyArn: awssdk.String("arn:kms"),
					EncryptionKeyType: backuptypes.EncryptionKeyTypeCustomerManagedKmsKey,
					Locked:            awssdk.Bool(true), LockDate: awssdk.Time(now.Add(time.Hour)),
					MinRetentionDays: awssdk.Int64(7), MaxRetentionDays: awssdk.Int64(365),
					NumberOfRecoveryPoints: 4, VaultState: backuptypes.VaultStateAvailable,
					VaultType: backuptypes.VaultTypeBackupVault,
				}},
			}, nil
		}
		if awssdk.ToString(input.NextToken) != "page-2" {
			t.Fatalf("expected second-page token, got %q", awssdk.ToString(input.NextToken))
		}
		return nil, errors.New("page denied")
	}}

	vaults, warnings, err := (&AwsRepository{BackupClient: client, Region: "ap-northeast-2"}).ListBackupVaults(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(vaults) != 1 || len(warnings) != 1 {
		t.Fatalf("expected one retained vault and one warning, calls=%d vaults=%+v warnings=%v", calls, vaults, warnings)
	}
	vault := vaults[0]
	if vault.Name != "prod" || vault.Region != "ap-northeast-2" || !vault.Locked || vault.RecoveryPointCount != 4 || vault.EncryptionKeyARN != "arn:kms" {
		t.Fatalf("unexpected vault mapping: %+v", vault)
	}
	for _, value := range []string{"prod", "available", "arn:kms", "ap-northeast-2"} {
		if !strings.Contains(vault.FilterText(), value) {
			t.Fatalf("expected filter text to contain %q: %q", value, vault.FilterText())
		}
	}
}

func TestListBackupVaultsReturnsFatalInitialError(t *testing.T) {
	client := &mockBackupClient{listVaults: func(context.Context, *backup.ListBackupVaultsInput, ...func(*backup.Options)) (*backup.ListBackupVaultsOutput, error) {
		return nil, errors.New("denied")
	}}
	_, _, err := (&AwsRepository{BackupClient: client}).ListBackupVaults(context.Background())
	if err == nil || !strings.Contains(err.Error(), "list AWS Backup vaults") {
		t.Fatalf("expected contextual fatal list error, got %v", err)
	}
}

func TestBackupOptionalNumericFieldsPreservePresence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		min, max *int64
		wantMin  bool
		wantMax  bool
	}{
		{name: "minimum only", min: awssdk.Int64(7), wantMin: true},
		{name: "maximum only", max: awssdk.Int64(365), wantMax: true},
		{name: "unbounded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vault := mapBackupVault(backuptypes.BackupVaultListMember{
				MinRetentionDays: tc.min,
				MaxRetentionDays: tc.max,
			}, "ap-northeast-2")
			if vault.MinRetentionKnown != tc.wantMin || vault.MaxRetentionKnown != tc.wantMax {
				t.Fatalf("unexpected retention presence: %+v", vault)
			}
		})
	}

	unknown := mapBackupRecoveryPoint(backuptypes.RecoveryPointByBackupVault{})
	zero := mapBackupRecoveryPoint(backuptypes.RecoveryPointByBackupVault{BackupSizeInBytes: awssdk.Int64(0)})
	if unknown.SizeBytesKnown || !zero.SizeBytesKnown || zero.SizeBytes != 0 {
		t.Fatalf("expected nil size to remain unknown and explicit zero to remain known: unknown=%+v zero=%+v", unknown, zero)
	}
}

func TestGetBackupVaultDetailKeepsPartialSectionsAndPrioritizesFailures(t *testing.T) {
	now := time.Date(2026, 8, 26, 2, 0, 0, 0, time.UTC)
	pointCalls := 0
	client := &mockBackupClient{
		listPoints: func(_ context.Context, input *backup.ListRecoveryPointsByBackupVaultInput, _ ...func(*backup.Options)) (*backup.ListRecoveryPointsByBackupVaultOutput, error) {
			pointCalls++
			if awssdk.ToString(input.BackupVaultName) != "prod" {
				t.Fatalf("expected prod vault, got %q", awssdk.ToString(input.BackupVaultName))
			}
			if pointCalls == 1 {
				return &backup.ListRecoveryPointsByBackupVaultOutput{
					NextToken: awssdk.String("page-2"),
					RecoveryPoints: []backuptypes.RecoveryPointByBackupVault{
						{RecoveryPointArn: awssdk.String("arn:healthy"), ResourceName: awssdk.String("healthy"), ResourceType: awssdk.String("RDS"), Status: backuptypes.RecoveryPointStatusCompleted, CreationDate: awssdk.Time(now.Add(-time.Hour))},
						{RecoveryPointArn: awssdk.String("arn:failed"), ResourceName: awssdk.String("failed"), ResourceType: awssdk.String("EBS"), Status: backuptypes.RecoveryPointStatusPartial, StatusMessage: awssdk.String("snapshot failed"), CreationDate: awssdk.Time(now), CalculatedLifecycle: &backuptypes.CalculatedLifecycle{DeleteAt: awssdk.Time(now.Add(24 * time.Hour))}},
					},
				}, nil
			}
			return nil, errors.New("second page denied")
		},
		listResources: func(_ context.Context, input *backup.ListProtectedResourcesByBackupVaultInput, _ ...func(*backup.Options)) (*backup.ListProtectedResourcesByBackupVaultOutput, error) {
			if awssdk.ToString(input.BackupVaultName) != "prod" {
				t.Fatalf("expected prod vault, got %q", awssdk.ToString(input.BackupVaultName))
			}
			return &backup.ListProtectedResourcesByBackupVaultOutput{Results: []backuptypes.ProtectedResource{{
				ResourceArn: awssdk.String("arn:resource"), ResourceName: awssdk.String("database"), ResourceType: awssdk.String("RDS"), LastBackupTime: awssdk.Time(now), LastRecoveryPointArn: awssdk.String("arn:healthy"),
			}}}, nil
		},
		listJobs: func(_ context.Context, input *backup.ListBackupJobsInput, _ ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
			if awssdk.ToString(input.ByBackupVaultName) != "prod" {
				t.Fatalf("expected prod vault filter, got %q", awssdk.ToString(input.ByBackupVaultName))
			}
			return &backup.ListBackupJobsOutput{BackupJobs: []backuptypes.BackupJob{
				{BackupJobId: awssdk.String("ok"), State: backuptypes.BackupJobStateCompleted, MessageCategory: awssdk.String("SUCCESS")},
				{BackupJobId: awssdk.String("expired"), ResourceName: awssdk.String("old"), State: backuptypes.BackupJobStateExpired, CreationDate: awssdk.Time(now.Add(-time.Hour))},
				{BackupJobId: awssdk.String("failed"), ResourceName: awssdk.String("db"), State: backuptypes.BackupJobStateFailed, StatusMessage: awssdk.String("access denied"), CreationDate: awssdk.Time(now)},
			}}, nil
		},
	}

	vault := BackupVault{Name: "prod", ARN: "arn:vault"}
	detail, warnings, err := (&AwsRepository{BackupClient: client}).GetBackupVaultDetail(context.Background(), vault)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Error(), "second page denied") {
		t.Fatalf("expected retained-page warning, got %v", warnings)
	}
	if len(detail.RecoveryPoints) != 2 || detail.RecoveryPoints[0].ARN != "arn:failed" {
		t.Fatalf("expected failed recovery point first after partial pagination, got %+v", detail.RecoveryPoints)
	}
	if len(detail.ProtectedResources) != 1 || detail.ProtectedResources[0].LastRecoveryPointARN != "arn:healthy" {
		t.Fatalf("unexpected protected resources: %+v", detail.ProtectedResources)
	}
	if len(detail.FailedJobs) != 2 || detail.FailedJobs[0].ID != "failed" || detail.FailedJobs[1].ID != "expired" {
		t.Fatalf("expected only failure jobs in priority order, got %+v", detail.FailedJobs)
	}
}

func TestBackupDetailSortsUseUniqueTieBreakers(t *testing.T) {
	now := time.Date(2026, 8, 26, 4, 0, 0, 0, time.UTC)

	resources := []BackupProtectedResource{
		{ARN: "arn:z", Name: "database", LastBackupAt: now},
		{ARN: "arn:a", Name: "database", LastBackupAt: now},
	}
	sortBackupProtectedResources(resources)
	if resources[0].ARN != "arn:a" {
		t.Fatalf("expected protected resources to use ARN tie-breaker, got %+v", resources)
	}

	jobs := []BackupJob{
		{ID: "job-z", State: "FAILED", CreatedAt: now},
		{ID: "job-a", State: "FAILED", CreatedAt: now},
	}
	sortBackupJobs(jobs)
	if jobs[0].ID != "job-a" {
		t.Fatalf("expected jobs to use ID tie-breaker, got %+v", jobs)
	}

	points := []BackupRecoveryPoint{
		{ARN: "arn:z", ResourceName: "database", Status: "COMPLETED", CreatedAt: now},
		{ARN: "arn:a", ResourceName: "database", Status: "COMPLETED", CreatedAt: now},
	}
	sortBackupRecoveryPoints(points)
	if points[0].ARN != "arn:a" {
		t.Fatalf("expected recovery points to use ARN tie-breaker, got %+v", points)
	}
}

func TestGetBackupVaultDetailReturnsWarningsWhenAllSectionsAreDenied(t *testing.T) {
	client := &mockBackupClient{
		listPoints: func(context.Context, *backup.ListRecoveryPointsByBackupVaultInput, ...func(*backup.Options)) (*backup.ListRecoveryPointsByBackupVaultOutput, error) {
			return nil, errors.New("points denied")
		},
		listResources: func(context.Context, *backup.ListProtectedResourcesByBackupVaultInput, ...func(*backup.Options)) (*backup.ListProtectedResourcesByBackupVaultOutput, error) {
			return nil, errors.New("resources denied")
		},
		listJobs: func(context.Context, *backup.ListBackupJobsInput, ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
			return nil, errors.New("jobs denied")
		},
	}
	detail, warnings, err := (&AwsRepository{BackupClient: client}).GetBackupVaultDetail(context.Background(), BackupVault{Name: "prod"})
	if err != nil || detail == nil || len(warnings) != 3 {
		t.Fatalf("expected empty detail with three warnings, detail=%+v warnings=%v err=%v", detail, warnings, err)
	}
}
