package aws

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
