package aws

import (
	"path/filepath"
	"testing"

	"unic/internal/config"
)

func TestBuildConsoleLoginCmd(t *testing.T) {
	cmd, err := BuildConsoleLoginCmd(&config.Config{
		ContextName: "local-dev",
		AuthType:    config.AuthTypeConsoleLogin,
		Profile:     "local-dev",
		Region:      "ap-northeast-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := cmd.Args; len(got) != 6 || filepath.Base(got[0]) != "aws" || got[1] != "login" || got[2] != "--profile" || got[3] != "local-dev" || got[4] != "--region" || got[5] != "ap-northeast-2" {
		t.Fatalf("unexpected args: %#v", got)
	}
}

func TestValidateConsoleLoginContextAllowsRoleChaining(t *testing.T) {
	err := ValidateConsoleLoginContext(&config.Config{
		ContextName: "local-dev",
		AuthType:    config.AuthTypeConsoleLogin,
		Profile:     "local-dev",
		Region:      "ap-northeast-2",
		RoleArn:     "arn:aws:iam::123456789012:role/Admin",
	})
	if err != nil {
		t.Fatalf("expected role_arn to be allowed for chaining, got %v", err)
	}
}
