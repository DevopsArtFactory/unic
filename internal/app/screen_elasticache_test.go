package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func elasticacheTestResources() []awsservice.ElastiCacheResource {
	return []awsservice.ElastiCacheResource{
		{
			ID: "prod-rg", Kind: "replication group", Engine: "valkey", EngineVersion: "8.0",
			Status: "available", NodeType: "cache.r7g.large", Endpoint: "prod.cache.amazonaws.com:6379",
			Nodes: []awsservice.ElastiCacheNode{{ID: "0001", ClusterID: "prod-cache-001", ShardID: "0001", Role: "primary", Status: "available", AZ: "us-east-1a", Endpoint: "prod-node.cache.amazonaws.com:6379"}},
		},
		{ID: "dev-memcached", Kind: "cluster", Engine: "memcached", Status: "available", NodeType: "cache.t4g.small"},
	}
}

func TestElastiCacheResourcesLoadedRendersAndFilters(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading

	_, _, handled := m.elasticache.HandleMessage(&m, elasticacheResourcesLoadedMsg{resources: elasticacheTestResources()})
	if !handled || m.screen != screenElastiCacheResourceList {
		t.Fatalf("expected ElastiCache resource list, got screen=%v handled=%v", m.screen, handled)
	}
	view, ok := m.elasticache.View(m)
	if !ok || !strings.Contains(view, "prod-rg") || !strings.Contains(view, "dev-memcached") || !strings.Contains(view, "NODES") {
		t.Fatalf("expected resource table, got:\n%s", view)
	}

	m.storeFilterValue(filterElastiCacheResources, "valkey")
	m.applyFilterTarget(filterElastiCacheResources)
	if len(m.elasticache.filtered) != 1 || m.elasticache.filtered[0].ID != "prod-rg" {
		t.Fatalf("expected Valkey filter result, got %+v", m.elasticache.filtered)
	}

	empty := New(testConfig(), "", "dev")
	empty.elasticache.HandleMessage(&empty, elasticacheResourcesLoadedMsg{})
	emptyView, _ := empty.elasticache.View(empty)
	if !strings.Contains(emptyView, "No ElastiCache resources found") {
		t.Fatalf("expected empty resource view, got:\n%s", emptyView)
	}
}

func TestElastiCacheNodeDrillDownAndEndpointCopy(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.elasticache.HandleMessage(&m, elasticacheResourcesLoadedMsg{resources: elasticacheTestResources()})

	m.elasticache.updateResourceList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenElastiCacheNodeList || m.elasticache.selectedResource == nil {
		t.Fatalf("expected node list, got screen=%v resource=%+v", m.screen, m.elasticache.selectedResource)
	}
	nodeView, _ := m.elasticache.View(m)
	if !strings.Contains(nodeView, "prod-cache-001") || !strings.Contains(nodeView, "primary") {
		t.Fatalf("expected node metadata, got:\n%s", nodeView)
	}

	m.elasticache.updateNodeList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenElastiCacheNodeDetail || m.elasticache.selectedNode == nil {
		t.Fatalf("expected node detail, got screen=%v node=%+v", m.screen, m.elasticache.selectedNode)
	}
	var copied string
	originalCopy := elasticacheCopyFn
	elasticacheCopyFn = func(value string) error { copied = value; return nil }
	defer func() { elasticacheCopyFn = originalCopy }()

	m.elasticache.updateNodeDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if copied != "prod-node.cache.amazonaws.com:6379" {
		t.Fatalf("unexpected copied endpoint %q", copied)
	}
	detailView, _ := m.elasticache.View(m)
	if !strings.Contains(detailView, "Copied endpoint to clipboard") || !strings.Contains(detailView, copied) {
		t.Fatalf("expected endpoint and copy notice, got:\n%s", detailView)
	}

	m.elasticache.updateNodeDetail(&m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenElastiCacheNodeList {
		t.Fatalf("expected esc to return to nodes, got %v", m.screen)
	}
	m.elasticache.updateNodeList(&m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenElastiCacheResourceList {
		t.Fatalf("expected esc to return to resources, got %v", m.screen)
	}
}

func TestElastiCacheSavedViewUsesResourceFilter(t *testing.T) {
	if target, ok := featurePrimaryFilter["ElastiCache Browser"]; !ok || target != filterElastiCacheResources {
		t.Fatalf("expected ElastiCache saved-view filter, got %v %v", target, ok)
	}
}

func TestElastiCacheHelpScreenTitles(t *testing.T) {
	m := New(testConfig(), "", "dev")
	for screen, want := range map[screen]string{
		screenElastiCacheResourceList: "ElastiCache Resources",
		screenElastiCacheNodeList:     "ElastiCache Nodes",
		screenElastiCacheNodeDetail:   "ElastiCache Node Detail",
	} {
		m.screen = screen
		if got := m.helpScreenTitle(); got != want {
			t.Fatalf("screen %v: expected %q, got %q", screen, want, got)
		}
	}
}
