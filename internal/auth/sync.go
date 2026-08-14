package auth

import (
	"context"
	"fmt"
	"sort"

	"unic/internal/config"
)

// ContextSyncPlan describes what a sync run would change for one SSO base context.
type ContextSyncPlan struct {
	Base string
	// Add holds new sync-managed contexts for SSO account/role pairs that
	// have no configured context yet.
	Add []config.ContextEntry
	// Keep lists configured contexts (manual or synced) that still match an
	// SSO-visible account/role pair. Sync never rewrites them.
	Keep []string
	// Orphans lists sync-managed contexts for this base whose account/role
	// pair is no longer visible in SSO.
	Orphans []string
}

// BuildContextSyncPlan compares the accounts and roles visible to an SSO base
// context with the configured contexts and returns the additions, unchanged
// matches, and sync-managed orphans. It never plans changes to manually
// managed contexts.
func BuildContextSyncPlan(ctx context.Context, configPath string, base config.ContextInfo) (ContextSyncPlan, error) {
	plan := ContextSyncPlan{Base: base.Name}

	accounts, err := ListSSOContextAccounts(ctx, configPath, base)
	if err != nil {
		return plan, err
	}

	existing, err := config.Contexts(configPath)
	if err != nil {
		return plan, err
	}

	type roleKey struct{ account, role string }
	configured := make(map[roleKey]config.ContextInfo)
	for _, info := range existing {
		if config.AuthType(info.AuthType) != config.AuthTypeSSO ||
			info.SSOStartURL != base.SSOStartURL ||
			info.SSOAccountID == "" || info.SSORoleName == "" {
			continue
		}
		configured[roleKey{info.SSOAccountID, info.SSORoleName}] = info
	}

	region := base.Region
	if region == "" {
		region = config.DefaultRegion
	}
	resourceRegions := contextRegions(region, base.Regions)

	desired := make(map[roleKey]bool)
	names := append([]config.ContextInfo(nil), existing...)
	for _, account := range accounts {
		roles, err := ListSSOContextRoles(ctx, configPath, base, account.ID)
		if err != nil {
			return plan, err
		}
		for _, role := range roles {
			key := roleKey{account.ID, role.Name}
			desired[key] = true
			if match, ok := configured[key]; ok {
				plan.Keep = append(plan.Keep, match.Name)
				continue
			}
			name := uniqueContextName(names, fmt.Sprintf("%s-%s-%s", base.Name, account.ID, sanitizeName(role.Name)))
			names = append(names, config.ContextInfo{Name: name})
			plan.Add = append(plan.Add, config.ContextEntry{
				Name:       name,
				SyncSource: base.Name,
				Auth: &config.ContextAuth{
					Type:         string(config.AuthTypeSSO),
					Profile:      base.Profile,
					SSOStartURL:  base.SSOStartURL,
					SSORegion:    base.SSORegion,
					SSOAccountID: account.ID,
					SSORoleName:  role.Name,
				},
				Resources: &config.ContextResources{
					DefaultRegion: region,
					Regions:       resourceRegions[1:],
				},
			})
		}
	}

	for _, info := range existing {
		if info.SyncSource != base.Name {
			continue
		}
		if !desired[roleKey{info.SSOAccountID, info.SSORoleName}] {
			plan.Orphans = append(plan.Orphans, info.Name)
		}
	}
	sort.Strings(plan.Keep)
	sort.Strings(plan.Orphans)
	return plan, nil
}

// ApplyContextSyncPlan persists a sync plan in a single config write:
// additions are upserted as sync-managed contexts, and orphans are removed
// only when prune is set. Applying everything in one mutation avoids leaving
// the config half-synced when a write fails partway.
func ApplyContextSyncPlan(configPath string, plan ContextSyncPlan, prune bool) error {
	var removals []string
	if prune {
		removals = plan.Orphans
	}
	return config.UpsertAndRemoveContexts(configPath, plan.Add, removals)
}
