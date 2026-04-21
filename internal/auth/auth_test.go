package auth

import (
	"strings"
	"testing"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func testPostSwitchSSOCfg() *config.Config {
	return &config.Config{
		ContextName:  "dev-sso",
		AuthType:     config.AuthTypeSSO,
		Region:       "us-east-1",
		SSOStartURL:  "https://example.awsapps.com/start",
		SSOAccountID: "123456789012",
		SSORoleName:  "AdministratorAccess",
	}
}

func TestPostSwitchSSOReportsCachedSession(t *testing.T) {
	origEnsure := ensureSSOLoginFn
	defer func() { ensureSSOLoginFn = origEnsure }()
	ensureSSOLoginFn = func(*config.Config) (awsservice.SSOLoginResult, error) {
		return awsservice.SSOLoginResult{
			StartURL: "https://example.awsapps.com/start",
		}, nil
	}

	msg, err := postSwitchSSO(testPostSwitchSSOCfg())
	if err != nil {
		t.Fatalf("expected cached SSO post-switch to succeed, got %v", err)
	}
	for _, want := range []string{"SSO session active", "cached", "123456789012", "AdministratorAccess"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected %q in message %q", want, msg)
		}
	}
}

func TestPostSwitchSSOReportsRefreshedLogin(t *testing.T) {
	origEnsure := ensureSSOLoginFn
	defer func() { ensureSSOLoginFn = origEnsure }()
	ensureSSOLoginFn = func(*config.Config) (awsservice.SSOLoginResult, error) {
		return awsservice.SSOLoginResult{
			StartURL:  "https://example.awsapps.com/start",
			Refreshed: true,
		}, nil
	}

	msg, err := postSwitchSSO(testPostSwitchSSOCfg())
	if err != nil {
		t.Fatalf("expected refreshed SSO post-switch to succeed, got %v", err)
	}
	for _, want := range []string{"SSO login refreshed", "123456789012", "AdministratorAccess"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected %q in message %q", want, msg)
		}
	}
}
