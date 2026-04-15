package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	uniclog "unic/internal/log"
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
	Current  string         `yaml:"current"`
	Defaults fileDefaults   `yaml:"defaults"`
	Contexts []contextEntry `yaml:"contexts"`
}

type fileDefaults struct {
	Region string `yaml:"region"`
}

// AuthType represents the authentication method for a context.
type AuthType string

const (
	AuthTypeDefault    AuthType = ""
	AuthTypeSSO        AuthType = "sso"
	AuthTypeCredential AuthType = "credential"
	AuthTypeAssumeRole AuthType = "assume_role"
)

// ContextEntry represents a single context definition in config.yaml.
type ContextEntry struct {
	Name         string `yaml:"name"`
	Order        int    `yaml:"order,omitempty"`
	Profile      string `yaml:"profile,omitempty"`
	Region       string `yaml:"region"`
	AuthType     string `yaml:"auth_type"`
	RoleArn      string `yaml:"role_arn,omitempty"`
	ExternalID   string `yaml:"external_id,omitempty"`
	SSOStartURL  string `yaml:"sso_start_url,omitempty"`
	SSOAccountID string `yaml:"sso_account_id,omitempty"`
	SSORoleName  string `yaml:"sso_role_name,omitempty"`
}

// contextEntry is the alias used internally for fileConfig unmarshalling.
type contextEntry = ContextEntry

type Config struct {
	Profile      string
	Region       string
	ContextName  string
	AuthType     AuthType
	RoleArn      string
	ExternalID   string
	SSOStartURL  string
	SSOAccountID string
	SSORoleName  string
}

func normalizeAuthType(value string) AuthType {
	switch value {
	case "":
		return AuthTypeDefault
	case "sso":
		return AuthTypeSSO
	case "credential", "credentials":
		return AuthTypeCredential
	case "assume_role", "assume-role":
		return AuthTypeAssumeRole
	default:
		return AuthType(value)
	}
}

// ContextInfo holds summary information about a context for listing.
type ContextInfo struct {
	Name         string
	Order        int
	Profile      string
	Region       string
	AuthType     string
	RoleArn      string
	ExternalID   string
	SSOStartURL  string
	SSOAccountID string
	SSORoleName  string
	Current      bool
}

// FilterText returns a lowercase string for keyword matching.
func (c ContextInfo) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s", c.Name, c.Profile, c.Region))
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
	var contextName, roleArn, externalID, ssoStartURL, ssoAccountID, ssoRoleName string
	var authType AuthType
	if fc.Current != "" {
		for _, ctx := range fc.Contexts {
			if ctx.Name == fc.Current {
				contextName = ctx.Name
				authType = normalizeAuthType(ctx.AuthType)
				if ctx.Profile != "" {
					profile = ctx.Profile
				}
				if ctx.Region != "" {
					region = ctx.Region
				}
				roleArn = ctx.RoleArn
				externalID = ctx.ExternalID
				ssoStartURL = ctx.SSOStartURL
				ssoAccountID = ctx.SSOAccountID
				ssoRoleName = ctx.SSORoleName
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

	uniclog.Debug("config", "config resolved",
		"path", configPath,
		"profile", profile,
		"region", region,
		"context", contextName,
		"auth_type", string(authType),
	)

	return &Config{
		Profile:      profile,
		Region:       region,
		ContextName:  contextName,
		AuthType:     authType,
		RoleArn:      roleArn,
		ExternalID:   externalID,
		SSOStartURL:  ssoStartURL,
		SSOAccountID: ssoAccountID,
		SSORoleName:  ssoRoleName,
	}, nil
}

// LoadNamedContext resolves a specific named context from the config file.
func LoadNamedContext(configPath, name string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
	}

	region := DefaultRegion
	if fc.DefaultRegion != "" {
		region = fc.DefaultRegion
	}
	if fc.Defaults.Region != "" {
		region = fc.Defaults.Region
	}

	for _, ctx := range fc.Contexts {
		if ctx.Name != name {
			continue
		}

		profile := ctx.Profile
		if ctx.Region != "" {
			region = ctx.Region
		}

		return &Config{
			Profile:      profile,
			Region:       region,
			ContextName:  ctx.Name,
			AuthType:     normalizeAuthType(ctx.AuthType),
			RoleArn:      ctx.RoleArn,
			ExternalID:   ctx.ExternalID,
			SSOStartURL:  ctx.SSOStartURL,
			SSOAccountID: ctx.SSOAccountID,
			SSORoleName:  ctx.SSORoleName,
		}, nil
	}

	return nil, fmt.Errorf("context %q not found in config", name)
}

// Contexts reads the config file and returns all defined contexts.
func Contexts(configPath string) ([]ContextInfo, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
	}

	var infos []ContextInfo
	for _, ctx := range fc.Contexts {
		infos = append(infos, ContextInfo{
			Name:         ctx.Name,
			Order:        ctx.Order,
			Profile:      ctx.Profile,
			Region:       ctx.Region,
			AuthType:     ctx.AuthType,
			RoleArn:      ctx.RoleArn,
			ExternalID:   ctx.ExternalID,
			SSOStartURL:  ctx.SSOStartURL,
			SSOAccountID: ctx.SSOAccountID,
			SSORoleName:  ctx.SSORoleName,
			Current:      ctx.Name == fc.Current,
		})
	}
	sort.SliceStable(infos, func(i, j int) bool {
		left, right := infos[i], infos[j]
		switch {
		case left.Order > 0 && right.Order > 0:
			if left.Order != right.Order {
				return left.Order < right.Order
			}
		case left.Order > 0:
			return true
		case right.Order > 0:
			return false
		}
		return false
	})
	return infos, nil
}

// SetCurrent updates the "current" field in the config file.
func SetCurrent(configPath, name string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Parse to validate the context name exists
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("failed to parse %s: %w", configPath, err)
	}

	found := false
	for _, ctx := range fc.Contexts {
		if ctx.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("context %q not found in config", name)
	}

	// Use yaml.Node to preserve formatting and comments
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("failed to parse config as node: %w", err)
	}

	// doc is a Document node; its first child is the Mapping
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("unexpected config structure")
	}
	mapping := doc.Content[0]

	updated := false
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "current" {
			mapping.Content[i+1].Value = name
			updated = true
			break
		}
	}

	if !updated {
		// Add "current" key at the top of the mapping
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "current"}
		valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: name}
		mapping.Content = append([]*yaml.Node{keyNode, valNode}, mapping.Content...)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// UnsetCurrent clears the "current" field in the config file.
func UnsetCurrent(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("failed to parse config as node: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("unexpected config structure")
	}
	mapping := doc.Content[0]

	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "current" {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			break
		}
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// AddContext appends a new context entry to the config file.
func AddContext(configPath string, entry ContextEntry) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// If file doesn't exist, create with minimal structure
		data = []byte("contexts: []\n")
	}

	// Check for duplicate name
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("failed to parse %s: %w", configPath, err)
	}
	for _, ctx := range fc.Contexts {
		if ctx.Name == entry.Name {
			return fmt.Errorf("context %q already exists", entry.Name)
		}
	}

	// Parse as Node to preserve formatting
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("failed to parse config as node: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("unexpected config structure")
	}
	mapping := doc.Content[0]

	// Find or create the "contexts" sequence
	var ctxSeq *yaml.Node
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == "contexts" {
			ctxSeq = mapping.Content[i+1]
			break
		}
	}
	if ctxSeq == nil {
		// Add "contexts" key
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "contexts"}
		ctxSeq = &yaml.Node{Kind: yaml.SequenceNode}
		mapping.Content = append(mapping.Content, keyNode, ctxSeq)
	}

	// Marshal the new entry to a yaml.Node and append
	entryBytes, err := yaml.Marshal(&entry)
	if err != nil {
		return fmt.Errorf("failed to marshal context entry: %w", err)
	}
	var entryNode yaml.Node
	if err := yaml.Unmarshal(entryBytes, &entryNode); err != nil {
		return fmt.Errorf("failed to parse context entry node: %w", err)
	}
	// entryNode is a Document containing a Mapping
	if entryNode.Kind == yaml.DocumentNode && len(entryNode.Content) > 0 {
		ctxSeq.Content = append(ctxSeq.Content, entryNode.Content[0])
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// UpsertContext adds a new context or replaces an existing one with the same name.
func UpsertContext(configPath string, entry ContextEntry) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		data = []byte("contexts: []\n")
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("failed to parse %s: %w", configPath, err)
	}

	replaced := false
	for i := range fc.Contexts {
		if fc.Contexts[i].Name == entry.Name {
			fc.Contexts[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		fc.Contexts = append(fc.Contexts, entry)
	}

	out, err := yaml.Marshal(&fc)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
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
