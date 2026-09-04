package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aws/smithy-go"

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
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(append([]string{"context", "sync"}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func TestContextSyncPrintsPlanWithoutApplying(t *testing.T) {
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
	if *applied || *pruned {
		t.Fatalf("expected plan only, applied=%v pruned=%v", *applied, *pruned)
	}
	for _, want := range []string{
		"add:    dev-sso-333333333333-developerrole",
		"orphan: dev-sso-222222222222-stale",
		"sync dev-sso: 1 added, 1 kept, 1 orphaned (preview, nothing written)",
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
	if strings.Contains(out, "preview, nothing written") {
		t.Fatalf("explicit dry run should not use the default preview marker, got:\n%s", out)
	}
}

func TestContextSyncPruneFlagPassesThrough(t *testing.T) {
	applied, pruned := stubSyncSeams(t, baseSSOContexts(), auth.ContextSyncPlan{
		Base:    "dev-sso",
		Orphans: []string{"dev-sso-222222222222-stale"},
	})

	out, err := runContextSync(t, "--prune", "--apply", "--confirm", "dev-sso")
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

func TestContextSyncRejectsConfirmationMismatch(t *testing.T) {
	applied, _ := stubSyncSeams(t, baseSSOContexts(), auth.ContextSyncPlan{Base: "dev-sso"})

	if _, err := runContextSync(t, "--apply", "--confirm", "wrong"); err == nil || !strings.Contains(err.Error(), "confirmation mismatch") {
		t.Fatalf("expected confirmation mismatch, got %v", err)
	}
	if *applied {
		t.Fatal("confirmation mismatch must not apply")
	}
}

func TestContextSyncJSONConfirmationMismatchHasStableCode(t *testing.T) {
	stubSyncSeams(t, baseSSOContexts(), auth.ContextSyncPlan{Base: "dev-sso"})
	out, err := runContextSync(t, "--json", "--apply", "--confirm", "wrong")
	if err == nil {
		t.Fatal("expected confirmation mismatch")
	}
	var got StructuredError
	if decodeErr := json.Unmarshal([]byte(out), &got); decodeErr != nil {
		t.Fatalf("invalid error JSON: %v\n%s", decodeErr, out)
	}
	if got.Code != "confirmation_mismatch" || got.Retryable {
		t.Fatalf("unexpected error: %#v", got)
	}
}

func TestContextSyncJSONPlanAndApplyResult(t *testing.T) {
	plan := auth.ContextSyncPlan{Base: "dev-sso", Add: []config.ContextEntry{{Name: "new-context"}}}
	applied, _ := stubSyncSeams(t, baseSSOContexts(), plan)

	out, err := runContextSync(t, "--json")
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	var preview MutationPlan
	if err := json.Unmarshal([]byte(out), &preview); err != nil {
		t.Fatalf("invalid plan JSON: %v\n%s", err, out)
	}
	if preview.Version != "v1" || preview.Operation != "context.sync" || preview.Confirmation != "dev-sso" || *applied {
		t.Fatalf("unexpected preview: %#v applied=%v", preview, *applied)
	}

	out, err = runContextSync(t, "--json", "--apply", "--confirm", "dev-sso")
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	var result MutationResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid result JSON: %v\n%s", err, out)
	}
	if !result.Changed || result.RollbackHint == "" || !*applied {
		t.Fatalf("unexpected result: %#v applied=%v", result, *applied)
	}
}

func TestContextSyncJSONErrorsClassifyRetryAndPermission(t *testing.T) {
	tests := []struct {
		name       string
		apiError   error
		retryable  bool
		permission bool
	}{
		{name: "retryable", apiError: &smithy.GenericAPIError{Code: "ThrottlingException", Message: "slow down", Fault: smithy.FaultClient}, retryable: true},
		{name: "permission", apiError: &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "denied", Fault: smithy.FaultClient}, permission: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubSyncSeams(t, baseSSOContexts(), auth.ContextSyncPlan{})
			orig := buildSyncPlanFn
			buildSyncPlanFn = func(context.Context, string, config.ContextInfo) (auth.ContextSyncPlan, error) {
				return auth.ContextSyncPlan{}, tt.apiError
			}
			t.Cleanup(func() { buildSyncPlanFn = orig })

			out, err := runContextSync(t, "--json")
			if err == nil {
				t.Fatal("expected failure")
			}
			var got StructuredError
			if decodeErr := json.Unmarshal([]byte(out), &got); decodeErr != nil {
				t.Fatalf("invalid error JSON: %v\n%s", decodeErr, out)
			}
			if got.Retryable != tt.retryable || (got.RequiredPermission != "") != tt.permission {
				t.Fatalf("unexpected error: %#v", got)
			}
		})
	}
}

func TestMutationErrorHandlesOrdinaryErrors(t *testing.T) {
	got := mutationError(errors.New("broken"), "")
	if got.Code != "operation_failed" || got.Message != "broken" || got.Retryable {
		t.Fatalf("unexpected error: %#v", got)
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
