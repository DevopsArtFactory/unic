package aws

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sso"

	"unic/internal/config"
)

// ssoTokenCache represents the cached SSO token file structure.
type ssoTokenCache struct {
	AccessToken string `json:"accessToken"`
	ExpiresAt   string `json:"expiresAt"`
	Region      string `json:"region"`
	StartURL    string `json:"startUrl"`
}

// resolveSSOCredentials handles SSO authentication and returns an aws.Config with SSO credentials.
func resolveSSOCredentials(ctx context.Context, cfg *config.Config) (aws.Config, error) {
	token, err := loadSSOToken(cfg.SSOStartURL)
	if err != nil || isTokenExpired(token) {
		// Token missing or expired — run aws sso login
		if err := RunSSOLogin(cfg); err != nil {
			return aws.Config{}, fmt.Errorf("SSO login failed: %w", err)
		}
		token, err = loadSSOToken(cfg.SSOStartURL)
		if err != nil {
			return aws.Config{}, fmt.Errorf("failed to read SSO token after login: %w", err)
		}
	}

	// Use the SSO token to get role credentials
	baseCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load base AWS config: %w", err)
	}

	ssoClient := sso.NewFromConfig(baseCfg)
	roleOutput, err := ssoClient.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
		AccessToken: aws.String(token.AccessToken),
		AccountId:   aws.String(cfg.SSOAccountID),
		RoleName:    aws.String(cfg.SSORoleName),
	})
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to get SSO role credentials: %w", err)
	}

	creds := roleOutput.RoleCredentials
	baseCfg.Credentials = credentials.NewStaticCredentialsProvider(
		aws.ToString(creds.AccessKeyId),
		aws.ToString(creds.SecretAccessKey),
		aws.ToString(creds.SessionToken),
	)

	return baseCfg, nil
}

// loadSSOToken reads the cached SSO token for the given start URL.
func loadSSOToken(startURL string) (*ssoTokenCache, error) {
	cacheDir, err := ssoTokenCacheDir()
	if err != nil {
		return nil, err
	}

	// AWS CLI caches tokens using SHA1 hash of the start URL
	hash := sha1.Sum([]byte(startURL))
	filename := hex.EncodeToString(hash[:]) + ".json"
	path := filepath.Join(cacheDir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("SSO token cache not found: %w", err)
	}

	var token ssoTokenCache
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse SSO token cache: %w", err)
	}

	return &token, nil
}

func isTokenExpired(token *ssoTokenCache) bool {
	if token == nil {
		return true
	}
	// AWS CLI stores expiry in RFC3339-like format (UTC)
	expiry, err := time.Parse(time.RFC3339, token.ExpiresAt)
	if err != nil {
		// Try alternative format used by some AWS CLI versions
		expiry, err = time.Parse("2006-01-02T15:04:05Z", token.ExpiresAt)
		if err != nil {
			return true
		}
	}
	return time.Now().After(expiry)
}

// RunSSOLogin executes `aws sso login` with a temporary SSO profile config.
func RunSSOLogin(cfg *config.Config) error {
	cmd, cleanup, err := BuildSSOLoginCmd(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Starting SSO login for %s ...\n", cfg.SSOStartURL)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aws sso login failed: %w", err)
	}
	return nil
}

// BuildSSOLoginCmd creates an *exec.Cmd for `aws sso login` and returns a cleanup function
// for the temporary config directory. Caller must defer cleanup().
func BuildSSOLoginCmd(cfg *config.Config) (*exec.Cmd, func(), error) {
	tmpDir, err := os.MkdirTemp("", "unic-sso-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	profileName := "unic-sso-login"
	configContent := fmt.Sprintf("[profile %s]\nsso_start_url = %s\nsso_region = %s\nsso_account_id = %s\nsso_role_name = %s\nregion = %s\n",
		profileName,
		cfg.SSOStartURL,
		cfg.Region,
		cfg.SSOAccountID,
		cfg.SSORoleName,
		cfg.Region,
	)

	configPath := filepath.Join(tmpDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to write temp SSO config: %w", err)
	}

	cmd := exec.Command("aws", "sso", "login", "--profile", profileName)
	cmd.Env = append(os.Environ(), "AWS_CONFIG_FILE="+configPath)
	return cmd, cleanup, nil
}

func ssoTokenCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".aws", "sso", "cache"), nil
}

// formatSSOStartURL returns a short display version of the SSO start URL.
func formatSSOStartURL(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimSuffix(url, "/start")
	return url
}
