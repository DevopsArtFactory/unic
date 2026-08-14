package aws

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"unic/internal/config"
	uniclog "unic/internal/log"
)

// AssumeRoleSession holds cached MFA-protected assume-role session credentials.
type AssumeRoleSession struct {
	AccessKeyID     string    `json:"access_key_id"`
	SecretAccessKey string    `json:"secret_access_key"`
	SessionToken    string    `json:"session_token"`
	Expiration      time.Time `json:"expiration"`
}

// Valid reports whether the session is still usable with a small clock skew margin.
func (s AssumeRoleSession) Valid() bool {
	return s.AccessKeyID != "" && time.Now().Add(2*time.Minute).Before(s.Expiration)
}

// assumeRoleCacheDirFn is a test seam for redirecting the cache directory.
var assumeRoleCacheDirFn = defaultAssumeRoleCacheDir

func defaultAssumeRoleCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "unic", "cache", "assume-role"), nil
}

func assumeRoleCachePath(cfg *config.Config) (string, error) {
	dir, err := assumeRoleCacheDirFn()
	if err != nil {
		return "", err
	}
	hash := sha1.Sum([]byte(cfg.Profile + "|" + cfg.RoleArn + "|" + cfg.MFASerial))
	return filepath.Join(dir, hex.EncodeToString(hash[:])+".json"), nil
}

// CachedAssumeRoleSession returns the cached MFA session for the context when
// it exists and has not expired.
func CachedAssumeRoleSession(cfg *config.Config) (AssumeRoleSession, bool) {
	path, err := assumeRoleCachePath(cfg)
	if err != nil {
		return AssumeRoleSession{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return AssumeRoleSession{}, false
	}
	var session AssumeRoleSession
	if err := json.Unmarshal(data, &session); err != nil {
		return AssumeRoleSession{}, false
	}
	if !session.Valid() {
		return AssumeRoleSession{}, false
	}
	return session, true
}

func saveAssumeRoleSession(cfg *config.Config, session AssumeRoleSession) error {
	path, err := assumeRoleCachePath(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create assume-role cache directory: %w", err)
	}
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal assume-role session: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write assume-role session cache: %w", err)
	}
	return nil
}

// AssumeRoleWithMFA assumes the context's role using the given MFA token code
// and caches the resulting session credentials for reuse until expiry.
func AssumeRoleWithMFA(ctx context.Context, cfg *config.Config, tokenCode string) (AssumeRoleSession, error) {
	uniclog.Debug("aws", "assuming role with MFA", "role_arn", cfg.RoleArn, "mfa_serial", cfg.MFASerial)
	baseCfg, err := LoadBaseConfig(ctx, cfg.Region, cfg.Profile)
	if err != nil {
		return AssumeRoleSession{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	input := &sts.AssumeRoleInput{
		RoleArn:         awssdk.String(cfg.RoleArn),
		RoleSessionName: awssdk.String("unic-session"),
		SerialNumber:    awssdk.String(cfg.MFASerial),
		TokenCode:       awssdk.String(tokenCode),
	}
	if cfg.ExternalID != "" {
		input.ExternalId = awssdk.String(cfg.ExternalID)
	}

	result, err := stsAssumeRoleFn(ctx, baseCfg, input)
	if err != nil {
		return AssumeRoleSession{}, fmt.Errorf("failed to assume role %s with MFA: %w", cfg.RoleArn, err)
	}

	creds := result.Credentials
	session := AssumeRoleSession{
		AccessKeyID:     awssdk.ToString(creds.AccessKeyId),
		SecretAccessKey: awssdk.ToString(creds.SecretAccessKey),
		SessionToken:    awssdk.ToString(creds.SessionToken),
	}
	if creds.Expiration != nil {
		session.Expiration = *creds.Expiration
	}
	if err := saveAssumeRoleSession(cfg, session); err != nil {
		uniclog.Info("aws", "failed to cache assume-role session", "error", err.Error())
	}
	return session, nil
}

// stsAssumeRoleFn is a test seam for the STS AssumeRole call.
var stsAssumeRoleFn = func(ctx context.Context, baseCfg awssdk.Config, input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	return sts.NewFromConfig(baseCfg).AssumeRole(ctx, input)
}
