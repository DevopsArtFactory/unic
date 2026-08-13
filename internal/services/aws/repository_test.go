package aws

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"unic/internal/config"
)

func TestLoadBaseConfig_ExplicitProfileOverridesEnvCredentials(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "credentials")
	content := `[test-profile]
aws_access_key_id = PROFILEKEY
aws_secret_access_key = PROFILESECRET
`
	if err := os.WriteFile(credsPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AWS_ACCESS_KEY_ID", "ENVKEY")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "ENVSECRET")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credsPath)
	t.Setenv("AWS_CONFIG_FILE", credsPath)

	cfg, err := LoadBaseConfig(context.Background(), "us-east-1", "test-profile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	creds, err := cfg.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("unexpected credentials error: %v", err)
	}

	if creds.AccessKeyID != "PROFILEKEY" {
		t.Fatalf("expected explicit profile credentials, got %q from %q", creds.AccessKeyID, creds.Source)
	}
}

func TestRepositoryForRegionReusesCredentials(t *testing.T) {
	provider := credentials.NewStaticCredentialsProvider("KEY", "SECRET", "TOKEN")
	repo := newRepositoryFromConfig(awssdk.Config{
		Region:      "ap-northeast-2",
		Credentials: provider,
	}, "ap-northeast-2", "production")

	switched := repo.ForRegion("us-east-1")
	if switched.Region != "us-east-1" {
		t.Fatalf("expected switched region us-east-1, got %q", switched.Region)
	}
	if switched.Profile != "production" {
		t.Fatalf("expected profile to be preserved, got %q", switched.Profile)
	}
	creds, err := switched.awsCfg.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessKeyID != "KEY" || creds.SessionToken != "TOKEN" {
		t.Fatalf("expected credentials to be reused, got %+v", creds)
	}
}

func TestLoadBaseConfig_UsesEnvCredentialsWhenProfileUnset(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "ENVKEY")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "ENVSECRET")

	cfg, err := LoadBaseConfig(context.Background(), "us-east-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	creds, err := cfg.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("unexpected credentials error: %v", err)
	}

	if creds.AccessKeyID != "ENVKEY" {
		t.Fatalf("expected env credentials, got %q from %q", creds.AccessKeyID, creds.Source)
	}
}

func TestNewAwsRepositoryRejectsOktaSAMLForNow(t *testing.T) {
	_, err := NewAwsRepository(context.Background(), &config.Config{
		ContextName: "okta-prod",
		AuthType:    config.AuthTypeOktaSAML,
		Region:      "us-east-1",
	})
	if err == nil || !strings.Contains(err.Error(), "okta_saml") {
		t.Fatalf("expected okta_saml not-implemented error, got %v", err)
	}
}
