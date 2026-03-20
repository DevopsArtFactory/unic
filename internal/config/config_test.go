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

func TestMalformedConfigReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeUnicConfig(t, dir, `this is not valid yaml: [[[`)

	_, err := Load(nil, nil, path)
	if err == nil {
		t.Error("expected error for malformed config, got nil")
	}
}
