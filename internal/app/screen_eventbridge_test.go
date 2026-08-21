package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func eventBridgeTestRules() []awsservice.EventBridgeRule {
	return []awsservice.EventBridgeRule{
		{
			Name: "nightly", EventBusName: "default", State: "DISABLED",
			ScheduleExpression: "cron(0 2 * * ? *)",
			Targets:            []awsservice.EventBridgeTarget{{ID: "worker", ARN: "arn:aws:lambda:us-east-1:1:function:worker"}},
			LastTriggeredAt:    time.Date(2026, 8, 20, 2, 0, 0, 0, time.Local),
		},
		{
			Name: "orders", EventBusName: "custom", State: "ENABLED",
			EventPattern:      `{"detail-type":["Order"]}`,
			LastTriggerStatus: "No activity in last 7 days",
		},
	}
}

func TestEventBridgeRulesLoadedOpensListAndRendersOperationalColumns(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width, m.height = 140, 30

	_, _, handled := m.eventBridge.HandleMessage(&m, eventBridgeRulesLoadedMsg{rules: eventBridgeTestRules()})
	if !handled || m.screen != screenEventBridgeRuleList {
		t.Fatalf("expected rule list, got screen %v", m.screen)
	}
	view, ok := m.eventBridge.View(m)
	for _, want := range []string{"EventBridge Rules", "nightly", "orders", "ENABLED", "event pattern", "none (7d)"} {
		if !ok || !strings.Contains(view, want) {
			t.Fatalf("expected rule list to contain %q, got:\n%s", want, view)
		}
	}
}

func TestEventBridgeRuleDetailShowsPatternTargetsAndActivitySource(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width, m.height = 140, 30
	m.eventBridge.HandleMessage(&m, eventBridgeRulesLoadedMsg{rules: eventBridgeTestRules()})
	m.eventBridge.idx = 1
	m.eventBridge.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenEventBridgeRuleDetail || m.eventBridge.selected == nil {
		t.Fatalf("expected selected rule detail, got screen=%v selected=%+v", m.screen, m.eventBridge.selected)
	}
	view, _ := m.eventBridge.View(m)
	for _, want := range []string{"orders", `"detail-type"`, `"Order"`, "No activity in last 7 days", "TriggeredRules"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected detail to contain %q, got:\n%s", want, view)
		}
	}
}

func TestEventBridgeStateChangeRequiresTypedRuleName(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.eventBridge.HandleMessage(&m, eventBridgeRulesLoadedMsg{rules: eventBridgeTestRules()})
	m.eventBridge.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m.eventBridge.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.screen != screenEventBridgeRuleConfirm || !m.eventBridge.desiredEnabled || !m.isTextEntryScreen() {
		t.Fatalf("expected enable confirmation text entry, screen=%v desired=%v", m.screen, m.eventBridge.desiredEnabled)
	}

	m.eventBridge.confirmInput = "wrong"
	_, cmd := m.eventBridge.updateConfirm(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || m.screen != screenEventBridgeRuleConfirm {
		t.Fatal("expected mismatched rule name to reject the action")
	}

	m.eventBridge.confirmInput = "nightly"
	updated, cmd := m.eventBridge.updateConfirm(&m, tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if cmd == nil || got.screen != screenLoading {
		t.Fatal("expected matching rule name to start the enable action")
	}
}

func TestEventBridgeManagedRuleCannotOpenStateConfirmation(t *testing.T) {
	m := New(testConfig(), "", "dev")
	rule := eventBridgeTestRules()[1]
	rule.ManagedBy = "events.amazonaws.com"
	m.eventBridge.HandleMessage(&m, eventBridgeRulesLoadedMsg{rules: []awsservice.EventBridgeRule{rule}})
	m.eventBridge.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m.eventBridge.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if m.screen != screenEventBridgeRuleDetail {
		t.Fatal("expected managed rule state action to be unavailable")
	}
	if strings.Contains(m.keymapHelpBar(), "disable") {
		t.Fatalf("expected managed rule mutation hidden from help, got %q", m.keymapHelpBar())
	}
}

func TestEventBridgeAllManagementEventsRuleIsReadOnly(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width, m.height = 120, 60
	rule := eventBridgeTestRules()[1]
	rule.State = "ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS"
	m.eventBridge.selected = &rule
	m.screen = screenEventBridgeRuleDetail

	m.eventBridge.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if m.screen != screenEventBridgeRuleDetail {
		t.Fatalf("expected all-management-events rule to remain read-only, got screen %v", m.screen)
	}
	if strings.Contains(m.keymapHelpBar(), "disable") {
		t.Fatalf("expected state mutation hidden from help, got %q", m.keymapHelpBar())
	}
	view, _ := m.eventBridge.View(m)
	if !strings.Contains(view, "read-only to preserve that matching mode") {
		t.Fatalf("expected read-only explanation, got:\n%s", view)
	}
}

func TestEventBridgeActionDoneUpdatesListAndDetailState(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.eventBridge.HandleMessage(&m, eventBridgeRulesLoadedMsg{rules: eventBridgeTestRules()})
	selected := m.eventBridge.rules[0]
	m.eventBridge.selected = &selected
	m.screen = screenLoading

	_, _, handled := m.eventBridge.HandleMessage(&m, eventBridgeActionDoneMsg{ruleName: "nightly", busName: "default", enabled: true})
	if !handled || m.screen != screenEventBridgeRuleDetail || m.eventBridge.selected.State != "ENABLED" || m.eventBridge.rules[1].State != "ENABLED" || m.eventBridge.idx != 1 {
		t.Fatalf("expected enabled state everywhere, screen=%v selected=%+v rules=%+v", m.screen, m.eventBridge.selected, m.eventBridge.rules)
	}
	if !strings.Contains(m.eventBridge.notice, "enabled") {
		t.Fatalf("expected success notice, got %q", m.eventBridge.notice)
	}
}

func TestEventBridgeActionDoneReappliesActiveFilter(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.storeFilterValue(filterEventBridgeRules, "disabled")
	m.eventBridge.HandleMessage(&m, eventBridgeRulesLoadedMsg{rules: eventBridgeTestRules()})
	selected := m.eventBridge.filtered[0]
	m.eventBridge.selected = &selected

	m.eventBridge.HandleMessage(&m, eventBridgeActionDoneMsg{ruleName: "nightly", busName: "default", enabled: true})

	if len(m.eventBridge.filtered) != 0 || m.eventBridge.idx != 0 {
		t.Fatalf("expected enabled rule removed from disabled filter, filtered=%+v idx=%d", m.eventBridge.filtered, m.eventBridge.idx)
	}
	if m.eventBridge.selected == nil || m.eventBridge.selected.State != "ENABLED" {
		t.Fatalf("expected detail selection to retain updated state, got %+v", m.eventBridge.selected)
	}
}

func TestEventBridgeContextSwitchDropsStaleActionState(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.eventBridge.rules = eventBridgeTestRules()
	selected := m.eventBridge.rules[0]
	m.eventBridge.selected = &selected
	m.ctxPrevScreen = screenEventBridgeRuleDetail
	m.screen = screenLoading
	nextConfig := testConfig()
	nextConfig.ContextName = "prod"

	updated, _ := m.Update(contextSwitchedMsg{cfg: nextConfig})
	got := updated.(Model)

	if got.screen != screenFeatureList {
		t.Fatalf("expected context switch to return to non-actionable feature list, got %v", got.screen)
	}
	if got.eventBridge.selected != nil || len(got.eventBridge.rules) != 0 {
		t.Fatalf("expected stale EventBridge action state cleared, got %+v", got.eventBridge)
	}
}

func TestEventBridgeDetailScrollIsBounded(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 12
	rule := eventBridgeTestRules()[0]
	for i := 0; i < 5; i++ {
		rule.Targets = append(rule.Targets, awsservice.EventBridgeTarget{ID: string(rune('a' + i)), ARN: "arn:target"})
	}
	m.eventBridge.selected = &rule
	m.screen = screenEventBridgeRuleDetail

	m.eventBridge.updateDetail(&m, tea.KeyMsg{Type: tea.KeyPgDown})
	if m.eventBridge.detailScroll == 0 {
		t.Fatal("expected page down to scroll long details")
	}
	for i := 0; i < 100; i++ {
		m.eventBridge.updateDetail(&m, tea.KeyMsg{Type: tea.KeyDown})
	}
	visibleLines := max(m.height-8, 5)
	maxOffset := max(len(m.eventBridge.detailLines(m))-visibleLines, 0)
	if m.eventBridge.detailScroll != maxOffset {
		t.Fatalf("expected scroll clamped to %d, got %d", maxOffset, m.eventBridge.detailScroll)
	}
}

func TestEventBridgeDetailKeepsLongEventPatternReachable(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width, m.height = 42, 12
	rule := eventBridgeTestRules()[1]
	rule.EventPattern = `{"source":["aws.ec2"],"detail":{"eventName":[{"prefix":"Describe"}],"tail":["tail-marker"]}}`
	m.eventBridge.selected = &rule
	m.screen = screenEventBridgeRuleDetail

	lines := m.eventBridge.detailLines(m)
	tailLine := -1
	for i, line := range lines {
		if strings.Contains(line, "tail-marker") {
			tailLine = i
			break
		}
	}
	if tailLine < 0 {
		t.Fatalf("expected complete event pattern in detail lines, got %#v", lines)
	}
	m.eventBridge.detailScroll = tailLine
	view, _ := m.eventBridge.View(m)
	if !strings.Contains(view, "tail-marker") {
		t.Fatalf("expected pattern tail reachable by vertical scrolling, got:\n%s", view)
	}
}

func TestEventBridgeLoadAndActionErrorsOpenErrorScreen(t *testing.T) {
	for _, msg := range []tea.Msg{
		eventBridgeRulesLoadedMsg{err: errors.New("list denied")},
		eventBridgeActionDoneMsg{err: errors.New("update denied")},
	} {
		m := New(testConfig(), "", "dev")
		updated, _, handled := m.eventBridge.HandleMessage(&m, msg)
		got := updated.(Model)
		if !handled || got.screen != screenError || !strings.Contains(got.errMsg, "denied") {
			t.Fatalf("expected error screen, got screen=%v message=%q", got.screen, got.errMsg)
		}
	}
}

func TestEventBridgeFilterAndHelpTitles(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.eventBridge.HandleMessage(&m, eventBridgeRulesLoadedMsg{rules: eventBridgeTestRules()})
	m.storeFilterValue(filterEventBridgeRules, "lambda:us-east-1")
	m.eventBridge.ApplyFilter(&m, filterEventBridgeRules)
	if len(m.eventBridge.filtered) != 1 || m.eventBridge.filtered[0].Name != "nightly" {
		t.Fatalf("expected target ARN filter match, got %+v", m.eventBridge.filtered)
	}
	for screen, want := range map[screen]string{
		screenEventBridgeRuleList:    "EventBridge Rules",
		screenEventBridgeRuleDetail:  "EventBridge Rule Detail",
		screenEventBridgeRuleConfirm: "EventBridge Rule Confirmation",
	} {
		m.screen = screen
		if got := m.helpScreenTitle(); got != want {
			t.Errorf("helpScreenTitle() = %q, want %q", got, want)
		}
	}
}
