package aws

import (
	"context"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"

	"unic/internal/config"
)

func mfaTestConfig() *config.Config {
	return &config.Config{
		ContextName: "prod-admin",
		Region:      "us-east-1",
		AuthType:    config.AuthTypeAssumeRole,
		RoleArn:     "arn:aws:iam::123456789012:role/Admin",
		MFASerial:   "arn:aws:iam::123456789012:mfa/user",
	}
}

func stubCacheDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig := assumeRoleCacheDirFn
	t.Cleanup(func() { assumeRoleCacheDirFn = orig })
	assumeRoleCacheDirFn = func() (string, error) { return dir, nil }
}

func TestAssumeRoleSessionCacheRoundTrip(t *testing.T) {
	stubCacheDir(t)
	cfg := mfaTestConfig()

	if _, ok := CachedAssumeRoleSession(cfg); ok {
		t.Fatal("expected empty cache before save")
	}

	session := AssumeRoleSession{
		AccessKeyID:     "AKIA123",
		SecretAccessKey: "secret",
		SessionToken:    "token",
		Expiration:      time.Now().Add(time.Hour),
	}
	if err := saveAssumeRoleSession(cfg, session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := CachedAssumeRoleSession(cfg)
	if !ok || got.AccessKeyID != "AKIA123" || got.SessionToken != "token" {
		t.Fatalf("expected cached session back, got %+v ok=%v", got, ok)
	}

	other := mfaTestConfig()
	other.RoleArn = "arn:aws:iam::123456789012:role/ReadOnly"
	if _, ok := CachedAssumeRoleSession(other); ok {
		t.Fatal("expected cache miss for a different role")
	}
}

func TestCachedAssumeRoleSessionRejectsExpired(t *testing.T) {
	stubCacheDir(t)
	cfg := mfaTestConfig()

	session := AssumeRoleSession{
		AccessKeyID:     "AKIA123",
		SecretAccessKey: "secret",
		SessionToken:    "token",
		Expiration:      time.Now().Add(30 * time.Second),
	}
	if err := saveAssumeRoleSession(cfg, session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := CachedAssumeRoleSession(cfg); ok {
		t.Fatal("expected session expiring within the skew margin to be rejected")
	}
}

func TestAssumeRoleWithMFACachesSession(t *testing.T) {
	stubCacheDir(t)
	cfg := mfaTestConfig()

	origSTS := stsAssumeRoleFn
	t.Cleanup(func() { stsAssumeRoleFn = origSTS })
	expiry := time.Now().Add(time.Hour)
	stsAssumeRoleFn = func(_ context.Context, _ awssdk.Config, input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
		if awssdk.ToString(input.SerialNumber) != cfg.MFASerial {
			t.Fatalf("expected MFA serial to be passed, got %+v", input)
		}
		if awssdk.ToString(input.TokenCode) != "123456" {
			t.Fatalf("expected token code to be passed, got %+v", input)
		}
		return &sts.AssumeRoleOutput{Credentials: &ststypes.Credentials{
			AccessKeyId:     awssdk.String("AKIA456"),
			SecretAccessKey: awssdk.String("secret"),
			SessionToken:    awssdk.String("token"),
			Expiration:      &expiry,
		}}, nil
	}

	session, err := AssumeRoleWithMFA(context.Background(), cfg, "123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.AccessKeyID != "AKIA456" {
		t.Fatalf("expected session credentials, got %+v", session)
	}

	cached, ok := CachedAssumeRoleSession(cfg)
	if !ok || cached.AccessKeyID != "AKIA456" {
		t.Fatalf("expected session to be cached, got %+v ok=%v", cached, ok)
	}
}

func TestResolveAssumeRoleCredentialsMFACacheMiss(t *testing.T) {
	stubCacheDir(t)
	cfg := mfaTestConfig()

	_, err := resolveAssumeRoleCredentials(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "requires MFA") || !strings.Contains(err.Error(), "unic env prod-admin") {
		t.Fatalf("expected actionable MFA error, got %v", err)
	}
}

func TestResolveAssumeRoleCredentialsMFAUsesCachedSession(t *testing.T) {
	stubCacheDir(t)
	cfg := mfaTestConfig()

	session := AssumeRoleSession{
		AccessKeyID:     "AKIA789",
		SecretAccessKey: "secret",
		SessionToken:    "token",
		Expiration:      time.Now().Add(time.Hour),
	}
	if err := saveAssumeRoleSession(cfg, session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	awsCfg, err := resolveAssumeRoleCredentials(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	creds, err := awsCfg.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error retrieving credentials: %v", err)
	}
	if creds.AccessKeyID != "AKIA789" || creds.SessionToken != "token" {
		t.Fatalf("expected cached static credentials, got %+v", creds)
	}
}
