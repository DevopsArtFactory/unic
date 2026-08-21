package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	awsservice "unic/internal/services/aws"
)

type dynamoDBModel struct {
	tables       []awsservice.DynamoDBTable
	filtered     []awsservice.DynamoDBTable
	tableIdx     int
	selected     *awsservice.DynamoDBTable
	detailScroll int
	lookupValues []string
	lookupField  int
	lookupInput  string
	item         *awsservice.DynamoDBItem
	itemScroll   int
}

func newDynamoDBModel() dynamoDBModel {
	return dynamoDBModel{}
}

func (dm *dynamoDBModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(dm.loadTables(*m))
}

func (dm *dynamoDBModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case dynamoDBTablesLoadedMsg:
		dm.tables = msg.tables
		dm.filtered = applyFilter(dm.tables, m.filterValue(filterDynamoDBTables))
		dm.tableIdx = 0
		m.screen = screenDynamoDBTableList
		return *m, nil, true
	case dynamoDBTableDetailLoadedMsg:
		dm.selected = msg.table
		dm.detailScroll = 0
		m.screen = screenDynamoDBTableDetail
		return *m, nil, true
	case dynamoDBItemLoadedMsg:
		dm.item = msg.item
		dm.itemScroll = 0
		m.screen = screenDynamoDBLookupResult
		return *m, nil, true
	}
	return *m, nil, false
}

func (dm *dynamoDBModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenDynamoDBTableList:
		newM, cmd := dm.updateTableList(m, msg)
		return newM, cmd, true
	case screenDynamoDBTableDetail:
		newM, cmd := dm.updateTableDetail(m, msg)
		return newM, cmd, true
	case screenDynamoDBLookupInput:
		newM, cmd := dm.updateLookupInput(m, msg)
		return newM, cmd, true
	case screenDynamoDBLookupResult:
		newM, cmd := dm.updateLookupResult(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (dm dynamoDBModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenDynamoDBTableList:
		return dm.viewTableList(m), true
	case screenDynamoDBTableDetail:
		return dm.viewTableDetail(m), true
	case screenDynamoDBLookupInput:
		return dm.viewLookupInput(m), true
	case screenDynamoDBLookupResult:
		return dm.viewLookupResult(m), true
	default:
		return "", false
	}
}

func (dm *dynamoDBModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterDynamoDBTables {
		return false
	}
	dm.filtered = applyFilter(dm.tables, m.filterValue(target))
	dm.tableIdx = 0
	return true
}

func (dm *dynamoDBModel) updateTableList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterDynamoDBTables); handled {
		return *m, cmd
	}

	switch msg.String() {
	case "q", "esc":
		m.resetFilter(filterDynamoDBTables)
		m.screen = screenFeatureList
	case "up", "k":
		dm.tableIdx = previousListIndex(dm.tableIdx, len(dm.filtered))
	case "down", "j":
		dm.tableIdx = nextListIndex(dm.tableIdx, len(dm.filtered))
	case "/":
		return *m, m.activateFilter(filterDynamoDBTables)
	case "r":
		return m.startLoading(dm.loadTables(*m))
	case "enter":
		if len(dm.filtered) > 0 && dm.tableIdx < len(dm.filtered) {
			selected := dm.filtered[dm.tableIdx]
			dm.selected = &selected
			return m.startLoadingWithMessage("Loading DynamoDB table...", []string{selected.Name}, dm.loadTableDetail(*m))
		}
	}
	return *m, nil
}

func (dm *dynamoDBModel) updateTableDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := dm.tableDetailLines()
	page := max(m.height-8, 5)
	switch msg.String() {
	case "q", "esc":
		m.screen = screenDynamoDBTableList
	case "up", "k":
		dm.detailScroll = dynamoDBScroll(dm.detailScroll-1, len(lines), page)
	case "down", "j":
		dm.detailScroll = dynamoDBScroll(dm.detailScroll+1, len(lines), page)
	case "pgup":
		dm.detailScroll = dynamoDBScroll(dm.detailScroll-page, len(lines), page)
	case "pgdown":
		dm.detailScroll = dynamoDBScroll(dm.detailScroll+page, len(lines), page)
	case "l":
		if dm.selected != nil && len(dm.selected.Keys) > 0 {
			dm.beginLookup(m)
		}
	case "r":
		if dm.selected != nil {
			return m.startLoadingWithMessage("Refreshing DynamoDB table...", []string{dm.selected.Name}, dm.loadTableDetail(*m))
		}
	}
	return *m, nil
}

func (dm *dynamoDBModel) updateLookupInput(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenDynamoDBTableDetail
	case "enter":
		if dm.selected == nil || dm.lookupField >= len(dm.selected.Keys) || dm.lookupInput == "" {
			return *m, nil
		}
		dm.lookupValues[dm.lookupField] = dm.lookupInput
		if dm.lookupField+1 < len(dm.selected.Keys) {
			dm.lookupField++
			dm.lookupInput = dm.lookupValues[dm.lookupField]
			return *m, nil
		}
		return m.startLoadingWithMessage("Looking up DynamoDB item...", []string{dm.selected.Name}, dm.lookupItem(*m))
	case "backspace":
		dm.lookupInput = trimLastRune(dm.lookupInput)
	default:
		dm.lookupInput = appendKeyRunes(dm.lookupInput, msg)
	}
	return *m, nil
}

func (dm *dynamoDBModel) updateLookupResult(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := dm.itemLines(*m)
	page := max(m.height-8, 5)
	switch msg.String() {
	case "q", "esc":
		m.screen = screenDynamoDBTableDetail
	case "up", "k":
		dm.itemScroll = dynamoDBScroll(dm.itemScroll-1, len(lines), page)
	case "down", "j":
		dm.itemScroll = dynamoDBScroll(dm.itemScroll+1, len(lines), page)
	case "pgup":
		dm.itemScroll = dynamoDBScroll(dm.itemScroll-page, len(lines), page)
	case "pgdown":
		dm.itemScroll = dynamoDBScroll(dm.itemScroll+page, len(lines), page)
	case "l":
		dm.beginLookup(m)
	}
	return *m, nil
}

func (dm *dynamoDBModel) beginLookup(m *Model) {
	if dm.selected == nil {
		return
	}
	dm.lookupValues = make([]string, len(dm.selected.Keys))
	dm.lookupField = 0
	dm.lookupInput = ""
	dm.item = nil
	m.screen = screenDynamoDBLookupInput
}

func (dm dynamoDBModel) loadTables(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}
		tables, err := repo.ListDynamoDBTables(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return dynamoDBTablesLoadedMsg{tables: tables}
	}
}

func (dm dynamoDBModel) loadTableDetail(m Model) tea.Cmd {
	if dm.selected == nil {
		return func() tea.Msg { return errMsg{err: fmt.Errorf("no DynamoDB table selected")} }
	}
	tableName := dm.selected.Name
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}
		table, err := repo.DescribeDynamoDBTable(ctx, tableName)
		if err != nil {
			return errMsg{err: err}
		}
		return dynamoDBTableDetailLoadedMsg{table: table}
	}
}

func (dm dynamoDBModel) lookupItem(m Model) tea.Cmd {
	if dm.selected == nil {
		return func() tea.Msg { return errMsg{err: fmt.Errorf("no DynamoDB table selected")} }
	}
	table := *dm.selected
	values := append([]string(nil), dm.lookupValues...)
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}
		item, err := repo.GetDynamoDBItem(ctx, table, values)
		if err != nil {
			return errMsg{err: err}
		}
		return dynamoDBItemLoadedMsg{item: item}
	}
}

func (dm dynamoDBModel) viewTableList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("DynamoDB Tables"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterDynamoDBTables))
	b.WriteString("\n\n")

	if len(dm.filtered) == 0 {
		panel.WriteString(dimStyle.Render("  No matching tables"))
		panel.WriteString("\n")
	} else {
		nameCol := lipgloss.NewStyle().Width(32)
		statusCol := lipgloss.NewStyle().Width(11)
		billingCol := lipgloss.NewStyle().Width(18)
		capacityCol := lipgloss.NewStyle().Width(15)
		itemsCol := lipgloss.NewStyle().Width(12)
		sizeCol := lipgloss.NewStyle().Width(11)
		gsiCol := lipgloss.NewStyle().Width(5)
		panel.WriteString(dimStyle.Render("  " + nameCol.Render("TABLE") + statusCol.Render("STATUS") + billingCol.Render("BILLING") + capacityCol.Render("CAPACITY") + itemsCol.Render("ITEMS") + sizeCol.Render("SIZE") + gsiCol.Render("GSIS")))
		panel.WriteString("\n")

		visibleLines := max(m.height-12, 5)
		start := 0
		if dm.tableIdx >= visibleLines {
			start = dm.tableIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(dm.filtered))
		for i := start; i < end; i++ {
			table := dm.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == dm.tableIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor))
			panel.WriteString(nameCol.Inherit(style).Render(m.renderHighlightedValue(filterDynamoDBTables, table.Name)))
			panel.WriteString(statusCol.Inherit(style).Render(table.Status))
			panel.WriteString(billingCol.Inherit(style).Render(table.BillingMode))
			panel.WriteString(capacityCol.Inherit(style).Render(table.CapacityLabel()))
			panel.WriteString(itemsCol.Inherit(style).Render(fmt.Sprintf("%d", table.ItemCount)))
			panel.WriteString(sizeCol.Inherit(style).Render(formatBytes(table.SizeBytes)))
			panel.WriteString(gsiCol.Inherit(style).Render(fmt.Sprintf("%d", table.GSICount)))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d tables", len(dm.filtered), len(dm.tables))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (dm dynamoDBModel) viewTableDetail(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	if dm.selected == nil {
		return b.String()
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("DynamoDB — %s", dm.selected.Name)))
	b.WriteString("\n\n")

	lines := dm.tableDetailLines()
	visibleLines := max(m.height-8, 5)
	start := dynamoDBScroll(dm.detailScroll, len(lines), visibleLines)
	end := min(start+visibleLines, len(lines))
	b.WriteString(m.renderListPanel(strings.Join(lines[start:end], "\n")))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (dm dynamoDBModel) tableDetailLines() []string {
	if dm.selected == nil {
		return nil
	}
	table := dm.selected
	lines := []string{
		renderDetailLine("Status", table.Status),
		renderDetailLine("Billing", table.BillingMode),
		renderDetailLine("Capacity", table.CapacityLabel()),
		renderDetailLine("Items", fmt.Sprintf("%d", table.ItemCount)),
		renderDetailLine("Size", formatBytes(table.SizeBytes)),
		renderDetailLine("ARN", table.ARN),
	}
	if !table.CreatedAt.IsZero() {
		lines = append(lines, renderDetailLine("Created", table.CreatedAt.Format("2006-01-02 15:04:05 MST")))
	}
	lines = append(lines, "", titleStyle.Render("  Primary Key"))
	for _, key := range table.Keys {
		lines = append(lines, renderDetailLine(key.Role, fmt.Sprintf("%s (%s)", key.Name, key.AttributeType)))
	}
	ttl := table.TTLStatus
	if ttl == "" {
		ttl = "UNKNOWN"
	}
	if table.TTLAttribute != "" {
		ttl += " on " + table.TTLAttribute
	}
	lines = append(lines, "", renderDetailLine("TTL", ttl))
	stream := "Disabled"
	if table.StreamEnabled {
		stream = table.StreamView
	}
	lines = append(lines, renderDetailLine("Stream", stream))
	if table.StreamARN != "" {
		lines = append(lines, renderDetailLine("Stream ARN", table.StreamARN))
	}
	lines = append(lines, "", titleStyle.Render("  Global Secondary Indexes"))
	if len(table.GSIs) == 0 {
		lines = append(lines, dimStyle.Render("  None"))
	}
	for _, index := range table.GSIs {
		lines = append(lines, normalStyle.Render("  "+index.Name), renderDetailLine("Status", index.Status), renderDetailLine("Projection", index.Projection))
		if table.BillingMode != "PAY_PER_REQUEST" {
			lines = append(lines, renderDetailLine("Capacity", fmt.Sprintf("%dR/%dW", index.ReadCapacity, index.WriteCapacity)))
		}
		for _, key := range index.Keys {
			lines = append(lines, renderDetailLine(key.Role, fmt.Sprintf("%s (%s)", key.Name, key.AttributeType)))
		}
	}
	return lines
}

func (dm dynamoDBModel) viewLookupInput(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	if dm.selected == nil || dm.lookupField >= len(dm.selected.Keys) {
		return b.String()
	}
	table := dm.selected
	key := table.Keys[dm.lookupField]
	b.WriteString(titleStyle.Render(fmt.Sprintf("GetItem — %s", table.Name)))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  This performs one GetItem request. It never scans the table."))
	b.WriteString("\n\n")
	for i := 0; i < dm.lookupField; i++ {
		previous := table.Keys[i]
		b.WriteString(renderDetailLine(previous.Role, fmt.Sprintf("%s = %s", previous.Name, dm.lookupValues[i])))
		b.WriteString("\n")
	}
	b.WriteString(normalStyle.Render(fmt.Sprintf("  %s key %s (%s):", key.Role, key.Name, key.AttributeType)))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render("  " + dm.lookupInput + "▏"))
	b.WriteString("\n")
	if key.AttributeType == "B" {
		b.WriteString(dimStyle.Render("  Enter binary keys as base64."))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (dm dynamoDBModel) viewLookupResult(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	tableName := ""
	if dm.selected != nil {
		tableName = dm.selected.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("DynamoDB Item — %s", tableName)))
	b.WriteString("\n\n")
	lines := dm.itemLines(m)
	visibleLines := max(m.height-8, 5)
	start := dynamoDBScroll(dm.itemScroll, len(lines), visibleLines)
	end := min(start+visibleLines, len(lines))
	var panel strings.Builder
	for _, line := range lines[start:end] {
		panel.WriteString(normalStyle.Render("  " + line))
		panel.WriteString("\n")
	}
	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (dm dynamoDBModel) itemLines(m Model) []string {
	if dm.item == nil || !dm.item.Found {
		return []string{"No item found for that primary key."}
	}
	lines := strings.Split(dm.item.JSON, "\n")
	if m.width <= 0 {
		return lines
	}

	width := max(m.width-m.currentListPanelStyle().GetHorizontalFrameSize()-2, 1)
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped = append(wrapped, strings.Split(ansi.Hardwrap(line, width, true), "\n")...)
	}
	return wrapped
}

func dynamoDBScroll(offset, total, visible int) int {
	if offset < 0 {
		return 0
	}
	maxOffset := max(total-visible, 0)
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}
