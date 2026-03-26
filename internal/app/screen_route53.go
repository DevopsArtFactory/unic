package app

import (
	"context"
	"fmt"
	"strings"

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

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • enter: detail • esc: back • H: home"))
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
	b.WriteString(dimStyle.Render("esc: back • H: home"))
	return b.String()
}
