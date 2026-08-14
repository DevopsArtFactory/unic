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

type reachabilityModel struct {
	regions         []string
	filteredRegions []string
	region          string
	regionIdx       int
	regionFilter    string
	regionFiltering bool
	targets         []awsservice.ReachabilityTarget
	filteredTargets []awsservice.ReachabilityTarget
	sourceTypes     []string
	sourceTypeIdx   int
	destTypes       []string
	destTypeIdx     int
	idx             int
	filter          string
	filterActive    bool
	source          *awsservice.ReachabilityTarget
	destination     *awsservice.ReachabilityTarget
	destinationIP   string
	protocolIdx     int
	portInput       string
	configField     int
	result          *awsservice.ReachabilityAnalysisResult
	scrollOffset    int
}

func newReachabilityModel() reachabilityModel {
	return reachabilityModel{}
}

func (rm *reachabilityModel) Start(m *Model) (tea.Model, tea.Cmd) {
	rm.regions = availableReachabilityRegions(m.cfg.Region)
	rm.filteredRegions = rm.regions
	rm.region = m.cfg.Region
	rm.regionIdx = indexOfString(rm.regions, rm.region)
	if rm.regionIdx < 0 {
		rm.regionIdx = 0
	}
	rm.regionFilter = ""
	rm.regionFiltering = false
	rm.targets = nil
	rm.filteredTargets = nil
	rm.source = nil
	rm.destination = nil
	rm.destinationIP = ""
	rm.result = nil
	rm.scrollOffset = 0
	m.awsRepo = nil
	m.screen = screenReachabilityRegionList
	return *m, nil
}

func (rm *reachabilityModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case reachabilityTargetsLoadedMsg:
		rm.targets = msg.targets
		rm.sourceTypes = buildReachabilityTargetTypes(msg.targets, false)
		rm.sourceTypeIdx = 0
		rm.destTypes = nil
		rm.destTypeIdx = 0
		rm.filteredTargets = applyReachabilityTargetFilter(msg.targets, rm.selectedSourceType(), "")
		rm.idx = 0
		rm.filter = ""
		rm.filterActive = false
		rm.source = nil
		rm.destination = nil
		rm.destinationIP = ""
		rm.protocolIdx = 0
		rm.portInput = "443"
		rm.configField = 0
		rm.result = nil
		rm.scrollOffset = 0
		m.screen = screenReachabilitySourceList
		return *m, nil, true
	case reachabilityAnalysisLoadedMsg:
		rm.result = msg.result
		rm.scrollOffset = 0
		m.screen = screenReachabilityResult
		return *m, nil, true
	}
	return *m, nil, false
}

func (rm *reachabilityModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenReachabilityRegionList:
		newM, cmd := rm.updateRegionList(m, msg)
		return newM, cmd, true
	case screenReachabilitySourceList:
		newM, cmd := rm.updateSourceList(m, msg)
		return newM, cmd, true
	case screenReachabilityDestinationList:
		newM, cmd := rm.updateDestinationList(m, msg)
		return newM, cmd, true
	case screenReachabilityConfig:
		newM, cmd := rm.updateConfig(m, msg)
		return newM, cmd, true
	case screenReachabilityResult:
		newM, cmd := rm.updateResult(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (rm reachabilityModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenReachabilityRegionList:
		return rm.viewRegionList(m), true
	case screenReachabilitySourceList:
		return rm.viewSourceList(m), true
	case screenReachabilityDestinationList:
		return rm.viewDestinationList(m), true
	case screenReachabilityConfig:
		return rm.viewConfig(m), true
	case screenReachabilityResult:
		return rm.viewResult(m), true
	default:
		return "", false
	}
}

func (rm *reachabilityModel) ApplyFilter(_ *Model, _ filterTarget) bool {
	return false
}

func (rm reachabilityModel) activeRegion(m Model) string {
	if strings.TrimSpace(rm.region) != "" {
		return rm.region
	}
	return m.cfg.Region
}

func (rm reachabilityModel) loadReachabilityTargets(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		reachabilityCfg := *m.cfg
		if strings.TrimSpace(rm.region) != "" {
			reachabilityCfg.Region = strings.TrimSpace(rm.region)
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

func (rm *reachabilityModel) updateRegionList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if rm.regionFiltering {
		newFilter, deactivate, changed := handleFilterKey(key, rm.regionFilter)
		rm.regionFilter = newFilter
		if deactivate {
			rm.regionFiltering = false
		}
		if changed {
			rm.filteredRegions = applyStringFilter(rm.regions, rm.regionFilter)
			rm.regionIdx = 0
		}
		return *m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
	case "up", "k":
		rm.regionIdx = previousListIndex(rm.regionIdx, len(rm.filteredRegions))
	case "down", "j":
		rm.regionIdx = nextListIndex(rm.regionIdx, len(rm.filteredRegions))
	case "/":
		rm.regionFiltering = true
	case "enter":
		if len(rm.filteredRegions) == 0 {
			return *m, nil
		}
		rm.region = rm.filteredRegions[rm.regionIdx]
		rm.targets = nil
		rm.filteredTargets = nil
		rm.source = nil
		rm.destination = nil
		rm.destinationIP = ""
		rm.result = nil
		rm.scrollOffset = 0
		rm.filter = ""
		rm.filterActive = false
		m.awsRepo = nil
		return m.startLoadingWithMessage(
			"Loading reachability targets...",
			[]string{
				fmt.Sprintf("Region: %s", m.activeReachabilityRegion()),
				"Collecting source and destination candidates for Reachability Analyzer.",
			},
			rm.loadReachabilityTargets(*m),
		)
	}
	return *m, nil
}

func (rm reachabilityModel) runAnalysis(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.commandContext(), 45*time.Second)
		defer cancel()
		repo := m.awsRepo
		if repo == nil {
			var err error
			reachabilityCfg := *m.cfg
			if strings.TrimSpace(rm.region) != "" {
				reachabilityCfg.Region = strings.TrimSpace(rm.region)
			}
			repo, err = awsservice.NewAwsRepository(ctx, &reachabilityCfg)
			if err != nil {
				return errMsg{err: err}
			}
		}

		port, err := strconv.Atoi(strings.TrimSpace(rm.portInput))
		if err != nil || port <= 0 || port > 65535 {
			return errMsg{err: fmt.Errorf("destination port must be between 1 and 65535")}
		}

		if rm.source == nil {
			return errMsg{err: fmt.Errorf("source is required")}
		}

		var destination awsservice.ReachabilityTarget
		destinationIP := strings.TrimSpace(rm.destinationIP)
		if rm.destination != nil && !rm.destination.ManualIP {
			destination = *rm.destination
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
			*rm.source,
			destination,
			destinationIP,
			reachabilityProtocols[rm.protocolIdx],
			int32(port),
		)
		if err != nil {
			return errMsg{err: err}
		}
		return reachabilityAnalysisLoadedMsg{result: result}
	}
}

func (rm *reachabilityModel) updateSourceList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if rm.filterActive {
		newFilter, deactivate, changed := handleFilterKey(key, rm.filter)
		rm.filter = newFilter
		if deactivate {
			rm.filterActive = false
		}
		if changed {
			rm.filteredTargets = applyReachabilityTargetFilter(rm.targets, rm.selectedSourceType(), rm.filter)
			rm.idx = 0
		}
		return *m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenReachabilityRegionList
	case "left", "h":
		if rm.sourceTypeIdx > 0 {
			rm.sourceTypeIdx--
			rm.filteredTargets = applyReachabilityTargetFilter(rm.targets, rm.selectedSourceType(), rm.filter)
			rm.idx = 0
		}
	case "right", "l", "tab":
		if rm.sourceTypeIdx < len(rm.sourceTypes)-1 {
			rm.sourceTypeIdx++
			rm.filteredTargets = applyReachabilityTargetFilter(rm.targets, rm.selectedSourceType(), rm.filter)
			rm.idx = 0
		}
	case "up", "k":
		rm.idx = previousListIndex(rm.idx, len(rm.filteredTargets))
	case "down", "j":
		rm.idx = nextListIndex(rm.idx, len(rm.filteredTargets))
	case "/":
		rm.filterActive = true
	case "r":
		return m.startLoadingWithMessage(
			"Refreshing reachability targets...",
			[]string{
				fmt.Sprintf("Region: %s", m.activeReachabilityRegion()),
			},
			rm.loadReachabilityTargets(*m),
		)
	case "enter":
		if len(rm.filteredTargets) == 0 {
			return *m, nil
		}
		selected := rm.filteredTargets[rm.idx]
		rm.source = &selected
		rm.destination = nil
		rm.destinationIP = ""
		rm.filter = ""
		rm.filterActive = false
		rm.destTypes = buildReachabilityTargetTypes(rm.targets, true)
		rm.destTypeIdx = 0
		rm.filteredTargets = applyReachabilityTargetFilter(reachabilityDestinationCandidates(rm.targets), rm.selectedDestinationType(), "")
		rm.idx = 0
		m.screen = screenReachabilityDestinationList
	}
	return *m, nil
}

func (rm *reachabilityModel) updateDestinationList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if rm.filterActive {
		newFilter, deactivate, changed := handleFilterKey(key, rm.filter)
		rm.filter = newFilter
		if deactivate {
			rm.filterActive = false
		}
		if changed {
			rm.filteredTargets = applyReachabilityTargetFilter(reachabilityDestinationCandidates(rm.targets), rm.selectedDestinationType(), rm.filter)
			rm.idx = 0
		}
		return *m, nil
	}

	switch key {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		rm.filteredTargets = applyReachabilityTargetFilter(rm.targets, rm.selectedSourceType(), "")
		rm.idx = 0
		rm.filter = ""
		rm.filterActive = false
		m.screen = screenReachabilitySourceList
	case "left", "h":
		if rm.destTypeIdx > 0 {
			rm.destTypeIdx--
			rm.filteredTargets = applyReachabilityTargetFilter(reachabilityDestinationCandidates(rm.targets), rm.selectedDestinationType(), rm.filter)
			rm.idx = 0
		}
	case "right", "l", "tab":
		if rm.destTypeIdx < len(rm.destTypes)-1 {
			rm.destTypeIdx++
			rm.filteredTargets = applyReachabilityTargetFilter(reachabilityDestinationCandidates(rm.targets), rm.selectedDestinationType(), rm.filter)
			rm.idx = 0
		}
	case "up", "k":
		rm.idx = previousListIndex(rm.idx, len(rm.filteredTargets))
	case "down", "j":
		rm.idx = nextListIndex(rm.idx, len(rm.filteredTargets))
	case "/":
		rm.filterActive = true
	case "r":
		return m.startLoadingWithMessage(
			"Refreshing reachability targets...",
			[]string{
				fmt.Sprintf("Region: %s", m.activeReachabilityRegion()),
			},
			rm.loadReachabilityTargets(*m),
		)
	case "enter":
		if len(rm.filteredTargets) == 0 {
			return *m, nil
		}
		selected := rm.filteredTargets[rm.idx]
		rm.destination = &selected
		if !selected.ManualIP {
			rm.destinationIP = ""
		}
		rm.protocolIdx = 0
		rm.portInput = "443"
		rm.configField = 0
		m.screen = screenReachabilityConfig
	}
	return *m, nil
}

func (rm *reachabilityModel) updateConfig(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenReachabilityDestinationList
	case "up", "k":
		rm.configField = previousListIndex(rm.configField, rm.configMaxField()+1)
	case "down", "j", "tab":
		rm.configField = nextListIndex(rm.configField, rm.configMaxField()+1)
	case "left", "h":
		if rm.configField == 0 && rm.protocolIdx > 0 {
			rm.protocolIdx--
		}
	case "right", "l":
		if rm.configField == 0 && rm.protocolIdx < len(reachabilityProtocols)-1 {
			rm.protocolIdx++
		}
	case "backspace":
		switch rm.configField {
		case 1:
			rm.portInput = trimLastRune(rm.portInput)
		case 2:
			rm.destinationIP = trimLastRune(rm.destinationIP)
		}
	case "enter":
		if rm.configField == 0 && rm.configField < rm.configMaxField() {
			rm.configField++
			return *m, nil
		}
		return m.startLoadingWithMessage(
			"Finding Network Path",
			rm.loadingDetails(*m),
			rm.runAnalysis(*m),
		)
	default:
		if len(key) != 1 {
			break
		}
		switch rm.configField {
		case 1:
			if key[0] >= '0' && key[0] <= '9' {
				rm.portInput += key
			}
		case 2:
			if strings.ContainsRune("0123456789.", rune(key[0])) {
				rm.destinationIP += key
			}
		}
	}
	return *m, nil
}

// configMaxField returns the last selectable config field index; manual-IP
// destinations expose an extra destination-IP field.
func (rm *reachabilityModel) configMaxField() int {
	if rm.destination != nil && rm.destination.ManualIP {
		return 2
	}
	return 1
}

func (rm *reachabilityModel) updateResult(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenReachabilityConfig
	case "r":
		return m.startLoadingWithMessage(
			"Finding Network Path",
			rm.loadingDetails(*m),
			rm.runAnalysis(*m),
		)
	case "up", "k":
		if rm.scrollOffset > 0 {
			rm.scrollOffset--
		}
	case "down", "j":
		lines := len(rm.resultLines(*m))
		visible := max(m.height-8, 5)
		if rm.scrollOffset < max(lines-visible, 0) {
			rm.scrollOffset++
		}
	}
	return *m, nil
}

func (rm reachabilityModel) viewSourceList(m Model) string {
	return rm.viewTargetList(m, "Reachability Analyzer > Source", fmt.Sprintf("Region: %s. Supported source types: EC2 instances, Internet gateways, Network interfaces, Transit gateways, Transit gateway attachments, Virtual private gateways, VPC endpoint services, VPC endpoints, and VPC peering connections.", m.activeReachabilityRegion()), rm.filteredTargets, rm.sourceTypes, rm.sourceTypeIdx, "←/→ or tab: type • ↑/↓: navigate • /: filter • r: refresh • enter: select • esc: back • H: home")
}

func (rm reachabilityModel) viewDestinationList(m Model) string {
	return rm.viewTargetList(m, "Reachability Analyzer > Destination", fmt.Sprintf("Region: %s. Supported destination types: EC2 instances, Internet gateways, Network interfaces, Transit gateways, Transit gateway attachments, Virtual private gateways, VPC endpoint services, VPC endpoints, VPC peering connections, and IP addresses.", m.activeReachabilityRegion()), rm.filteredTargets, rm.destTypes, rm.destTypeIdx, "←/→ or tab: type • ↑/↓: navigate • /: filter • r: refresh • enter: select • esc: back • H: home")
}

func (rm reachabilityModel) viewRegionList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Reachability Analyzer > Region"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Start with the region you want to inspect. The current context region is preselected."))
	b.WriteString("\n")
	if rm.regionFiltering {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", rm.regionFilter)))
	} else if rm.regionFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", rm.regionFilter)))
	}
	b.WriteString("\n\n")

	if len(rm.filteredRegions) == 0 {
		panel.WriteString(dimStyle.Render("  No matching regions"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-12, 5)
		start := 0
		if rm.regionIdx >= visibleLines {
			start = rm.regionIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(rm.filteredRegions))
		for i := start; i < end; i++ {
			region := rm.filteredRegions[i]
			cursor := "  "
			style := normalStyle
			if i == rm.regionIdx {
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

func (rm reachabilityModel) viewTargetList(m Model, title, subtitle string, items []awsservice.ReachabilityTarget, typeOptions []string, typeIdx int, footer string) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(subtitle))
	b.WriteString("\n")
	if len(typeOptions) > 0 {
		b.WriteString(rm.renderTypeSelector(typeOptions, typeIdx))
		b.WriteString("\n")
	}
	if rm.filterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", rm.filter)))
	} else if rm.filter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", rm.filter)))
	}
	b.WriteString("\n\n")

	if len(items) == 0 {
		panel.WriteString(dimStyle.Render("  No matching targets"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-12, 5)
		start := 0
		if rm.idx >= visibleLines {
			start = rm.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(items))

		for i := start; i < end; i++ {
			item := items[i]
			cursor := "  "
			style := normalStyle
			if i == rm.idx {
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

func (rm reachabilityModel) viewConfig(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Reachability Analyzer > Analysis Settings"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Region      : " + m.activeReachabilityRegion()))
	b.WriteString("\n")

	source := ""
	if rm.source != nil {
		source = rm.source.DisplayTitle()
	}
	destination := ""
	if rm.destination != nil {
		destination = rm.destination.DisplayTitle()
	}

	b.WriteString(normalStyle.Render("  Source      : " + source))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  Destination : " + destination))
	b.WriteString("\n\n")

	protocol := reachabilityProtocols[rm.protocolIdx]
	if rm.configField == 0 {
		b.WriteString(selectedStyle.Render("  Protocol    : " + protocol))
	} else {
		b.WriteString(normalStyle.Render("  Protocol    : " + protocol))
	}
	b.WriteString("\n")

	portValue := rm.portInput
	if portValue == "" {
		portValue = "443"
	}
	if rm.configField == 1 {
		b.WriteString(selectedStyle.Render("  Dest Port   : " + portValue + "▏"))
	} else {
		b.WriteString(normalStyle.Render("  Dest Port   : " + portValue))
	}
	b.WriteString("\n")

	if rm.destination != nil && rm.destination.ManualIP {
		ipValue := rm.destinationIP
		if rm.configField == 2 {
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

func (rm reachabilityModel) viewResult(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Reachability Analyzer > Result"))
	b.WriteString("\n\n")

	lines := rm.resultLines(m)
	visibleLines := max(m.height-8, 5)
	start := min(rm.scrollOffset, max(len(lines)-visibleLines, 0))
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

func (rm reachabilityModel) resultLines(m Model) []string {
	if rm.result == nil {
		return []string{dimStyle.Render("No analysis result")}
	}

	r := rm.result
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

func (rm reachabilityModel) selectedSourceType() string {
	if len(rm.sourceTypes) == 0 {
		return ""
	}
	return rm.sourceTypes[rm.sourceTypeIdx]
}

func (rm reachabilityModel) selectedDestinationType() string {
	if len(rm.destTypes) == 0 {
		return ""
	}
	return rm.destTypes[rm.destTypeIdx]
}

func (rm reachabilityModel) renderTypeSelector(options []string, selected int) string {
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
	return m.reachability.activeRegion(m)
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

func (rm reachabilityModel) loadingDetails(m Model) []string {
	source := "source pending"
	if rm.source != nil {
		source = rm.source.DisplayTitle()
	}

	destination := "destination pending"
	if strings.TrimSpace(rm.destinationIP) != "" {
		destination = strings.TrimSpace(rm.destinationIP)
	} else if rm.destination != nil {
		destination = rm.destination.DisplayTitle()
	}

	protocol := ""
	if rm.protocolIdx >= 0 && rm.protocolIdx < len(reachabilityProtocols) {
		protocol = reachabilityProtocols[rm.protocolIdx]
	}

	intent := protocol
	if strings.TrimSpace(rm.portInput) != "" {
		intent = fmt.Sprintf("%s/%s", protocol, strings.TrimSpace(rm.portInput))
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
