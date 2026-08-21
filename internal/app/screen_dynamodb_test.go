package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

type appDynamoDBClient struct {
	getItemFunc func(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

func (*appDynamoDBClient) ListTables(context.Context, *dynamodb.ListTablesInput, ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	return &dynamodb.ListTablesOutput{}, nil
}

func (*appDynamoDBClient) DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{}, nil
}

func (*appDynamoDBClient) DescribeTimeToLive(context.Context, *dynamodb.DescribeTimeToLiveInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTimeToLiveOutput, error) {
	return &dynamodb.DescribeTimeToLiveOutput{}, nil
}

func (c *appDynamoDBClient) GetItem(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return c.getItemFunc(ctx, input, opts...)
}

func runDynamoDBLookupBatch(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected lookup command batch")
	}
	for _, batched := range batch {
		msg := batched()
		switch msg.(type) {
		case dynamoDBItemLoadedMsg, errMsg:
			return msg
		}
	}
	t.Fatal("lookup batch did not return a result")
	return nil
}

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
	m.commands = nil
	m.awsRepo = &awsservice.AwsRepository{DynamoDBClient: &appDynamoDBClient{getItemFunc: func(_ context.Context, input *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		partition, partitionOK := input.Key["tenant"].(*dynamodbtypes.AttributeValueMemberS)
		sortKey, sortOK := input.Key["order"].(*dynamodbtypes.AttributeValueMemberN)
		if !partitionOK || partition.Value != "acme" || !sortOK || sortKey.Value != "42" {
			t.Fatalf("unexpected typed key: %#v", input.Key)
		}
		return &dynamodb.GetItemOutput{Item: map[string]dynamodbtypes.AttributeValue{
			"tenant": &dynamodbtypes.AttributeValueMemberS{Value: "acme"},
		}}, nil
	}}}
	next, cmd := m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyEnter})
	model := next.(Model)
	if cmd == nil || model.screen != screenLoading || model.dynamodb.lookupValues[1] != "42" {
		t.Fatalf("expected complete key to run GetItem, screen=%v values=%v cmd=%v", model.screen, model.dynamodb.lookupValues, cmd)
	}
	result, ok := runDynamoDBLookupBatch(t, cmd).(dynamoDBItemLoadedMsg)
	if !ok || result.item == nil || !result.item.Found || !strings.Contains(result.item.JSON, `"tenant": "acme"`) {
		t.Fatalf("expected loaded item result, got %#v", result)
	}
}

func TestDynamoDBLookupCommandReturnsAPIErrMsg(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.commands = nil
	table := dynamoDBTestTable()
	m.dynamodb.selected = &table
	m.dynamodb.lookupValues = []string{"acme", "42"}
	m.dynamodb.lookupField = 1
	m.dynamodb.lookupInput = "42"
	m.awsRepo = &awsservice.AwsRepository{DynamoDBClient: &appDynamoDBClient{getItemFunc: func(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		return nil, errors.New("access denied")
	}}}

	_, cmd := m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyEnter})
	msg, ok := runDynamoDBLookupBatch(t, cmd).(errMsg)
	if !ok || msg.err == nil || !strings.Contains(msg.err.Error(), "failed to get item") {
		t.Fatalf("expected wrapped API error message, got %#v", msg)
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

func TestDynamoDBLookupResultWrapsLongValuesIntoScrollableLines(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 32
	m.height = 12
	m.screen = screenDynamoDBLookupResult
	m.dynamodb.item = &awsservice.DynamoDBItem{
		Found: true,
		JSON:  fmt.Sprintf("{\n  \"payload\": \"%sTAIL\"\n}", strings.Repeat("x", 240)),
	}

	lines := m.dynamodb.itemLines(m)
	if len(lines) <= max(m.height-8, 5) {
		t.Fatalf("expected long JSON to wrap beyond one page, got %d lines", len(lines))
	}
	for range len(lines) {
		m.dynamodb.updateLookupResult(&m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if view := m.dynamodb.viewLookupResult(m); !strings.Contains(view, "TAIL") {
		t.Fatalf("expected wrapped suffix to be reachable after scrolling:\n%s", view)
	}
}

func TestDynamoDBContextSwitchClearsPreviousAccountState(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 100
	m.height = 20
	m.screen = screenLoading
	m.ctxPrevScreen = screenDynamoDBLookupResult
	table := dynamoDBTestTable()
	m.dynamodb.tables = []awsservice.DynamoDBTable{table}
	m.dynamodb.filtered = m.dynamodb.tables
	m.dynamodb.selected = &table
	m.dynamodb.item = &awsservice.DynamoDBItem{Found: true, JSON: `{"secret":"old-account"}`}
	nextCfg := *m.cfg
	nextCfg.ContextName = "next-account"

	updated, _ := m.Update(contextSwitchedMsg{cfg: &nextCfg})
	model := updated.(Model)
	if model.screen != screenServiceList {
		t.Fatalf("expected context switch to leave DynamoDB screens, got %v", model.screen)
	}
	if len(model.dynamodb.tables) != 0 || len(model.dynamodb.filtered) != 0 || model.dynamodb.selected != nil || model.dynamodb.item != nil {
		t.Fatalf("expected context-scoped DynamoDB state to be cleared, got %+v", model.dynamodb)
	}
	if view := model.View(); strings.Contains(view, "old-account") {
		t.Fatalf("previous account item leaked after context switch:\n%s", view)
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

func TestDynamoDBBeginLookupWithoutSelectionIsNoop(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenDynamoDBLookupResult
	m.dynamodb.beginLookup(&m)
	if m.screen != screenDynamoDBLookupResult {
		t.Fatalf("expected screen to remain unchanged, got %v", m.screen)
	}
}
