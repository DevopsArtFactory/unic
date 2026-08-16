package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	"unic/internal/domain"
)

func viewsTestModel(t *testing.T) Model {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	m := New(testConfig(), configPath, "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	m.screen = screenServiceList
	return m
}

func typeIntoViews(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		next, _ := m.updateViews(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	return m
}

// drillToRDS selects RDS through the real sorted/filtered service list, so
// the capture path is exercised with display indices that differ from the
// catalog order.
func drillToRDS(t *testing.T, m Model) Model {
	t.Helper()
	for i, svc := range m.serviceList() {
		if svc.Name == domain.ServiceRDS {
			m.svcIdx = i
			next, _ := m.updateServiceList(tea.KeyMsg{Type: tea.KeyEnter})
			return next.(Model)
		}
	}
	t.Fatal("RDS service not found in service list")
	return m
}

func TestViewsSaveCapturesFeatureFilterAndContext(t *testing.T) {
	m := viewsTestModel(t)
	m = drillToRDS(t, m)
	m.storeFilterValue(filterRDS, "prod-db")

	updated, _ := m.openViews()
	m = updated.(Model)
	next, _ := m.updateViews(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = next.(Model)
	m = typeIntoViews(t, m, "incident-rds")
	next, _ = m.updateViews(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	views, err := config.Views(m.configPath)
	if err != nil || len(views) != 1 {
		t.Fatalf("expected one persisted view, got %+v err=%v", views, err)
	}
	view := views[0]
	if view.Name != "incident-rds" || view.Service != "RDS" || view.Filter != "prod-db" || view.Context != "dev" {
		t.Fatalf("expected captured view fields, got %+v", view)
	}
	if !strings.Contains(m.views.notice, "Saved view") {
		t.Fatalf("expected save notice, got %q", m.views.notice)
	}
}

func TestViewsApplySameContextJumpsWithFilterPrefilled(t *testing.T) {
	m := viewsTestModel(t)
	view := config.ViewEntry{
		Name: "incident-rds", Context: "dev",
		Service: "RDS", Feature: string(domain.FeatureRDSBrowser), Filter: "prod-db",
	}

	next, cmd := m.applyView(view)
	model := next.(Model)
	if cmd == nil {
		t.Fatal("expected the RDS browser to start loading")
	}
	if model.filterValue(filterRDS) != "prod-db" {
		t.Fatalf("expected prefilled RDS filter, got %q", model.filterValue(filterRDS))
	}
	if model.pendingView != nil {
		t.Fatal("expected no pending view for a same-context apply")
	}
}

func TestViewsCaptureAndApplyKMSFilter(t *testing.T) {
	m := viewsTestModel(t)
	m.enterServiceForPalette(paletteItem{
		service: domain.ServiceKMS,
		feature: domain.FeatureKMSKeyBrowser,
	})
	m.storeFilterValue(filterKMSKeys, "alias/prod")

	view, ok := m.captureCurrentView()
	if !ok || view.Filter != "alias/prod" {
		t.Fatalf("expected captured KMS filter, got %+v", view)
	}

	replayed := viewsTestModel(t)
	next, cmd := replayed.applyView(view)
	model := next.(Model)
	if cmd == nil || model.filterValue(filterKMSKeys) != "alias/prod" {
		t.Fatalf("expected replayed KMS filter, got %q", model.filterValue(filterKMSKeys))
	}
}

func TestViewsApplyAcrossContextsDefersJumpUntilSwitch(t *testing.T) {
	m := viewsTestModel(t)
	view := config.ViewEntry{
		Name: "prod-incident", Context: "prod-admin",
		Service: "RDS", Feature: string(domain.FeatureRDSBrowser), Filter: "prod-db",
	}

	next, cmd := m.applyView(view)
	model := next.(Model)
	if cmd == nil || model.pendingView == nil || model.pendingView.Name != "prod-incident" {
		t.Fatalf("expected context switch with pending view, got %+v", model.pendingView)
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen during switch, got %v", model.screen)
	}

	// Context switch completes: the pending view jump continues.
	switched, _, handled := model.handleContextMsg(contextSwitchedMsg{
		cfg: &config.Config{Region: "us-east-1", ContextName: "prod-admin"},
	})
	if !handled {
		t.Fatal("expected context switch message to be handled")
	}
	after := switched.(Model)
	if after.pendingView != nil {
		t.Fatal("expected pending view to be consumed")
	}
	if after.filterValue(filterRDS) != "prod-db" {
		t.Fatalf("expected prefilled filter after switch, got %q", after.filterValue(filterRDS))
	}
}

func TestViewsDeleteRemovesPersistedView(t *testing.T) {
	m := viewsTestModel(t)
	if err := config.SaveView(m.configPath, config.ViewEntry{
		Name: "old", Service: "RDS", Feature: string(domain.FeatureRDSBrowser),
	}); err != nil {
		t.Fatal(err)
	}

	updated, _ := m.openViews()
	m = updated.(Model)
	next, _ := m.updateViews(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = next.(Model)

	views, err := config.Views(m.configPath)
	if err != nil || len(views) != 0 {
		t.Fatalf("expected the view to be deleted, got %+v err=%v", views, err)
	}
}

func TestViewsNamingCapturesGlobalShortcutLetters(t *testing.T) {
	m := viewsTestModel(t)
	m = drillToRDS(t, m)
	updated, _ := m.openViews()
	m = updated.(Model)
	next, _ := m.updateViews(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = next.(Model)

	if !m.isTextEntryScreen() {
		t.Fatal("expected view naming to count as text entry so global shortcuts stay out")
	}
	m = typeIntoViews(t, m, "PSVH")
	if m.views.nameInput != "PSVH" {
		t.Fatalf("expected uppercase letters to land in the name input, got %q", m.views.nameInput)
	}
}

func TestViewsCaptureUsesServiceIdentityNotDisplayIndex(t *testing.T) {
	m := viewsTestModel(t)
	m = drillToRDS(t, m)

	view, ok := m.captureCurrentView()
	if !ok {
		t.Fatal("expected a capturable view after drilling into RDS")
	}
	if view.Service != "RDS" || view.Feature != string(m.features[m.featIdx].Kind) {
		t.Fatalf("expected service identity RDS from the sorted list, got %+v", view)
	}
}

func TestFailedViewSwitchClearsPendingView(t *testing.T) {
	m := viewsTestModel(t)
	view := config.ViewEntry{
		Name: "prod-incident", Context: "prod-admin",
		Service: "RDS", Feature: string(domain.FeatureRDSBrowser), Filter: "prod-db",
	}
	next, _ := m.applyView(view)
	m = next.(Model)
	if m.pendingView == nil {
		t.Fatal("expected pending view before the switch completes")
	}

	// The switch fails: the deferred jump must be disarmed.
	failed, _ := m.Update(errMsg{err: errViewSwitch})
	m = failed.(Model)
	if m.pendingView != nil {
		t.Fatal("expected failed switch to clear the pending view")
	}

	// A later unrelated successful switch must not jump anywhere.
	switched, _, _ := m.handleContextMsg(contextSwitchedMsg{
		cfg: &config.Config{Region: "us-east-1", ContextName: "other"},
	})
	after := switched.(Model)
	if after.filterValue(filterRDS) != "" {
		t.Fatalf("expected no stale view application, filter=%q", after.filterValue(filterRDS))
	}
}

var errViewSwitch = errors.New("sso login failed")
