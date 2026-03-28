package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsservice "unic/internal/services/aws"
)

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

func (m Model) updateSecurityGroupList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.sgFilterActive {
		switch key {
		case "esc":
			m.sgFilterActive = false
		case "enter":
			m.sgFilterActive = false
		case "backspace":
			if len(m.sgFilter) > 0 {
				m.sgFilter = m.sgFilter[:len(m.sgFilter)-1]
				m.applySecurityGroupFilter()
			}
		default:
			if len(key) == 1 {
				m.sgFilter += key
				m.applySecurityGroupFilter()
			}
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.sgFilter = ""
		m.filteredSecurityGroups = m.securityGroups
		m.sgIdx = 0
	case "up", "k":
		if m.sgIdx > 0 {
			m.sgIdx--
		}
	case "down", "j":
		if m.sgIdx < len(m.filteredSecurityGroups)-1 {
			m.sgIdx++
		}
	case "/":
		m.sgFilterActive = true
	case "r":
		m.screen = screenLoading
		m.sgFilter = ""
		m.sgIdx = 0
		return m, m.loadSecurityGroups()
	case "enter":
		if len(m.filteredSecurityGroups) > 0 && m.sgIdx < len(m.filteredSecurityGroups) {
			selected := m.filteredSecurityGroups[m.sgIdx]
			m.selectedSecurityGroup = &selected
			m.screen = screenSecurityGroupDetail
		}
	}
	return m, nil
}

func (m *Model) applySecurityGroupFilter() {
	if m.sgFilter == "" {
		m.filteredSecurityGroups = m.securityGroups
	} else {
		query := strings.ToLower(m.sgFilter)
		var result []awsservice.SecurityGroup
		for _, sg := range m.securityGroups {
			if strings.Contains(sg.FilterText(), query) {
				result = append(result, sg)
			}
		}
		m.filteredSecurityGroups = result
	}
	m.sgIdx = 0
}

func (m Model) updateSecurityGroupDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenSecurityGroupList
	}
	return m, nil
}

func (m Model) viewSecurityGroupList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Security Groups"))
	b.WriteString("\n")

	if m.sgFilterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.sgFilter)))
	} else if m.sgFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.sgFilter)))
	}
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
	b.WriteString(titleStyle.Render("Inbound Rules"))
	b.WriteString("\n")
	if len(sg.IngressRules) == 0 {
		b.WriteString(dimStyle.Render("  No inbound rules"))
		b.WriteString("\n")
	} else {
		protoCol := lipgloss.NewStyle().Width(8)
		portCol := lipgloss.NewStyle().Width(14)
		b.WriteString(dimStyle.Render("  " + protoCol.Render("PROTO") + portCol.Render("PORT") + "SOURCE"))
		b.WriteString("\n")
		for _, rule := range sg.IngressRules {
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
			b.WriteString(normalStyle.Render(row))
			b.WriteString("\n")
		}
	}

	// Outbound rules
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Outbound Rules"))
	b.WriteString("\n")
	if len(sg.EgressRules) == 0 {
		b.WriteString(dimStyle.Render("  No outbound rules"))
		b.WriteString("\n")
	} else {
		protoCol := lipgloss.NewStyle().Width(8)
		portCol := lipgloss.NewStyle().Width(14)
		b.WriteString(dimStyle.Render("  " + protoCol.Render("PROTO") + portCol.Render("PORT") + "DESTINATION"))
		b.WriteString("\n")
		for _, rule := range sg.EgressRules {
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
			dest := rule.CIDRV4
			if dest == "" {
				dest = rule.CIDRV6
			}
			if dest == "" && rule.ReferencedSGID != "" {
				dest = rule.ReferencedSGID
			}
			if dest == "" {
				dest = "-"
			}
			row := "  " + protoCol.Render(proto) + portCol.Render(portRange) + dest
			if rule.Description != "" {
				row += dimStyle.Render("  " + rule.Description)
			}
			b.WriteString(normalStyle.Render(row))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("esc: back • H: home"))
	return b.String()
}
