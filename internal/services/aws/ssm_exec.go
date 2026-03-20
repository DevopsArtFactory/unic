package aws

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const sessionManagerPluginBinary = "session-manager-plugin"

// CheckPluginInstalled verifies that session-manager-plugin is available in PATH.
func CheckPluginInstalled() error {
	_, err := exec.LookPath(sessionManagerPluginBinary)
	if err != nil {
		return fmt.Errorf("session-manager-plugin not found in PATH: install it from https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
	}
	return nil
}

// ExecSessionManagerPlugin executes the session-manager-plugin subprocess
// with the given session details.
func ExecSessionManagerPlugin(sess *ssm.StartSessionOutput, region, profile, instanceID, endpoint string) error {
	sessJSON, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("failed to marshal session output: %w", err)
	}

	paramsInput := map[string]string{
		"Target": instanceID,
	}
	paramsJSON, err := json.Marshal(paramsInput)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	return callSubprocess(
		string(sessJSON),
		region,
		"StartSession",
		profile,
		string(paramsJSON),
		endpoint,
	)
}

// callSubprocess launches session-manager-plugin with the given arguments,
// connecting stdin/stdout/stderr and ignoring SIGINT (the plugin handles it).
func callSubprocess(args ...string) error {
	cmd := exec.Command(sessionManagerPluginBinary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	// Ignore SIGINT — the session-manager-plugin handles it internally.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	defer signal.Stop(sigCh)

	return cmd.Run()
}

// BuildPluginCommand creates the exec.Cmd for session-manager-plugin
// without running it. Used by the Bubbletea tea.ExecProcess integration.
func BuildPluginCommand(sess *ssm.StartSessionOutput, region, profile, instanceID, endpoint string) (*exec.Cmd, error) {
	sessJSON, err := json.Marshal(sess)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session output: %w", err)
	}

	paramsInput := map[string]string{
		"Target": instanceID,
	}
	paramsJSON, err := json.Marshal(paramsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	cmd := exec.Command(sessionManagerPluginBinary,
		string(sessJSON),
		region,
		"StartSession",
		profile,
		string(paramsJSON),
		endpoint,
	)

	return cmd, nil
}
