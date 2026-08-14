package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func mfaEnvConfig() *config.Config {
	return &config.Config{
		ContextName: "prod-admin",
		Profile:     "base-profile",
		Region:      "us-east-1",
		AuthType:    config.AuthTypeAssumeRole,
		RoleArn:     "arn:aws:iam::123456789012:role/Admin",
		MFASerial:   "arn:aws:iam::123456789012:mfa/user",
	}
}

func stubMFASeams(t *testing.T) (prompted *bool, assumed *bool) {
	t.Helper()
	origPrompt := promptMFATokenFn
	origCached := cachedMFASessionFn
	origAssume := assumeRoleWithMFAFn
	t.Cleanup(func() {
		promptMFATokenFn = origPrompt
		cachedMFASessionFn = origCached
		assumeRoleWithMFAFn = origAssume
	})
	prompted = new(bool)
	assumed = new(bool)
	promptMFATokenFn = func(string) (string, error) {
		*prompted = true
		return "123456", nil
	}
	cachedMFASessionFn = func(*config.Config) (awsservice.AssumeRoleSession, bool) {
		return awsservice.AssumeRoleSession{}, false
	}
	assumeRoleWithMFAFn = func(_ context.Context, _ *config.Config, code string) (awsservice.AssumeRoleSession, error) {
		if code != "123456" {
			t.Fatalf("expected prompted token code to be used, got %q", code)
		}
		*assumed = true
		return awsservice.AssumeRoleSession{
			AccessKeyID:     "AKIA123",
			SecretAccessKey: "secret",
			SessionToken:    "token",
			Expiration:      time.Now().Add(time.Hour),
		}, nil
	}
	return prompted, assumed
}

func TestAssumeRoleEnvMFAPromptsAndExportsSession(t *testing.T) {
	prompted, assumed := stubMFASeams(t)

	values, err := assumeRoleEnv(context.Background(), mfaEnvConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*prompted || !*assumed {
		t.Fatalf("expected prompt and MFA assume-role call, prompted=%v assumed=%v", *prompted, *assumed)
	}
	if values["AWS_ACCESS_KEY_ID"] != "AKIA123" || values["AWS_SESSION_TOKEN"] != "token" {
		t.Fatalf("expected session exports, got %+v", values)
	}
}

func TestAssumeRoleEnvMFAReusesCachedSessionWithoutPrompt(t *testing.T) {
	prompted, assumed := stubMFASeams(t)
	cachedMFASessionFn = func(*config.Config) (awsservice.AssumeRoleSession, bool) {
		return awsservice.AssumeRoleSession{
			AccessKeyID:     "AKIA-CACHED",
			SecretAccessKey: "secret",
			SessionToken:    "cached-token",
			Expiration:      time.Now().Add(time.Hour),
		}, true
	}

	values, err := assumeRoleEnv(context.Background(), mfaEnvConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *prompted || *assumed {
		t.Fatalf("expected cached session to skip prompting, prompted=%v assumed=%v", *prompted, *assumed)
	}
	if values["AWS_ACCESS_KEY_ID"] != "AKIA-CACHED" {
		t.Fatalf("expected cached session exports, got %+v", values)
	}
}

func TestAssumeRoleEnvMFAPropagatesPromptError(t *testing.T) {
	stubMFASeams(t)
	promptMFATokenFn = func(string) (string, error) {
		return "", errors.New("no tty")
	}

	if _, err := assumeRoleEnv(context.Background(), mfaEnvConfig()); err == nil {
		t.Fatal("expected prompt error to propagate")
	}
}

func TestBuildEnvExportsChainsRoleForProfileBackedContexts(t *testing.T) {
	origAssume := assumeRoleFn
	t.Cleanup(func() { assumeRoleFn = origAssume })
	assumed := false
	assumeRoleFn = func(_ context.Context, cfg *config.Config) (map[string]string, error) {
		assumed = true
		if cfg.RoleArn != "arn:aws:iam::123456789012:role/Chained" {
			t.Fatalf("expected chained role arn, got %q", cfg.RoleArn)
		}
		return map[string]string{
			"AWS_ACCESS_KEY_ID":     "AKIACHAIN",
			"AWS_SECRET_ACCESS_KEY": "secret",
			"AWS_SESSION_TOKEN":     "token",
		}, nil
	}

	for _, authType := range []config.AuthType{config.AuthTypeCredential, config.AuthTypeConsoleLogin} {
		assumed = false
		exports, err := BuildEnvExports(context.Background(), &config.Config{
			ContextName: "chained",
			Profile:     "login-profile",
			Region:      "us-east-1",
			AuthType:    authType,
			RoleArn:     "arn:aws:iam::123456789012:role/Chained",
		})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", authType, err)
		}
		if !assumed {
			t.Fatalf("%s: expected role chaining to assume the role", authType)
		}
		if !strings.Contains(exports, "AKIACHAIN") || !strings.Contains(exports, "unset AWS_PROFILE") {
			t.Fatalf("%s: expected session credential exports without a profile, got:\n%s", authType, exports)
		}
	}
}

func TestBuildEnvExportsCredentialWithoutRoleStillUsesProfile(t *testing.T) {
	exports, err := BuildEnvExports(context.Background(), &config.Config{
		ContextName: "plain",
		Profile:     "plain-profile",
		Region:      "us-east-1",
		AuthType:    config.AuthTypeCredential,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(exports, "export AWS_PROFILE='plain-profile'") {
		t.Fatalf("expected profile export without chaining, got:\n%s", exports)
	}
}
