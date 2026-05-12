package app

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func styleTestContexts() []config.ContextInfo {
	return []config.ContextInfo{
		{Name: "dev", Region: "us-east-1", AuthType: "credential"},
		{Name: "prod", Region: "us-west-2", AuthType: "sso", Current: true},
		{Name: "staging", Region: "ap-northeast-2", AuthType: "assume_role"},
	}
}

func TestRenderStatusBarUsesFullWidthAndUpdateHint(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 120
	m.cfg.ContextName = "prod"
	m.cfg.AuthType = config.AuthTypeCredential
	m.callerIdentity = &awsservice.CallerIdentity{Account: "123456789012"}
	m.updateAvailable = "v1.2.3"

	bar := strings.TrimRight(m.renderStatusBar(), "\n")
	if got := lipgloss.Width(bar); got != 120 {
		t.Fatalf("expected status bar width 120, got %d (%q)", got, stripANSI(bar))
	}

	plain := stripANSI(bar)
	for _, want := range []string{"[prod]", "region:us-east-1", "auth:credential", "account:123456789012", "v1.2.3 available"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected status bar to contain %q, got %q", want, plain)
		}
	}
}

func TestInspectorModeStatusBarAddsModeMarkerAndDifferentChrome(t *testing.T) {
	regular := New(testConfig(), "", "dev")
	regular.width = 80
	regular.cfg.ContextName = "prod"
	regular.screen = screenServiceList

	inspectorMode := New(testConfig(), "", "dev")
	inspectorMode.width = 80
	inspectorMode.cfg.ContextName = "prod"
	inspectorMode.screen = screenInspectorHome

	regularBar := regular.renderStatusBar()
	inspectorBar := inspectorMode.renderStatusBar()
	if regularBar == inspectorBar {
		t.Fatal("expected inspector mode status bar styling to differ from the regular service flow")
	}
	if !strings.Contains(stripANSI(inspectorBar), "mode:inspector") {
		t.Fatalf("expected inspector status bar to include mode marker, got %q", stripANSI(inspectorBar))
	}
}

func TestInspectorModeHelpBarUsesDifferentStyling(t *testing.T) {
	regular := New(testConfig(), "", "dev")
	regular.width = 48
	regular.screen = screenServiceList

	inspectorMode := New(testConfig(), "", "dev")
	inspectorMode.width = 48
	inspectorMode.screen = screenInspectorHome

	regularBar := regular.renderHelpBar("esc: back • H: home")
	inspectorBar := inspectorMode.renderHelpBar("esc: back • H: home")
	if reflect.DeepEqual(regular.currentHelpStyle(), inspectorMode.currentHelpStyle()) {
		t.Fatal("expected inspector help bar styling to differ from the regular service flow")
	}
	if stripANSI(regularBar) != stripANSI(inspectorBar) {
		t.Fatalf("expected inspector styling to keep the visible help text unchanged, got regular=%q inspector=%q", stripANSI(regularBar), stripANSI(inspectorBar))
	}
}

func TestRenderHelpBarUsesFullWidth(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 48

	bar := m.renderHelpBar("esc: back • H: home")
	if got := lipgloss.Width(bar); got != 48 {
		t.Fatalf("expected help bar width 48, got %d (%q)", got, stripANSI(bar))
	}
	if !strings.Contains(stripANSI(bar), "esc: back • H: home") {
		t.Fatalf("expected help bar text, got %q", stripANSI(bar))
	}
}

func TestRenderListPanelUsesRoundedBorder(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80

	panel := stripANSI(m.renderListPanel("row 1\nrow 2"))
	lines := strings.Split(panel, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected bordered panel output, got %q", panel)
	}
	if !strings.HasPrefix(lines[0], "╭") || !strings.HasPrefix(lines[len(lines)-1], "╰") {
		t.Fatalf("expected rounded border, got %q", panel)
	}
	if !strings.Contains(panel, "row 1") || !strings.Contains(panel, "row 2") {
		t.Fatalf("expected panel body content, got %q", panel)
	}
	if !strings.Contains(panel, "│ row 1") {
		t.Fatalf("expected list panel to add inner padding, got %q", panel)
	}
}

func TestRenderDetailLineUsesStandardWidth(t *testing.T) {
	got := stripANSI(renderDetailLine("Name", "value"))
	if got != "  Name          value" {
		t.Fatalf("expected standardized detail line spacing, got %q", got)
	}
}

func TestContextPickerUsesBorderedPanelAndHelpBar(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20
	updated, _ := m.Update(contextsLoadedMsg{contexts: styleTestContexts()})
	m = updated.(Model)

	view := stripANSI(m.viewContextPicker())
	for _, want := range []string{"╭", "╰", "NAME", "AUTH TYPE", "prod", "↑/↓: navigate • type: filter • /: filter • enter: switch • s: setup • y: copy"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected context picker view to contain %q, got %q", want, view)
		}
	}
}

func TestContextPickerKeepsCurrentColumnVisible(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 60
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: styleTestContexts()})
	m = updated.(Model)

	view := stripANSI(m.viewContextPicker())
	for _, want := range []string{"AUTH TYPE", "CURRENT", "*"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected context picker to keep %q visible at width 60, got %q", want, view)
		}
	}
}

func TestContextPickerPanelDoesNotOverflowHelpBar(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20

	updated, _ := m.Update(contextsLoadedMsg{contexts: styleTestContexts()})
	m = updated.(Model)
	view := stripANSI(m.View())
	lines := strings.Split(view, "\n")

	if len(lines) != 20 {
		t.Fatalf("expected fitted view height 20, got %d lines", len(lines))
	}

	borderLine := -1
	helpLine := -1
	for i, line := range lines {
		if strings.Contains(line, "╰") {
			borderLine = i
		}
		if strings.Contains(line, "q: quit") {
			helpLine = i
		}
	}

	if borderLine == -1 || helpLine == -1 {
		t.Fatalf("expected both bottom border and help bar, got %q", view)
	}
	if helpLine <= borderLine {
		t.Fatalf("expected help bar below panel border, got border line %d help line %d in %q", borderLine, helpLine, view)
	}
}

func TestContextPickerKeepsSelectionVisibleInCompactTerminal(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 14

	updated, _ := m.Update(contextsLoadedMsg{contexts: styleTestContexts()})
	m = updated.(Model)
	view := stripANSI(m.View())

	for _, want := range []string{"Select Context", "prod", "q: quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected compact context picker to keep %q visible, got %q", want, view)
		}
	}
	if strings.Contains(view, "...") {
		t.Fatalf("expected compact context picker to fit without middle truncation, got %q", view)
	}
}

func TestContextTableSelectedStyleUsesHighContrast(t *testing.T) {
	want := selectedStyle.Copy().
		Bold(true).
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("57"))
	if !reflect.DeepEqual(contextTableSelectedStyle(), want) {
		t.Fatal("expected context table selected style to use high-contrast foreground/background")
	}
}
