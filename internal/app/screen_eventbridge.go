package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsservice "unic/internal/services/aws"
)

type eventBridgeModel struct {
	rules, filtered []awsservice.EventBridgeRule
	idx             int
	selected        *awsservice.EventBridgeRule
	desiredEnabled  bool
	confirmInput    string
	notice          string
	detailScroll    int
}

func newEventBridgeModel() eventBridgeModel { return eventBridgeModel{} }

func isEventBridgeScreen(value screen) bool {
	switch value {
	case screenEventBridgeRuleList, screenEventBridgeRuleDetail, screenEventBridgeRuleConfirm:
		return true
	default:
		return false
	}
}

func eventBridgeRuleIdentity(rule awsservice.EventBridgeRule) string {
	return rule.EventBusName + "/" + rule.Name
}

func (em *eventBridgeModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoadingFor(screenEventBridgeRuleList, "Loading...", nil, em.load(*m))
}

func (em eventBridgeModel) load(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return eventBridgeRulesLoadedMsg{err: err}
		}
		rules, err := repo.ListEventBridgeRules(ctx)
		return eventBridgeRulesLoadedMsg{rules: rules, err: err}
	}
}

func (em *eventBridgeModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case eventBridgeRulesLoadedMsg:
		if msg.err != nil {
			if em.preserveOverlay(m, screenError) {
				m.errMsg = msg.err.Error()
				m.loadingTitle = ""
				m.loadingDetails = nil
				return *m, nil, true
			}
			m.loadingReturnScreen = 0
			updated, cmd := m.Update(errMsg{err: msg.err})
			return updated, cmd, true
		}
		em.rules = msg.rules
		em.filtered = applyFilter(em.rules, m.filterValue(filterEventBridgeRules))
		em.idx = 0
		em.selected = nil
		em.notice = ""
		em.detailScroll = 0
		if !em.preserveOverlay(m, screenEventBridgeRuleList) {
			m.loadingReturnScreen = 0
			m.screen = screenEventBridgeRuleList
		}
		return *m, nil, true
	case eventBridgeActionDoneMsg:
		if msg.err != nil {
			if em.preserveOverlay(m, screenError) {
				m.errMsg = msg.err.Error()
				m.loadingTitle = ""
				m.loadingDetails = nil
				return *m, nil, true
			}
			m.loadingReturnScreen = 0
			updated, cmd := m.Update(errMsg{err: msg.err})
			return updated, cmd, true
		}
		em.updateRuleState(msg.ruleName, msg.busName, msg.enabled, m.filterValue(filterEventBridgeRules))
		action := "disabled"
		if msg.enabled {
			action = "enabled"
		}
		em.notice = fmt.Sprintf("Rule %s", action)
		em.confirmInput = ""
		if !em.preserveOverlay(m, screenEventBridgeRuleDetail) {
			m.loadingReturnScreen = 0
			m.screen = screenEventBridgeRuleDetail
		}
		return *m, nil, true
	}
	return *m, nil, false
}

func (em *eventBridgeModel) preserveOverlay(m *Model, target screen) bool {
	pendingContextPicker := m.ctxPickerPending && m.screen == screenLoading && m.ctxPrevScreen == screenLoading
	switch m.screen {
	case screenSettings, screenCommandPalette, screenViewList, screenContextPicker, screenContextAdd:
	default:
		if !pendingContextPicker {
			return false
		}
	}
	if !isEventBridgeScreen(m.loadingReturnScreen) {
		return false
	}
	preserved := false
	for _, previous := range []*screen{&m.settingsPrevScreen, &m.palette.prevScreen, &m.views.prevScreen, &m.ctxPrevScreen} {
		if *previous == screenLoading {
			*previous = target
			preserved = true
		}
	}
	if preserved {
		m.loadingReturnScreen = 0
	}
	return preserved
}

func (em *eventBridgeModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenEventBridgeRuleList:
		updated, cmd := em.updateList(m, msg)
		return updated, cmd, true
	case screenEventBridgeRuleDetail:
		updated, cmd := em.updateDetail(m, msg)
		return updated, cmd, true
	case screenEventBridgeRuleConfirm:
		updated, cmd := em.updateConfirm(m, msg)
		return updated, cmd, true
	}
	return *m, nil, false
}

func (em eventBridgeModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenEventBridgeRuleList:
		return em.viewList(m), true
	case screenEventBridgeRuleDetail:
		return em.viewDetail(m), true
	case screenEventBridgeRuleConfirm:
		return em.viewConfirm(m), true
	}
	return "", false
}

func (em *eventBridgeModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterEventBridgeRules {
		return false
	}
	em.filtered = applyFilter(em.rules, m.filterValue(target))
	em.idx = 0
	return true
}

func (em *eventBridgeModel) updateList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterEventBridgeRules); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterEventBridgeRules)
	case "up", "k":
		em.idx = previousListIndex(em.idx, len(em.filtered))
	case "down", "j":
		em.idx = nextListIndex(em.idx, len(em.filtered))
	case "/":
		return *m, m.activateFilter(filterEventBridgeRules)
	case "r":
		m.resetFilter(filterEventBridgeRules)
		return m.startLoadingFor(screenEventBridgeRuleList, "Loading...", nil, em.load(*m))
	case "enter":
		if len(em.filtered) > 0 && em.idx < len(em.filtered) {
			selected := em.filtered[em.idx]
			em.selected = &selected
			em.notice = ""
			em.detailScroll = 0
			m.screen = screenEventBridgeRuleDetail
		}
	}
	return *m, nil
}

func (em *eventBridgeModel) updateDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := em.detailLines(*m)
	visibleLines := max(m.height-8, 5)
	maxOffset := max(len(lines)-visibleLines, 0)
	switch msg.String() {
	case "q", "esc":
		em.selected = nil
		em.notice = ""
		em.detailScroll = 0
		m.screen = screenEventBridgeRuleList
	case "up", "k":
		em.detailScroll = max(em.detailScroll-1, 0)
	case "down", "j":
		em.detailScroll = min(em.detailScroll+1, maxOffset)
	case "pgup":
		em.detailScroll = max(em.detailScroll-visibleLines, 0)
	case "pgdown":
		em.detailScroll = min(em.detailScroll+visibleLines, maxOffset)
	case "e":
		if em.selected != nil && !em.selected.IsEnabled() && em.selected.CanChangeState() {
			em.desiredEnabled = true
			em.confirmInput = ""
			m.screen = screenEventBridgeRuleConfirm
		}
	case "d":
		if em.selected != nil && em.selected.IsEnabled() && em.selected.CanChangeState() {
			em.desiredEnabled = false
			em.confirmInput = ""
			m.screen = screenEventBridgeRuleConfirm
		}
	}
	return *m, nil
}

func (em *eventBridgeModel) updateConfirm(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		em.confirmInput = ""
		m.screen = screenEventBridgeRuleDetail
	case "enter":
		if em.selected != nil && em.confirmInput == eventBridgeRuleIdentity(*em.selected) {
			rule := *em.selected
			enabled := em.desiredEnabled
			action := "Disabling"
			if enabled {
				action = "Enabling"
			}
			m.screen = screenEventBridgeRuleDetail
			return m.startLoadingFor(screenEventBridgeRuleDetail, action+" EventBridge rule...", []string{rule.Name, rule.EventBusName}, em.setEnabled(*m, rule, enabled))
		}
	case "backspace":
		em.confirmInput = trimLastRune(em.confirmInput)
	default:
		em.confirmInput = appendKeyRunes(em.confirmInput, msg)
	}
	return *m, nil
}

func (em eventBridgeModel) setEnabled(m Model, rule awsservice.EventBridgeRule, enabled bool) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err == nil {
			err = repo.SetEventBridgeRuleEnabled(ctx, rule.Name, rule.EventBusName, enabled)
		}
		return eventBridgeActionDoneMsg{ruleName: rule.Name, busName: rule.EventBusName, enabled: enabled, err: err}
	}
}

func (em *eventBridgeModel) updateRuleState(name, bus string, enabled bool, filter string) {
	state := "DISABLED"
	if enabled {
		state = "ENABLED"
	}
	for i := range em.rules {
		if em.rules[i].Name == name && em.rules[i].EventBusName == bus {
			em.rules[i].State = state
		}
	}
	if em.selected != nil && em.selected.Name == name && em.selected.EventBusName == bus {
		em.selected.State = state
	}
	sortEventBridgeRuleRows(em.rules)
	em.filtered = applyFilter(em.rules, filter)
	em.idx = min(em.idx, max(len(em.filtered)-1, 0))
	for i := range em.filtered {
		if em.filtered[i].Name == name && em.filtered[i].EventBusName == bus {
			em.idx = i
			break
		}
	}
}

func sortEventBridgeRuleRows(rules []awsservice.EventBridgeRule) {
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].IsEnabled() != rules[j].IsEnabled() {
			return !rules[i].IsEnabled()
		}
		leftBus, rightBus := strings.ToLower(rules[i].EventBusName), strings.ToLower(rules[j].EventBusName)
		if leftBus != rightBus {
			return leftBus < rightBus
		}
		if rules[i].EventBusName != rules[j].EventBusName {
			return rules[i].EventBusName < rules[j].EventBusName
		}
		leftName, rightName := strings.ToLower(rules[i].Name), strings.ToLower(rules[j].Name)
		if leftName != rightName {
			return leftName < rightName
		}
		return rules[i].Name < rules[j].Name
	})
}

func (em eventBridgeModel) viewList(m Model) string {
	var b, panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("EventBridge Rules"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterEventBridgeRules))
	b.WriteString("\n\n")

	if len(em.filtered) == 0 {
		empty := "  No EventBridge rules found"
		if len(em.rules) > 0 {
			empty = "  No matching EventBridge rules"
		}
		panel.WriteString(dimStyle.Render(empty + "\n"))
	} else {
		nameCol := lipgloss.NewStyle().Width(30).MaxWidth(30)
		busCol := lipgloss.NewStyle().Width(18).MaxWidth(18)
		stateCol := lipgloss.NewStyle().Width(10).MaxWidth(10)
		targetCol := lipgloss.NewStyle().Width(7).MaxWidth(7)
		triggerCol := lipgloss.NewStyle().Width(28).MaxWidth(28)
		panel.WriteString(dimStyle.Render("  " + nameCol.Render("RULE") + " " + busCol.Render("BUS") + " " + stateCol.Render("STATE") + " " + targetCol.Render("TARGETS") + " " + triggerCol.Render("TRIGGER") + " LAST SEEN\n"))
		visibleLines := max(m.height-11, 5)
		start := 0
		if em.idx >= visibleLines {
			start = em.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filtered))
		for i := start; i < end; i++ {
			rule := em.filtered[i]
			row := nameCol.Render(escapeTerminalControls(rule.Name)) + " " +
				busCol.Render(escapeTerminalControls(rule.EventBusName)) + " " +
				stateCol.Render(rule.State) + " " + targetCol.Render(rule.TargetSummary()) + " " +
				triggerCol.Render(escapeTerminalControls(rule.TriggerSummary())) + " " + eventBridgeActivitySummary(rule)
			cursor := "  "
			style := normalStyle
			if i == em.idx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterEventBridgeRules, row)))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d rules (disabled first; activity is best-effort over 7 days)", len(em.filtered), len(em.rules))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func eventBridgeActivitySummary(rule awsservice.EventBridgeRule) string {
	if !rule.LastTriggeredAt.IsZero() {
		return rule.LastTriggeredAt.Local().Format("01-02 15:04")
	}
	if strings.HasPrefix(rule.LastTriggerStatus, "Unavailable") {
		return "unavailable"
	}
	return "none (7d)"
}

func (em eventBridgeModel) viewDetail(m Model) string {
	if em.selected == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("EventBridge Rule Detail"))
	b.WriteString("\n\n")
	lines := em.detailLines(m)
	visibleLines := max(m.height-8, 5)
	start := min(em.detailScroll, max(len(lines)-visibleLines, 0))
	for _, line := range lines[start:min(start+visibleLines, len(lines))] {
		b.WriteString(line)
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (em eventBridgeModel) detailLines(m Model) []string {
	if em.selected == nil {
		return nil
	}
	rule := em.selected
	lines := []string{
		m.renderEC2DetailLine("Name", rule.Name),
		m.renderEC2DetailLine("Event Bus", rule.EventBusName),
	}
	if rule.IsEnabled() {
		lines = append(lines, m.renderEC2StyledDetailLine("State", selectedStyle.Render(rule.State)))
	} else {
		lines = append(lines, m.renderEC2StyledDetailLine("State", errorStyle.Render(rule.State)))
	}
	lines = append(lines,
		m.renderEC2DetailLine("ARN", rule.ARN),
		m.renderEC2DetailLine("Schedule", ec2ValueOrDash(rule.ScheduleExpression)),
	)
	lines = append(lines, eventBridgePatternDetailLines(m, rule.EventPattern)...)
	lines = append(lines,
		m.renderEC2DetailLine("Last Seen", rule.LastTriggerDisplay()),
		m.renderEC2DetailLine("Description", ec2ValueOrDash(rule.Description)),
		m.renderEC2DetailLine("Role ARN", ec2ValueOrDash(rule.RoleARN)),
		m.renderEC2DetailLine("Managed By", ec2ValueOrDash(rule.ManagedBy)),
		"\n",
		titleStyle.Render(fmt.Sprintf("Targets (%d)", len(rule.Targets)))+"\n",
	)
	if len(rule.Targets) == 0 {
		lines = append(lines, dimStyle.Render("  (no targets)")+"\n")
	}
	for _, target := range rule.Targets {
		label := "Target " + target.ID
		lines = append(lines, m.renderEC2DetailLine(label, target.ARN))
		if target.RoleARN != "" {
			lines = append(lines, m.renderEC2DetailLine("Target Role", target.RoleARN))
		}
		if target.DeadLetterARN != "" {
			lines = append(lines, m.renderEC2DetailLine("Dead Letter", target.DeadLetterARN))
		}
	}
	lines = append(lines, "\n", dimStyle.Render("  Last Seen uses best-effort CloudWatch TriggeredRules data (7-day, 30-minute window).")+"\n")
	if rule.IsManaged() {
		lines = append(lines, dimStyle.Render("  AWS-managed rule state cannot be changed here.")+"\n")
	} else if rule.IncludesAllCloudTrailManagementEvents() {
		lines = append(lines, dimStyle.Render("  Rules that include all CloudTrail management events are read-only to preserve that matching mode.")+"\n")
	}
	if em.notice != "" {
		lines = append(lines, selectedStyle.Render("  "+em.notice)+"\n")
	}
	return lines
}

func eventBridgePatternDetailLines(m Model, pattern string) []string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return []string{m.renderEC2DetailLine("Event Pattern", "-")}
	}

	var pretty bytes.Buffer
	if json.Indent(&pretty, []byte(pattern), "", "  ") == nil {
		pattern = pretty.String()
	}
	width := m.ec2DetailValueWidth(ec2DetailLabelWidth)
	label := "Event Pattern"
	var lines []string
	for _, sourceLine := range strings.Split(pattern, "\n") {
		sourceLine = escapeTerminalControls(sourceLine)
		for _, wrapped := range wrapEventBridgeDetailValue(sourceLine, width) {
			lines = append(lines, m.renderEC2StyledDetailLine(label, normalStyle.Render(wrapped)))
			label = ""
		}
	}
	return lines
}

func wrapEventBridgeDetailValue(value string, width int) []string {
	if width <= 0 || lipgloss.Width(value) <= width {
		return []string{value}
	}
	var lines []string
	var line strings.Builder
	lineWidth := 0
	for _, r := range value {
		runeWidth := lipgloss.Width(string(r))
		if lineWidth+runeWidth > width && line.Len() > 0 {
			lines = append(lines, line.String())
			line.Reset()
			lineWidth = 0
		}
		line.WriteRune(r)
		lineWidth += runeWidth
	}
	lines = append(lines, line.String())
	return lines
}

func (em eventBridgeModel) viewConfirm(m Model) string {
	if em.selected == nil {
		return ""
	}
	action := "DISABLE"
	if em.desiredEnabled {
		action = "ENABLE"
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(errorStyle.Render("Confirm EventBridge Rule State Change"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  You are about to %s rule:", action)))
	b.WriteString("\n")
	b.WriteString(selectedStyle.Render("  " + escapeTerminalControls(eventBridgeRuleIdentity(*em.selected))))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("  Type event-bus/rule to confirm:"))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", escapeTerminalControls(em.confirmInput))))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}
