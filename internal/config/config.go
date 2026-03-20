package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultRegion = "us-east-1"
)

// fileConfig supports both the legacy flat format and the new contexts-based format.
type fileConfig struct {
	// Legacy flat format
	DefaultProfile string `yaml:"default_profile"`
	DefaultRegion  string `yaml:"default_region"`

	// New contexts-based format
	Current  string          `yaml:"current"`
	Defaults fileDefaults    `yaml:"defaults"`
	Contexts []contextEntry  `yaml:"contexts"`
}

type fileDefaults struct {
	Region string `yaml:"region"`
}

type contextEntry struct {
	Name    string `yaml:"name"`
	Profile string `yaml:"profile"`
	Region  string `yaml:"region"`
}

type Config struct {
	Profile string
	Region  string
}

// Load resolves config with priority: CLI flags > context > config file defaults > hardcoded defaults.
func Load(cliProfile, cliRegion *string, configPath string) (*Config, error) {
	var fc fileConfig

	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
		}
	}

	var profile string
	region := DefaultRegion

	// Legacy flat format
	if fc.DefaultProfile != "" && fc.DefaultProfile != "default" {
		profile = fc.DefaultProfile
	}
	if fc.DefaultRegion != "" {
		region = fc.DefaultRegion
	}

	// New format: defaults section
	if fc.Defaults.Region != "" {
		region = fc.Defaults.Region
	}

	// New format: resolve current context
	if fc.Current != "" {
		for _, ctx := range fc.Contexts {
			if ctx.Name == fc.Current {
				if ctx.Profile != "" {
					profile = ctx.Profile
				}
				if ctx.Region != "" {
					region = ctx.Region
				}
				break
			}
		}
	}

	// CLI flags have the highest priority
	if cliProfile != nil {
		profile = *cliProfile
	}
	if cliRegion != nil {
		region = *cliRegion
	}

	return &Config{Profile: profile, Region: region}, nil
}

// DefaultPath returns the default config file path following XDG Base Directory spec.
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not determine home directory: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "unic", "config.yaml"), nil
}

// defaultContent returns the initial config file content.
func defaultContent() string {
	return fmt.Sprintf("default_region: %q\n", DefaultRegion)
}

// EnsureConfigExists creates a default config file if it doesn't exist.
func EnsureConfigExists(configPath string) error {
	if _, err := os.Stat(configPath); err == nil {
		return nil
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(defaultContent()), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// CreateConfig creates a config file. Returns true if a new file was created.
// If force is true, overwrites an existing file.
func CreateConfig(configPath string, force bool) (bool, error) {
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return false, nil
		}
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(defaultContent()), 0644); err != nil {
		return false, fmt.Errorf("failed to write config file: %w", err)
	}

	return true, nil
}
