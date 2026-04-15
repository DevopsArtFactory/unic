package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/auth"
	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func testContexts() []config.ContextInfo {
	return []config.ContextInfo{
		{Name: "dev", Region: "us-east-1", AuthType: "credential"},
		{Name: "prod", Region: "us-west-2", AuthType: "sso", Current: true},
		{Name: "staging", Region: "ap-northeast-2", AuthType: "assume_role"},
	}
}

func writeContextConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestContextsLoadedSyncsContextTable(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	model := updated.(Model)

	if model.screen != screenContextPicker {
		t.Fatalf("expected context picker screen, got %v", model.screen)
	}
	if got := len(model.contextTable.Rows()); got != 3 {
		t.Fatalf("expected 3 context rows, got %d", got)
	}
	if model.ctxIdx != 1 {
		t.Fatalf("expected current context cursor at index 1, got %d", model.ctxIdx)
	}
	if model.contextTable.Cursor() != 1 {
		t.Fatalf("expected table cursor at index 1, got %d", model.contextTable.Cursor())
	}
	selected := model.contextTable.SelectedRow()
	if len(selected) != 4 || selected[0] != "prod" || selected[3] != "*" {
		t.Fatalf("expected selected row for current context, got %#v", selected)
	}

	view := model.viewContextPicker()
	for _, want := range []string{"NAME", "AUTH TYPE", "CURRENT", "prod"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected context picker view to contain %q, got %q", want, view)
		}
	}
}

func TestContextPickerNavigationUsesTableModel(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	model := updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)

	if model.ctxIdx != 2 {
		t.Fatalf("expected cursor to move to index 2, got %d", model.ctxIdx)
	}
	if model.contextTable.Cursor() != 2 {
		t.Fatalf("expected table cursor to move to index 2, got %d", model.contextTable.Cursor())
	}
}

func TestContextPickerFilterUpdatesTableRows(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	model := updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updated.(Model)
	for _, ch := range []rune("prod") {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(Model)
	}

	if got := len(model.filteredCtxList); got != 1 {
		t.Fatalf("expected 1 filtered context, got %d", got)
	}
	if got := len(model.contextTable.Rows()); got != 1 {
		t.Fatalf("expected table to show 1 filtered row, got %d", got)
	}
	if selected := model.contextTable.SelectedRow(); len(selected) == 0 || selected[0] != "prod" {
		t.Fatalf("expected filtered table row for prod, got %#v", selected)
	}
}

func TestContextPickerIncrementalFilterStartsOnTyping(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	model := updated.(Model)

	for _, ch := range []rune("prod") {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(Model)
	}

	if !model.isFiltering(filterContexts) {
		t.Fatal("expected incremental typing to activate context filter")
	}
	if got := model.filterValue(filterContexts); got != "prod" {
		t.Fatalf("expected filter value 'prod', got %q", got)
	}
	if got := len(model.filteredCtxList); got != 1 {
		t.Fatalf("expected 1 filtered context, got %d", got)
	}
	if selected := model.contextTable.SelectedRow(); len(selected) == 0 || selected[0] != "prod" {
		t.Fatalf("expected filtered context prod, got %#v", selected)
	}
}

func TestContextPickerEscClearsFilterBeforeBackingOut(t *testing.T) {
	m := New(&config.Config{ContextName: "prod"}, "", "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	model := updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)

	if model.screen != screenContextPicker {
		t.Fatalf("expected to remain on context picker after clearing filter, got %v", model.screen)
	}
	if got := model.filterValue(filterContexts); got != "" {
		t.Fatalf("expected filter to be cleared, got %q", got)
	}
	if got := len(model.filteredCtxList); got != len(testContexts()) {
		t.Fatalf("expected full context list after clear, got %d", got)
	}
}

func TestContextPickerEnterUsesSelectedTableRow(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	model := updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if cmd == nil {
		t.Fatal("expected context switch command on enter")
	}
	if model.pendingContextName != "staging" {
		t.Fatalf("expected pending context staging, got %q", model.pendingContextName)
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen after selecting context, got %v", model.screen)
	}
}

func TestContextPickerShowsUNICAndShellContext(t *testing.T) {
	t.Setenv(auth.ContextEnvVar, "prod")

	m := New(&config.Config{ContextName: "staging", Region: "ap-northeast-2"}, "", "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	model := updated.(Model)
	view := model.viewContextPicker()

	for _, want := range []string{
		"UNIC current",
		"staging",
		"Shell env",
		"prod (UNIC_CONTEXT)",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected context picker view to contain %q, got %q", want, view)
		}
	}
}

func TestContextPickerSetupCopiesExportsAndQuits(t *testing.T) {
	path := writeContextConfig(t, `
current: prod
defaults:
  region: us-east-1
contexts:
  - name: dev
    auth_type: credential
    profile: dev-profile
    region: us-east-1
  - name: prod
    auth_type: credential
    profile: prod-profile
    region: us-west-2
`)

	origBuildEnv := contextBuildEnvExportsFn
	origCopy := contextCopyClipboardFn
	defer func() {
		contextBuildEnvExportsFn = origBuildEnv
		contextCopyClipboardFn = origCopy
	}()

	var copied string
	contextBuildEnvExportsFn = func(_ context.Context, cfg *config.Config) (string, error) {
		return fmt.Sprintf("export %s='%s'", auth.ContextEnvVar, cfg.ContextName), nil
	}
	contextCopyClipboardFn = func(text string) error {
		copied = text
		return nil
	}

	m := New(testConfig(), path, "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{
		{Name: "dev", Region: "us-east-1", AuthType: "credential"},
		{Name: "prod", Region: "us-west-2", AuthType: "credential", Current: true},
	}})
	model := updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(Model)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model = updated.(Model)
	if model.screen != screenLoading || cmd == nil {
		t.Fatalf("expected loading command for setup, got screen=%v cmd=%v", model.screen, cmd)
	}

	msg := model.setupSelectedContextForTerminal("dev")()
	updated, noticeCmd := model.Update(msg)
	model = updated.(Model)
	if model.quitting || noticeCmd != nil {
		t.Fatalf("expected setup success to open exit notice, got quitting=%v cmd=%v", model.quitting, noticeCmd)
	}
	if model.screen != screenExitNotice {
		t.Fatalf("expected exit notice screen, got %v", model.screen)
	}
	if model.exitTitle != "CONTEXT SET UP" {
		t.Fatalf("unexpected exit title: %q", model.exitTitle)
	}
	if model.exitMessage != "[dev] has been set up and copied. Goodbye!" {
		t.Fatalf("unexpected exit message: %q", model.exitMessage)
	}
	if copied != "export UNIC_CONTEXT='dev'" {
		t.Fatalf("unexpected clipboard exports: %q", copied)
	}
	if view := model.viewExitNotice(); !strings.Contains(view, "Press any key to exit unic") {
		t.Fatalf("expected exit notice prompt, got %q", view)
	}

	updated, quitCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if !model.quitting || quitCmd == nil {
		t.Fatalf("expected keypress on exit notice to quit the app")
	}

	cfg, err := config.Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextName != "dev" {
		t.Fatalf("expected current context to switch to dev, got %q", cfg.ContextName)
	}
}

func TestContextPickerCopyExportsDoesNotChangeCurrentContext(t *testing.T) {
	path := writeContextConfig(t, `
current: prod
defaults:
  region: us-east-1
contexts:
  - name: dev
    auth_type: credential
    profile: dev-profile
    region: us-east-1
  - name: prod
    auth_type: credential
    profile: prod-profile
    region: us-west-2
`)

	origBuildEnv := contextBuildEnvExportsFn
	origCopy := contextCopyClipboardFn
	defer func() {
		contextBuildEnvExportsFn = origBuildEnv
		contextCopyClipboardFn = origCopy
	}()

	var copied string
	contextBuildEnvExportsFn = func(_ context.Context, cfg *config.Config) (string, error) {
		return "exports-for-" + cfg.ContextName, nil
	}
	contextCopyClipboardFn = func(text string) error {
		copied = text
		return nil
	}

	m := New(testConfig(), path, "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{
		{Name: "dev", Region: "us-east-1", AuthType: "credential"},
		{Name: "prod", Region: "us-west-2", AuthType: "credential", Current: true},
	}})
	model := updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(Model)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model = updated.(Model)
	if model.screen != screenLoading || cmd == nil {
		t.Fatalf("expected loading command for copy env, got screen=%v cmd=%v", model.screen, cmd)
	}

	msg := model.copySelectedContextExports("dev")()
	updated, noticeCmd := model.Update(msg)
	model = updated.(Model)
	if model.quitting || noticeCmd != nil {
		t.Fatalf("expected copy exports to open exit notice, got quitting=%v cmd=%v", model.quitting, noticeCmd)
	}
	if model.screen != screenExitNotice {
		t.Fatalf("expected exit notice screen, got %v", model.screen)
	}
	if model.exitTitle != "EXPORTS COPIED" {
		t.Fatalf("unexpected exit title: %q", model.exitTitle)
	}
	if model.exitMessage != "[dev] exports have been copied. Goodbye!" {
		t.Fatalf("unexpected exit message: %q", model.exitMessage)
	}
	if copied != "exports-for-dev" {
		t.Fatalf("unexpected clipboard exports: %q", copied)
	}

	updated, quitCmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model = updated.(Model)
	if !model.quitting || quitCmd == nil {
		t.Fatalf("expected any key on exit notice to quit the app")
	}

	cfg, err := config.Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextName != "prod" {
		t.Fatalf("expected current context to remain prod, got %q", cfg.ContextName)
	}
}

func TestContextPickerUnsetCopiesCleanupAndClearsCurrent(t *testing.T) {
	path := writeContextConfig(t, `
current: prod
defaults:
  region: us-east-1
contexts:
  - name: prod
    auth_type: credential
    profile: prod-profile
    region: us-west-2
`)

	origCleanup := contextBuildEnvCleanupFn
	origCopy := contextCopyClipboardFn
	defer func() {
		contextBuildEnvCleanupFn = origCleanup
		contextCopyClipboardFn = origCopy
	}()

	var copied string
	contextBuildEnvCleanupFn = func() string { return "unset UNIC_CONTEXT" }
	contextCopyClipboardFn = func(text string) error {
		copied = text
		return nil
	}

	m := New(testConfig(), path, "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{
		{Name: "prod", Region: "us-west-2", AuthType: "credential", Current: true},
	}})
	model := updated.(Model)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	model = updated.(Model)
	if model.screen != screenLoading || cmd == nil {
		t.Fatalf("expected loading command for unset, got screen=%v cmd=%v", model.screen, cmd)
	}

	msg := model.unsetTerminalContext()()
	updated, noticeCmd := model.Update(msg)
	model = updated.(Model)
	if model.quitting || noticeCmd != nil {
		t.Fatalf("expected unset to open exit notice, got quitting=%v cmd=%v", model.quitting, noticeCmd)
	}
	if model.screen != screenExitNotice {
		t.Fatalf("expected exit notice screen, got %v", model.screen)
	}
	if model.exitTitle != "CONTEXT CLEARED" {
		t.Fatalf("unexpected exit title: %q", model.exitTitle)
	}
	if model.exitMessage != "Shell context has been cleared and cleanup commands copied. Goodbye!" {
		t.Fatalf("unexpected exit message: %q", model.exitMessage)
	}
	if copied != "unset UNIC_CONTEXT" {
		t.Fatalf("unexpected cleanup commands: %q", copied)
	}

	updated, quitCmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if !model.quitting || quitCmd == nil {
		t.Fatalf("expected exit notice dismissal to quit the app")
	}

	cfg, err := config.Load(nil, nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextName != "" {
		t.Fatalf("expected current context to be cleared, got %q", cfg.ContextName)
	}
}

func TestContextPickerSetupBaseSSOUsesNativeSelectionFlow(t *testing.T) {
	path := writeContextConfig(t, `
defaults:
  region: ap-northeast-2
contexts:
  - name: base-sso
    auth_type: sso
    profile: sso-profile
    region: ap-northeast-2
    sso_start_url: https://example.awsapps.com/start
`)

	origAccounts := contextListSSOAccountsFn
	origRoles := contextListSSORolesFn
	origResolve := contextResolveSSOSelectionFn
	origSetCurrent := contextSetCurrentFn
	origLoadNamed := contextLoadNamedContextFn
	origBuildEnv := contextBuildEnvExportsFn
	origCopy := contextCopyClipboardFn
	defer func() {
		contextListSSOAccountsFn = origAccounts
		contextListSSORolesFn = origRoles
		contextResolveSSOSelectionFn = origResolve
		contextSetCurrentFn = origSetCurrent
		contextLoadNamedContextFn = origLoadNamed
		contextBuildEnvExportsFn = origBuildEnv
		contextCopyClipboardFn = origCopy
	}()

	contextListSSOAccountsFn = func(_ context.Context, configPath string, selected config.ContextInfo) ([]awsservice.SSOAccount, error) {
		if configPath != path || selected.Name != "base-sso" {
			t.Fatalf("unexpected account selection input: path=%q selected=%+v", configPath, selected)
		}
		return []awsservice.SSOAccount{{ID: "123456789012", Name: "dev"}}, nil
	}
	contextListSSORolesFn = func(_ context.Context, configPath string, selected config.ContextInfo, accountID string) ([]awsservice.SSORole, error) {
		if configPath != path || selected.Name != "base-sso" || accountID != "123456789012" {
			t.Fatalf("unexpected role selection input: path=%q selected=%+v account=%q", configPath, selected, accountID)
		}
		return []awsservice.SSORole{{Name: "AdministratorAccess"}}, nil
	}
	contextResolveSSOSelectionFn = func(configPath string, selected config.ContextInfo, account awsservice.SSOAccount, role awsservice.SSORole) (string, error) {
		if configPath != path || selected.Name != "base-sso" || account.ID != "123456789012" || role.Name != "AdministratorAccess" {
			t.Fatalf("unexpected resolve input: path=%q selected=%+v account=%+v role=%+v", configPath, selected, account, role)
		}
		return "resolved-sso", nil
	}

	var currentSet string
	contextSetCurrentFn = func(_ string, name string) error {
		currentSet = name
		return nil
	}
	contextLoadNamedContextFn = func(_ string, name string) (*config.Config, error) {
		return &config.Config{
			ContextName:  name,
			AuthType:     config.AuthTypeSSO,
			Region:       "ap-northeast-2",
			SSOStartURL:  "https://example.awsapps.com/start",
			SSOAccountID: "123456789012",
			SSORoleName:  "AdministratorAccess",
		}, nil
	}
	contextBuildEnvExportsFn = func(_ context.Context, cfg *config.Config) (string, error) {
		return "exports-for-" + cfg.ContextName, nil
	}

	var copied string
	contextCopyClipboardFn = func(text string) error {
		copied = text
		return nil
	}

	m := New(testConfig(), path, "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{
		{Name: "base-sso", Region: "ap-northeast-2", AuthType: "sso", SSOStartURL: "https://example.awsapps.com/start"},
	}})
	model := updated.(Model)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model = updated.(Model)
	if model.screen != screenLoading || cmd == nil {
		t.Fatalf("expected loading command for base SSO setup, got screen=%v cmd=%v", model.screen, cmd)
	}

	updated, _ = model.Update(model.loadSSOContextAccounts(model.filteredCtxList[0])())
	model = updated.(Model)
	if model.screen != screenContextSSOAccountList || len(model.contextSSOAccounts) != 1 {
		t.Fatalf("expected SSO account selection screen, got screen=%v accounts=%d", model.screen, len(model.contextSSOAccounts))
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenLoading || cmd == nil {
		t.Fatalf("expected loading command for role lookup, got screen=%v cmd=%v", model.screen, cmd)
	}

	updated, _ = model.Update(model.loadSSOContextRoles(model.contextSSOBase, model.contextSSOAccounts[0])())
	model = updated.(Model)
	if model.screen != screenContextSSORoleList || len(model.contextSSORoles) != 1 {
		t.Fatalf("expected SSO role selection screen, got screen=%v roles=%d", model.screen, len(model.contextSSORoles))
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenLoading || cmd == nil {
		t.Fatalf("expected loading command for final setup, got screen=%v cmd=%v", model.screen, cmd)
	}

	updated, noticeCmd := model.Update(model.setupResolvedSSOContextForTerminal()())
	model = updated.(Model)
	if model.quitting || noticeCmd != nil {
		t.Fatalf("expected final setup to open exit notice, got quitting=%v cmd=%v", model.quitting, noticeCmd)
	}
	if model.screen != screenExitNotice {
		t.Fatalf("expected exit notice screen, got %v", model.screen)
	}
	if model.exitTitle != "CONTEXT SET UP" {
		t.Fatalf("unexpected exit title: %q", model.exitTitle)
	}
	if model.exitMessage != "[resolved-sso] has been set up and copied. Goodbye!" {
		t.Fatalf("unexpected exit message: %q", model.exitMessage)
	}
	if currentSet != "resolved-sso" {
		t.Fatalf("expected resolved context to be set current, got %q", currentSet)
	}
	if copied != "exports-for-resolved-sso" {
		t.Fatalf("unexpected clipboard exports: %q", copied)
	}

	updated, quitCmd := model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = updated.(Model)
	if !model.quitting || quitCmd == nil {
		t.Fatalf("expected exit notice dismissal to quit the app")
	}
}

func TestContextPickerSetupResolvedSSORejectsInvalidRoleSelection(t *testing.T) {
	m := New(testConfig(), "/tmp/config.yaml", "dev")
	m.contextSSOBase = config.ContextInfo{Name: "base-sso", AuthType: "sso"}
	m.contextSSOAccount = awsservice.SSOAccount{ID: "123456789012", Name: "dev"}
	m.contextSSORoles = []awsservice.SSORole{{Name: "AdministratorAccess"}}
	m.contextSSORoleIdx = 3

	msg := m.setupResolvedSSOContextForTerminal()()
	err, ok := msg.(errMsg)
	if !ok {
		t.Fatalf("expected errMsg for invalid role selection, got %T", msg)
	}
	if err.err == nil || err.err.Error() != "invalid role selection" {
		t.Fatalf("unexpected error: %v", err.err)
	}
}
