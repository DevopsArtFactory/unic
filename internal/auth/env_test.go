package auth

import (
	"context"
	"strings"
	"testing"

	"unic/internal/config"
)

func TestBuildEnvExportsCredentialContext(t *testing.T) {
	cfg := &config.Config{
		ContextName: "dev",
		AuthType:    config.AuthTypeCredential,
		Profile:     "dev-profile",
		Region:      "ap-northeast-2",
	}

	exports, err := BuildEnvExports(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		"export AWS_PROFILE='dev-profile'",
		"export AWS_REGION='ap-northeast-2'",
		"export AWS_DEFAULT_REGION='ap-northeast-2'",
		"unset AWS_ACCESS_KEY_ID",
		"unset AWS_SECRET_ACCESS_KEY",
		"unset AWS_SESSION_TOKEN",
	} {
		if !strings.Contains(exports, expected) {
			t.Fatalf("expected exports to contain %q, got:\n%s", expected, exports)
		}
	}
}

func TestBuildEnvExportsAssumeRoleContext(t *testing.T) {
	orig := assumeRoleFn
	t.Cleanup(func() { assumeRoleFn = orig })

	assumeRoleFn = func(ctx context.Context, cfg *config.Config) (map[string]string, error) {
		return map[string]string{
			"AWS_ACCESS_KEY_ID":     "AKIA123",
			"AWS_SECRET_ACCESS_KEY": "secret",
			"AWS_SESSION_TOKEN":     "token",
		}, nil
	}

	cfg := &config.Config{
		ContextName: "prod",
		AuthType:    config.AuthTypeAssumeRole,
		Profile:     "base",
		Region:      "us-east-1",
		RoleArn:     "arn:aws:iam::111111111111:role/Admin",
	}

	exports, err := BuildEnvExports(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(exports, "export AWS_ACCESS_KEY_ID='AKIA123'") {
		t.Fatalf("expected assumed credentials in exports, got:\n%s", exports)
	}
	if !strings.Contains(exports, "unset AWS_PROFILE") {
		t.Fatalf("expected AWS_PROFILE to be unset for temp credentials, got:\n%s", exports)
	}
}

func TestBuildEnvExportsRejectsIncompleteSSOContext(t *testing.T) {
	cfg := &config.Config{
		ContextName: "base-sso",
		AuthType:    config.AuthTypeSSO,
		Region:      "us-east-1",
		SSOStartURL: "https://example.awsapps.com/start",
	}

	_, err := BuildEnvExports(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "run `unic context setup` first") {
		t.Fatalf("expected incomplete SSO error, got %v", err)
	}
}
