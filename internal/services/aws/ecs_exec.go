package aws

import (
	"fmt"
	"os/exec"
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

// BuildECSExecCommand creates the exec.Cmd for `aws ecs execute-command`
// without running it. Used by the Bubbletea tea.ExecProcess integration.
func BuildECSExecCommand(clusterARN, taskARN, containerName, region string) *exec.Cmd {
	return exec.Command(awsCLIBinary,
		"ecs", "execute-command",
		"--cluster", clusterARN,
		"--task", taskARN,
		"--container", containerName,
		"--interactive",
		"--command", "/bin/sh",
		"--region", region,
	)
}
