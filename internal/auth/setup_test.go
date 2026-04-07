package auth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSetupContextUpsertsConcreteSSOContext(t *testing.T) {
	origAccounts := listSSOAccountsFn
	origRoles := listSSOAccountRolesFn
	origBuild := buildEnvExportsFn
	defer func() {
		listSSOAccountsFn = origAccounts
		listSSOAccountRolesFn = origRoles
		buildEnvExportsFn = origBuild
	}()

	listSSOAccountsFn = func(ctx context.Context, cfg *config.Config) ([]awsservice.SSOAccount, error) {
		return []awsservice.SSOAccount{{ID: "123456789012", Name: "dev"}}, nil
	}
	listSSOAccountRolesFn = func(ctx context.Context, cfg *config.Config, accountID string) ([]awsservice.SSORole, error) {
		return []awsservice.SSORole{{Name: "AdministratorAccess"}}, nil
	}
	buildEnvExportsFn = func(ctx context.Context, cfg *config.Config) (string, error) {
		return "export AWS_REGION='ap-northeast-2'", nil
	}

	dir := t.TempDir()
	path := writeConfig(t, dir, `
defaults:
  region: ap-northeast-2
contexts:
  - name: base-sso
    auth_type: sso
    sso_start_url: https://example.awsapps.com/start
    region: ap-northeast-2
`)

	var stderr strings.Builder
	exports, err := SetupContext(context.Background(), path, strings.NewReader("1\n1\n1\n"), &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(exports, "export AWS_REGION='ap-northeast-2'") {
		t.Fatalf("expected exports, got %q", exports)
	}

	cfg, err := config.Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextName != "base-sso-123456789012-administratoraccess" {
		t.Fatalf("expected current context to switch to concrete SSO context, got %q", cfg.ContextName)
	}

	infos, err := config.Contexts(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 contexts after upsert, got %d", len(infos))
	}
}

func TestBuildSSOContextEntryReusesMatchingContext(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `
contexts:
  - name: base-sso
    auth_type: sso
    sso_start_url: https://example.awsapps.com/start
    region: ap-northeast-2
  - name: base-sso-123456789012-admin
    auth_type: sso
    sso_start_url: https://example.awsapps.com/start
    sso_account_id: "123456789012"
    sso_role_name: Admin
    region: ap-northeast-2
`)

	entry, name, err := buildSSOContextEntry(path, config.ContextInfo{
		Name:        "base-sso",
		AuthType:    "sso",
		Region:      "ap-northeast-2",
		SSOStartURL: "https://example.awsapps.com/start",
	}, awsservice.SSOAccount{ID: "123456789012"}, awsservice.SSORole{Name: "Admin"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "base-sso-123456789012-admin" {
		t.Fatalf("expected matching context reuse, got %q", name)
	}
	if entry.Name != name {
		t.Fatalf("expected entry name %q, got %q", name, entry.Name)
	}
}
