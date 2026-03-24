package app

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	screenRDSList
	screenRDSDetail
	screenRDSConfirm
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

type rdsInstancesLoadedMsg struct {
	instances []awsservice.RDSInstance
}

type rdsActionDoneMsg struct {
	action     string
	instanceID string
	err        error
}

type rdsStatusRefreshedMsg struct {
	instance *awsservice.RDSInstance
	err      error
}

type rdsTickMsg struct {
	instanceID string
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
	filteredIPs    []string
	ipScrollOffset int
	ipFilter       string
	ipFilterActive bool

	// RDS browser state
	rdsInstances    []awsservice.RDSInstance
	filteredRDS     []awsservice.RDSInstance
	rdsIdx          int
	rdsFilter       string
	rdsFilterActive bool
	selectedRDS     *awsservice.RDSInstance
	rdsAction       string // "start", "stop", "failover"
	rdsPolling      bool

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
		m.filteredIPs = msg.ips
		m.ipScrollOffset = 0
		m.ipFilter = ""
		m.ipFilterActive = false
		m.screen = screenSubnetDetail
		return m, nil

	case rdsInstancesLoadedMsg:
		m.rdsInstances = msg.instances
		m.filteredRDS = msg.instances
		m.rdsIdx = 0
		m.screen = screenRDSList
		return m, nil

	case rdsActionDoneMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil
		}
		m.rdsPolling = true
		m.screen = screenRDSDetail
		return m, m.tickRDSPoll(msg.instanceID)

	case rdsStatusRefreshedMsg:
		if msg.err != nil {
			m.rdsPolling = false
			return m, nil
		}
		m.selectedRDS = msg.instance
		// Update the instance in the list
		for i, inst := range m.rdsInstances {
			if inst.DBInstanceID == msg.instance.DBInstanceID {
				m.rdsInstances[i] = *msg.instance
				break
			}
		}
		m.applyRDSFilter()
		if awsservice.IsTransitionalStatus(msg.instance.Status) {
			return m, m.tickRDSPoll(msg.instance.DBInstanceID)
		}
		m.rdsPolling = false
		return m, nil

	case rdsTickMsg:
		if m.rdsPolling {
			return m, m.pollRDSStatus(msg.instanceID)
		}
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
		// Global home — return to service list from any screen
		if msg.String() == "H" && m.screen != screenServiceList {
			m.screen = screenServiceList
			return m, nil
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
		case screenRDSList:
			return m.updateRDSList(msg)
		case screenRDSDetail:
			return m.updateRDSDetail(msg)
		case screenRDSConfirm:
			return m.updateRDSConfirm(msg)
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
			case domain.FeatureRDSBrowser:
				m.screen = screenLoading
				return m, m.loadRDSInstances()
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
	key := msg.String()

	if m.ipFilterActive {
		switch key {
		case "esc", "enter":
			m.ipFilterActive = false
		case "backspace":
			if len(m.ipFilter) > 0 {
				m.ipFilter = m.ipFilter[:len(m.ipFilter)-1]
				m.applyIPFilter()
			}
		default:
			if len(key) == 1 {
				m.ipFilter += key
				m.applyIPFilter()
			}
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

func (m Model) updateRDSList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.rdsFilterActive {
		switch key {
		case "esc":
			m.rdsFilterActive = false
		case "enter":
			m.rdsFilterActive = false
		case "backspace":
			if len(m.rdsFilter) > 0 {
				m.rdsFilter = m.rdsFilter[:len(m.rdsFilter)-1]
				m.applyRDSFilter()
			}
		default:
			if len(key) == 1 {
				m.rdsFilter += key
				m.applyRDSFilter()
			}
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.rdsFilter = ""
		m.filteredRDS = m.rdsInstances
		m.rdsIdx = 0
	case "up", "k":
		if m.rdsIdx > 0 {
			m.rdsIdx--
		}
	case "down", "j":
		if m.rdsIdx < len(m.filteredRDS)-1 {
			m.rdsIdx++
		}
	case "/":
		m.rdsFilterActive = true
	case "enter":
		if len(m.filteredRDS) > 0 && m.rdsIdx < len(m.filteredRDS) {
			selected := m.filteredRDS[m.rdsIdx]
			m.selectedRDS = &selected
			m.screen = screenRDSDetail
		}
	}
	return m, nil
}

func (m Model) updateRDSDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.rdsPolling = false
		m.screen = screenRDSList
	case "s":
		if m.selectedRDS != nil && m.selectedRDS.CanStart() {
			m.rdsAction = "start"
			m.screen = screenRDSConfirm
		}
	case "x":
		if m.selectedRDS != nil && m.selectedRDS.CanStop() {
			m.rdsAction = "stop"
			m.screen = screenRDSConfirm
		}
	case "f":
		if m.selectedRDS != nil && m.selectedRDS.CanFailover() {
			m.rdsAction = "failover"
			m.screen = screenRDSConfirm
		}
	case "r":
		if m.selectedRDS != nil {
			return m, m.pollRDSStatus(m.selectedRDS.DBInstanceID)
		}
	}
	return m, nil
}

func (m Model) updateRDSConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		if m.selectedRDS != nil {
			m.screen = screenRDSDetail
			return m, m.executeRDSAction(m.rdsAction, m.selectedRDS.DBInstanceID)
		}
	case "n", "esc":
		m.screen = screenRDSDetail
	}
	return m, nil
}

func (m *Model) applyRDSFilter() {
	if m.rdsFilter == "" {
		m.filteredRDS = m.rdsInstances
	} else {
		query := strings.ToLower(m.rdsFilter)
		var result []awsservice.RDSInstance
		for _, inst := range m.rdsInstances {
			if strings.Contains(inst.FilterText(), query) {
				result = append(result, inst)
			}
		}
		m.filteredRDS = result
	}
	m.rdsIdx = 0
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

func (m Model) loadRDSInstances() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		instances, err := repo.ListDBInstances(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(instances) == 0 {
			return errMsg{err: fmt.Errorf("no RDS instances found")}
		}
		return rdsInstancesLoadedMsg{instances: instances}
	}
}

func (m Model) executeRDSAction(action, dbInstanceID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return rdsActionDoneMsg{action: action, instanceID: dbInstanceID, err: err}
			}
		}

		var err error
		switch action {
		case "start":
			err = repo.StartDBInstance(ctx, dbInstanceID)
		case "stop":
			err = repo.StopDBInstance(ctx, dbInstanceID)
		case "failover":
			err = repo.RebootDBInstance(ctx, dbInstanceID, true)
		}
		return rdsActionDoneMsg{action: action, instanceID: dbInstanceID, err: err}
	}
}

func (m Model) pollRDSStatus(dbInstanceID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return rdsStatusRefreshedMsg{err: err}
			}
		}

		inst, err := repo.DescribeDBInstance(ctx, dbInstanceID)
		return rdsStatusRefreshedMsg{instance: inst, err: err}
	}
}

func (m Model) tickRDSPoll(dbInstanceID string) tea.Cmd {
	return tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
		return rdsTickMsg{instanceID: dbInstanceID}
	})
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
	case screenRDSList:
		return m.viewRDSList()
	case screenRDSDetail:
		return m.viewRDSDetail()
	case screenRDSConfirm:
		return m.viewRDSConfirm()
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
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • enter: connect • esc: back • H: home"))
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
	b.WriteString(dimStyle.Render("↑/↓: navigate • enter: select • esc: back • H: home"))
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
	b.WriteString(dimStyle.Render("↑/↓: navigate • enter: detail • esc: back • H: home"))
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

func (m Model) viewRDSList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("RDS Instances"))
	b.WriteString("\n")

	// Filter bar
	if m.rdsFilterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.rdsFilter)))
	} else if m.rdsFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.rdsFilter)))
	}
	b.WriteString("\n\n")

	if len(m.filteredRDS) == 0 {
		b.WriteString(dimStyle.Render("  No matching instances"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-8, 5)
		start := 0
		if m.rdsIdx >= visibleLines {
			start = m.rdsIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredRDS))

		for i := start; i < end; i++ {
			inst := m.filteredRDS[i]
			cursor := "  "
			style := normalStyle
			if i == m.rdsIdx {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, inst.DisplayTitle())))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d instances", len(m.filteredRDS), len(m.rdsInstances))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • enter: detail • esc: back • H: home"))
	return b.String()
}

func (m Model) viewRDSDetail() string {
	if m.selectedRDS == nil {
		return ""
	}
	r := m.selectedRDS
	var b strings.Builder
	b.WriteString(titleStyle.Render("RDS Instance Detail"))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render(fmt.Sprintf("  Identifier : %s", r.DBInstanceID)))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  Engine     : %s %s", r.Engine, r.EngineVersion)))
	b.WriteString("\n")

	// Color-code status
	statusStr := r.Status
	if r.Status == "available" {
		statusStr = selectedStyle.Render(r.Status)
	} else if awsservice.IsTransitionalStatus(r.Status) {
		statusStr = filterStyle.Render(r.Status)
	} else if r.Status == "stopped" || r.Status == "failed" {
		statusStr = errorStyle.Render(r.Status)
	}
	pollingIndicator := ""
	if m.rdsPolling {
		pollingIndicator = filterStyle.Render(" (polling...)")
	}
	b.WriteString(fmt.Sprintf("  Status     : %s%s", statusStr, pollingIndicator))
	b.WriteString("\n")

	b.WriteString(normalStyle.Render(fmt.Sprintf("  Class      : %s", r.InstanceClass)))
	b.WriteString("\n")
	multiAZStr := "No"
	if r.MultiAZ {
		multiAZStr = "Yes"
	}
	b.WriteString(normalStyle.Render(fmt.Sprintf("  Multi-AZ   : %s", multiAZStr)))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  Storage    : %d GB", r.StorageGB)))
	b.WriteString("\n")
	endpoint := r.Endpoint
	if endpoint == "" {
		endpoint = dimStyle.Render("(unavailable)")
	}
	b.WriteString(normalStyle.Render(fmt.Sprintf("  Endpoint   : %s", endpoint)))
	b.WriteString("\n")
	if r.ClusterID != "" {
		b.WriteString(normalStyle.Render(fmt.Sprintf("  Cluster    : %s", r.ClusterID)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Actions"))
	b.WriteString("\n")
	if r.CanStart() {
		b.WriteString(normalStyle.Render("  [s] Start"))
		b.WriteString("\n")
	} else {
		b.WriteString(dimStyle.Render("  [s] Start"))
		b.WriteString("\n")
	}
	if r.CanStop() {
		b.WriteString(normalStyle.Render("  [x] Stop"))
		b.WriteString("\n")
	} else {
		b.WriteString(dimStyle.Render("  [x] Stop"))
		b.WriteString("\n")
	}
	if r.CanFailover() {
		b.WriteString(normalStyle.Render("  [f] Failover"))
		b.WriteString("\n")
	} else {
		b.WriteString(dimStyle.Render("  [f] Failover"))
		b.WriteString("\n")
	}
	b.WriteString(normalStyle.Render("  [r] Refresh"))
	b.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("esc: back • H: home"))
	return b.String()
}

func (m Model) viewRDSConfirm() string {
	if m.selectedRDS == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(errorStyle.Render("Confirm Action"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  Are you sure you want to %s instance %s?",
		m.rdsAction, m.selectedRDS.DBInstanceID)))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("  [y] Yes  [n] No"))
	b.WriteString("\n")
	return b.String()
}
