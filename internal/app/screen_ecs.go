package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

const ecsAPITimeout = 30 * time.Second

// handleECSMsg routes ECS messages to the correct screen.
func (m Model) handleECSMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case ecsClustersLoadedMsg:
		m.ecsClusters = msg.clusters
		m.filteredECSClusters = msg.clusters
		m.ecsClusterIdx = 0
		m.selectedECSService = nil
		m.selectedECSDetail = nil
		m.ecsDetailScroll = 0
		m.resetFilter(filterECSClusters)
		m.screen = screenECSClusterList
		return m, nil, true

	case ecsServicesLoadedMsg:
		m.ecsServices = msg.services
		m.filteredECSServices = msg.services
		m.ecsServiceIdx = 0
		m.selectedECSService = nil
		m.selectedECSDetail = nil
		m.ecsDetailScroll = 0
		m.resetFilter(filterECSServices)
		m.screen = screenECSServiceList
		return m, nil, true

	case ecsServiceDetailLoadedMsg:
		m.selectedECSDetail = msg.detail
		m.ecsDetailScroll = 0
		if msg.detail != nil {
			summary := msg.detail.Summary()
			m.selectedECSService = &summary
			for i, svc := range m.ecsServices {
				if svc.ARN == summary.ARN {
					m.ecsServices[i] = summary
				}
			}
			m.filteredECSServices = applyFilter(m.ecsServices, m.filterValue(filterECSServices))
		}
		m.screen = screenECSServiceDetail
		return m, nil, true

	case ecsTasksLoadedMsg:
		m.ecsTasks = msg.tasks
		m.ecsTaskIdx = 0
		m.screen = screenECSTaskList
		return m, nil, true

	case ecsContainersLoadedMsg:
		m.ecsContainers = msg.containers
		m.ecsContainerIdx = 0
		m.screen = screenECSContainerList
		return m, nil, true

	case ecsExecDoneMsg:
		m.screen = screenECSContainerList
		return m, nil, true
	}
	return m, nil, false
}

// --- Cluster List ---

func (m Model) updateECSClusterList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterECSClusters); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
	case "up", "k":
		if m.ecsClusterIdx > 0 {
			m.ecsClusterIdx--
		}
	case "down", "j":
		if m.ecsClusterIdx < len(m.filteredECSClusters)-1 {
			m.ecsClusterIdx++
		}
	case "/":
		return m, m.activateFilter(filterECSClusters)
	case "r":
		return m.startLoading(m.loadECSClusters())
	case "enter":
		if len(m.filteredECSClusters) > 0 && m.ecsClusterIdx < len(m.filteredECSClusters) {
			cluster := m.filteredECSClusters[m.ecsClusterIdx]
			m.selectedECSCluster = &cluster
			return m.startLoading(m.loadECSServices())
		}
	}
	return m, nil
}

func (m Model) viewECSClusterList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("ECS Clusters"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterECSClusters))
	b.WriteString("\n\n")

	if len(m.filteredECSClusters) == 0 {
		panel.WriteString(dimStyle.Render("  No clusters found"))
		panel.WriteString("\n")
	} else {
		// overhead: status bar (2) + title (1) + filter line (1) + blank (1) + list panel (2) + blank (1) + footer (1) = 10
		visibleLines := max(m.height-10, 5)
		start := 0
		if m.ecsClusterIdx >= visibleLines {
			start = m.ecsClusterIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredECSClusters))

		for i := start; i < end; i++ {
			c := m.filteredECSClusters[i]
			cursor := "  "
			style := normalStyle
			if i == m.ecsClusterIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterECSClusters, c.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d clusters", len(m.filteredECSClusters), len(m.ecsClusters))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: select • esc: back • H: home"))
	return b.String()
}

// --- Service List ---

func (m Model) updateECSServiceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterECSServices); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenECSClusterList
	case "up", "k":
		if m.ecsServiceIdx > 0 {
			m.ecsServiceIdx--
		}
	case "down", "j":
		if m.ecsServiceIdx < len(m.filteredECSServices)-1 {
			m.ecsServiceIdx++
		}
	case "/":
		return m, m.activateFilter(filterECSServices)
	case "r":
		return m.startLoading(m.loadECSServices())
	case "enter":
		if len(m.filteredECSServices) > 0 && m.ecsServiceIdx < len(m.filteredECSServices) {
			svc := m.filteredECSServices[m.ecsServiceIdx]
			m.selectedECSService = &svc
			return m.startLoading(m.loadECSServiceDetail())
		}
	}
	return m, nil
}

func (m Model) viewECSServiceList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	clusterName := ""
	if m.selectedECSCluster != nil {
		clusterName = m.selectedECSCluster.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECS Services — %s", clusterName)))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterECSServices))
	b.WriteString("\n\n")

	if len(m.filteredECSServices) == 0 {
		panel.WriteString(dimStyle.Render("  No services found"))
		panel.WriteString("\n")
	} else {
		// overhead: status bar (2) + title (1) + filter line (1) + blank (1) + list panel (2) + blank (1) + footer (1) = 10
		visibleLines := max(m.height-10, 5)
		start := 0
		if m.ecsServiceIdx >= visibleLines {
			start = m.ecsServiceIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredECSServices))

		for i := start; i < end; i++ {
			s := m.filteredECSServices[i]
			cursor := "  "
			style := normalStyle
			if i == m.ecsServiceIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterECSServices, s.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d services", len(m.filteredECSServices), len(m.ecsServices))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: detail • esc: back • H: home"))
	return b.String()
}

// --- Service Detail ---

func (m Model) updateECSServiceDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := m.ecsServiceDetailLines()
	visibleLines := max(m.height-9, 5)
	maxOffset := max(len(lines)-visibleLines, 0)

	switch msg.String() {
	case "q", "esc":
		m.screen = screenECSServiceList
	case "up", "k":
		if m.ecsDetailScroll > 0 {
			m.ecsDetailScroll--
		}
	case "down", "j":
		if m.ecsDetailScroll < maxOffset {
			m.ecsDetailScroll++
		}
	case "pgup":
		m.ecsDetailScroll -= visibleLines
		if m.ecsDetailScroll < 0 {
			m.ecsDetailScroll = 0
		}
	case "pgdown":
		m.ecsDetailScroll += visibleLines
		if m.ecsDetailScroll > maxOffset {
			m.ecsDetailScroll = maxOffset
		}
	case "r":
		return m.startLoading(m.loadECSServiceDetail())
	case "enter":
		return m.startLoading(m.loadECSTasks())
	}
	return m, nil
}

func (m Model) viewECSServiceDetail() string {
	if m.selectedECSDetail == nil {
		return ""
	}

	var b strings.Builder
	var panel strings.Builder
	detail := m.selectedECSDetail

	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECS Service Rollout — %s", detail.Name)))
	b.WriteString("\n\n")

	lines := m.ecsServiceDetailLines()
	if len(lines) == 0 {
		panel.WriteString(dimStyle.Render("  No rollout detail available"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-9, 5)
		start := m.ecsDetailScroll
		maxOffset := max(len(lines)-visibleLines, 0)
		if start > maxOffset {
			start = maxOffset
		}
		if start < 0 {
			start = 0
		}
		end := min(start+visibleLines, len(lines))
		for _, line := range lines[start:end] {
			panel.WriteString(line)
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  lines %d-%d/%d", start+1, end, len(lines))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: scroll • pgup/pgdn: page • enter: tasks • r: refresh • esc: back • H: home"))
	return b.String()
}

func (m Model) ecsServiceDetailLines() []string {
	if m.selectedECSDetail == nil {
		return nil
	}

	detail := m.selectedECSDetail
	lines := []string{
		renderDetailLine("Status", renderECSServiceStatus(detail.Status)),
		renderDetailLine("Launch", renderECSValue(detail.LaunchType)),
		renderDetailLine("Tasks", renderECSValue(fmt.Sprintf("running:%d desired:%d pending:%d", detail.RunningCount, detail.DesiredCount, detail.PendingCount))),
		renderDetailLine("Deploy Ctrl", renderECSValue(zeroFallback(detail.DeploymentControllerType, "-"))),
		renderDetailLine("Schedule", renderECSValue(zeroFallback(detail.SchedulingStrategy, "-"))),
		renderDetailLine("Task Def", renderECSValue(detail.TaskDefinitionLabel())),
		renderDetailLine("Platform", renderECSValue(zeroFallback(detail.PlatformVersion, "-"))),
		renderDetailLine("Network", renderECSValue(zeroFallback(detail.NetworkMode, "-"))),
		renderDetailLine("Compat", renderECSValue(detail.CompatibilityLabel())),
	}

	execStatus := "Disabled"
	if detail.EnableExecuteCommand {
		execStatus = "Enabled"
	}
	lines = append(lines, renderDetailLine("Exec", renderECSExecStatus(execStatus)))

	lines = append(lines, "")
	lines = append(lines, titleStyle.Render("Deployments"))
	if len(detail.Deployments) == 0 {
		lines = append(lines, dimStyle.Render("  No deployments found"))
	} else {
		for _, deployment := range detail.Deployments {
			lines = append(lines, renderECSDeploymentSummary(deployment))
			if deployment.RolloutStateReason != "" {
				lines = append(lines, "    "+dimStyle.Render(deployment.RolloutStateReason))
			}
			if !deployment.UpdatedAt.IsZero() {
				lines = append(lines, "    "+dimStyle.Render("Updated "+deployment.UpdatedAt.Local().Format("2006-01-02 15:04:05")))
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, titleStyle.Render("Task Definition Images"))
	if len(detail.ContainerImages) == 0 {
		lines = append(lines, dimStyle.Render("  No container images found"))
	} else {
		for _, image := range detail.ContainerImages {
			lines = append(lines, renderDetailLine(image.Name, normalStyle.Render(image.Image)))
		}
	}

	lines = append(lines, "")
	lines = append(lines, titleStyle.Render("Recent Service Events"))
	if len(detail.Events) == 0 {
		lines = append(lines, dimStyle.Render("  No recent service events"))
	} else {
		for _, event := range detail.Events {
			lines = append(lines, "  "+normalStyle.Render(event.DisplayTitle()))
		}
	}

	return lines
}

// --- Task List ---

func (m Model) updateECSTaskList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenECSServiceDetail
	case "up", "k":
		if m.ecsTaskIdx > 0 {
			m.ecsTaskIdx--
		}
	case "down", "j":
		if m.ecsTaskIdx < len(m.ecsTasks)-1 {
			m.ecsTaskIdx++
		}
	case "r":
		return m.startLoading(m.loadECSTasks())
	case "enter":
		if len(m.ecsTasks) > 0 && m.ecsTaskIdx < len(m.ecsTasks) {
			task := m.ecsTasks[m.ecsTaskIdx]
			m.selectedECSTask = &task
			return m.startLoading(m.loadECSContainers())
		}
	}
	return m, nil
}

func (m Model) viewECSTaskList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	svcName := ""
	if m.selectedECSService != nil {
		svcName = m.selectedECSService.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECS Tasks — %s", svcName)))
	b.WriteString("\n\n")

	if len(m.ecsTasks) == 0 {
		panel.WriteString(dimStyle.Render("  No running tasks found"))
		panel.WriteString("\n")
	} else {
		// overhead: status bar (2) + title (1) + blank (1) + list panel (2) + blank (1) + footer (1) = 9
		visibleLines := max(m.height-9, 5)
		start := 0
		if m.ecsTaskIdx >= visibleLines {
			start = m.ecsTaskIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.ecsTasks))

		for i := start; i < end; i++ {
			t := m.ecsTasks[i]
			cursor := "  "
			style := normalStyle
			if i == m.ecsTaskIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, t.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d tasks", len(m.ecsTasks))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • r: refresh • enter: select • esc: service detail • H: home"))
	return b.String()
}

// --- Container List ---

func (m Model) updateECSContainerList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenECSTaskList
	case "up", "k":
		if m.ecsContainerIdx > 0 {
			m.ecsContainerIdx--
		}
	case "down", "j":
		if m.ecsContainerIdx < len(m.ecsContainers)-1 {
			m.ecsContainerIdx++
		}
	case "enter":
		if len(m.ecsContainers) > 0 && m.ecsContainerIdx < len(m.ecsContainers) {
			container := m.ecsContainers[m.ecsContainerIdx]
			if !container.ExecEnabled {
				m.errMsg = fmt.Sprintf(
					"ECS Exec is not enabled for container %q.\n\nTo enable it, update the task definition with enableExecuteCommand=true\nand ensure the task IAM role has ssmmessages permissions.",
					container.Name,
				)
				m.screen = screenError
				return m, nil
			}
			return m, m.startECSExec(container)
		}
	}
	return m, nil
}

func (m Model) viewECSContainerList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	taskID := ""
	if m.selectedECSTask != nil {
		taskID = m.selectedECSTask.TaskID
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECS Containers — %s", taskID)))
	b.WriteString("\n\n")

	if len(m.ecsContainers) == 0 {
		panel.WriteString(dimStyle.Render("  No containers found"))
		panel.WriteString("\n")
	} else {
		// overhead: status bar (2) + title (1) + blank (1) + list panel (2) + blank (1) + footer (1) = 9
		visibleLines := max(m.height-9, 5)
		start := 0
		if m.ecsContainerIdx >= visibleLines {
			start = m.ecsContainerIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.ecsContainers))

		for i := start; i < end; i++ {
			c := m.ecsContainers[i]
			cursor := "  "
			style := normalStyle
			if i == m.ecsContainerIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, c.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d containers", len(m.ecsContainers))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • enter: exec session • esc: back • H: home"))
	return b.String()
}

// --- Load Commands ---

func (m Model) loadECSClusters() tea.Cmd {
	return func() tea.Msg {
		if err := awsservice.CheckAWSCLIInstalled(); err != nil {
			return errMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), ecsAPITimeout)
		defer cancel()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		clusters, err := repo.ListClusters(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(clusters) == 0 {
			return errMsg{err: fmt.Errorf("no ECS clusters found in region %s", m.cfg.Region)}
		}
		return ecsClustersLoadedMsg{clusters: clusters}
	}
}

func (m Model) loadECSServices() tea.Cmd {
	return func() tea.Msg {
		if m.selectedECSCluster == nil {
			return errMsg{err: fmt.Errorf("no cluster selected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), ecsAPITimeout)
		defer cancel()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		services, err := repo.ListServices(ctx, m.selectedECSCluster.ARN)
		if err != nil {
			return errMsg{err: err}
		}
		if len(services) == 0 {
			return errMsg{err: fmt.Errorf("no services found in cluster %s", m.selectedECSCluster.Name)}
		}
		return ecsServicesLoadedMsg{services: services}
	}
}

func (m Model) loadECSTasks() tea.Cmd {
	return func() tea.Msg {
		if m.selectedECSCluster == nil || m.selectedECSService == nil {
			return errMsg{err: fmt.Errorf("no cluster or service selected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), ecsAPITimeout)
		defer cancel()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		tasks, err := repo.ListTasks(ctx, m.selectedECSCluster.ARN, m.selectedECSService.ARN)
		if err != nil {
			return errMsg{err: err}
		}
		if len(tasks) == 0 {
			return errMsg{err: fmt.Errorf("no running tasks found for service %s", m.selectedECSService.Name)}
		}
		return ecsTasksLoadedMsg{tasks: tasks}
	}
}

func (m Model) loadECSServiceDetail() tea.Cmd {
	return func() tea.Msg {
		if m.selectedECSCluster == nil || m.selectedECSService == nil {
			return errMsg{err: fmt.Errorf("no cluster or service selected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), ecsAPITimeout)
		defer cancel()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		detail, err := repo.DescribeServiceDetail(ctx, m.selectedECSCluster.ARN, m.selectedECSService.ARN)
		if err != nil {
			return errMsg{err: err}
		}
		return ecsServiceDetailLoadedMsg{detail: detail}
	}
}

func (m Model) loadECSContainers() tea.Cmd {
	return func() tea.Msg {
		if m.selectedECSCluster == nil || m.selectedECSTask == nil {
			return errMsg{err: fmt.Errorf("no cluster or task selected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), ecsAPITimeout)
		defer cancel()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		containers, err := repo.DescribeTaskContainers(ctx, m.selectedECSCluster.ARN, m.selectedECSTask.TaskARN)
		if err != nil {
			return errMsg{err: err}
		}
		if len(containers) == 0 {
			return errMsg{err: fmt.Errorf("no containers found for task %s", m.selectedECSTask.TaskID)}
		}
		return ecsContainersLoadedMsg{containers: containers}
	}
}

// startECSExec launches an ECS exec session for the given container.
func (m Model) startECSExec(container awsservice.ECSContainer) tea.Cmd {
	return func() tea.Msg {
		if m.selectedECSCluster == nil || m.selectedECSTask == nil {
			return errMsg{err: fmt.Errorf("no cluster or task selected")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), ecsAPITimeout)
		defer cancel()

		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}

		credEnv, err := repo.ResolveCredentialEnv(ctx)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to resolve credentials for ECS exec: %w", err)}
		}

		cmd := awsservice.BuildECSExecCommand(
			m.selectedECSCluster.ARN,
			m.selectedECSTask.TaskARN,
			container.Name,
			m.cfg.Region,
			credEnv,
		)

		execCmd := tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return errMsg{err: err}
			}
			return ecsExecDoneMsg{}
		})
		return execCmd()
	}
}

func renderECSValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return dimStyle.Render("-")
	}
	return normalStyle.Render(value)
}

func renderECSExecStatus(value string) string {
	if value == "Enabled" {
		return successStyle.Render(value)
	}
	return dimStyle.Render(value)
}

func renderECSServiceStatus(status string) string {
	switch status {
	case "ACTIVE":
		return successStyle.Render(status)
	case "DRAINING":
		return warningStyle.Render(status)
	case "INACTIVE":
		return errorStyle.Render(status)
	default:
		return normalStyle.Render(status)
	}
}

func renderECSRolloutState(state string) string {
	switch state {
	case "COMPLETED":
		return successStyle.Render(state)
	case "IN_PROGRESS":
		return warningStyle.Render(state)
	case "FAILED":
		return errorStyle.Render(state)
	default:
		return normalStyle.Render(zeroFallback(state, "-"))
	}
}

func renderECSDeploymentSummary(deployment awsservice.ECSDeployment) string {
	prefix := "  "
	if deployment.Status == "PRIMARY" {
		prefix = selectedStyle.Render("▸ ")
	}
	return prefix +
		normalStyle.Render(fmt.Sprintf("%-9s rollout:", deployment.Status)) +
		renderECSRolloutState(deployment.RolloutState) +
		normalStyle.Render(fmt.Sprintf(
			"  tasks:%d/%d pending:%d failed:%d  td:%s",
			deployment.RunningCount,
			deployment.DesiredCount,
			deployment.PendingCount,
			deployment.FailedTasks,
			zeroFallback(deployment.TaskDefinition, "-"),
		))
}

func zeroFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
