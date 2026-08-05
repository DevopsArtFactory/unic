package auth

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func TestSetupContextSelectsResourceRegionForShellSession(t *testing.T) {
	origBuild := buildEnvExportsFn
	defer func() { buildEnvExportsFn = origBuild }()

	var exportedRegion string
	buildEnvExportsFn = func(_ context.Context, cfg *config.Config) (string, error) {
		exportedRegion = cfg.Region
		return "export AWS_REGION='" + cfg.Region + "'", nil
	}

	dir := t.TempDir()
	path := writeConfig(t, dir, `
current: old
contexts:
  - name: old
    auth_type: credential
    profile: old
    region: ap-southeast-1
  - name: production
    auth:
      type: credential
      profile: production
    resources:
      default_region: ap-northeast-2
      regions:
        - us-east-1
`)

	var stderr strings.Builder
	exports, err := SetupContext(context.Background(), path, strings.NewReader("2\n2\n"), &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if exportedRegion != "us-east-1" || !strings.Contains(exports, "us-east-1") {
		t.Fatalf("expected selected shell region us-east-1, got region=%q exports=%q", exportedRegion, exports)
	}
	if !strings.Contains(stderr.String(), "Available resource regions") {
		t.Fatalf("expected resource region picker, got %q", stderr.String())
	}

	stored, err := config.LoadNamedContext(path, "production")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Region != "ap-northeast-2" {
		t.Fatalf("expected persisted default region to remain unchanged, got %q", stored.Region)
	}
	current, err := config.Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContextName != "production" {
		t.Fatalf("expected current context production, got %q", current.ContextName)
	}
}

func TestSetupContextRegionCancellationDoesNotChangeCurrentContext(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `
current: old
contexts:
  - name: old
    auth_type: credential
    profile: old
    region: ap-southeast-1
  - name: production
    auth_type: credential
    profile: production
    region: ap-northeast-2
    regions:
      - us-east-1
`)

	_, err := SetupContext(context.Background(), path, strings.NewReader("2\n"), &strings.Builder{})
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected region selection cancellation, got %v", err)
	}
	cfg, loadErr := config.Load(nil, nil, path)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if cfg.ContextName != "old" {
		t.Fatalf("expected current context to remain old, got %q", cfg.ContextName)
	}
}

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
		return "export AWS_REGION='" + cfg.Region + "'", nil
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
    regions:
      - us-east-1
`)

	var stderr strings.Builder
	exports, err := SetupContext(context.Background(), path, strings.NewReader("1\n1\n1\n2\n"), &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(exports, "export AWS_REGION='us-east-1'") {
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
	concrete, err := config.LoadNamedContext(path, "base-sso-123456789012-administratoraccess")
	if err != nil {
		t.Fatal(err)
	}
	if concrete.Region != "ap-northeast-2" || len(concrete.Regions) != 2 || concrete.Regions[1] != "us-east-1" {
		t.Fatalf("expected generated context to preserve default and additional regions, got default=%q regions=%v", concrete.Region, concrete.Regions)
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

func TestBuildSSOContextEntryDoesNotDuplicateDefaultRegion(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `
contexts:
  - name: base-sso
    auth_type: sso
    sso_start_url: https://example.awsapps.com/start
    region: ap-northeast-2
`)

	entry, _, err := buildSSOContextEntry(path, config.ContextInfo{
		Name:        "base-sso",
		AuthType:    "sso",
		Region:      "ap-northeast-2",
		Regions:     []string{"ap-northeast-2", "us-east-1", "eu-west-1"},
		SSOStartURL: "https://example.awsapps.com/start",
	}, awsservice.SSOAccount{ID: "123456789012"}, awsservice.SSORole{Name: "Admin"})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Resources == nil {
		t.Fatal("expected structured resources")
	}
	if got := entry.Resources.Regions; len(got) != 2 || got[0] != "us-east-1" || got[1] != "eu-west-1" {
		t.Fatalf("expected only additional regions, got %v", got)
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
