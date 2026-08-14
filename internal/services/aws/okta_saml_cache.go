package aws

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"unic/internal/config"
)

// OktaSAMLSession holds cached AWS credentials obtained through the Okta SAML
// exchange. The Okta password and session token are never cached.
type OktaSAMLSession struct {
	AccessKeyID     string    `json:"access_key_id"`
	SecretAccessKey string    `json:"secret_access_key"`
	SessionToken    string    `json:"session_token"`
	Expiration      time.Time `json:"expiration"`
}

// Valid reports whether the session is still usable with a small clock skew margin.
func (s OktaSAMLSession) Valid() bool {
	return s.AccessKeyID != "" && time.Now().Add(2*time.Minute).Before(s.Expiration)
}

// oktaSAMLCacheDirFn is a test seam for redirecting the cache directory.
var oktaSAMLCacheDirFn = defaultOktaSAMLCacheDir

func defaultOktaSAMLCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "unic", "cache", "okta-saml"), nil
}

func oktaSAMLCachePath(cfg *config.Config) (string, error) {
	dir, err := oktaSAMLCacheDirFn()
	if err != nil {
		return "", err
	}
	hash := sha1.Sum([]byte(cfg.OktaOrgURL + "|" + cfg.OktaAppID + "|" + cfg.RoleArn))
	return filepath.Join(dir, hex.EncodeToString(hash[:])+".json"), nil
}

// CachedOktaSAMLSession returns the cached session for the context when it
// exists and has not expired.
func CachedOktaSAMLSession(cfg *config.Config) (OktaSAMLSession, bool) {
	path, err := oktaSAMLCachePath(cfg)
	if err != nil {
		return OktaSAMLSession{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return OktaSAMLSession{}, false
	}
	var session OktaSAMLSession
	if err := json.Unmarshal(data, &session); err != nil {
		return OktaSAMLSession{}, false
	}
	if !session.Valid() {
		return OktaSAMLSession{}, false
	}
	return session, true
}

// SaveOktaSAMLSession caches the session with owner-only permissions so the
// TUI can reuse it passively until expiry.
func SaveOktaSAMLSession(cfg *config.Config, session OktaSAMLSession) error {
	path, err := oktaSAMLCachePath(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create okta-saml cache directory: %w", err)
	}
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal okta-saml session: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write okta-saml session cache: %w", err)
	}
	return nil
}
