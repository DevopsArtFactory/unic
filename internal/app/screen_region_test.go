package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func TestGlobalRegionShortcutOpensPicker(t *testing.T) {
	m := New(&config.Config{
		ContextName: "production",
		Region:      "ap-northeast-2",
		Regions:     []string{"ap-northeast-2", "us-east-1"},
	}, "", "dev")
	m.screen = screenServiceList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	model := updated.(Model)
	if model.screen != screenRegionPicker {
		t.Fatalf("expected region picker, got %v", model.screen)
	}
	if model.regionIdx != 0 {
		t.Fatalf("expected active region at index 0, got %d", model.regionIdx)
	}
}

func TestRegionSwitchedMessageUpdatesRuntimeRegion(t *testing.T) {
	m := New(&config.Config{
		ContextName: "production",
		Region:      "ap-northeast-2",
		Regions:     []string{"ap-northeast-2", "us-east-1"},
	}, "", "dev")
	repo := &awsservice.AwsRepository{Region: "us-east-1"}

	updated, _ := m.Update(regionSwitchedMsg{region: "us-east-1", repo: repo})
	model := updated.(Model)
	if model.cfg.Region != "us-east-1" {
		t.Fatalf("expected active region us-east-1, got %q", model.cfg.Region)
	}
	if model.awsRepo != repo {
		t.Fatal("expected switched repository to be installed")
	}
	if model.screen != screenServiceList {
		t.Fatalf("expected service list after switch, got %v", model.screen)
	}
}

func TestSingleRegionContextDoesNotOpenPicker(t *testing.T) {
	m := New(&config.Config{
		ContextName: "production",
		Region:      "ap-northeast-2",
		Regions:     []string{"ap-northeast-2"},
	}, "", "dev")
	m.screen = screenServiceList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if model := updated.(Model); model.screen != screenServiceList {
		t.Fatalf("single-region context should stay on service list, got %v", model.screen)
	}
}
