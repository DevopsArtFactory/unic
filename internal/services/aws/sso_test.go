package aws

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"unic/internal/config"
)

const testSSOStartURL = "https://example.awsapps.com/start"

func testSSOConfig() *config.Config {
	return &config.Config{
		ContextName:  "dev-sso",
		AuthType:     config.AuthTypeSSO,
		Region:       "us-east-1",
		SSOStartURL:  testSSOStartURL,
		SSOAccountID: "123456789012",
		SSORoleName:  "AdministratorAccess",
	}
}

func writeTestSSOToken(t *testing.T, home string, startURL string, expiresAt time.Time) {
	t.Helper()
	cacheDir := filepath.Join(home, ".aws", "sso", "cache")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatal(err)
	}

	token := ssoTokenCache{
		AccessToken: "cached-token",
		ExpiresAt:   expiresAt.UTC().Format(time.RFC3339),
		Region:      "us-east-1",
		StartURL:    startURL,
	}
	data, err := json.Marshal(token)
	if err != nil {
		t.Fatal(err)
	}

	hash := sha1.Sum([]byte(startURL))
	filename := hex.EncodeToString(hash[:]) + ".json"
	if err := os.WriteFile(filepath.Join(cacheDir, filename), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSSOSessionUsesValidCachedToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeTestSSOToken(t, home, testSSOStartURL, time.Now().Add(time.Hour))

	check, err := CheckSSOSession(testSSOConfig())
	if err != nil {
		t.Fatalf("expected session check to succeed, got %v", err)
	}
	if check.LoginRequired {
		t.Fatal("expected valid cached SSO token to skip login")
	}
	if check.StartURL != testSSOStartURL {
		t.Fatalf("expected start URL %q, got %q", testSSOStartURL, check.StartURL)
	}
}

func TestEnsureSSOLoginReusesValidCachedToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeTestSSOToken(t, home, testSSOStartURL, time.Now().Add(time.Hour))

	origRunLogin := runSSOLoginFn
	defer func() { runSSOLoginFn = origRunLogin }()
	runSSOLoginFn = func(*config.Config) error {
		t.Fatal("RunSSOLogin should not be called with a valid cached token")
		return nil
	}

	result, err := EnsureSSOLogin(testSSOConfig())
	if err != nil {
		t.Fatalf("expected cached SSO login to succeed, got %v", err)
	}
	if result.Refreshed {
		t.Fatal("expected cached SSO token to be reused")
	}
	if result.StartURL != testSSOStartURL {
		t.Fatalf("expected start URL %q, got %q", testSSOStartURL, result.StartURL)
	}
}

func TestEnsureSSOLoginRunsLoginWhenTokenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	origRunLogin := runSSOLoginFn
	defer func() { runSSOLoginFn = origRunLogin }()
	loginCalls := 0
	runSSOLoginFn = func(*config.Config) error {
		loginCalls++
		writeTestSSOToken(t, home, testSSOStartURL, time.Now().Add(time.Hour))
		return nil
	}

	result, err := EnsureSSOLogin(testSSOConfig())
	if err != nil {
		t.Fatalf("expected missing SSO token to trigger login, got %v", err)
	}
	if loginCalls != 1 {
		t.Fatalf("expected one login call, got %d", loginCalls)
	}
	if !result.Refreshed {
		t.Fatal("expected login result to report a refreshed session")
	}
}

func TestEnsureSSOLoginRunsLoginWhenTokenExpired(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeTestSSOToken(t, home, testSSOStartURL, time.Now().Add(-time.Hour))

	origRunLogin := runSSOLoginFn
	defer func() { runSSOLoginFn = origRunLogin }()
	loginCalls := 0
	runSSOLoginFn = func(*config.Config) error {
		loginCalls++
		writeTestSSOToken(t, home, testSSOStartURL, time.Now().Add(time.Hour))
		return nil
	}

	result, err := EnsureSSOLogin(testSSOConfig())
	if err != nil {
		t.Fatalf("expected expired SSO token to trigger login, got %v", err)
	}
	if loginCalls != 1 {
		t.Fatalf("expected one login call, got %d", loginCalls)
	}
	if !result.Refreshed {
		t.Fatal("expected login result to report a refreshed session")
	}
}
