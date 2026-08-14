package app

import (
	"strings"
	"testing"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func keymapTestModel() Model {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	return m
}

func TestKeymapRendersBarAndOverlayFromOneDefinition(t *testing.T) {
	m := keymapTestModel()
	m.screen = screenRoute53ZoneList

	bar := m.keymapHelpBar()
	for _, want := range []string{"↑/↓: navigate", "/: filter", "enter: records", "esc: back", "H: home"} {
		if !strings.Contains(bar, want) {
			t.Fatalf("expected bar to contain %q, got %q", want, bar)
		}
	}

	shortcuts, ok := m.keymapShortcuts()
	if !ok || len(shortcuts) != 4 {
		t.Fatalf("expected 4 overlay entries (H stays in the Global section), got %d", len(shortcuts))
	}
	if shortcuts[2].keys != "enter" || shortcuts[2].description != "Open the selected hosted zone" {
		t.Fatalf("expected overlay entry from the same definition, got %+v", shortcuts[2])
	}
}

func TestKeymapConditionalBindingsFollowModelState(t *testing.T) {
	m := keymapTestModel()
	m.screen = screenRoute53RecordDetail

	// NS records: no edit, no delete.
	m.route53.selectedRecord = &awsservice.DNSRecord{Type: "NS"}
	bar := m.keymapHelpBar()
	if strings.Contains(bar, "e: edit") || strings.Contains(bar, "d: delete") {
		t.Fatalf("expected protected record to hide edit/delete, got %q", bar)
	}

	// Plain A records: both appear, in bar and overlay alike.
	m.route53.selectedRecord = &awsservice.DNSRecord{Type: "A"}
	bar = m.keymapHelpBar()
	if !strings.Contains(bar, "e: edit") || !strings.Contains(bar, "d: delete") {
		t.Fatalf("expected editable record to offer edit/delete, got %q", bar)
	}
	shortcuts, _ := m.keymapShortcuts()
	joined := ""
	for _, shortcut := range shortcuts {
		joined += shortcut.keys + ";"
	}
	if !strings.Contains(joined, "e;") || !strings.Contains(joined, "d;") {
		t.Fatalf("expected overlay to match the bar's conditional bindings, got %q", joined)
	}
}

func TestKeymapOverlayEntriesDriveCurrentScreenShortcuts(t *testing.T) {
	m := keymapTestModel()
	m.screen = screenIAMKeyDetail
	m.iam.rotationEnabled = true
	m.iam.selectedKey = &awsservice.AccessKey{Status: "Active"}

	shortcuts := m.currentScreenShortcuts()
	found := false
	for _, shortcut := range shortcuts {
		if shortcut.keys == "r" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected the rotation binding to reach the overlay through the keymap")
	}
	// The bar previously omitted r: rotate while the overlay showed it — with
	// one definition both surfaces agree.
	if !strings.Contains(m.keymapHelpBar(), "r: rotate") {
		t.Fatal("expected the bar to agree with the overlay on the rotation binding")
	}
}

func TestLegacyScreensStillUseTheSwitch(t *testing.T) {
	m := keymapTestModel()
	m.screen = screenCWLogViewer

	if _, ok := m.keymapShortcuts(); ok {
		t.Fatal("expected the log viewer to stay on the legacy catalog (stateful bar labels)")
	}
	if len(m.currentScreenShortcuts()) == 0 {
		t.Fatal("expected legacy shortcuts to keep working")
	}
}
