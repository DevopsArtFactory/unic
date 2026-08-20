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
	DefaultRegion              = "us-east-1"
	DefaultACMExpiryWindowDays = 30
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
	Inspector fileInspector  `yaml:"inspector,omitempty"`
	Views     []ViewEntry    `yaml:"views,omitempty"`
	Contexts  []contextEntry `yaml:"contexts"`
}

// ViewEntry is a saved operational view: a jump target (service feature, an
// optional context to switch into, and an optional prefilled filter) that can
// be reapplied in one step. The format is additive: future fields extend it
// without breaking existing files.
type ViewEntry struct {
	Name    string `yaml:"name"`
	Context string `yaml:"context,omitempty"`
	Service string `yaml:"service"`
	Feature string `yaml:"feature"`
	Filter  string `yaml:"filter,omitempty"`
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

type fileInspector struct {
	ACMExpiryWindowDays int `yaml:"acm_expiry_window_days,omitempty"`
}

// AuthType represents the authentication method for a context.
type AuthType string

const (
	AuthTypeDefault      AuthType = ""
	AuthTypeSSO          AuthType = "sso"
	AuthTypeCredential   AuthType = "credential"
	AuthTypeConsoleLogin AuthType = "console_login"
	AuthTypeAssumeRole   AuthType = "assume_role"
	AuthTypeOktaSAML     AuthType = "okta_saml"
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
	MFASerial    string            `yaml:"mfa_serial,omitempty"`
	SSOStartURL  string            `yaml:"sso_start_url,omitempty"`
	SSORegion    string            `yaml:"sso_region,omitempty"`
	SSOAccountID string            `yaml:"sso_account_id,omitempty"`
	SSORoleName  string            `yaml:"sso_role_name,omitempty"`
	OktaOrgURL   string            `yaml:"okta_org_url,omitempty"`
	OktaAppID    string            `yaml:"okta_app_id,omitempty"`
	Regions      []string          `yaml:"regions,omitempty"`
	SyncSource   string            `yaml:"sync_source,omitempty"`
	Auth         *ContextAuth      `yaml:"auth,omitempty"`
	Resources    *ContextResources `yaml:"resources,omitempty"`
}

// ContextAuth defines the credentials and identity independently from resource location.
type ContextAuth struct {
	Type         string `yaml:"type"`
	Profile      string `yaml:"profile,omitempty"`
	RoleArn      string `yaml:"role_arn,omitempty"`
	ExternalID   string `yaml:"external_id,omitempty"`
	MFASerial    string `yaml:"mfa_serial,omitempty"`
	SSOStartURL  string `yaml:"sso_start_url,omitempty"`
	SSORegion    string `yaml:"sso_region,omitempty"`
	SSOAccountID string `yaml:"sso_account_id,omitempty"`
	SSORoleName  string `yaml:"sso_role_name,omitempty"`
	OktaOrgURL   string `yaml:"okta_org_url,omitempty"`
	OktaAppID    string `yaml:"okta_app_id,omitempty"`
}

// ContextResources defines the default and selectable AWS resource regions.
type ContextResources struct {
	DefaultRegion string   `yaml:"default_region"`
	Regions       []string `yaml:"regions,omitempty"`
}

// contextEntry is the alias used internally for fileConfig unmarshalling.
type contextEntry = ContextEntry

type Config struct {
	Profile             string
	Region              string
	ContextName         string
	AuthType            AuthType
	RoleArn             string
	ExternalID          string
	MFASerial           string
	SSOStartURL         string
	SSORegion           string
	SSOAccountID        string
	SSORoleName         string
	OktaOrgURL          string
	OktaAppID           string
	Regions             []string
	FavoriteServices    []string
	FavoriteContexts    []string
	BootSplash          bool
	BootSplashSeen      string
	ACMExpiryWindowDays int
}

func acmExpiryWindowDays(value int) int {
	if value > 0 {
		return value
	}
	return DefaultACMExpiryWindowDays
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
	case "okta_saml", "okta-saml":
		return AuthTypeOktaSAML
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
	MFASerial    string
	SSOStartURL  string
	SSORegion    string
	SSOAccountID string
	SSORoleName  string
	OktaOrgURL   string
	OktaAppID    string
	Regions      []string
	SyncSource   string
	Current      bool
	Favorite     bool
}

type resolvedContextEntry struct {
	Profile, Region, AuthType, RoleArn, ExternalID    string
	MFASerial                                         string
	SSOStartURL, SSORegion, SSOAccountID, SSORoleName string
	OktaOrgURL, OktaAppID                             string
	Regions                                           []string
}

func (c ContextEntry) resolved(defaultRegion string) resolvedContextEntry {
	r := resolvedContextEntry{
		Profile: c.Profile, Region: c.Region, AuthType: c.AuthType,
		RoleArn: c.RoleArn, ExternalID: c.ExternalID,
		MFASerial:   c.MFASerial,
		SSOStartURL: c.SSOStartURL, SSORegion: c.SSORegion,
		SSOAccountID: c.SSOAccountID, SSORoleName: c.SSORoleName,
		OktaOrgURL: c.OktaOrgURL, OktaAppID: c.OktaAppID,
		Regions: c.Regions,
	}
	if c.Auth != nil {
		r.AuthType = c.Auth.Type
		r.Profile = c.Auth.Profile
		r.RoleArn = c.Auth.RoleArn
		r.ExternalID = c.Auth.ExternalID
		r.MFASerial = c.Auth.MFASerial
		r.SSOStartURL = c.Auth.SSOStartURL
		r.SSORegion = c.Auth.SSORegion
		r.SSOAccountID = c.Auth.SSOAccountID
		r.SSORoleName = c.Auth.SSORoleName
		r.OktaOrgURL = c.Auth.OktaOrgURL
		r.OktaAppID = c.Auth.OktaAppID
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
	var contextName, roleArn, externalID, mfaSerial, ssoStartURL, ssoRegion, ssoAccountID, ssoRoleName, oktaOrgURL, oktaAppID string
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
				mfaSerial = resolved.MFASerial
				ssoStartURL = resolved.SSOStartURL
				ssoRegion = resolved.SSORegion
				ssoAccountID = resolved.SSOAccountID
				ssoRoleName = resolved.SSORoleName
				oktaOrgURL = resolved.OktaOrgURL
				oktaAppID = resolved.OktaAppID
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
		Profile:             profile,
		Region:              region,
		ContextName:         contextName,
		AuthType:            authType,
		RoleArn:             roleArn,
		ExternalID:          externalID,
		MFASerial:           mfaSerial,
		SSOStartURL:         ssoStartURL,
		SSORegion:           ssoRegion,
		SSOAccountID:        ssoAccountID,
		SSORoleName:         ssoRoleName,
		OktaOrgURL:          oktaOrgURL,
		OktaAppID:           oktaAppID,
		Regions:             regions,
		FavoriteServices:    normalizeFavoriteServices(fc.Favorites.Services),
		FavoriteContexts:    normalizeFavoriteContexts(fc.Favorites.Contexts),
		BootSplash:          boolValue(fc.UI.BootSplash, false),
		BootSplashSeen:      fc.UI.LastBootSplashVersion,
		ACMExpiryWindowDays: acmExpiryWindowDays(fc.Inspector.ACMExpiryWindowDays),
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
			Profile:             profile,
			Region:              region,
			ContextName:         ctx.Name,
			AuthType:            normalizeAuthType(resolved.AuthType),
			RoleArn:             resolved.RoleArn,
			ExternalID:          resolved.ExternalID,
			MFASerial:           resolved.MFASerial,
			SSOStartURL:         resolved.SSOStartURL,
			SSORegion:           resolved.SSORegion,
			SSOAccountID:        resolved.SSOAccountID,
			SSORoleName:         resolved.SSORoleName,
			OktaOrgURL:          resolved.OktaOrgURL,
			OktaAppID:           resolved.OktaAppID,
			Regions:             resolved.Regions,
			FavoriteServices:    normalizeFavoriteServices(fc.Favorites.Services),
			FavoriteContexts:    normalizeFavoriteContexts(fc.Favorites.Contexts),
			BootSplash:          boolValue(fc.UI.BootSplash, false),
			BootSplashSeen:      fc.UI.LastBootSplashVersion,
			ACMExpiryWindowDays: acmExpiryWindowDays(fc.Inspector.ACMExpiryWindowDays),
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
			MFASerial:    resolved.MFASerial,
			SSOStartURL:  resolved.SSOStartURL,
			SSORegion:    resolved.SSORegion,
			SSOAccountID: resolved.SSOAccountID,
			SSORoleName:  resolved.SSORoleName,
			OktaOrgURL:   resolved.OktaOrgURL,
			OktaAppID:    resolved.OktaAppID,
			Regions:      resolved.Regions,
			SyncSource:   ctx.SyncSource,
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

// The config store routes every mutation through one of two paths and a
// single atomic writer:
//
//   - mutateFileConfig: typed path. The whole file is parsed into fileConfig,
//     mutated, and rewritten. Comments and key order are NOT preserved.
//   - mutateConfigNode: formatting-preserving path. The file is parsed into a
//     yaml.Node tree so comments and layout survive targeted edits.
//
// Which operation uses which path is a deliberate, per-operation choice.

// missingPolicy controls how a mutation treats an absent config file.
type missingPolicy int

const (
	failIfMissing missingPolicy = iota
	emptyContextsIfMissing
	defaultsIfMissing
)

func readConfigOrSeed(configPath string, onMissing missingPolicy) ([]byte, error) {
	data, err := os.ReadFile(configPath)
	if err == nil {
		return data, nil
	}
	switch onMissing {
	case emptyContextsIfMissing:
		return []byte("contexts: []\n"), nil
	case defaultsIfMissing:
		if os.IsNotExist(err) {
			return []byte(defaultContent()), nil
		}
	}
	return nil, fmt.Errorf("failed to read config: %w", err)
}

// writeConfigBytes persists the config atomically: the content is written to a
// temp file in the same directory and renamed into place, so a crash mid-write
// can never leave a truncated config.yaml behind. os.Rename replaces the
// destination as a single operation on POSIX and on Windows (MoveFileEx with
// MOVEFILE_REPLACE_EXISTING), so the previous config is never unlinked first.
// An existing file keeps its permission bits (a user's 0600 stays 0600); only
// newly created files get the 0644 default.
func writeConfigBytes(configPath string, out []byte) error {
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	mode := os.FileMode(0644)
	if info, err := os.Stat(configPath); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, ".config-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp config file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("failed to write temp config file: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("failed to chmod temp config file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("failed to close temp config file: %w", err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		cleanup()
		return fmt.Errorf("failed to replace config file: %w", err)
	}
	return nil
}

func mutateFileConfig(configPath string, onMissing missingPolicy, mutate func(*fileConfig) error) error {
	data, err := readConfigOrSeed(configPath, onMissing)
	if err != nil {
		return err
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("failed to parse %s: %w", configPath, err)
	}
	if err := mutate(&fc); err != nil {
		return err
	}
	out, err := yaml.Marshal(&fc)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return writeConfigBytes(configPath, out)
}

// mutateConfigNode passes the raw file content alongside the document mapping
// so callers can run typed validation before editing nodes.
func mutateConfigNode(configPath string, onMissing missingPolicy, mutate func(data []byte, mapping *yaml.Node) error) error {
	data, err := readConfigOrSeed(configPath, onMissing)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("failed to parse config as node: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("unexpected config structure")
	}
	if err := mutate(data, doc.Content[0]); err != nil {
		return err
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return writeConfigBytes(configPath, out)
}

// SetCurrent updates the "current" field in the config file.
func SetCurrent(configPath, name string) error {
	return mutateConfigNode(configPath, failIfMissing, func(data []byte, mapping *yaml.Node) error {
		// Validate the context name exists before editing nodes.
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

		for i := 0; i < len(mapping.Content)-1; i += 2 {
			if mapping.Content[i].Value == "current" {
				mapping.Content[i+1].Value = name
				return nil
			}
		}
		// Add "current" key at the top of the mapping
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "current"}
		valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: name}
		mapping.Content = append([]*yaml.Node{keyNode, valNode}, mapping.Content...)
		return nil
	})
}

// UnsetCurrent clears the "current" field in the config file.
func UnsetCurrent(configPath string) error {
	return mutateConfigNode(configPath, failIfMissing, func(_ []byte, mapping *yaml.Node) error {
		return unsetCurrentFromMapping(mapping)
	})
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
	return mutateConfigNode(configPath, emptyContextsIfMissing, func(data []byte, mapping *yaml.Node) error {
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

		// Find or create the "contexts" sequence
		var ctxSeq *yaml.Node
		for i := 0; i < len(mapping.Content)-1; i += 2 {
			if mapping.Content[i].Value == "contexts" {
				ctxSeq = mapping.Content[i+1]
				break
			}
		}
		if ctxSeq == nil {
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
		return nil
	})
}

// UpsertContext adds a new context or replaces an existing one with the same name.
func UpsertContext(configPath string, entry ContextEntry) error {
	return UpsertAndRemoveContexts(configPath, []ContextEntry{entry}, nil)
}

// RemoveContexts deletes the named contexts from config. The current context
// selection is cleared when it points at a removed context.
func RemoveContexts(configPath string, names []string) error {
	return UpsertAndRemoveContexts(configPath, nil, names)
}

// UpsertAndRemoveContexts applies additions/updates and removals in one
// read-modify-write pass so a sync plan is persisted atomically instead of one
// entry at a time. The current context selection is cleared when it points at
// a removed context.
func UpsertAndRemoveContexts(configPath string, upserts []ContextEntry, removals []string) error {
	if len(upserts) == 0 && len(removals) == 0 {
		return nil
	}
	onMissing := failIfMissing
	if len(upserts) > 0 {
		onMissing = emptyContextsIfMissing
	}
	return mutateFileConfig(configPath, onMissing, func(fc *fileConfig) error {
		for _, entry := range upserts {
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
		}

		remove := make(map[string]struct{}, len(removals))
		for _, name := range removals {
			remove[name] = struct{}{}
		}
		kept := fc.Contexts[:0]
		for _, ctx := range fc.Contexts {
			if _, ok := remove[ctx.Name]; ok {
				if fc.Current == ctx.Name {
					fc.Current = ""
				}
				continue
			}
			kept = append(kept, ctx)
		}
		fc.Contexts = kept
		return nil
	})
}

// SetContextOrder updates the display order for a named context.
func SetContextOrder(configPath, name string, order int) error {
	return mutateFileConfig(configPath, failIfMissing, func(fc *fileConfig) error {
		for i := range fc.Contexts {
			if fc.Contexts[i].Name == name {
				fc.Contexts[i].Order = order
				return nil
			}
		}
		return fmt.Errorf("context %q not found in config", name)
	})
}

// SetContextOrders rewrites all context orders based on the provided name order.
func SetContextOrders(configPath string, names []string) error {
	return mutateFileConfig(configPath, failIfMissing, func(fc *fileConfig) error {
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
		return nil
	})
}

// SetFavoriteServices updates the user's preferred service ordering.
func SetFavoriteServices(configPath string, services []string) error {
	return mutateFileConfig(configPath, defaultsIfMissing, func(fc *fileConfig) error {
		fc.Favorites.Services = normalizeFavoriteServices(services)
		return nil
	})
}

// SetFavoriteContexts updates the user's preferred context ordering.
func SetFavoriteContexts(configPath string, contexts []string) error {
	return mutateFileConfig(configPath, defaultsIfMissing, func(fc *fileConfig) error {
		fc.Favorites.Contexts = normalizeFavoriteContexts(contexts)
		return nil
	})
}

// SetBootSplashEnabled updates whether the startup splash should run on every launch.
func SetBootSplashEnabled(configPath string, enabled bool) error {
	return mutateFileConfig(configPath, defaultsIfMissing, func(fc *fileConfig) error {
		fc.UI.BootSplash = &enabled
		return nil
	})
}

// SetBootSplashSeenVersion records the app version that has already shown the one-time splash.
func SetBootSplashSeenVersion(configPath, version string) error {
	return mutateFileConfig(configPath, defaultsIfMissing, func(fc *fileConfig) error {
		fc.UI.LastBootSplashVersion = strings.TrimSpace(version)
		return nil
	})
}

// Views returns the saved views from config, in file order.
func Views(configPath string) ([]ViewEntry, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
	}
	return fc.Views, nil
}

// SaveView adds a view or replaces an existing one with the same name.
func SaveView(configPath string, view ViewEntry) error {
	if strings.TrimSpace(view.Name) == "" {
		return fmt.Errorf("view name is required")
	}
	return mutateFileConfig(configPath, defaultsIfMissing, func(fc *fileConfig) error {
		for i := range fc.Views {
			if fc.Views[i].Name == view.Name {
				fc.Views[i] = view
				return nil
			}
		}
		fc.Views = append(fc.Views, view)
		return nil
	})
}

// DeleteView removes the named view; deleting a missing view is not an error.
func DeleteView(configPath, name string) error {
	return mutateFileConfig(configPath, failIfMissing, func(fc *fileConfig) error {
		kept := fc.Views[:0]
		for _, view := range fc.Views {
			if view.Name != name {
				kept = append(kept, view)
			}
		}
		fc.Views = kept
		return nil
	})
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
	return writeConfigBytes(configPath, []byte(defaultContent()))
}

// CreateConfig creates a config file. Returns true if a new file was created.
// If force is true, overwrites an existing file.
func CreateConfig(configPath string, force bool) (bool, error) {
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return false, nil
		}
	}
	if err := writeConfigBytes(configPath, []byte(defaultContent())); err != nil {
		return false, err
	}
	return true, nil
}
