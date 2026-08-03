package aws

import (
	"bufio"
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

// Verify *sso.Client satisfies SSOClientAPI at compile time.
var _ SSOClientAPI = (*sso.Client)(nil)

type SSOClientAPI interface {
	GetRoleCredentials(ctx context.Context, params *sso.GetRoleCredentialsInput, optFns ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error)
	ListAccounts(ctx context.Context, params *sso.ListAccountsInput, optFns ...func(*sso.Options)) (*sso.ListAccountsOutput, error)
	ListAccountRoles(ctx context.Context, params *sso.ListAccountRolesInput, optFns ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error)
}

type SSOAccount struct {
	ID    string
	Name  string
	Email string
}

type SSORole struct {
	Name string
}

var newSSOClient = func(cfg aws.Config) SSOClientAPI {
	return sso.NewFromConfig(cfg)
}

var runSSOLoginFn = RunSSOLogin
var writeSSOConfigFile = os.WriteFile

// ssoTokenCache represents the cached SSO token file structure.
type ssoTokenCache struct {
	AccessToken string `json:"accessToken"`
	ExpiresAt   string `json:"expiresAt"`
	Region      string `json:"region"`
	StartURL    string `json:"startUrl"`
}

// SSOSessionCheck describes whether a cached AWS CLI SSO session can be reused.
type SSOSessionCheck struct {
	StartURL      string
	LoginRequired bool
}

// SSOLoginResult describes whether EnsureSSOLogin reused cache or refreshed login.
type SSOLoginResult struct {
	StartURL  string
	Refreshed bool
}

// resolveSSOCredentials handles SSO authentication and returns an aws.Config with SSO credentials.
func resolveSSOCredentials(ctx context.Context, cfg *config.Config) (aws.Config, error) {
	token, err := ensureSSOToken(cfg)
	if err != nil {
		return aws.Config{}, err
	}

	// The SSO portal (GetRoleCredentials) lives in the SSO region, which may
	// differ from the region where the account's resources live.
	baseCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.EffectiveSSORegion()))
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load base AWS config: %w", err)
	}

	ssoClient := newSSOClient(baseCfg)
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
	// Downstream resource clients must query the resource region, not the SSO region.
	baseCfg.Region = cfg.Region

	return baseCfg, nil
}

// ResolveSSOCredentials exposes SSO credential resolution for CLI env export flows.
func ResolveSSOCredentials(ctx context.Context, cfg *config.Config) (aws.Config, error) {
	return resolveSSOCredentials(ctx, cfg)
}

// CheckSSOSession reports whether the AWS CLI SSO token cache can be reused.
func CheckSSOSession(cfg *config.Config) (SSOSessionCheck, error) {
	_, check, err := checkSSOToken(cfg)
	return check, err
}

// EnsureSSOLogin reuses a valid cached AWS CLI SSO token or runs login if needed.
func EnsureSSOLogin(cfg *config.Config) (SSOLoginResult, error) {
	_, result, err := ensureSSOTokenWithResult(cfg)
	return result, err
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

func checkSSOToken(cfg *config.Config) (*ssoTokenCache, SSOSessionCheck, error) {
	startURL, err := resolveSSOStartURL(cfg)
	if err != nil {
		return nil, SSOSessionCheck{}, err
	}
	check := SSOSessionCheck{StartURL: startURL}
	token, err := loadSSOToken(startURL)
	if err == nil && !isTokenExpired(token) {
		return token, check, nil
	}

	check.LoginRequired = true
	return nil, check, nil
}

func ensureSSOToken(cfg *config.Config) (*ssoTokenCache, error) {
	token, _, err := ensureSSOTokenWithResult(cfg)
	return token, err
}

func ensureSSOTokenWithResult(cfg *config.Config) (*ssoTokenCache, SSOLoginResult, error) {
	token, check, err := checkSSOToken(cfg)
	if err != nil {
		return nil, SSOLoginResult{}, err
	}
	result := SSOLoginResult{StartURL: check.StartURL}
	if !check.LoginRequired {
		return token, result, nil
	}

	if err := runSSOLoginFn(cfg); err != nil {
		return nil, SSOLoginResult{}, fmt.Errorf("SSO login failed: %w", err)
	}
	token, err = loadSSOToken(check.StartURL)
	if err != nil {
		return nil, SSOLoginResult{}, fmt.Errorf("failed to read SSO token after login: %w", err)
	}
	if isTokenExpired(token) {
		return nil, SSOLoginResult{}, fmt.Errorf("SSO token is still expired after login")
	}
	result.Refreshed = true
	return token, result, nil
}

// ListSSOAccounts lists accessible AWS accounts for the configured SSO start URL.
func ListSSOAccounts(ctx context.Context, cfg *config.Config) ([]SSOAccount, error) {
	token, err := ensureSSOToken(cfg)
	if err != nil {
		return nil, err
	}

	baseCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.EffectiveSSORegion()))
	if err != nil {
		return nil, fmt.Errorf("failed to load base AWS config: %w", err)
	}

	client := newSSOClient(baseCfg)
	var accounts []SSOAccount
	var nextToken *string
	for {
		out, err := client.ListAccounts(ctx, &sso.ListAccountsInput{
			AccessToken: aws.String(token.AccessToken),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list SSO accounts: %w", err)
		}

		for _, acct := range out.AccountList {
			accounts = append(accounts, SSOAccount{
				ID:    aws.ToString(acct.AccountId),
				Name:  aws.ToString(acct.AccountName),
				Email: aws.ToString(acct.EmailAddress),
			})
		}

		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}

	return accounts, nil
}

// ListSSOAccountRoles lists accessible roles for a specific SSO account.
func ListSSOAccountRoles(ctx context.Context, cfg *config.Config, accountID string) ([]SSORole, error) {
	token, err := ensureSSOToken(cfg)
	if err != nil {
		return nil, err
	}

	baseCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.EffectiveSSORegion()))
	if err != nil {
		return nil, fmt.Errorf("failed to load base AWS config: %w", err)
	}

	client := newSSOClient(baseCfg)
	var roles []SSORole
	var nextToken *string
	for {
		out, err := client.ListAccountRoles(ctx, &sso.ListAccountRolesInput{
			AccessToken: aws.String(token.AccessToken),
			AccountId:   aws.String(accountID),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list SSO account roles: %w", err)
		}

		for _, role := range out.RoleList {
			roles = append(roles, SSORole{Name: aws.ToString(role.RoleName)})
		}

		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}

	return roles, nil
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
	if cfg.Profile != "" && (cfg.SSOAccountID == "" || cfg.SSORoleName == "") {
		cmd := exec.Command("aws", "sso", "login", "--profile", cfg.Profile)
		return cmd, func() {}, nil
	}
	if cfg.SSOAccountID == "" || cfg.SSORoleName == "" {
		return nil, nil, fmt.Errorf("SSO login requires sso_account_id and sso_role_name when profile is not set")
	}

	tmpDir, err := os.MkdirTemp("", "unic-sso-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	profileName := "unic-sso-login"
	configContent := fmt.Sprintf("[profile %s]\nsso_start_url = %s\nsso_region = %s\nsso_account_id = %s\nsso_role_name = %s\nregion = %s\n",
		profileName,
		cfg.SSOStartURL,
		cfg.EffectiveSSORegion(),
		cfg.SSOAccountID,
		cfg.SSORoleName,
		cfg.Region,
	)

	configPath := filepath.Join(tmpDir, "config")
	if err := writeSSOConfigFile(configPath, []byte(configContent), 0600); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to write temp SSO config: %w", err)
	}

	cmd := exec.Command("aws", "sso", "login", "--profile", profileName)
	cmd.Env = append(os.Environ(), "AWS_CONFIG_FILE="+configPath)
	// On success, callers own the temporary config directory and must defer cleanup().
	return cmd, cleanup, nil
}

func ssoTokenCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".aws", "sso", "cache"), nil
}

func resolveSSOStartURL(cfg *config.Config) (string, error) {
	if cfg.SSOStartURL != "" {
		return cfg.SSOStartURL, nil
	}
	if cfg.Profile == "" {
		return "", fmt.Errorf("SSO context %q is missing sso_start_url and profile", cfg.ContextName)
	}

	profileValues, sessionValues, err := loadSharedConfigSSOSections()
	if err != nil {
		return "", err
	}

	profileName := cfg.Profile
	section := profileValues[profileName]
	if section == nil && profileName == "default" {
		section = profileValues["default"]
	}
	if section == nil {
		return "", fmt.Errorf("profile %q not found in ~/.aws/config", profileName)
	}
	if startURL := section["sso_start_url"]; startURL != "" {
		return startURL, nil
	}

	sessionName := section["sso_session"]
	if sessionName == "" {
		return "", fmt.Errorf("profile %q is missing sso_start_url", profileName)
	}
	if startURL := sessionValues[sessionName]["sso_start_url"]; startURL != "" {
		return startURL, nil
	}
	return "", fmt.Errorf("sso-session %q is missing sso_start_url", sessionName)
}

func loadSharedConfigSSOSections() (map[string]map[string]string, map[string]map[string]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("could not determine home directory: %w", err)
	}
	path := filepath.Join(home, ".aws", "config")
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read ~/.aws/config: %w", err)
	}
	defer file.Close()

	profiles := map[string]map[string]string{}
	sessions := map[string]map[string]string{}
	var current map[string]string
	var sectionKind string
	var sectionName string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			switch {
			case strings.HasPrefix(name, "profile "):
				sectionKind = "profile"
				sectionName = strings.TrimSpace(strings.TrimPrefix(name, "profile "))
				profiles[sectionName] = map[string]string{}
				current = profiles[sectionName]
			case strings.HasPrefix(name, "sso-session "):
				sectionKind = "session"
				sectionName = strings.TrimSpace(strings.TrimPrefix(name, "sso-session "))
				sessions[sectionName] = map[string]string{}
				current = sessions[sectionName]
			case name == "default":
				sectionKind = "profile"
				sectionName = "default"
				profiles[sectionName] = map[string]string{}
				current = profiles[sectionName]
			default:
				sectionKind = ""
				sectionName = ""
				current = nil
			}
			continue
		}

		if current == nil || sectionKind == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		current[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to parse ~/.aws/config: %w", err)
	}

	return profiles, sessions, nil
}

// formatSSOStartURL returns a short display version of the SSO start URL.
func formatSSOStartURL(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimSuffix(url, "/start")
	return url
}
