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
	listUsersFunc            func(ctx context.Context, params *iam.ListUsersInput, optFns ...func(*iam.Options)) (*iam.ListUsersOutput, error)
	getUserFunc              func(ctx context.Context, params *iam.GetUserInput, optFns ...func(*iam.Options)) (*iam.GetUserOutput, error)
	getAccountSummaryFunc    func(ctx context.Context, params *iam.GetAccountSummaryInput, optFns ...func(*iam.Options)) (*iam.GetAccountSummaryOutput, error)
	getAuthzDetailsFunc      func(ctx context.Context, params *iam.GetAccountAuthorizationDetailsInput, optFns ...func(*iam.Options)) (*iam.GetAccountAuthorizationDetailsOutput, error)
	listGroupsForUserFunc    func(ctx context.Context, params *iam.ListGroupsForUserInput, optFns ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error)
	listAttachedPoliciesFunc func(ctx context.Context, params *iam.ListAttachedUserPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error)
	listMFADevicesFunc       func(ctx context.Context, params *iam.ListMFADevicesInput, optFns ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error)
	listAccessKeysFunc       func(ctx context.Context, params *iam.ListAccessKeysInput, optFns ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error)
	getAccessKeyLastUsedFunc func(ctx context.Context, params *iam.GetAccessKeyLastUsedInput, optFns ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error)
	createAccessKeyFunc      func(ctx context.Context, params *iam.CreateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error)
	updateAccessKeyFunc      func(ctx context.Context, params *iam.UpdateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error)
	deleteAccessKeyFunc      func(ctx context.Context, params *iam.DeleteAccessKeyInput, optFns ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error)
	listServiceCredsFunc     func(ctx context.Context, params *iam.ListServiceSpecificCredentialsInput, optFns ...func(*iam.Options)) (*iam.ListServiceSpecificCredentialsOutput, error)
	createServiceCredFunc    func(ctx context.Context, params *iam.CreateServiceSpecificCredentialInput, optFns ...func(*iam.Options)) (*iam.CreateServiceSpecificCredentialOutput, error)
	resetServiceCredFunc     func(ctx context.Context, params *iam.ResetServiceSpecificCredentialInput, optFns ...func(*iam.Options)) (*iam.ResetServiceSpecificCredentialOutput, error)
	deleteServiceCredFunc    func(ctx context.Context, params *iam.DeleteServiceSpecificCredentialInput, optFns ...func(*iam.Options)) (*iam.DeleteServiceSpecificCredentialOutput, error)
}

func (m *mockIAMClient) ListUsers(ctx context.Context, params *iam.ListUsersInput, optFns ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	if m.listUsersFunc != nil {
		return m.listUsersFunc(ctx, params, optFns...)
	}
	return &iam.ListUsersOutput{}, nil
}

func (m *mockIAMClient) GetUser(ctx context.Context, params *iam.GetUserInput, optFns ...func(*iam.Options)) (*iam.GetUserOutput, error) {
	if m.getUserFunc != nil {
		return m.getUserFunc(ctx, params, optFns...)
	}
	return &iam.GetUserOutput{}, nil
}

func (m *mockIAMClient) GetAccountSummary(ctx context.Context, params *iam.GetAccountSummaryInput, optFns ...func(*iam.Options)) (*iam.GetAccountSummaryOutput, error) {
	if m.getAccountSummaryFunc != nil {
		return m.getAccountSummaryFunc(ctx, params, optFns...)
	}
	return &iam.GetAccountSummaryOutput{}, nil
}

func (m *mockIAMClient) GetAccountAuthorizationDetails(ctx context.Context, params *iam.GetAccountAuthorizationDetailsInput, optFns ...func(*iam.Options)) (*iam.GetAccountAuthorizationDetailsOutput, error) {
	if m.getAuthzDetailsFunc != nil {
		return m.getAuthzDetailsFunc(ctx, params, optFns...)
	}
	return &iam.GetAccountAuthorizationDetailsOutput{}, nil
}

func (m *mockIAMClient) ListGroupsForUser(ctx context.Context, params *iam.ListGroupsForUserInput, optFns ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	if m.listGroupsForUserFunc != nil {
		return m.listGroupsForUserFunc(ctx, params, optFns...)
	}
	return &iam.ListGroupsForUserOutput{}, nil
}

func (m *mockIAMClient) ListAttachedUserPolicies(ctx context.Context, params *iam.ListAttachedUserPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	if m.listAttachedPoliciesFunc != nil {
		return m.listAttachedPoliciesFunc(ctx, params, optFns...)
	}
	return &iam.ListAttachedUserPoliciesOutput{}, nil
}

func (m *mockIAMClient) ListMFADevices(ctx context.Context, params *iam.ListMFADevicesInput, optFns ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
	if m.listMFADevicesFunc != nil {
		return m.listMFADevicesFunc(ctx, params, optFns...)
	}
	return &iam.ListMFADevicesOutput{}, nil
}

func (m *mockIAMClient) ListAccessKeys(ctx context.Context, params *iam.ListAccessKeysInput, optFns ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	if m.listAccessKeysFunc != nil {
		return m.listAccessKeysFunc(ctx, params, optFns...)
	}
	return &iam.ListAccessKeysOutput{}, nil
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

func (m *mockIAMClient) ListServiceSpecificCredentials(ctx context.Context, params *iam.ListServiceSpecificCredentialsInput, optFns ...func(*iam.Options)) (*iam.ListServiceSpecificCredentialsOutput, error) {
	if m.listServiceCredsFunc != nil {
		return m.listServiceCredsFunc(ctx, params, optFns...)
	}
	return &iam.ListServiceSpecificCredentialsOutput{}, nil
}

func (m *mockIAMClient) CreateServiceSpecificCredential(ctx context.Context, params *iam.CreateServiceSpecificCredentialInput, optFns ...func(*iam.Options)) (*iam.CreateServiceSpecificCredentialOutput, error) {
	if m.createServiceCredFunc != nil {
		return m.createServiceCredFunc(ctx, params, optFns...)
	}
	return &iam.CreateServiceSpecificCredentialOutput{}, nil
}

func (m *mockIAMClient) ResetServiceSpecificCredential(ctx context.Context, params *iam.ResetServiceSpecificCredentialInput, optFns ...func(*iam.Options)) (*iam.ResetServiceSpecificCredentialOutput, error) {
	if m.resetServiceCredFunc != nil {
		return m.resetServiceCredFunc(ctx, params, optFns...)
	}
	return &iam.ResetServiceSpecificCredentialOutput{}, nil
}

func (m *mockIAMClient) DeleteServiceSpecificCredential(ctx context.Context, params *iam.DeleteServiceSpecificCredentialInput, optFns ...func(*iam.Options)) (*iam.DeleteServiceSpecificCredentialOutput, error) {
	if m.deleteServiceCredFunc != nil {
		return m.deleteServiceCredFunc(ctx, params, optFns...)
	}
	return &iam.DeleteServiceSpecificCredentialOutput{}, nil
}

// --- IAM user tests ---

func TestListIAMUserSummariesPage_Success(t *testing.T) {
	createdAlice := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	mock := &mockIAMClient{
		listUsersFunc: func(_ context.Context, params *iam.ListUsersInput, _ ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
			if awssdk.ToInt32(params.MaxItems) != 25 {
				t.Fatalf("expected MaxItems 25, got %d", awssdk.ToInt32(params.MaxItems))
			}
			if awssdk.ToString(params.Marker) != "page-1" {
				t.Fatalf("expected marker page-1, got %q", awssdk.ToString(params.Marker))
			}
			return &iam.ListUsersOutput{
				Users: []iamtypes.User{
					{
						UserName:   awssdk.String("alice"),
						UserId:     awssdk.String("AIDAALICE"),
						Arn:        awssdk.String("arn:aws:iam::123456789012:user/alice"),
						Path:       awssdk.String("/engineering/"),
						CreateDate: &createdAlice,
					},
				},
				IsTruncated: true,
				Marker:      awssdk.String("page-2"),
			}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	page, err := repo.ListIAMUserSummariesPage(context.Background(), "page-1", 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(page.Users))
	}
	if !page.HasMore {
		t.Fatal("expected HasMore to be true")
	}
	if page.NextMarker != "page-2" {
		t.Fatalf("expected next marker page-2, got %q", page.NextMarker)
	}
	if page.Users[0].AccessKeyCount != 0 {
		t.Fatalf("expected lightweight summary without access key metadata, got %d", page.Users[0].AccessKeyCount)
	}
}

func TestListIAMUsers_Success(t *testing.T) {
	createdAlice := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	createdBob := time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC)
	passwordLastUsed := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	accessKeyLastUsed := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	mock := &mockIAMClient{
		listUsersFunc: func(_ context.Context, params *iam.ListUsersInput, _ ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
			if params.Marker == nil {
				return &iam.ListUsersOutput{
					Users: []iamtypes.User{
						{
							UserName:         awssdk.String("bob"),
							UserId:           awssdk.String("AIDABOB"),
							Arn:              awssdk.String("arn:aws:iam::123456789012:user/bob"),
							Path:             awssdk.String("/"),
							CreateDate:       &createdBob,
							PasswordLastUsed: &passwordLastUsed,
						},
					},
					IsTruncated: true,
					Marker:      awssdk.String("page-2"),
				}, nil
			}
			if awssdk.ToString(params.Marker) != "page-2" {
				return nil, fmt.Errorf("unexpected marker %q", awssdk.ToString(params.Marker))
			}
			return &iam.ListUsersOutput{
				Users: []iamtypes.User{
					{
						UserName:   awssdk.String("alice"),
						UserId:     awssdk.String("AIDAALICE"),
						Arn:        awssdk.String("arn:aws:iam::123456789012:user/alice"),
						Path:       awssdk.String("/engineering/"),
						CreateDate: &createdAlice,
					},
				},
			}, nil
		},
		listAccessKeysFunc: func(_ context.Context, params *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			switch awssdk.ToString(params.UserName) {
			case "alice":
				return &iam.ListAccessKeysOutput{
					AccessKeyMetadata: []iamtypes.AccessKeyMetadata{
						{
							AccessKeyId: awssdk.String("AKIAALICE"),
							Status:      iamtypes.StatusTypeActive,
							CreateDate:  &createdAlice,
						},
					},
				}, nil
			case "bob":
				return &iam.ListAccessKeysOutput{}, nil
			default:
				return nil, fmt.Errorf("unexpected user %q", awssdk.ToString(params.UserName))
			}
		},
		getAccessKeyLastUsedFunc: func(_ context.Context, params *iam.GetAccessKeyLastUsedInput, _ ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error) {
			if awssdk.ToString(params.AccessKeyId) != "AKIAALICE" {
				return nil, fmt.Errorf("unexpected access key %q", awssdk.ToString(params.AccessKeyId))
			}
			return &iam.GetAccessKeyLastUsedOutput{
				AccessKeyLastUsed: &iamtypes.AccessKeyLastUsed{
					LastUsedDate: &accessKeyLastUsed,
					ServiceName:  awssdk.String("ec2"),
				},
			}, nil
		},
		listMFADevicesFunc: func(_ context.Context, params *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
			if awssdk.ToString(params.UserName) == "alice" {
				return &iam.ListMFADevicesOutput{
					MFADevices: []iamtypes.MFADevice{
						{UserName: awssdk.String("alice")},
					},
				}, nil
			}
			return &iam.ListMFADevicesOutput{}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	users, err := repo.ListIAMUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].UserName != "alice" {
		t.Fatalf("expected alphabetical sort with alice first, got %q", users[0].UserName)
	}
	if !users[0].MFAEnabled {
		t.Fatal("expected alice MFA status to be true")
	}
	if !users[0].LastActivity.Equal(accessKeyLastUsed) {
		t.Fatalf("expected alice last activity %v, got %v", accessKeyLastUsed, users[0].LastActivity)
	}
	if users[1].LastActivity.IsZero() || !users[1].LastActivity.Equal(passwordLastUsed) {
		t.Fatalf("expected bob last activity from password use, got %v", users[1].LastActivity)
	}
}

func TestGetIAMUserDetail_Success(t *testing.T) {
	created := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	passwordLastUsed := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	accessKeyLastUsed := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	mock := &mockIAMClient{
		getUserFunc: func(_ context.Context, params *iam.GetUserInput, _ ...func(*iam.Options)) (*iam.GetUserOutput, error) {
			if awssdk.ToString(params.UserName) != "alice" {
				return nil, fmt.Errorf("unexpected user %q", awssdk.ToString(params.UserName))
			}
			return &iam.GetUserOutput{
				User: &iamtypes.User{
					UserName:         awssdk.String("alice"),
					UserId:           awssdk.String("AIDAALICE"),
					Arn:              awssdk.String("arn:aws:iam::123456789012:user/alice"),
					Path:             awssdk.String("/engineering/"),
					CreateDate:       &created,
					PasswordLastUsed: &passwordLastUsed,
				},
			}, nil
		},
		listAccessKeysFunc: func(_ context.Context, params *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			if awssdk.ToString(params.UserName) != "alice" {
				return nil, fmt.Errorf("unexpected user %q", awssdk.ToString(params.UserName))
			}
			return &iam.ListAccessKeysOutput{
				AccessKeyMetadata: []iamtypes.AccessKeyMetadata{
					{
						AccessKeyId: awssdk.String("AKIAALICE"),
						Status:      iamtypes.StatusTypeActive,
						CreateDate:  &created,
					},
				},
			}, nil
		},
		getAccessKeyLastUsedFunc: func(_ context.Context, params *iam.GetAccessKeyLastUsedInput, _ ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error) {
			if awssdk.ToString(params.AccessKeyId) != "AKIAALICE" {
				return nil, fmt.Errorf("unexpected access key %q", awssdk.ToString(params.AccessKeyId))
			}
			return &iam.GetAccessKeyLastUsedOutput{
				AccessKeyLastUsed: &iamtypes.AccessKeyLastUsed{
					LastUsedDate: &accessKeyLastUsed,
					ServiceName:  awssdk.String("ec2"),
				},
			}, nil
		},
		listGroupsForUserFunc: func(_ context.Context, params *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
			if awssdk.ToString(params.UserName) != "alice" {
				return nil, fmt.Errorf("unexpected user %q", awssdk.ToString(params.UserName))
			}
			return &iam.ListGroupsForUserOutput{
				Groups: []iamtypes.Group{
					{GroupName: awssdk.String("admins")},
					{GroupName: awssdk.String("devs")},
				},
			}, nil
		},
		listAttachedPoliciesFunc: func(_ context.Context, params *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
			if awssdk.ToString(params.UserName) != "alice" {
				return nil, fmt.Errorf("unexpected user %q", awssdk.ToString(params.UserName))
			}
			return &iam.ListAttachedUserPoliciesOutput{
				AttachedPolicies: []iamtypes.AttachedPolicy{
					{PolicyName: awssdk.String("ReadOnlyAccess")},
				},
			}, nil
		},
		listMFADevicesFunc: func(_ context.Context, params *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
			if awssdk.ToString(params.UserName) != "alice" {
				return nil, fmt.Errorf("unexpected user %q", awssdk.ToString(params.UserName))
			}
			return &iam.ListMFADevicesOutput{
				MFADevices: []iamtypes.MFADevice{
					{UserName: awssdk.String("alice")},
				},
			}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	detail, err := repo.GetIAMUserDetail(context.Background(), "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.UserName != "alice" {
		t.Fatalf("expected alice, got %q", detail.UserName)
	}
	if len(detail.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(detail.Groups))
	}
	if len(detail.AttachedPolicies) != 1 || detail.AttachedPolicies[0] != "ReadOnlyAccess" {
		t.Fatalf("expected ReadOnlyAccess policy, got %#v", detail.AttachedPolicies)
	}
	if len(detail.AccessKeys) != 1 || detail.AccessKeys[0].AccessKeyID != "AKIAALICE" {
		t.Fatalf("expected access key AKIAALICE, got %#v", detail.AccessKeys)
	}
	if !detail.LastActivity.Equal(accessKeyLastUsed) {
		t.Fatalf("expected last activity %v, got %v", accessKeyLastUsed, detail.LastActivity)
	}
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

func TestIAMUserFilterText(t *testing.T) {
	user := IAMUser{
		UserName: "alice",
		UserID:   "AIDAALICE",
		ARN:      "arn:aws:iam::123456789012:user/alice",
		Path:     "/engineering/",
	}

	ft := user.FilterText()
	if !containsIAM(ft, "alice") {
		t.Fatalf("expected filter text to include username, got %q", ft)
	}
	if !containsIAM(ft, "engineering") {
		t.Fatalf("expected filter text to include path, got %q", ft)
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
