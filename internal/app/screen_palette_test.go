package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	"unic/internal/domain"
)

func paletteTestModel() Model {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	m.ctxList = []config.ContextInfo{{Name: "prod-admin"}, {Name: "dev"}}
	m.screen = screenServiceList
	return m
}

func TestOpenPaletteBuildsFeatureAndContextItems(t *testing.T) {
	m := paletteTestModel()

	updated, cmd := m.openPalette()
	model := updated.(Model)
	if model.screen != screenCommandPalette {
		t.Fatalf("expected palette screen, got %v", model.screen)
	}
	if cmd == nil {
		t.Fatal("expected resource indexing to start")
	}
	if !model.palette.indexing {
		t.Fatal("expected indexing flag while the resource index builds")
	}

	var hasFeature, hasContext bool
	for _, item := range model.palette.filtered {
		if item.kind == paletteItemFeature && item.feature == domain.FeatureRDSBrowser {
			hasFeature = true
		}
		if item.kind == paletteItemContext && item.contextName == "prod-admin" {
			hasContext = true
		}
	}
	if !hasFeature || !hasContext {
		t.Fatalf("expected feature and context items, feature=%v context=%v", hasFeature, hasContext)
	}
}

func TestPaletteQueryFiltersItems(t *testing.T) {
	m := paletteTestModel()
	updated, _ := m.openPalette()
	m = updated.(Model)

	for _, r := range "rds browser" {
		next, _ := m.updatePalette(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	if len(m.palette.filtered) == 0 {
		t.Fatal("expected fuzzy matches for 'rds browser'")
	}
	for _, item := range m.palette.filtered {
		if !strings.Contains(item.FilterText(), "rds") {
			t.Fatalf("expected only RDS-related matches, got %q", item.label)
		}
	}
}

func TestPaletteResourceJumpPrefillsBrowserFilter(t *testing.T) {
	m := paletteTestModel()
	updated, _ := m.openPalette()
	m = updated.(Model)

	item := paletteResourceItem("RDS", "prod-db", "prod-db postgres",
		domain.FeatureRDSBrowser, domain.ServiceRDS, filterRDS, "prod-db")
	next, cmd := m.executePaletteItem(item)
	model := next.(Model)
	if cmd == nil {
		t.Fatal("expected the RDS browser to start loading")
	}
	if model.filterValue(filterRDS) != "prod-db" {
		t.Fatalf("expected RDS filter prefilled with the resource key, got %q", model.filterValue(filterRDS))
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen for the jump, got %v", model.screen)
	}
}

func TestPaletteIndexedResourcesStreamIn(t *testing.T) {
	m := paletteTestModel()
	updated, _ := m.openPalette()
	m = updated.(Model)

	items := []paletteItem{paletteResourceItem("EC2", "web-1 (i-123)", "web-1 i-123",
		domain.FeatureEC2InstanceBrowser, domain.ServiceEC2, filterEC2BrowserInstances, "i-123")}
	next, _ := m.Update(paletteResourcesIndexedMsg{generation: m.palette.generation, items: items, errs: []string{"Route53: access denied"}})
	model := next.(Model)

	if model.palette.indexing {
		t.Fatal("expected indexing flag to clear")
	}
	found := false
	for _, item := range model.palette.filtered {
		if item.kind == paletteItemResource && item.resourceKey == "i-123" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected indexed resource to appear in results")
	}
	view := model.viewPalette()
	if !strings.Contains(view, "Route53: access denied") {
		t.Fatalf("expected per-service index error surfaced, got:\n%s", view)
	}
}

func TestPaletteEscReturnsToPreviousScreen(t *testing.T) {
	m := paletteTestModel()
	m.screen = screenFeatureList
	updated, _ := m.openPalette()
	m = updated.(Model)

	next, _ := m.updatePalette(tea.KeyMsg{Type: tea.KeyEsc})
	model := next.(Model)
	if model.screen != screenFeatureList {
		t.Fatalf("expected esc to restore the previous screen, got %v", model.screen)
	}
}

func TestPaletteIgnoresStaleIndexGenerations(t *testing.T) {
	m := paletteTestModel()

	// First open (generation 1), close, reopen (generation 2).
	updated, _ := m.openPalette()
	m = updated.(Model)
	staleGen := m.palette.generation
	next, _ := m.updatePalette(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	updated, _ = m.openPalette()
	m = updated.(Model)

	// The slow index from the first open finishes last: it must be dropped.
	staleItems := []paletteItem{paletteResourceItem("EC2", "stale (i-old)", "stale i-old",
		domain.FeatureEC2InstanceBrowser, domain.ServiceEC2, filterEC2BrowserInstances, "i-old")}
	next, _ = m.Update(paletteResourcesIndexedMsg{generation: staleGen, items: staleItems, errs: []string{"stale error"}})
	m = next.(Model)
	if !m.palette.indexing {
		t.Fatal("expected stale results to leave the current indexing state untouched")
	}
	if len(m.palette.resources) != 0 || len(m.palette.indexErrs) != 0 {
		t.Fatalf("expected stale results to be dropped, got %d resources %v", len(m.palette.resources), m.palette.indexErrs)
	}

	// The current generation's results still apply.
	freshItems := []paletteItem{paletteResourceItem("EC2", "fresh (i-new)", "fresh i-new",
		domain.FeatureEC2InstanceBrowser, domain.ServiceEC2, filterEC2BrowserInstances, "i-new")}
	next, _ = m.Update(paletteResourcesIndexedMsg{generation: m.palette.generation, items: freshItems})
	m = next.(Model)
	if m.palette.indexing || len(m.palette.resources) != 1 || m.palette.resources[0].resourceKey != "i-new" {
		t.Fatalf("expected current-generation results to apply, got %+v indexing=%v", m.palette.resources, m.palette.indexing)
	}
}

func TestPaletteBackspaceIsRuneSafe(t *testing.T) {
	m := paletteTestModel()
	updated, _ := m.openPalette()
	m = updated.(Model)

	next, _ := m.updatePalette(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'한'}})
	m = next.(Model)
	next, _ = m.updatePalette(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'글'}})
	m = next.(Model)
	if m.palette.query != "한글" {
		t.Fatalf("expected multibyte query, got %q", m.palette.query)
	}

	next, _ = m.updatePalette(tea.KeyMsg{Type: tea.KeyBackspace})
	m = next.(Model)
	if m.palette.query != "한" {
		t.Fatalf("expected backspace to remove one rune, got %q", m.palette.query)
	}
}

func TestPaletteCrossContextIndexIncludesSyncedContextsAndFailures(t *testing.T) {
	originalLoad := paletteLoadNamedContextFn
	originalIndex := paletteIndexContextFn
	t.Cleanup(func() {
		paletteLoadNamedContextFn = originalLoad
		paletteIndexContextFn = originalIndex
	})

	m := paletteTestModel()
	m.ctxList = []config.ContextInfo{
		{Name: "dev", Current: true},
		{Name: "prod-admin", SyncSource: "company-sso"},
		{Name: "manual"},
	}
	paletteLoadNamedContextFn = func(_ string, name string) (*config.Config, error) {
		return &config.Config{ContextName: name, Region: "eu-west-1"}, nil
	}
	paletteIndexContextFn = func(_ context.Context, cfg *config.Config, name string) ([]paletteItem, []string) {
		if name == "prod-admin" {
			return nil, []string{fmt.Sprintf("%s/%s: access denied", name, cfg.Region)}
		}
		item := paletteResourceItem("EC2", "web", "web", domain.FeatureEC2InstanceBrowser,
			domain.ServiceEC2, filterEC2BrowserInstances, "i-123")
		item.contextName = name
		return []paletteItem{item}, nil
	}

	updated, _ := m.openPalette()
	m = updated.(Model)
	next, cmd := m.updatePalette(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	msg := cmd()
	next, _ = m.Update(msg)
	m = next.(Model)

	if !m.palette.crossContext || m.palette.indexing {
		t.Fatalf("expected completed cross-context index, state=%+v", m.palette)
	}
	if len(m.palette.resources) != 1 || m.palette.resources[0].contextName != "dev" {
		t.Fatalf("expected only active-context resource, got %+v", m.palette.resources)
	}
	if got := strings.Join(m.palette.indexErrs, "\n"); !strings.Contains(got, "prod-admin/eu-west-1: access denied") {
		t.Fatalf("expected synced-context failure, got %q", got)
	}
}

func TestPaletteCrossContextResourceJumpSwitchesThenAppliesView(t *testing.T) {
	m := paletteTestModel()
	item := paletteResourceItem("RDS", "prod-db", "prod-db", domain.FeatureRDSBrowser,
		domain.ServiceRDS, filterRDS, "prod-db")
	item.contextName = "prod-admin"

	next, cmd := m.executePaletteItem(item)
	model := next.(Model)
	if cmd == nil || model.pendingView == nil {
		t.Fatal("expected context switch with a deferred resource jump")
	}
	if model.pendingView.Context != "prod-admin" || model.pendingView.Filter != "prod-db" {
		t.Fatalf("unexpected deferred jump: %+v", model.pendingView)
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen during context switch, got %v", model.screen)
	}
}
