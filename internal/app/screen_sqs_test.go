package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func sqsTestModel() Model {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	m.screen = screenLoading
	return m
}

func testQueues() []awsservice.SQSQueue {
	return []awsservice.SQSQueue{
		{Name: "orders-dlq", ARN: "arn:aws:sqs:us-east-1:1:orders-dlq", Depth: 120, SourceQueueCount: 1, Region: "us-east-1"},
		{Name: "orders", ARN: "arn:aws:sqs:us-east-1:1:orders", Depth: 5,
			DLQTargetARN: "arn:aws:sqs:us-east-1:1:orders-dlq", MaxReceiveCount: 3, Region: "us-east-1"},
	}
}

func TestSQSQueuesLoadedOpensList(t *testing.T) {
	m := sqsTestModel()

	_, _, handled := m.sqs.HandleMessage(&m, sqsQueuesLoadedMsg{queues: testQueues()})
	if !handled || m.screen != screenSQSQueueList {
		t.Fatalf("expected queue list screen, got %v", m.screen)
	}
	view, ok := m.sqs.View(m)
	if !ok || !strings.Contains(view, "orders-dlq") || !strings.Contains(view, "! = DLQ") {
		t.Fatalf("expected queue list with DLQ legend, got:\n%s", view)
	}
}

func TestSQSDLQJumpFromSourceQueue(t *testing.T) {
	m := sqsTestModel()
	m.sqs.HandleMessage(&m, sqsQueuesLoadedMsg{queues: testQueues()})
	source := testQueues()[1]
	m.sqs.selected = &source
	m.screen = screenSQSQueueDetail

	m.sqs.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if m.sqs.selected == nil || m.sqs.selected.Name != "orders-dlq" {
		t.Fatalf("expected d to jump to the DLQ, got %+v", m.sqs.selected)
	}
}

func TestSQSPurgeRequiresTypedQueueName(t *testing.T) {
	m := sqsTestModel()
	m.sqs.HandleMessage(&m, sqsQueuesLoadedMsg{queues: testQueues()})
	queue := testQueues()[0]
	m.sqs.selected = &queue
	m.screen = screenSQSQueueDetail

	m.sqs.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.screen != screenSQSConfirm || m.sqs.action != "purge" {
		t.Fatalf("expected purge confirm screen, got %v action=%q", m.screen, m.sqs.action)
	}
	if !m.isTextEntryScreen() {
		t.Fatal("expected the confirm input to count as text entry")
	}

	// Wrong name: no action.
	m.sqs.confirmInput = "wrong"
	_, cmd := m.sqs.updateConfirm(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || m.screen != screenSQSConfirm {
		t.Fatal("expected mismatched name to be rejected")
	}

	m.sqs.confirmInput = "orders-dlq"
	_, cmd = m.sqs.updateConfirm(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected matching name to run the purge")
	}
}

func TestSQSRedriveOnlyOfferedForDLQs(t *testing.T) {
	m := sqsTestModel()
	m.sqs.HandleMessage(&m, sqsQueuesLoadedMsg{queues: testQueues()})

	// A non-DLQ queue: m must be a no-op.
	source := testQueues()[1]
	m.sqs.selected = &source
	m.screen = screenSQSQueueDetail
	m.sqs.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if m.screen != screenSQSQueueDetail {
		t.Fatal("expected redrive to be a no-op on a non-DLQ queue")
	}
	if strings.Contains(m.keymapHelpBar(), "m: redrive") {
		t.Fatal("expected redrive hidden for non-DLQ queues")
	}

	// The DLQ: redrive opens the confirm screen and shows in help.
	dlq := testQueues()[0]
	m.sqs.selected = &dlq
	if !strings.Contains(m.keymapHelpBar(), "m: redrive") {
		t.Fatal("expected redrive offered for DLQs")
	}
	m.sqs.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if m.screen != screenSQSConfirm || m.sqs.action != "redrive" {
		t.Fatalf("expected redrive confirm, got %v action=%q", m.screen, m.sqs.action)
	}
}
