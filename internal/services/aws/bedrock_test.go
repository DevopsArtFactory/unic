package aws

import (
	"context"
	"fmt"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

func TestListBedrockAPIKeys_Success(t *testing.T) {
	createdA := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	createdB := time.Date(2026, 1, 3, 3, 4, 5, 0, time.UTC)
	expires := time.Date(2026, 2, 2, 3, 4, 5, 0, time.UTC)

	mock := &mockIAMClient{
		listServiceCredsFunc: func(_ context.Context, params *iam.ListServiceSpecificCredentialsInput, _ ...func(*iam.Options)) (*iam.ListServiceSpecificCredentialsOutput, error) {
			if awssdk.ToString(params.ServiceName) != BedrockServiceSpecificCredentialName {
				t.Fatalf("expected service %s, got %q", BedrockServiceSpecificCredentialName, awssdk.ToString(params.ServiceName))
			}
			if !awssdk.ToBool(params.AllUsers) {
				t.Fatal("expected all-users list")
			}
			return &iam.ListServiceSpecificCredentialsOutput{
				ServiceSpecificCredentials: []iamtypes.ServiceSpecificCredentialMetadata{
					{
						ServiceSpecificCredentialId: awssdk.String("ACCA2"),
						UserName:                    awssdk.String("zeta"),
						ServiceName:                 awssdk.String(BedrockServiceSpecificCredentialName),
						ServiceCredentialAlias:      awssdk.String("zeta-key"),
						Status:                      iamtypes.StatusTypeActive,
						CreateDate:                  &createdA,
						ExpirationDate:              &expires,
					},
					{
						ServiceSpecificCredentialId: awssdk.String("ACCA1"),
						UserName:                    awssdk.String("alpha"),
						ServiceName:                 awssdk.String(BedrockServiceSpecificCredentialName),
						ServiceCredentialAlias:      awssdk.String("alpha-key"),
						Status:                      iamtypes.StatusTypeActive,
						CreateDate:                  &createdB,
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	keys, err := repo.ListBedrockAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0].UserName != "alpha" || keys[0].CredentialID != "ACCA1" {
		t.Fatalf("expected alpha key first, got %+v", keys[0])
	}
	if keys[1].ExpiresDisplay() != "2026-02-02" {
		t.Fatalf("unexpected expiration display: %q", keys[1].ExpiresDisplay())
	}
}

func TestListBedrockAPIKeys_FallsBackToCurrentUser(t *testing.T) {
	calls := 0
	mock := &mockIAMClient{
		listServiceCredsFunc: func(_ context.Context, params *iam.ListServiceSpecificCredentialsInput, _ ...func(*iam.Options)) (*iam.ListServiceSpecificCredentialsOutput, error) {
			calls++
			if awssdk.ToBool(params.AllUsers) {
				return nil, fmt.Errorf("access denied")
			}
			return &iam.ListServiceSpecificCredentialsOutput{
				ServiceSpecificCredentials: []iamtypes.ServiceSpecificCredentialMetadata{
					{
						ServiceSpecificCredentialId: awssdk.String("ACCA1"),
						UserName:                    awssdk.String("self"),
						ServiceName:                 awssdk.String(BedrockServiceSpecificCredentialName),
						Status:                      iamtypes.StatusTypeActive,
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	keys, err := repo.ListBedrockAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected two list attempts, got %d", calls)
	}
	if len(keys) != 1 || keys[0].UserName != "self" {
		t.Fatalf("unexpected fallback keys: %+v", keys)
	}
}

func TestCreateBedrockAPIKey_Success(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	mock := &mockIAMClient{
		createServiceCredFunc: func(_ context.Context, params *iam.CreateServiceSpecificCredentialInput, _ ...func(*iam.Options)) (*iam.CreateServiceSpecificCredentialOutput, error) {
			if awssdk.ToString(params.UserName) != "bedrock-user" {
				t.Fatalf("expected user bedrock-user, got %q", awssdk.ToString(params.UserName))
			}
			if awssdk.ToString(params.ServiceName) != BedrockServiceSpecificCredentialName {
				t.Fatalf("expected service %s, got %q", BedrockServiceSpecificCredentialName, awssdk.ToString(params.ServiceName))
			}
			if awssdk.ToInt32(params.CredentialAgeDays) != 30 {
				t.Fatalf("expected 30 age days, got %d", awssdk.ToInt32(params.CredentialAgeDays))
			}
			return &iam.CreateServiceSpecificCredentialOutput{
				ServiceSpecificCredential: &iamtypes.ServiceSpecificCredential{
					ServiceSpecificCredentialId: awssdk.String("ACCA123"),
					UserName:                    awssdk.String("bedrock-user"),
					ServiceName:                 awssdk.String(BedrockServiceSpecificCredentialName),
					ServiceCredentialAlias:      awssdk.String("bedrock-user+v1"),
					ServiceCredentialSecret:     awssdk.String("secret-token"),
					Status:                      iamtypes.StatusTypeActive,
					CreateDate:                  &created,
				},
			}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	key, err := repo.CreateBedrockAPIKey(context.Background(), " bedrock-user ", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.CredentialID != "ACCA123" || key.Secret != "secret-token" {
		t.Fatalf("unexpected generated key: %+v", key)
	}
	if key.EnvExport() != "export AWS_BEARER_TOKEN_BEDROCK=secret-token" {
		t.Fatalf("unexpected env export: %q", key.EnvExport())
	}
}

func TestRotateBedrockAPIKey_Success(t *testing.T) {
	mock := &mockIAMClient{
		resetServiceCredFunc: func(_ context.Context, params *iam.ResetServiceSpecificCredentialInput, _ ...func(*iam.Options)) (*iam.ResetServiceSpecificCredentialOutput, error) {
			if awssdk.ToString(params.UserName) != "bedrock-user" {
				t.Fatalf("expected user bedrock-user, got %q", awssdk.ToString(params.UserName))
			}
			if awssdk.ToString(params.ServiceSpecificCredentialId) != "ACCA123" {
				t.Fatalf("expected credential ID ACCA123, got %q", awssdk.ToString(params.ServiceSpecificCredentialId))
			}
			return &iam.ResetServiceSpecificCredentialOutput{
				ServiceSpecificCredential: &iamtypes.ServiceSpecificCredential{
					ServiceSpecificCredentialId: awssdk.String("ACCA123"),
					UserName:                    awssdk.String("bedrock-user"),
					ServiceName:                 awssdk.String(BedrockServiceSpecificCredentialName),
					ServiceCredentialSecret:     awssdk.String("rotated-secret"),
					Status:                      iamtypes.StatusTypeActive,
				},
			}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	key, err := repo.RotateBedrockAPIKey(context.Background(), "bedrock-user", "ACCA123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.Secret != "rotated-secret" {
		t.Fatalf("expected rotated secret, got %q", key.Secret)
	}
}

func TestDeleteBedrockAPIKey_Success(t *testing.T) {
	called := false
	mock := &mockIAMClient{
		deleteServiceCredFunc: func(_ context.Context, params *iam.DeleteServiceSpecificCredentialInput, _ ...func(*iam.Options)) (*iam.DeleteServiceSpecificCredentialOutput, error) {
			called = true
			if awssdk.ToString(params.UserName) != "bedrock-user" {
				t.Fatalf("expected user bedrock-user, got %q", awssdk.ToString(params.UserName))
			}
			if awssdk.ToString(params.ServiceSpecificCredentialId) != "ACCA123" {
				t.Fatalf("expected credential ID ACCA123, got %q", awssdk.ToString(params.ServiceSpecificCredentialId))
			}
			return &iam.DeleteServiceSpecificCredentialOutput{}, nil
		},
	}

	repo := &AwsRepository{IAMClient: mock}
	if err := repo.DeleteBedrockAPIKey(context.Background(), "bedrock-user", "ACCA123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected delete call")
	}
}
