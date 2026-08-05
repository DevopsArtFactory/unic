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
	Current   string         `yaml:"current"`
	Defaults  fileDefaults   `yaml:"defaults"`
	Favorites fileFavorites  `yaml:"favorites,omitempty"`
	UI        fileUI         `yaml:"ui,omitempty"`
	Contexts  []contextEntry `yaml:"contexts"`
}

type fileDefaults struct {
	Region string `yaml:"region"`
}

type fileFavorites struct {
	Services []string `yaml:"services,omitempty"`
	Contexts []string `yaml:"contexts,omitempty"`
}

type fileUI struct {
	// BootSplash is tri-state: nil means "unset" (use default behavior),
	// while a non-nil value records an explicit user choice. This lets an
	// explicit `boot_splash: false` survive a save/load round-trip instead of
	// being collapsed into the zero value by `omitempty`.
	BootSplash            *bool  `yaml:"boot_splash,omitempty"`
	LastBootSplashVersion string `yaml:"last_boot_splash_version,omitempty"`
}

// AuthType represents the authentication method for a context.
type AuthType string

const (
	AuthTypeDefault      AuthType = ""
	AuthTypeSSO          AuthType = "sso"
	AuthTypeCredential   AuthType = "credential"
	AuthTypeConsoleLogin AuthType = "console_login"
	AuthTypeAssumeRole   AuthType = "assume_role"
)

// ContextEntry represents a single context definition in config.yaml.
type ContextEntry struct {
	Name         string            `yaml:"name"`
	Order        int               `yaml:"order,omitempty"`
	Profile      string            `yaml:"profile,omitempty"`
	Region       string            `yaml:"region,omitempty"`
	AuthType     string            `yaml:"auth_type,omitempty"`
	RoleArn      string            `yaml:"role_arn,omitempty"`
	ExternalID   string            `yaml:"external_id,omitempty"`
	SSOStartURL  string            `yaml:"sso_start_url,omitempty"`
	SSORegion    string            `yaml:"sso_region,omitempty"`
	SSOAccountID string            `yaml:"sso_account_id,omitempty"`
	SSORoleName  string            `yaml:"sso_role_name,omitempty"`
	Regions      []string          `yaml:"regions,omitempty"`
	Auth         *ContextAuth      `yaml:"auth,omitempty"`
	Resources    *ContextResources `yaml:"resources,omitempty"`
}

// ContextAuth defines the credentials and identity independently from resource location.
type ContextAuth struct {
	Type         string `yaml:"type"`
	Profile      string `yaml:"profile,omitempty"`
	RoleArn      string `yaml:"role_arn,omitempty"`
	ExternalID   string `yaml:"external_id,omitempty"`
	SSOStartURL  string `yaml:"sso_start_url,omitempty"`
	SSORegion    string `yaml:"sso_region,omitempty"`
	SSOAccountID string `yaml:"sso_account_id,omitempty"`
	SSORoleName  string `yaml:"sso_role_name,omitempty"`
}

// ContextResources defines the default and selectable AWS resource regions.
type ContextResources struct {
	DefaultRegion string   `yaml:"default_region"`
	Regions       []string `yaml:"regions,omitempty"`
}

// contextEntry is the alias used internally for fileConfig unmarshalling.
type contextEntry = ContextEntry

type Config struct {
	Profile          string
	Region           string
	ContextName      string
	AuthType         AuthType
	RoleArn          string
	ExternalID       string
	SSOStartURL      string
	SSORegion        string
	SSOAccountID     string
	SSORoleName      string
	Regions          []string
	FavoriteServices []string
	FavoriteContexts []string
	BootSplash       bool
	BootSplashSeen   string
}

// EffectiveSSORegion returns the region used for SSO/portal calls
// (GetRoleCredentials, ListAccounts, aws sso login). It falls back to the
// resource Region when sso_region is not set, preserving behavior for configs
// written before sso_region existed.
func (c *Config) EffectiveSSORegion() string {
	if c.SSORegion != "" {
		return c.SSORegion
	}
	return c.Region
}

func normalizeAuthType(value string) AuthType {
	switch value {
	case "":
		return AuthTypeDefault
	case "sso":
		return AuthTypeSSO
	case "credential", "credentials":
		return AuthTypeCredential
	case "console_login", "console-login":
		return AuthTypeConsoleLogin
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
	SSORegion    string
	SSOAccountID string
	SSORoleName  string
	Regions      []string
	Current      bool
	Favorite     bool
}

type resolvedContextEntry struct {
	Profile, Region, AuthType, RoleArn, ExternalID    string
	SSOStartURL, SSORegion, SSOAccountID, SSORoleName string
	Regions                                           []string
}

func (c ContextEntry) resolved(defaultRegion string) resolvedContextEntry {
	r := resolvedContextEntry{
		Profile: c.Profile, Region: c.Region, AuthType: c.AuthType,
		RoleArn: c.RoleArn, ExternalID: c.ExternalID,
		SSOStartURL: c.SSOStartURL, SSORegion: c.SSORegion,
		SSOAccountID: c.SSOAccountID, SSORoleName: c.SSORoleName,
		Regions: c.Regions,
	}
	if c.Auth != nil {
		r.AuthType = c.Auth.Type
		r.Profile = c.Auth.Profile
		r.RoleArn = c.Auth.RoleArn
		r.ExternalID = c.Auth.ExternalID
		r.SSOStartURL = c.Auth.SSOStartURL
		r.SSORegion = c.Auth.SSORegion
		r.SSOAccountID = c.Auth.SSOAccountID
		r.SSORoleName = c.Auth.SSORoleName
	}
	if c.Resources != nil {
		r.Region = c.Resources.DefaultRegion
		r.Regions = c.Resources.Regions
	}
	if r.Region == "" {
		r.Region = defaultRegion
	}
	r.Regions = normalizeRegions(r.Region, r.Regions)
	return r
}

func normalizeRegions(defaultRegion string, regions []string) []string {
	seen := make(map[string]struct{}, len(regions)+1)
	result := make([]string, 0, len(regions)+1)
	add := func(region string) {
		region = strings.TrimSpace(region)
		if region == "" {
			return
		}
		if _, ok := seen[region]; ok {
			return
		}
		seen[region] = struct{}{}
		result = append(result, region)
	}
	add(defaultRegion)
	for _, region := range regions {
		add(region)
	}
	return result
}

// FilterText returns a lowercase string for keyword matching.
func (c ContextInfo) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s", c.Name, c.Profile, strings.Join(c.Regions, " ")))
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
	var contextName, roleArn, externalID, ssoStartURL, ssoRegion, ssoAccountID, ssoRoleName string
	var regions []string
	var authType AuthType
	if fc.Current != "" {
		for _, ctx := range fc.Contexts {
			if ctx.Name == fc.Current {
				resolved := ctx.resolved(region)
				contextName = ctx.Name
				authType = normalizeAuthType(resolved.AuthType)
				if resolved.Profile != "" {
					profile = resolved.Profile
				}
				region = resolved.Region
				regions = resolved.Regions
				roleArn = resolved.RoleArn
				externalID = resolved.ExternalID
				ssoStartURL = resolved.SSOStartURL
				ssoRegion = resolved.SSORegion
				ssoAccountID = resolved.SSOAccountID
				ssoRoleName = resolved.SSORoleName
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
		regions = normalizeRegions(region, regions)
	}
	if len(regions) == 0 {
		regions = normalizeRegions(region, nil)
	}

	uniclog.Debug("config", "config resolved",
		"path", configPath,
		"profile", profile,
		"region", region,
		"context", contextName,
		"auth_type", string(authType),
	)

	return &Config{
		Profile:          profile,
		Region:           region,
		ContextName:      contextName,
		AuthType:         authType,
		RoleArn:          roleArn,
		ExternalID:       externalID,
		SSOStartURL:      ssoStartURL,
		SSORegion:        ssoRegion,
		SSOAccountID:     ssoAccountID,
		SSORoleName:      ssoRoleName,
		Regions:          regions,
		FavoriteServices: normalizeFavoriteServices(fc.Favorites.Services),
		FavoriteContexts: normalizeFavoriteContexts(fc.Favorites.Contexts),
		BootSplash:       boolValue(fc.UI.BootSplash, false),
		BootSplashSeen:   fc.UI.LastBootSplashVersion,
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

		resolved := ctx.resolved(region)
		profile := resolved.Profile
		region = resolved.Region

		return &Config{
			Profile:          profile,
			Region:           region,
			ContextName:      ctx.Name,
			AuthType:         normalizeAuthType(resolved.AuthType),
			RoleArn:          resolved.RoleArn,
			ExternalID:       resolved.ExternalID,
			SSOStartURL:      resolved.SSOStartURL,
			SSORegion:        resolved.SSORegion,
			SSOAccountID:     resolved.SSOAccountID,
			SSORoleName:      resolved.SSORoleName,
			Regions:          resolved.Regions,
			FavoriteServices: normalizeFavoriteServices(fc.Favorites.Services),
			FavoriteContexts: normalizeFavoriteContexts(fc.Favorites.Contexts),
			BootSplash:       boolValue(fc.UI.BootSplash, false),
			BootSplashSeen:   fc.UI.LastBootSplashVersion,
		}, nil
	}

	return nil, fmt.Errorf("context %q not found in config", name)
}

// boolValue dereferences a tri-state *bool, returning fallback when unset (nil).
func boolValue(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func normalizeFavoriteServices(services []string) []string {
	return normalizeFavoriteNames(services)
}

func normalizeFavoriteContexts(contexts []string) []string {
	return normalizeFavoriteNames(contexts)
}

func normalizeFavoriteNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	normalized := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	sort.Strings(normalized)
	return normalized
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

	favoriteContexts := favoriteContextSet(fc.Favorites.Contexts)
	var infos []ContextInfo
	for _, ctx := range fc.Contexts {
		defaultRegion := fc.Defaults.Region
		if defaultRegion == "" {
			defaultRegion = fc.DefaultRegion
		}
		if defaultRegion == "" {
			defaultRegion = DefaultRegion
		}
		resolved := ctx.resolved(defaultRegion)
		_, favorite := favoriteContexts[ctx.Name]
		infos = append(infos, ContextInfo{
			Name:         ctx.Name,
			Order:        ctx.Order,
			Profile:      resolved.Profile,
			Region:       resolved.Region,
			AuthType:     resolved.AuthType,
			RoleArn:      resolved.RoleArn,
			ExternalID:   resolved.ExternalID,
			SSOStartURL:  resolved.SSOStartURL,
			SSORegion:    resolved.SSORegion,
			SSOAccountID: resolved.SSOAccountID,
			SSORoleName:  resolved.SSORoleName,
			Regions:      resolved.Regions,
			Current:      ctx.Name == fc.Current,
			Favorite:     favorite,
		})
	}
	sort.SliceStable(infos, func(i, j int) bool {
		left, right := infos[i], infos[j]
		switch {
		case left.Order > 0 && right.Order > 0:
			return left.Order < right.Order
		case left.Order > 0:
			return true
		case right.Order > 0:
			return false
		}
		return false
	})
	return infos, nil
}

func favoriteContextSet(names []string) map[string]struct{} {
	favorites := make(map[string]struct{}, len(names))
	for _, name := range normalizeFavoriteContexts(names) {
		favorites[name] = struct{}{}
	}
	return favorites
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

	if err := unsetCurrentFromMapping(mapping); err != nil {
		return err
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

func unsetCurrentFromMapping(mapping *yaml.Node) error {
	if len(mapping.Content)%2 != 0 {
		return fmt.Errorf("malformed config: YAML mapping has odd number of content nodes")
	}

	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "current" {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			break
		}
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

// SetContextOrder updates the display order for a named context.
func SetContextOrder(configPath, name string, order int) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("failed to parse %s: %w", configPath, err)
	}

	found := false
	for i := range fc.Contexts {
		if fc.Contexts[i].Name == name {
			fc.Contexts[i].Order = order
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("context %q not found in config", name)
	}

	out, err := yaml.Marshal(&fc)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// SetContextOrders rewrites all context orders based on the provided name order.
func SetContextOrders(configPath string, names []string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("failed to parse %s: %w", configPath, err)
	}

	if len(names) != len(fc.Contexts) {
		return fmt.Errorf("expected %d context names, got %d", len(fc.Contexts), len(names))
	}

	orderMap := make(map[string]int, len(names))
	for i, name := range names {
		orderMap[name] = i + 1
	}

	for i := range fc.Contexts {
		order, ok := orderMap[fc.Contexts[i].Name]
		if !ok {
			return fmt.Errorf("context %q missing from order list", fc.Contexts[i].Name)
		}
		fc.Contexts[i].Order = order
	}

	out, err := yaml.Marshal(&fc)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// SetFavoriteServices updates the user's preferred service ordering.
func SetFavoriteServices(configPath string, services []string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read config: %w", err)
		}
		data = []byte(defaultContent())
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("failed to parse %s: %w", configPath, err)
	}
	fc.Favorites.Services = normalizeFavoriteServices(services)

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

// SetFavoriteContexts updates the user's preferred context ordering.
func SetFavoriteContexts(configPath string, contexts []string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read config: %w", err)
		}
		data = []byte(defaultContent())
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("failed to parse %s: %w", configPath, err)
	}
	fc.Favorites.Contexts = normalizeFavoriteContexts(contexts)

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

// SetBootSplashEnabled updates whether the startup splash should run on every launch.
func SetBootSplashEnabled(configPath string, enabled bool) error {
	fc, err := readFileConfigOrDefault(configPath)
	if err != nil {
		return err
	}
	fc.UI.BootSplash = &enabled
	return writeFileConfig(configPath, fc)
}

// SetBootSplashSeenVersion records the app version that has already shown the one-time splash.
func SetBootSplashSeenVersion(configPath, version string) error {
	fc, err := readFileConfigOrDefault(configPath)
	if err != nil {
		return err
	}
	fc.UI.LastBootSplashVersion = strings.TrimSpace(version)
	return writeFileConfig(configPath, fc)
}

func readFileConfigOrDefault(configPath string) (*fileConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		data = []byte(defaultContent())
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
	}
	return &fc, nil
}

func writeFileConfig(configPath string, fc *fileConfig) error {
	out, err := yaml.Marshal(fc)
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
