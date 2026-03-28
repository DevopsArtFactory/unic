package aws

import (
	"context"
	"fmt"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// mockIAMClient implements IAMClientAPI for testing.
type mockIAMClient struct {
	listAccessKeysFunc       func(ctx context.Context, params *iam.ListAccessKeysInput, optFns ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error)
	getAccessKeyLastUsedFunc func(ctx context.Context, params *iam.GetAccessKeyLastUsedInput, optFns ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error)
	createAccessKeyFunc      func(ctx context.Context, params *iam.CreateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error)
	updateAccessKeyFunc      func(ctx context.Context, params *iam.UpdateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error)
	deleteAccessKeyFunc      func(ctx context.Context, params *iam.DeleteAccessKeyInput, optFns ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error)
}

func (m *mockIAMClient) ListAccessKeys(ctx context.Context, params *iam.ListAccessKeysInput, optFns ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	return m.listAccessKeysFunc(ctx, params, optFns...)
}

func (m *mockIAMClient) GetAccessKeyLastUsed(ctx context.Context, params *iam.GetAccessKeyLastUsedInput, optFns ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error) {
	if m.getAccessKeyLastUsedFunc != nil {
		return m.getAccessKeyLastUsedFunc(ctx, params, optFns...)
	}
	return &iam.GetAccessKeyLastUsedOutput{
		AccessKeyLastUsed: &iamtypes.AccessKeyLastUsed{
			ServiceName: awssdk.String("N/A"),
		},
	}, nil
}

func (m *mockIAMClient) CreateAccessKey(ctx context.Context, params *iam.CreateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error) {
	if m.createAccessKeyFunc != nil {
		return m.createAccessKeyFunc(ctx, params, optFns...)
	}
	return &iam.CreateAccessKeyOutput{}, nil
}

func (m *mockIAMClient) UpdateAccessKey(ctx context.Context, params *iam.UpdateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error) {
	if m.updateAccessKeyFunc != nil {
		return m.updateAccessKeyFunc(ctx, params, optFns...)
	}
	return &iam.UpdateAccessKeyOutput{}, nil
}

func (m *mockIAMClient) DeleteAccessKey(ctx context.Context, params *iam.DeleteAccessKeyInput, optFns ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error) {
	if m.deleteAccessKeyFunc != nil {
		return m.deleteAccessKeyFunc(ctx, params, optFns...)
	}
	return &iam.DeleteAccessKeyOutput{}, nil
}

// --- ListAccessKeys tests ---

func TestListAccessKeys_Success(t *testing.T) {
	created := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	lastUsed := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	mock := &mockIAMClient{
		listAccessKeysFunc: func(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			return &iam.ListAccessKeysOutput{
				AccessKeyMetadata: []iamtypes.AccessKeyMetadata{
					{
						AccessKeyId: awssdk.String("AKIAIOSFODNN7EXAMPLE"),
						Status:      iamtypes.StatusTypeActive,
						CreateDate:  &created,
					},
				},
			}, nil
		},
		getAccessKeyLastUsedFunc: func(_ context.Context, params *iam.GetAccessKeyLastUsedInput, _ ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error) {
			return &iam.GetAccessKeyLastUsedOutput{
				AccessKeyLastUsed: &iamtypes.AccessKeyLastUsed{
					LastUsedDate: &lastUsed,
					ServiceName:  awssdk.String("s3"),
				},
			}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	keys, err := repo.ListAccessKeys(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	key := keys[0]
	if key.AccessKeyID != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("expected key ID 'AKIAIOSFODNN7EXAMPLE', got %q", key.AccessKeyID)
	}
	if key.Status != "Active" {
		t.Errorf("expected status 'Active', got %q", key.Status)
	}
	if !key.LastUsed.Equal(lastUsed) {
		t.Errorf("expected last used %v, got %v", lastUsed, key.LastUsed)
	}
	if key.ServiceName != "s3" {
		t.Errorf("expected service 's3', got %q", key.ServiceName)
	}
}

func TestListAccessKeys_Empty(t *testing.T) {
	mock := &mockIAMClient{
		listAccessKeysFunc: func(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			return &iam.ListAccessKeysOutput{
				AccessKeyMetadata: []iamtypes.AccessKeyMetadata{},
			}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	keys, err := repo.ListAccessKeys(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected empty slice, got %d", len(keys))
	}
}

func TestListAccessKeys_Error(t *testing.T) {
	mock := &mockIAMClient{
		listAccessKeysFunc: func(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	_, err := repo.ListAccessKeys(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Access key lifecycle tests ---

func TestCreateAccessKey_Success(t *testing.T) {
	mock := &mockIAMClient{
		createAccessKeyFunc: func(_ context.Context, _ *iam.CreateAccessKeyInput, _ ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error) {
			return &iam.CreateAccessKeyOutput{
				AccessKey: &iamtypes.AccessKey{
					AccessKeyId:     awssdk.String("AKIANEWKEY1234567890"),
					SecretAccessKey: awssdk.String("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
				},
			}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	newKey, err := repo.CreateAccessKey(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newKey.AccessKeyID != "AKIANEWKEY1234567890" {
		t.Errorf("expected new key ID 'AKIANEWKEY1234567890', got %q", newKey.AccessKeyID)
	}
	if newKey.SecretAccessKey != "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" {
		t.Errorf("unexpected secret key")
	}
}

func TestCreateAccessKey_Error(t *testing.T) {
	mock := &mockIAMClient{
		createAccessKeyFunc: func(_ context.Context, _ *iam.CreateAccessKeyInput, _ ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error) {
			return nil, fmt.Errorf("limit exceeded")
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	_, err := repo.CreateAccessKey(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDeactivateAccessKey_Success(t *testing.T) {
	var deactivatedKeyID string
	mock := &mockIAMClient{
		updateAccessKeyFunc: func(_ context.Context, params *iam.UpdateAccessKeyInput, _ ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error) {
			deactivatedKeyID = awssdk.ToString(params.AccessKeyId)
			if params.Status != iamtypes.StatusTypeInactive {
				t.Errorf("expected status Inactive, got %v", params.Status)
			}
			return &iam.UpdateAccessKeyOutput{}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	err := repo.DeactivateAccessKey(context.Background(), "AKIAOLDKEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deactivatedKeyID != "AKIAOLDKEY" {
		t.Fatalf("expected AKIAOLDKEY to be deactivated, got %q", deactivatedKeyID)
	}
}

func TestDeactivateAccessKey_Error(t *testing.T) {
	mock := &mockIAMClient{
		updateAccessKeyFunc: func(_ context.Context, _ *iam.UpdateAccessKeyInput, _ ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error) {
			return nil, fmt.Errorf("permission denied")
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	err := repo.DeactivateAccessKey(context.Background(), "AKIAOLDKEY")
	if err == nil {
		t.Fatal("expected error for deactivation failure")
	}
}

func TestDeleteAccessKey_Success(t *testing.T) {
	mock := &mockIAMClient{
		deleteAccessKeyFunc: func(_ context.Context, in *iam.DeleteAccessKeyInput, _ ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error) {
			if awssdk.ToString(in.AccessKeyId) != "AKIAOLDKEY" {
				t.Fatalf("expected AKIAOLDKEY, got %q", awssdk.ToString(in.AccessKeyId))
			}
			if in.UserName != nil {
				t.Fatalf("expected current identity delete without UserName, got %#v", in.UserName)
			}
			return &iam.DeleteAccessKeyOutput{}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	err := repo.DeleteAccessKey(context.Background(), "AKIAOLDKEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyAccessKey_RetriesTransientInvalidToken(t *testing.T) {
	original := getCallerIdentityWithConfig
	defer func() {
		getCallerIdentityWithConfig = original
	}()

	attempts := 0
	getCallerIdentityWithConfig = func(_ context.Context, _ awssdk.Config) (*sts.GetCallerIdentityOutput, error) {
		attempts++
		if attempts < 3 {
			return nil, fmt.Errorf("operation error STS: GetCallerIdentity, api error InvalidClientTokenId:")
		}
		return &sts.GetCallerIdentityOutput{
			Account: awssdk.String("123456789012"),
			Arn:     awssdk.String("arn:aws:iam::123456789012:user/test"),
			UserId:  awssdk.String("AIDATEST"),
		}, nil
	}

	repo := &AwsRepository{Region: "ap-northeast-2"}
	identity, err := repo.VerifyAccessKey(context.Background(), &NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if identity.Account != "123456789012" {
		t.Fatalf("expected account 123456789012, got %s", identity.Account)
	}
}

func TestVerifyAccessKey_DoesNotRetryNonRetryableError(t *testing.T) {
	original := getCallerIdentityWithConfig
	defer func() {
		getCallerIdentityWithConfig = original
	}()

	attempts := 0
	getCallerIdentityWithConfig = func(_ context.Context, _ awssdk.Config) (*sts.GetCallerIdentityOutput, error) {
		attempts++
		return nil, fmt.Errorf("operation error STS: GetCallerIdentity, api error AccessDenied:")
	}

	repo := &AwsRepository{Region: "ap-northeast-2"}
	_, err := repo.VerifyAccessKey(context.Background(), &NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

// --- Model tests ---

func TestAccessKeyAge(t *testing.T) {
	key := AccessKey{
		CreateDate: time.Now().Add(-100 * 24 * time.Hour),
	}
	if key.Age() < 99 || key.Age() > 101 {
		t.Errorf("expected age ~100, got %d", key.Age())
	}
	if !key.IsAged() {
		t.Error("key should be aged (>90 days)")
	}
}

func TestAccessKeyNotAged(t *testing.T) {
	key := AccessKey{
		CreateDate: time.Now().Add(-30 * 24 * time.Hour),
	}
	if key.IsAged() {
		t.Error("key should not be aged (<90 days)")
	}
}

func TestAccessKeyDisplayTitle(t *testing.T) {
	key := AccessKey{
		AccessKeyID: "AKIATEST",
		Status:      "Active",
		CreateDate:  time.Now().Add(-10 * 24 * time.Hour),
		LastUsed:    time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	title := key.DisplayTitle()
	if !containsIAM(title, "AKIATEST") {
		t.Errorf("title should contain key ID, got %q", title)
	}
	if !containsIAM(title, "Active") {
		t.Errorf("title should contain status, got %q", title)
	}
	if !containsIAM(title, "last:2025-06-01") {
		t.Errorf("title should contain last used date, got %q", title)
	}
}

func TestAccessKeyFilterText(t *testing.T) {
	key := AccessKey{
		AccessKeyID: "AKIATEST",
		Status:      "Active",
		ServiceName: "s3",
	}
	ft := key.FilterText()
	if !containsIAM(ft, "akiatest") {
		t.Errorf("filter text should contain lowercase key ID, got %q", ft)
	}
}

func containsIAM(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsIAMStr(s, substr))
}

func containsIAMStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
