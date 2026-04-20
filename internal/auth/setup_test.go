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
	origConsoleLogin := runConsoleLoginFn
	defer func() {
		listSSOAccountsFn = origAccounts
		listSSOAccountRolesFn = origRoles
		buildEnvExportsFn = origBuild
		runConsoleLoginFn = origConsoleLogin
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
	runConsoleLoginFn = func(cfg *config.Config) error { return nil }

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

func TestSetupContextRunsConsoleLoginForConsoleLoginContext(t *testing.T) {
	origBuild := buildEnvExportsFn
	origConsoleLogin := runConsoleLoginFn
	defer func() {
		buildEnvExportsFn = origBuild
		runConsoleLoginFn = origConsoleLogin
	}()

	var ran bool
	buildEnvExportsFn = func(ctx context.Context, cfg *config.Config) (string, error) {
		return "export AWS_PROFILE='local-dev'", nil
	}
	runConsoleLoginFn = func(cfg *config.Config) error {
		ran = true
		if cfg.Profile != "local-dev" {
			t.Fatalf("expected local-dev profile, got %q", cfg.Profile)
		}
		return nil
	}

	dir := t.TempDir()
	path := writeConfig(t, dir, `
defaults:
  region: ap-northeast-2
contexts:
  - name: local-dev
    auth_type: console_login
    profile: local-dev
    region: ap-northeast-2
`)

	var stderr strings.Builder
	exports, err := SetupContext(context.Background(), path, strings.NewReader("1\n"), &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("expected console login to run")
	}
	if !strings.Contains(exports, "export AWS_PROFILE='local-dev'") {
		t.Fatalf("expected exports, got %q", exports)
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

func TestSetupContextSupportsSearchBeforeSelectingContext(t *testing.T) {
	origBuild := buildEnvExportsFn
	defer func() {
		buildEnvExportsFn = origBuild
	}()

	buildEnvExportsFn = func(ctx context.Context, cfg *config.Config) (string, error) {
		return "export UNIC_CONTEXT='" + cfg.ContextName + "'", nil
	}

	dir := t.TempDir()
	path := writeConfig(t, dir, `
current: prod
defaults:
  region: ap-northeast-2
contexts:
  - name: dev
    auth_type: credential
    profile: dev
    region: ap-northeast-2
  - name: prod
    auth_type: credential
    profile: prod
    region: us-east-1
  - name: staging
    auth_type: credential
    profile: staging
    region: ap-southeast-1
`)

	var stderr strings.Builder
	exports, err := SetupContext(context.Background(), path, strings.NewReader("stag\n1\n"), &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if exports != "export UNIC_CONTEXT='staging'" {
		t.Fatalf("unexpected exports: %q", exports)
	}

	cfg, err := config.Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextName != "staging" {
		t.Fatalf("expected current context staging, got %q", cfg.ContextName)
	}
	if !strings.Contains(stderr.String(), `Available contexts (filtered by "stag")`) {
		t.Fatalf("expected filtered context list in stderr, got %q", stderr.String())
	}
}

func TestSetupContextSupportsSearchBeforeSelectingSSOAccountAndRole(t *testing.T) {
	origAccounts := listSSOAccountsFn
	origRoles := listSSOAccountRolesFn
	origBuild := buildEnvExportsFn
	defer func() {
		listSSOAccountsFn = origAccounts
		listSSOAccountRolesFn = origRoles
		buildEnvExportsFn = origBuild
	}()

	listSSOAccountsFn = func(ctx context.Context, cfg *config.Config) ([]awsservice.SSOAccount, error) {
		return []awsservice.SSOAccount{
			{ID: "111111111111", Name: "sandbox"},
			{ID: "222222222222", Name: "production"},
		}, nil
	}
	listSSOAccountRolesFn = func(ctx context.Context, cfg *config.Config, accountID string) ([]awsservice.SSORole, error) {
		return []awsservice.SSORole{
			{Name: "ReadOnly"},
			{Name: "AdministratorAccess"},
		}, nil
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
	_, err := SetupContext(context.Background(), path, strings.NewReader("1\nprod\n1\nadmin\n1\n"), &stderr)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextName != "base-sso-222222222222-administratoraccess" {
		t.Fatalf("expected resolved SSO context, got %q", cfg.ContextName)
	}
	output := stderr.String()
	if !strings.Contains(output, `Available AWS accounts (filtered by "prod")`) {
		t.Fatalf("expected filtered account list, got %q", output)
	}
	if !strings.Contains(output, `Available AWS roles (filtered by "admin")`) {
		t.Fatalf("expected filtered role list, got %q", output)
	}
}
