package auth

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"unic/internal/clipboard"
	"unic/internal/config"
	uniclog "unic/internal/log"
	awsservice "unic/internal/services/aws"
)

// PostSwitch performs the auth action after switching to a context.
// Returns a human-readable status message.
func PostSwitch(cfg *config.Config) (string, error) {
	uniclog.Info("auth", "post-switch started", "context", cfg.ContextName, "auth_type", string(cfg.AuthType))

	var msg string
	var err error

	switch cfg.AuthType {
	case config.AuthTypeSSO:
		msg, err = postSwitchSSO(cfg)
	case config.AuthTypeCredential:
		msg, err = postSwitchCredential(cfg)
	case config.AuthTypeConsoleLogin:
		msg, err = postSwitchConsoleLogin(cfg)
	case config.AuthTypeAssumeRole:
		msg, err = postSwitchAssumeRole(cfg)
	default:
		msg = fmt.Sprintf("Context %q activated (profile: %s, region: %s)", cfg.ContextName, cfg.Profile, cfg.Region)
	}
	if err != nil {
		uniclog.Error("auth", "post-switch failed", "error", err.Error())
		return "", err
	}

	// Verify identity after switch
	identityMsg := verifyIdentity(cfg)
	if identityMsg != "" {
		msg += "\n" + identityMsg
	}

	return msg, nil
}

func postSwitchSSO(cfg *config.Config) (string, error) {
	uniclog.Debug("auth", "starting SSO login", "sso_start_url", cfg.SSOStartURL)
	if err := awsservice.RunSSOLogin(cfg); err != nil {
		return "", err
	}
	return fmt.Sprintf("SSO login complete for %s (account: %s, role: %s)", cfg.SSOStartURL, cfg.SSOAccountID, cfg.SSORoleName), nil
}

func postSwitchCredential(cfg *config.Config) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}

	credsPath := home + "/.aws/credentials"
	data, err := os.ReadFile(credsPath)
	if err != nil {
		return "", fmt.Errorf("~/.aws/credentials not found: %w", err)
	}

	profileHeader := fmt.Sprintf("[%s]", cfg.Profile)
	if !strings.Contains(string(data), profileHeader) {
		return "", fmt.Errorf("profile %q not found in ~/.aws/credentials", cfg.Profile)
	}

	return fmt.Sprintf("Using credentials profile %q from ~/.aws/credentials (region: %s)", cfg.Profile, cfg.Region), nil
}

func postSwitchConsoleLogin(cfg *config.Config) (string, error) {
	if err := awsservice.ValidateConsoleLoginContext(cfg); err != nil {
		return "", err
	}
	if err := awsservice.RunConsoleLogin(cfg); err != nil {
		return "", err
	}
	return fmt.Sprintf("Console login complete for profile %q (region: %s)", cfg.Profile, cfg.Region), nil
}

func postSwitchAssumeRole(cfg *config.Config) (string, error) {
	uniclog.Debug("auth", "assuming role", "role_arn", cfg.RoleArn)
	ctx := context.Background()

	awsCfg, err := awsservice.LoadBaseConfig(ctx, cfg.Region, cfg.Profile)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(awsCfg)
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(cfg.RoleArn),
		RoleSessionName: aws.String("unic-session"),
	}
	if cfg.ExternalID != "" {
		input.ExternalId = aws.String(cfg.ExternalID)
	}

	result, err := stsClient.AssumeRole(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to assume role %s: %w", cfg.RoleArn, err)
	}

	creds := result.Credentials
	exportStr := fmt.Sprintf(
		"export AWS_ACCESS_KEY_ID=%s\nexport AWS_SECRET_ACCESS_KEY=%s\nexport AWS_SESSION_TOKEN=%s",
		aws.ToString(creds.AccessKeyId),
		aws.ToString(creds.SecretAccessKey),
		aws.ToString(creds.SessionToken),
	)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Assumed role: %s\n", cfg.RoleArn))

	if err := clipboard.Copy(exportStr); err != nil {
		sb.WriteString("\nClipboard unavailable. Copy and paste the following:\n\n")
		sb.WriteString(exportStr)
	} else {
		sb.WriteString("Export commands copied to clipboard.\n")
		sb.WriteString("Paste into your terminal to align 'aws sts get-caller-identity' with unic.")
	}

	return sb.String(), nil
}

// verifyIdentity calls GetCallerIdentity using the context's resolved credentials
// and returns a summary. Non-fatal on error.
func verifyIdentity(cfg *config.Config) string {
	ctx := context.Background()
	repo, err := awsservice.NewAwsRepository(ctx, cfg)
	if err != nil {
		return ""
	}
	identity, err := repo.GetCallerIdentity(ctx)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("  Identity: %s (account: %s)", identity.Arn, identity.Account)
}
