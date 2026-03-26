package aws

import (
	"context"
	"fmt"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// mockSecretsManagerClient implements SecretsManagerClientAPI for testing.
type mockSecretsManagerClient struct {
	listSecretsFunc    func(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
	getSecretValueFunc func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

func (m *mockSecretsManagerClient) ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	return m.listSecretsFunc(ctx, params, optFns...)
}

func (m *mockSecretsManagerClient) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	return m.getSecretValueFunc(ctx, params, optFns...)
}

// --- ListSecrets tests ---

func TestListSecrets_Success(t *testing.T) {
	mock := &mockSecretsManagerClient{
		listSecretsFunc: func(_ context.Context, _ *secretsmanager.ListSecretsInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
			return &secretsmanager.ListSecretsOutput{
				SecretList: []smtypes.SecretListEntry{
					{
						Name:        awssdk.String("prod/db/password"),
						ARN:         awssdk.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password"),
						Description: awssdk.String("Production DB password"),
						KmsKeyId:    awssdk.String("alias/aws/secretsmanager"),
					},
					{
						Name:        awssdk.String("dev/api/key"),
						ARN:         awssdk.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:dev/api/key"),
						Description: nil,
						KmsKeyId:    nil,
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
	if len(secrets) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(secrets))
	}

	s := secrets[0]
	if s.Name != "prod/db/password" {
		t.Errorf("expected Name 'prod/db/password', got %q", s.Name)
	}
	if s.Description != "Production DB password" {
		t.Errorf("expected Description 'Production DB password', got %q", s.Description)
	}
	if s.KMSKeyID != "alias/aws/secretsmanager" {
		t.Errorf("expected KMSKeyID 'alias/aws/secretsmanager', got %q", s.KMSKeyID)
	}

	s2 := secrets[1]
	if s2.KMSKeyID != "" {
		t.Errorf("expected empty KMSKeyID for nil input, got %q", s2.KMSKeyID)
	}
}

func TestListSecrets_Empty(t *testing.T) {
	mock := &mockSecretsManagerClient{
		listSecretsFunc: func(_ context.Context, _ *secretsmanager.ListSecretsInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
			return &secretsmanager.ListSecretsOutput{SecretList: []smtypes.SecretListEntry{}}, nil
		},
	}

	repo := &AwsRepository{SecretsManagerClient: mock}
	secrets, err := repo.ListSecrets(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(secrets) != 0 {
		t.Errorf("expected empty slice, got %d", len(secrets))
	}
}

func TestListSecrets_Error(t *testing.T) {
	mock := &mockSecretsManagerClient{
		listSecretsFunc: func(_ context.Context, _ *secretsmanager.ListSecretsInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}

	repo := &AwsRepository{SecretsManagerClient: mock}
	_, err := repo.ListSecrets(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetSecretDetail tests ---

func TestGetSecretDetail_JSONValue(t *testing.T) {
	mock := &mockSecretsManagerClient{
		getSecretValueFunc: func(_ context.Context, params *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			if awssdk.ToString(params.SecretId) != "prod/db/password" {
				t.Errorf("expected SecretId 'prod/db/password', got %q", awssdk.ToString(params.SecretId))
			}
			return &secretsmanager.GetSecretValueOutput{
				Name:         awssdk.String("prod/db/password"),
				ARN:          awssdk.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password"),
				SecretString: awssdk.String(`{"username":"admin","password":"s3cr3t"}`),
			}, nil
		},
	}

	repo := &AwsRepository{SecretsManagerClient: mock}
	detail, err := repo.GetSecretDetail(context.Background(), "prod/db/password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Name != "prod/db/password" {
		t.Errorf("expected Name 'prod/db/password', got %q", detail.Name)
	}
	if detail.Values["username"] != "admin" {
		t.Errorf("expected username 'admin', got %q", detail.Values["username"])
	}
	if detail.Values["password"] != "s3cr3t" {
		t.Errorf("expected password 's3cr3t', got %q", detail.Values["password"])
	}
}

func TestGetSecretDetail_PlainStringValue(t *testing.T) {
	mock := &mockSecretsManagerClient{
		getSecretValueFunc: func(_ context.Context, _ *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return &secretsmanager.GetSecretValueOutput{
				Name:         awssdk.String("my-token"),
				ARN:          awssdk.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:my-token"),
				SecretString: awssdk.String("plain-text-token"),
			}, nil
		},
	}

	repo := &AwsRepository{SecretsManagerClient: mock}
	detail, err := repo.GetSecretDetail(context.Background(), "my-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Raw != "plain-text-token" {
		t.Errorf("expected Raw 'plain-text-token', got %q", detail.Raw)
	}
	if len(detail.Values) != 0 {
		t.Errorf("expected empty Values for non-JSON secret, got %v", detail.Values)
	}
}

func TestGetSecretDetail_Error(t *testing.T) {
	mock := &mockSecretsManagerClient{
		getSecretValueFunc: func(_ context.Context, _ *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return nil, fmt.Errorf("ResourceNotFoundException")
		},
	}

	repo := &AwsRepository{SecretsManagerClient: mock}
	_, err := repo.GetSecretDetail(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Model tests ---

func TestSecretDisplayTitle_WithDescription(t *testing.T) {
	s := Secret{Name: "prod/db/password", Description: "Production DB password"}
	expected := "prod/db/password — Production DB password"
	if got := s.DisplayTitle(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestSecretDisplayTitle_NoDescription(t *testing.T) {
	s := Secret{Name: "dev/api/key"}
	if got := s.DisplayTitle(); got != "dev/api/key" {
		t.Errorf("expected 'dev/api/key', got %q", got)
	}
}

func TestSecretFilterText(t *testing.T) {
	s := Secret{
		Name:        "Prod/DB/Password",
		Description: "Production Database",
		ARN:         "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod",
	}
	ft := s.FilterText()
	for _, kw := range []string{"prod/db/password", "production database", "arn:aws"} {
		if !strings.Contains(ft, kw) {
			t.Errorf("FilterText %q should contain %q", ft, kw)
		}
	}
}
