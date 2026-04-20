package aws

import (
	"fmt"
	"os"
	"os/exec"

	"unic/internal/config"
)

// RunConsoleLogin executes `aws login` for a profile-backed context.
func RunConsoleLogin(cfg *config.Config) error {
	cmd, err := BuildConsoleLoginCmd(cfg)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Starting console login for profile %s ...\n", cfg.Profile)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aws login failed: %w", err)
	}
	return nil
}

// BuildConsoleLoginCmd creates an *exec.Cmd for `aws login`.
func BuildConsoleLoginCmd(cfg *config.Config) (*exec.Cmd, error) {
	if err := ValidateConsoleLoginContext(cfg); err != nil {
		return nil, err
	}
	args := []string{"login", "--profile", cfg.Profile}
	if cfg.Region != "" {
		args = append(args, "--region", cfg.Region)
	}
	awsPath, err := exec.LookPath("aws")
	if err != nil {
		return nil, fmt.Errorf("aws CLI not found in PATH: %w", err)
	}
	return exec.Command(awsPath, args...), nil
}

func ValidateConsoleLoginContext(cfg *config.Config) error {
	if cfg.Profile == "" {
		return fmt.Errorf("console_login context %q requires profile", cfg.ContextName)
	}
	if cfg.RoleArn != "" {
		return fmt.Errorf("console_login context %q does not support role_arn", cfg.ContextName)
	}
	return nil
}
