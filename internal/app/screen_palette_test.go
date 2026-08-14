package app

import (
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
	next, _ := m.Update(paletteResourcesIndexedMsg{items: items, errs: []string{"Route53: access denied"}})
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
