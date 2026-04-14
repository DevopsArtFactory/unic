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
	screenCWLogGroupList
	screenCWLogStreamList
	screenCWLogViewer
	screenECSClusterList
	screenECSServiceList
	screenECSTaskList
	screenECSContainerList
	screenS3BucketList
	screenS3ObjectList
	screenS3ObjectDetail
	screenInspectorHome
	screenInspectorScanning
	screenInspectorResults
	screenInspectorFindingDetail
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

	// Service selection
	services []domain.Service
	svcIdx   int

	// Feature selection
	features []domain.Feature
	featIdx  int

	// Instance list with filtering
	instances []awsservice.EC2Instance
	filtered  []awsservice.EC2Instance
	instIdx   int

	// SSM session state
	selectedInstance *awsservice.EC2Instance

	// VPC browser state
	vpcs                        []awsservice.VPC
	filteredVPCs                []awsservice.VPC
	vpcIdx                      int
	subnets                     []awsservice.Subnet
	subnetIdx                   int
	selectedVPC                 *awsservice.VPC
	selectedSubnet              *awsservice.Subnet
	availableIPs                []string
	filteredIPs                 []string
	ipScrollOffset              int
	ipFilter                    string
	ipFilterActive              bool
	reachabilityRegions         []string
	filteredReachabilityRegions []string
	reachabilityRegion          string
	reachabilityRegionIdx       int
	reachabilityRegionFilter    string
	reachabilityRegionFiltering bool
	reachabilityTargets         []awsservice.ReachabilityTarget
	filteredReachabilityTargets []awsservice.ReachabilityTarget
	reachabilitySourceTypes     []string
	reachabilitySourceTypeIdx   int
	reachabilityDestTypes       []string
	reachabilityDestTypeIdx     int
	reachabilityIdx             int
	reachabilityFilter          string
	reachabilityFilterActive    bool
	reachabilitySource          *awsservice.ReachabilityTarget
	reachabilityDestination     *awsservice.ReachabilityTarget
	reachabilityDestinationIP   string
	reachabilityProtocolIdx     int
	reachabilityPortInput       string
	reachabilityConfigField     int
	reachabilityResult          *awsservice.ReachabilityAnalysisResult
	reachabilityScrollOffset    int
	// RDS browser state
	rdsInstances    []awsservice.RDSInstance
	filteredRDS     []awsservice.RDSInstance
	rdsIdx          int
	selectedRDS     *awsservice.RDSInstance
	rdsAction       string // "start", "stop", "failover"
	rdsConfirmInput string // typed input for destructive action confirmation
	rdsPolling      bool

	// Route53 browser state
	route53Zones           []awsservice.HostedZone
	filteredRoute53Zones   []awsservice.HostedZone
	route53ZoneIdx         int
	selectedRoute53Zone    *awsservice.HostedZone
	route53Records         []awsservice.DNSRecord
	filteredRoute53Records []awsservice.DNSRecord
	route53RecordIdx       int
	selectedRoute53Record  *awsservice.DNSRecord

	// Route53 mutation state
	route53Action        string            // "create", "edit", "delete"
	route53ConfirmInput  string            // type-to-confirm for delete
	route53EditField     int               // current form field index
	route53EditValues    map[string]string // accumulated form values
	route53EditInput     string            // current field text input
	route53EditSelectIdx int               // index for select-type fields (record type)
	route53ChangeID      string            // for status polling
	route53ChangeStatus  string            // "PENDING" / "INSYNC"
	route53Polling       bool              // polling active

	// IAM credentials state
	iamUsers            []awsservice.IAMUser
	filteredIAMUsers    []awsservice.IAMUser
	iamUserIdx          int
	iamUserLoadingMore  bool
	iamUserHasMore      bool
	iamUserNextMarker   string
	selectedIAMUser     *awsservice.IAMUserDetail
	iamKeys             []awsservice.AccessKey
	iamKeyIdx           int
	selectedIAMKey      *awsservice.AccessKey
	iamRotationEnabled  bool
	iamRotateConfirm    string // typed input for rotate confirmation
	iamRotationOldKeyID string
	iamNewKey           *awsservice.NewAccessKey
	iamCopyMsg          string // feedback message for clipboard copy
	iamRotationStatus   string
	iamNewKeyVerified   bool
	iamOldKeyDeleted    bool
	iamOldKeyInactive   bool

	// Secrets Manager browser state
	secrets         []awsservice.Secret
	filteredSecrets []awsservice.Secret
	secretIdx       int
	selectedSecret  *awsservice.SecretDetail

	// Security Group browser state
	securityGroups         []awsservice.SecurityGroup
	filteredSecurityGroups []awsservice.SecurityGroup
	sgIdx                  int
	selectedSecurityGroup  *awsservice.SecurityGroup
	sgRuleSection          string // "ingress" or "egress" — active section in detail view
	sgRuleIdx              int    // selected rule index within the active section
	sgDeleteConfirm        string // type-to-confirm input for rule deletion
	sgDeleteRule           *awsservice.SecurityGroupRule
	sgAddField             int               // current field in add form (0=direction, 1=protocol, 2=fromPort, 3=toPort, 4=source, 5=description)
	sgAddValues            map[string]string // accumulated form values
	sgAddInput             string            // current field text input
	sgAddSelectIdx         int               // index for select-type fields (direction, protocol)

	// ECS browser state
	ecsClusters         []awsservice.ECSCluster
	filteredECSClusters []awsservice.ECSCluster
	ecsClusterIdx       int
	selectedECSCluster  *awsservice.ECSCluster

	ecsServices         []awsservice.ECSService
	filteredECSServices []awsservice.ECSService
	ecsServiceIdx       int
	selectedECSService  *awsservice.ECSService

	ecsTasks        []awsservice.ECSTask
	ecsTaskIdx      int
	selectedECSTask *awsservice.ECSTask

	ecsContainers   []awsservice.ECSContainer
	ecsContainerIdx int

	// CloudWatch Logs browser state
	cwLogGroups             []awsservice.LogGroup
	filteredCWLogGroups     []awsservice.LogGroup
	cwLogGroupIdx           int
	cwLogGroupFilter        string
	cwLogGroupFilterActive  bool
	selectedCWLogGroup      *awsservice.LogGroup
	cwLogStreams            []awsservice.LogStream
	filteredCWLogStreams    []awsservice.LogStream
	cwLogStreamIdx          int
	cwLogStreamFilter       string
	cwLogStreamFilterActive bool
	cwLogStreamNextToken    *string
	selectedCWLogStream     *awsservice.LogStream
	cwLogEvents             []awsservice.LogEvent
	cwLogScrollOffset       int
	cwLogGroupNextToken     *string
	cwLogNextToken          *string
	cwLogTimeRange          int // index into preset time ranges
	cwLogFilterPattern      string
	cwLogFilterActive       bool // filter pattern input active
	cwLogTailing            bool // live tail active
	cwLogTailToken          *string
	cwLogWrap               bool
	cwLogHorizontalOffset   int

	// S3 browser state
	s3Buckets         []awsservice.S3Bucket
	filteredS3Buckets []awsservice.S3Bucket
	s3BucketIdx       int
	selectedS3Bucket  *awsservice.S3Bucket
	s3Objects         []awsservice.S3Object
	filteredS3Objects []awsservice.S3Object
	s3ObjectIdx       int
	s3CurrentPrefix   string
	s3PrefixStack     []string
	selectedS3Object  *awsservice.S3ObjectDetail

	// Inspector browser state
	inspectorReport          *awsservice.SecurityScanReport
	inspectorFindings        []awsservice.SecurityFinding
	inspectorIdx             int
	inspectorSeverityFilter  awsservice.RuleSeverity
	selectedInspectorFinding *awsservice.SecurityFinding

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
func New(cfg *config.Config, configPath string, version string) Model {
	services := domain.Catalog()
	filterTI := newFilterInput()
	return Model{
		cfg:            cfg,
		configPath:     configPath,
		currentVersion: version,
		screen:         screenContextPicker,
		ctxPrevScreen:  screenServiceList,
		services:       services,
		loadingSpinner: newLoadingSpinner(),
		filterTI:       filterTI,
		filters:        make(map[filterTarget]string),
		contextTable:   newContextTable(),
	}
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

func (m Model) checkForUpdate() tea.Cmd {
	return func() tea.Msg {
		method := update.DetectInstallMethod()
		newVersion := update.CheckForUpdate(m.currentVersion)
		return updateAvailableMsg{version: newVersion, method: method}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadContexts(), m.checkForUpdate())
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
		m.handleRoute53Msg,
		m.handleRDSMsg,
		m.handleSecurityGroupMsg,
		m.handleIAMMsg,
		m.handleSecretMsg,
		m.handleCloudWatchLogsMsg,
		m.handleECSMsg,
		m.handleS3Msg,
		m.handleInspectorMsg,
		m.handleContextMsg,
	} {
		if newM, cmd, handled := h(msg); handled {
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
			m.screen != screenSecurityGroupAddRule && m.screen != screenSecurityGroupDeleteConfirm {
			m.deactivateFilter()
			m.screen = screenServiceList
			return m, nil
		}
		// Global context switch — C key opens context picker (skip text-input screens)
		if msg.String() == "C" && m.screen != screenContextPicker &&
			m.screen != screenSecurityGroupAddRule && m.screen != screenSecurityGroupDeleteConfirm {
			m.deactivateFilter()
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
		case screenReachabilityRegionList:
			return m.updateReachabilityRegionList(msg)
		case screenReachabilitySourceList:
			return m.updateReachabilitySourceList(msg)
		case screenReachabilityDestinationList:
			return m.updateReachabilityDestinationList(msg)
		case screenReachabilityConfig:
			return m.updateReachabilityConfig(msg)
		case screenReachabilityResult:
			return m.updateReachabilityResult(msg)
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
		case screenRoute53RecordCreate:
			return m.updateRoute53RecordCreate(msg)
		case screenRoute53RecordEdit:
			return m.updateRoute53RecordEdit(msg)
		case screenRoute53RecordDeleteConfirm:
			return m.updateRoute53RecordDeleteConfirm(msg)
		case screenSecretList:
			return m.updateSecretList(msg)
		case screenSecretDetail:
			return m.updateSecretDetail(msg)
		case screenCWLogGroupList:
			return m.updateCWLogGroupList(msg)
		case screenCWLogStreamList:
			return m.updateCWLogStreamList(msg)
		case screenCWLogViewer:
			return m.updateCWLogViewer(msg)
		case screenS3BucketList:
			return m.updateS3BucketList(msg)
		case screenS3ObjectList:
			return m.updateS3ObjectList(msg)
		case screenS3ObjectDetail:
			return m.updateS3ObjectDetail(msg)
		case screenInspectorHome:
			return m.updateInspectorHome(msg)
		case screenInspectorResults:
			return m.updateInspectorResults(msg)
		case screenInspectorFindingDetail:
			return m.updateInspectorFindingDetail(msg)
		case screenSecurityGroupList:
			return m.updateSecurityGroupList(msg)
		case screenSecurityGroupDetail:
			return m.updateSecurityGroupDetail(msg)
		case screenSecurityGroupAddRule:
			return m.updateSecurityGroupAddRule(msg)
		case screenSecurityGroupDeleteConfirm:
			return m.updateSecurityGroupDeleteConfirm(msg)
		case screenIAMUserList:
			return m.updateIAMUserList(msg)
		case screenIAMUserDetail:
			return m.updateIAMUserDetail(msg)
		case screenIAMKeyList:
			return m.updateIAMKeyList(msg)
		case screenIAMKeyDetail:
			return m.updateIAMKeyDetail(msg)
		case screenIAMKeyRotateConfirm:
			return m.updateIAMKeyRotateConfirm(msg)
		case screenIAMKeyRotateResult:
			return m.updateIAMKeyRotateResult(msg)
		case screenECSClusterList:
			return m.updateECSClusterList(msg)
		case screenECSServiceList:
			return m.updateECSServiceList(msg)
		case screenECSTaskList:
			return m.updateECSTaskList(msg)
		case screenECSContainerList:
			return m.updateECSContainerList(msg)
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

// handleSecretMsg handles Secrets Manager messages.
func (m Model) handleSecretMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case secretsLoadedMsg:
		m.secrets = msg.secrets
		m.filteredSecrets = msg.secrets
		m.secretIdx = 0
		m.screen = screenSecretList
		return m, nil, true
	case secretDetailLoadedMsg:
		m.selectedSecret = msg.detail
		m.screen = screenSecretDetail
		return m, nil, true
	}
	return m, nil, false
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
				return m.startLoading(m.loadInstances())
			case domain.FeatureVPCBrowser:
				return m.startLoading(m.loadVPCs())
			case domain.FeatureReachabilityAnalyzer:
				m.reachabilityRegions = availableReachabilityRegions(m.cfg.Region)
				m.filteredReachabilityRegions = m.reachabilityRegions
				m.reachabilityRegion = m.cfg.Region
				m.reachabilityRegionIdx = indexOfString(m.reachabilityRegions, m.reachabilityRegion)
				if m.reachabilityRegionIdx < 0 {
					m.reachabilityRegionIdx = 0
				}
				m.reachabilityRegionFilter = ""
				m.reachabilityRegionFiltering = false
				m.reachabilityTargets = nil
				m.filteredReachabilityTargets = nil
				m.reachabilitySource = nil
				m.reachabilityDestination = nil
				m.reachabilityDestinationIP = ""
				m.reachabilityResult = nil
				m.reachabilityScrollOffset = 0
				m.awsRepo = nil
				m.screen = screenReachabilityRegionList
				return m, nil
			case domain.FeatureRDSBrowser:
				return m.startLoading(m.loadRDSInstances())
			case domain.FeatureRoute53Browser:
				return m.startLoading(m.loadRoute53Zones())
			case domain.FeatureSecretsBrowser:
				return m.startLoading(m.loadSecrets())
			case domain.FeatureCloudWatchLogsBrowser:
				return m.startLoading(m.loadCWLogGroups(false))
			case domain.FeatureS3Browser:
				return m.startLoading(m.loadS3Buckets())
			case domain.FeatureSecurityGroupBrowser:
				return m.startLoading(m.loadSecurityGroups())
			case domain.FeatureIAMUsersBrowser:
				return m.startLoading(m.loadIAMUsers())
			case domain.FeatureListAccessKeys:
				m.iamRotationEnabled = false
				return m.startLoading(m.loadIAMKeys())
			case domain.FeatureRotateAccessKey:
				m.iamRotationEnabled = true
				return m.startLoading(m.loadIAMKeys())
			case domain.FeatureECSExec:
				return m.startLoading(m.loadECSClusters())
			case domain.FeatureSecurityScan:
				m.inspectorReport = nil
				m.inspectorFindings = nil
				m.inspectorIdx = 0
				m.inspectorSeverityFilter = ""
				m.selectedInspectorFinding = nil
				m.screen = screenInspectorHome
				return m, nil
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
	case screenReachabilityRegionList:
		v = m.viewReachabilityRegionList()
	case screenReachabilitySourceList:
		v = m.viewReachabilitySourceList()
	case screenReachabilityDestinationList:
		v = m.viewReachabilityDestinationList()
	case screenReachabilityConfig:
		v = m.viewReachabilityConfig()
	case screenReachabilityResult:
		v = m.viewReachabilityResult()
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
	case screenRoute53RecordCreate:
		v = m.viewRoute53RecordCreate()
	case screenRoute53RecordEdit:
		v = m.viewRoute53RecordEdit()
	case screenRoute53RecordDeleteConfirm:
		v = m.viewRoute53RecordDeleteConfirm()
	case screenSecretList:
		v = m.viewSecretList()
	case screenSecretDetail:
		v = m.viewSecretDetail()
	case screenCWLogGroupList:
		v = m.viewCWLogGroupList()
	case screenCWLogStreamList:
		v = m.viewCWLogStreamList()
	case screenCWLogViewer:
		v = m.viewCWLogViewer()
	case screenS3BucketList:
		v = m.viewS3BucketList()
	case screenS3ObjectList:
		v = m.viewS3ObjectList()
	case screenS3ObjectDetail:
		v = m.viewS3ObjectDetail()
	case screenInspectorHome:
		v = m.viewInspectorHome()
	case screenInspectorScanning:
		v = m.viewInspectorScanning()
	case screenInspectorResults:
		v = m.viewInspectorResults()
	case screenInspectorFindingDetail:
		v = m.viewInspectorFindingDetail()
	case screenSecurityGroupList:
		v = m.viewSecurityGroupList()
	case screenSecurityGroupDetail:
		v = m.viewSecurityGroupDetail()
	case screenSecurityGroupAddRule:
		v = m.viewSecurityGroupAddRule()
	case screenSecurityGroupDeleteConfirm:
		v = m.viewSecurityGroupDeleteConfirm()
	case screenIAMUserList:
		v = m.viewIAMUserList()
	case screenIAMUserDetail:
		v = m.viewIAMUserDetail()
	case screenIAMKeyList:
		v = m.viewIAMKeyList()
	case screenIAMKeyDetail:
		v = m.viewIAMKeyDetail()
	case screenIAMKeyRotateConfirm:
		v = m.viewIAMKeyRotateConfirm()
	case screenIAMKeyRotateResult:
		v = m.viewIAMKeyRotateResult()
	case screenECSClusterList:
		v = m.viewECSClusterList()
	case screenECSServiceList:
		v = m.viewECSServiceList()
	case screenECSTaskList:
		v = m.viewECSTaskList()
	case screenECSContainerList:
		v = m.viewECSContainerList()
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
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Select AWS Service"))
	b.WriteString("\n\n")

	// overhead: status bar (2) + title (1) + blank (1) + list panel (2) + blank (1) + help bar (1) = 8
	visibleLines := max(m.height-8, 3)
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
		panel.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, svc.Name)))
		panel.WriteString("\n")
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • enter: select • esc: context • q: quit"))
	return b.String()
}

func (m Model) viewFeatureList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	svcName := m.services[m.svcIdx].Name
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

	if cmd, handled := m.updateSharedFilter(msg, filterSecrets); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterSecrets)
	case "up", "k":
		if m.secretIdx > 0 {
			m.secretIdx--
		}
	case "down", "j":
		if m.secretIdx < len(m.filteredSecrets)-1 {
			m.secretIdx++
		}
	case "/":
		return m, m.activateFilter(filterSecrets)
	case "enter":
		if len(m.filteredSecrets) > 0 && m.secretIdx < len(m.filteredSecrets) {
			selected := m.filteredSecrets[m.secretIdx]
			return m.startLoading(m.loadSecretDetail(selected.Name))
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

func (m Model) viewSecretList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Secrets Manager"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterSecrets))
	b.WriteString("\n\n")

	if len(m.filteredSecrets) == 0 {
		panel.WriteString(dimStyle.Render("  No matching secrets"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
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
			panel.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, s.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d secrets", len(m.filteredSecrets), len(m.secrets))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: detail • esc: back • H: home"))
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

	b.WriteString(renderDetailLine("Name", normalStyle.Render(d.Name)))
	b.WriteString("\n")

	kmsKey := d.KMSKeyID
	if kmsKey == "" {
		kmsKey = dimStyle.Render("(aws/secretsmanager)")
	}
	b.WriteString(renderDetailLine("Encryption Key", kmsKey))
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
	b.WriteString(m.renderHelpBar("esc: back • H: home"))
	return b.String()
}
