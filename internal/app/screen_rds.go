package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

type rdsModel struct {
	instances    []awsservice.RDSInstance
	filtered     []awsservice.RDSInstance
	idx          int
	selected     *awsservice.RDSInstance
	action       string // "start", "stop", "failover", "modify"
	confirmInput string // typed input for destructive action confirmation
	polling      bool

	// Instance class modification
	classes          []string
	filteredClasses  []string
	classIdx         int
	classFilter      string
	classFiltering   bool
	pendingClass     string
	applyImmediately bool
}

func newRDSModel() rdsModel {
	return rdsModel{}
}

func (rm *rdsModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(rm.loadInstances(*m))
}

func (rm *rdsModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case rdsInstancesLoadedMsg:
		rm.instances = msg.instances
		rm.filtered = applyFilter(rm.instances, m.filterValue(filterRDS))
		rm.idx = 0
		m.screen = screenRDSList
		return *m, nil, true

	case rdsClassesLoadedMsg:
		if rm.selected == nil || rm.selected.DBInstanceID != msg.instanceID {
			return *m, nil, true
		}
		rm.classes = msg.classes
		rm.filteredClasses = msg.classes
		rm.classIdx = 0
		rm.classFilter = ""
		rm.classFiltering = false
		m.screen = screenRDSClassPicker
		return *m, nil, true

	case rdsActionDoneMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return *m, nil, true
		}
		rm.polling = true
		m.screen = screenRDSDetail
		return *m, rm.tickPoll(msg.instanceID), true

	case rdsStatusRefreshedMsg:
		if msg.err != nil {
			rm.polling = false
			return *m, nil, true
		}
		rm.selected = msg.instance
		for i, inst := range rm.instances {
			if inst.DBInstanceID == msg.instance.DBInstanceID {
				rm.instances[i] = *msg.instance
				break
			}
		}
		rm.filtered = applyFilter(rm.instances, m.filterValue(filterRDS))
		rm.idx = 0
		// An immediate class modify can briefly report `available` with the
		// change still pending; keep polling until the pending value clears.
		modifyInFlight := rm.action == "modify" && rm.applyImmediately && msg.instance.PendingInstanceClass != ""
		if awsservice.IsTransitionalStatus(msg.instance.Status) || modifyInFlight {
			return *m, rm.tickPoll(msg.instance.DBInstanceID), true
		}
		rm.polling = false
		return *m, nil, true

	case rdsTickMsg:
		if rm.polling {
			return *m, rm.pollStatus(*m, msg.instanceID), true
		}
		return *m, nil, true
	}
	return *m, nil, false
}

func (rm *rdsModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenRDSList:
		newM, cmd := rm.updateList(m, msg)
		return newM, cmd, true
	case screenRDSDetail:
		newM, cmd := rm.updateDetail(m, msg)
		return newM, cmd, true
	case screenRDSClassPicker:
		newM, cmd := rm.updateClassPicker(m, msg)
		return newM, cmd, true
	case screenRDSConfirm:
		newM, cmd := rm.updateConfirm(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (rm rdsModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenRDSList:
		return rm.viewList(m), true
	case screenRDSDetail:
		return rm.viewDetail(m), true
	case screenRDSClassPicker:
		return rm.viewClassPicker(m), true
	case screenRDSConfirm:
		return rm.viewConfirm(m), true
	default:
		return "", false
	}
}

func (rm *rdsModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterRDS {
		return false
	}
	rm.filtered = applyFilter(rm.instances, m.filterValue(target))
	rm.idx = 0
	return true
}

func (rm *rdsModel) updateList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterRDS); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterRDS)
	case "up", "k":
		rm.idx = previousListIndex(rm.idx, len(rm.filtered))
	case "down", "j":
		rm.idx = nextListIndex(rm.idx, len(rm.filtered))
	case "/":
		return *m, m.activateFilter(filterRDS)
	case "enter":
		if len(rm.filtered) > 0 && rm.idx < len(rm.filtered) {
			selected := rm.filtered[rm.idx]
			rm.selected = &selected
			m.screen = screenRDSDetail
		}
	}
	return *m, nil
}

func (rm *rdsModel) updateDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		rm.polling = false
		m.screen = screenRDSList
	case "s":
		if rm.selected != nil && rm.selected.CanStart() {
			rm.action = "start"
			rm.confirmInput = ""
			m.screen = screenRDSConfirm
		}
	case "x":
		if rm.selected != nil && rm.selected.CanStop() {
			rm.action = "stop"
			rm.confirmInput = ""
			m.screen = screenRDSConfirm
		}
	case "f":
		if rm.selected != nil && rm.selected.CanFailover() {
			rm.action = "failover"
			rm.confirmInput = ""
			m.screen = screenRDSConfirm
		}
	case "m":
		if rm.selected != nil {
			return m.startLoadingWithMessage(
				"Loading instance classes...",
				[]string{rm.selected.DBInstanceID, fmt.Sprintf("engine=%s %s", rm.selected.Engine, rm.selected.EngineVersion)},
				rm.loadClasses(*m),
			)
		}
	case "r":
		if rm.selected != nil {
			return *m, rm.pollStatus(*m, rm.selected.DBInstanceID)
		}
	}
	return *m, nil
}

func (rm *rdsModel) updateClassPicker(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if rm.classFiltering {
		newFilter, deactivate, changed := handleFilterKey(key, rm.classFilter)
		rm.classFilter = newFilter
		if deactivate {
			rm.classFiltering = false
		}
		if changed {
			rm.filteredClasses = applyStringFilter(rm.classes, rm.classFilter)
			rm.classIdx = 0
		}
		if !isFilterNavigationKey(key) {
			return *m, nil
		}
	}

	switch key {
	case "q", "esc":
		m.screen = screenRDSDetail
	case "up", "k":
		rm.classIdx = previousListIndex(rm.classIdx, len(rm.filteredClasses))
	case "down", "j":
		rm.classIdx = nextListIndex(rm.classIdx, len(rm.filteredClasses))
	case "/":
		rm.classFiltering = true
	case "enter":
		if len(rm.filteredClasses) == 0 || rm.classIdx >= len(rm.filteredClasses) {
			return *m, nil
		}
		chosen := rm.filteredClasses[rm.classIdx]
		if rm.selected != nil && chosen == rm.selected.InstanceClass {
			// Modifying to the identical class is a no-op API call; refuse it.
			return *m, nil
		}
		rm.pendingClass = chosen
		rm.applyImmediately = false
		rm.action = "modify"
		rm.confirmInput = ""
		m.screen = screenRDSConfirm
	}
	return *m, nil
}

func (rm *rdsModel) updateConfirm(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Start action uses simple y/n confirmation
	if rm.action == "start" {
		switch msg.String() {
		case "y", "enter":
			if rm.selected != nil {
				m.screen = screenRDSDetail
				return *m, rm.executeAction(*m, rm.action, rm.selected.DBInstanceID)
			}
		case "n", "esc":
			m.screen = screenRDSDetail
		}
		return *m, nil
	}

	// Stop/failover require typing the identifier to confirm
	// For Aurora cluster members, confirm with cluster ID; for standalone, instance ID
	// Class modification is instance-level, so it always confirms with the instance ID
	confirmTarget := ""
	if rm.selected != nil {
		if rm.selected.IsClusterMember() && rm.action != "modify" {
			confirmTarget = rm.selected.ClusterID
		} else {
			confirmTarget = rm.selected.DBInstanceID
		}
	}
	switch msg.String() {
	case "esc":
		if rm.action == "modify" {
			m.screen = screenRDSClassPicker
			return *m, nil
		}
		m.screen = screenRDSDetail
	case "tab":
		if rm.action == "modify" {
			rm.applyImmediately = !rm.applyImmediately
			return *m, nil
		}
	case "enter":
		if rm.selected != nil && rm.confirmInput == confirmTarget {
			m.screen = screenRDSDetail
			return *m, rm.executeAction(*m, rm.action, rm.selected.DBInstanceID)
		}
	case "backspace":
		if len(rm.confirmInput) > 0 {
			rm.confirmInput = rm.confirmInput[:len(rm.confirmInput)-1]
		}
	default:
		if runes := msg.Runes; len(runes) > 0 {
			rm.confirmInput += string(runes)
		}
	}
	return *m, nil
}

func (rm rdsModel) loadClasses(m Model) tea.Cmd {
	instance := *rm.selected
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}

		classes, err := repo.ListOrderableDBInstanceClasses(ctx, instance.Engine, instance.EngineVersion)
		if err != nil {
			return errMsg{err: err}
		}
		if len(classes) == 0 {
			return errMsg{err: fmt.Errorf("no orderable instance classes found for %s %s in this region", instance.Engine, instance.EngineVersion)}
		}
		return rdsClassesLoadedMsg{instanceID: instance.DBInstanceID, classes: classes}
	}
}

func (rm rdsModel) loadInstances(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
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

func (rm rdsModel) executeAction(m Model, action, dbInstanceID string) tea.Cmd {
	clusterID := ""
	if rm.selected != nil {
		clusterID = rm.selected.ClusterID
	}
	pendingClass := rm.pendingClass
	applyImmediately := rm.applyImmediately
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return rdsActionDoneMsg{action: action, instanceID: dbInstanceID, err: err}
			}
		}

		var err error
		if action == "modify" {
			// Class modification is always instance-level, even for cluster members.
			err = repo.ModifyDBInstanceClass(ctx, dbInstanceID, pendingClass, applyImmediately)
			return rdsActionDoneMsg{action: action, instanceID: dbInstanceID, err: err}
		}
		if clusterID != "" {
			// Aurora cluster-level actions
			switch action {
			case "start":
				err = repo.StartDBCluster(ctx, clusterID)
			case "stop":
				err = repo.StopDBCluster(ctx, clusterID)
			case "failover":
				err = repo.FailoverDBCluster(ctx, clusterID)
			}
		} else {
			// Standalone instance actions
			switch action {
			case "start":
				err = repo.StartDBInstance(ctx, dbInstanceID)
			case "stop":
				err = repo.StopDBInstance(ctx, dbInstanceID)
			case "failover":
				err = repo.RebootDBInstance(ctx, dbInstanceID, true)
			}
		}
		return rdsActionDoneMsg{action: action, instanceID: dbInstanceID, err: err}
	}
}

func (rm rdsModel) pollStatus(m Model, dbInstanceID string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
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

func (rm rdsModel) tickPoll(dbInstanceID string) tea.Cmd {
	return tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
		return rdsTickMsg{instanceID: dbInstanceID}
	})
}

func (rm rdsModel) viewList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("RDS Instances"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterRDS))
	b.WriteString("\n\n")

	if len(rm.filtered) == 0 {
		panel.WriteString(dimStyle.Render("  No matching instances"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if rm.idx >= visibleLines {
			start = rm.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(rm.filtered))

		for i := start; i < end; i++ {
			inst := rm.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == rm.idx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterRDS, inst.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d instances", len(rm.filtered), len(rm.instances))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: detail • esc: back • H: home"))
	return b.String()
}

func (rm rdsModel) viewDetail(m Model) string {
	if rm.selected == nil {
		return ""
	}
	r := rm.selected
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("RDS Instance Detail"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Identifier", normalStyle.Render(r.DBInstanceID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Engine", normalStyle.Render(fmt.Sprintf("%s %s", r.Engine, r.EngineVersion))))
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
	if rm.polling {
		pollingIndicator = filterStyle.Render(" (polling...)")
	}
	b.WriteString(renderDetailLine("Status", statusStr+pollingIndicator))
	b.WriteString("\n")

	b.WriteString(renderDetailLine("Class", normalStyle.Render(r.InstanceClass)))
	b.WriteString("\n")
	if r.PendingInstanceClass != "" {
		b.WriteString(renderDetailLine("Pending Class", filterStyle.Render(r.PendingInstanceClass)+dimStyle.Render(" (applies at next maintenance window unless applied immediately)")))
		b.WriteString("\n")
	}
	multiAZStr := "No"
	if r.MultiAZ {
		multiAZStr = "Yes"
	}
	b.WriteString(renderDetailLine("Multi-AZ", normalStyle.Render(multiAZStr)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Storage", normalStyle.Render(fmt.Sprintf("%d GB", r.StorageGB))))
	b.WriteString("\n")
	endpoint := r.Endpoint
	if endpoint == "" {
		endpoint = dimStyle.Render("(unavailable)")
	}
	b.WriteString(renderDetailLine("Endpoint", endpoint))
	b.WriteString("\n")
	if r.ClusterID != "" {
		b.WriteString(renderDetailLine("Cluster", normalStyle.Render(r.ClusterID)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	suffix := ""
	if r.IsClusterMember() {
		suffix = " Cluster"
	}
	b.WriteString(titleStyle.Render("Actions"))
	b.WriteString("\n")
	if r.CanStart() {
		b.WriteString(normalStyle.Render(fmt.Sprintf("  [s] Start%s", suffix)))
		b.WriteString("\n")
	} else {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  [s] Start%s", suffix)))
		b.WriteString("\n")
	}
	if r.CanStop() {
		b.WriteString(normalStyle.Render(fmt.Sprintf("  [x] Stop%s", suffix)))
		b.WriteString("\n")
	} else {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  [x] Stop%s", suffix)))
		b.WriteString("\n")
	}
	if r.CanFailover() {
		b.WriteString(normalStyle.Render(fmt.Sprintf("  [f] Failover%s", suffix)))
		b.WriteString("\n")
	} else {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  [f] Failover%s", suffix)))
		b.WriteString("\n")
	}
	b.WriteString(normalStyle.Render("  [r] Refresh"))
	b.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("esc: back • H: home"))
	return b.String()
}

func (rm rdsModel) viewClassPicker(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Select Instance Class"))
	b.WriteString("\n")
	if rm.selected != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Instance: %s  current: %s  engine: %s %s",
			rm.selected.DBInstanceID, rm.selected.InstanceClass, rm.selected.Engine, rm.selected.EngineVersion)))
		b.WriteString("\n")
	}
	if rm.classFiltering || rm.classFilter != "" {
		b.WriteString(filterStyle.Render(fmt.Sprintf("  /%s▏", rm.classFilter)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if len(rm.filteredClasses) == 0 {
		emptyText := "  No instance classes found"
		if len(rm.classes) > 0 {
			emptyText = "  No matching instance classes"
		}
		panel.WriteString(dimStyle.Render(emptyText))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-11, 5)
		start := 0
		if rm.classIdx >= visibleLines {
			start = rm.classIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(rm.filteredClasses))
		for i := start; i < end; i++ {
			class := rm.filteredClasses[i]
			cursor := "  "
			style := normalStyle
			marker := ""
			if rm.selected != nil && class == rm.selected.InstanceClass {
				marker = "  (current)"
			}
			if i == rm.classIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + class))
			panel.WriteString(dimStyle.Render(marker))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d classes", len(rm.filteredClasses), len(rm.classes))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: choose • esc: back"))
	return b.String()
}

func (rm rdsModel) viewConfirm(m Model) string {
	if rm.selected == nil {
		return ""
	}
	r := rm.selected

	// For Aurora cluster members, show cluster-level info
	targetLabel := "instance"
	targetID := r.DBInstanceID
	if r.IsClusterMember() {
		targetLabel = "cluster"
		targetID = r.ClusterID
	}

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(errorStyle.Render("Confirm Action"))
	b.WriteString("\n\n")

	if rm.action == "start" {
		b.WriteString(normalStyle.Render(fmt.Sprintf("  Are you sure you want to start %s %s?",
			targetLabel, targetID)))
		b.WriteString("\n\n")
		b.WriteString(normalStyle.Render("  [y] Yes  [n] No"))
		b.WriteString("\n")
	} else if rm.action == "modify" {
		b.WriteString(normalStyle.Render("  You are about to modify the instance class of:"))
		b.WriteString("\n")
		b.WriteString(selectedStyle.Render(fmt.Sprintf("  %s", r.DBInstanceID)))
		b.WriteString("\n\n")
		b.WriteString(m.renderEC2DetailLine("Current class", ec2ValueOrDash(r.InstanceClass)))
		b.WriteString(m.renderEC2DetailLine("New class", ec2ValueOrDash(rm.pendingClass)))
		applyLabel := "no (next maintenance window)"
		if rm.applyImmediately {
			applyLabel = "yes (may cause downtime now)"
		}
		b.WriteString(m.renderEC2DetailLine("Apply immediately", applyLabel))
		b.WriteString("\n")
		b.WriteString(normalStyle.Render("  Type the instance identifier to confirm:"))
		b.WriteString("\n")
		b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", rm.confirmInput)))
		b.WriteString("\n\n")
		b.WriteString(m.renderHelpBar("tab: toggle apply immediately • enter: confirm • esc: back"))
		b.WriteString("\n")
	} else {
		b.WriteString(normalStyle.Render(fmt.Sprintf("  You are about to %s %s:", rm.action, targetLabel)))
		b.WriteString("\n")
		b.WriteString(selectedStyle.Render(fmt.Sprintf("  %s", targetID)))
		b.WriteString("\n\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  Type the %s identifier to confirm:", targetLabel)))
		b.WriteString("\n")
		b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", rm.confirmInput)))
		b.WriteString("\n\n")
		b.WriteString(m.renderHelpBar("enter: confirm • esc: cancel"))
		b.WriteString("\n")
	}
	return b.String()
}
