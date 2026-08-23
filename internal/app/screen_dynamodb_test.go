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
	"github.com/charmbracelet/lipgloss"

	"unic/internal/config"
	"unic/internal/domain"
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

func TestDynamoDBTableListKeepsLongRowsWithinNarrowPanel(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 64
	m.height = 30
	m.screen = screenDynamoDBTableList
	table := dynamoDBTestTable()
	table.Name = "customer-orders-with-an-extraordinarily-long-table-name"
	table.Status = "INACCESSIBLE_ENCRYPTION_CREDENTIALS"
	table.GSICount = 987
	m.dynamodb.tables = []awsservice.DynamoDBTable{table}
	m.dynamodb.filtered = m.dynamodb.tables

	plain := ansiEscapePattern.ReplaceAllString(m.dynamodb.viewTableList(m), "")
	var selected string
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "> ") {
			selected = line
			break
		}
	}
	if selected == "" {
		t.Fatalf("expected a selected table row:\n%s", plain)
	}
	for _, want := range []string{"5R/2W", "42", "2.0 KB", "987"} {
		if !strings.Contains(selected, want) {
			t.Fatalf("expected summary %q on the selected row %q", want, selected)
		}
	}
	if !strings.Contains(selected, "...") || strings.Contains(selected, table.Name) || strings.Contains(selected, table.Status) {
		t.Fatalf("expected long cells to be truncated on one row, got %q", selected)
	}
	if width := lipgloss.Width(selected); width > m.width {
		t.Fatalf("selected row width %d exceeds terminal width %d: %q", width, m.width, selected)
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

func TestDynamoDBDetailAndLookupEscapeTerminalControlsInMetadata(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 120
	m.height = 40
	table := dynamoDBTestTable()
	table.Keys[0].Name = "tenant\n\x1b[31m\a"
	table.TTLAttribute = "expires\x1b]52;c;payload\a"
	m.dynamodb.selected = &table
	m.screen = screenDynamoDBTableDetail

	detail := m.dynamodb.viewTableDetail(m)
	for _, raw := range []string{table.Keys[0].Name, table.TTLAttribute, "\x1b]52;c;payload", "\a"} {
		if strings.Contains(detail, raw) {
			t.Fatalf("detail metadata must not contain active terminal controls %q: %q", raw, detail)
		}
	}
	for _, escaped := range []string{`tenant\n\x1b[31m\a`, `expires\x1b]52;c;payload\a`} {
		if !strings.Contains(detail, escaped) {
			t.Fatalf("expected visibly escaped metadata %q: %q", escaped, detail)
		}
	}

	m.dynamodb.beginLookup(&m)
	lookup := m.dynamodb.viewLookupInput(m)
	if strings.Contains(lookup, table.Keys[0].Name) || strings.Contains(lookup, "\a") {
		t.Fatalf("lookup metadata must not contain active terminal controls: %q", lookup)
	}
	if !strings.Contains(lookup, `tenant\n\x1b[31m\a`) {
		t.Fatalf("expected visibly escaped lookup key metadata: %q", lookup)
	}
}

func TestDynamoDBDetailWrapsLongValuesIntoScrollableLines(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 32
	m.height = 12
	m.screen = screenDynamoDBTableDetail
	table := dynamoDBTestTable()
	table.ARN = "arn:aws:dynamodb:" + strings.Repeat("x", 240) + "TAIL"
	m.dynamodb.selected = &table

	lines := m.dynamodb.tableDetailLines(m)
	if len(lines) <= max(m.height-8, 5) {
		t.Fatalf("expected long detail values to wrap beyond one page, got %d lines", len(lines))
	}
	found := false
	for range len(lines) {
		if strings.Contains(m.dynamodb.viewTableDetail(m), "TAIL") {
			found = true
			break
		}
		m.dynamodb.updateTableDetail(&m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if !found {
		t.Fatal("expected wrapped detail suffix to be reachable by scrolling")
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

func TestDynamoDBLookupEscapesPastedValuesWithoutChangingRequest(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 48
	m.screen = screenDynamoDBTableDetail
	table := dynamoDBTestTable()
	table.Keys[1].AttributeType = "S"
	m.dynamodb.selected = &table
	m.dynamodb.beginLookup(&m)

	partition := "acme\n\x1b]52;c;payload\a" + strings.Repeat("x", 64)
	sortKey := "order\n\x1b]52;c;payload\a" + strings.Repeat("y", 64)
	assertSafeView := func(view string) {
		t.Helper()
		if strings.Contains(view, "\x1b]52;c;payload") || strings.ContainsRune(view, '\a') {
			t.Fatalf("lookup view contains pasted terminal controls: %q", view)
		}
		plain := ansiEscapePattern.ReplaceAllString(view, "")
		for _, line := range strings.Split(plain, "\n") {
			if strings.Contains(line, `\n`) && lipgloss.Width(line) > m.width {
				t.Fatalf("lookup value line width %d exceeds terminal width %d: %q", lipgloss.Width(line), m.width, line)
			}
		}
	}

	m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(partition), Paste: true})
	if m.dynamodb.lookupInput != partition {
		t.Fatalf("expected raw partition key to remain unchanged, got %q", m.dynamodb.lookupInput)
	}
	assertSafeView(m.dynamodb.viewLookupInput(m))
	m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(sortKey), Paste: true})
	view := m.dynamodb.viewLookupInput(m)
	assertSafeView(view)
	if !strings.Contains(view, `tenant = acme\n`) || !strings.Contains(view, `order\n`) {
		t.Fatalf("expected visibly escaped pasted values: %q", view)
	}

	m.commands = nil
	m.awsRepo = &awsservice.AwsRepository{DynamoDBClient: &appDynamoDBClient{getItemFunc: func(_ context.Context, input *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		partitionValue, partitionOK := input.Key["tenant"].(*dynamodbtypes.AttributeValueMemberS)
		sortValue, sortOK := input.Key["order"].(*dynamodbtypes.AttributeValueMemberS)
		if !partitionOK || partitionValue.Value != partition || !sortOK || sortValue.Value != sortKey {
			t.Fatalf("expected original pasted key bytes, got %#v", input.Key)
		}
		return &dynamodb.GetItemOutput{}, nil
	}}}
	_, cmd := m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := runDynamoDBLookupBatch(t, cmd).(dynamoDBItemLoadedMsg); !ok {
		t.Fatal("expected pasted keys to reach GetItem")
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

func TestDynamoDBValidationErrorsDoNotRenderControlBearingKeyNames(t *testing.T) {
	for _, tc := range []struct {
		name          string
		attributeType string
		input         string
		want          string
	}{
		{name: "number", attributeType: "N", input: "not-a-number", want: "partition key must be a number"},
		{name: "binary", attributeType: "B", input: "not-base64", want: "partition key must be non-empty base64"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(testConfig(), "", "dev")
			m.commands = nil
			table := dynamoDBTestTable()
			table.Keys = []awsservice.DynamoDBKey{{
				Name:          "id\n\x1b]52;c;payload\a",
				Role:          "PARTITION",
				AttributeType: tc.attributeType,
			}}
			m.dynamodb.selected = &table
			m.dynamodb.beginLookup(&m)
			m.awsRepo = &awsservice.AwsRepository{DynamoDBClient: &appDynamoDBClient{getItemFunc: func(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
				t.Fatal("validation error must not call GetItem")
				return nil, nil
			}}}

			m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.input)})
			next, cmd := m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyEnter})
			model := next.(Model)
			msg := runDynamoDBLookupBatch(t, cmd)
			updated, _ := model.Update(msg)
			view := updated.(Model).View()
			if !strings.Contains(view, tc.want) {
				t.Fatalf("expected validation error %q: %q", tc.want, view)
			}
			if strings.Contains(view, "\x1b]52;c;payload") || strings.ContainsRune(view, '\a') {
				t.Fatalf("validation error rendered key-name terminal controls: %q", view)
			}
		})
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

func TestDynamoDBLookupResultEscapesC1TerminalControls(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 100
	m.screen = screenDynamoDBLookupResult
	m.dynamodb.item = &awsservice.DynamoDBItem{
		Found: true,
		JSON:  "{\n  \"payload\": \"\u009d52;c;payload\u009c\u009b31m\"\n}",
	}

	view := m.dynamodb.viewLookupResult(m)
	for _, control := range []rune{'\u009b', '\u009c', '\u009d'} {
		if strings.ContainsRune(view, control) {
			t.Fatalf("item view contains raw C1 terminal control %U: %q", control, view)
		}
	}
	for _, escaped := range []string{`\u009b`, `\u009c`, `\u009d`} {
		if !strings.Contains(view, escaped) {
			t.Fatalf("expected item view to visibly escape %s: %q", escaped, view)
		}
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
	m.storeFilterValue(filterDynamoDBTables, "orders")
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
	if filter := model.filterValue(filterDynamoDBTables); filter != "" {
		t.Fatalf("expected context-scoped DynamoDB filter to be cleared, got %q", filter)
	}
	if view := model.View(); strings.Contains(view, "old-account") {
		t.Fatalf("previous account item leaked after context switch:\n%s", view)
	}
}

func TestDynamoDBContextSwitchDuringLoadDoesNotRestoreLoadingScreen(t *testing.T) {
	m := New(testConfig(), "", "dev")
	table := dynamoDBTestTable()
	m.dynamodb.tables = []awsservice.DynamoDBTable{table}
	m.dynamodb.filtered = m.dynamodb.tables
	m.dynamodb.selected = &table

	loading, _ := m.dynamodb.Start(&m)
	m = loading.(Model)
	inFlight := m.commands.Current()
	if m.screen != screenLoading || m.loadingReturnScreen != screenDynamoDBTableList {
		t.Fatalf("expected an owned DynamoDB list load, screen=%v return=%v", m.screen, m.loadingReturnScreen)
	}

	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = opened.(Model)
	if m.ctxPrevScreen != screenDynamoDBTableList || !m.ctxPrevWasLoading {
		t.Fatalf("expected context picker to preserve the DynamoDB load owner, screen=%v loading=%v", m.ctxPrevScreen, m.ctxPrevWasLoading)
	}
	if inFlight.Err() == nil {
		t.Fatal("expected opening the context picker to cancel the DynamoDB load")
	}

	switching, _ := m.startLoading(func() tea.Msg { return nil })
	m = switching.(Model)
	nextCfg := *m.cfg
	nextCfg.ContextName = "next-account"
	updated, _ := m.Update(contextSwitchedMsg{cfg: &nextCfg})
	model := updated.(Model)
	if model.screen != screenServiceList {
		t.Fatalf("expected a usable service list after switching context, got %v", model.screen)
	}
	if len(model.dynamodb.tables) != 0 || model.dynamodb.selected != nil {
		t.Fatalf("expected context-scoped DynamoDB state to be cleared, got %+v", model.dynamodb)
	}
}

func TestDynamoDBRegionPickerContextSwitchInvalidatesReturn(t *testing.T) {
	m := multiRegionTestModel()
	table := dynamoDBTestTable()
	m.dynamodb.selected = &table
	m.screen = screenDynamoDBTableDetail

	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	m = opened.(Model)
	if m.screen != screenRegionPicker || m.regionPrevScreen != screenDynamoDBTableDetail {
		t.Fatalf("expected region picker to remember DynamoDB detail, screen=%v return=%v", m.screen, m.regionPrevScreen)
	}

	openingContexts, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = openingContexts.(Model)
	loaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	m = loaded.(Model)
	if m.screen != screenContextPicker || m.ctxPrevScreen != screenRegionPicker {
		t.Fatalf("expected context picker to remember the region picker, screen=%v return=%v", m.screen, m.ctxPrevScreen)
	}

	switching, _ := m.startLoading(func() tea.Msg { return nil })
	m = switching.(Model)
	nextCfg := *m.cfg
	nextCfg.ContextName = "next-account"
	nextCfg.Region = "us-east-1"
	nextCfg.Regions = []string{"us-east-1"}
	switched, _ := m.Update(contextSwitchedMsg{cfg: &nextCfg})
	m = switched.(Model)

	closed, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := closed.(Model)
	if model.screen != screenServiceList {
		t.Fatalf("expected region picker cancel to discard the old DynamoDB return, got %v", model.screen)
	}
}

func TestDynamoDBContextPickerDropsInterruptedResultAndRestartsOnCancel(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg.ContextName = "dev"
	loading, _ := m.dynamodb.Start(&m)
	m = loading.(Model)
	staleGeneration := m.commands.CurrentGen()

	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = opened.(Model)
	loaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	m = loaded.(Model)
	if m.screen != screenContextPicker {
		t.Fatalf("expected context picker, got %v", m.screen)
	}

	stale, _ := m.Update(genBoundMsg{gen: staleGeneration, msg: dynamoDBTablesLoadedMsg{tables: []awsservice.DynamoDBTable{dynamoDBTestTable()}}})
	m = stale.(Model)
	if m.screen != screenContextPicker || len(m.dynamodb.tables) != 0 {
		t.Fatalf("expected stale DynamoDB result to leave the picker unchanged, screen=%v tables=%d", m.screen, len(m.dynamodb.tables))
	}

	resumed, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := resumed.(Model)
	if model.screen != screenLoading || model.loadingReturnScreen != screenDynamoDBTableList || cmd == nil {
		t.Fatalf("expected cancel to restart the interrupted list load, screen=%v return=%v cmd=%v", model.screen, model.loadingReturnScreen, cmd)
	}
}

func TestDynamoDBFailedContextSwitchDoesNotResumeInterruptedLoad(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg.ContextName = "dev"
	loading, _ := m.dynamodb.Start(&m)
	m = loading.(Model)

	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = opened.(Model)
	loaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	m = loaded.(Model)
	if !m.ctxPrevWasLoading {
		t.Fatal("expected context picker to remember the interrupted DynamoDB load")
	}

	switching, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = switching.(Model)
	if m.screen != screenLoading || cmd == nil {
		t.Fatalf("expected selected context switch to start loading, screen=%v cmd=%v", m.screen, cmd)
	}
	if m.ctxPrevWasLoading {
		t.Fatal("expected committed context switch to clear the interrupted-load resume flag")
	}

	failed, _ := m.Update(errMsg{err: errors.New("switch failed")})
	m = failed.(Model)
	dismissed, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = dismissed.(Model)
	if m.screen != screenServiceList {
		t.Fatalf("expected switch error dismissal to return to service list, got %v", m.screen)
	}

	reopened, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = reopened.(Model)
	reloaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	m = reloaded.(Model)
	canceled, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := canceled.(Model)
	if model.screen != screenServiceList || cmd != nil {
		t.Fatalf("expected later picker cancel to return to service list, screen=%v cmd=%v", model.screen, cmd)
	}
}

func TestDynamoDBFailedContextTerminalActionDoesNotResumeInterruptedLoad(t *testing.T) {
	origLoadNamed := contextLoadNamedContextFn
	origBuildEnv := contextBuildEnvExportsFn
	origCopy := contextCopyClipboardFn
	t.Cleanup(func() {
		contextLoadNamedContextFn = origLoadNamed
		contextBuildEnvExportsFn = origBuildEnv
		contextCopyClipboardFn = origCopy
	})
	contextLoadNamedContextFn = func(_ string, name string) (*config.Config, error) {
		return &config.Config{ContextName: name}, nil
	}
	contextBuildEnvExportsFn = func(context.Context, *config.Config) (string, error) {
		return "exports", nil
	}
	contextCopyClipboardFn = func(string) error { return errors.New("clipboard failed") }

	m := New(testConfig(), "", "dev")
	m.cfg.ContextName = "dev"
	loading, _ := m.dynamodb.Start(&m)
	m = loading.(Model)

	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = opened.(Model)
	loaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	m = loaded.(Model)
	if !m.ctxPrevWasLoading {
		t.Fatal("expected context picker to remember the interrupted DynamoDB load")
	}

	copying, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = copying.(Model)
	if m.screen != screenLoading || cmd == nil {
		t.Fatalf("expected terminal export to start loading, screen=%v cmd=%v", m.screen, cmd)
	}
	selected, ok := m.selectedContextInfo()
	if !ok {
		t.Fatal("expected a selected context")
	}
	failed, _ := m.Update(m.copySelectedContextExports(selected.Name)())
	m = failed.(Model)
	if m.screen != screenError || !strings.Contains(m.errMsg, "clipboard failed") {
		t.Fatalf("expected clipboard failure, screen=%v error=%q", m.screen, m.errMsg)
	}

	dismissed, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = dismissed.(Model)
	reopened, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = reopened.(Model)
	reloaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	m = reloaded.(Model)
	canceled, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := canceled.(Model)
	if model.screen != screenServiceList || cmd != nil {
		t.Fatalf("expected later picker cancel to stay on service list, screen=%v cmd=%v", model.screen, cmd)
	}
}

func TestDynamoDBContextFavoriteErrorDoesNotResumeInterruptedLoad(t *testing.T) {
	originalSetFavoriteContextsFn := configSetFavoriteContextsFn
	t.Cleanup(func() {
		configSetFavoriteContextsFn = originalSetFavoriteContextsFn
	})
	configSetFavoriteContextsFn = func(string, []string) error {
		return errors.New("write failed")
	}

	m := New(testConfig(), "/tmp/unic-test-config.yaml", "dev")
	m.cfg.ContextName = "dev"
	loading, _ := m.dynamodb.Start(&m)
	m = loading.(Model)

	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = opened.(Model)
	loaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	m = loaded.(Model)
	if !m.ctxPrevWasLoading {
		t.Fatal("expected context picker to remember the interrupted DynamoDB load")
	}

	failed, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = failed.(Model)
	if m.screen != screenError || !strings.Contains(m.errMsg, "write failed") {
		t.Fatalf("expected favorite persistence error, screen=%v error=%q", m.screen, m.errMsg)
	}

	dismissed, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = dismissed.(Model)
	if m.ctxPrevWasLoading {
		t.Fatal("expected error dismissal to clear the interrupted-load resume flag")
	}

	reopened, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = reopened.(Model)
	reloaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
	m = reloaded.(Model)
	canceled, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := canceled.(Model)
	if model.screen != screenServiceList || cmd != nil {
		t.Fatalf("expected later picker cancel to stay on service list, screen=%v cmd=%v", model.screen, cmd)
	}
}

func TestDynamoDBGlobalHomeClearsInterruptedContextLoad(t *testing.T) {
	for _, test := range []struct {
		name    string
		prepare func(*testing.T, *Model)
	}{
		{
			name: "settings",
			prepare: func(t *testing.T, m *Model) {
				t.Helper()
				opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
				*m = opened.(Model)
				if m.screen != screenSettings {
					t.Fatalf("expected settings screen, got %v", m.screen)
				}
			},
		},
		{
			name: "error",
			prepare: func(t *testing.T, m *Model) {
				t.Helper()
				originalSetFavoriteContextsFn := configSetFavoriteContextsFn
				t.Cleanup(func() { configSetFavoriteContextsFn = originalSetFavoriteContextsFn })
				configSetFavoriteContextsFn = func(string, []string) error { return errors.New("write failed") }
				failed, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
				*m = failed.(Model)
				if m.screen != screenError {
					t.Fatalf("expected context favorite error, got %v", m.screen)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := New(testConfig(), t.TempDir()+"/config.yaml", "dev")
			m.cfg.ContextName = "dev"
			loading, _ := m.dynamodb.Start(&m)
			m = loading.(Model)

			opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
			m = opened.(Model)
			loaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
			m = loaded.(Model)
			if !m.ctxPrevWasLoading {
				t.Fatal("expected context picker to remember the interrupted DynamoDB load")
			}

			test.prepare(t, &m)
			home, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
			m = home.(Model)
			if m.screen != screenServiceList || m.ctxPrevWasLoading {
				t.Fatalf("expected Home to clear interrupted context state, screen=%v loading=%v", m.screen, m.ctxPrevWasLoading)
			}

			reopened, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = reopened.(Model)
			reloaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
			m = reloaded.(Model)
			canceled, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			model := canceled.(Model)
			if model.screen != screenServiceList || cmd != nil {
				t.Fatalf("expected later picker cancel to stay on service list, screen=%v cmd=%v", model.screen, cmd)
			}
		})
	}
}

func TestDynamoDBContextPickerCommittedOverlaysClearInterruptedLoad(t *testing.T) {
	tests := []struct {
		name        string
		key         rune
		prepare     func(*Model)
		switchFails bool
	}{
		{
			name: "palette feature",
			key:  'P',
			prepare: func(m *Model) {
				m.palette.filtered = []paletteItem{{
					kind: paletteItemFeature, service: domain.ServiceRDS, feature: domain.FeatureRDSBrowser,
				}}
			},
		},
		{
			name: "saved view feature",
			key:  'V',
			prepare: func(m *Model) {
				m.views.views = []config.ViewEntry{{
					Context: "dev", Service: string(domain.ServiceRDS), Feature: string(domain.FeatureRDSBrowser),
				}}
			},
		},
		{
			name: "palette context",
			key:  'P',
			prepare: func(m *Model) {
				m.palette.filtered = []paletteItem{{kind: paletteItemContext, contextName: "next-account"}}
			},
			switchFails: true,
		},
		{
			name: "saved view context",
			key:  'V',
			prepare: func(m *Model) {
				m.views.views = []config.ViewEntry{{
					Context: "next-account", Service: string(domain.ServiceRDS), Feature: string(domain.FeatureRDSBrowser),
				}}
			},
			switchFails: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := New(testConfig(), t.TempDir()+"/config.yaml", "dev")
			m.cfg.ContextName = "dev"
			loading, _ := m.dynamodb.Start(&m)
			m = loading.(Model)

			opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
			m = opened.(Model)
			loaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
			m = loaded.(Model)
			if !m.ctxPrevWasLoading {
				t.Fatal("expected context picker to remember the interrupted DynamoDB load")
			}

			overlay, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{test.key}})
			m = overlay.(Model)
			test.prepare(&m)
			committed, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = committed.(Model)
			if m.ctxPrevWasLoading {
				t.Fatal("expected committed overlay action to clear the interrupted-load resume flag")
			}
			if m.screen != screenLoading || cmd == nil {
				t.Fatalf("expected committed overlay action to start loading, screen=%v cmd=%v", m.screen, cmd)
			}

			if test.switchFails {
				failed, _ := m.Update(errMsg{err: errors.New("switch failed")})
				m = failed.(Model)
				dismissed, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
				m = dismissed.(Model)
			} else {
				home, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
				m = home.(Model)
			}

			reopened, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = reopened.(Model)
			reloaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
			m = reloaded.(Model)
			canceled, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			model := canceled.(Model)
			if model.screen != screenServiceList || cmd != nil {
				t.Fatalf("expected later picker cancel to stay on service list, screen=%v cmd=%v", model.screen, cmd)
			}
		})
	}
}

func TestDynamoDBGlobalOverlaysDropInterruptedResultAndRestartOnCancel(t *testing.T) {
	for _, tc := range []struct {
		name   string
		key    rune
		screen screen
	}{
		{name: "settings", key: 'S', screen: screenSettings},
		{name: "palette", key: 'P', screen: screenCommandPalette},
		{name: "views", key: 'V', screen: screenViewList},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(testConfig(), t.TempDir()+"/config.yaml", "dev")
			loading, _ := m.dynamodb.Start(&m)
			m = loading.(Model)
			staleGeneration := m.commands.CurrentGen()

			opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
			m = opened.(Model)
			if m.screen != tc.screen {
				t.Fatalf("expected %s overlay, got %v", tc.name, m.screen)
			}

			stale, _ := m.Update(genBoundMsg{gen: staleGeneration, msg: dynamoDBTablesLoadedMsg{tables: []awsservice.DynamoDBTable{dynamoDBTestTable()}}})
			m = stale.(Model)
			if m.screen != tc.screen || len(m.dynamodb.tables) != 0 {
				t.Fatalf("expected stale DynamoDB result to leave %s active, screen=%v tables=%d", tc.name, m.screen, len(m.dynamodb.tables))
			}

			resumed, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			model := resumed.(Model)
			if model.screen != screenLoading || model.loadingReturnScreen != screenDynamoDBTableList || cmd == nil {
				t.Fatalf("expected %s cancel to restart the list load, screen=%v return=%v cmd=%v", tc.name, model.screen, model.loadingReturnScreen, cmd)
			}
		})
	}
}

func TestDynamoDBOverlayContextSwitchInvalidatesLoadingReturn(t *testing.T) {
	for _, tc := range []struct {
		name   string
		key    rune
		screen screen
	}{
		{name: "settings", key: 'S', screen: screenSettings},
		{name: "views", key: 'V', screen: screenViewList},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(testConfig(), t.TempDir()+"/config.yaml", "dev")
			loading, _ := m.dynamodb.Start(&m)
			m = loading.(Model)

			opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
			m = opened.(Model)
			if m.screen != tc.screen {
				t.Fatalf("expected %s overlay, got %v", tc.name, m.screen)
			}

			openingPicker, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
			m = openingPicker.(Model)
			loaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
			m = loaded.(Model)
			if m.screen != screenContextPicker {
				t.Fatalf("expected context picker from %s, got %v", tc.name, m.screen)
			}

			switching, _ := m.startLoading(func() tea.Msg { return nil })
			m = switching.(Model)
			nextCfg := *m.cfg
			nextCfg.ContextName = "next-account"
			switched, _ := m.Update(contextSwitchedMsg{cfg: &nextCfg})
			m = switched.(Model)
			if m.screen != tc.screen {
				t.Fatalf("expected context switch to return to %s, got %v", tc.name, m.screen)
			}

			closed, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			model := closed.(Model)
			if model.screen != screenServiceList {
				t.Fatalf("expected %s to discard the old loading return, got %v", tc.name, model.screen)
			}
		})
	}
}

func TestDynamoDBOverlayContextSwitchInvalidatesNonLoadingReturn(t *testing.T) {
	for _, tc := range []struct {
		name   string
		key    rune
		screen screen
	}{
		{name: "settings", key: 'S', screen: screenSettings},
		{name: "views", key: 'V', screen: screenViewList},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(testConfig(), t.TempDir()+"/config.yaml", "dev")
			table := dynamoDBTestTable()
			m.dynamodb.selected = &table
			m.screen = screenDynamoDBTableDetail

			opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
			m = opened.(Model)
			if m.screen != tc.screen {
				t.Fatalf("expected %s overlay, got %v", tc.name, m.screen)
			}

			openingPicker, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
			m = openingPicker.(Model)
			loaded, _ := m.Update(contextsLoadedMsg{contexts: testContexts()})
			m = loaded.(Model)
			switching, _ := m.startLoading(func() tea.Msg { return nil })
			m = switching.(Model)
			nextCfg := *m.cfg
			nextCfg.ContextName = "next-account"
			switched, _ := m.Update(contextSwitchedMsg{cfg: &nextCfg})
			m = switched.(Model)
			if m.screen != tc.screen {
				t.Fatalf("expected context switch to return to %s, got %v", tc.name, m.screen)
			}

			closed, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			model := closed.(Model)
			if model.screen != screenServiceList {
				t.Fatalf("expected %s to discard the old DynamoDB return, got %v", tc.name, model.screen)
			}
		})
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

func TestDynamoDBLookupInputIgnoresIncompleteState(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenDynamoDBLookupInput
	table := dynamoDBTestTable()
	m.dynamodb.selected = &table
	m.dynamodb.lookupInput = "acme"

	updated, cmd := m.dynamodb.updateLookupInput(&m, tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if cmd != nil || model.screen != screenDynamoDBLookupInput {
		t.Fatalf("expected incomplete lookup state to remain idle, screen=%v cmd=%v", model.screen, cmd)
	}
}
