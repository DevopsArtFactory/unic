package aws

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
)

const awsCLIBinary = "aws"

// CheckAWSCLIInstalled verifies that the aws CLI is available in PATH.
func CheckAWSCLIInstalled() error {
	_, err := exec.LookPath(awsCLIBinary)
	if err != nil {
		return fmt.Errorf("aws CLI not found in PATH: install it from https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html")
	}
	return nil
}

// CredentialEnv builds an os.Environ()-based slice with AWS_ACCESS_KEY_ID,
// AWS_SECRET_ACCESS_KEY, and (when present) AWS_SESSION_TOKEN injected.
// AWS_PROFILE / AWS_DEFAULT_PROFILE are stripped so the CLI uses the injected
// credentials rather than the base profile (which may be a different account).
func CredentialEnv(creds awssdk.Credentials) []string {
	env := make([]string, 0, len(os.Environ())+3)
	for _, e := range os.Environ() {
		key, _, _ := strings.Cut(e, "=")
		if key == "AWS_PROFILE" || key == "AWS_DEFAULT_PROFILE" {
			continue
		}
		env = append(env, e)
	}
	env = append(env,
		"AWS_ACCESS_KEY_ID="+creds.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY="+creds.SecretAccessKey,
	)
	if creds.SessionToken != "" {
		env = append(env, "AWS_SESSION_TOKEN="+creds.SessionToken)
	}
	return env
}

// BuildECSExecCommand creates the exec.Cmd for `aws ecs execute-command`
// without running it. Used by the Bubbletea tea.ExecProcess integration.
// credEnv, if non-nil, is set as the command's environment to inject
// assume-role temporary credentials and avoid AccountIDs mismatch errors.
func BuildECSExecCommand(clusterARN, taskARN, containerName, region string, credEnv []string) *exec.Cmd {
	cmd := exec.Command(awsCLIBinary,
		"ecs", "execute-command",
		"--cluster", clusterARN,
		"--task", taskARN,
		"--container", containerName,
		"--interactive",
		"--command", "/bin/sh",
		"--region", region,
	)
	if credEnv != nil {
		cmd.Env = credEnv
	}
	return cmd
}
