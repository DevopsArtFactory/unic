package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"unic/internal/config"
	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

// screen represents the current TUI screen.
type screen int

const (
	screenServiceList screen = iota
	screenFeatureList
	screenInstanceList
	screenVPCList
	screenSubnetList
	screenSubnetDetail
	screenLoading
	screenError
)

// Messages for Bubbletea commands.
type instancesLoadedMsg struct {
	instances []awsservice.EC2Instance
}

type vpcsLoadedMsg struct {
	vpcs []awsservice.VPC
}

type subnetsLoadedMsg struct {
	subnets []awsservice.Subnet
}

type availableIPsLoadedMsg struct {
	subnet awsservice.Subnet
	ips    []string
}

type errMsg struct {
	err error
}

type ssmSessionDoneMsg struct {
	err error
}

// Model is the root Bubbletea model.
type Model struct {
	cfg      *config.Config
	awsRepo  *awsservice.AwsRepository
	screen   screen
	quitting bool

	// Service selection
	services []domain.Service
	svcIdx   int

	// Feature selection
	features []domain.Feature
	featIdx  int

	// Instance list with filtering
	instances    []awsservice.EC2Instance
	filtered     []awsservice.EC2Instance
	instIdx      int
	filterInput  string
	filterActive bool

	// SSM session state
	selectedInstance *awsservice.EC2Instance

	// VPC browser state
	vpcs           []awsservice.VPC
	filteredVPCs   []awsservice.VPC
	vpcIdx         int
	subnets        []awsservice.Subnet
	subnetIdx      int
	selectedVPC    *awsservice.VPC
	selectedSubnet *awsservice.Subnet
	availableIPs   []string
	ipScrollOffset int

	// Error display
	errMsg string

	// Terminal size
	width  int
	height int
}

// New creates a new app Model.
func New(cfg *config.Config) Model {
	services := domain.Catalog()
	return Model{
		cfg:      cfg,
		screen:   screenServiceList,
		services: services,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case instancesLoadedMsg:
		m.instances = msg.instances
		m.filtered = msg.instances
		m.instIdx = 0
		m.screen = screenInstanceList
		return m, nil

	case vpcsLoadedMsg:
		m.vpcs = msg.vpcs
		m.filteredVPCs = msg.vpcs
		m.vpcIdx = 0
		m.screen = screenVPCList
		return m, nil

	case subnetsLoadedMsg:
		m.subnets = msg.subnets
		m.subnetIdx = 0
		m.screen = screenSubnetList
		return m, nil

	case availableIPsLoadedMsg:
		m.availableIPs = msg.ips
		m.ipScrollOffset = 0
		m.screen = screenSubnetDetail
		return m, nil

	case errMsg:
		m.errMsg = msg.err.Error()
		m.screen = screenError
		return m, nil

	case ssmSessionDoneMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil
		}
		// Return to instance list after session ends
		m.screen = screenInstanceList
		return m, nil

	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		switch m.screen {
		case screenServiceList:
			return m.updateServiceList(msg)
		case screenFeatureList:
			return m.updateFeatureList(msg)
		case screenInstanceList:
			return m.updateInstanceList(msg)
		case screenVPCList:
			return m.updateVPCList(msg)
		case screenSubnetList:
			return m.updateSubnetList(msg)
		case screenSubnetDetail:
			return m.updateSubnetDetail(msg)
		case screenError:
			return m.updateError(msg)
		}
	}

	return m, nil
}

func (m Model) updateServiceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.svcIdx > 0 {
			m.svcIdx--
		}
	case "down", "j":
		if m.svcIdx < len(m.services)-1 {
			m.svcIdx++
		}
	case "enter":
		if m.svcIdx < len(m.services) {
			m.features = m.services[m.svcIdx].Features
			m.featIdx = 0
			m.screen = screenFeatureList
		}
	}
	return m, nil
}

func (m Model) updateFeatureList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenServiceList
	case "up", "k":
		if m.featIdx > 0 {
			m.featIdx--
		}
	case "down", "j":
		if m.featIdx < len(m.features)-1 {
			m.featIdx++
		}
	case "enter":
		if m.featIdx < len(m.features) {
			feat := m.features[m.featIdx]
			switch feat.Kind {
			case domain.FeatureSSMSession:
				m.screen = screenLoading
				return m, m.loadInstances()
			case domain.FeatureVPCBrowser:
				m.screen = screenLoading
				return m, m.loadVPCs()
			}
		}
	}
	return m, nil
}

func (m Model) updateInstanceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// If filter is active, handle text input
	if m.filterActive {
		switch key {
		case "esc":
			m.filterActive = false
		case "enter":
			m.filterActive = false
		case "backspace":
			if len(m.filterInput) > 0 {
				m.filterInput = m.filterInput[:len(m.filterInput)-1]
				m.applyFilter()
			}
		default:
			if len(key) == 1 {
				m.filterInput += key
				m.applyFilter()
			}
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.filterInput = ""
		m.filtered = m.instances
		m.instIdx = 0
	case "up", "k":
		if m.instIdx > 0 {
			m.instIdx--
		}
	case "down", "j":
		if m.instIdx < len(m.filtered)-1 {
			m.instIdx++
		}
	case "/":
		m.filterActive = true
	case "enter":
		if len(m.filtered) > 0 && m.instIdx < len(m.filtered) {
			return m, m.startSSMSession(m.filtered[m.instIdx])
		}
	}
	return m, nil
}

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
	switch msg.String() {
	case "q", "esc":
		m.screen = screenSubnetList
	case "up", "k":
		if m.ipScrollOffset > 0 {
			m.ipScrollOffset--
		}
	case "down", "j":
		visibleLines := max(m.height-12, 5)
		if m.ipScrollOffset < len(m.availableIPs)-visibleLines {
			m.ipScrollOffset++
		}
	}
	return m, nil
}

func (m Model) updateError(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc", "enter":
		m.screen = screenServiceList
	}
	return m, nil
}

func (m *Model) applyFilter() {
	if m.filterInput == "" {
		m.filtered = m.instances
	} else {
		query := strings.ToLower(m.filterInput)
		var result []awsservice.EC2Instance
		for _, inst := range m.instances {
			if strings.Contains(inst.FilterText(), query) {
				result = append(result, inst)
			}
		}
		m.filtered = result
	}
	m.instIdx = 0
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

func (m Model) loadInstances() tea.Cmd {
	return func() tea.Msg {
		if err := awsservice.CheckPluginInstalled(); err != nil {
			return errMsg{err: err}
		}

		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		instances, err := repo.ListRunningInstances(ctx)
		if err != nil {
			return errMsg{err: err}
		}

		if len(instances) == 0 {
			return errMsg{err: fmt.Errorf("no running EC2 instances found")}
		}

		return instancesLoadedMsg{instances: instances}
	}
}

func (m Model) startSSMSession(inst awsservice.EC2Instance) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Initialize AWS repo if needed
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}

		sess, endpoint, err := repo.StartSession(ctx, inst.InstanceID)
		if err != nil {
			return errMsg{err: err}
		}

		cmd, err := awsservice.BuildPluginCommand(sess, repo.Region, repo.Profile, inst.InstanceID, endpoint)
		if err != nil {
			return errMsg{err: err}
		}

		execCmd := tea.ExecProcess(cmd, func(err error) tea.Msg {
			// Terminate session after plugin exits
			if sess.SessionId != nil {
				_ = repo.TerminateSession(context.Background(), *sess.SessionId)
			}
			return ssmSessionDoneMsg{err: err}
		})
		return execCmd()
	}
}

// View renders the current screen.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	switch m.screen {
	case screenServiceList:
		return m.viewServiceList()
	case screenFeatureList:
		return m.viewFeatureList()
	case screenInstanceList:
		return m.viewInstanceList()
	case screenVPCList:
		return m.viewVPCList()
	case screenSubnetList:
		return m.viewSubnetList()
	case screenSubnetDetail:
		return m.viewSubnetDetail()
	case screenLoading:
		return m.viewLoading()
	case screenError:
		return m.viewError()
	}

	return ""
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	normalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	filterStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

func (m Model) viewServiceList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Select AWS Service"))
	b.WriteString("\n\n")

	for i, svc := range m.services {
		cursor := "  "
		style := normalStyle
		if i == m.svcIdx {
			cursor = "> "
			style = selectedStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, svc.Name)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • enter: select • q: quit"))
	return b.String()
}

func (m Model) viewFeatureList() string {
	var b strings.Builder
	svcName := m.services[m.svcIdx].Name
	b.WriteString(titleStyle.Render(fmt.Sprintf("%s > Select Feature", svcName)))
	b.WriteString("\n\n")

	for i, feat := range m.features {
		cursor := "  "
		style := normalStyle
		if i == m.featIdx {
			cursor = "> "
			style = selectedStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, feat.Kind)))
		b.WriteString("\n")
		if i == m.featIdx {
			b.WriteString(dimStyle.Render(fmt.Sprintf("    %s", feat.Description)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • enter: select • esc: back"))
	return b.String()
}

func (m Model) viewInstanceList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("EC2 Instances (Running)"))
	b.WriteString("\n")

	// Filter bar
	if m.filterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.filterInput)))
	} else if m.filterInput != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.filterInput)))
	}
	b.WriteString("\n\n")

	if len(m.filtered) == 0 {
		b.WriteString(dimStyle.Render("  No matching instances"))
		b.WriteString("\n")
	} else {
		// Calculate visible range for scrolling
		visibleLines := max(m.height-8, 5)
		start := 0
		if m.instIdx >= visibleLines {
			start = m.instIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filtered))

		for i := start; i < end; i++ {
			inst := m.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == m.instIdx {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, inst.DisplayTitle())))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d instances", len(m.filtered), len(m.instances))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • enter: connect • esc: back"))
	return b.String()
}

func (m Model) viewLoading() string {
	return titleStyle.Render("Loading...")
}

func (m Model) viewError() string {
	var b strings.Builder
	b.WriteString(errorStyle.Render("Error"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render(m.errMsg))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("enter/esc: go back • q: quit"))
	return b.String()
}

func (m Model) viewVPCList() string {
	var b strings.Builder
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
	b.WriteString(dimStyle.Render("↑/↓: navigate • enter: select • esc: back"))
	return b.String()
}

func (m Model) viewSubnetList() string {
	var b strings.Builder
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
	b.WriteString(dimStyle.Render("↑/↓: navigate • enter: detail • esc: back"))
	return b.String()
}

func (m Model) viewSubnetDetail() string {
	if m.selectedSubnet == nil {
		return ""
	}
	s := m.selectedSubnet
	var b strings.Builder
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

	if len(m.availableIPs) == 0 {
		b.WriteString(dimStyle.Render("  No available IPs"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-12, 5)
		start := m.ipScrollOffset
		end := min(start+visibleLines, len(m.availableIPs))

		for _, ip := range m.availableIPs[start:end] {
			b.WriteString(normalStyle.Render(fmt.Sprintf("  %s", ip)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d-%d of %d IPs", start+1, end, len(m.availableIPs))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: scroll • esc: back"))
	return b.String()
}
