package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func snsTestTopics() []awsservice.SNSTopic {
	return []awsservice.SNSTopic{
		{
			ARN: "arn:aws:sns:us-east-1:1:alpha-orders.fifo", Name: "alpha-orders.fifo", DisplayName: "Orders",
			Region: "us-east-1", KMSMasterKeyID: "alias/aws/sns", FIFO: true, ContentBasedDeduplication: true,
			SubscriptionsConfirmed: 3, SubscriptionsPending: 1, AttributesKnown: true,
		},
		{ARN: "arn:aws:sns:us-east-1:1:locked-topic", Name: "locked-topic", Region: "us-east-1"},
	}
}

func snsTestSubscriptions() []awsservice.SNSSubscription {
	return []awsservice.SNSSubscription{
		{ARN: "PendingConfirmation", Protocol: "email", Endpoint: "ops@example.com", Owner: "1"},
		{ARN: "arn:sub:confirmed", Protocol: "sqs", Endpoint: "arn:queue", Owner: "1",
			RedrivePolicy: `{"deadLetterTargetArn":"arn:dlq"}`, AttributesKnown: true},
		{ARN: "arn:sub:denied", Protocol: "lambda", Endpoint: "arn:fn", Owner: "1"},
	}
}

func snsLoadedModel(t *testing.T) Model {
	t.Helper()
	m := New(testConfig(), "", "dev")
	m.height, m.width = 24, 140
	m.screen = screenLoading
	_, _, handled := m.sns.HandleMessage(&m, snsTopicsLoadedMsg{topics: snsTestTopics()})
	if !handled || m.screen != screenSNSTopicList {
		t.Fatalf("expected topic list, screen=%v handled=%v", m.screen, handled)
	}
	return m
}

func TestSNSTopicListRendersFiltersAndShowsUnknownCounts(t *testing.T) {
	m := snsLoadedModel(t)

	rawView, ok := m.sns.View(m)
	if !ok {
		t.Fatal("expected the SNS view to render")
	}
	view := stripANSI(rawView)
	for _, want := range []string{"SNS Topics", "alpha-orders.fifo", "locked-topic", "FIFO", "3 (+1 pending)", "SUBSCRIPTIONS"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in topic list, got:\n%s", want, view)
		}
	}
	// The denied topic reports unknown rather than zero subscriptions.
	if !strings.Contains(view, "locked-topic") || strings.Contains(view, "locked-topic  standard   0 ") {
		t.Fatalf("expected unknown attributes rendered as a dash, got:\n%s", view)
	}

	m.storeFilterValue(filterSNSTopics, "orders")
	m.applyFilterTarget(filterSNSTopics)
	if len(m.sns.filteredTopics) != 1 || m.sns.filteredTopics[0].Name != "alpha-orders.fifo" {
		t.Fatalf("expected the filter to select one topic, got %+v", m.sns.filteredTopics)
	}
}

func TestSNSTopicWarningsSurfaceWithoutHidingTopics(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height, m.width = 24, 140
	m.screen = screenLoading
	m.sns.HandleMessage(&m, snsTopicsLoadedMsg{
		topics:   snsTestTopics(),
		warnings: []error{errors.New("failed to describe SNS topic arn:aws:sns:us-east-1:1:locked-topic: AuthorizationError")},
	})

	view, _ := m.sns.View(m)
	if !strings.Contains(view, "Warnings: 1 topic attribute lookup failures") {
		t.Fatalf("expected a warning summary, got:\n%s", view)
	}
	if !strings.Contains(view, "alpha-orders.fifo") || !strings.Contains(view, "locked-topic") {
		t.Fatalf("expected both topics to stay listed alongside the warning, got:\n%s", view)
	}
}

func TestSNSDrillDownToSubscriptionsAndBack(t *testing.T) {
	m := snsLoadedModel(t)

	updated, _ := m.sns.updateTopicList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.screen != screenSNSTopicDetail || m.sns.selectedTopic == nil {
		t.Fatalf("expected topic detail, screen=%v selected=%+v", m.screen, m.sns.selectedTopic)
	}
	detail, _ := m.sns.View(m)
	for _, want := range []string{"SNS Topic Detail", "Orders", "alias/aws/sns", "Content Dedup"} {
		if !strings.Contains(stripANSI(detail), want) {
			t.Fatalf("expected %q in topic detail, got:\n%s", want, stripANSI(detail))
		}
	}

	updated, cmd := m.sns.updateTopicDetail(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || m.screen != screenLoading {
		t.Fatalf("expected a subscription load, screen=%v cmd=%v", m.screen, cmd)
	}

	m.sns.HandleMessage(&m, snsSubscriptionsLoadedMsg{
		topicARN: "arn:aws:sns:us-east-1:1:alpha-orders.fifo", subscriptions: snsTestSubscriptions(),
	})
	if m.screen != screenSNSSubscriptionList {
		t.Fatalf("expected the subscription list, got %v", m.screen)
	}
	subs, _ := m.sns.View(m)
	for _, want := range []string{"SNS Subscriptions", "ops@example.com", "pending", "arn:dlq", "arn:fn"} {
		if !strings.Contains(stripANSI(subs), want) {
			t.Fatalf("expected %q in subscription list, got:\n%s", want, stripANSI(subs))
		}
	}
	unknownRow := snsSubscriptionRow(m, "lambda", "arn:fn", "1", "confirmed", "?")
	if !strings.Contains(stripANSI(subs), unknownRow) {
		t.Fatalf("expected denied subscription attributes to render an unknown DLQ marker, got:\n%s", stripANSI(subs))
	}

	updated, _ = m.sns.updateSubscriptionList(&m, tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenSNSTopicDetail {
		t.Fatalf("expected esc to return to topic detail, got %v", m.screen)
	}
	updated, _ = m.sns.updateTopicDetail(&m, tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenSNSTopicList {
		t.Fatalf("expected esc to return to the topic list, got %v", m.screen)
	}
}

func TestSNSStaleSubscriptionLoadIsDropped(t *testing.T) {
	m := snsLoadedModel(t)
	updated, _ := m.sns.updateTopicList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	m.screen = screenLoading

	// A subscription load for a topic the operator already left must not take
	// over the screen or install the wrong topic's subscriptions.
	m.sns.HandleMessage(&m, snsSubscriptionsLoadedMsg{
		topicARN: "arn:aws:sns:us-east-1:1:some-other-topic", subscriptions: snsTestSubscriptions(),
	})
	if m.screen != screenLoading || len(m.sns.subscriptions) != 0 {
		t.Fatalf("expected the stale load to be dropped, screen=%v subs=%+v", m.screen, m.sns.subscriptions)
	}
}

func TestSNSLoadErrorsShowErrorScreen(t *testing.T) {
	loadErr := errors.New("SNS unavailable")

	t.Run("topics", func(t *testing.T) {
		m := New(testConfig(), "", "dev")
		m.screen = screenLoading
		updated, _, handled := m.sns.HandleMessage(&m, snsTopicsLoadedMsg{err: loadErr})
		m = updated.(Model)
		if !handled || m.screen != screenError || m.errMsg != loadErr.Error() {
			t.Fatalf("expected topic error screen, screen=%v err=%q handled=%v", m.screen, m.errMsg, handled)
		}
	})

	t.Run("subscriptions", func(t *testing.T) {
		m := snsLoadedModel(t)
		m.sns.selectedTopic = &m.sns.topics[0]
		m.screen = screenLoading
		updated, _, handled := m.sns.HandleMessage(&m, snsSubscriptionsLoadedMsg{
			topicARN: m.sns.selectedTopic.ARN,
			err:      loadErr,
		})
		m = updated.(Model)
		if !handled || m.screen != screenError || m.errMsg != loadErr.Error() {
			t.Fatalf("expected subscription error screen, screen=%v err=%q handled=%v", m.screen, m.errMsg, handled)
		}
	})
}

func TestSNSLoadErrorsStayBehindGlobalOverlay(t *testing.T) {
	loadErr := errors.New("SNS unavailable")

	for _, tc := range []struct {
		name     string
		start    func(*Model) Model
		complete func(*Model)
	}{
		{
			name: "topics",
			start: func(m *Model) Model {
				updated, _ := m.sns.Start(m)
				return updated.(Model)
			},
			complete: func(m *Model) {
				m.sns.HandleMessage(m, snsTopicsLoadedMsg{err: loadErr})
			},
		},
		{
			name: "subscriptions",
			start: func(m *Model) Model {
				m.sns.topics = snsTestTopics()
				m.sns.selectedTopic = &m.sns.topics[0]
				updated, _ := m.startLoadingFor(screenSNSSubscriptionList, "Loading subscriptions...", nil, func() tea.Msg { return nil })
				return updated.(Model)
			},
			complete: func(m *Model) {
				m.sns.HandleMessage(m, snsSubscriptionsLoadedMsg{topicARN: m.sns.selectedTopic.ARN, err: loadErr})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(testConfig(), "", "dev")
			m = tc.start(&m)
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
			m = updated.(Model)

			tc.complete(&m)
			if m.screen != screenSettings || m.settingsPrevScreen != screenError || m.errMsg != loadErr.Error() {
				t.Fatalf("expected error behind Settings, screen=%v previous=%v err=%q", m.screen, m.settingsPrevScreen, m.errMsg)
			}

			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = updated.(Model)
			if m.screen != screenError {
				t.Fatalf("expected Settings to reveal the SNS error, got %v", m.screen)
			}
		})
	}
}

func TestSNSSubscriptionFilterAppliesToSubscriptionsOnly(t *testing.T) {
	m := snsLoadedModel(t)
	updated, _ := m.sns.updateTopicList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	m.sns.HandleMessage(&m, snsSubscriptionsLoadedMsg{
		topicARN: "arn:aws:sns:us-east-1:1:alpha-orders.fifo", subscriptions: snsTestSubscriptions(),
	})

	m.storeFilterValue(filterSNSSubscriptions, "email")
	m.applyFilterTarget(filterSNSSubscriptions)
	if len(m.sns.filteredSubs) != 1 || m.sns.filteredSubs[0].Protocol != "email" {
		t.Fatalf("expected the email subscription, got %+v", m.sns.filteredSubs)
	}
	if len(m.sns.filteredTopics) != len(snsTestTopics()) {
		t.Fatalf("expected the topic list to be untouched, got %+v", m.sns.filteredTopics)
	}
}

func TestSNSDetailScrollsWithinBounds(t *testing.T) {
	m := snsLoadedModel(t)
	m.height = 12
	updated, _ := m.sns.updateTopicList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	for range 20 {
		updated, _ = m.sns.updateTopicDetail(&m, tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	lines := m.sns.topicDetailLines(m)
	maxOffset := max(len(lines)-max(m.height-9, 5), 0)
	if m.sns.detailScroll != maxOffset {
		t.Fatalf("expected scroll clamped to %d, got %d", maxOffset, m.sns.detailScroll)
	}
	for range 30 {
		updated, _ = m.sns.updateTopicDetail(&m, tea.KeyMsg{Type: tea.KeyUp})
		m = updated.(Model)
	}
	if m.sns.detailScroll != 0 {
		t.Fatalf("expected scroll back to the top, got %d", m.sns.detailScroll)
	}
}

func TestSNSTopicPoliciesWrapWithoutTruncation(t *testing.T) {
	m := snsLoadedModel(t)
	topic := m.sns.topics[0]
	topic.DeliveryPolicy = `{"http":{"defaultHealthyRetryPolicy":{"numRetries":3,"backoffFunction":"exponential"}}}`
	topic.EffectiveDeliveryPolicy = `{"http":{"disableSubscriptionOverrides":true}}`
	m.sns.selectedTopic = &topic
	m.width = 72

	detail := stripANSI(strings.Join(m.sns.topicDetailLines(m), ""))
	for _, want := range []string{"Delivery Policy", "backoffFunction", "exponential", "Effective Policy", "disableSubscriptionOverrides"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("expected wrapped policy field %q to remain visible, got:\n%s", want, detail)
		}
	}
}

func TestSNSSubscriptionRowEscapesTerminalControls(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 140
	row := snsSubscriptionRow(m, "sq\ns", "arn:\x1b[31mqueue", "1\t2", "confirmed", "arn:dlq\r")
	for _, control := range []string{"\n", "\x1b", "\t", "\r"} {
		if strings.Contains(row, control) {
			t.Fatalf("expected subscription row to escape %q, got %q", control, row)
		}
	}
	for _, escaped := range []string{`sq\ns`, `arn:\x1b[31mqueue`, `1\t2`, `arn:dlq\r`} {
		if !strings.Contains(row, escaped) {
			t.Fatalf("expected escaped value %q, got %q", escaped, row)
		}
	}
}

func TestSNSListRowsKeepPostureIdentifiersVisible(t *testing.T) {
	const (
		kmsKeyARN = "arn:aws:kms:us-east-1:123456789012:key/550e8400-e29b-41d4-a716-446655440000"
		dlqARN    = "arn:aws:sqs:us-east-1:123456789012:prod-orders-dlq"
		endpoint  = "arn:aws:sqs:us-east-1:123456789012:endpointq42"
	)

	for _, width := range []int{80, 120} {
		t.Run(fmt.Sprintf("width-%d", width), func(t *testing.T) {
			m := snsLoadedModel(t)
			m.width = width
			m.sns.topics[0].KMSMasterKeyID = kmsKeyARN
			m.sns.filteredTopics[0].KMSMasterKeyID = kmsKeyARN

			topics := stripANSI(m.sns.viewTopicList(m))
			if !strings.Contains(topics, "446655440000") {
				t.Fatalf("expected KMS key suffix at width %d, got:\n%s", width, topics)
			}

			m.sns.selectedTopic = &m.sns.topics[0]
			m.sns.subscriptions = []awsservice.SNSSubscription{{
				ARN: "arn:aws:sns:us-east-1:123456789012:orders:subscription-id", Protocol: "sqs",
				Endpoint: endpoint, Owner: "123456789012",
				RedrivePolicy: `{"deadLetterTargetArn":"` + dlqARN + `"}`, AttributesKnown: true,
			}}
			m.sns.filteredSubs = m.sns.subscriptions

			subscriptions := stripANSI(m.sns.viewSubscriptionList(m))
			if !strings.Contains(subscriptions, "endpointq42") {
				t.Fatalf("expected endpoint resource suffix at width %d, got:\n%s", width, subscriptions)
			}
			if !strings.Contains(subscriptions, "prod-orders-dlq") {
				t.Fatalf("expected DLQ name at width %d, got:\n%s", width, subscriptions)
			}
		})
	}
}

func TestSNSListRowsAlignUnicodeColumns(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 140

	displayColumn := func(t *testing.T, row, marker string) int {
		t.Helper()
		idx := strings.Index(row, marker)
		if idx < 0 {
			t.Fatalf("expected marker %q in row %q", marker, row)
		}
		return lipgloss.Width(row[:idx])
	}
	assertColumns := func(t *testing.T, plain, unicode string, markers ...string) {
		t.Helper()
		for _, marker := range markers {
			if got, want := displayColumn(t, unicode, marker), displayColumn(t, plain, marker); got != want {
				t.Fatalf("marker %q shifted from column %d to %d: plain=%q unicode=%q", marker, want, got, plain, unicode)
			}
		}
	}

	plainTopic := snsTopicRow(m, "orders", "standard", "7 confirmed", "alias/orders")
	unicodeTopic := snsTopicRow(m, "東京😀", "standard", "7 confirmed", "alias/orders")
	assertColumns(t, plainTopic, unicodeTopic, "standard", "7 confirmed", "alias/orders")

	plainSubscription := snsSubscriptionRow(m, "email", "ops@example.com", "123456789012", "confirmed", "orders-dlq")
	unicodeSubscription := snsSubscriptionRow(m, "📨", "東京😀", "123456789012", "confirmed", "orders-dlq")
	assertColumns(t, plainSubscription, unicodeSubscription, "123456789012", "confirmed", "orders-dlq")
}

func TestSNSLoadCompletionStaysBehindGlobalOverlay(t *testing.T) {
	m := New(testConfig(), "", "dev")
	started, _ := m.sns.Start(&m)
	m = started.(Model)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	m = updated.(Model)
	if m.screen != screenSettings {
		t.Fatalf("expected the settings overlay, got %v", m.screen)
	}

	m.sns.HandleMessage(&m, snsTopicsLoadedMsg{topics: snsTestTopics()})
	if m.screen != screenSettings {
		t.Fatalf("expected the completed load to stay behind settings, got %v", m.screen)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenSNSTopicList {
		t.Fatalf("expected esc to reveal the loaded topic list, got %v", m.screen)
	}
}

func TestSNSActiveContextSelectionResumesPendingLoad(t *testing.T) {
	for _, tc := range []struct {
		name       string
		start      func(*Model) (tea.Model, tea.Cmd)
		complete   func(*Model)
		wantScreen screen
		wantCount  func(Model) int
	}{
		{
			name: "topics",
			start: func(m *Model) (tea.Model, tea.Cmd) {
				return m.startLoadingFor(screenSNSTopicList, "Loading SNS topics...", nil, func() tea.Msg { return nil })
			},
			complete: func(m *Model) {
				m.sns.HandleMessage(m, snsTopicsLoadedMsg{topics: snsTestTopics()})
			},
			wantScreen: screenSNSTopicList,
			wantCount:  func(m Model) int { return len(m.sns.topics) },
		},
		{
			name: "subscriptions",
			start: func(m *Model) (tea.Model, tea.Cmd) {
				m.sns.topics = snsTestTopics()
				m.sns.selectedTopic = &m.sns.topics[0]
				return m.startLoadingFor(screenSNSSubscriptionList, "Loading subscriptions...", nil, func() tea.Msg { return nil })
			},
			complete: func(m *Model) {
				m.sns.HandleMessage(m, snsSubscriptionsLoadedMsg{
					topicARN: m.sns.selectedTopic.ARN, subscriptions: snsTestSubscriptions(),
				})
			},
			wantScreen: screenSNSSubscriptionList,
			wantCount:  func(m Model) int { return len(m.sns.subscriptions) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.ContextName = "account-a"
			m := New(cfg, "", "dev")
			started, _ := tc.start(&m)
			m = started.(Model)
			generation := m.commands.CurrentGen()

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
			m = updated.(Model)
			updated, _ = m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{{Name: "account-a", Current: true}}})
			m = updated.(Model)

			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(Model)
			if cmd != nil || m.screen != screenLoading || m.commands.CurrentGen() != generation {
				t.Fatalf("expected active context to resume generation %d, screen=%v generation=%d command=%v", generation, m.screen, m.commands.CurrentGen(), cmd)
			}

			tc.complete(&m)
			if m.screen != tc.wantScreen || tc.wantCount(m) == 0 {
				t.Fatalf("expected pending load to complete on screen %v, got screen=%v state=%+v", tc.wantScreen, m.screen, m.sns)
			}
		})
	}
}

func TestSNSContextSwitchClearsStateAndNestedReturns(t *testing.T) {
	for _, tc := range []struct {
		name           string
		contextReturn  screen
		settingsReturn screen
		viewsReturn    screen
		wantScreen     screen
	}{
		{
			name: "subscription list", contextReturn: screenSNSSubscriptionList,
			wantScreen: screenFeatureList,
		},
		{
			name: "settings and views over topic detail", contextReturn: screenSettings,
			settingsReturn: screenViewList, viewsReturn: screenSNSTopicDetail,
			wantScreen: screenSettings,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := snsLoadedModel(t)
			m.screen = screenContextPicker
			m.ctxPrevScreen = tc.contextReturn
			m.settingsPrevScreen = tc.settingsReturn
			m.views.prevScreen = tc.viewsReturn
			m.sns.selectedTopic = &m.sns.topics[0]
			m.sns.subscriptions = snsTestSubscriptions()
			m.storeFilterValue(filterSNSTopics, "orders")
			m.storeFilterValue(filterSNSSubscriptions, "email")

			nextCfg := *m.cfg
			nextCfg.ContextName = "account-b"
			updated, _ := m.Update(contextSwitchedMsg{cfg: &nextCfg})
			m = updated.(Model)

			if m.screen != tc.wantScreen {
				t.Fatalf("expected safe return screen %v, got %v", tc.wantScreen, m.screen)
			}
			if len(m.sns.topics) != 0 || len(m.sns.subscriptions) != 0 || m.sns.selectedTopic != nil {
				t.Fatalf("expected SNS resource state cleared, got %+v", m.sns)
			}
			if m.filterValue(filterSNSTopics) != "" || m.filterValue(filterSNSSubscriptions) != "" {
				t.Fatal("expected SNS filters cleared")
			}
			if tc.viewsReturn != 0 && m.views.prevScreen != screenFeatureList {
				t.Fatalf("expected nested overlay chain normalized, got views return %v", m.views.prevScreen)
			}
		})
	}
}

func TestSNSRegionSwitchClearsState(t *testing.T) {
	m := snsLoadedModel(t)
	m.sns.selectedTopic = &m.sns.topics[0]
	m.sns.subscriptions = snsTestSubscriptions()
	m.storeFilterValue(filterSNSTopics, "orders")
	m.storeFilterValue(filterSNSSubscriptions, "email")

	updated, _ := m.Update(regionSwitchedMsg{region: "us-west-2"})
	m = updated.(Model)
	if m.screen != screenServiceList || len(m.sns.topics) != 0 || m.sns.selectedTopic != nil ||
		m.filterValue(filterSNSTopics) != "" || m.filterValue(filterSNSSubscriptions) != "" {
		t.Fatalf("expected region switch to clear SNS state, screen=%v state=%+v", m.screen, m.sns)
	}
}
