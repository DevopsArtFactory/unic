package auth

import (
	"context"
	"errors"
	"testing"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

const syncBaseConfig = `current: dev-sso
contexts:
  - name: dev-sso
    profile: sso-profile
    region: ap-northeast-2
    auth_type: sso
    sso_start_url: https://example.awsapps.com/start
  - name: manual-admin
    profile: sso-profile
    region: ap-northeast-2
    auth_type: sso
    sso_start_url: https://example.awsapps.com/start
    sso_account_id: "111111111111"
    sso_role_name: Admin
  - name: dev-sso-222222222222-stale
    auth_type: sso
    sso_start_url: https://example.awsapps.com/start
    sso_account_id: "222222222222"
    sso_role_name: Stale
    region: ap-northeast-2
    sync_source: dev-sso
`

func stubSSOListing(t *testing.T, accounts []awsservice.SSOAccount, roles map[string][]awsservice.SSORole) {
	t.Helper()
	origAccounts := listSSOAccountsFn
	origRoles := listSSOAccountRolesFn
	t.Cleanup(func() {
		listSSOAccountsFn = origAccounts
		listSSOAccountRolesFn = origRoles
	})
	listSSOAccountsFn = func(_ context.Context, _ *config.Config) ([]awsservice.SSOAccount, error) {
		return accounts, nil
	}
	listSSOAccountRolesFn = func(_ context.Context, _ *config.Config, accountID string) ([]awsservice.SSORole, error) {
		out, ok := roles[accountID]
		if !ok {
			return nil, errors.New("unexpected account " + accountID)
		}
		return out, nil
	}
}

func syncBase(t *testing.T, configPath string) config.ContextInfo {
	t.Helper()
	contexts, err := config.Contexts(configPath)
	if err != nil {
		t.Fatalf("failed to list contexts: %v", err)
	}
	for _, ctx := range contexts {
		if ctx.Name == "dev-sso" {
			return ctx
		}
	}
	t.Fatal("dev-sso base context not found")
	return config.ContextInfo{}
}

func TestBuildContextSyncPlan(t *testing.T) {
	configPath := writeConfig(t, t.TempDir(), syncBaseConfig)
	stubSSOListing(t,
		[]awsservice.SSOAccount{
			{ID: "111111111111", Name: "prod"},
			{ID: "333333333333", Name: "dev"},
		},
		map[string][]awsservice.SSORole{
			"111111111111": {{Name: "Admin"}},
			"333333333333": {{Name: "DeveloperRole"}},
		},
	)

	plan, err := BuildContextSyncPlan(context.Background(), configPath, syncBase(t, configPath))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plan.Add) != 1 || plan.Add[0].Name != "dev-sso-333333333333-developerrole" {
		t.Fatalf("expected one added context for the new account/role, got %+v", plan.Add)
	}
	added := plan.Add[0]
	if added.SyncSource != "dev-sso" {
		t.Fatalf("expected added context to be marked sync-managed, got %+v", added)
	}
	if added.Auth == nil || added.Auth.SSOAccountID != "333333333333" || added.Auth.SSORoleName != "DeveloperRole" {
		t.Fatalf("expected added context auth fields, got %+v", added.Auth)
	}
	if added.Resources == nil || added.Resources.DefaultRegion != "ap-northeast-2" {
		t.Fatalf("expected added context to inherit the base region, got %+v", added.Resources)
	}

	if len(plan.Keep) != 1 || plan.Keep[0] != "manual-admin" {
		t.Fatalf("expected the matching manual context to be kept, got %+v", plan.Keep)
	}
	if len(plan.Orphans) != 1 || plan.Orphans[0] != "dev-sso-222222222222-stale" {
		t.Fatalf("expected the stale sync-managed context to be orphaned, got %+v", plan.Orphans)
	}
}

func TestApplyContextSyncPlanWithoutPruneKeepsOrphans(t *testing.T) {
	configPath := writeConfig(t, t.TempDir(), syncBaseConfig)
	plan := ContextSyncPlan{
		Base: "dev-sso",
		Add: []config.ContextEntry{{
			Name:       "dev-sso-333333333333-developerrole",
			SyncSource: "dev-sso",
			Auth: &config.ContextAuth{
				Type:         "sso",
				SSOStartURL:  "https://example.awsapps.com/start",
				SSOAccountID: "333333333333",
				SSORoleName:  "DeveloperRole",
			},
			Resources: &config.ContextResources{DefaultRegion: "ap-northeast-2"},
		}},
		Orphans: []string{"dev-sso-222222222222-stale"},
	}

	if err := ApplyContextSyncPlan(configPath, plan, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	contexts, err := config.Contexts(configPath)
	if err != nil {
		t.Fatalf("failed to list contexts: %v", err)
	}
	byName := map[string]config.ContextInfo{}
	for _, ctx := range contexts {
		byName[ctx.Name] = ctx
	}
	added, ok := byName["dev-sso-333333333333-developerrole"]
	if !ok || added.SyncSource != "dev-sso" {
		t.Fatalf("expected sync-managed context to be persisted, got %+v", byName)
	}
	if _, ok := byName["dev-sso-222222222222-stale"]; !ok {
		t.Fatal("expected orphan to survive without --prune")
	}
}

func TestApplyContextSyncPlanPruneRemovesOnlyOrphans(t *testing.T) {
	configPath := writeConfig(t, t.TempDir(), syncBaseConfig)
	plan := ContextSyncPlan{
		Base:    "dev-sso",
		Orphans: []string{"dev-sso-222222222222-stale"},
	}

	if err := ApplyContextSyncPlan(configPath, plan, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	contexts, err := config.Contexts(configPath)
	if err != nil {
		t.Fatalf("failed to list contexts: %v", err)
	}
	names := map[string]bool{}
	for _, ctx := range contexts {
		names[ctx.Name] = true
	}
	if names["dev-sso-222222222222-stale"] {
		t.Fatal("expected orphan to be pruned")
	}
	if !names["dev-sso"] || !names["manual-admin"] {
		t.Fatalf("expected base and manual contexts to survive prune, got %+v", names)
	}
}
