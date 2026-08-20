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

type ecsModel struct {
	clusters         []awsservice.ECSCluster
	filteredClusters []awsservice.ECSCluster
	clusterIdx       int
	selectedCluster  *awsservice.ECSCluster
	services         []awsservice.ECSService
	filteredServices []awsservice.ECSService
	serviceIdx       int
	selectedService  *awsservice.ECSService
	selectedDetail   *awsservice.ECSServiceDetail
	detailScroll     int
	tasks            []awsservice.ECSTask
	taskIdx          int
	selectedTask     *awsservice.ECSTask
	containers       []awsservice.ECSContainer
	containerIdx     int
}

func newECSModel() ecsModel {
	return ecsModel{}
}

func (em *ecsModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(em.loadClusters(*m))
}

func (em *ecsModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case ecsClustersLoadedMsg:
		em.clusters = msg.clusters
		em.filteredClusters = msg.clusters
		em.clusterIdx = 0
		em.selectedService = nil
		em.selectedDetail = nil
		em.detailScroll = 0
		m.resetFilter(filterECSClusters)
		m.screen = screenECSClusterList
		return *m, nil, true
	case ecsServicesLoadedMsg:
		em.services = msg.services
		em.filteredServices = msg.services
		em.serviceIdx = 0
		em.selectedService = nil
		em.selectedDetail = nil
		em.detailScroll = 0
		m.resetFilter(filterECSServices)
		m.screen = screenECSServiceList
		return *m, nil, true
	case ecsServiceDetailLoadedMsg:
		refreshing := m.watch.refreshing
		previousScroll := em.detailScroll
		em.selectedDetail = msg.detail
		if refreshing {
			visibleLines := max(m.height-9, 5)
			em.detailScroll = min(previousScroll, max(len(em.serviceDetailLines())-visibleLines, 0))
		} else {
			em.detailScroll = 0
		}
		if msg.detail != nil {
			summary := msg.detail.Summary()
			em.selectedService = &summary
			for i, svc := range em.services {
				if svc.ARN == summary.ARN {
					em.services[i] = summary
				}
			}
			em.filteredServices = applyFilter(em.services, m.filterValue(filterECSServices))
		}
		m.screen = screenECSServiceDetail
		return *m, nil, true
	case ecsTasksLoadedMsg:
		em.tasks = msg.tasks
		em.taskIdx = 0
		m.screen = screenECSTaskList
		return *m, nil, true
	case ecsContainersLoadedMsg:
		em.containers = msg.containers
		em.containerIdx = 0
		m.screen = screenECSContainerList
		return *m, nil, true
	case ecsExecDoneMsg:
		m.screen = screenECSContainerList
		return *m, nil, true
	}
	return *m, nil, false
}

func (em *ecsModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenECSClusterList:
		newM, cmd := em.updateClusterList(m, msg)
		return newM, cmd, true
	case screenECSServiceList:
		newM, cmd := em.updateServiceList(m, msg)
		return newM, cmd, true
	case screenECSServiceDetail:
		newM, cmd := em.updateServiceDetail(m, msg)
		return newM, cmd, true
	case screenECSTaskList:
		newM, cmd := em.updateTaskList(m, msg)
		return newM, cmd, true
	case screenECSContainerList:
		newM, cmd := em.updateContainerList(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (em ecsModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenECSClusterList:
		return em.viewClusterList(m), true
	case screenECSServiceList:
		return em.viewServiceList(m), true
	case screenECSServiceDetail:
		return em.viewServiceDetail(m), true
	case screenECSTaskList:
		return em.viewTaskList(m), true
	case screenECSContainerList:
		return em.viewContainerList(m), true
	default:
		return "", false
	}
}

func (em *ecsModel) ApplyFilter(m *Model, target filterTarget) bool {
	switch target {
	case filterECSClusters:
		em.filteredClusters = applyFilter(em.clusters, m.filterValue(target))
		em.clusterIdx = 0
		return true
	case filterECSServices:
		em.filteredServices = applyFilter(em.services, m.filterValue(target))
		em.serviceIdx = 0
		return true
	default:
		return false
	}
}

// --- Cluster List ---

func (em *ecsModel) updateClusterList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterECSClusters); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
	case "up", "k":
		em.clusterIdx = previousListIndex(em.clusterIdx, len(em.filteredClusters))
	case "down", "j":
		em.clusterIdx = nextListIndex(em.clusterIdx, len(em.filteredClusters))
	case "/":
		return *m, m.activateFilter(filterECSClusters)
	case "r":
		return m.startLoading(em.loadClusters(*m))
	case "enter":
		if len(em.filteredClusters) > 0 && em.clusterIdx < len(em.filteredClusters) {
			cluster := em.filteredClusters[em.clusterIdx]
			em.selectedCluster = &cluster
			return m.startLoading(em.loadServices(*m))
		}
	}
	return *m, nil
}

func (em ecsModel) viewClusterList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("ECS Clusters"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterECSClusters))
	b.WriteString("\n\n")

	if len(em.filteredClusters) == 0 {
		panel.WriteString(dimStyle.Render("  No clusters found"))
		panel.WriteString("\n")
	} else {
		// overhead: status bar (2) + title (1) + filter line (1) + blank (1) + list panel (2) + blank (1) + footer (1) = 10
		visibleLines := max(m.height-10, 5)
		start := 0
		if em.clusterIdx >= visibleLines {
			start = em.clusterIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filteredClusters))

		for i := start; i < end; i++ {
			c := em.filteredClusters[i]
			cursor := "  "
			style := normalStyle
			if i == em.clusterIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterECSClusters, c.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d clusters", len(em.filteredClusters), len(em.clusters))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: select • esc: back • H: home"))
	return b.String()
}

// --- Service List ---

func (em *ecsModel) updateServiceList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterECSServices); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenECSClusterList
	case "up", "k":
		em.serviceIdx = previousListIndex(em.serviceIdx, len(em.filteredServices))
	case "down", "j":
		em.serviceIdx = nextListIndex(em.serviceIdx, len(em.filteredServices))
	case "/":
		return *m, m.activateFilter(filterECSServices)
	case "r":
		return m.startLoading(em.loadServices(*m))
	case "enter":
		if len(em.filteredServices) > 0 && em.serviceIdx < len(em.filteredServices) {
			svc := em.filteredServices[em.serviceIdx]
			em.selectedService = &svc
			return m.startLoading(em.loadServiceDetail(*m))
		}
	}
	return *m, nil
}

func (em ecsModel) viewServiceList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	clusterName := ""
	if em.selectedCluster != nil {
		clusterName = em.selectedCluster.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECS Services — %s", clusterName)))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterECSServices))
	b.WriteString("\n\n")

	if len(em.filteredServices) == 0 {
		panel.WriteString(dimStyle.Render("  No services found"))
		panel.WriteString("\n")
	} else {
		// overhead: status bar (2) + title (1) + filter line (1) + blank (1) + list panel (2) + blank (1) + footer (1) = 10
		visibleLines := max(m.height-10, 5)
		start := 0
		if em.serviceIdx >= visibleLines {
			start = em.serviceIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filteredServices))

		for i := start; i < end; i++ {
			s := em.filteredServices[i]
			cursor := "  "
			style := normalStyle
			if i == em.serviceIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterECSServices, s.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d services", len(em.filteredServices), len(em.services))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: detail • esc: back • H: home"))
	return b.String()
}

// --- Service Detail ---

func (em *ecsModel) updateServiceDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := em.serviceDetailLines()
	visibleLines := max(m.height-9, 5)
	maxOffset := max(len(lines)-visibleLines, 0)

	switch msg.String() {
	case "q", "esc":
		m.stopWatch()
		m.screen = screenECSServiceList
	case "up", "k":
		if em.detailScroll > 0 {
			em.detailScroll--
		}
	case "down", "j":
		if em.detailScroll < maxOffset {
			em.detailScroll++
		}
	case "pgup":
		em.detailScroll -= visibleLines
		if em.detailScroll < 0 {
			em.detailScroll = 0
		}
	case "pgdown":
		em.detailScroll += visibleLines
		if em.detailScroll > maxOffset {
			em.detailScroll = maxOffset
		}
	case "r":
		return m.startLoading(em.loadServiceDetail(*m))
	case "enter":
		return m.startLoading(em.loadTasks(*m))
	}
	return *m, nil
}

func (em ecsModel) viewServiceDetail(m Model) string {
	if em.selectedDetail == nil {
		return ""
	}

	var b strings.Builder
	var panel strings.Builder
	detail := em.selectedDetail

	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECS Service Rollout — %s", detail.Name)))
	b.WriteString(m.watchBadge())
	b.WriteString("\n\n")

	lines := em.serviceDetailLines()
	if len(lines) == 0 {
		panel.WriteString(dimStyle.Render("  No rollout detail available"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-9, 5)
		start := em.detailScroll
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
	b.WriteString(m.renderHelpBar("↑/↓: scroll • pgup/pgdn: page • W: watch • I: interval • enter: tasks • r: refresh • esc: back • H: home"))
	return b.String()
}

func (em ecsModel) serviceDetailLines() []string {
	if em.selectedDetail == nil {
		return nil
	}

	detail := em.selectedDetail
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

func (em *ecsModel) updateTaskList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenECSServiceDetail
	case "up", "k":
		em.taskIdx = previousListIndex(em.taskIdx, len(em.tasks))
	case "down", "j":
		em.taskIdx = nextListIndex(em.taskIdx, len(em.tasks))
	case "r":
		return m.startLoading(em.loadTasks(*m))
	case "enter":
		if len(em.tasks) > 0 && em.taskIdx < len(em.tasks) {
			task := em.tasks[em.taskIdx]
			em.selectedTask = &task
			return m.startLoading(em.loadContainers(*m))
		}
	}
	return *m, nil
}

func (em ecsModel) viewTaskList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	svcName := ""
	if em.selectedService != nil {
		svcName = em.selectedService.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECS Tasks — %s", svcName)))
	b.WriteString("\n\n")

	if len(em.tasks) == 0 {
		panel.WriteString(dimStyle.Render("  No running tasks found"))
		panel.WriteString("\n")
	} else {
		// overhead: status bar (2) + title (1) + blank (1) + list panel (2) + blank (1) + footer (1) = 9
		visibleLines := max(m.height-9, 5)
		start := 0
		if em.taskIdx >= visibleLines {
			start = em.taskIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.tasks))

		for i := start; i < end; i++ {
			t := em.tasks[i]
			cursor := "  "
			style := normalStyle
			if i == em.taskIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, t.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d tasks", len(em.tasks))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • r: refresh • enter: select • esc: service detail • H: home"))
	return b.String()
}

// --- Container List ---

func (em *ecsModel) updateContainerList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenECSTaskList
	case "up", "k":
		em.containerIdx = previousListIndex(em.containerIdx, len(em.containers))
	case "down", "j":
		em.containerIdx = nextListIndex(em.containerIdx, len(em.containers))
	case "enter":
		if len(em.containers) > 0 && em.containerIdx < len(em.containers) {
			container := em.containers[em.containerIdx]
			if !container.ExecEnabled {
				m.errMsg = fmt.Sprintf(
					"ECS Exec is not enabled for container %q.\n\nTo enable it, update the task definition with enableExecuteCommand=true\nand ensure the task IAM role has ssmmessages permissions.",
					container.Name,
				)
				m.screen = screenError
				return *m, nil
			}
			return *m, em.startExec(*m, container)
		}
	}
	return *m, nil
}

func (em ecsModel) viewContainerList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	taskID := ""
	if em.selectedTask != nil {
		taskID = em.selectedTask.TaskID
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECS Containers — %s", taskID)))
	b.WriteString("\n\n")

	if len(em.containers) == 0 {
		panel.WriteString(dimStyle.Render("  No containers found"))
		panel.WriteString("\n")
	} else {
		// overhead: status bar (2) + title (1) + blank (1) + list panel (2) + blank (1) + footer (1) = 9
		visibleLines := max(m.height-9, 5)
		start := 0
		if em.containerIdx >= visibleLines {
			start = em.containerIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.containers))

		for i := start; i < end; i++ {
			c := em.containers[i]
			cursor := "  "
			style := normalStyle
			if i == em.containerIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, c.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d containers", len(em.containers))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • enter: exec session • esc: back • H: home"))
	return b.String()
}

// --- Load Commands ---

func (em ecsModel) loadClusters(m Model) tea.Cmd {
	return func() tea.Msg {
		if err := awsservice.CheckAWSCLIInstalled(); err != nil {
			return errMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(m.commandContext(), ecsAPITimeout)
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

func (em ecsModel) loadServices(m Model) tea.Cmd {
	return func() tea.Msg {
		if em.selectedCluster == nil {
			return errMsg{err: fmt.Errorf("no cluster selected")}
		}
		ctx, cancel := context.WithTimeout(m.commandContext(), ecsAPITimeout)
		defer cancel()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		services, err := repo.ListServices(ctx, em.selectedCluster.ARN)
		if err != nil {
			return errMsg{err: err}
		}
		if len(services) == 0 {
			return errMsg{err: fmt.Errorf("no services found in cluster %s", em.selectedCluster.Name)}
		}
		return ecsServicesLoadedMsg{services: services}
	}
}

func (em ecsModel) loadTasks(m Model) tea.Cmd {
	return func() tea.Msg {
		if em.selectedCluster == nil || em.selectedService == nil {
			return errMsg{err: fmt.Errorf("no cluster or service selected")}
		}
		ctx, cancel := context.WithTimeout(m.commandContext(), ecsAPITimeout)
		defer cancel()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		tasks, err := repo.ListTasks(ctx, em.selectedCluster.ARN, em.selectedService.ARN)
		if err != nil {
			return errMsg{err: err}
		}
		if len(tasks) == 0 {
			return errMsg{err: fmt.Errorf("no running tasks found for service %s", em.selectedService.Name)}
		}
		return ecsTasksLoadedMsg{tasks: tasks}
	}
}

func (em ecsModel) loadServiceDetail(m Model) tea.Cmd {
	return func() tea.Msg {
		if em.selectedCluster == nil || em.selectedService == nil {
			return errMsg{err: fmt.Errorf("no cluster or service selected")}
		}
		ctx, cancel := context.WithTimeout(m.commandContext(), ecsAPITimeout)
		defer cancel()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		detail, err := repo.DescribeServiceDetail(ctx, em.selectedCluster.ARN, em.selectedService.ARN)
		if err != nil {
			return errMsg{err: err}
		}
		return ecsServiceDetailLoadedMsg{detail: detail}
	}
}

func (em ecsModel) loadContainers(m Model) tea.Cmd {
	return func() tea.Msg {
		if em.selectedCluster == nil || em.selectedTask == nil {
			return errMsg{err: fmt.Errorf("no cluster or task selected")}
		}
		ctx, cancel := context.WithTimeout(m.commandContext(), ecsAPITimeout)
		defer cancel()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		containers, err := repo.DescribeTaskContainers(ctx, em.selectedCluster.ARN, em.selectedTask.TaskARN)
		if err != nil {
			return errMsg{err: err}
		}
		if len(containers) == 0 {
			return errMsg{err: fmt.Errorf("no containers found for task %s", em.selectedTask.TaskID)}
		}
		return ecsContainersLoadedMsg{containers: containers}
	}
}

// startECSExec launches an ECS exec session for the given container.
func (em ecsModel) startExec(m Model, container awsservice.ECSContainer) tea.Cmd {
	return func() tea.Msg {
		if em.selectedCluster == nil || em.selectedTask == nil {
			return errMsg{err: fmt.Errorf("no cluster or task selected")}
		}

		ctx, cancel := context.WithTimeout(m.commandContext(), ecsAPITimeout)
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
			em.selectedCluster.ARN,
			em.selectedTask.TaskARN,
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
