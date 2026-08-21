package app

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func dynamoDBTestTable() awsservice.DynamoDBTable {
	return awsservice.DynamoDBTable{
		Name:          "orders",
		ARN:           "arn:aws:dynamodb:ap-northeast-2:123456789012:table/orders",
		Status:        "ACTIVE",
		BillingMode:   "PROVISIONED",
		ReadCapacity:  5,
		WriteCapacity: 2,
		ItemCount:     42,
		SizeBytes:     2048,
		GSICount:      1,
		Keys: []awsservice.DynamoDBKey{
			{Name: "tenant", Role: "PARTITION", AttributeType: "S"},
			{Name: "order", Role: "SORT", AttributeType: "N"},
		},
		TTLStatus:     "ENABLED",
		TTLAttribute:  "expires_at",
		StreamEnabled: true,
		StreamView:    "NEW_AND_OLD_IMAGES",
		StreamARN:     "arn:stream",
		GSIs: []awsservice.DynamoDBGSI{{
			Name: "by-status", Status: "ACTIVE", Projection: "ALL",
			ReadCapacity: 3, WriteCapacity: 1,
			Keys: []awsservice.DynamoDBKey{{Name: "status", Role: "PARTITION", AttributeType: "S"}},
		}},
	}
}

func TestDynamoDBTableListRendersSummaryColumns(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 140
	m.height = 30
	m.screen = screenDynamoDBTableList
	table := dynamoDBTestTable()
	m.dynamodb.tables = []awsservice.DynamoDBTable{table}
	m.dynamodb.filtered = m.dynamodb.tables

	view := m.dynamodb.viewTableList(m)
	for _, want := range []string{"DynamoDB Tables", "TABLE", "BILLING", "CAPACITY", "orders", "5R/2W", "42", "2.0 KB"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in table list:\n%s", want, view)
		}
	}
}

func TestDynamoDBTableListEnterLoadsSelectedDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenDynamoDBTableList
	m.dynamodb.filtered = []awsservice.DynamoDBTable{dynamoDBTestTable()}

	next, cmd := m.dynamodb.updateTableList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	model := next.(Model)
	if cmd == nil || model.screen != screenLoading || model.dynamodb.selected == nil || model.dynamodb.selected.Name != "orders" {
		t.Fatalf("expected selected table detail load, screen=%v selected=%+v cmd=%v", model.screen, model.dynamodb.selected, cmd)
	}
}

func TestDynamoDBDetailRendersKeysTTLStreamsAndGSI(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 120
	m.height = 40
	table := dynamoDBTestTable()
	m.dynamodb.HandleMessage(&m, dynamoDBTableDetailLoadedMsg{table: &table})

	view := m.dynamodb.viewTableDetail(m)
	for _, want := range []string{"DynamoDB — orders", "tenant (S)", "order (N)", "ENABLED on expires_at", "NEW_AND_OLD_IMAGES", "by-status", "ALL"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in detail:\n%s", want, view)
		}
	}
	if got := m.helpScreenTitle(); got != "DynamoDB Table Detail" {
		t.Fatalf("unexpected help title %q", got)
	}
}

func TestDynamoDBLookupPromptsForCompletePrimaryKey(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenDynamoDBTableDetail
	table := dynamoDBTestTable()
	m.dynamodb.selected = &table

	m.dynamodb.updateTableDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.screen != screenDynamoDBLookupInput || !m.isTextEntryScreen() {
		t.Fatalf("expected text-entry lookup screen, got %v", m.screen)
	}
	m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("acme")})
	m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.dynamodb.lookupField != 1 || m.dynamodb.lookupValues[0] != "acme" {
		t.Fatalf("expected partition key to advance to sort key, field=%d values=%v", m.dynamodb.lookupField, m.dynamodb.lookupValues)
	}
	m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("42")})
	next, cmd := m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyEnter})
	model := next.(Model)
	if cmd == nil || model.screen != screenLoading || model.dynamodb.lookupValues[1] != "42" {
		t.Fatalf("expected complete key to run GetItem, screen=%v values=%v cmd=%v", model.screen, model.dynamodb.lookupValues, cmd)
	}
}

func TestDynamoDBLookupResultSupportsScrollingAndNotFound(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 100
	m.height = 12
	m.screen = screenDynamoDBLookupResult
	table := dynamoDBTestTable()
	m.dynamodb.selected = &table
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("line-%02d", i))
	}
	m.dynamodb.item = &awsservice.DynamoDBItem{Found: true, JSON: strings.Join(lines, "\n")}

	m.dynamodb.updateLookupResult(&m, tea.KeyMsg{Type: tea.KeyDown})
	if m.dynamodb.itemScroll != 1 {
		t.Fatalf("expected item scroll to advance, got %d", m.dynamodb.itemScroll)
	}
	view := m.dynamodb.viewLookupResult(m)
	if !strings.Contains(view, "line-01") {
		t.Fatalf("expected scrolled JSON in result:\n%s", view)
	}

	m.dynamodb.item = &awsservice.DynamoDBItem{}
	m.dynamodb.itemScroll = 0
	if view := m.dynamodb.viewLookupResult(m); !strings.Contains(view, "No item found") {
		t.Fatalf("expected not-found result:\n%s", view)
	}
}

func TestDynamoDBLoadCommandsRequireSelectedTable(t *testing.T) {
	m := New(testConfig(), "", "dev")
	for name, cmd := range map[string]tea.Cmd{
		"detail": m.dynamodb.loadTableDetail(m),
		"item":   m.dynamodb.lookupItem(m),
	} {
		msg, ok := cmd().(errMsg)
		if !ok || msg.err == nil || !strings.Contains(msg.err.Error(), "no DynamoDB table selected") {
			t.Fatalf("%s: expected missing-selection error, got %#v", name, msg)
		}
	}
}
