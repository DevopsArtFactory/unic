package auth

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

var assumeRoleFn = assumeRoleEnv
var resolveSSORoleFn = resolveSSORoleEnv

// BuildEnvExports renders shell export commands for the given context.
func BuildEnvExports(ctx context.Context, cfg *config.Config) (string, error) {
	switch cfg.AuthType {
	case config.AuthTypeSSO:
		if cfg.SSOAccountID == "" || cfg.SSORoleName == "" {
			return "", fmt.Errorf("context %q is an SSO base context; run `unic context setup` first", cfg.ContextName)
		}
		values, err := resolveSSORoleFn(ctx, cfg)
		if err != nil {
			return "", err
		}
		values["AWS_REGION"] = cfg.Region
		values["AWS_DEFAULT_REGION"] = cfg.Region
		values["AWS_PROFILE"] = ""
		return renderExports(values), nil

	case config.AuthTypeAssumeRole:
		values, err := assumeRoleFn(ctx, cfg)
		if err != nil {
			return "", err
		}
		values["AWS_REGION"] = cfg.Region
		values["AWS_DEFAULT_REGION"] = cfg.Region
		values["AWS_PROFILE"] = ""
		return renderExports(values), nil

	case config.AuthTypeCredential, config.AuthTypeDefault:
		profile := cfg.Profile
		if profile == "" {
			profile = "default"
		}
		return renderExports(map[string]string{
			"AWS_PROFILE":           profile,
			"AWS_REGION":            cfg.Region,
			"AWS_DEFAULT_REGION":    cfg.Region,
			"AWS_ACCESS_KEY_ID":     "",
			"AWS_SECRET_ACCESS_KEY": "",
			"AWS_SESSION_TOKEN":     "",
		}), nil

	default:
		return "", fmt.Errorf("unsupported auth type %q", cfg.AuthType)
	}
}

func assumeRoleEnv(ctx context.Context, cfg *config.Config) (map[string]string, error) {
	baseCfg, err := awsservice.LoadBaseConfig(ctx, cfg.Region, cfg.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := sts.NewFromConfig(baseCfg)
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(cfg.RoleArn),
		RoleSessionName: aws.String("unic-env"),
	}
	if cfg.ExternalID != "" {
		input.ExternalId = aws.String(cfg.ExternalID)
	}

	out, err := client.AssumeRole(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role %s: %w", cfg.RoleArn, err)
	}

	creds := out.Credentials
	return map[string]string{
		"AWS_ACCESS_KEY_ID":     aws.ToString(creds.AccessKeyId),
		"AWS_SECRET_ACCESS_KEY": aws.ToString(creds.SecretAccessKey),
		"AWS_SESSION_TOKEN":     aws.ToString(creds.SessionToken),
	}, nil
}

func resolveSSORoleEnv(ctx context.Context, cfg *config.Config) (map[string]string, error) {
	awsCfg, err := awsservice.ResolveSSOCredentials(ctx, cfg)
	if err != nil {
		return nil, err
	}
	creds, err := awsCfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve SSO credentials: %w", err)
	}
	return map[string]string{
		"AWS_ACCESS_KEY_ID":     creds.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY": creds.SecretAccessKey,
		"AWS_SESSION_TOKEN":     creds.SessionToken,
	}, nil
}

func renderExports(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		value := values[key]
		if value == "" {
			lines = append(lines, fmt.Sprintf("unset %s", key))
			continue
		}
		lines = append(lines, fmt.Sprintf("export %s=%s", key, shellQuote(value)))
	}
	return strings.Join(lines, "\n")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
