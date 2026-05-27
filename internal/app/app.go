package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"unic/internal/config"
	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
	"unic/internal/update"
)

// screen represents the current TUI screen.
type screen int

const (
	screenServiceList screen = iota
	screenFeatureList
	screenInstanceList
	screenEC2InstanceBrowserList
	screenEC2InstanceBrowserDetail
	screenEC2InstanceBrowserRelatedList
	screenEC2InstanceBrowserRelatedDetail
	screenVPCList
	screenSubnetList
	screenSubnetDetail
	screenReachabilityRegionList
	screenReachabilitySourceList
	screenReachabilityDestinationList
	screenReachabilityConfig
	screenReachabilityResult
	screenRDSList
	screenRDSDetail
	screenRDSConfirm
	screenRoute53ZoneList
	screenRoute53RecordList
	screenRoute53RecordDetail
	screenRoute53RecordCreate
	screenRoute53RecordEdit
	screenRoute53RecordDeleteConfirm
	screenSecretList
	screenSecretDetail
	screenSecurityGroupList
	screenSecurityGroupDetail
	screenSecurityGroupAddRule
	screenSecurityGroupDeleteConfirm
	screenIAMUserList
	screenIAMUserDetail
	screenIAMKeyList
	screenIAMKeyDetail
	screenIAMKeyRotateConfirm
	screenIAMKeyRotateResult
	screenCWMetricList
	screenCWMetricDetail
	screenCWLogGroupList
	screenCWLogStreamList
	screenCWLogViewer
	screenECSClusterList
	screenECSServiceList
	screenECSServiceDetail
	screenECSTaskList
	screenECSContainerList
	screenEKSClusterList
	screenEKSUpgradeReadiness
	screenEKSAccessHelper
	screenEKSNodeGroupList
	screenEKSNodeGroupDetail
	screenEKSAddonList
	screenEKSAddonDetail
	screenECRRepositoryList
	screenECRRepositoryDetail
	screenECRImageList
	screenECRImageDetail
	screenFISTemplateList
	screenFISTemplateDetail
	screenFISExperimentList
	screenFISExperimentDetail
	screenS3BucketList
	screenS3ObjectList
	screenS3ObjectDetail
	screenLambdaFunctionList
	screenLambdaFunctionDetail
	screenLambdaInvokeInput
	screenLambdaInvokeResult
	screenBedrockKeyList
	screenBedrockKeyDetail
	screenBedrockKeyCreate
	screenBedrockKeyConfirm
	screenBedrockKeyResult
	screenInspectorHome
	screenInspectorWorkflowPlaceholder
	screenInspectorChecklistPicker
	screenInspectorScanning
	screenInspectorResults
	screenInspectorFindingDetail
	screenInspectorChecklistResults
	screenInspectorChecklistDetail
	screenContextPicker
	screenContextAdd
	screenContextSSOAccountList
	screenContextSSORoleList
	screenLoading
	screenError
	screenExitNotice
)

// Model is the root Bubbletea model.
type Model struct {
	cfg         *config.Config
	awsRepo     *awsservice.AwsRepository
	screen      screen
	quitting    bool
	exitMessage string
	exitTitle   string

	// App-shell state stays root-owned because it coordinates global navigation,
	// context/session setup, shared chrome, and cross-feature transitions.

	// Service selection
	services         []domain.Service
	filteredServices []domain.Service
	svcIdx           int
	favoriteServices map[domain.AwsService]struct{}

	// Feature selection
	features []domain.Feature
	featIdx  int

	// Instance list with filtering
	instances []awsservice.EC2Instance
	filtered  []awsservice.EC2Instance
	instIdx   int

	// SSM session state
	selectedInstance *awsservice.EC2Instance

	// Feature submodels
	ec2Browser   ec2InstanceBrowserModel
	ecs          ecsModel
	eks          eksModel
	ecr          ecrModel
	fis          fisModel
	vpc          vpcModel
	reachability reachabilityModel
	cwMetrics    cloudWatchMetricsModel
	cwLogs       cloudWatchLogsModel
	rds          rdsModel
	route53      route53Model
	iam          iamModel
	bedrock      bedrockModel
	secrets      secretsModel
	security     securityGroupModel
	s3           s3Model
	lambda       lambdaModel
	inspector    inspectorModel

	// Context picker
	configPath         string
	ctxList            []config.ContextInfo
	filteredCtxList    []config.ContextInfo
	ctxIdx             int
	contextTable       table.Model
	ctxPrevScreen      screen
	pendingContextName string
	envContextName     string
	envContextSource   string
	envContextKnown    bool

	contextSSOBase       config.ContextInfo
	contextSSOAccounts   []awsservice.SSOAccount
	contextSSOAccountIdx int
	contextSSOAccount    awsservice.SSOAccount
	contextSSORoles      []awsservice.SSORole
	contextSSORoleIdx    int

	// Context add wizard
	addStep     int // 0=auth_type select, 1+=field input, -1=confirm
	addAuthIdx  int
	addFields   []fieldDef
	addFieldIdx int
	addInput    string
	addValues   map[string]string

	// Caller identity (loaded at startup)
	callerIdentity *awsservice.CallerIdentity

	// Update check
	currentVersion  string
	updateAvailable string // non-empty = new version available
	installMethod   update.InstallMethod

	// Error display
	errMsg string

	// Loading state
	loadingSpinner spinner.Model
	loadingTitle   string
	loadingDetails []string
	filterTI       textinput.Model
	activeFilter   filterTarget
	filters        map[filterTarget]string
	helpVisible    bool

	// Terminal size
	width  int
	height int
}

// New creates a new app Model.
func New(cfg *config.Config, configPath string, version string, checklistPath ...string) Model {
	services := domain.Catalog()
	filterTI := newFilterInput()
	var configuredChecklistPath string
	if len(checklistPath) > 0 {
		configuredChecklistPath = checklistPath[0]
	}
	var favoriteServiceNames []string
	if cfg != nil {
		favoriteServiceNames = cfg.FavoriteServices
	}
	model := Model{
		cfg:              cfg,
		configPath:       configPath,
		currentVersion:   version,
		screen:           screenContextPicker,
		ctxPrevScreen:    screenServiceList,
		services:         services,
		favoriteServices: favoriteServiceSet(favoriteServiceNames),
		loadingSpinner:   newLoadingSpinner(),
		filterTI:         filterTI,
		filters:          make(map[filterTarget]string),
		contextTable:     newContextTable(),
	}
	model.ec2Browser = newEC2InstanceBrowserModel()
	model.ecs = newECSModel()
	model.eks = newEKSModel()
	model.ecr = newECRModel()
	model.fis = newFISModel()
	model.vpc = newVPCModel()
	model.reachability = newReachabilityModel()
	model.cwMetrics = newCloudWatchMetricsModel()
	model.cwLogs = newCloudWatchLogsModel()
	model.rds = newRDSModel()
	model.route53 = newRoute53Model()
	model.iam = newIAMModel()
	model.bedrock = newBedrockModel()
	model.secrets = newSecretsModel()
	model.security = newSecurityGroupModel()
	model.s3 = newS3Model()
	model.lambda = newLambdaModel()
	model.inspector = newInspectorModel(configuredChecklistPath)
	model.applyServiceListFilter()
	return model
}

func newLoadingSpinner() spinner.Model {
	return spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(titleStyle),
	)
}

// updateAvailableMsg is sent when a background version check completes.
type updateAvailableMsg struct {
	version string
	method  update.InstallMethod
}

var appLoadCallerIdentityFn = func(m Model) tea.Cmd {
	return m.loadCallerIdentity()
}

func (m Model) checkForUpdate() tea.Cmd {
	return func() tea.Msg {
		method := update.DetectInstallMethod()
		newVersion := update.CheckForUpdate(m.currentVersion)
		return updateAvailableMsg{version: newVersion, method: method}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadContexts(), m.checkForUpdate(), m.loadStartupCallerIdentity())
}

func (m Model) loadStartupCallerIdentity() tea.Cmd {
	return func() tea.Msg {
		if m.cfg == nil {
			return callerIdentityMsg{}
		}
		if m.cfg.AuthType == config.AuthTypeSSO {
			check, err := contextCheckSSOSessionFn(m.cfg)
			if err != nil || check.LoginRequired {
				return callerIdentityMsg{}
			}
		}
		return appLoadCallerIdentityFn(m)()
	}
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

func (m Model) startLoading(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	return m.startLoadingWithMessage("Loading...", nil, cmd)
}

func (m Model) startLoadingWithMessage(title string, details []string, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.screen = screenLoading
	m.loadingSpinner = newLoadingSpinner()
	m.loadingTitle = title
	m.loadingDetails = append([]string(nil), details...)
	if cmd == nil {
		return m, m.loadingSpinner.Tick
	}
	return m, tea.Batch(m.loadingSpinner.Tick, cmd)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global messages
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncFilterInputWidth()
		m.syncContextTable()
		return m, nil
	case callerIdentityMsg:
		m.callerIdentity = msg.identity
		return m, nil
	case updateAvailableMsg:
		m.installMethod = msg.method
		if msg.version != "" {
			m.updateAvailable = msg.version
		}
		return m, nil
	case spinner.TickMsg:
		if m.screen != screenLoading && m.screen != screenInspectorScanning {
			return m, nil
		}
		var cmd tea.Cmd
		m.loadingSpinner, cmd = m.loadingSpinner.Update(msg)
		return m, cmd
	case errMsg:
		m.errMsg = msg.err.Error()
		m.loadingTitle = ""
		m.loadingDetails = nil
		m.screen = screenError
		return m, nil
	}

	// Domain message handlers
	for _, h := range []func(tea.Msg) (tea.Model, tea.Cmd, bool){
		m.handleEC2VPCMsg,
		m.handleContextMsg,
	} {
		if newM, cmd, handled := h(msg); handled {
			return newM, cmd
		}
	}
	for _, submodel := range m.featureSubmodels() {
		if newM, cmd, handled := submodel.HandleMessage(&m, msg); handled {
			return newM, cmd
		}
	}

	// Key messages — screen dispatch
	if msg, ok := msg.(tea.KeyMsg); ok {
		// Global quit
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		if m.screen == screenExitNotice {
			m.quitting = true
			return m, tea.Quit
		}
		if msg.String() == "?" {
			m.helpVisible = !m.helpVisible
			return m, nil
		}
		if m.helpVisible {
			switch msg.String() {
			case "esc", "enter":
				m.helpVisible = false
			}
			return m, nil
		}
		// Global home — return to service list from any screen (skip text-input screens)
		if msg.String() == "H" && m.screen != screenServiceList && m.screen != screenContextPicker &&
			m.screen != screenSecurityGroupAddRule && m.screen != screenSecurityGroupDeleteConfirm &&
			m.screen != screenLambdaInvokeInput && m.screen != screenBedrockKeyCreate &&
			m.screen != screenBedrockKeyConfirm && m.screen != screenFISTemplateList {
			m.deactivateFilter()
			m.screen = screenServiceList
			return m, nil
		}
		// Global context switch — C key opens context picker (skip text-input screens)
		if msg.String() == "C" && m.screen != screenContextPicker &&
			m.screen != screenSecurityGroupAddRule && m.screen != screenSecurityGroupDeleteConfirm &&
			m.screen != screenLambdaInvokeInput && m.screen != screenBedrockKeyCreate &&
			m.screen != screenBedrockKeyConfirm {
			m.deactivateFilter()
			m.ctxPrevScreen = m.screen
			return m, m.loadContexts()
		}

		for _, submodel := range m.featureSubmodels() {
			if newM, cmd, handled := submodel.HandleKey(&m, msg); handled {
				return newM, cmd
			}
		}

		switch m.screen {
		case screenServiceList:
			return m.updateServiceList(msg)
		case screenFeatureList:
			return m.updateFeatureList(msg)
		case screenInstanceList:
			return m.updateInstanceList(msg)
		case screenContextPicker:
			return m.updateContextPicker(msg)
		case screenContextAdd:
			return m.updateContextAdd(msg)
		case screenContextSSOAccountList:
			return m.updateContextSSOAccountList(msg)
		case screenContextSSORoleList:
			return m.updateContextSSORoleList(msg)
		case screenError:
			return m.updateError(msg)
		}
	}

	if m.filterTI.Focused() {
		var cmd tea.Cmd
		m.filterTI, cmd = m.filterTI.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateServiceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterServices); handled {
		return m, cmd
	}

	switch key {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.resetFilter(filterServices)
		m.ctxPrevScreen = screenServiceList
		return m, m.loadContexts()
	case "up", "k":
		m.svcIdx = previousListIndex(m.svcIdx, len(m.serviceList()))
	case "down", "j":
		m.svcIdx = nextListIndex(m.svcIdx, len(m.serviceList()))
	case "/":
		return m, m.activateFilter(filterServices)
	case "f":
		if service, ok := m.selectedService(); ok {
			if err := m.toggleFavoriteService(service.Name); err != nil {
				m.errMsg = err.Error()
				m.screen = screenError
			}
		}
	case "i":
		m.inspector.Enter(&m)
	case "enter":
		if service, ok := m.selectedService(); ok {
			m.features = service.Features
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
		m.featIdx = previousListIndex(m.featIdx, len(m.features))
	case "down", "j":
		m.featIdx = nextListIndex(m.featIdx, len(m.features))
	case "enter":
		if m.featIdx < len(m.features) {
			feat := m.features[m.featIdx]
			switch feat.Kind {
			case domain.FeatureSSMSession:
				return m.startLoading(m.loadInstances())
			case domain.FeatureEC2InstanceBrowser:
				return m.ec2Browser.Start(&m)
			case domain.FeatureVPCBrowser:
				return m.vpc.Start(&m)
			case domain.FeatureReachabilityAnalyzer:
				return m.reachability.Start(&m)
			case domain.FeatureRDSBrowser:
				return m.rds.Start(&m)
			case domain.FeatureRoute53Browser:
				return m.route53.Start(&m)
			case domain.FeatureSecretsBrowser:
				return m.secrets.Start(&m)
			case domain.FeatureCloudWatchMetrics:
				return m.cwMetrics.Start(&m)
			case domain.FeatureCloudWatchLogsBrowser:
				return m.cwLogs.Start(&m)
			case domain.FeatureS3Browser:
				return m.s3.Start(&m)
			case domain.FeatureSecurityGroupBrowser:
				return m.security.Start(&m)
			case domain.FeatureIAMUsersBrowser:
				return m.iam.StartUsers(&m)
			case domain.FeatureListAccessKeys:
				return m.iam.StartKeys(&m, false)
			case domain.FeatureRotateAccessKey:
				return m.iam.StartKeys(&m, true)
			case domain.FeatureECSExec:
				return m.ecs.Start(&m)
			case domain.FeatureECRRepositoryBrowser:
				return m.ecr.Start(&m)
			case domain.FeatureEKSBrowser:
				return m.eks.Start(&m)
			case domain.FeatureFISTemplateBrowser:
				return m.fis.Start(&m)
			case domain.FeatureLambdaBrowser:
				return m.lambda.Start(&m)
			case domain.FeatureBedrockAPIKeys:
				return m.bedrock.Start(&m)
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

	if m.helpVisible {
		return m.fitToHeight(m.viewHelp())
	}

	var v string
	for _, submodel := range m.featureSubmodels() {
		if view, handled := submodel.View(m); handled {
			return m.fitToHeight(view)
		}
	}
	switch m.screen {
	case screenServiceList:
		v = m.viewServiceList()
	case screenFeatureList:
		v = m.viewFeatureList()
	case screenInstanceList:
		v = m.viewInstanceList()
	case screenContextPicker:
		v = m.viewContextPicker()
	case screenContextAdd:
		v = m.viewContextAdd()
	case screenContextSSOAccountList:
		v = m.viewContextSSOAccountList()
	case screenContextSSORoleList:
		v = m.viewContextSSORoleList()
	case screenLoading:
		v = m.viewLoading()
	case screenError:
		v = m.viewError()
	case screenExitNotice:
		v = m.viewExitNotice()
	}

	return m.fitToHeight(v)
}

func (m Model) viewServiceList() string {
	var b strings.Builder
	var panel strings.Builder
	services := m.serviceList()
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Select AWS Service"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterServices))
	b.WriteString("\n\n")

	// overhead: status bar (2) + title (1) + filter (1) + blank (1) + list panel (2) + blank (1) + help bar (1) = 9
	visibleLines := max(m.height-9, 3)
	start := 0
	if m.svcIdx >= visibleLines {
		start = m.svcIdx - visibleLines + 1
	}
	end := min(start+visibleLines, len(services))

	if len(services) == 0 {
		panel.WriteString(dimStyle.Render("  No matching services"))
		panel.WriteString("\n")
	} else {
		for i := start; i < end; i++ {
			svc := services[i]
			cursor := "  "
			style := normalStyle
			if i == m.svcIdx {
				cursor = "> "
				style = selectedStyle
			}
			favoriteMarker := "  "
			if m.isFavoriteService(svc.Name) {
				favoriteMarker = favoriteServiceStyle.Render("* ")
			}
			label := m.renderHighlightedValue(filterServices, string(svc.Name))
			if m.isFavoriteService(svc.Name) {
				label = favoriteServiceStyle.Render(label)
			}
			panel.WriteString(style.Render(cursor) + favoriteMarker + style.Render(label))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d services • %s", len(services), len(m.services), m.serviceListSummary())))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • f: favorite • enter: select • i: Inspector mode • esc: context • q: quit"))
	return b.String()
}

func (m Model) viewFeatureList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	svc, ok := m.selectedService()
	if !ok {
		return m.viewServiceList()
	}
	svcName := svc.Name
	b.WriteString(titleStyle.Render(fmt.Sprintf("%s > Select Feature", svcName)))
	b.WriteString("\n\n")

	// Each selected item takes 2 lines (name + description), others take 1.
	// overhead: status bar (2) + title (1) + blank (1) + list panel (2) + blank (1) + help bar (1) = 8
	visibleLines := max(m.height-8, 3)
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
		panel.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, feat.Kind)))
		panel.WriteString("\n")
		if i == m.featIdx {
			panel.WriteString(dimStyle.Render(fmt.Sprintf("    %s", feat.Description)))
			panel.WriteString("\n")
		}
		linesUsed += needed
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • enter: select • esc: back"))
	return b.String()
}
