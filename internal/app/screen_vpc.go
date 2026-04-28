package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

type vpcModel struct {
	vpcs            []awsservice.VPC
	filteredVPCs    []awsservice.VPC
	vpcIdx          int
	subnets         []awsservice.Subnet
	filteredSubnets []awsservice.Subnet
	subnetIdx       int
	selectedVPC     *awsservice.VPC
	selectedSubnet  *awsservice.Subnet
	availableIPs    []string
	filteredIPs     []string
	ipScrollOffset  int
}

func newVPCModel() vpcModel {
	return vpcModel{}
}

func (vm *vpcModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(vm.loadVPCs(*m))
}

func (vm *vpcModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case vpcsLoadedMsg:
		vm.vpcs = msg.vpcs
		m.resetFilter(filterVPCs)
		vm.vpcIdx = 0
		m.screen = screenVPCList
		return *m, nil, true
	case subnetsLoadedMsg:
		vm.subnets = msg.subnets
		m.resetFilter(filterSubnets)
		vm.subnetIdx = 0
		m.screen = screenSubnetList
		return *m, nil, true
	case availableIPsLoadedMsg:
		vm.availableIPs = msg.ips
		m.resetFilter(filterSubnetIPs)
		m.screen = screenSubnetDetail
		return *m, nil, true
	}
	return *m, nil, false
}

func (vm *vpcModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenVPCList:
		newM, cmd := vm.updateVPCList(m, msg)
		return newM, cmd, true
	case screenSubnetList:
		newM, cmd := vm.updateSubnetList(m, msg)
		return newM, cmd, true
	case screenSubnetDetail:
		newM, cmd := vm.updateSubnetDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (vm vpcModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenVPCList:
		return vm.viewVPCList(m), true
	case screenSubnetList:
		return vm.viewSubnetList(m), true
	case screenSubnetDetail:
		return vm.viewSubnetDetail(m), true
	default:
		return "", false
	}
}

func (vm *vpcModel) ApplyFilter(m *Model, target filterTarget) bool {
	switch target {
	case filterVPCs:
		vm.filteredVPCs = applyFilter(vm.vpcs, m.filterValue(target))
		vm.vpcIdx = 0
		return true
	case filterSubnets:
		vm.filteredSubnets = applyFilter(vm.subnets, m.filterValue(target))
		vm.subnetIdx = 0
		return true
	case filterSubnetIPs:
		vm.applyIPFilter(m)
		return true
	default:
		return false
	}
}

func (vm *vpcModel) updateVPCList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterVPCs); handled {
		return *m, cmd
	}

	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		vm.vpcIdx = 0
		m.resetFilter(filterVPCs)
	case "up", "k":
		vm.vpcIdx = previousListIndex(vm.vpcIdx, len(vm.filteredVPCs))
	case "down", "j":
		vm.vpcIdx = nextListIndex(vm.vpcIdx, len(vm.filteredVPCs))
	case "/":
		return *m, m.activateFilter(filterVPCs)
	case "enter":
		if len(vm.filteredVPCs) > 0 && vm.vpcIdx < len(vm.filteredVPCs) {
			selected := vm.filteredVPCs[vm.vpcIdx]
			vm.selectedVPC = &selected
			return m.startLoading(vm.loadSubnets(*m, selected.VPCID))
		}
	}
	return *m, nil
}

func (vm *vpcModel) updateSubnetList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterSubnets); handled {
		return *m, cmd
	}

	switch msg.String() {
	case "q", "esc":
		m.screen = screenVPCList
		vm.subnetIdx = 0
		m.resetFilter(filterSubnets)
	case "up", "k":
		vm.subnetIdx = previousListIndex(vm.subnetIdx, len(vm.filteredSubnets))
	case "down", "j":
		vm.subnetIdx = nextListIndex(vm.subnetIdx, len(vm.filteredSubnets))
	case "/":
		return *m, m.activateFilter(filterSubnets)
	case "enter":
		if len(vm.filteredSubnets) > 0 && vm.subnetIdx < len(vm.filteredSubnets) {
			selected := vm.filteredSubnets[vm.subnetIdx]
			vm.selectedSubnet = &selected
			return m.startLoading(vm.loadAvailableIPs(*m, selected))
		}
	}
	return *m, nil
}

func (vm *vpcModel) updateSubnetDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterSubnetIPs); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenSubnetList
	case "up", "k":
		if vm.ipScrollOffset > 0 {
			vm.ipScrollOffset--
		}
	case "down", "j":
		visibleLines := max(m.height-12, 5)
		if vm.ipScrollOffset < len(vm.filteredIPs)-visibleLines {
			vm.ipScrollOffset++
		}
	case "/":
		return *m, m.activateFilter(filterSubnetIPs)
	}
	return *m, nil
}

func (vm *vpcModel) applyIPFilter(m *Model) {
	query := m.filterValue(filterSubnetIPs)
	if query == "" {
		vm.filteredIPs = vm.availableIPs
	} else {
		var result []string
		for _, ip := range vm.availableIPs {
			if strings.Contains(ip, query) {
				result = append(result, ip)
			}
		}
		vm.filteredIPs = result
	}
	vm.ipScrollOffset = 0
}

func (vm vpcModel) loadVPCs(m Model) tea.Cmd {
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

func (vm vpcModel) loadSubnets(m Model, vpcID string) tea.Cmd {
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

func (vm vpcModel) loadAvailableIPs(m Model, subnet awsservice.Subnet) tea.Cmd {
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

func (vm vpcModel) viewVPCList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("VPCs"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterVPCs))
	b.WriteString("\n\n")

	if len(vm.filteredVPCs) == 0 {
		panel.WriteString(dimStyle.Render("  No matching VPCs"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-11, 5)
		start := 0
		if vm.vpcIdx >= visibleLines {
			start = vm.vpcIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(vm.filteredVPCs))

		for i := start; i < end; i++ {
			vpc := vm.filteredVPCs[i]
			cursor := "  "
			style := normalStyle
			if i == vm.vpcIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterVPCs, vpc.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d VPCs", len(vm.filteredVPCs), len(vm.vpcs))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: select • esc: back • H: home"))
	return b.String()
}

func (vm vpcModel) viewSubnetList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	vpcName := ""
	if vm.selectedVPC != nil {
		vpcName = fmt.Sprintf(" (%s)", vm.selectedVPC.Name)
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Subnets%s", vpcName)))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterSubnets))
	b.WriteString("\n\n")

	if len(vm.filteredSubnets) == 0 {
		panel.WriteString(dimStyle.Render("  No matching subnets"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-11, 5)
		start := 0
		if vm.subnetIdx >= visibleLines {
			start = vm.subnetIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(vm.filteredSubnets))

		for i := start; i < end; i++ {
			s := vm.filteredSubnets[i]
			cursor := "  "
			style := normalStyle
			if i == vm.subnetIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterSubnets, s.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d subnets", len(vm.filteredSubnets), len(vm.subnets))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: detail • esc: back • H: home"))
	return b.String()
}

func (vm vpcModel) viewSubnetDetail(m Model) string {
	if vm.selectedSubnet == nil {
		return ""
	}
	s := vm.selectedSubnet
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Subnet Detail"))
	b.WriteString("\n\n")
	b.WriteString(renderDetailLine("Subnet ID", normalStyle.Render(s.SubnetID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Name", normalStyle.Render(s.Name)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("CIDR", normalStyle.Render(s.CIDR)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("AZ", normalStyle.Render(s.AvailabilityZone)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Available IPs", normalStyle.Render(fmt.Sprintf("%d", len(vm.availableIPs)))))
	b.WriteString("\n\n")

	b.WriteString(m.renderFilterValue(filterSubnetIPs))
	b.WriteString("\n")

	if len(vm.filteredIPs) == 0 {
		b.WriteString(dimStyle.Render("  No matching IPs"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-14, 5)
		start := vm.ipScrollOffset
		end := min(start+visibleLines, len(vm.filteredIPs))

		for _, ip := range vm.filteredIPs[start:end] {
			b.WriteString(normalStyle.Render(fmt.Sprintf("  %s", ip)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d-%d of %d IPs", start+1, end, len(vm.filteredIPs))))
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("↑/↓: scroll • /: filter • esc: back • H: home"))
	return b.String()
}
