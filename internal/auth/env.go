package auth

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

var assumeRoleFn = assumeRoleEnv
var resolveSSORoleFn = resolveSSORoleEnv

const ContextEnvVar = "UNIC_CONTEXT"

type EnvContextDetection struct {
	Name   string
	Source string
	Known  bool
}

// BuildEnvExports renders shell export commands for the given context.
func BuildEnvExports(ctx context.Context, cfg *config.Config) (string, error) {
	var values map[string]string

	switch cfg.AuthType {
	case config.AuthTypeSSO:
		if cfg.SSOAccountID == "" || cfg.SSORoleName == "" {
			return "", fmt.Errorf("context %q is an SSO base context; run `unic context setup` first", cfg.ContextName)
		}
		var err error
		values, err = resolveSSORoleFn(ctx, cfg)
		if err != nil {
			return "", err
		}
		values["AWS_REGION"] = cfg.Region
		values["AWS_DEFAULT_REGION"] = cfg.Region
		values["AWS_PROFILE"] = ""

	case config.AuthTypeAssumeRole:
		var err error
		values, err = assumeRoleFn(ctx, cfg)
		if err != nil {
			return "", err
		}
		values["AWS_REGION"] = cfg.Region
		values["AWS_DEFAULT_REGION"] = cfg.Region
		values["AWS_PROFILE"] = ""

	case config.AuthTypeCredential, config.AuthTypeConsoleLogin, config.AuthTypeDefault:
		if cfg.AuthType == config.AuthTypeConsoleLogin {
			if err := awsservice.ValidateConsoleLoginContext(cfg); err != nil {
				return "", err
			}
		}
		profile := cfg.Profile
		if profile == "" {
			profile = "default"
		}
		values = map[string]string{
			"AWS_PROFILE":           profile,
			"AWS_REGION":            cfg.Region,
			"AWS_DEFAULT_REGION":    cfg.Region,
			"AWS_ACCESS_KEY_ID":     "",
			"AWS_SECRET_ACCESS_KEY": "",
			"AWS_SESSION_TOKEN":     "",
		}

	default:
		return "", fmt.Errorf("unsupported auth type %q", cfg.AuthType)
	}

	values[ContextEnvVar] = cfg.ContextName
	return renderExports(values), nil
}

func BuildEnvCleanupCommands() string {
	return renderExports(map[string]string{
		"AWS_PROFILE":           "",
		"AWS_REGION":            "",
		"AWS_DEFAULT_REGION":    "",
		"AWS_ACCESS_KEY_ID":     "",
		"AWS_SECRET_ACCESS_KEY": "",
		"AWS_SESSION_TOKEN":     "",
		ContextEnvVar:           "",
	})
}

func DetectEnvContext(contexts []config.ContextInfo, lookup func(string) string) EnvContextDetection {
	if lookup == nil {
		lookup = os.Getenv
	}

	marker := strings.TrimSpace(lookup(ContextEnvVar))
	if marker != "" {
		return EnvContextDetection{
			Name:   marker,
			Source: ContextEnvVar,
			Known:  hasContext(contexts, marker),
		}
	}

	profile := strings.TrimSpace(lookup("AWS_PROFILE"))
	if profile == "" {
		return EnvContextDetection{}
	}

	region := strings.TrimSpace(lookup("AWS_REGION"))
	if region == "" {
		region = strings.TrimSpace(lookup("AWS_DEFAULT_REGION"))
	}

	matches := matchContextsByProfile(contexts, profile, region)
	if len(matches) == 1 {
		return EnvContextDetection{
			Name:   matches[0].Name,
			Source: "AWS_PROFILE",
			Known:  true,
		}
	}

	if region == "" {
		matches = matchContextsByProfile(contexts, profile, "")
		if len(matches) == 1 {
			return EnvContextDetection{
				Name:   matches[0].Name,
				Source: "AWS_PROFILE",
				Known:  true,
			}
		}
	}

	return EnvContextDetection{
		Name:   profile,
		Source: "AWS_PROFILE",
		Known:  false,
	}
}

var (
	promptMFATokenFn    = promptMFAToken
	cachedMFASessionFn  = awsservice.CachedAssumeRoleSession
	assumeRoleWithMFAFn = awsservice.AssumeRoleWithMFA
)

// promptMFAToken asks for an MFA token code on stderr and reads it from stdin,
// so `eval "$(unic env ...)"` keeps stdout clean for the exports.
func promptMFAToken(serial string) (string, error) {
	fmt.Fprintf(os.Stderr, "MFA token code for %s: ", serial)
	var code string
	if _, err := fmt.Fscanln(os.Stdin, &code); err != nil {
		return "", fmt.Errorf("failed to read MFA token code: %w", err)
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("MFA token code is required for %s", serial)
	}
	return code, nil
}

func assumeRoleMFAEnv(ctx context.Context, cfg *config.Config) (map[string]string, error) {
	session, ok := cachedMFASessionFn(cfg)
	if !ok {
		code, err := promptMFATokenFn(cfg.MFASerial)
		if err != nil {
			return nil, err
		}
		session, err = assumeRoleWithMFAFn(ctx, cfg, code)
		if err != nil {
			return nil, err
		}
	}
	return map[string]string{
		"AWS_ACCESS_KEY_ID":     session.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY": session.SecretAccessKey,
		"AWS_SESSION_TOKEN":     session.SessionToken,
	}, nil
}

func assumeRoleEnv(ctx context.Context, cfg *config.Config) (map[string]string, error) {
	if cfg.MFASerial != "" {
		return assumeRoleMFAEnv(ctx, cfg)
	}

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

func hasContext(contexts []config.ContextInfo, name string) bool {
	for _, ctx := range contexts {
		if ctx.Name == name {
			return true
		}
	}
	return false
}

func matchContextsByProfile(contexts []config.ContextInfo, profile, region string) []config.ContextInfo {
	var matches []config.ContextInfo
	for _, ctx := range contexts {
		if ctx.Profile != profile {
			continue
		}
		if region != "" && ctx.Region != region {
			continue
		}
		matches = append(matches, ctx)
	}
	return matches
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
