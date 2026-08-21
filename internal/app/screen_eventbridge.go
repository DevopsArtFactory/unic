package app

import (
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

func (em *eventBridgeModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(em.load(*m))
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
			updated, cmd := m.Update(errMsg{err: msg.err})
			return updated, cmd, true
		}
		em.rules = msg.rules
		em.filtered = applyFilter(em.rules, m.filterValue(filterEventBridgeRules))
		em.idx = 0
		em.selected = nil
		em.notice = ""
		em.detailScroll = 0
		m.screen = screenEventBridgeRuleList
		return *m, nil, true
	case eventBridgeActionDoneMsg:
		if msg.err != nil {
			updated, cmd := m.Update(errMsg{err: msg.err})
			return updated, cmd, true
		}
		em.updateRuleState(msg.ruleName, msg.busName, msg.enabled)
		action := "disabled"
		if msg.enabled {
			action = "enabled"
		}
		em.notice = fmt.Sprintf("Rule %s", action)
		em.confirmInput = ""
		m.screen = screenEventBridgeRuleDetail
		return *m, nil, true
	}
	return *m, nil, false
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
		return m.startLoading(em.load(*m))
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
		if em.selected != nil && !em.selected.IsEnabled() && !em.selected.IsManaged() {
			em.desiredEnabled = true
			em.confirmInput = ""
			m.screen = screenEventBridgeRuleConfirm
		}
	case "d":
		if em.selected != nil && em.selected.IsEnabled() && !em.selected.IsManaged() {
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
		if em.selected != nil && em.confirmInput == em.selected.Name {
			rule := *em.selected
			enabled := em.desiredEnabled
			action := "Disabling"
			if enabled {
				action = "Enabling"
			}
			m.screen = screenEventBridgeRuleDetail
			return m.startLoadingWithMessage(action+" EventBridge rule...", []string{rule.Name, rule.EventBusName}, em.setEnabled(*m, rule, enabled))
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

func (em *eventBridgeModel) updateRuleState(name, bus string, enabled bool) {
	state := "DISABLED"
	if enabled {
		state = "ENABLED"
	}
	for i := range em.rules {
		if em.rules[i].Name == name && em.rules[i].EventBusName == bus {
			em.rules[i].State = state
		}
	}
	for i := range em.filtered {
		if em.filtered[i].Name == name && em.filtered[i].EventBusName == bus {
			em.filtered[i].State = state
		}
	}
	if em.selected != nil && em.selected.Name == name && em.selected.EventBusName == bus {
		em.selected.State = state
	}
	sortEventBridgeRuleRows(em.rules)
	sortEventBridgeRuleRows(em.filtered)
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
		m.renderEC2DetailLine("Event Pattern", ec2ValueOrDash(rule.CompactEventPattern())),
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
	}
	if em.notice != "" {
		lines = append(lines, selectedStyle.Render("  "+em.notice)+"\n")
	}
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
	b.WriteString(selectedStyle.Render("  " + escapeTerminalControls(em.selected.Name)))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("  Type the rule name to confirm:"))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", escapeTerminalControls(em.confirmInput))))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}
