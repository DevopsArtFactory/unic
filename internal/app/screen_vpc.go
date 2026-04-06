package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func (m Model) updateVPCList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		m.vpcIdx = 0
	case "up", "k":
		if m.vpcIdx > 0 {
			m.vpcIdx--
		}
	case "down", "j":
		if m.vpcIdx < len(m.filteredVPCs)-1 {
			m.vpcIdx++
		}
	case "enter":
		if len(m.filteredVPCs) > 0 && m.vpcIdx < len(m.filteredVPCs) {
			selected := m.filteredVPCs[m.vpcIdx]
			m.selectedVPC = &selected
			m.screen = screenLoading
			return m, m.loadSubnets(selected.VPCID)
		}
	}
	return m, nil
}

func (m Model) updateSubnetList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenVPCList
		m.subnetIdx = 0
	case "up", "k":
		if m.subnetIdx > 0 {
			m.subnetIdx--
		}
	case "down", "j":
		if m.subnetIdx < len(m.subnets)-1 {
			m.subnetIdx++
		}
	case "enter":
		if len(m.subnets) > 0 && m.subnetIdx < len(m.subnets) {
			selected := m.subnets[m.subnetIdx]
			m.selectedSubnet = &selected
			m.screen = screenLoading
			return m, m.loadAvailableIPs(selected)
		}
	}
	return m, nil
}

func (m Model) updateSubnetDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.ipFilterActive {
		newFilter, deactivate, changed := handleFilterKey(key, m.ipFilter)
		m.ipFilter = newFilter
		if deactivate {
			m.ipFilterActive = false
		}
		if changed {
			m.applyIPFilter()
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenSubnetList
	case "up", "k":
		if m.ipScrollOffset > 0 {
			m.ipScrollOffset--
		}
	case "down", "j":
		visibleLines := max(m.height-12, 5)
		if m.ipScrollOffset < len(m.filteredIPs)-visibleLines {
			m.ipScrollOffset++
		}
	case "/":
		m.ipFilterActive = true
	}
	return m, nil
}

func (m *Model) applyIPFilter() {
	if m.ipFilter == "" {
		m.filteredIPs = m.availableIPs
	} else {
		var result []string
		for _, ip := range m.availableIPs {
			if strings.Contains(ip, m.ipFilter) {
				result = append(result, ip)
			}
		}
		m.filteredIPs = result
	}
	m.ipScrollOffset = 0
}

func (m Model) loadVPCs() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		vpcs, err := repo.ListVPCs(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(vpcs) == 0 {
			return errMsg{err: fmt.Errorf("no VPCs found")}
		}
		return vpcsLoadedMsg{vpcs: vpcs}
	}
}

func (m Model) loadSubnets(vpcID string) tea.Cmd {
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

		subnets, err := repo.ListSubnets(ctx, vpcID)
		if err != nil {
			return errMsg{err: err}
		}
		if len(subnets) == 0 {
			return errMsg{err: fmt.Errorf("no subnets found in VPC %s", vpcID)}
		}
		return subnetsLoadedMsg{subnets: subnets}
	}
}

func (m Model) loadAvailableIPs(subnet awsservice.Subnet) tea.Cmd {
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
		ips, err := repo.ListAvailableIPs(ctx, subnet.SubnetID, subnet.CIDR)
		if err != nil {
			return errMsg{err: err}
		}
		return availableIPsLoadedMsg{subnet: subnet, ips: ips}
	}
}

func (m Model) viewVPCList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("VPCs"))
	b.WriteString("\n\n")

	if len(m.filteredVPCs) == 0 {
		b.WriteString(dimStyle.Render("  No VPCs found"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-8, 5)
		start := 0
		if m.vpcIdx >= visibleLines {
			start = m.vpcIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredVPCs))

		for i := start; i < end; i++ {
			vpc := m.filteredVPCs[i]
			cursor := "  "
			style := normalStyle
			if i == m.vpcIdx {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, vpc.DisplayTitle())))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d VPCs", len(m.filteredVPCs))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • enter: select • esc: back • H: home"))
	return b.String()
}

func (m Model) viewSubnetList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	vpcName := ""
	if m.selectedVPC != nil {
		vpcName = fmt.Sprintf(" (%s)", m.selectedVPC.Name)
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Subnets%s", vpcName)))
	b.WriteString("\n\n")

	if len(m.subnets) == 0 {
		b.WriteString(dimStyle.Render("  No subnets found"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-8, 5)
		start := 0
		if m.subnetIdx >= visibleLines {
			start = m.subnetIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.subnets))

		for i := start; i < end; i++ {
			s := m.subnets[i]
			cursor := "  "
			style := normalStyle
			if i == m.subnetIdx {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, s.DisplayTitle())))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d subnets", len(m.subnets))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • enter: detail • esc: back • H: home"))
	return b.String()
}

func (m Model) viewSubnetDetail() string {
	if m.selectedSubnet == nil {
		return ""
	}
	s := m.selectedSubnet
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Subnet Detail"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  Subnet ID  : %s", s.SubnetID)))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  Name       : %s", s.Name)))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  CIDR       : %s", s.CIDR)))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  AZ         : %s", s.AvailabilityZone)))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  Available IPs : %d", len(m.availableIPs))))
	b.WriteString("\n\n")

	// Filter bar
	if m.ipFilterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.ipFilter)))
	} else if m.ipFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.ipFilter)))
	}
	b.WriteString("\n")

	if len(m.filteredIPs) == 0 {
		b.WriteString(dimStyle.Render("  No matching IPs"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-14, 5)
		start := m.ipScrollOffset
		end := min(start+visibleLines, len(m.filteredIPs))

		for _, ip := range m.filteredIPs[start:end] {
			b.WriteString(normalStyle.Render(fmt.Sprintf("  %s", ip)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d-%d of %d IPs", start+1, end, len(m.filteredIPs))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: scroll • /: filter • esc: back • H: home"))
	return b.String()
}
