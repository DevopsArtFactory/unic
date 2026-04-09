package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func (m Model) handleRDSMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case rdsInstancesLoadedMsg:
		m.rdsInstances = msg.instances
		m.filteredRDS = msg.instances
		m.rdsIdx = 0
		m.screen = screenRDSList
		return m, nil, true

	case rdsActionDoneMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil, true
		}
		m.rdsPolling = true
		m.screen = screenRDSDetail
		return m, m.tickRDSPoll(msg.instanceID), true

	case rdsStatusRefreshedMsg:
		if msg.err != nil {
			m.rdsPolling = false
			return m, nil, true
		}
		m.selectedRDS = msg.instance
		for i, inst := range m.rdsInstances {
			if inst.DBInstanceID == msg.instance.DBInstanceID {
				m.rdsInstances[i] = *msg.instance
				break
			}
		}
		m.filteredRDS = applyFilter(m.rdsInstances, m.filterValue(filterRDS))
		m.rdsIdx = 0
		if awsservice.IsTransitionalStatus(msg.instance.Status) {
			return m, m.tickRDSPoll(msg.instance.DBInstanceID), true
		}
		m.rdsPolling = false
		return m, nil, true

	case rdsTickMsg:
		if m.rdsPolling {
			return m, m.pollRDSStatus(msg.instanceID), true
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) updateRDSList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterRDS); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterRDS)
	case "up", "k":
		if m.rdsIdx > 0 {
			m.rdsIdx--
		}
	case "down", "j":
		if m.rdsIdx < len(m.filteredRDS)-1 {
			m.rdsIdx++
		}
	case "/":
		return m, m.activateFilter(filterRDS)
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
			m.rdsConfirmInput = ""
			m.screen = screenRDSConfirm
		}
	case "x":
		if m.selectedRDS != nil && m.selectedRDS.CanStop() {
			m.rdsAction = "stop"
			m.rdsConfirmInput = ""
			m.screen = screenRDSConfirm
		}
	case "f":
		if m.selectedRDS != nil && m.selectedRDS.CanFailover() {
			m.rdsAction = "failover"
			m.rdsConfirmInput = ""
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
	// Start action uses simple y/n confirmation
	if m.rdsAction == "start" {
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

	// Stop/failover require typing the identifier to confirm
	// For Aurora cluster members, confirm with cluster ID; for standalone, instance ID
	confirmTarget := ""
	if m.selectedRDS != nil {
		if m.selectedRDS.IsClusterMember() {
			confirmTarget = m.selectedRDS.ClusterID
		} else {
			confirmTarget = m.selectedRDS.DBInstanceID
		}
	}
	switch msg.String() {
	case "esc":
		m.screen = screenRDSDetail
	case "enter":
		if m.selectedRDS != nil && m.rdsConfirmInput == confirmTarget {
			m.screen = screenRDSDetail
			return m, m.executeRDSAction(m.rdsAction, m.selectedRDS.DBInstanceID)
		}
	case "backspace":
		if len(m.rdsConfirmInput) > 0 {
			m.rdsConfirmInput = m.rdsConfirmInput[:len(m.rdsConfirmInput)-1]
		}
	default:
		if runes := msg.Runes; len(runes) > 0 {
			m.rdsConfirmInput += string(runes)
		}
	}
	return m, nil
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
	clusterID := ""
	if m.selectedRDS != nil {
		clusterID = m.selectedRDS.ClusterID
	}
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

func (m Model) viewRDSList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("RDS Instances"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterRDS))
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
	b.WriteString(dimStyle.Render("esc: back • H: home"))
	return b.String()
}

func (m Model) viewRDSConfirm() string {
	if m.selectedRDS == nil {
		return ""
	}
	r := m.selectedRDS

	// For Aurora cluster members, show cluster-level info
	targetLabel := "instance"
	targetID := r.DBInstanceID
	if r.IsClusterMember() {
		targetLabel = "cluster"
		targetID = r.ClusterID
	}

	var b strings.Builder
	b.WriteString(errorStyle.Render("Confirm Action"))
	b.WriteString("\n\n")

	if m.rdsAction == "start" {
		b.WriteString(normalStyle.Render(fmt.Sprintf("  Are you sure you want to start %s %s?",
			targetLabel, targetID)))
		b.WriteString("\n\n")
		b.WriteString(normalStyle.Render("  [y] Yes  [n] No"))
		b.WriteString("\n")
	} else {
		b.WriteString(normalStyle.Render(fmt.Sprintf("  You are about to %s %s:", m.rdsAction, targetLabel)))
		b.WriteString("\n")
		b.WriteString(selectedStyle.Render(fmt.Sprintf("  %s", targetID)))
		b.WriteString("\n\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  Type the %s identifier to confirm:", targetLabel)))
		b.WriteString("\n")
		b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", m.rdsConfirmInput)))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("  enter: confirm • esc: cancel"))
		b.WriteString("\n")
	}
	return b.String()
}
