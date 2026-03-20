package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func TestBuildPluginCommand(t *testing.T) {
	sess := &ssm.StartSessionOutput{
		SessionId: aws.String("sess-abc123"),
		StreamUrl: aws.String("wss://example.com"),
		TokenValue: aws.String("token123"),
	}

	cmd, err := BuildPluginCommand(sess, "ap-northeast-2", "myprofile", "i-1234567890abcdef0", "https://ssm.ap-northeast-2.amazonaws.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Path == "" {
		t.Error("command path should not be empty")
	}

	// Should have 6 arguments: sessionJSON, region, "StartSession", profile, paramsJSON, endpoint
	if len(cmd.Args) != 7 { // Args[0] is the command itself
		t.Errorf("expected 7 args (including cmd name), got %d", len(cmd.Args))
	}

	// Verify region argument
	if cmd.Args[2] != "ap-northeast-2" {
		t.Errorf("expected region ap-northeast-2, got %s", cmd.Args[2])
	}

	// Verify StartSession action
	if cmd.Args[3] != "StartSession" {
		t.Errorf("expected StartSession, got %s", cmd.Args[3])
	}

	// Verify profile
	if cmd.Args[4] != "myprofile" {
		t.Errorf("expected myprofile, got %s", cmd.Args[4])
	}
}

func TestBuildPluginCommandContainsInstanceID(t *testing.T) {
	sess := &ssm.StartSessionOutput{
		SessionId: aws.String("sess-abc"),
	}

	cmd, err := BuildPluginCommand(sess, "us-east-1", "default", "i-target", "https://ssm.us-east-1.amazonaws.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The params JSON (Args[5]) should contain the instance ID
	paramsArg := cmd.Args[5]
	if !containsSubstr(paramsArg, "i-target") {
		t.Errorf("params JSON should contain instance ID, got: %s", paramsArg)
	}
}
