package app

import (
	"context"
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

func (m Model) handleSecurityGroupMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case securityGroupsLoadedMsg:
		m.securityGroups = msg.securityGroups
		m.filteredSecurityGroups = msg.securityGroups
		m.sgIdx = 0
		m.screen = screenSecurityGroupList
		return m, nil, true

	case sgRuleAddedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil, true
		}
		return m, m.refreshSecurityGroup(), true

	case sgRuleDeletedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil, true
		}
		return m, m.refreshSecurityGroup(), true

	case sgRefreshedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil, true
		}
		m.selectedSecurityGroup = msg.sg
		for i, sg := range m.securityGroups {
			if sg.GroupID == msg.sg.GroupID {
				m.securityGroups[i] = *msg.sg
				break
			}
		}
		m.filteredSecurityGroups = applyFilter(m.securityGroups, m.filterValue(filterSecurityGroups))
		m.sgIdx = 0
		m.sgRuleIdx = 0
		m.screen = screenSecurityGroupDetail
		return m, nil, true
	}
	return m, nil, false
}

// --- Commands ---

func (m Model) loadSecurityGroups() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
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

func (m Model) executeSGAddRule() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return sgRuleAddedMsg{err: err}
			}
		}

		direction := m.sgAddValues["direction"]
		protocol := m.sgAddValues["protocol"]
		rule := awsservice.SecurityGroupRule{
			Protocol:    protocol,
			Description: m.sgAddValues["description"],
		}
		if protocol != "-1" {
			fmt.Sscanf(m.sgAddValues["fromPort"], "%d", &rule.FromPort)
			fmt.Sscanf(m.sgAddValues["toPort"], "%d", &rule.ToPort)
		}

		source := m.sgAddValues["source"]
		if strings.HasPrefix(source, "sg-") {
			rule.ReferencedSGID = source
		} else if strings.Contains(source, ":") {
			rule.CIDRV6 = source
		} else {
			rule.CIDRV4 = source
		}

		err := repo.AddSecurityGroupRule(ctx, m.selectedSecurityGroup.GroupID, direction, rule)
		return sgRuleAddedMsg{err: err}
	}
}

func (m Model) executeSGDeleteRule() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return sgRuleDeletedMsg{err: err}
			}
		}

		err := repo.DeleteSecurityGroupRule(ctx, m.selectedSecurityGroup.GroupID, m.sgRuleSection, *m.sgDeleteRule)
		return sgRuleDeletedMsg{err: err}
	}
}

func (m Model) refreshSecurityGroup() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return sgRefreshedMsg{err: err}
			}
		}

		sg, err := repo.RefreshSecurityGroup(ctx, m.selectedSecurityGroup.GroupID)
		return sgRefreshedMsg{sg: sg, err: err}
	}
}

// --- List screen ---

func (m Model) updateSecurityGroupList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterSecurityGroups); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterSecurityGroups)
	case "up", "k":
		if m.sgIdx > 0 {
			m.sgIdx--
		}
	case "down", "j":
		if m.sgIdx < len(m.filteredSecurityGroups)-1 {
			m.sgIdx++
		}
	case "/":
		return m, m.activateFilter(filterSecurityGroups)
	case "r":
		m.resetFilter(filterSecurityGroups)
		return m.startLoading(m.loadSecurityGroups())
	case "enter":
		if len(m.filteredSecurityGroups) > 0 && m.sgIdx < len(m.filteredSecurityGroups) {
			selected := m.filteredSecurityGroups[m.sgIdx]
			m.selectedSecurityGroup = &selected
			m.sgRuleSection = "ingress"
			m.sgRuleIdx = 0
			m.screen = screenSecurityGroupDetail
		}
	}
	return m, nil
}

func (m Model) viewSecurityGroupList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Security Groups"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterSecurityGroups))
	b.WriteString("\n\n")

	if len(m.filteredSecurityGroups) == 0 {
		b.WriteString(dimStyle.Render("  No matching security groups"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-8, 5)
		start := 0
		if m.sgIdx >= visibleLines {
			start = m.sgIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredSecurityGroups))

		for i := start; i < end; i++ {
			sg := m.filteredSecurityGroups[i]
			cursor := "  "
			style := normalStyle
			if i == m.sgIdx {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, sg.DisplayTitle())))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d security groups", len(m.filteredSecurityGroups), len(m.securityGroups))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • r: refresh • enter: detail • esc: back • H: home"))
	return b.String()
}

// --- Detail screen with rule navigation ---

func (m Model) activeRules() []awsservice.SecurityGroupRule {
	if m.selectedSecurityGroup == nil {
		return nil
	}
	if m.sgRuleSection == "egress" {
		return m.selectedSecurityGroup.EgressRules
	}
	return m.selectedSecurityGroup.IngressRules
}

func (m Model) updateSecurityGroupDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenSecurityGroupList
	case "tab":
		if m.sgRuleSection == "ingress" {
			m.sgRuleSection = "egress"
		} else {
			m.sgRuleSection = "ingress"
		}
		m.sgRuleIdx = 0
	case "up", "k":
		if m.sgRuleIdx > 0 {
			m.sgRuleIdx--
		}
	case "down", "j":
		rules := m.activeRules()
		if m.sgRuleIdx < len(rules)-1 {
			m.sgRuleIdx++
		}
	case "a":
		m.sgAddField = 0
		m.sgAddValues = map[string]string{}
		m.sgAddInput = ""
		m.sgAddSelectIdx = 0
		m.screen = screenSecurityGroupAddRule
	case "d":
		rules := m.activeRules()
		if len(rules) > 0 && m.sgRuleIdx < len(rules) {
			rule := rules[m.sgRuleIdx]
			m.sgDeleteRule = &rule
			m.sgDeleteConfirm = ""
			m.screen = screenSecurityGroupDeleteConfirm
		}
	}
	return m, nil
}

func (m Model) viewSecurityGroupDetail() string {
	if m.selectedSecurityGroup == nil {
		return ""
	}
	sg := m.selectedSecurityGroup
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Security Group Detail"))
	b.WriteString("\n\n")

	labelStyle := lipgloss.NewStyle().Width(16)
	b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("Group ID"), sg.GroupID)))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("Name"), sg.Name)))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("Description"), sg.Description)))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("VPC ID"), sg.VPCID)))
	b.WriteString("\n")

	// Inbound rules
	b.WriteString("\n")
	ingressHeader := "Inbound Rules"
	if m.sgRuleSection == "ingress" {
		ingressHeader = "▸ Inbound Rules"
	}
	b.WriteString(titleStyle.Render(ingressHeader))
	b.WriteString("\n")
	m.renderRuleTable(&b, sg.IngressRules, "SOURCE", m.sgRuleSection == "ingress")

	// Outbound rules
	b.WriteString("\n")
	egressHeader := "Outbound Rules"
	if m.sgRuleSection == "egress" {
		egressHeader = "▸ Outbound Rules"
	}
	b.WriteString(titleStyle.Render(egressHeader))
	b.WriteString("\n")
	m.renderRuleTable(&b, sg.EgressRules, "DESTINATION", m.sgRuleSection == "egress")

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: select rule • tab: switch section • a: add rule • d: delete rule • esc: back • H: home"))
	return b.String()
}

func (m Model) renderRuleTable(b *strings.Builder, rules []awsservice.SecurityGroupRule, sourceLabel string, isActive bool) {
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

		if isActive && i == m.sgRuleIdx {
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

func (m Model) updateSecurityGroupAddRule(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.sgAddField {
	case 0: // direction select
		return m.updateSGAddSelect(key, sgDirectionOptions)
	case 1: // protocol select
		return m.updateSGAddSelect(key, sgProtocolOptions)
	case 2, 3: // fromPort, toPort (text input)
		return m.updateSGAddTextInput(key)
	case 4: // source/dest (text input)
		return m.updateSGAddTextInput(key)
	case 5: // description (text input, optional)
		return m.updateSGAddTextInput(key)
	}
	return m, nil
}

func (m Model) updateSGAddSelect(key string, options []string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = screenSecurityGroupDetail
	case "up", "k":
		if m.sgAddSelectIdx > 0 {
			m.sgAddSelectIdx--
		}
	case "down", "j":
		if m.sgAddSelectIdx < len(options)-1 {
			m.sgAddSelectIdx++
		}
	case "enter":
		fieldKey := strings.ToLower(sgAddFieldLabels[m.sgAddField])
		if m.sgAddField == 0 {
			m.sgAddValues["direction"] = options[m.sgAddSelectIdx]
		} else {
			m.sgAddValues["protocol"] = options[m.sgAddSelectIdx]
		}
		_ = fieldKey
		m.sgAddField++
		m.sgAddSelectIdx = 0
		m.sgAddInput = ""
		// If protocol is "all", skip port fields
		if m.sgAddField == 2 && m.sgAddValues["protocol"] == "-1" {
			m.sgAddField = 4
		}
	}
	return m, nil
}

func (m Model) updateSGAddTextInput(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = screenSecurityGroupDetail
	case "enter":
		switch m.sgAddField {
		case 2:
			m.sgAddValues["fromPort"] = m.sgAddInput
		case 3:
			m.sgAddValues["toPort"] = m.sgAddInput
		case 4:
			if m.sgAddInput == "" {
				return m, nil // source is required
			}
			m.sgAddValues["source"] = m.sgAddInput
		case 5:
			m.sgAddValues["description"] = m.sgAddInput
			// Last field — execute the add
			m.sgAddInput = ""
			return m.startLoading(m.executeSGAddRule())
		}
		m.sgAddField++
		m.sgAddInput = ""
		// If protocol is "all", skip port fields
		if m.sgAddField == 2 && m.sgAddValues["protocol"] == "-1" {
			m.sgAddField = 4
		}
	case "backspace":
		if len(m.sgAddInput) > 0 {
			m.sgAddInput = m.sgAddInput[:len(m.sgAddInput)-1]
		}
	default:
		if len(key) == 1 {
			m.sgAddInput += key
		}
	}
	return m, nil
}

func (m Model) viewSecurityGroupAddRule() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Add Security Group Rule"))
	b.WriteString("\n\n")

	if m.selectedSecurityGroup != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Security Group: %s (%s)", m.selectedSecurityGroup.Name, m.selectedSecurityGroup.GroupID)))
		b.WriteString("\n\n")
	}

	// Show completed fields
	for i := 0; i < m.sgAddField; i++ {
		label := sgAddFieldLabels[i]
		val := ""
		switch i {
		case 0:
			val = m.sgAddValues["direction"]
		case 1:
			proto := m.sgAddValues["protocol"]
			for j, p := range sgProtocolOptions {
				if p == proto {
					val = sgProtocolLabels[j]
					break
				}
			}
		case 2:
			val = m.sgAddValues["fromPort"]
		case 3:
			val = m.sgAddValues["toPort"]
		case 4:
			val = m.sgAddValues["source"]
		case 5:
			val = m.sgAddValues["description"]
		}
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %s: %s", label, val)))
		b.WriteString("\n")
	}

	// Show current field
	if m.sgAddField < len(sgAddFieldLabels) {
		label := sgAddFieldLabels[m.sgAddField]
		b.WriteString("\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  %s:", label)))
		b.WriteString("\n")

		switch m.sgAddField {
		case 0: // direction select
			for i, opt := range []string{"Ingress (inbound)", "Egress (outbound)"} {
				cursor := "  "
				style := normalStyle
				if i == m.sgAddSelectIdx {
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
				if i == m.sgAddSelectIdx {
					cursor = "> "
					style = selectedStyle
				}
				b.WriteString(style.Render(fmt.Sprintf("  %s%s", cursor, label)))
				b.WriteString("\n")
			}
		default: // text input
			hint := ""
			switch m.sgAddField {
			case 2:
				hint = " (e.g. 443)"
			case 3:
				hint = " (e.g. 443)"
			case 4:
				hint = " (CIDR, IPv6, or sg-xxxxx)"
			case 5:
				hint = " (optional, press enter to skip)"
			}
			b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", m.sgAddInput)))
			if hint != "" {
				b.WriteString(dimStyle.Render(hint))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.sgAddField == 0 || m.sgAddField == 1 {
		b.WriteString(dimStyle.Render("  ↑/↓: select • enter: confirm • esc: cancel"))
	} else {
		b.WriteString(dimStyle.Render("  enter: confirm • esc: cancel"))
	}
	return b.String()
}

// --- Delete Confirm screen ---

func (m Model) updateSecurityGroupDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.selectedSecurityGroup == nil || m.sgDeleteRule == nil {
		m.screen = screenSecurityGroupDetail
		return m, nil
	}

	confirmTarget := m.selectedSecurityGroup.GroupID

	switch msg.String() {
	case "esc":
		m.screen = screenSecurityGroupDetail
	case "enter":
		if m.sgDeleteConfirm == confirmTarget {
			return m.startLoading(m.executeSGDeleteRule())
		}
	case "backspace":
		if len(m.sgDeleteConfirm) > 0 {
			m.sgDeleteConfirm = m.sgDeleteConfirm[:len(m.sgDeleteConfirm)-1]
		}
	default:
		if runes := msg.Runes; len(runes) > 0 {
			m.sgDeleteConfirm += string(runes)
		}
	}
	return m, nil
}

func (m Model) viewSecurityGroupDeleteConfirm() string {
	if m.selectedSecurityGroup == nil || m.sgDeleteRule == nil {
		return ""
	}
	sg := m.selectedSecurityGroup
	rule := m.sgDeleteRule

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(errorStyle.Render("Confirm Rule Deletion"))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render(fmt.Sprintf("  You are about to delete a %s rule from:", m.sgRuleSection)))
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
	if m.sgRuleSection == "egress" {
		sourceLabel = "Destination"
	}
	b.WriteString(normalStyle.Render(fmt.Sprintf("    %s: %s", sourceLabel, source)))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render("  Type the security group ID to confirm:"))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", m.sgDeleteConfirm)))
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render("  enter: confirm • esc: cancel"))
	return b.String()
}
