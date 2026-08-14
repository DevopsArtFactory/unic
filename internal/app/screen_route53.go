package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsservice "unic/internal/services/aws"
)

type route53Model struct {
	zones           []awsservice.HostedZone
	filteredZones   []awsservice.HostedZone
	zoneIdx         int
	selectedZone    *awsservice.HostedZone
	records         []awsservice.DNSRecord
	filteredRecords []awsservice.DNSRecord
	recordIdx       int
	selectedRecord  *awsservice.DNSRecord
	action          string            // "create", "edit", "delete"
	confirmInput    string            // type-to-confirm for delete
	editField       int               // current form field index
	editValues      map[string]string // accumulated form values
	editInput       string            // current field text input
	editSelectIdx   int               // index for select-type fields (record type)
	changeID        string            // for status polling
	changeStatus    string            // "PENDING" / "INSYNC"
	polling         bool              // polling active
}

func newRoute53Model() route53Model {
	return route53Model{}
}

func (rm *route53Model) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(rm.loadZones(*m))
}

func (rm *route53Model) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case route53ZonesLoadedMsg:
		rm.zones = msg.zones
		rm.filteredZones = applyFilter(rm.zones, m.filterValue(filterRoute53Zones))
		rm.zoneIdx = 0
		m.screen = screenRoute53ZoneList
		return *m, nil, true

	case route53RecordsLoadedMsg:
		rm.records = msg.records
		rm.filteredRecords = applyFilter(rm.records, m.filterValue(filterRoute53Records))
		rm.recordIdx = 0
		m.screen = screenRoute53RecordList
		return *m, nil, true

	case route53ActionDoneMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return *m, nil, true
		}
		rm.changeID = msg.changeID
		rm.changeStatus = "PENDING"
		rm.polling = true
		if rm.selectedZone != nil {
			return *m, tea.Batch(
				rm.loadRecords(*m, rm.selectedZone.ID),
				rm.pollChangeStatus(*m),
			), true
		}
		m.screen = screenRoute53RecordList
		return *m, nil, true

	case route53ChangeStatusMsg:
		if msg.err != nil {
			rm.polling = false
			return *m, nil, true
		}
		rm.changeStatus = msg.status
		if msg.status == "INSYNC" {
			rm.polling = false
			return *m, nil, true
		}
		return *m, rm.tickPoll(), true

	case route53PollTickMsg:
		if rm.polling {
			return *m, rm.pollChangeStatus(*m), true
		}
		return *m, nil, true
	}
	return *m, nil, false
}

func (rm *route53Model) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenRoute53ZoneList:
		newM, cmd := rm.updateZoneList(m, msg)
		return newM, cmd, true
	case screenRoute53RecordList:
		newM, cmd := rm.updateRecordList(m, msg)
		return newM, cmd, true
	case screenRoute53RecordDetail:
		newM, cmd := rm.updateRecordDetail(m, msg)
		return newM, cmd, true
	case screenRoute53RecordCreate:
		newM, cmd := rm.updateRecordCreate(m, msg)
		return newM, cmd, true
	case screenRoute53RecordEdit:
		newM, cmd := rm.updateRecordEdit(m, msg)
		return newM, cmd, true
	case screenRoute53RecordDeleteConfirm:
		newM, cmd := rm.updateRecordDeleteConfirm(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (rm route53Model) View(m Model) (string, bool) {
	switch m.screen {
	case screenRoute53ZoneList:
		return rm.viewZoneList(m), true
	case screenRoute53RecordList:
		return rm.viewRecordList(m), true
	case screenRoute53RecordDetail:
		return rm.viewRecordDetail(m), true
	case screenRoute53RecordCreate:
		return rm.viewRecordCreate(m), true
	case screenRoute53RecordEdit:
		return rm.viewRecordEdit(m), true
	case screenRoute53RecordDeleteConfirm:
		return rm.viewRecordDeleteConfirm(m), true
	default:
		return "", false
	}
}

func (rm *route53Model) ApplyFilter(m *Model, target filterTarget) bool {
	switch target {
	case filterRoute53Zones:
		rm.filteredZones = applyFilter(rm.zones, m.filterValue(target))
		rm.zoneIdx = 0
		return true
	case filterRoute53Records:
		rm.filteredRecords = applyFilter(rm.records, m.filterValue(target))
		rm.recordIdx = 0
		return true
	default:
		return false
	}
}

func (rm *route53Model) updateZoneList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterRoute53Zones); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterRoute53Zones)
	case "up", "k":
		rm.zoneIdx = previousListIndex(rm.zoneIdx, len(rm.filteredZones))
	case "down", "j":
		rm.zoneIdx = nextListIndex(rm.zoneIdx, len(rm.filteredZones))
	case "/":
		return *m, m.activateFilter(filterRoute53Zones)
	case "enter":
		if len(rm.filteredZones) > 0 && rm.zoneIdx < len(rm.filteredZones) {
			selected := rm.filteredZones[rm.zoneIdx]
			rm.selectedZone = &selected
			return m.startLoading(rm.loadRecords(*m, selected.ID))
		}
	}
	return *m, nil
}

func (rm *route53Model) updateRecordList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterRoute53Records); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenRoute53ZoneList
		m.resetFilter(filterRoute53Records)
	case "up", "k":
		rm.recordIdx = previousListIndex(rm.recordIdx, len(rm.filteredRecords))
	case "down", "j":
		rm.recordIdx = nextListIndex(rm.recordIdx, len(rm.filteredRecords))
	case "/":
		return *m, m.activateFilter(filterRoute53Records)
	case "c":
		rm.action = "create"
		rm.editField = 0
		rm.editValues = map[string]string{}
		rm.editInput = ""
		rm.editSelectIdx = 0
		m.screen = screenRoute53RecordCreate
	case "enter":
		if len(rm.filteredRecords) > 0 && rm.recordIdx < len(rm.filteredRecords) {
			selected := rm.filteredRecords[rm.recordIdx]
			rm.selectedRecord = &selected
			m.screen = screenRoute53RecordDetail
		}
	}
	return *m, nil
}

func (rm *route53Model) updateRecordDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenRoute53RecordList
	case "e":
		// Edit only for A/CNAME non-alias records
		if rm.selectedRecord != nil && rm.selectedRecord.AliasTarget == "" &&
			(rm.selectedRecord.Type == "A" || rm.selectedRecord.Type == "CNAME") {
			rm.action = "edit"
			rm.editField = 0
			rm.editValues = map[string]string{
				"value": strings.Join(rm.selectedRecord.Values, ","),
				"ttl":   fmt.Sprintf("%d", rm.selectedRecord.TTL),
			}
			rm.editInput = strings.Join(rm.selectedRecord.Values, ",")
			m.screen = screenRoute53RecordEdit
		}
	case "d":
		// Delete allowed for non-NS/SOA records
		if rm.selectedRecord != nil &&
			rm.selectedRecord.Type != "NS" && rm.selectedRecord.Type != "SOA" {
			rm.action = "delete"
			rm.confirmInput = ""
			m.screen = screenRoute53RecordDeleteConfirm
		}
	case "c":
		rm.action = "create"
		rm.editField = 0
		rm.editValues = map[string]string{}
		rm.editInput = ""
		rm.editSelectIdx = 0
		m.screen = screenRoute53RecordCreate
	}
	return *m, nil
}

func (rm route53Model) loadZones(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		zones, err := repo.ListHostedZones(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(zones) == 0 {
			return errMsg{err: fmt.Errorf("no hosted zones found")}
		}
		return route53ZonesLoadedMsg{zones: zones}
	}
}

func (rm route53Model) loadRecords(m Model, zoneID string) tea.Cmd {
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

		records, err := repo.ListResourceRecordSets(ctx, zoneID)
		if err != nil {
			return errMsg{err: err}
		}
		if len(records) == 0 {
			return errMsg{err: fmt.Errorf("no records found in zone %s", zoneID)}
		}
		return route53RecordsLoadedMsg{records: records}
	}
}

func (rm route53Model) viewZoneList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Route53 Hosted Zones"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterRoute53Zones))
	b.WriteString("\n\n")

	if len(rm.filteredZones) == 0 {
		panel.WriteString(dimStyle.Render("  No matching hosted zones"))
		panel.WriteString("\n")
	} else {
		// Measure max widths for column alignment
		maxName, maxID := 4, 2 // "NAME", "ID"
		for _, zone := range rm.filteredZones {
			if len(zone.Name) > maxName {
				maxName = len(zone.Name)
			}
			if len(zone.ID) > maxID {
				maxID = len(zone.ID)
			}
		}
		nameCol := lipgloss.NewStyle().Width(maxName + 2)
		idCol := lipgloss.NewStyle().Width(maxID + 2)
		recordsCol := lipgloss.NewStyle().Width(9) // "RECORDS" + padding

		// Header
		panel.WriteString(dimStyle.Render("  " + nameCol.Render("NAME") + idCol.Render("ID") + recordsCol.Render("RECORDS") + "TYPE"))
		panel.WriteString("\n")

		visibleLines := max(m.height-11, 5)
		start := 0
		if rm.zoneIdx >= visibleLines {
			start = rm.zoneIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(rm.filteredZones))

		for i := start; i < end; i++ {
			zone := rm.filteredZones[i]
			cursor := "  "
			style := normalStyle
			if i == rm.zoneIdx {
				cursor = "> "
				style = selectedStyle
			}
			zoneType := "Public"
			if zone.IsPrivate {
				zoneType = "Private"
			}
			row := cursor +
				nameCol.Inherit(style).Render(m.renderHighlightedValue(filterRoute53Zones, zone.Name)) +
				idCol.Inherit(dimStyle).Render(m.renderHighlightedValue(filterRoute53Zones, zone.ID)) +
				recordsCol.Inherit(dimStyle).Render(fmt.Sprintf("%d", zone.ResourceRecordCount)) +
				dimStyle.Render(m.renderHighlightedValue(filterRoute53Zones, zoneType))
			panel.WriteString(row)
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d zones", len(rm.filteredZones), len(rm.zones))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: records • esc: back • H: home"))
	return b.String()
}

func (rm route53Model) viewRecordList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	zoneName := ""
	if rm.selectedZone != nil {
		zoneName = rm.selectedZone.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("DNS Records — %s", zoneName)))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterRoute53Records))
	b.WriteString("\n\n")

	if len(rm.filteredRecords) == 0 {
		panel.WriteString(dimStyle.Render("  No matching records"))
		panel.WriteString("\n")
	} else {
		// Measure max widths for column alignment
		maxName, maxType := 4, 4 // "NAME", "TYPE"
		for _, rec := range rm.filteredRecords {
			if len(rec.Name) > maxName {
				maxName = len(rec.Name)
			}
			if len(rec.Type) > maxType {
				maxType = len(rec.Type)
			}
		}
		nameCol := lipgloss.NewStyle().Width(maxName + 2)
		typeCol := lipgloss.NewStyle().Width(maxType + 2)

		// Header
		panel.WriteString(dimStyle.Render("  " + nameCol.Render("NAME") + typeCol.Render("TYPE") + "VALUE"))
		panel.WriteString("\n")

		visibleLines := max(m.height-11, 5)
		start := 0
		if rm.recordIdx >= visibleLines {
			start = rm.recordIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(rm.filteredRecords))

		for i := start; i < end; i++ {
			rec := rm.filteredRecords[i]
			cursor := "  "
			style := normalStyle
			if i == rm.recordIdx {
				cursor = "> "
				style = selectedStyle
			}
			// Build value string
			valStr := ""
			if rec.AliasTarget != "" {
				valStr = "ALIAS → " + rec.AliasTarget
			} else {
				valStr = strings.Join(rec.Values, ", ")
			}
			if len(valStr) > 60 {
				valStr = valStr[:57] + "..."
			}
			row := cursor +
				nameCol.Inherit(style).Render(m.renderHighlightedValue(filterRoute53Records, rec.Name)) +
				typeCol.Inherit(filterStyle).Render(m.renderHighlightedValue(filterRoute53Records, rec.Type)) +
				dimStyle.Render(m.renderHighlightedValue(filterRoute53Records, valStr))
			panel.WriteString(row)
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d records", len(rm.filteredRecords), len(rm.records))))
	}

	// Show change status if polling
	if rm.polling {
		panel.WriteString("\n")
		panel.WriteString(filterStyle.Render(fmt.Sprintf("  Change: %s...", rm.changeStatus)))
		panel.WriteString("\n")
	} else if rm.changeStatus == "INSYNC" {
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render("  Change: INSYNC"))
		panel.WriteString("\n")
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • c: create • enter: detail • esc: back • H: home"))
	return b.String()
}

func (rm route53Model) viewRecordDetail(m Model) string {
	if rm.selectedRecord == nil {
		return ""
	}
	r := rm.selectedRecord
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("DNS Record Detail"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Name", normalStyle.Render(r.Name)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Type", filterStyle.Render(r.Type)))
	b.WriteString("\n")

	if r.AliasTarget != "" {
		b.WriteString(renderDetailLine("Alias", normalStyle.Render(r.AliasTarget)))
		b.WriteString("\n")
	} else {
		b.WriteString(renderDetailLine("TTL", normalStyle.Render(fmt.Sprintf("%d", r.TTL))))
		b.WriteString("\n")
	}

	if len(r.Values) > 0 {
		b.WriteString(renderDetailLine("Values", ""))
		b.WriteString("\n")
		for _, v := range r.Values {
			b.WriteString(renderDetailLine("", normalStyle.Render(v)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	hints := "esc: back • H: home"
	if rm.selectedRecord != nil {
		canEdit := rm.selectedRecord.AliasTarget == "" &&
			(rm.selectedRecord.Type == "A" || rm.selectedRecord.Type == "CNAME")
		canDelete := rm.selectedRecord.Type != "NS" && rm.selectedRecord.Type != "SOA"
		if canEdit {
			hints = "e: edit • " + hints
		}
		if canDelete {
			hints = "d: delete • " + hints
		}
		hints = "c: create • " + hints
	}
	b.WriteString(m.renderHelpBar(hints))
	return b.String()
}

// --- Create record form ---

var route53CreateFieldLabels = []string{"Name", "Type", "Value", "TTL"}
var route53TypeOptions = []string{"A", "CNAME"}

func (rm *route53Model) updateRecordCreate(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch rm.editField {
	case 0: // Name (text input)
		return rm.updateTextInput(m, key, screenRoute53RecordList)
	case 1: // Type (select)
		return rm.updateTypeSelect(m, key)
	case 2: // Value (text input)
		return rm.updateTextInput(m, key, screenRoute53RecordList)
	case 3: // TTL (text input, last field → execute)
		return rm.updateCreateTTL(m, key)
	}
	return *m, nil
}

func (rm *route53Model) updateTypeSelect(m *Model, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = screenRoute53RecordList
	case "up", "k":
		rm.editSelectIdx = previousListIndex(rm.editSelectIdx, len(route53TypeOptions))
	case "down", "j":
		rm.editSelectIdx = nextListIndex(rm.editSelectIdx, len(route53TypeOptions))
	case "enter":
		rm.editValues["type"] = route53TypeOptions[rm.editSelectIdx]
		rm.editField++
		rm.editInput = ""
	}
	return *m, nil
}

func (rm *route53Model) updateTextInput(m *Model, key string, cancelScreen screen) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = cancelScreen
	case "enter":
		if rm.editInput == "" {
			return *m, nil // required field
		}
		switch rm.editField {
		case 0:
			rm.editValues["name"] = rm.editInput
		case 2:
			rm.editValues["value"] = rm.editInput
		}
		rm.editField++
		rm.editInput = ""
		// Pre-fill TTL default
		if rm.editField == 3 {
			rm.editInput = "300"
		}
	case "backspace":
		if len(rm.editInput) > 0 {
			rm.editInput = rm.editInput[:len(rm.editInput)-1]
		}
	default:
		if len(key) == 1 {
			rm.editInput += key
		}
	}
	return *m, nil
}

func (rm *route53Model) updateCreateTTL(m *Model, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = screenRoute53RecordList
	case "enter":
		if rm.editInput == "" {
			rm.editInput = "300"
		}
		rm.editValues["ttl"] = rm.editInput
		// Execute create
		return m.startLoading(rm.executeCreate(*m))
	case "backspace":
		if len(rm.editInput) > 0 {
			rm.editInput = rm.editInput[:len(rm.editInput)-1]
		}
	default:
		if len(key) == 1 && key >= "0" && key <= "9" {
			rm.editInput += key
		}
	}
	return *m, nil
}

func (rm route53Model) viewRecordCreate(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Create DNS Record"))
	b.WriteString("\n\n")

	if rm.selectedZone != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Zone: %s (%s)", rm.selectedZone.Name, rm.selectedZone.ID)))
		b.WriteString("\n\n")
	}

	// Show completed fields
	for i := 0; i < rm.editField; i++ {
		label := route53CreateFieldLabels[i]
		val := ""
		switch i {
		case 0:
			val = rm.editValues["name"]
		case 1:
			val = rm.editValues["type"]
		case 2:
			val = rm.editValues["value"]
		case 3:
			val = rm.editValues["ttl"]
		}
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %s: %s", label, val)))
		b.WriteString("\n")
	}

	// Show current field
	if rm.editField < len(route53CreateFieldLabels) {
		label := route53CreateFieldLabels[rm.editField]
		b.WriteString("\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  %s:", label)))
		b.WriteString("\n")

		if rm.editField == 1 { // Type select
			for i, opt := range route53TypeOptions {
				cursor := "  "
				style := normalStyle
				if i == rm.editSelectIdx {
					cursor = "> "
					style = selectedStyle
				}
				b.WriteString(style.Render(fmt.Sprintf("  %s%s", cursor, opt)))
				b.WriteString("\n")
			}
		} else {
			b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", rm.editInput)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("enter: next • esc: cancel"))
	return b.String()
}

// --- Edit record form ---

func (rm *route53Model) updateRecordEdit(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch rm.editField {
	case 0: // Value (text input)
		switch key {
		case "esc":
			m.screen = screenRoute53RecordDetail
		case "enter":
			if rm.editInput == "" {
				return *m, nil
			}
			rm.editValues["value"] = rm.editInput
			rm.editField++
			rm.editInput = rm.editValues["ttl"]
		case "backspace":
			if len(rm.editInput) > 0 {
				rm.editInput = rm.editInput[:len(rm.editInput)-1]
			}
		default:
			if len(key) == 1 {
				rm.editInput += key
			}
		}
	case 1: // TTL (text input, last field → execute)
		switch key {
		case "esc":
			m.screen = screenRoute53RecordDetail
		case "enter":
			if rm.editInput == "" {
				return *m, nil
			}
			rm.editValues["ttl"] = rm.editInput
			return m.startLoading(rm.executeUpdate(*m))
		case "backspace":
			if len(rm.editInput) > 0 {
				rm.editInput = rm.editInput[:len(rm.editInput)-1]
			}
		default:
			if len(key) == 1 && key >= "0" && key <= "9" {
				rm.editInput += key
			}
		}
	}
	return *m, nil
}

func (rm route53Model) viewRecordEdit(m Model) string {
	if rm.selectedRecord == nil {
		return ""
	}
	r := rm.selectedRecord
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Edit DNS Record"))
	b.WriteString("\n\n")

	labelStyle := dimStyle.Width(10)
	b.WriteString("  " + labelStyle.Render("Name") + normalStyle.Render(r.Name))
	b.WriteString("\n")
	b.WriteString("  " + labelStyle.Render("Type") + filterStyle.Render(r.Type))
	b.WriteString("\n\n")

	editFields := []string{"Value", "TTL"}

	// Show completed edit fields
	for i := 0; i < rm.editField; i++ {
		label := editFields[i]
		val := ""
		switch i {
		case 0:
			val = rm.editValues["value"]
		case 1:
			val = rm.editValues["ttl"]
		}
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %s: %s", label, val)))
		b.WriteString("\n")
	}

	// Show current edit field
	if rm.editField < len(editFields) {
		label := editFields[rm.editField]
		b.WriteString("\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  %s:", label)))
		b.WriteString("\n")
		b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", rm.editInput)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("enter: next • esc: cancel"))
	return b.String()
}

// --- Delete record confirm ---

func (rm *route53Model) updateRecordDeleteConfirm(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if rm.selectedRecord == nil {
		m.screen = screenRoute53RecordList
		return *m, nil
	}
	confirmTarget := rm.selectedRecord.Name

	switch msg.String() {
	case "esc":
		m.screen = screenRoute53RecordDetail
	case "enter":
		if rm.confirmInput == confirmTarget {
			return m.startLoading(rm.executeDelete(*m))
		}
	case "backspace":
		if len(rm.confirmInput) > 0 {
			rm.confirmInput = rm.confirmInput[:len(rm.confirmInput)-1]
		}
	default:
		if runes := msg.Runes; len(runes) > 0 {
			rm.confirmInput += string(runes)
		}
	}
	return *m, nil
}

func (rm route53Model) viewRecordDeleteConfirm(m Model) string {
	if rm.selectedRecord == nil {
		return ""
	}
	r := rm.selectedRecord
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Delete DNS Record"))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render("  You are about to delete this record:"))
	b.WriteString("\n\n")

	labelStyle := dimStyle.Width(10)
	b.WriteString("  " + labelStyle.Render("Name") + selectedStyle.Render(r.Name))
	b.WriteString("\n")
	b.WriteString("  " + labelStyle.Render("Type") + filterStyle.Render(r.Type))
	b.WriteString("\n")
	if r.AliasTarget != "" {
		b.WriteString("  " + labelStyle.Render("Alias") + normalStyle.Render(r.AliasTarget))
	} else {
		b.WriteString("  " + labelStyle.Render("Value") + normalStyle.Render(strings.Join(r.Values, ", ")))
	}
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render("  Type the record name to confirm:"))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", rm.confirmInput)))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("enter: confirm • esc: cancel"))
	return b.String()
}

// --- Execution commands ---

func (rm route53Model) executeCreate(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return route53ActionDoneMsg{err: err}
			}
		}

		name := rm.editValues["name"]
		recordType := rm.editValues["type"]
		value := rm.editValues["value"]
		ttlStr := rm.editValues["ttl"]
		ttl := parseTTL(ttlStr)

		values := strings.Split(value, ",")
		for i := range values {
			values[i] = strings.TrimSpace(values[i])
		}

		zoneID := ""
		if rm.selectedZone != nil {
			zoneID = rm.selectedZone.ID
		}

		info, err := repo.CreateRecord(ctx, zoneID, name, recordType, values, ttl)
		if err != nil {
			return route53ActionDoneMsg{action: "create", err: err}
		}
		changeID := ""
		if info != nil {
			changeID = info.ID
		}
		return route53ActionDoneMsg{action: "create", changeID: changeID}
	}
}

func (rm route53Model) executeUpdate(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return route53ActionDoneMsg{err: err}
			}
		}

		if rm.selectedRecord == nil || rm.selectedZone == nil {
			return route53ActionDoneMsg{err: fmt.Errorf("no record selected")}
		}

		value := rm.editValues["value"]
		ttlStr := rm.editValues["ttl"]
		ttl := parseTTL(ttlStr)

		values := strings.Split(value, ",")
		for i := range values {
			values[i] = strings.TrimSpace(values[i])
		}

		info, err := repo.UpdateRecord(ctx, rm.selectedZone.ID, *rm.selectedRecord, values, ttl)
		if err != nil {
			return route53ActionDoneMsg{action: "edit", err: err}
		}
		changeID := ""
		if info != nil {
			changeID = info.ID
		}
		return route53ActionDoneMsg{action: "edit", changeID: changeID}
	}
}

func (rm route53Model) executeDelete(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return route53ActionDoneMsg{err: err}
			}
		}

		if rm.selectedRecord == nil || rm.selectedZone == nil {
			return route53ActionDoneMsg{err: fmt.Errorf("no record selected")}
		}

		info, err := repo.DeleteRecord(ctx, rm.selectedZone.ID, *rm.selectedRecord)
		if err != nil {
			return route53ActionDoneMsg{action: "delete", err: err}
		}
		changeID := ""
		if info != nil {
			changeID = info.ID
		}
		return route53ActionDoneMsg{action: "delete", changeID: changeID}
	}
}

// --- Change status polling ---

func (rm route53Model) pollChangeStatus(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return route53ChangeStatusMsg{err: err}
			}
		}

		status, err := repo.GetChangeStatus(ctx, rm.changeID)
		return route53ChangeStatusMsg{status: status, err: err}
	}
}

func (rm route53Model) tickPoll() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return route53PollTickMsg{}
	})
}

func parseTTL(s string) int64 {
	const defaultTTL int64 = 300
	if s == "" {
		return defaultTTL
	}
	var ttl int64
	if _, err := fmt.Sscanf(s, "%d", &ttl); err != nil || ttl < 0 {
		return defaultTTL
	}
	return ttl
}
