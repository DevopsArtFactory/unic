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
	screenRDSList
	screenRDSDetail
	screenRDSConfirm
	screenRoute53ZoneList
	screenRoute53RecordList
	screenRoute53RecordDetail
	screenSecretList
	screenSecretDetail
	screenContextPicker
	screenContextAdd
	screenLoading
	screenError
)

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
	rdsConfirmInput string // typed input for destructive action confirmation
	rdsPolling      bool

	// Route53 browser state
	route53Zones              []awsservice.HostedZone
	filteredRoute53Zones      []awsservice.HostedZone
	route53ZoneIdx            int
	route53ZoneFilter         string
	route53ZoneFilterActive   bool
	selectedRoute53Zone       *awsservice.HostedZone
	route53Records            []awsservice.DNSRecord
	filteredRoute53Records    []awsservice.DNSRecord
	route53RecordIdx          int
	route53RecordFilter       string
	route53RecordFilterActive bool
	selectedRoute53Record     *awsservice.DNSRecord

	// Secrets Manager browser state
	secrets             []awsservice.Secret
	filteredSecrets     []awsservice.Secret
	secretIdx           int
	secretFilter        string
	secretFilterActive  bool
	selectedSecret      *awsservice.SecretDetail

	// Context picker
	configPath         string
	ctxList            []config.ContextInfo
	filteredCtxList    []config.ContextInfo
	ctxIdx             int
	ctxFilterInput     string
	ctxFilterActive    bool
	ctxPrevScreen      screen
	pendingContextName string

	// Context add wizard
	addStep     int // 0=auth_type select, 1+=field input, -1=confirm
	addAuthIdx  int
	addFields   []fieldDef
	addFieldIdx int
	addInput    string
	addValues   map[string]string

	// Caller identity (loaded at startup)
	callerIdentity *awsservice.CallerIdentity

	// Error display
	errMsg string

	// Terminal size
	width  int
	height int
}

// New creates a new app Model.
func New(cfg *config.Config, configPath string) Model {
	services := domain.Catalog()
	return Model{
		cfg:           cfg,
		configPath:    configPath,
		screen:        screenContextPicker,
		ctxPrevScreen: screenServiceList,
		services:      services,
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadContexts()
}

func (m Model) loadCallerIdentity() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			// Non-fatal: just skip identity display
			return callerIdentityMsg{}
		}
		identity, err := repo.GetCallerIdentity(ctx)
		if err != nil {
			return callerIdentityMsg{}
		}
		return callerIdentityMsg{identity: identity}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case callerIdentityMsg:
		m.callerIdentity = msg.identity
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

	case route53ZonesLoadedMsg:
		m.route53Zones = msg.zones
		m.filteredRoute53Zones = msg.zones
		m.route53ZoneIdx = 0
		m.screen = screenRoute53ZoneList
		return m, nil

	case route53RecordsLoadedMsg:
		m.route53Records = msg.records
		m.filteredRoute53Records = msg.records
		m.route53RecordIdx = 0
		m.screen = screenRoute53RecordList
		return m, nil

	case rdsInstancesLoadedMsg:
		m.rdsInstances = msg.instances
		m.filteredRDS = msg.instances
		m.rdsIdx = 0
		m.screen = screenRDSList
		return m, nil

	case secretsLoadedMsg:
		m.secrets = msg.secrets
		m.filteredSecrets = msg.secrets
		m.secretIdx = 0
		m.screen = screenSecretList
		return m, nil

	case secretDetailLoadedMsg:
		m.selectedSecret = msg.detail
		m.screen = screenSecretDetail
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

	case contextsLoadedMsg:
		m.ctxList = msg.contexts
		m.filteredCtxList = msg.contexts
		m.ctxIdx = 0
		m.ctxFilterInput = ""
		m.ctxFilterActive = false
		for i, ctx := range m.filteredCtxList {
			if ctx.Current {
				m.ctxIdx = i
				break
			}
		}
		m.screen = screenContextPicker
		return m, nil

	case ssoLoginDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("SSO login failed: %s", msg.err)
			m.screen = screenError
			return m, tea.ClearScreen
		}
		// SSO login done, now finalize the context switch
		return m, m.finalizeContextSwitch()

	case contextSwitchedMsg:
		m.cfg = msg.cfg
		m.callerIdentity = msg.identity
		m.awsRepo = nil // reset so next AWS call uses new credentials
		m.screen = m.ctxPrevScreen
		return m, tea.ClearScreen

	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		// Global home — return to service list from any screen
		if msg.String() == "H" && m.screen != screenServiceList && m.screen != screenContextPicker {
			m.screen = screenServiceList
			return m, nil
		}
		// Global context switch — C key opens context picker
		if msg.String() == "C" && m.screen != screenContextPicker {
			m.ctxPrevScreen = m.screen
			return m, m.loadContexts()
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
		case screenRoute53ZoneList:
			return m.updateRoute53ZoneList(msg)
		case screenRoute53RecordList:
			return m.updateRoute53RecordList(msg)
		case screenRoute53RecordDetail:
			return m.updateRoute53RecordDetail(msg)
		case screenSecretList:
			return m.updateSecretList(msg)
		case screenSecretDetail:
			return m.updateSecretDetail(msg)
		case screenContextPicker:
			return m.updateContextPicker(msg)
		case screenContextAdd:
			return m.updateContextAdd(msg)
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
	case "esc":
		m.ctxPrevScreen = screenServiceList
		return m, m.loadContexts()
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
			case domain.FeatureRoute53Browser:
				m.screen = screenLoading
				return m, m.loadRoute53Zones()
			case domain.FeatureSecretsBrowser:
				m.screen = screenLoading
				return m, m.loadSecrets()
			}
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

// View renders the current screen.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var v string
	switch m.screen {
	case screenServiceList:
		v = m.viewServiceList()
	case screenFeatureList:
		v = m.viewFeatureList()
	case screenInstanceList:
		v = m.viewInstanceList()
	case screenVPCList:
		v = m.viewVPCList()
	case screenSubnetList:
		v = m.viewSubnetList()
	case screenSubnetDetail:
		v = m.viewSubnetDetail()
	case screenRDSList:
		v = m.viewRDSList()
	case screenRDSDetail:
		v = m.viewRDSDetail()
	case screenRDSConfirm:
		v = m.viewRDSConfirm()
	case screenRoute53ZoneList:
		v = m.viewRoute53ZoneList()
	case screenRoute53RecordList:
		v = m.viewRoute53RecordList()
	case screenRoute53RecordDetail:
		v = m.viewRoute53RecordDetail()
	case screenSecretList:
		v = m.viewSecretList()
	case screenSecretDetail:
		v = m.viewSecretDetail()
	case screenContextPicker:
		v = m.viewContextPicker()
	case screenContextAdd:
		v = m.viewContextAdd()
	case screenLoading:
		v = m.viewLoading()
	case screenError:
		v = m.viewError()
	}

	return m.fitToHeight(v)
}

func (m Model) viewServiceList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Select AWS Service"))
	b.WriteString("\n\n")

	// overhead: status bar (2 lines) + title (1) + blank (1) + blank (1) + footer (1) = 6
	visibleLines := max(m.height-6, 3)
	start := 0
	if m.svcIdx >= visibleLines {
		start = m.svcIdx - visibleLines + 1
	}
	end := min(start+visibleLines, len(m.services))

	for i := start; i < end; i++ {
		svc := m.services[i]
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
	b.WriteString(dimStyle.Render("↑/↓: navigate • enter: select • esc: context • q: quit"))
	return b.String()
}

func (m Model) viewFeatureList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	svcName := m.services[m.svcIdx].Name
	b.WriteString(titleStyle.Render(fmt.Sprintf("%s > Select Feature", svcName)))
	b.WriteString("\n\n")

	// Each selected item takes 2 lines (name + description), others take 1.
	// overhead: status bar (2) + title (1) + blank (1) + blank (1) + footer (1) = 6
	visibleLines := max(m.height-6, 3)
	start := 0
	// Count lines from start to cursor to determine if we need to scroll
	linesFromStart := 0
	for i := 0; i <= m.featIdx && i < len(m.features); i++ {
		linesFromStart++
		if i == m.featIdx {
			linesFromStart++ // selected item has description line
		}
	}
	if linesFromStart > visibleLines {
		// Scroll forward: find start index that fits cursor in view
		linesFromStart = 0
		for i := m.featIdx; i >= 0; i-- {
			needed := 1
			if i == m.featIdx {
				needed = 2
			}
			if linesFromStart+needed > visibleLines {
				start = i + 1
				break
			}
			linesFromStart += needed
		}
	}

	linesUsed := 0
	for i := start; i < len(m.features); i++ {
		feat := m.features[i]
		needed := 1
		if i == m.featIdx {
			needed = 2
		}
		if linesUsed+needed > visibleLines {
			break
		}
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
		linesUsed += needed
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • enter: select • esc: back"))
	return b.String()
}
func (m Model) loadSecrets() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		secrets, err := repo.ListSecrets(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(secrets) == 0 {
			return errMsg{err: fmt.Errorf("no secrets found")}
		}
		return secretsLoadedMsg{secrets: secrets}
	}
}

func (m Model) loadSecretDetail(name string) tea.Cmd {
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
		detail, err := repo.GetSecretDetail(ctx, name)
		if err != nil {
			return errMsg{err: err}
		}
		return secretDetailLoadedMsg{detail: detail}
	}
}

func (m Model) updateSecretList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.secretFilterActive {
		switch key {
		case "esc":
			m.secretFilterActive = false
		case "enter":
			m.secretFilterActive = false
		case "backspace":
			if len(m.secretFilter) > 0 {
				m.secretFilter = m.secretFilter[:len(m.secretFilter)-1]
				m.applySecretFilter()
			}
		default:
			if len(key) == 1 {
				m.secretFilter += key
				m.applySecretFilter()
			}
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.secretFilter = ""
		m.filteredSecrets = m.secrets
		m.secretIdx = 0
	case "up", "k":
		if m.secretIdx > 0 {
			m.secretIdx--
		}
	case "down", "j":
		if m.secretIdx < len(m.filteredSecrets)-1 {
			m.secretIdx++
		}
	case "/":
		m.secretFilterActive = true
	case "enter":
		if len(m.filteredSecrets) > 0 && m.secretIdx < len(m.filteredSecrets) {
			selected := m.filteredSecrets[m.secretIdx]
			m.screen = screenLoading
			return m, m.loadSecretDetail(selected.Name)
		}
	}
	return m, nil
}

func (m Model) updateSecretDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.selectedSecret = nil
		m.screen = screenSecretList
	}
	return m, nil
}

func (m *Model) applySecretFilter() {
	if m.secretFilter == "" {
		m.filteredSecrets = m.secrets
	} else {
		query := strings.ToLower(m.secretFilter)
		var result []awsservice.Secret
		for _, s := range m.secrets {
			if strings.Contains(s.FilterText(), query) {
				result = append(result, s)
			}
		}
		m.filteredSecrets = result
	}
	m.secretIdx = 0
}

func (m Model) viewSecretList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Secrets Manager"))
	b.WriteString("\n")

	if m.secretFilterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.secretFilter)))
	} else if m.secretFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.secretFilter)))
	}
	b.WriteString("\n\n")

	if len(m.filteredSecrets) == 0 {
		b.WriteString(dimStyle.Render("  No matching secrets"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-8, 5)
		start := 0
		if m.secretIdx >= visibleLines {
			start = m.secretIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredSecrets))

		for i := start; i < end; i++ {
			s := m.filteredSecrets[i]
			cursor := "  "
			style := normalStyle
			if i == m.secretIdx {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, s.DisplayTitle())))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d secrets", len(m.filteredSecrets), len(m.secrets))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • enter: detail • esc: back • H: home"))
	return b.String()
}

func (m Model) viewSecretDetail() string {
	if m.selectedSecret == nil {
		return ""
	}
	d := m.selectedSecret
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Secret Detail"))
	b.WriteString("\n\n")

	labelStyle := lipgloss.NewStyle().Width(14)
	b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("Name"), d.Name)))
	b.WriteString("\n")

	kmsKey := d.KMSKeyID
	if kmsKey == "" {
		kmsKey = dimStyle.Render("(aws/secretsmanager)")
	}
	b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("Encryption Key"), kmsKey)))
	b.WriteString("\n\n")

	if len(d.Values) > 0 {
		b.WriteString(titleStyle.Render("Key / Value"))
		b.WriteString("\n\n")

		keys := make([]string, 0, len(d.Values))
		for k := range d.Values {
			keys = append(keys, k)
		}
		for i := 1; i < len(keys); i++ {
			for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
				keys[j], keys[j-1] = keys[j-1], keys[j]
			}
		}

		for _, k := range keys {
			b.WriteString(fmt.Sprintf("  %s  %s\n", dimStyle.Render(k), normalStyle.Render(d.Values[k])))
		}
	} else if d.Raw != "" {
		b.WriteString(titleStyle.Render("Value"))
		b.WriteString("\n\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  %s", d.Raw)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("esc: back • H: home"))
	return b.String()
}
