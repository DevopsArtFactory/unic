package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsservice "unic/internal/services/aws"
)

// --- Messages ---

type sgRuleAddedMsg struct{ err error }
type sgRuleDeletedMsg struct{ err error }
type sgRefreshedMsg struct {
	sg  *awsservice.SecurityGroup
	err error
}

type securityGroupModel struct {
	securityGroups         []awsservice.SecurityGroup
	filteredSecurityGroups []awsservice.SecurityGroup
	sgIdx                  int
	selectedSecurityGroup  *awsservice.SecurityGroup
	sgRuleSection          string
	sgRuleIdx              int
	sgDeleteConfirm        string
	sgDeleteRule           *awsservice.SecurityGroupRule
	sgAddField             int
	sgAddValues            map[string]string
	sgAddInput             string
	sgAddSelectIdx         int
}

func newSecurityGroupModel() securityGroupModel {
	return securityGroupModel{}
}

func (sm *securityGroupModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(sm.loadSecurityGroups(*m))
}

func (sm *securityGroupModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case securityGroupsLoadedMsg:
		sm.securityGroups = msg.securityGroups
		sm.filteredSecurityGroups = msg.securityGroups
		sm.sgIdx = 0
		m.screen = screenSecurityGroupList
		return *m, nil, true

	case sgRuleAddedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return *m, nil, true
		}
		return *m, sm.refreshSecurityGroup(*m), true

	case sgRuleDeletedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return *m, nil, true
		}
		return *m, sm.refreshSecurityGroup(*m), true

	case sgRefreshedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return *m, nil, true
		}
		sm.selectedSecurityGroup = msg.sg
		for i, sg := range sm.securityGroups {
			if sg.GroupID == msg.sg.GroupID {
				sm.securityGroups[i] = *msg.sg
				break
			}
		}
		sm.filteredSecurityGroups = applyFilter(sm.securityGroups, m.filterValue(filterSecurityGroups))
		sm.sgIdx = 0
		sm.sgRuleIdx = 0
		m.screen = screenSecurityGroupDetail
		return *m, nil, true
	}
	return *m, nil, false
}

func (sm *securityGroupModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenSecurityGroupList:
		newM, cmd := sm.updateSecurityGroupList(m, msg)
		return newM, cmd, true
	case screenSecurityGroupDetail:
		newM, cmd := sm.updateSecurityGroupDetail(m, msg)
		return newM, cmd, true
	case screenSecurityGroupAddRule:
		newM, cmd := sm.updateSecurityGroupAddRule(m, msg)
		return newM, cmd, true
	case screenSecurityGroupDeleteConfirm:
		newM, cmd := sm.updateSecurityGroupDeleteConfirm(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (sm securityGroupModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenSecurityGroupList:
		return sm.viewSecurityGroupList(m), true
	case screenSecurityGroupDetail:
		return sm.viewSecurityGroupDetail(m), true
	case screenSecurityGroupAddRule:
		return sm.viewSecurityGroupAddRule(m), true
	case screenSecurityGroupDeleteConfirm:
		return sm.viewSecurityGroupDeleteConfirm(m), true
	default:
		return "", false
	}
}

func (sm *securityGroupModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterSecurityGroups {
		return false
	}
	sm.filteredSecurityGroups = applyFilter(sm.securityGroups, m.filterValue(target))
	sm.sgIdx = 0
	return true
}

// --- Commands ---

func (sm *securityGroupModel) loadSecurityGroups(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		sgs, err := repo.ListSecurityGroups(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(sgs) == 0 {
			return errMsg{err: fmt.Errorf("no security groups found")}
		}
		return securityGroupsLoadedMsg{securityGroups: sgs}
	}
}

func (sm *securityGroupModel) executeSGAddRule(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return sgRuleAddedMsg{err: err}
			}
		}

		direction := sm.sgAddValues["direction"]
		protocol := sm.sgAddValues["protocol"]
		rule := awsservice.SecurityGroupRule{
			Protocol:    protocol,
			Description: sm.sgAddValues["description"],
		}
		if protocol != "-1" {
			fmt.Sscanf(sm.sgAddValues["fromPort"], "%d", &rule.FromPort)
			fmt.Sscanf(sm.sgAddValues["toPort"], "%d", &rule.ToPort)
		}

		source := sm.sgAddValues["source"]
		if strings.HasPrefix(source, "sg-") {
			rule.ReferencedSGID = source
		} else if strings.Contains(source, ":") {
			rule.CIDRV6 = source
		} else {
			rule.CIDRV4 = source
		}

		err := repo.AddSecurityGroupRule(ctx, sm.selectedSecurityGroup.GroupID, direction, rule)
		return sgRuleAddedMsg{err: err}
	}
}

func (sm *securityGroupModel) executeSGDeleteRule(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return sgRuleDeletedMsg{err: err}
			}
		}

		err := repo.DeleteSecurityGroupRule(ctx, sm.selectedSecurityGroup.GroupID, sm.sgRuleSection, *sm.sgDeleteRule)
		return sgRuleDeletedMsg{err: err}
	}
}

func (sm *securityGroupModel) refreshSecurityGroup(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return sgRefreshedMsg{err: err}
			}
		}

		sg, err := repo.RefreshSecurityGroup(ctx, sm.selectedSecurityGroup.GroupID)
		return sgRefreshedMsg{sg: sg, err: err}
	}
}

// --- List screen ---

func (sm *securityGroupModel) updateSecurityGroupList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterSecurityGroups); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterSecurityGroups)
	case "up", "k":
		sm.sgIdx = previousListIndex(sm.sgIdx, len(sm.filteredSecurityGroups))
	case "down", "j":
		sm.sgIdx = nextListIndex(sm.sgIdx, len(sm.filteredSecurityGroups))
	case "/":
		return *m, m.activateFilter(filterSecurityGroups)
	case "r":
		m.resetFilter(filterSecurityGroups)
		return m.startLoading(sm.loadSecurityGroups(*m))
	case "enter":
		if len(sm.filteredSecurityGroups) > 0 && sm.sgIdx < len(sm.filteredSecurityGroups) {
			selected := sm.filteredSecurityGroups[sm.sgIdx]
			sm.selectedSecurityGroup = &selected
			sm.sgRuleSection = "ingress"
			sm.sgRuleIdx = 0
			m.screen = screenSecurityGroupDetail
		}
	}
	return *m, nil
}

func (sm securityGroupModel) viewSecurityGroupList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Security Groups"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterSecurityGroups))
	b.WriteString("\n\n")

	if len(sm.filteredSecurityGroups) == 0 {
		panel.WriteString(dimStyle.Render("  No matching security groups"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if sm.sgIdx >= visibleLines {
			start = sm.sgIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(sm.filteredSecurityGroups))

		for i := start; i < end; i++ {
			sg := sm.filteredSecurityGroups[i]
			cursor := "  "
			style := normalStyle
			if i == sm.sgIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterSecurityGroups, sg.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d security groups", len(sm.filteredSecurityGroups), len(sm.securityGroups))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: detail • esc: back • H: home"))
	return b.String()
}

// --- Detail screen with rule navigation ---

func (sm securityGroupModel) activeRules() []awsservice.SecurityGroupRule {
	if sm.selectedSecurityGroup == nil {
		return nil
	}
	if sm.sgRuleSection == "egress" {
		return sm.selectedSecurityGroup.EgressRules
	}
	return sm.selectedSecurityGroup.IngressRules
}

func (sm *securityGroupModel) updateSecurityGroupDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenSecurityGroupList
	case "tab":
		if sm.sgRuleSection == "ingress" {
			sm.sgRuleSection = "egress"
		} else {
			sm.sgRuleSection = "ingress"
		}
		sm.sgRuleIdx = 0
	case "up", "k":
		sm.sgRuleIdx = previousListIndex(sm.sgRuleIdx, len(sm.activeRules()))
	case "down", "j":
		sm.sgRuleIdx = nextListIndex(sm.sgRuleIdx, len(sm.activeRules()))
	case "a":
		sm.sgAddField = 0
		sm.sgAddValues = map[string]string{}
		sm.sgAddInput = ""
		sm.sgAddSelectIdx = 0
		m.screen = screenSecurityGroupAddRule
	case "d":
		rules := sm.activeRules()
		if len(rules) > 0 && sm.sgRuleIdx < len(rules) {
			rule := rules[sm.sgRuleIdx]
			sm.sgDeleteRule = &rule
			sm.sgDeleteConfirm = ""
			m.screen = screenSecurityGroupDeleteConfirm
		}
	}
	return *m, nil
}

func (sm securityGroupModel) viewSecurityGroupDetail(m Model) string {
	if sm.selectedSecurityGroup == nil {
		return ""
	}
	sg := sm.selectedSecurityGroup
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Security Group Detail"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Group ID", normalStyle.Render(sg.GroupID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Name", normalStyle.Render(sg.Name)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Description", normalStyle.Render(sg.Description)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("VPC ID", normalStyle.Render(sg.VPCID)))
	b.WriteString("\n")

	// Inbound rules
	b.WriteString("\n")
	ingressHeader := "Inbound Rules"
	if sm.sgRuleSection == "ingress" {
		ingressHeader = "▸ Inbound Rules"
	}
	b.WriteString(titleStyle.Render(ingressHeader))
	b.WriteString("\n")
	sm.renderRuleTable(&b, sg.IngressRules, "SOURCE", sm.sgRuleSection == "ingress")

	// Outbound rules
	b.WriteString("\n")
	egressHeader := "Outbound Rules"
	if sm.sgRuleSection == "egress" {
		egressHeader = "▸ Outbound Rules"
	}
	b.WriteString(titleStyle.Render(egressHeader))
	b.WriteString("\n")
	sm.renderRuleTable(&b, sg.EgressRules, "DESTINATION", sm.sgRuleSection == "egress")

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("↑/↓: select rule • tab: switch section • a: add rule • d: delete rule • esc: back • H: home"))
	return b.String()
}

func (sm securityGroupModel) renderRuleTable(b *strings.Builder, rules []awsservice.SecurityGroupRule, sourceLabel string, isActive bool) {
	if len(rules) == 0 {
		b.WriteString(dimStyle.Render("  No rules"))
		b.WriteString("\n")
		return
	}

	protoCol := lipgloss.NewStyle().Width(8)
	portCol := lipgloss.NewStyle().Width(14)
	b.WriteString(dimStyle.Render("  " + protoCol.Render("PROTO") + portCol.Render("PORT") + sourceLabel))
	b.WriteString("\n")

	for i, rule := range rules {
		proto := rule.Protocol
		if proto == "-1" {
			proto = "All"
		}
		portRange := "All"
		if rule.Protocol != "-1" {
			if rule.FromPort == rule.ToPort {
				portRange = fmt.Sprintf("%d", rule.FromPort)
			} else {
				portRange = fmt.Sprintf("%d-%d", rule.FromPort, rule.ToPort)
			}
		}
		source := rule.CIDRV4
		if source == "" {
			source = rule.CIDRV6
		}
		if source == "" && rule.ReferencedSGID != "" {
			source = rule.ReferencedSGID
		}
		if source == "" {
			source = "-"
		}
		row := "  " + protoCol.Render(proto) + portCol.Render(portRange) + source
		if rule.Description != "" {
			row += dimStyle.Render("  " + rule.Description)
		}

		if isActive && i == sm.sgRuleIdx {
			b.WriteString(selectedStyle.Render("> " + row[2:]))
		} else {
			b.WriteString(normalStyle.Render(row))
		}
		b.WriteString("\n")
	}
}

// --- Add Rule form screen ---

var sgAddFieldLabels = []string{"Direction", "Protocol", "From Port", "To Port", "Source/Dest", "Description"}
var sgDirectionOptions = []string{"ingress", "egress"}
var sgProtocolOptions = []string{"tcp", "udp", "-1"}
var sgProtocolLabels = []string{"TCP", "UDP", "All Traffic"}

func (sm *securityGroupModel) updateSecurityGroupAddRule(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch sm.sgAddField {
	case 0: // direction select
		return sm.updateSGAddSelect(m, key, sgDirectionOptions)
	case 1: // protocol select
		return sm.updateSGAddSelect(m, key, sgProtocolOptions)
	case 2, 3: // fromPort, toPort (text input)
		return sm.updateSGAddTextInput(m, key)
	case 4: // source/dest (text input)
		return sm.updateSGAddTextInput(m, key)
	case 5: // description (text input, optional)
		return sm.updateSGAddTextInput(m, key)
	}
	return *m, nil
}

func (sm *securityGroupModel) updateSGAddSelect(m *Model, key string, options []string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = screenSecurityGroupDetail
	case "up", "k":
		sm.sgAddSelectIdx = previousListIndex(sm.sgAddSelectIdx, len(options))
	case "down", "j":
		sm.sgAddSelectIdx = nextListIndex(sm.sgAddSelectIdx, len(options))
	case "enter":
		fieldKey := strings.ToLower(sgAddFieldLabels[sm.sgAddField])
		if sm.sgAddField == 0 {
			sm.sgAddValues["direction"] = options[sm.sgAddSelectIdx]
		} else {
			sm.sgAddValues["protocol"] = options[sm.sgAddSelectIdx]
		}
		_ = fieldKey
		sm.sgAddField++
		sm.sgAddSelectIdx = 0
		sm.sgAddInput = ""
		// If protocol is "all", skip port fields
		if sm.sgAddField == 2 && sm.sgAddValues["protocol"] == "-1" {
			sm.sgAddField = 4
		}
	}
	return *m, nil
}

func (sm *securityGroupModel) updateSGAddTextInput(m *Model, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = screenSecurityGroupDetail
	case "enter":
		switch sm.sgAddField {
		case 2:
			sm.sgAddValues["fromPort"] = sm.sgAddInput
		case 3:
			sm.sgAddValues["toPort"] = sm.sgAddInput
		case 4:
			if sm.sgAddInput == "" {
				return *m, nil // source is required
			}
			sm.sgAddValues["source"] = sm.sgAddInput
		case 5:
			sm.sgAddValues["description"] = sm.sgAddInput
			// Last field — execute the add
			sm.sgAddInput = ""
			return m.startLoading(sm.executeSGAddRule(*m))
		}
		sm.sgAddField++
		sm.sgAddInput = ""
		// If protocol is "all", skip port fields
		if sm.sgAddField == 2 && sm.sgAddValues["protocol"] == "-1" {
			sm.sgAddField = 4
		}
	case "backspace":
		sm.sgAddInput = trimLastRune(sm.sgAddInput)
	default:
		if len(key) == 1 {
			sm.sgAddInput += key
		}
	}
	return *m, nil
}

func (sm securityGroupModel) viewSecurityGroupAddRule(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Add Security Group Rule"))
	b.WriteString("\n\n")

	if sm.selectedSecurityGroup != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Security Group: %s (%s)", sm.selectedSecurityGroup.Name, sm.selectedSecurityGroup.GroupID)))
		b.WriteString("\n\n")
	}

	// Show completed fields
	for i := 0; i < sm.sgAddField; i++ {
		label := sgAddFieldLabels[i]
		val := ""
		switch i {
		case 0:
			val = sm.sgAddValues["direction"]
		case 1:
			proto := sm.sgAddValues["protocol"]
			for j, p := range sgProtocolOptions {
				if p == proto {
					val = sgProtocolLabels[j]
					break
				}
			}
		case 2:
			val = sm.sgAddValues["fromPort"]
		case 3:
			val = sm.sgAddValues["toPort"]
		case 4:
			val = sm.sgAddValues["source"]
		case 5:
			val = sm.sgAddValues["description"]
		}
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %s: %s", label, val)))
		b.WriteString("\n")
	}

	// Show current field
	if sm.sgAddField < len(sgAddFieldLabels) {
		label := sgAddFieldLabels[sm.sgAddField]
		b.WriteString("\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  %s:", label)))
		b.WriteString("\n")

		switch sm.sgAddField {
		case 0: // direction select
			for i, opt := range []string{"Ingress (inbound)", "Egress (outbound)"} {
				cursor := "  "
				style := normalStyle
				if i == sm.sgAddSelectIdx {
					cursor = "> "
					style = selectedStyle
				}
				b.WriteString(style.Render(fmt.Sprintf("  %s%s", cursor, opt)))
				b.WriteString("\n")
			}
		case 1: // protocol select
			for i, label := range sgProtocolLabels {
				cursor := "  "
				style := normalStyle
				if i == sm.sgAddSelectIdx {
					cursor = "> "
					style = selectedStyle
				}
				b.WriteString(style.Render(fmt.Sprintf("  %s%s", cursor, label)))
				b.WriteString("\n")
			}
		default: // text input
			hint := ""
			switch sm.sgAddField {
			case 2:
				hint = " (e.g. 443)"
			case 3:
				hint = " (e.g. 443)"
			case 4:
				hint = " (CIDR, IPv6, or sg-xxxxx)"
			case 5:
				hint = " (optional, press enter to skip)"
			}
			b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", sm.sgAddInput)))
			if hint != "" {
				b.WriteString(dimStyle.Render(hint))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if sm.sgAddField == 0 || sm.sgAddField == 1 {
		b.WriteString(m.renderHelpBar("↑/↓: select • enter: confirm • esc: cancel"))
	} else {
		b.WriteString(m.renderHelpBar("enter: confirm • esc: cancel"))
	}
	return b.String()
}

// --- Delete Confirm screen ---

func (sm *securityGroupModel) updateSecurityGroupDeleteConfirm(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if sm.selectedSecurityGroup == nil || sm.sgDeleteRule == nil {
		m.screen = screenSecurityGroupDetail
		return *m, nil
	}

	confirmTarget := sm.selectedSecurityGroup.GroupID

	switch msg.String() {
	case "esc":
		m.screen = screenSecurityGroupDetail
	case "enter":
		if sm.sgDeleteConfirm == confirmTarget {
			return m.startLoading(sm.executeSGDeleteRule(*m))
		}
	case "backspace":
		sm.sgDeleteConfirm = trimLastRune(sm.sgDeleteConfirm)
	default:
		sm.sgDeleteConfirm = appendKeyRunes(sm.sgDeleteConfirm, msg)
	}
	return *m, nil
}

func (sm securityGroupModel) viewSecurityGroupDeleteConfirm(m Model) string {
	if sm.selectedSecurityGroup == nil || sm.sgDeleteRule == nil {
		return ""
	}
	sg := sm.selectedSecurityGroup
	rule := sm.sgDeleteRule

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(errorStyle.Render("Confirm Rule Deletion"))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render(fmt.Sprintf("  You are about to delete a %s rule from:", sm.sgRuleSection)))
	b.WriteString("\n")
	b.WriteString(selectedStyle.Render(fmt.Sprintf("  %s (%s)", sg.Name, sg.GroupID)))
	b.WriteString("\n\n")

	// Show rule details
	b.WriteString(normalStyle.Render("  Rule:"))
	b.WriteString("\n")
	proto := rule.Protocol
	if proto == "-1" {
		proto = "All"
	}
	b.WriteString(normalStyle.Render(fmt.Sprintf("    Protocol: %s", proto)))
	b.WriteString("\n")
	if rule.Protocol != "-1" {
		if rule.FromPort == rule.ToPort {
			b.WriteString(normalStyle.Render(fmt.Sprintf("    Port: %d", rule.FromPort)))
		} else {
			b.WriteString(normalStyle.Render(fmt.Sprintf("    Port: %d-%d", rule.FromPort, rule.ToPort)))
		}
		b.WriteString("\n")
	}
	source := rule.CIDRV4
	if source == "" {
		source = rule.CIDRV6
	}
	if source == "" && rule.ReferencedSGID != "" {
		source = rule.ReferencedSGID
	}
	if source == "" {
		source = "-"
	}
	sourceLabel := "Source"
	if sm.sgRuleSection == "egress" {
		sourceLabel = "Destination"
	}
	b.WriteString(normalStyle.Render(fmt.Sprintf("    %s: %s", sourceLabel, source)))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render("  Type the security group ID to confirm:"))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", sm.sgDeleteConfirm)))
	b.WriteString("\n\n")

	b.WriteString(m.renderHelpBar("enter: confirm • esc: cancel"))
	return b.String()
}
