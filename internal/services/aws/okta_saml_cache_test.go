package aws

import (
	"context"
	"strings"
	"testing"
	"time"

	"unic/internal/config"
)

func oktaCacheTestConfig() *config.Config {
	return &config.Config{
		ContextName: "okta-prod",
		Region:      "us-east-1",
		AuthType:    config.AuthTypeOktaSAML,
		OktaOrgURL:  "https://acme.okta.com",
		OktaAppID:   "amazon_aws/app123/272",
	}
}

func stubOktaCacheDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig := oktaSAMLCacheDirFn
	t.Cleanup(func() { oktaSAMLCacheDirFn = orig })
	oktaSAMLCacheDirFn = func() (string, error) { return dir, nil }
}

func TestOktaSAMLSessionCacheRoundTripAndExpiry(t *testing.T) {
	stubOktaCacheDir(t)
	cfg := oktaCacheTestConfig()

	if _, ok := CachedOktaSAMLSession(cfg); ok {
		t.Fatal("expected empty cache before save")
	}

	session := OktaSAMLSession{
		AccessKeyID:     "AKIAOKTA",
		SecretAccessKey: "secret",
		SessionToken:    "token",
		Expiration:      time.Now().Add(time.Hour),
	}
	if err := SaveOktaSAMLSession(cfg, session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := CachedOktaSAMLSession(cfg)
	if !ok || got.AccessKeyID != "AKIAOKTA" {
		t.Fatalf("expected cached session, got %+v ok=%v", got, ok)
	}

	expired := session
	expired.Expiration = time.Now().Add(30 * time.Second)
	if err := SaveOktaSAMLSession(cfg, expired); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := CachedOktaSAMLSession(cfg); ok {
		t.Fatal("expected near-expiry session to be rejected")
	}
}

func TestNewAwsRepositoryOktaSAMLUsesCachedSession(t *testing.T) {
	stubOktaCacheDir(t)
	cfg := oktaCacheTestConfig()

	_, err := NewAwsRepository(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "unic env okta-prod") {
		t.Fatalf("expected cache-miss error pointing at unic env, got %v", err)
	}

	session := OktaSAMLSession{
		AccessKeyID:     "AKIAOKTA",
		SecretAccessKey: "secret",
		SessionToken:    "token",
		Expiration:      time.Now().Add(time.Hour),
	}
	if err := SaveOktaSAMLSession(cfg, session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo, err := NewAwsRepository(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	creds, err := repo.awsCfg.Credentials.Retrieve(context.Background())
	if err != nil || creds.AccessKeyID != "AKIAOKTA" {
		t.Fatalf("expected cached credentials in repository, got %+v err=%v", creds, err)
	}
}
