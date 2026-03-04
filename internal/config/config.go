package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultProfile = "default"
	DefaultRegion  = "us-east-1"
)

type fileConfig struct {
	DefaultProfile string `yaml:"default_profile"`
	DefaultRegion  string `yaml:"default_region"`
}

type Config struct {
	Profile string
	Region  string
}

// Load resolves config with priority: CLI flags > config file > hardcoded defaults.
func Load(cliProfile, cliRegion *string, configPath string) (*Config, error) {
	var fc fileConfig

	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
		}
	}

	profile := DefaultProfile
	if fc.DefaultProfile != "" {
		profile = fc.DefaultProfile
	}
	if cliProfile != nil {
		profile = *cliProfile
	}

	region := DefaultRegion
	if fc.DefaultRegion != "" {
		region = fc.DefaultRegion
	}
	if cliRegion != nil {
		region = *cliRegion
	}

	return &Config{Profile: profile, Region: region}, nil
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

	content := fmt.Sprintf("default_profile: %q\ndefault_region: %q\n", DefaultProfile, DefaultRegion)
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
