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
		"export UNIC_CONTEXT='dev'",
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
	if !strings.Contains(exports, "export UNIC_CONTEXT='prod'") {
		t.Fatalf("expected UNIC_CONTEXT marker in exports, got:\n%s", exports)
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

func TestBuildEnvCleanupCommandsIncludesUNICContext(t *testing.T) {
	exports := BuildEnvCleanupCommands()
	for _, expected := range []string{
		"unset AWS_PROFILE",
		"unset AWS_REGION",
		"unset AWS_DEFAULT_REGION",
		"unset AWS_ACCESS_KEY_ID",
		"unset AWS_SECRET_ACCESS_KEY",
		"unset AWS_SESSION_TOKEN",
		"unset UNIC_CONTEXT",
	} {
		if !strings.Contains(exports, expected) {
			t.Fatalf("expected cleanup commands to contain %q, got:\n%s", expected, exports)
		}
	}
}

func TestDetectEnvContextPrefersUNICContextMarker(t *testing.T) {
	contexts := []config.ContextInfo{
		{Name: "dev", Profile: "dev-profile", Region: "ap-northeast-2"},
	}

	detection := DetectEnvContext(contexts, func(key string) string {
		if key == ContextEnvVar {
			return "dev"
		}
		if key == "AWS_PROFILE" {
			return "wrong"
		}
		return ""
	})

	if detection.Name != "dev" || detection.Source != ContextEnvVar || !detection.Known {
		t.Fatalf("unexpected detection: %+v", detection)
	}
}

func TestDetectEnvContextFallsBackToAWSProfile(t *testing.T) {
	contexts := []config.ContextInfo{
		{Name: "dev", Profile: "dev-profile", Region: "ap-northeast-2"},
		{Name: "prod", Profile: "prod-profile", Region: "us-east-1"},
	}

	detection := DetectEnvContext(contexts, func(key string) string {
		switch key {
		case ContextEnvVar:
			return ""
		case "AWS_PROFILE":
			return "dev-profile"
		case "AWS_REGION":
			return "ap-northeast-2"
		default:
			return ""
		}
	})

	if detection.Name != "dev" || detection.Source != "AWS_PROFILE" || !detection.Known {
		t.Fatalf("unexpected detection: %+v", detection)
	}
}
