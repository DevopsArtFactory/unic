package app

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func TestWAFListKeepsResultsWithWarningsAndFiltersAtCompactWidth(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width, m.height = 80, 15
	m.waf.HandleMessage(&m, wafWebACLsLoadedMsg{
		acls: []awsservice.WAFWebACL{
			{Name: "regional-open", Scope: "REGIONAL", DefaultAction: "ALLOW", DetailKnown: true, LoggingKnown: true, ResourcesComplete: true},
			{Name: "edge-locked", Scope: "CLOUDFRONT", DefaultAction: "BLOCK", DetailKnown: true, LoggingKnown: true, LogDestinations: []string{"arn:log"}},
		},
		warnings: []error{fmt.Errorf("regional association lookup denied")},
	})
	m.storeFilterValue(filterWAFWebACLs, "regional")
	m.waf.ApplyFilter(&m, filterWAFWebACLs)

	view := stripANSI(m.waf.viewList(m))
	if !strings.Contains(view, "Warnings: 1 scope or detail lookup failures") ||
		!strings.Contains(view, "regional-open") || !strings.Contains(view, "no-logs,allow-default,unassociated") ||
		strings.Contains(view, "edge-locked") {
		t.Fatalf("expected filtered partial WAF result and posture signals, got:\n%s", view)
	}
	for _, terminalWidth := range []int{80, 100, 128} {
		m.width = terminalWidth
		for _, line := range strings.Split(stripANSI(m.waf.viewList(m)), "\n") {
			if width := lipgloss.Width(line); width > terminalWidth {
				t.Fatalf("WAF view line is %d cells wide (limit %d): %q", width, terminalWidth, line)
			}
		}
	}
}

func TestWAFDetailScrollsThroughRulesAndResources(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width, m.height = 100, 10
	m.screen = screenWAFWebACLDetail
	m.waf.selected = &awsservice.WAFWebACL{
		Name: "app-acl", Scope: "REGIONAL", Region: "ap-northeast-2", ARN: "arn:acl", Description: "application protection",
		DefaultAction: "BLOCK", DetailKnown: true, Capacity: 200, LoggingKnown: true, LogDestinations: []string{"arn:log"},
		ResourcesComplete: true, ResourceARNs: []string{"arn:resource:one", "arn:resource:last"},
		Rules: []awsservice.WAFRule{
			{Name: "managed", Priority: 1, Statement: "managed AWS/common", Action: "GROUP ACTION", Managed: true, MetricName: "managed", MetricsEnabled: true},
			{Name: "last-rule", Priority: 50, Statement: "IP set", Action: "BLOCK", MetricName: "last", SamplingEnabled: true},
		},
	}

	initial := stripANSI(m.waf.viewDetail(m))
	if strings.Contains(initial, "last-rule") {
		t.Fatalf("expected later rule rows to be windowed initially:\n%s", initial)
	}
	_, _, handled := m.waf.HandleKey(&m, tea.KeyMsg{Type: tea.KeyPgDown})
	if !handled {
		t.Fatal("expected WAF detail page-down to be handled")
	}
	for range 3 {
		m.waf.HandleKey(&m, tea.KeyMsg{Type: tea.KeyPgDown})
	}
	scrolled := stripANSI(m.waf.viewDetail(m))
	if !strings.Contains(scrolled, "last-rule") || !strings.Contains(scrolled, "sampled=true") {
		t.Fatalf("expected scrolling to reveal later rule visibility details:\n%s", scrolled)
	}
}

func TestWAFDetailRendersRuleActionOverrides(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.waf.selected = &awsservice.WAFWebACL{DetailKnown: true, Rules: []awsservice.WAFRule{
		{Name: "managed", Statement: "managed AWS/common", Action: "GROUP ACTION", ActionOverrides: []string{"SizeRestrictions_BODY=CAPTCHA"}},
		{Name: "custom-group", Statement: "rule group custom", Action: "GROUP ACTION", ActionOverrides: []string{"CustomBlock=BLOCK"}},
	}}

	view := stripANSI(strings.Join(m.waf.detailLines(m), ""))
	if !strings.Contains(view, "Rule Override") || !strings.Contains(view, "SizeRestrictions_BODY=CAPTCHA") || !strings.Contains(view, "CustomBlock=BLOCK") {
		t.Fatalf("expected managed and custom rule-group overrides in WAF detail, got:\n%s", view)
	}
}

func TestWAFDetailTruncatesLongTitleAtTerminalWidth(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width, m.height = 80, 15
	m.screen = screenWAFWebACLDetail
	m.waf.selected = &awsservice.WAFWebACL{
		Name: strings.Repeat("customer-facing-edge-acl-", 6), Scope: "CLOUDFRONT", Region: "us-east-1",
		DetailKnown: true, LoggingKnown: true, ResourcesComplete: true,
	}

	view := m.waf.viewDetail(m)
	for _, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("WAF detail line is %d cells wide (limit %d): %q", width, m.width, line)
		}
	}
	if !strings.Contains(stripANSI(view), "...") {
		t.Fatalf("expected overlong title to be truncated, got:\n%s", stripANSI(view))
	}
}

func TestWAFLoadCompletingBehindSettingsPreservesOverlay(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading
	m.loadingReturnScreen = screenWAFWebACLList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	m = updated.(Model)
	if m.screen != screenSettings || m.settingsPrevScreen != screenLoading {
		t.Fatalf("expected settings above pending WAF load, screen=%v previous=%v", m.screen, m.settingsPrevScreen)
	}
	m.waf.HandleMessage(&m, wafWebACLsLoadedMsg{acls: []awsservice.WAFWebACL{{Name: "loaded"}}})
	if m.screen != screenSettings || m.settingsPrevScreen != screenWAFWebACLList {
		t.Fatalf("completed WAF load should stay behind settings, screen=%v previous=%v", m.screen, m.settingsPrevScreen)
	}
}

func TestWAFFailureBehindSettingsPreservesOverlay(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenSettings
	m.settingsPrevScreen = screenLoading
	m.loadingReturnScreen = screenWAFWebACLList

	m.waf.HandleMessage(&m, wafWebACLsLoadedMsg{err: fmt.Errorf("WAF denied")})
	if m.screen != screenSettings || m.settingsPrevScreen != screenError || m.errMsg != "WAF denied" {
		t.Fatalf("failed WAF load should stay behind settings, screen=%v previous=%v error=%q", m.screen, m.settingsPrevScreen, m.errMsg)
	}
}

func TestWAFFailureBehindContextPickerIsNotRestoredAfterContextSwitch(t *testing.T) {
	cfg := testConfig()
	cfg.ContextName = "old"
	m := New(cfg, "", "dev")
	started, _ := m.waf.Start(&m)
	m = started.(Model)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = updated.(Model)
	updated, _ = m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{{Name: "new"}}})
	m = updated.(Model)
	updated, _ = m.Update(wafWebACLsLoadedMsg{err: fmt.Errorf("old-context WAF denied")})
	m = updated.(Model)
	if m.screen != screenContextPicker || m.ctxPrevScreen != screenError {
		t.Fatalf("expected failed old-context load behind picker, screen=%v previous=%v", m.screen, m.ctxPrevScreen)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || m.screen != screenLoading {
		t.Fatalf("expected context switch load, screen=%v command=%v", m.screen, cmd)
	}
	newCfg := testConfig()
	newCfg.ContextName = "new"
	updated, _ = m.Update(contextSwitchedMsg{cfg: newCfg})
	m = updated.(Model)
	if m.screen != screenServiceList {
		t.Fatalf("expected new context to discard old WAF error, got screen=%v error=%q", m.screen, m.errMsg)
	}
}

func TestWAFFailureBehindContextPickerIsShownWhenPickerIsCancelled(t *testing.T) {
	cfg := testConfig()
	cfg.ContextName = "old"
	m := New(cfg, "", "dev")
	m.screen = screenContextPicker
	m.ctxPrevScreen = screenLoading
	m.loadingReturnScreen = screenWAFWebACLList

	m.waf.HandleMessage(&m, wafWebACLsLoadedMsg{err: fmt.Errorf("WAF denied")})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenError || m.errMsg != "WAF denied" {
		t.Fatalf("expected cancelled picker to show WAF failure, screen=%v error=%q", m.screen, m.errMsg)
	}
}

func TestWAFContextChangeClearsResourcesAndReturnTarget(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg.ContextName = "old"
	m.waf.acls = []awsservice.WAFWebACL{{Name: "stale"}}
	m.ctxPrevScreen = screenWAFWebACLDetail
	newCfg := &config.Config{ContextName: "new", Region: "us-west-2"}

	updated, _, handled := m.handleContextMsg(contextSwitchedMsg{cfg: newCfg})
	if !handled {
		t.Fatal("expected context switch message to be handled")
	}
	after := updated.(Model)
	if len(after.waf.acls) != 0 || after.ctxPrevScreen != screenServiceList || after.screen != screenServiceList {
		t.Fatalf("expected WAF state and return target cleared on context change, got screen=%v previous=%v acls=%v", after.screen, after.ctxPrevScreen, after.waf.acls)
	}
}

func TestPendingWAFContextSetupNormalizesCancelledLoad(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenContextPicker
	m.ctxPrevScreen = screenLoading
	m.loadingReturnScreen = screenWAFWebACLList

	if !pendingWAFContextReturn(&m) {
		t.Fatal("expected pending WAF context return")
	}
	normalizePendingWAFContextReturn(&m)
	if m.ctxPrevScreen != screenServiceList {
		t.Fatalf("expected cancelled WAF load to return safely to services, got %v", m.ctxPrevScreen)
	}
}
