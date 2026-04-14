package app

import (
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
	for _, want := range []string{"╭", "╰", "NAME", "AUTH TYPE", "prod", "↑/↓: navigate • /: filter • enter: select • a: add • q: quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected context picker view to contain %q, got %q", want, view)
		}
	}
}
