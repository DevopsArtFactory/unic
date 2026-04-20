package inspector

import (
	"context"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

type inspectorIAMMockClient struct {
	listUsersFunc            func(ctx context.Context, params *iam.ListUsersInput, optFns ...func(*iam.Options)) (*iam.ListUsersOutput, error)
	listAccessKeysFunc       func(ctx context.Context, params *iam.ListAccessKeysInput, optFns ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error)
	getAccessKeyLastUsedFunc func(ctx context.Context, params *iam.GetAccessKeyLastUsedInput, optFns ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error)
	getAccountSummaryFunc    func(ctx context.Context, params *iam.GetAccountSummaryInput, optFns ...func(*iam.Options)) (*iam.GetAccountSummaryOutput, error)
	getAuthzDetailsFunc      func(ctx context.Context, params *iam.GetAccountAuthorizationDetailsInput, optFns ...func(*iam.Options)) (*iam.GetAccountAuthorizationDetailsOutput, error)
}

func (m *inspectorIAMMockClient) ListUsers(ctx context.Context, params *iam.ListUsersInput, optFns ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	if m.listUsersFunc != nil {
		return m.listUsersFunc(ctx, params, optFns...)
	}
	return &iam.ListUsersOutput{}, nil
}

func (m *inspectorIAMMockClient) GetUser(context.Context, *iam.GetUserInput, ...func(*iam.Options)) (*iam.GetUserOutput, error) {
	return &iam.GetUserOutput{}, nil
}

func (m *inspectorIAMMockClient) GetAccountSummary(ctx context.Context, params *iam.GetAccountSummaryInput, optFns ...func(*iam.Options)) (*iam.GetAccountSummaryOutput, error) {
	if m.getAccountSummaryFunc != nil {
		return m.getAccountSummaryFunc(ctx, params, optFns...)
	}
	return &iam.GetAccountSummaryOutput{}, nil
}

func (m *inspectorIAMMockClient) GetAccountAuthorizationDetails(ctx context.Context, params *iam.GetAccountAuthorizationDetailsInput, optFns ...func(*iam.Options)) (*iam.GetAccountAuthorizationDetailsOutput, error) {
	if m.getAuthzDetailsFunc != nil {
		return m.getAuthzDetailsFunc(ctx, params, optFns...)
	}
	return &iam.GetAccountAuthorizationDetailsOutput{}, nil
}

func (m *inspectorIAMMockClient) ListGroupsForUser(context.Context, *iam.ListGroupsForUserInput, ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	return &iam.ListGroupsForUserOutput{}, nil
}

func (m *inspectorIAMMockClient) ListAttachedUserPolicies(context.Context, *iam.ListAttachedUserPoliciesInput, ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	return &iam.ListAttachedUserPoliciesOutput{}, nil
}

func (m *inspectorIAMMockClient) ListMFADevices(context.Context, *iam.ListMFADevicesInput, ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
	return &iam.ListMFADevicesOutput{}, nil
}

func (m *inspectorIAMMockClient) ListAccessKeys(ctx context.Context, params *iam.ListAccessKeysInput, optFns ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	if m.listAccessKeysFunc != nil {
		return m.listAccessKeysFunc(ctx, params, optFns...)
	}
	return &iam.ListAccessKeysOutput{}, nil
}

func (m *inspectorIAMMockClient) GetAccessKeyLastUsed(ctx context.Context, params *iam.GetAccessKeyLastUsedInput, optFns ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error) {
	if m.getAccessKeyLastUsedFunc != nil {
		return m.getAccessKeyLastUsedFunc(ctx, params, optFns...)
	}
	return &iam.GetAccessKeyLastUsedOutput{}, nil
}

func (m *inspectorIAMMockClient) CreateAccessKey(context.Context, *iam.CreateAccessKeyInput, ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error) {
	return &iam.CreateAccessKeyOutput{}, nil
}

func (m *inspectorIAMMockClient) UpdateAccessKey(context.Context, *iam.UpdateAccessKeyInput, ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error) {
	return &iam.UpdateAccessKeyOutput{}, nil
}

func (m *inspectorIAMMockClient) DeleteAccessKey(context.Context, *iam.DeleteAccessKeyInput, ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error) {
	return &iam.DeleteAccessKeyOutput{}, nil
}

func (m *inspectorIAMMockClient) ListServiceSpecificCredentials(context.Context, *iam.ListServiceSpecificCredentialsInput, ...func(*iam.Options)) (*iam.ListServiceSpecificCredentialsOutput, error) {
	return &iam.ListServiceSpecificCredentialsOutput{}, nil
}

func (m *inspectorIAMMockClient) CreateServiceSpecificCredential(context.Context, *iam.CreateServiceSpecificCredentialInput, ...func(*iam.Options)) (*iam.CreateServiceSpecificCredentialOutput, error) {
	return &iam.CreateServiceSpecificCredentialOutput{}, nil
}

func (m *inspectorIAMMockClient) ResetServiceSpecificCredential(context.Context, *iam.ResetServiceSpecificCredentialInput, ...func(*iam.Options)) (*iam.ResetServiceSpecificCredentialOutput, error) {
	return &iam.ResetServiceSpecificCredentialOutput{}, nil
}

func (m *inspectorIAMMockClient) DeleteServiceSpecificCredential(context.Context, *iam.DeleteServiceSpecificCredentialInput, ...func(*iam.Options)) (*iam.DeleteServiceSpecificCredentialOutput, error) {
	return &iam.DeleteServiceSpecificCredentialOutput{}, nil
}

type inspectorSecretsMockClient struct {
	listSecretsFunc    func(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
	getSecretValueFunc func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

func (m *inspectorSecretsMockClient) ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	if m.listSecretsFunc != nil {
		return m.listSecretsFunc(ctx, params, optFns...)
	}
	return &secretsmanager.ListSecretsOutput{}, nil
}

func (m *inspectorSecretsMockClient) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	if m.getSecretValueFunc != nil {
		return m.getSecretValueFunc(ctx, params, optFns...)
	}
	return &secretsmanager.GetSecretValueOutput{}, nil
}

func TestListSecretsMapsRotationMetadata(t *testing.T) {
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	lastChanged := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	lastRotated := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	nextRotation := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	mock := &inspectorSecretsMockClient{
		listSecretsFunc: func(_ context.Context, _ *secretsmanager.ListSecretsInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
			return &secretsmanager.ListSecretsOutput{
				SecretList: []smtypes.SecretListEntry{
					{
						Name:             awssdk.String("prod/db"),
						ARN:              awssdk.String("arn:aws:secretsmanager:ap-northeast-2:123456789012:secret:prod/db"),
						Description:      awssdk.String("database credentials"),
						KmsKeyId:         awssdk.String("key-123"),
						RotationEnabled:  awssdk.Bool(true),
						CreatedDate:      &created,
						LastChangedDate:  &lastChanged,
						LastRotatedDate:  &lastRotated,
						NextRotationDate: &nextRotation,
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{SecretsManagerClient: mock}
	secrets, err := repo.ListSecrets(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}

	secret := secrets[0]
	if !secret.RotationEnabled {
		t.Fatal("expected rotation enabled")
	}
	if !secret.CreatedDate.Equal(created) || !secret.LastChangedDate.Equal(lastChanged) || !secret.LastRotatedDate.Equal(lastRotated) {
		t.Fatalf("unexpected rotation metadata: %+v", secret)
	}
	if !secret.NextRotationDate.Equal(nextRotation) {
		t.Fatalf("unexpected next rotation date: %+v", secret)
	}
}

func TestInspectIAMAccessKeysFlagsStaleActiveKeys(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	oldCreate := now.AddDate(0, 0, -inspectorIAMAccessKeyHighAgeDays)
	recentCreate := now.AddDate(0, 0, -30)

	mockIAM := &inspectorIAMMockClient{
		listUsersFunc: func(_ context.Context, _ *iam.ListUsersInput, _ ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
			return &iam.ListUsersOutput{
				Users: []iamtypes.User{
					{UserName: awssdk.String("alice")},
				},
			}, nil
		},
		listAccessKeysFunc: func(_ context.Context, params *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			if awssdk.ToString(params.UserName) != "alice" {
				t.Fatalf("unexpected user: %q", awssdk.ToString(params.UserName))
			}
			return &iam.ListAccessKeysOutput{
				AccessKeyMetadata: []iamtypes.AccessKeyMetadata{
					{AccessKeyId: awssdk.String("AKIAOLD"), Status: iamtypes.StatusTypeActive, CreateDate: &oldCreate},
					{AccessKeyId: awssdk.String("AKIARECENT"), Status: iamtypes.StatusTypeActive, CreateDate: &recentCreate},
					{AccessKeyId: awssdk.String("AKIAINACTIVE"), Status: iamtypes.StatusTypeInactive, CreateDate: &oldCreate},
				},
			}, nil
		},
		getAccessKeyLastUsedFunc: func(_ context.Context, _ *iam.GetAccessKeyLastUsedInput, _ ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error) {
			return &iam.GetAccessKeyLastUsedOutput{}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mockIAM}
	findings, err := inspectIAMAccessKeyAges(context.Background(), repo, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 stale access key finding, got %d", len(findings))
	}

	finding := findings[0]
	if finding.ResourceID != "alice/AKIAOLD" {
		t.Fatalf("unexpected resource id: %+v", finding)
	}
	if finding.Severity != RuleSeverityHigh {
		t.Fatalf("expected high severity at the IAM high-age threshold, got %s", finding.Severity)
	}
}

func TestInspectSecretsManagerRotationFlagsDisabledAndOverdueSecrets(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	oldRotated := now.AddDate(0, 0, -120)
	veryOldRotated := now.AddDate(0, 0, -inspectorSecretRotationHighAgeDays)

	mockSecrets := &inspectorSecretsMockClient{
		listSecretsFunc: func(_ context.Context, _ *secretsmanager.ListSecretsInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
			return &secretsmanager.ListSecretsOutput{
				SecretList: []smtypes.SecretListEntry{
					{
						Name:            awssdk.String("prod/no-rotation"),
						RotationEnabled: awssdk.Bool(false),
					},
					{
						Name:            awssdk.String("prod/stale-rotation"),
						RotationEnabled: awssdk.Bool(true),
						LastRotatedDate: &oldRotated,
					},
					{
						Name:            awssdk.String("prod/very-stale-rotation"),
						RotationEnabled: awssdk.Bool(true),
						LastRotatedDate: &veryOldRotated,
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{SecretsManagerClient: mockSecrets}
	findings, err := inspectSecretsManagerRotationAges(context.Background(), repo, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 3 {
		t.Fatalf("expected 3 secret findings, got %d", len(findings))
	}

	if findings[0].ResourceID != "prod/no-rotation" {
		t.Fatalf("expected disabled-rotation finding first, got %+v", findings[0])
	}
	if findings[0].Severity != RuleSeverityHigh {
		t.Fatalf("expected high severity for disabled rotation, got %s", findings[0].Severity)
	}
	if findings[1].ResourceID != "prod/stale-rotation" {
		t.Fatalf("expected overdue rotation finding second, got %+v", findings[1])
	}
	if findings[1].Severity != RuleSeverityMedium {
		t.Fatalf("expected medium severity for stale rotation, got %s", findings[1].Severity)
	}
	if findings[2].ResourceID != "prod/very-stale-rotation" {
		t.Fatalf("expected very overdue rotation finding third, got %+v", findings[2])
	}
	if findings[2].Severity != RuleSeverityHigh {
		t.Fatalf("expected high severity at the secrets high-age threshold, got %s", findings[2].Severity)
	}
}
