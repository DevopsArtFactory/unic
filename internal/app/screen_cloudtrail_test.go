package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func cloudTrailTestModel() Model {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	m.screen = screenLoading
	return m
}

func testCloudTrailEvents() []awsservice.CloudTrailEvent {
	return []awsservice.CloudTrailEvent{
		{ID: "evt-1", Name: "DeleteDBInstance", Username: "admin", Source: "rds.amazonaws.com",
			Region: "us-east-1", RawJSON: `{"eventName":"DeleteDBInstance"}`},
		{ID: "evt-2", Name: "DescribeInstances", Username: "reader", Source: "ec2.amazonaws.com", ReadOnly: true,
			RawJSON: `{"eventName":"DescribeInstances"}`},
	}
}

func TestCloudTrailEventsLoadedOpensList(t *testing.T) {
	m := cloudTrailTestModel()

	_, _, handled := m.cloudTrail.HandleMessage(&m, cloudTrailEventsLoadedMsg{events: testCloudTrailEvents()})
	if !handled || m.screen != screenCloudTrailEventList {
		t.Fatalf("expected event list screen, got %v handled=%v", m.screen, handled)
	}
	view, ok := m.cloudTrail.View(m)
	if !ok || !strings.Contains(view, "DeleteDBInstance") || !strings.Contains(view, "[3:24h]") {
		t.Fatalf("expected event list with default 24h window, got:\n%s", view)
	}
}

func TestCloudTrailWindowAndMutationKeysReload(t *testing.T) {
	m := cloudTrailTestModel()
	m.cloudTrail.HandleMessage(&m, cloudTrailEventsLoadedMsg{events: testCloudTrailEvents()})

	_, cmd := m.cloudTrail.updateList(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if m.cloudTrail.windowIdx != 0 || cmd == nil {
		t.Fatalf("expected 1h window reload, got idx=%d", m.cloudTrail.windowIdx)
	}

	_, cmd = m.cloudTrail.updateList(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if !m.cloudTrail.mutationsOnly || cmd == nil {
		t.Fatal("expected mutations-only toggle to reload")
	}
}

func TestCloudTrailResourceLookupInput(t *testing.T) {
	m := cloudTrailTestModel()
	m.cloudTrail.HandleMessage(&m, cloudTrailEventsLoadedMsg{events: testCloudTrailEvents()})
	m.screen = screenCloudTrailEventList

	m.cloudTrail.updateList(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if !m.cloudTrail.lookupInput || !m.isTextEntryScreen() {
		t.Fatal("expected lookup input mode to count as text entry")
	}
	for _, r := range "prod-db" {
		m.cloudTrail.updateList(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := m.cloudTrail.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.cloudTrail.resourceName != "prod-db" || cmd == nil {
		t.Fatalf("expected resource lookup reload, got %q", m.cloudTrail.resourceName)
	}
}

func TestCloudTrailDetailShowsEnvelopeAndRawEvent(t *testing.T) {
	m := cloudTrailTestModel()
	m.cloudTrail.HandleMessage(&m, cloudTrailEventsLoadedMsg{events: testCloudTrailEvents()})
	m.screen = screenCloudTrailEventList

	m.cloudTrail.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenCloudTrailEventDetail || m.cloudTrail.selected == nil {
		t.Fatalf("expected detail screen, got %v", m.screen)
	}
	view, _ := m.cloudTrail.View(m)
	for _, want := range []string{"DeleteDBInstance", "admin", "no (mutation)", "eventName"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected detail view to contain %q, got:\n%s", want, view)
		}
	}
}

func TestCloudTrailCancelAndExitTransitions(t *testing.T) {
	m := cloudTrailTestModel()
	m.cloudTrail.HandleMessage(&m, cloudTrailEventsLoadedMsg{events: testCloudTrailEvents()})
	m.screen = screenCloudTrailEventList

	// esc cancels the lookup input without leaving the list
	m.cloudTrail.updateList(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m.cloudTrail.updateList(&m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.cloudTrail.lookupInput || m.screen != screenCloudTrailEventList {
		t.Fatalf("expected esc to cancel lookup input, lookup=%v screen=%v", m.cloudTrail.lookupInput, m.screen)
	}

	// esc from detail returns to the list
	m.cloudTrail.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m.cloudTrail.updateDetail(&m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenCloudTrailEventList {
		t.Fatalf("expected detail esc to return to the list, got %v", m.screen)
	}

	// esc from the list exits to the feature list
	m.cloudTrail.updateList(&m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenFeatureList {
		t.Fatalf("expected list esc to exit to the feature list, got %v", m.screen)
	}
}
