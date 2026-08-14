package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"unic/internal/auth"
	"unic/internal/config"
)

func stubSyncSeams(t *testing.T, contexts []config.ContextInfo, plan auth.ContextSyncPlan) (applied *bool, pruned *bool) {
	t.Helper()
	origPath := defaultPathFn
	origEnsure := ensureConfigExistsFn
	origList := listContextsFn
	origBuild := buildSyncPlanFn
	origApply := applySyncPlanFn
	t.Cleanup(func() {
		defaultPathFn = origPath
		ensureConfigExistsFn = origEnsure
		listContextsFn = origList
		buildSyncPlanFn = origBuild
		applySyncPlanFn = origApply
	})

	defaultPathFn = func() (string, error) { return "/tmp/unused-config.yaml", nil }
	ensureConfigExistsFn = func(string) error { return nil }
	listContextsFn = func(string) ([]config.ContextInfo, error) { return contexts, nil }
	buildSyncPlanFn = func(context.Context, string, config.ContextInfo) (auth.ContextSyncPlan, error) {
		return plan, nil
	}
	applied = new(bool)
	pruned = new(bool)
	applySyncPlanFn = func(_ string, _ auth.ContextSyncPlan, prune bool) error {
		*applied = true
		*pruned = prune
		return nil
	}
	return applied, pruned
}

func baseSSOContexts() []config.ContextInfo {
	return []config.ContextInfo{
		{Name: "dev-sso", AuthType: "sso", SSOStartURL: "https://example.awsapps.com/start"},
		{Name: "manual-admin", AuthType: "sso", SSOStartURL: "https://example.awsapps.com/start", SSOAccountID: "111111111111", SSORoleName: "Admin"},
	}
}

func runContextSync(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"context", "sync"}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func TestContextSyncAppliesPlanAndPrintsSummary(t *testing.T) {
	plan := auth.ContextSyncPlan{
		Base:    "dev-sso",
		Add:     []config.ContextEntry{{Name: "dev-sso-333333333333-developerrole"}},
		Keep:    []string{"manual-admin"},
		Orphans: []string{"dev-sso-222222222222-stale"},
	}
	applied, pruned := stubSyncSeams(t, baseSSOContexts(), plan)

	out, err := runContextSync(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*applied || *pruned {
		t.Fatalf("expected plan applied without prune, applied=%v pruned=%v", *applied, *pruned)
	}
	for _, want := range []string{
		"add:    dev-sso-333333333333-developerrole",
		"orphan: dev-sso-222222222222-stale",
		"sync dev-sso: 1 added, 1 kept, 1 orphaned",
		"use --prune",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestContextSyncDryRunDoesNotApply(t *testing.T) {
	applied, _ := stubSyncSeams(t, baseSSOContexts(), auth.ContextSyncPlan{Base: "dev-sso"})

	out, err := runContextSync(t, "--dry-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *applied {
		t.Fatal("expected dry run to skip applying the plan")
	}
	if !strings.Contains(out, "dry run, nothing written") {
		t.Fatalf("expected dry-run marker in output, got:\n%s", out)
	}
}

func TestContextSyncPruneFlagPassesThrough(t *testing.T) {
	applied, pruned := stubSyncSeams(t, baseSSOContexts(), auth.ContextSyncPlan{
		Base:    "dev-sso",
		Orphans: []string{"dev-sso-222222222222-stale"},
	})

	out, err := runContextSync(t, "--prune")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*applied || !*pruned {
		t.Fatalf("expected plan applied with prune, applied=%v pruned=%v", *applied, *pruned)
	}
	if !strings.Contains(out, "remove: dev-sso-222222222222-stale") {
		t.Fatalf("expected remove line in output, got:\n%s", out)
	}
}

func TestContextSyncRejectsNonBaseContextArgument(t *testing.T) {
	stubSyncSeams(t, baseSSOContexts(), auth.ContextSyncPlan{})

	if _, err := runContextSync(t, "manual-admin"); err == nil || !strings.Contains(err.Error(), "not an SSO base context") {
		t.Fatalf("expected non-base context rejection, got %v", err)
	}
	if _, err := runContextSync(t, "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing context rejection, got %v", err)
	}
}

func TestContextSyncRequiresArgumentWhenMultipleBases(t *testing.T) {
	contexts := append(baseSSOContexts(), config.ContextInfo{
		Name: "other-sso", AuthType: "sso", SSOStartURL: "https://other.awsapps.com/start",
	})
	stubSyncSeams(t, contexts, auth.ContextSyncPlan{})

	if _, err := runContextSync(t); err == nil || !strings.Contains(err.Error(), "multiple SSO base contexts") {
		t.Fatalf("expected multiple-base error, got %v", err)
	}
}
