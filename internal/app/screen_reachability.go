package app

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

var reachabilityProtocols = []string{"TCP", "UDP"}

func (m Model) loadReachabilityTargets() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		reachabilityCfg := *m.cfg
		if strings.TrimSpace(m.reachabilityRegion) != "" {
			reachabilityCfg.Region = strings.TrimSpace(m.reachabilityRegion)
		}
		repo, err := awsservice.NewAwsRepository(ctx, &reachabilityCfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		targets, err := repo.ListReachabilityTargets(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(targets) == 0 {
			return errMsg{err: fmt.Errorf("no reachability analysis targets found")}
		}

		return reachabilityTargetsLoadedMsg{targets: targets}
	}
}

func (m Model) updateReachabilityRegionList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.reachabilityRegionFiltering {
		newFilter, deactivate, changed := handleFilterKey(key, m.reachabilityRegionFilter)
		m.reachabilityRegionFilter = newFilter
		if deactivate {
			m.reachabilityRegionFiltering = false
		}
		if changed {
			m.filteredReachabilityRegions = applyStringFilter(m.reachabilityRegions, m.reachabilityRegionFilter)
			m.reachabilityRegionIdx = 0
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
	case "up", "k":
		m.reachabilityRegionIdx = previousListIndex(m.reachabilityRegionIdx, len(m.filteredReachabilityRegions))
	case "down", "j":
		m.reachabilityRegionIdx = nextListIndex(m.reachabilityRegionIdx, len(m.filteredReachabilityRegions))
	case "/":
		m.reachabilityRegionFiltering = true
	case "enter":
		if len(m.filteredReachabilityRegions) == 0 {
			return m, nil
		}
		m.reachabilityRegion = m.filteredReachabilityRegions[m.reachabilityRegionIdx]
		m.reachabilityTargets = nil
		m.filteredReachabilityTargets = nil
		m.reachabilitySource = nil
		m.reachabilityDestination = nil
		m.reachabilityDestinationIP = ""
		m.reachabilityResult = nil
		m.reachabilityScrollOffset = 0
		m.reachabilityFilter = ""
		m.reachabilityFilterActive = false
		m.awsRepo = nil
		return m.startLoadingWithMessage(
			"Loading reachability targets...",
			[]string{
				fmt.Sprintf("Region: %s", m.activeReachabilityRegion()),
				"Collecting source and destination candidates for Reachability Analyzer.",
			},
			m.loadReachabilityTargets(),
		)
	}
	return m, nil
}

func (m Model) runReachabilityAnalysis() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		repo := m.awsRepo
		if repo == nil {
			var err error
			reachabilityCfg := *m.cfg
			if strings.TrimSpace(m.reachabilityRegion) != "" {
				reachabilityCfg.Region = strings.TrimSpace(m.reachabilityRegion)
			}
			repo, err = awsservice.NewAwsRepository(ctx, &reachabilityCfg)
			if err != nil {
				return errMsg{err: err}
			}
		}

		port, err := strconv.Atoi(strings.TrimSpace(m.reachabilityPortInput))
		if err != nil || port <= 0 || port > 65535 {
			return errMsg{err: fmt.Errorf("destination port must be between 1 and 65535")}
		}

		if m.reachabilitySource == nil {
			return errMsg{err: fmt.Errorf("source is required")}
		}

		var destination awsservice.ReachabilityTarget
		destinationIP := strings.TrimSpace(m.reachabilityDestinationIP)
		if m.reachabilityDestination != nil && !m.reachabilityDestination.ManualIP {
			destination = *m.reachabilityDestination
			destinationIP = ""
		}
		if destination.ID == "" && destinationIP == "" {
			return errMsg{err: fmt.Errorf("destination is required")}
		}
		if destinationIP != "" {
			ip := net.ParseIP(destinationIP)
			if ip == nil || ip.To4() == nil {
				return errMsg{err: fmt.Errorf("destination IPv4 must be a valid IPv4 address")}
			}
		}

		result, err := repo.RunReachabilityAnalysis(
			ctx,
			*m.reachabilitySource,
			destination,
			destinationIP,
			reachabilityProtocols[m.reachabilityProtocolIdx],
			int32(port),
		)
		if err != nil {
			return errMsg{err: err}
		}
		return reachabilityAnalysisLoadedMsg{result: result}
	}
}

func (m Model) updateReachabilitySourceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.reachabilityFilterActive {
		newFilter, deactivate, changed := handleFilterKey(key, m.reachabilityFilter)
		m.reachabilityFilter = newFilter
		if deactivate {
			m.reachabilityFilterActive = false
		}
		if changed {
			m.filteredReachabilityTargets = applyReachabilityTargetFilter(m.reachabilityTargets, m.selectedReachabilitySourceType(), m.reachabilityFilter)
			m.reachabilityIdx = 0
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenReachabilityRegionList
	case "left", "h":
		if m.reachabilitySourceTypeIdx > 0 {
			m.reachabilitySourceTypeIdx--
			m.filteredReachabilityTargets = applyReachabilityTargetFilter(m.reachabilityTargets, m.selectedReachabilitySourceType(), m.reachabilityFilter)
			m.reachabilityIdx = 0
		}
	case "right", "l", "tab":
		if m.reachabilitySourceTypeIdx < len(m.reachabilitySourceTypes)-1 {
			m.reachabilitySourceTypeIdx++
			m.filteredReachabilityTargets = applyReachabilityTargetFilter(m.reachabilityTargets, m.selectedReachabilitySourceType(), m.reachabilityFilter)
			m.reachabilityIdx = 0
		}
	case "up", "k":
		m.reachabilityIdx = previousListIndex(m.reachabilityIdx, len(m.filteredReachabilityTargets))
	case "down", "j":
		m.reachabilityIdx = nextListIndex(m.reachabilityIdx, len(m.filteredReachabilityTargets))
	case "/":
		m.reachabilityFilterActive = true
	case "r":
		return m.startLoadingWithMessage(
			"Refreshing reachability targets...",
			[]string{
				fmt.Sprintf("Region: %s", m.activeReachabilityRegion()),
			},
			m.loadReachabilityTargets(),
		)
	case "enter":
		if len(m.filteredReachabilityTargets) == 0 {
			return m, nil
		}
		selected := m.filteredReachabilityTargets[m.reachabilityIdx]
		m.reachabilitySource = &selected
		m.reachabilityDestination = nil
		m.reachabilityDestinationIP = ""
		m.reachabilityFilter = ""
		m.reachabilityFilterActive = false
		m.reachabilityDestTypes = buildReachabilityTargetTypes(m.reachabilityTargets, true)
		m.reachabilityDestTypeIdx = 0
		m.filteredReachabilityTargets = applyReachabilityTargetFilter(reachabilityDestinationCandidates(m.reachabilityTargets), m.selectedReachabilityDestinationType(), "")
		m.reachabilityIdx = 0
		m.screen = screenReachabilityDestinationList
	}
	return m, nil
}

func (m Model) updateReachabilityDestinationList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.reachabilityFilterActive {
		newFilter, deactivate, changed := handleFilterKey(key, m.reachabilityFilter)
		m.reachabilityFilter = newFilter
		if deactivate {
			m.reachabilityFilterActive = false
		}
		if changed {
			m.filteredReachabilityTargets = applyReachabilityTargetFilter(reachabilityDestinationCandidates(m.reachabilityTargets), m.selectedReachabilityDestinationType(), m.reachabilityFilter)
			m.reachabilityIdx = 0
		}
		return m, nil
	}

	switch key {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.filteredReachabilityTargets = applyReachabilityTargetFilter(m.reachabilityTargets, m.selectedReachabilitySourceType(), "")
		m.reachabilityIdx = 0
		m.reachabilityFilter = ""
		m.reachabilityFilterActive = false
		m.screen = screenReachabilitySourceList
	case "left", "h":
		if m.reachabilityDestTypeIdx > 0 {
			m.reachabilityDestTypeIdx--
			m.filteredReachabilityTargets = applyReachabilityTargetFilter(reachabilityDestinationCandidates(m.reachabilityTargets), m.selectedReachabilityDestinationType(), m.reachabilityFilter)
			m.reachabilityIdx = 0
		}
	case "right", "l", "tab":
		if m.reachabilityDestTypeIdx < len(m.reachabilityDestTypes)-1 {
			m.reachabilityDestTypeIdx++
			m.filteredReachabilityTargets = applyReachabilityTargetFilter(reachabilityDestinationCandidates(m.reachabilityTargets), m.selectedReachabilityDestinationType(), m.reachabilityFilter)
			m.reachabilityIdx = 0
		}
	case "up", "k":
		m.reachabilityIdx = previousListIndex(m.reachabilityIdx, len(m.filteredReachabilityTargets))
	case "down", "j":
		m.reachabilityIdx = nextListIndex(m.reachabilityIdx, len(m.filteredReachabilityTargets))
	case "/":
		m.reachabilityFilterActive = true
	case "r":
		return m.startLoadingWithMessage(
			"Refreshing reachability targets...",
			[]string{
				fmt.Sprintf("Region: %s", m.activeReachabilityRegion()),
			},
			m.loadReachabilityTargets(),
		)
	case "enter":
		if len(m.filteredReachabilityTargets) == 0 {
			return m, nil
		}
		selected := m.filteredReachabilityTargets[m.reachabilityIdx]
		m.reachabilityDestination = &selected
		if !selected.ManualIP {
			m.reachabilityDestinationIP = ""
		}
		m.reachabilityProtocolIdx = 0
		m.reachabilityPortInput = "443"
		m.reachabilityConfigField = 0
		m.screen = screenReachabilityConfig
	}
	return m, nil
}

func (m Model) updateReachabilityConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenReachabilityDestinationList
	case "up", "k":
		maxField := 1
		if m.reachabilityDestination != nil && m.reachabilityDestination.ManualIP {
			maxField = 2
		}
		m.reachabilityConfigField = previousListIndex(m.reachabilityConfigField, maxField+1)
	case "down", "j", "tab":
		maxField := 1
		if m.reachabilityDestination != nil && m.reachabilityDestination.ManualIP {
			maxField = 2
		}
		m.reachabilityConfigField = nextListIndex(m.reachabilityConfigField, maxField+1)
	case "left", "h":
		if m.reachabilityConfigField == 0 && m.reachabilityProtocolIdx > 0 {
			m.reachabilityProtocolIdx--
		}
	case "right", "l":
		if m.reachabilityConfigField == 0 && m.reachabilityProtocolIdx < len(reachabilityProtocols)-1 {
			m.reachabilityProtocolIdx++
		}
	case "backspace":
		switch m.reachabilityConfigField {
		case 1:
			if len(m.reachabilityPortInput) > 0 {
				m.reachabilityPortInput = m.reachabilityPortInput[:len(m.reachabilityPortInput)-1]
			}
		case 2:
			if len(m.reachabilityDestinationIP) > 0 {
				m.reachabilityDestinationIP = m.reachabilityDestinationIP[:len(m.reachabilityDestinationIP)-1]
			}
		}
	case "enter":
		if m.reachabilityConfigField == 0 {
			maxField := 1
			if m.reachabilityDestination != nil && m.reachabilityDestination.ManualIP {
				maxField = 2
			}
			if m.reachabilityConfigField < maxField {
				m.reachabilityConfigField++
				return m, nil
			}
		}
		return m.startLoadingWithMessage(
			"Finding Network Path",
			m.reachabilityLoadingDetails(),
			m.runReachabilityAnalysis(),
		)
	default:
		if len(msg.String()) == 1 {
			switch m.reachabilityConfigField {
			case 1:
				if msg.String()[0] >= '0' && msg.String()[0] <= '9' {
					m.reachabilityPortInput += msg.String()
				}
			case 2:
				if strings.ContainsRune("0123456789.", rune(msg.String()[0])) {
					m.reachabilityDestinationIP += msg.String()
				}
			}
		}
	}
	return m, nil
}

func (m Model) updateReachabilityResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenReachabilityConfig
	case "r":
		return m.startLoadingWithMessage(
			"Finding Network Path",
			m.reachabilityLoadingDetails(),
			m.runReachabilityAnalysis(),
		)
	case "up", "k":
		if m.reachabilityScrollOffset > 0 {
			m.reachabilityScrollOffset--
		}
	case "down", "j":
		lines := len(m.reachabilityResultLines())
		visible := max(m.height-8, 5)
		if m.reachabilityScrollOffset < max(lines-visible, 0) {
			m.reachabilityScrollOffset++
		}
	}
	return m, nil
}

func (m Model) viewReachabilitySourceList() string {
	return m.viewReachabilityTargetList("Reachability Analyzer > Source", fmt.Sprintf("Region: %s. Supported source types: EC2 instances, Internet gateways, Network interfaces, Transit gateways, Transit gateway attachments, Virtual private gateways, VPC endpoint services, VPC endpoints, and VPC peering connections.", m.activeReachabilityRegion()), m.filteredReachabilityTargets, m.reachabilitySourceTypes, m.reachabilitySourceTypeIdx, "←/→ or tab: type • ↑/↓: navigate • /: filter • r: refresh • enter: select • esc: back • H: home")
}

func (m Model) viewReachabilityDestinationList() string {
	return m.viewReachabilityTargetList("Reachability Analyzer > Destination", fmt.Sprintf("Region: %s. Supported destination types: EC2 instances, Internet gateways, Network interfaces, Transit gateways, Transit gateway attachments, Virtual private gateways, VPC endpoint services, VPC endpoints, VPC peering connections, and IP addresses.", m.activeReachabilityRegion()), m.filteredReachabilityTargets, m.reachabilityDestTypes, m.reachabilityDestTypeIdx, "←/→ or tab: type • ↑/↓: navigate • /: filter • r: refresh • enter: select • esc: back • H: home")
}

func (m Model) viewReachabilityRegionList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Reachability Analyzer > Region"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Start with the region you want to inspect. The current context region is preselected."))
	b.WriteString("\n")
	if m.reachabilityRegionFiltering {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.reachabilityRegionFilter)))
	} else if m.reachabilityRegionFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.reachabilityRegionFilter)))
	}
	b.WriteString("\n\n")

	if len(m.filteredReachabilityRegions) == 0 {
		panel.WriteString(dimStyle.Render("  No matching regions"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-12, 5)
		start := 0
		if m.reachabilityRegionIdx >= visibleLines {
			start = m.reachabilityRegionIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredReachabilityRegions))
		for i := start; i < end; i++ {
			region := m.filteredReachabilityRegions[i]
			cursor := "  "
			style := normalStyle
			if i == m.reachabilityRegionIdx {
				cursor = "> "
				style = selectedStyle
			}
			label := region
			if region == m.cfg.Region {
				label += " [context default]"
			}
			panel.WriteString(style.Render(cursor + label))
			panel.WriteString("\n")
		}
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: load targets • esc: back • H: home"))
	return b.String()
}

func (m Model) viewReachabilityTargetList(title, subtitle string, items []awsservice.ReachabilityTarget, typeOptions []string, typeIdx int, footer string) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(subtitle))
	b.WriteString("\n")
	if len(typeOptions) > 0 {
		b.WriteString(m.renderReachabilityTypeSelector(typeOptions, typeIdx))
		b.WriteString("\n")
	}
	if m.reachabilityFilterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.reachabilityFilter)))
	} else if m.reachabilityFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.reachabilityFilter)))
	}
	b.WriteString("\n\n")

	if len(items) == 0 {
		panel.WriteString(dimStyle.Render("  No matching targets"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-12, 5)
		start := 0
		if m.reachabilityIdx >= visibleLines {
			start = m.reachabilityIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(items))

		for i := start; i < end; i++ {
			item := items[i]
			cursor := "  "
			style := normalStyle
			if i == m.reachabilityIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, item.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d targets", len(items))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(footer))
	return b.String()
}

func (m Model) viewReachabilityConfig() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Reachability Analyzer > Analysis Settings"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Region      : " + m.activeReachabilityRegion()))
	b.WriteString("\n")

	source := ""
	if m.reachabilitySource != nil {
		source = m.reachabilitySource.DisplayTitle()
	}
	destination := ""
	if m.reachabilityDestination != nil {
		destination = m.reachabilityDestination.DisplayTitle()
	}

	b.WriteString(normalStyle.Render("  Source      : " + source))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  Destination : " + destination))
	b.WriteString("\n\n")

	protocol := reachabilityProtocols[m.reachabilityProtocolIdx]
	if m.reachabilityConfigField == 0 {
		b.WriteString(selectedStyle.Render("  Protocol    : " + protocol))
	} else {
		b.WriteString(normalStyle.Render("  Protocol    : " + protocol))
	}
	b.WriteString("\n")

	portValue := m.reachabilityPortInput
	if portValue == "" {
		portValue = "443"
	}
	if m.reachabilityConfigField == 1 {
		b.WriteString(selectedStyle.Render("  Dest Port   : " + portValue + "▏"))
	} else {
		b.WriteString(normalStyle.Render("  Dest Port   : " + portValue))
	}
	b.WriteString("\n")

	if m.reachabilityDestination != nil && m.reachabilityDestination.ManualIP {
		ipValue := m.reachabilityDestinationIP
		if m.reachabilityConfigField == 2 {
			b.WriteString(selectedStyle.Render("  Dest IPv4   : " + ipValue + "▏"))
		} else {
			b.WriteString(normalStyle.Render("  Dest IPv4   : " + ipValue))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Protocol and destination port are part of the path intent. Reachability Analyzer evaluates the shortest matching path and identifies blockers when traffic is not reachable."))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓ or tab: field • ←/→: protocol • type: edit • enter: analyze • esc: back • H: home"))
	return b.String()
}

func (m Model) viewReachabilityResult() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Reachability Analyzer > Result"))
	b.WriteString("\n\n")

	lines := m.reachabilityResultLines()
	visibleLines := max(m.height-8, 5)
	start := min(m.reachabilityScrollOffset, max(len(lines)-visibleLines, 0))
	end := min(start+visibleLines, len(lines))
	for _, line := range lines[start:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}
	if len(lines) > 0 {
		b.WriteString("\n")
	}
	b.WriteString(m.renderHelpBar("j/k: scroll • r: rerun • esc: back • H: home"))
	return b.String()
}

func (m Model) reachabilityResultLines() []string {
	if m.reachabilityResult == nil {
		return []string{dimStyle.Render("No analysis result")}
	}

	r := m.reachabilityResult
	lines := make([]string, 0, 64)
	status := "Not reachable"
	statusStyle := errorStyle
	if r.NetworkPathFound {
		status = "Reachable"
		statusStyle = successStyle
	}
	lines = append(lines, titleStyle.Render("Summary"))
	lines = append(lines, statusStyle.Render("  "+status))
	lines = append(lines, dimStyle.Render("  Analysis status : "+r.Status))
	lines = append(lines, dimStyle.Render("  Region          : "+m.activeReachabilityRegion()))
	if r.StatusMessage != "" {
		lines = append(lines, normalStyle.Render("  Message         : "+r.StatusMessage))
	}
	lines = append(lines, normalStyle.Render("  Source          : "+r.Source.DisplayTitle()))
	if r.DestinationIP != "" {
		lines = append(lines, normalStyle.Render("  Destination     : "+r.DestinationIP))
	} else {
		lines = append(lines, normalStyle.Render("  Destination     : "+r.Destination.DisplayTitle()))
	}
	lines = append(lines, normalStyle.Render(fmt.Sprintf("  Intent          : %s/%d", strings.ToUpper(r.Protocol), r.DestinationPort)))
	if r.WarningMessage != "" {
		lines = append(lines, errorStyle.Render("  Warning         : "+r.WarningMessage))
	}

	lines = append(lines, "")
	lines = append(lines, titleStyle.Render("Path"))
	if len(r.ForwardPath) == 0 {
		lines = append(lines, dimStyle.Render("  No path components returned"))
	} else {
		for i, component := range r.ForwardPath {
			lines = append(lines, pathNodeStyle.Render(fmt.Sprintf("  ● %s", component.Title)))
			for _, detail := range component.Details {
				lines = append(lines, infoStyle.Render("  │   "+detail))
			}
			for _, explanation := range component.Explanations {
				lines = append(lines, warningStyle.Render("  │   blocker: "+explanation))
			}
			if i < len(r.ForwardPath)-1 {
				lines = append(lines, pathLineStyle.Render("  │"))
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, titleStyle.Render("Findings"))
	if len(r.Explanations) == 0 {
		lines = append(lines, dimStyle.Render("  No blocker explanations returned"))
	} else {
		for idx, explanation := range r.Explanations {
			lines = append(lines, warningStyle.Render(fmt.Sprintf("  %d) %s", idx+1, explanation.Summary)))
			for _, detail := range explanation.Details {
				lines = append(lines, dimStyle.Render("     - "+detail))
			}
			lines = append(lines, "")
		}
		if lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	}

	return lines
}

func manualReachabilityDestination() awsservice.ReachabilityTarget {
	return awsservice.ReachabilityTarget{
		Name:     "Manual IP address",
		Type:     "IP addresses",
		ManualIP: true,
	}
}

func reachabilityDestinationCandidates(targets []awsservice.ReachabilityTarget) []awsservice.ReachabilityTarget {
	return append([]awsservice.ReachabilityTarget{manualReachabilityDestination()}, targets...)
}

func buildReachabilityTargetTypes(targets []awsservice.ReachabilityTarget, includeManual bool) []string {
	seen := map[string]struct{}{}
	ordered := []string{
		"EC2 instances",
		"Internet gateways",
		"Network interfaces",
		"Transit gateways",
		"Transit gateway attachments",
		"Virtual private gateways",
		"VPC endpoint services",
		"VPC endpoints",
		"VPC peering connections",
	}
	types := make([]string, 0, len(ordered)+1)
	for _, target := range targets {
		if strings.TrimSpace(target.Type) != "" {
			seen[target.Type] = struct{}{}
		}
	}
	for _, candidate := range ordered {
		if _, ok := seen[candidate]; ok {
			types = append(types, candidate)
		}
	}
	if includeManual {
		types = append(types, "IP addresses")
	}
	return types
}

func applyReachabilityTargetFilter(items []awsservice.ReachabilityTarget, targetType, query string) []awsservice.ReachabilityTarget {
	filtered := items
	if targetType != "" {
		filtered = make([]awsservice.ReachabilityTarget, 0, len(items))
		for _, item := range items {
			if item.Type == targetType {
				filtered = append(filtered, item)
			}
		}
	}
	return applyFilter(filtered, query)
}

func (m Model) selectedReachabilitySourceType() string {
	if len(m.reachabilitySourceTypes) == 0 {
		return ""
	}
	return m.reachabilitySourceTypes[m.reachabilitySourceTypeIdx]
}

func (m Model) selectedReachabilityDestinationType() string {
	if len(m.reachabilityDestTypes) == 0 {
		return ""
	}
	return m.reachabilityDestTypes[m.reachabilityDestTypeIdx]
}

func (m Model) renderReachabilityTypeSelector(options []string, selected int) string {
	var parts []string
	for i, option := range options {
		label := "[" + option + "]"
		if i == selected {
			parts = append(parts, selectedStyle.Render(label))
		} else {
			parts = append(parts, dimStyle.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

func (m Model) activeReachabilityRegion() string {
	if strings.TrimSpace(m.reachabilityRegion) != "" {
		return m.reachabilityRegion
	}
	return m.cfg.Region
}

func availableReachabilityRegions(current string) []string {
	regions := []string{
		"af-south-1",
		"ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
		"ap-south-1", "ap-south-2",
		"ap-southeast-1", "ap-southeast-2", "ap-southeast-3", "ap-southeast-4",
		"ca-central-1",
		"eu-central-1", "eu-central-2",
		"eu-north-1",
		"eu-south-1", "eu-south-2",
		"eu-west-1", "eu-west-2", "eu-west-3",
		"me-central-1", "me-south-1",
		"sa-east-1",
		"us-east-1", "us-east-2",
		"us-west-1", "us-west-2",
	}
	current = strings.TrimSpace(current)
	if current != "" && !slices.Contains(regions, current) {
		regions = append(regions, current)
		slices.Sort(regions)
	}
	return regions
}

func applyStringFilter(items []string, filter string) []string {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return append([]string(nil), items...)
	}
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(item), filter) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func indexOfString(items []string, target string) int {
	for i, item := range items {
		if item == target {
			return i
		}
	}
	return -1
}

func (m Model) reachabilityLoadingDetails() []string {
	source := "source pending"
	if m.reachabilitySource != nil {
		source = m.reachabilitySource.DisplayTitle()
	}

	destination := "destination pending"
	if strings.TrimSpace(m.reachabilityDestinationIP) != "" {
		destination = strings.TrimSpace(m.reachabilityDestinationIP)
	} else if m.reachabilityDestination != nil {
		destination = m.reachabilityDestination.DisplayTitle()
	}

	protocol := ""
	if m.reachabilityProtocolIdx >= 0 && m.reachabilityProtocolIdx < len(reachabilityProtocols) {
		protocol = reachabilityProtocols[m.reachabilityProtocolIdx]
	}

	intent := protocol
	if strings.TrimSpace(m.reachabilityPortInput) != "" {
		intent = fmt.Sprintf("%s/%s", protocol, strings.TrimSpace(m.reachabilityPortInput))
	}

	source = truncateReachabilityLoadingLabel(source, m.width)
	destination = truncateReachabilityLoadingLabel(destination, m.width)

	return []string{
		dimStyle.Render(fmt.Sprintf("Region: %s", m.activeReachabilityRegion())),
		pathNodeStyle.Render(source),
		pathLineStyle.Render("  │"),
		pathLineStyle.Render("  ↓"),
		pathNodeStyle.Render(destination),
		infoStyle.Render(fmt.Sprintf("Intent: %s", intent)),
	}
}

func truncateReachabilityLoadingLabel(label string, width int) string {
	if width <= 0 {
		return label
	}

	maxWidth := width - 6
	if maxWidth < 24 {
		maxWidth = 24
	}

	runes := []rune(label)
	if len(runes) <= maxWidth {
		return label
	}
	if maxWidth <= 1 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-1]) + "…"
}
