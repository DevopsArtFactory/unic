package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsservice "unic/internal/services/aws"
)

func (m Model) updateRoute53ZoneList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.route53ZoneFilterActive {
		switch key {
		case "esc":
			m.route53ZoneFilterActive = false
		case "enter":
			m.route53ZoneFilterActive = false
		case "backspace":
			if len(m.route53ZoneFilter) > 0 {
				m.route53ZoneFilter = m.route53ZoneFilter[:len(m.route53ZoneFilter)-1]
				m.applyRoute53ZoneFilter()
			}
		default:
			if len(key) == 1 {
				m.route53ZoneFilter += key
				m.applyRoute53ZoneFilter()
			}
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.route53ZoneFilter = ""
		m.filteredRoute53Zones = m.route53Zones
		m.route53ZoneIdx = 0
	case "up", "k":
		if m.route53ZoneIdx > 0 {
			m.route53ZoneIdx--
		}
	case "down", "j":
		if m.route53ZoneIdx < len(m.filteredRoute53Zones)-1 {
			m.route53ZoneIdx++
		}
	case "/":
		m.route53ZoneFilterActive = true
	case "enter":
		if len(m.filteredRoute53Zones) > 0 && m.route53ZoneIdx < len(m.filteredRoute53Zones) {
			selected := m.filteredRoute53Zones[m.route53ZoneIdx]
			m.selectedRoute53Zone = &selected
			m.screen = screenLoading
			return m, m.loadRoute53Records(selected.ID)
		}
	}
	return m, nil
}

func (m Model) updateRoute53RecordList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.route53RecordFilterActive {
		switch key {
		case "esc":
			m.route53RecordFilterActive = false
		case "enter":
			m.route53RecordFilterActive = false
		case "backspace":
			if len(m.route53RecordFilter) > 0 {
				m.route53RecordFilter = m.route53RecordFilter[:len(m.route53RecordFilter)-1]
				m.applyRoute53RecordFilter()
			}
		default:
			if len(key) == 1 {
				m.route53RecordFilter += key
				m.applyRoute53RecordFilter()
			}
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenRoute53ZoneList
		m.route53RecordFilter = ""
		m.filteredRoute53Records = m.route53Records
		m.route53RecordIdx = 0
	case "up", "k":
		if m.route53RecordIdx > 0 {
			m.route53RecordIdx--
		}
	case "down", "j":
		if m.route53RecordIdx < len(m.filteredRoute53Records)-1 {
			m.route53RecordIdx++
		}
	case "/":
		m.route53RecordFilterActive = true
	case "c":
		m.route53Action = "create"
		m.route53EditField = 0
		m.route53EditValues = map[string]string{}
		m.route53EditInput = ""
		m.route53EditSelectIdx = 0
		m.screen = screenRoute53RecordCreate
	case "enter":
		if len(m.filteredRoute53Records) > 0 && m.route53RecordIdx < len(m.filteredRoute53Records) {
			selected := m.filteredRoute53Records[m.route53RecordIdx]
			m.selectedRoute53Record = &selected
			m.screen = screenRoute53RecordDetail
		}
	}
	return m, nil
}

func (m Model) updateRoute53RecordDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenRoute53RecordList
	case "e":
		// Edit only for A/CNAME non-alias records
		if m.selectedRoute53Record != nil && m.selectedRoute53Record.AliasTarget == "" &&
			(m.selectedRoute53Record.Type == "A" || m.selectedRoute53Record.Type == "CNAME") {
			m.route53Action = "edit"
			m.route53EditField = 0
			m.route53EditValues = map[string]string{
				"value": strings.Join(m.selectedRoute53Record.Values, ","),
				"ttl":   fmt.Sprintf("%d", m.selectedRoute53Record.TTL),
			}
			m.route53EditInput = strings.Join(m.selectedRoute53Record.Values, ",")
			m.screen = screenRoute53RecordEdit
		}
	case "d":
		// Delete allowed for non-NS/SOA records
		if m.selectedRoute53Record != nil &&
			m.selectedRoute53Record.Type != "NS" && m.selectedRoute53Record.Type != "SOA" {
			m.route53Action = "delete"
			m.route53ConfirmInput = ""
			m.screen = screenRoute53RecordDeleteConfirm
		}
	case "c":
		m.route53Action = "create"
		m.route53EditField = 0
		m.route53EditValues = map[string]string{}
		m.route53EditInput = ""
		m.route53EditSelectIdx = 0
		m.screen = screenRoute53RecordCreate
	}
	return m, nil
}

func (m *Model) applyRoute53ZoneFilter() {
	if m.route53ZoneFilter == "" {
		m.filteredRoute53Zones = m.route53Zones
	} else {
		query := strings.ToLower(m.route53ZoneFilter)
		var result []awsservice.HostedZone
		for _, zone := range m.route53Zones {
			if strings.Contains(zone.FilterText(), query) {
				result = append(result, zone)
			}
		}
		m.filteredRoute53Zones = result
	}
	m.route53ZoneIdx = 0
}

func (m *Model) applyRoute53RecordFilter() {
	if m.route53RecordFilter == "" {
		m.filteredRoute53Records = m.route53Records
	} else {
		query := strings.ToLower(m.route53RecordFilter)
		var result []awsservice.DNSRecord
		for _, rec := range m.route53Records {
			if strings.Contains(rec.FilterText(), query) {
				result = append(result, rec)
			}
		}
		m.filteredRoute53Records = result
	}
	m.route53RecordIdx = 0
}

func (m Model) loadRoute53Zones() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
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

func (m Model) loadRoute53Records(zoneID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
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

func (m Model) viewRoute53ZoneList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Route53 Hosted Zones"))
	b.WriteString("\n")

	if m.route53ZoneFilterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.route53ZoneFilter)))
	} else if m.route53ZoneFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.route53ZoneFilter)))
	}
	b.WriteString("\n\n")

	if len(m.filteredRoute53Zones) == 0 {
		b.WriteString(dimStyle.Render("  No matching hosted zones"))
		b.WriteString("\n")
	} else {
		// Measure max widths for column alignment
		maxName, maxID := 4, 2 // "NAME", "ID"
		for _, zone := range m.filteredRoute53Zones {
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
		b.WriteString(dimStyle.Render("  " + nameCol.Render("NAME") + idCol.Render("ID") + recordsCol.Render("RECORDS") + "TYPE"))
		b.WriteString("\n")

		visibleLines := max(m.height-9, 5)
		start := 0
		if m.route53ZoneIdx >= visibleLines {
			start = m.route53ZoneIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredRoute53Zones))

		for i := start; i < end; i++ {
			zone := m.filteredRoute53Zones[i]
			cursor := "  "
			style := normalStyle
			if i == m.route53ZoneIdx {
				cursor = "> "
				style = selectedStyle
			}
			zoneType := "Public"
			if zone.IsPrivate {
				zoneType = "Private"
			}
			row := cursor +
				nameCol.Inherit(style).Render(zone.Name) +
				idCol.Inherit(dimStyle).Render(zone.ID) +
				recordsCol.Inherit(dimStyle).Render(fmt.Sprintf("%d", zone.ResourceRecordCount)) +
				dimStyle.Render(zoneType)
			b.WriteString(row)
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d zones", len(m.filteredRoute53Zones), len(m.route53Zones))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • enter: records • esc: back • H: home"))
	return b.String()
}

func (m Model) viewRoute53RecordList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	zoneName := ""
	if m.selectedRoute53Zone != nil {
		zoneName = m.selectedRoute53Zone.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("DNS Records — %s", zoneName)))
	b.WriteString("\n")

	if m.route53RecordFilterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.route53RecordFilter)))
	} else if m.route53RecordFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.route53RecordFilter)))
	}
	b.WriteString("\n\n")

	if len(m.filteredRoute53Records) == 0 {
		b.WriteString(dimStyle.Render("  No matching records"))
		b.WriteString("\n")
	} else {
		// Measure max widths for column alignment
		maxName, maxType := 4, 4 // "NAME", "TYPE"
		for _, rec := range m.filteredRoute53Records {
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
		b.WriteString(dimStyle.Render("  " + nameCol.Render("NAME") + typeCol.Render("TYPE") + "VALUE"))
		b.WriteString("\n")

		visibleLines := max(m.height-9, 5)
		start := 0
		if m.route53RecordIdx >= visibleLines {
			start = m.route53RecordIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredRoute53Records))

		for i := start; i < end; i++ {
			rec := m.filteredRoute53Records[i]
			cursor := "  "
			style := normalStyle
			if i == m.route53RecordIdx {
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
				nameCol.Inherit(style).Render(rec.Name) +
				typeCol.Inherit(filterStyle).Render(rec.Type) +
				dimStyle.Render(valStr)
			b.WriteString(row)
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d records", len(m.filteredRoute53Records), len(m.route53Records))))
	}

	// Show change status if polling
	if m.route53Polling {
		b.WriteString(filterStyle.Render(fmt.Sprintf("  Change: %s...", m.route53ChangeStatus)))
		b.WriteString("\n")
	} else if m.route53ChangeStatus == "INSYNC" {
		b.WriteString(dimStyle.Render("  Change: INSYNC"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • c: create • enter: detail • esc: back • H: home"))
	return b.String()
}

func (m Model) viewRoute53RecordDetail() string {
	if m.selectedRoute53Record == nil {
		return ""
	}
	r := m.selectedRoute53Record
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("DNS Record Detail"))
	b.WriteString("\n\n")

	labelStyle := dimStyle.Width(10)

	b.WriteString("  " + labelStyle.Render("Name") + normalStyle.Render(r.Name))
	b.WriteString("\n")
	b.WriteString("  " + labelStyle.Render("Type") + filterStyle.Render(r.Type))
	b.WriteString("\n")

	if r.AliasTarget != "" {
		b.WriteString("  " + labelStyle.Render("Alias") + normalStyle.Render(r.AliasTarget))
		b.WriteString("\n")
	} else {
		b.WriteString("  " + labelStyle.Render("TTL") + normalStyle.Render(fmt.Sprintf("%d", r.TTL)))
		b.WriteString("\n")
	}

	if len(r.Values) > 0 {
		b.WriteString("  " + labelStyle.Render("Values"))
		b.WriteString("\n")
		for _, v := range r.Values {
			b.WriteString("  " + labelStyle.Render("") + normalStyle.Render(v))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	hints := "esc: back • H: home"
	if m.selectedRoute53Record != nil {
		canEdit := m.selectedRoute53Record.AliasTarget == "" &&
			(m.selectedRoute53Record.Type == "A" || m.selectedRoute53Record.Type == "CNAME")
		canDelete := m.selectedRoute53Record.Type != "NS" && m.selectedRoute53Record.Type != "SOA"
		if canEdit {
			hints = "e: edit • " + hints
		}
		if canDelete {
			hints = "d: delete • " + hints
		}
		hints = "c: create • " + hints
	}
	b.WriteString(dimStyle.Render(hints))
	return b.String()
}

// --- Create record form ---

var route53CreateFieldLabels = []string{"Name", "Type", "Value", "TTL"}
var route53TypeOptions = []string{"A", "CNAME"}

func (m Model) updateRoute53RecordCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.route53EditField {
	case 0: // Name (text input)
		return m.updateRoute53TextInput(key, screenRoute53RecordList)
	case 1: // Type (select)
		return m.updateRoute53TypeSelect(key)
	case 2: // Value (text input)
		return m.updateRoute53TextInput(key, screenRoute53RecordList)
	case 3: // TTL (text input, last field → execute)
		return m.updateRoute53CreateTTL(key)
	}
	return m, nil
}

func (m Model) updateRoute53TypeSelect(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = screenRoute53RecordList
	case "up", "k":
		if m.route53EditSelectIdx > 0 {
			m.route53EditSelectIdx--
		}
	case "down", "j":
		if m.route53EditSelectIdx < len(route53TypeOptions)-1 {
			m.route53EditSelectIdx++
		}
	case "enter":
		m.route53EditValues["type"] = route53TypeOptions[m.route53EditSelectIdx]
		m.route53EditField++
		m.route53EditInput = ""
	}
	return m, nil
}

func (m Model) updateRoute53TextInput(key string, cancelScreen screen) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = cancelScreen
	case "enter":
		if m.route53EditInput == "" {
			return m, nil // required field
		}
		switch m.route53EditField {
		case 0:
			m.route53EditValues["name"] = m.route53EditInput
		case 2:
			m.route53EditValues["value"] = m.route53EditInput
		}
		m.route53EditField++
		m.route53EditInput = ""
		// Pre-fill TTL default
		if m.route53EditField == 3 {
			m.route53EditInput = "300"
		}
	case "backspace":
		if len(m.route53EditInput) > 0 {
			m.route53EditInput = m.route53EditInput[:len(m.route53EditInput)-1]
		}
	default:
		if len(key) == 1 {
			m.route53EditInput += key
		}
	}
	return m, nil
}

func (m Model) updateRoute53CreateTTL(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = screenRoute53RecordList
	case "enter":
		if m.route53EditInput == "" {
			m.route53EditInput = "300"
		}
		m.route53EditValues["ttl"] = m.route53EditInput
		// Execute create
		m.screen = screenLoading
		return m, m.executeRoute53Create()
	case "backspace":
		if len(m.route53EditInput) > 0 {
			m.route53EditInput = m.route53EditInput[:len(m.route53EditInput)-1]
		}
	default:
		if len(key) == 1 && key >= "0" && key <= "9" {
			m.route53EditInput += key
		}
	}
	return m, nil
}

func (m Model) viewRoute53RecordCreate() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Create DNS Record"))
	b.WriteString("\n\n")

	if m.selectedRoute53Zone != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Zone: %s (%s)", m.selectedRoute53Zone.Name, m.selectedRoute53Zone.ID)))
		b.WriteString("\n\n")
	}

	// Show completed fields
	for i := 0; i < m.route53EditField; i++ {
		label := route53CreateFieldLabels[i]
		val := ""
		switch i {
		case 0:
			val = m.route53EditValues["name"]
		case 1:
			val = m.route53EditValues["type"]
		case 2:
			val = m.route53EditValues["value"]
		case 3:
			val = m.route53EditValues["ttl"]
		}
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %s: %s", label, val)))
		b.WriteString("\n")
	}

	// Show current field
	if m.route53EditField < len(route53CreateFieldLabels) {
		label := route53CreateFieldLabels[m.route53EditField]
		b.WriteString("\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  %s:", label)))
		b.WriteString("\n")

		if m.route53EditField == 1 { // Type select
			for i, opt := range route53TypeOptions {
				cursor := "  "
				style := normalStyle
				if i == m.route53EditSelectIdx {
					cursor = "> "
					style = selectedStyle
				}
				b.WriteString(style.Render(fmt.Sprintf("  %s%s", cursor, opt)))
				b.WriteString("\n")
			}
		} else {
			b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", m.route53EditInput)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("enter: next • esc: cancel"))
	return b.String()
}

// --- Edit record form ---

func (m Model) updateRoute53RecordEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.route53EditField {
	case 0: // Value (text input)
		switch key {
		case "esc":
			m.screen = screenRoute53RecordDetail
		case "enter":
			if m.route53EditInput == "" {
				return m, nil
			}
			m.route53EditValues["value"] = m.route53EditInput
			m.route53EditField++
			m.route53EditInput = m.route53EditValues["ttl"]
		case "backspace":
			if len(m.route53EditInput) > 0 {
				m.route53EditInput = m.route53EditInput[:len(m.route53EditInput)-1]
			}
		default:
			if len(key) == 1 {
				m.route53EditInput += key
			}
		}
	case 1: // TTL (text input, last field → execute)
		switch key {
		case "esc":
			m.screen = screenRoute53RecordDetail
		case "enter":
			if m.route53EditInput == "" {
				return m, nil
			}
			m.route53EditValues["ttl"] = m.route53EditInput
			m.screen = screenLoading
			return m, m.executeRoute53Update()
		case "backspace":
			if len(m.route53EditInput) > 0 {
				m.route53EditInput = m.route53EditInput[:len(m.route53EditInput)-1]
			}
		default:
			if len(key) == 1 && key >= "0" && key <= "9" {
				m.route53EditInput += key
			}
		}
	}
	return m, nil
}

func (m Model) viewRoute53RecordEdit() string {
	if m.selectedRoute53Record == nil {
		return ""
	}
	r := m.selectedRoute53Record
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
	for i := 0; i < m.route53EditField; i++ {
		label := editFields[i]
		val := ""
		switch i {
		case 0:
			val = m.route53EditValues["value"]
		case 1:
			val = m.route53EditValues["ttl"]
		}
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %s: %s", label, val)))
		b.WriteString("\n")
	}

	// Show current edit field
	if m.route53EditField < len(editFields) {
		label := editFields[m.route53EditField]
		b.WriteString("\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  %s:", label)))
		b.WriteString("\n")
		b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", m.route53EditInput)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("enter: next • esc: cancel"))
	return b.String()
}

// --- Delete record confirm ---

func (m Model) updateRoute53RecordDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.selectedRoute53Record == nil {
		m.screen = screenRoute53RecordList
		return m, nil
	}
	confirmTarget := m.selectedRoute53Record.Name

	switch msg.String() {
	case "esc":
		m.screen = screenRoute53RecordDetail
	case "enter":
		if m.route53ConfirmInput == confirmTarget {
			m.screen = screenLoading
			return m, m.executeRoute53Delete()
		}
	case "backspace":
		if len(m.route53ConfirmInput) > 0 {
			m.route53ConfirmInput = m.route53ConfirmInput[:len(m.route53ConfirmInput)-1]
		}
	default:
		if runes := msg.Runes; len(runes) > 0 {
			m.route53ConfirmInput += string(runes)
		}
	}
	return m, nil
}

func (m Model) viewRoute53RecordDeleteConfirm() string {
	if m.selectedRoute53Record == nil {
		return ""
	}
	r := m.selectedRoute53Record
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
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", m.route53ConfirmInput)))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("enter: confirm • esc: cancel"))
	return b.String()
}

// --- Execution commands ---

func (m Model) executeRoute53Create() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return route53ActionDoneMsg{err: err}
			}
		}

		name := m.route53EditValues["name"]
		recordType := m.route53EditValues["type"]
		value := m.route53EditValues["value"]
		ttlStr := m.route53EditValues["ttl"]
		ttl := parseTTL(ttlStr)

		values := strings.Split(value, ",")
		for i := range values {
			values[i] = strings.TrimSpace(values[i])
		}

		zoneID := ""
		if m.selectedRoute53Zone != nil {
			zoneID = m.selectedRoute53Zone.ID
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

func (m Model) executeRoute53Update() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return route53ActionDoneMsg{err: err}
			}
		}

		if m.selectedRoute53Record == nil || m.selectedRoute53Zone == nil {
			return route53ActionDoneMsg{err: fmt.Errorf("no record selected")}
		}

		value := m.route53EditValues["value"]
		ttlStr := m.route53EditValues["ttl"]
		ttl := parseTTL(ttlStr)

		values := strings.Split(value, ",")
		for i := range values {
			values[i] = strings.TrimSpace(values[i])
		}

		info, err := repo.UpdateRecord(ctx, m.selectedRoute53Zone.ID, *m.selectedRoute53Record, values, ttl)
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

func (m Model) executeRoute53Delete() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return route53ActionDoneMsg{err: err}
			}
		}

		if m.selectedRoute53Record == nil || m.selectedRoute53Zone == nil {
			return route53ActionDoneMsg{err: fmt.Errorf("no record selected")}
		}

		info, err := repo.DeleteRecord(ctx, m.selectedRoute53Zone.ID, *m.selectedRoute53Record)
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

func (m Model) pollRoute53ChangeStatus() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return route53ChangeStatusMsg{err: err}
			}
		}

		status, err := repo.GetChangeStatus(ctx, m.route53ChangeID)
		return route53ChangeStatusMsg{status: status, err: err}
	}
}

func (m Model) tickRoute53Poll() tea.Cmd {
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
