package app

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func keyA() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}}
}

func TestEC2BrowserAllRegionsToggleStartsReload(t *testing.T) {
	m := Model{
		cfg:    &config.Config{Region: "us-east-1", Regions: []string{"us-east-1", "eu-west-1"}},
		screen: screenEC2InstanceBrowserList,
	}

	newM, cmd := m.ec2Browser.updateList(&m, keyA())
	updated := newM.(Model)
	if !updated.ec2Browser.allRegions {
		t.Fatal("expected all-regions scope to be enabled")
	}
	if updated.screen != screenLoading {
		t.Fatalf("expected loading screen after toggle, got %v", updated.screen)
	}
	if cmd == nil {
		t.Fatal("expected reload command after toggle")
	}
}

func TestEC2BrowserAllRegionsToggleIgnoredForSingleRegion(t *testing.T) {
	m := Model{
		cfg:    &config.Config{Region: "us-east-1", Regions: []string{"us-east-1"}},
		screen: screenEC2InstanceBrowserList,
	}

	_, cmd := m.ec2Browser.updateList(&m, keyA())
	if m.ec2Browser.allRegions {
		t.Fatal("expected all-regions scope to stay disabled for single-region contexts")
	}
	if m.screen != screenEC2InstanceBrowserList || cmd != nil {
		t.Fatalf("expected no-op for single-region contexts, got screen %v", m.screen)
	}
}

func TestEC2BrowserStoresRegionErrorsFromLoadedMsg(t *testing.T) {
	m := Model{
		cfg:    &config.Config{Region: "us-east-1", Regions: []string{"us-east-1", "eu-west-1"}},
		screen: screenLoading,
	}
	m.ec2Browser.allRegions = true

	msg := ec2BrowserInstancesLoadedMsg{
		instances: []awsservice.EC2Instance{
			{InstanceID: "i-east", Region: "us-east-1"},
		},
		regionErrors: []awsservice.EC2RegionError{
			{Region: "eu-west-1", Err: errors.New("access denied")},
		},
	}
	_, _, handled := m.ec2Browser.HandleMessage(&m, msg)
	if !handled {
		t.Fatal("expected instances-loaded message to be handled")
	}
	if m.screen != screenEC2InstanceBrowserList {
		t.Fatalf("expected instance list screen, got %v", m.screen)
	}
	if len(m.ec2Browser.regionErrors) != 1 || m.ec2Browser.regionErrors[0].Region != "eu-west-1" {
		t.Fatalf("expected eu-west-1 region error to be stored, got %+v", m.ec2Browser.regionErrors)
	}

	view := m.ec2Browser.viewList(m)
	for _, want := range []string{"(all regions)", "[us-east-1]", "eu-west-1: access denied"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
		}
	}
}
