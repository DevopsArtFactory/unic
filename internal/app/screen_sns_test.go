package app

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

	view, ok := m.sns.View(m)
	if !ok {
		t.Fatal("expected the SNS view to render")
	}
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
	for _, want := range []string{"SNS Subscriptions", "ops@example.com", "pending", "yes"} {
		if !strings.Contains(stripANSI(subs), want) {
			t.Fatalf("expected %q in subscription list, got:\n%s", want, stripANSI(subs))
		}
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
