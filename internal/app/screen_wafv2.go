package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsservice "unic/internal/services/aws"
)

type wafModel struct {
	acls         []awsservice.WAFWebACL
	filtered     []awsservice.WAFWebACL
	idx          int
	selected     *awsservice.WAFWebACL
	detailScroll int
	warnings     []error
	loadFailed   bool
}

func newWAFModel() wafModel { return wafModel{} }

func isWAFScreen(value screen) bool {
	return value == screenWAFWebACLList || value == screenWAFWebACLDetail
}

func resetWAFContextState(m *Model) {
	m.waf = newWAFModel()
	m.resetFilter(filterWAFWebACLs)
}

func (wm *wafModel) Start(m *Model) (tea.Model, tea.Cmd) {
	wm.loadFailed = false
	return m.startLoadingFor(screenWAFWebACLList, "Loading WAFv2 web ACLs...", []string{"Regional: " + m.cfg.Region, "CloudFront: us-east-1 endpoint"}, wm.load(*m))
}

func (wm wafModel) load(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return wafWebACLsLoadedMsg{err: err}
		}
		acls, warnings, err := repo.ListWAFWebACLs(ctx)
		if err != nil {
			return wafWebACLsLoadedMsg{err: err}
		}
		return wafWebACLsLoadedMsg{acls: acls, warnings: warnings}
	}
}

func (wm *wafModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	loaded, ok := msg.(wafWebACLsLoadedMsg)
	if !ok {
		return *m, nil, false
	}
	if loaded.err != nil {
		wm.loadFailed = true
		m.errMsg = loaded.err.Error()
		m.loadingTitle = ""
		m.loadingDetails = nil
		finishWAFLoad(m, screenError)
		return *m, nil, true
	}
	wm.loadFailed = false
	wm.acls = loaded.acls
	wm.warnings = loaded.warnings
	wm.filtered = applyFilter(wm.acls, m.filterValue(filterWAFWebACLs))
	wm.idx, wm.selected, wm.detailScroll = 0, nil, 0
	finishWAFLoad(m, screenWAFWebACLList)
	return *m, nil, true
}

func (wm *wafModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenWAFWebACLList:
		if cmd, handled := m.updateSharedFilter(msg, filterWAFWebACLs); handled {
			return *m, cmd, true
		}
		switch msg.String() {
		case "q", "esc":
			m.resetFilter(filterWAFWebACLs)
			m.screen = screenFeatureList
		case "up", "k":
			wm.idx = previousListIndex(wm.idx, len(wm.filtered))
		case "down", "j":
			wm.idx = nextListIndex(wm.idx, len(wm.filtered))
		case "/":
			return *m, m.activateFilter(filterWAFWebACLs), true
		case "r":
			newM, cmd := wm.Start(m)
			return newM, cmd, true
		case "enter":
			if wm.idx < len(wm.filtered) {
				selected := wm.filtered[wm.idx]
				wm.selected = &selected
				wm.detailScroll = 0
				m.screen = screenWAFWebACLDetail
			}
		}
		return *m, nil, true
	case screenWAFWebACLDetail:
		visibleLines := max(m.height-8, 5)
		maxOffset := max(len(wm.detailLines(*m))-visibleLines, 0)
		switch msg.String() {
		case "q", "esc":
			wm.detailScroll = 0
			m.screen = screenWAFWebACLList
		case "up", "k":
			wm.detailScroll = max(wm.detailScroll-1, 0)
		case "down", "j":
			wm.detailScroll = min(wm.detailScroll+1, maxOffset)
		case "pgup":
			wm.detailScroll = max(wm.detailScroll-visibleLines, 0)
		case "pgdown":
			wm.detailScroll = min(wm.detailScroll+visibleLines, maxOffset)
		}
		return *m, nil, true
	}
	return *m, nil, false
}

func (wm wafModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenWAFWebACLList:
		return wm.viewList(m), true
	case screenWAFWebACLDetail:
		return wm.viewDetail(m), true
	default:
		return "", false
	}
}

func (wm *wafModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterWAFWebACLs {
		return false
	}
	wm.filtered = applyFilter(wm.acls, m.filterValue(target))
	wm.idx = 0
	return true
}

func (wm wafModel) viewList(m Model) string {
	var b, panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("WAFv2 Web ACLs — regional + CloudFront"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterWAFWebACLs))
	b.WriteString("\n")
	warningLines := 0
	if len(wm.warnings) > 0 {
		b.WriteString(m.renderWarningSummary(len(wm.warnings), "scope or detail lookup failures", wm.warnings[0].Error()))
		warningLines = 2
	}
	b.WriteString("\n")
	if len(wm.filtered) == 0 {
		message := "  No WAF web ACLs found"
		if len(wm.acls) > 0 {
			message = "  No matching WAF web ACLs"
		}
		panel.WriteString(dimStyle.Render(message + "\n"))
	} else {
		panel.WriteString(dimStyle.Render("  " + wafListHeader(m)))
		panel.WriteString("\n")
		visibleLines := max(m.height-11-warningLines, 5)
		start := max(wm.idx-visibleLines+1, 0)
		for i := start; i < min(start+visibleLines, len(wm.filtered)); i++ {
			cursor, style := "  ", normalStyle
			if len(wm.filtered[i].Signals()) > 0 {
				style = warningStyle
			}
			if i == wm.idx {
				cursor, style = "> ", selectedStyle
			}
			row := wafListRow(m, wm.filtered[i])
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterWAFWebACLs, row)))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d web ACLs; posture flags are informational", len(wm.filtered), len(wm.acls))))
	}
	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func wafListHeader(m Model) string {
	nameWidth := wafNameColumnWidth(m)
	if m.width > 0 && m.width < 128 {
		return lipgloss.NewStyle().Width(nameWidth).Render("NAME") + " " +
			lipgloss.NewStyle().Width(11).Render("SCOPE") + " " +
			lipgloss.NewStyle().MaxWidth(wafSignalColumnWidth(m)).Render("SIGNALS")
	}
	return lipgloss.NewStyle().Width(nameWidth).Render("NAME") + " " +
		lipgloss.NewStyle().Width(11).Render("SCOPE") + " " +
		lipgloss.NewStyle().Width(8).Render("DEFAULT") + " " +
		lipgloss.NewStyle().Width(5).Render("WCU") + " " +
		lipgloss.NewStyle().Width(7).Render("RULES") + " " +
		lipgloss.NewStyle().Width(8).Render("LOGGING") + " " +
		lipgloss.NewStyle().Width(10).Render("RESOURCES") + " SIGNALS"
}

func wafListRow(m Model, acl awsservice.WAFWebACL) string {
	nameWidth := wafNameColumnWidth(m)
	name := lipgloss.NewStyle().Width(nameWidth).MaxWidth(nameWidth).Render(escapeTerminalControls(acl.Name))
	scope := lipgloss.NewStyle().Width(11).MaxWidth(11).Render(acl.Scope)
	if m.width > 0 && m.width < 128 {
		return name + " " + scope + " " + lipgloss.NewStyle().MaxWidth(wafSignalColumnWidth(m)).Render(escapeTerminalControls(acl.SignalLabel()))
	}
	defaultAction := acl.DefaultAction
	capacity := fmt.Sprintf("%d", acl.Capacity)
	rules := fmt.Sprintf("%d/%d", acl.ManagedRuleCount(), len(acl.Rules))
	if !acl.DetailKnown {
		defaultAction = "unknown"
		capacity = "-"
		rules = "-"
	}
	return name + " " + scope + " " +
		lipgloss.NewStyle().Width(8).MaxWidth(8).Render(defaultAction) + " " +
		lipgloss.NewStyle().Width(5).MaxWidth(5).Render(capacity) + " " +
		lipgloss.NewStyle().Width(7).MaxWidth(7).Render(rules) + " " +
		lipgloss.NewStyle().Width(8).MaxWidth(8).Render(acl.LoggingLabel()) + " " +
		lipgloss.NewStyle().Width(10).MaxWidth(10).Render(acl.ResourceCountLabel()) + " " + escapeTerminalControls(acl.SignalLabel())
}

func wafNameColumnWidth(m Model) int {
	if m.width <= 0 {
		return 28
	}
	if m.width < 128 {
		return max(min(m.width-51, 36), 10)
	}
	return max(min(m.width-86, 32), 12)
}

func wafSignalColumnWidth(m Model) int {
	if m.width <= 0 {
		return 34
	}
	return max(m.width-wafNameColumnWidth(m)-17, 8)
}

func (wm wafModel) viewDetail(m Model) string {
	if wm.selected == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	titleWidth := m.width
	if titleWidth <= 0 {
		titleWidth = 80
	}
	title := "WAFv2 Web ACL — " + escapeTerminalControls(wm.selected.Name)
	b.WriteString(titleStyle.Render(truncateEC2DetailValue(title, titleWidth)))
	b.WriteString("\n\n")
	lines := wm.detailLines(m)
	visibleLines := max(m.height-8, 5)
	start := min(wm.detailScroll, max(len(lines)-visibleLines, 0))
	for _, line := range lines[start:min(start+visibleLines, len(lines))] {
		b.WriteString(line)
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (wm wafModel) detailLines(m Model) []string {
	acl := wm.selected
	if acl == nil {
		return nil
	}
	managed := acl.ManagedRuleCount()
	defaultAction := acl.DefaultAction
	capacity := fmt.Sprintf("%d WCU", acl.Capacity)
	rules := fmt.Sprintf("%d total, %d managed, %d custom", len(acl.Rules), managed, len(acl.Rules)-managed)
	if !acl.DetailKnown {
		defaultAction, capacity, rules = "unknown", "unknown", "unknown"
	}
	lines := []string{
		m.renderEC2DetailLine("Name", acl.Name),
		m.renderEC2DetailLine("Scope", acl.Scope),
		m.renderEC2DetailLine("Region", acl.Region),
		m.renderEC2DetailLine("Default Action", defaultAction),
		m.renderEC2DetailLine("Capacity", capacity),
		m.renderEC2DetailLine("Rules", rules),
		m.renderEC2DetailLine("Logging", acl.LoggingLabel()),
		m.renderEC2DetailLine("Posture", acl.SignalLabel()),
		m.renderEC2DetailLine("ARN", acl.ARN),
		m.renderEC2DetailLine("Description", acl.Description),
	}
	for _, destination := range acl.LogDestinations {
		lines = append(lines, m.renderEC2DetailLine("Log Destination", destination))
	}
	lines = append(lines, "\n"+titleStyle.Render("  Protected Resources")+"\n")
	switch {
	case len(acl.ResourceARNs) == 0 && acl.ResourcesComplete:
		lines = append(lines, m.renderEC2DetailLine("Resource", "None"))
	case len(acl.ResourceARNs) == 0:
		lines = append(lines, m.renderEC2DetailLine("Resource", "Unknown (one or more lookups failed)"))
	default:
		if !acl.ResourcesComplete {
			lines = append(lines, m.renderEC2DetailLine("Associations", "Partial (one or more lookups failed)"))
		}
		for _, arn := range acl.ResourceARNs {
			lines = append(lines, m.renderEC2DetailLine("Resource", arn))
		}
	}
	lines = append(lines, "\n"+titleStyle.Render("  Rules — priority order")+"\n")
	if !acl.DetailKnown {
		lines = append(lines, m.renderEC2DetailLine("Rule", "Unknown (detail lookup failed)"))
	} else if len(acl.Rules) == 0 {
		lines = append(lines, m.renderEC2DetailLine("Rule", "None"))
	}
	for _, rule := range acl.Rules {
		visibility := fmt.Sprintf("metric=%s metrics=%t sampled=%t", rule.MetricName, rule.MetricsEnabled, rule.SamplingEnabled)
		ruleLines := []string{
			m.renderEC2DetailLine("Rule", fmt.Sprintf("[%d] %s", rule.Priority, rule.Name)),
			m.renderEC2DetailLine("Statement", rule.Statement),
			m.renderEC2DetailLine("Action", rule.Action),
		}
		for _, override := range rule.ActionOverrides {
			ruleLines = append(ruleLines, m.renderEC2DetailLine("Rule Override", override))
		}
		lines = append(lines, append(ruleLines, m.renderEC2DetailLine("Visibility", visibility))...)
	}
	return lines
}

func pendingWAFContextReturn(m *Model) bool {
	previous := wafContextReturn(m)
	return previous != nil && *previous == screenLoading && isWAFScreen(m.loadingReturnScreen)
}

func preservePendingWAFContextReturn(m *Model) {
	if previous := wafContextReturn(m); previous != nil && *previous == screenLoading && isWAFScreen(m.loadingReturnScreen) {
		*previous = m.loadingReturnScreen
	}
}

func normalizePendingWAFContextReturn(m *Model) {
	if pendingWAFContextReturn(m) {
		normalizeWAFContextReturn(m)
	}
}

func normalizeWAFContextReturn(m *Model) {
	if previous := wafContextReturn(m); previous != nil {
		*previous = screenServiceList
	}
}

func wafContextReturn(m *Model) *screen {
	previous := &m.ctxPrevScreen
	seen := make(map[screen]struct{})
	for range 8 {
		current := *previous
		if _, ok := seen[current]; ok {
			return nil
		}
		seen[current] = struct{}{}
		if isWAFScreen(current) ||
			(current == screenLoading && isWAFScreen(m.loadingReturnScreen)) ||
			(current == screenError && m.waf.loadFailed) {
			return previous
		}
		previous = wafOverlayPrevious(m, current)
		if previous == nil {
			return nil
		}
	}
	return nil
}

func wafOverlayPrevious(m *Model, current screen) *screen {
	switch current {
	case screenSettings:
		return &m.settingsPrevScreen
	case screenCommandPalette:
		return &m.palette.prevScreen
	case screenViewList:
		return &m.views.prevScreen
	case screenContextPicker:
		return &m.ctxPrevScreen
	case screenRegionPicker:
		return &m.regionPrevScreen
	default:
		return nil
	}
}

// finishWAFLoad rewrites a global overlay's return target instead of stealing it.
func finishWAFLoad(m *Model, target screen) {
	if m.ctxPrevScreen == screenLoading || isWAFScreen(m.ctxPrevScreen) {
		m.ctxPrevScreen = target
	}
	if m.screen == screenLoading {
		m.screen = target
		return
	}
	current := m.screen
	seen := make(map[screen]struct{})
	for range 8 {
		if _, ok := seen[current]; ok {
			return
		}
		seen[current] = struct{}{}
		previous := wafOverlayPrevious(m, current)
		if previous == nil {
			return
		}
		if *previous == screenLoading {
			*previous = target
			return
		}
		current = *previous
	}
}
