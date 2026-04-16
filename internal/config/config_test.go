package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeUnicConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCLIFlagsOverrideEverything(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
default_profile: from-config
default_region: us-west-2
`)
	profile := "from-cli"
	region := "ap-northeast-2"
	cfg, err := Load(&profile, &region, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "from-cli" {
		t.Errorf("expected profile 'from-cli', got '%s'", cfg.Profile)
	}
	if cfg.Region != "ap-northeast-2" {
		t.Errorf("expected region 'ap-northeast-2', got '%s'", cfg.Region)
	}
}

func TestFallsBackToConfigWhenNoCLIFlags(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
default_profile: staging
default_region: eu-west-1
`)
	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "staging" {
		t.Errorf("expected profile 'staging', got '%s'", cfg.Profile)
	}
	if cfg.Region != "eu-west-1" {
		t.Errorf("expected region 'eu-west-1', got '%s'", cfg.Region)
	}
}

func TestFallsBackToHardcodedDefaultsWhenNothingSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")
	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "" {
		t.Errorf("expected empty profile, got '%s'", cfg.Profile)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got '%s'", cfg.Region)
	}
}

func TestPartialConfigFillsMissingWithDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
default_profile: prod
`)
	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "prod" {
		t.Errorf("expected profile 'prod', got '%s'", cfg.Profile)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got '%s'", cfg.Region)
	}
}

func TestCLIProfileWithConfigRegion(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
default_region: ap-southeast-1
`)
	profile := "dev"
	cfg, err := Load(&profile, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "dev" {
		t.Errorf("expected profile 'dev', got '%s'", cfg.Profile)
	}
	if cfg.Region != "ap-southeast-1" {
		t.Errorf("expected region 'ap-southeast-1', got '%s'", cfg.Region)
	}
}

func TestCreatesDefaultConfigFileWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unic", "config.yaml")

	if _, err := os.Stat(path); err == nil {
		t.Fatal("config file should not exist yet")
	}

	if err := EnsureConfigExists(path); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatal("config file should exist after EnsureConfigExists")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Error("config file should not be empty")
	}
}

func TestContextBasedConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: dev-sso
defaults:
  region: us-east-1
contexts:
  - name: dev-sso
    profile: dev-sso
  - name: prod-admin
    profile: base-user
`)
	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "dev-sso" {
		t.Errorf("expected profile 'dev-sso', got '%s'", cfg.Profile)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got '%s'", cfg.Region)
	}
}

func TestContextWithRegionOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: tokyo
defaults:
  region: us-east-1
contexts:
  - name: tokyo
    profile: tokyo-profile
    region: ap-northeast-1
`)
	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "tokyo-profile" {
		t.Errorf("expected profile 'tokyo-profile', got '%s'", cfg.Profile)
	}
	if cfg.Region != "ap-northeast-1" {
		t.Errorf("expected region 'ap-northeast-1', got '%s'", cfg.Region)
	}
}

func TestCLIFlagsOverrideContext(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: dev-sso
defaults:
  region: us-east-1
contexts:
  - name: dev-sso
    profile: dev-sso
`)
	profile := "override-profile"
	region := "eu-west-1"
	cfg, err := Load(&profile, &region, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "override-profile" {
		t.Errorf("expected profile 'override-profile', got '%s'", cfg.Profile)
	}
	if cfg.Region != "eu-west-1" {
		t.Errorf("expected region 'eu-west-1', got '%s'", cfg.Region)
	}
}

func TestContextWithRoleArn(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: prod-admin
defaults:
  region: us-east-1
contexts:
  - name: prod-admin
    profile: base-user
    role_arn: arn:aws:iam::111111111111:role/AdministratorAccess
    external_id: my-ext-id
`)
	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "base-user" {
		t.Errorf("expected profile 'base-user', got '%s'", cfg.Profile)
	}
	if cfg.ContextName != "prod-admin" {
		t.Errorf("expected context 'prod-admin', got '%s'", cfg.ContextName)
	}
	if cfg.RoleArn != "arn:aws:iam::111111111111:role/AdministratorAccess" {
		t.Errorf("expected role_arn, got '%s'", cfg.RoleArn)
	}
	if cfg.ExternalID != "my-ext-id" {
		t.Errorf("expected external_id 'my-ext-id', got '%s'", cfg.ExternalID)
	}
}

func TestContextsList(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: dev
contexts:
  - name: dev
    profile: dev-profile
  - name: prod
    profile: prod-profile
    role_arn: arn:aws:iam::222222222222:role/Admin
`)
	infos, err := Contexts(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(infos))
	}
	if !infos[0].Current {
		t.Error("expected first context to be current")
	}
	if infos[1].Current {
		t.Error("expected second context to not be current")
	}
	if infos[1].RoleArn != "arn:aws:iam::222222222222:role/Admin" {
		t.Errorf("expected role_arn on prod context, got '%s'", infos[1].RoleArn)
	}
}

func TestContextsRespectsExplicitOrderBeforeFallbackOrder(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: dev
contexts:
  - name: prod
    profile: prod-profile
  - name: staging
    profile: staging-profile
    order: 20
  - name: dev
    profile: dev-profile
    order: 10
`)

	infos, err := Contexts(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 3 {
		t.Fatalf("expected 3 contexts, got %d", len(infos))
	}
	if infos[0].Name != "dev" || infos[1].Name != "staging" || infos[2].Name != "prod" {
		t.Fatalf("unexpected ordered contexts: %#v", infos)
	}
	if infos[0].Order != 10 || infos[1].Order != 20 || infos[2].Order != 0 {
		t.Fatalf("unexpected order values: %#v", infos)
	}
}

func TestSetContextOrder(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
contexts:
  - name: dev
    profile: dev-profile
  - name: prod
    profile: prod-profile
`)

	if err := SetContextOrder(path, "prod", 5); err != nil {
		t.Fatal(err)
	}

	infos, err := Contexts(path)
	if err != nil {
		t.Fatal(err)
	}
	if infos[0].Name != "prod" {
		t.Fatalf("expected prod to move first after ordering, got %q", infos[0].Name)
	}
	if infos[0].Order != 5 {
		t.Fatalf("expected prod order 5, got %#v", infos[0].Order)
	}
}

func TestSetCurrent(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: dev
contexts:
  - name: dev
    profile: dev-profile
  - name: prod
    profile: prod-profile
`)
	if err := SetCurrent(path, "prod"); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "prod-profile" {
		t.Errorf("expected profile 'prod-profile' after SetCurrent, got '%s'", cfg.Profile)
	}
	if cfg.ContextName != "prod" {
		t.Errorf("expected context 'prod', got '%s'", cfg.ContextName)
	}
}

func TestContextWithSSOAuthType(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: dev-sso
contexts:
  - name: dev-sso
    auth_type: sso
    sso_start_url: https://my-org.awsapps.com/start
    sso_account_id: "111111111111"
    sso_role_name: AdministratorAccess
    region: us-east-1
`)
	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthType != AuthTypeSSO {
		t.Errorf("expected auth_type 'sso', got '%s'", cfg.AuthType)
	}
	if cfg.SSOStartURL != "https://my-org.awsapps.com/start" {
		t.Errorf("expected sso_start_url, got '%s'", cfg.SSOStartURL)
	}
	if cfg.SSOAccountID != "111111111111" {
		t.Errorf("expected sso_account_id '111111111111', got '%s'", cfg.SSOAccountID)
	}
	if cfg.SSORoleName != "AdministratorAccess" {
		t.Errorf("expected sso_role_name 'AdministratorAccess', got '%s'", cfg.SSORoleName)
	}
}

func TestContextWithCredentialAuthType(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: dev-creds
contexts:
  - name: dev-creds
    auth_type: credential
    profile: dev-user
    region: ap-northeast-2
`)
	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthType != AuthTypeCredential {
		t.Errorf("expected auth_type 'credential', got '%s'", cfg.AuthType)
	}
	if cfg.Profile != "dev-user" {
		t.Errorf("expected profile 'dev-user', got '%s'", cfg.Profile)
	}
	if cfg.Region != "ap-northeast-2" {
		t.Errorf("expected region 'ap-northeast-2', got '%s'", cfg.Region)
	}
}

func TestContextWithCredentialsAliasAuthType(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: default-cred
contexts:
  - name: default-cred
    profile: default
    auth_type: credentials
`)
	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthType != AuthTypeCredential {
		t.Errorf("expected auth_type alias 'credentials' to normalize to 'credential', got '%s'", cfg.AuthType)
	}
}

func TestContextWithAssumeRoleAliasAuthType(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: prod-admin
contexts:
  - name: prod-admin
    profile: default
    auth_type: assume-role
    role_arn: arn:aws:iam::111111111111:role/Admin
`)
	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthType != AuthTypeAssumeRole {
		t.Errorf("expected auth_type alias 'assume-role' to normalize to 'assume_role', got '%s'", cfg.AuthType)
	}
}

func TestContextWithAssumeRoleAuthType(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: prod
contexts:
  - name: prod
    auth_type: assume_role
    profile: base-user
    role_arn: arn:aws:iam::333333333333:role/Admin
    region: us-west-2
`)
	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthType != AuthTypeAssumeRole {
		t.Errorf("expected auth_type 'assume_role', got '%s'", cfg.AuthType)
	}
	if cfg.RoleArn != "arn:aws:iam::333333333333:role/Admin" {
		t.Errorf("expected role_arn, got '%s'", cfg.RoleArn)
	}
}

func TestSetCurrentInvalidContext(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
contexts:
  - name: dev
    profile: dev-profile
`)
	err := SetCurrent(path, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent context")
	}
}

func TestMalformedConfigReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `this is not valid yaml: [[[`)

	_, err := Load(nil, nil, path)
	if err == nil {
		t.Error("expected error for malformed config, got nil")
	}
}

func TestUnsetCurrent(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: dev
defaults:
  region: ap-northeast-2
contexts:
  - name: dev
    profile: dev-profile
`)

	if err := UnsetCurrent(path); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextName != "" {
		t.Fatalf("expected empty current context, got %q", cfg.ContextName)
	}
	if cfg.Profile != "" {
		t.Fatalf("expected empty profile after unset, got %q", cfg.Profile)
	}
	if cfg.Region != "ap-northeast-2" {
		t.Fatalf("expected defaults region to remain, got %q", cfg.Region)
	}
}

func TestLoadNamedContext(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
defaults:
  region: us-east-1
contexts:
  - name: dev
    profile: dev-profile
  - name: prod
    auth_type: assume_role
    profile: base
    region: ap-northeast-2
    role_arn: arn:aws:iam::111111111111:role/Admin
`)

	cfg, err := LoadNamedContext(path, "prod")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextName != "prod" {
		t.Fatalf("expected context name prod, got %q", cfg.ContextName)
	}
	if cfg.Profile != "base" {
		t.Fatalf("expected profile base, got %q", cfg.Profile)
	}
	if cfg.Region != "ap-northeast-2" {
		t.Fatalf("expected region ap-northeast-2, got %q", cfg.Region)
	}
	if cfg.AuthType != AuthTypeAssumeRole {
		t.Fatalf("expected assume_role auth type, got %q", cfg.AuthType)
	}
}

func TestUpsertContextAddsEntry(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
defaults:
  region: us-east-1
contexts:
  - name: dev
    profile: dev-profile
`)

	err := UpsertContext(path, ContextEntry{
		Name:     "prod",
		Order:    5,
		Profile:  "prod-profile",
		Region:   "ap-northeast-2",
		AuthType: "credential",
	})
	if err != nil {
		t.Fatal(err)
	}

	infos, err := Contexts(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(infos))
	}
	if infos[0].Name != "prod" {
		t.Fatalf("expected ordered context prod to sort first, got %q", infos[0].Name)
	}
}

func TestUpsertContextReplacesEntry(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `
current: dev
defaults:
  region: us-east-1
contexts:
  - name: dev
    profile: old-profile
    region: us-west-2
`)

	err := UpsertContext(path, ContextEntry{
		Name:     "dev",
		Profile:  "new-profile",
		Region:   "ap-northeast-2",
		AuthType: "credential",
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadNamedContext(path, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "new-profile" {
		t.Fatalf("expected updated profile new-profile, got %q", cfg.Profile)
	}
	if cfg.Region != "ap-northeast-2" {
		t.Fatalf("expected updated region ap-northeast-2, got %q", cfg.Region)
	}
}
