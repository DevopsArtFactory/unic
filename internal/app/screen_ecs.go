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
		m.ecsClusterFilter = ""
		m.ecsClusterFilterActive = false
		m.screen = screenECSClusterList
		return m, nil, true

	case ecsServicesLoadedMsg:
		m.ecsServices = msg.services
		m.filteredECSServices = msg.services
		m.ecsServiceIdx = 0
		m.ecsServiceFilter = ""
		m.ecsServiceFilterActive = false
		m.screen = screenECSServiceList
		return m, nil, true

	case ecsTasksLoadedMsg:
		m.ecsTasks = msg.tasks
		m.filteredECSTasks = msg.tasks
		m.ecsTaskIdx = 0
		m.screen = screenECSTaskList
		return m, nil, true

	case ecsContainersLoadedMsg:
		m.ecsContainers = msg.containers
		m.ecsContainerIdx = 0
		m.screen = screenECSContainerList
		return m, nil, true

	case ecsExecDoneMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil, true
		}
		m.screen = screenECSContainerList
		return m, nil, true
	}
	return m, nil, false
}

// --- Cluster List ---

func (m Model) updateECSClusterList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.ecsClusterFilterActive {
		newFilter, deactivate, changed := handleFilterKey(key, m.ecsClusterFilter)
		m.ecsClusterFilter = newFilter
		if deactivate {
			m.ecsClusterFilterActive = false
		}
		if changed {
			m.filteredECSClusters = applyFilter(m.ecsClusters, m.ecsClusterFilter)
			m.ecsClusterIdx = 0
		}
		return m, nil
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
		m.ecsClusterFilterActive = true
	case "r":
		m.screen = screenLoading
		return m, m.loadECSClusters()
	case "enter":
		if len(m.filteredECSClusters) > 0 && m.ecsClusterIdx < len(m.filteredECSClusters) {
			cluster := m.filteredECSClusters[m.ecsClusterIdx]
			m.selectedECSCluster = &cluster
			m.screen = screenLoading
			return m, m.loadECSServices()
		}
	}
	return m, nil
}

func (m Model) viewECSClusterList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("ECS Clusters"))
	b.WriteString("\n")

	if m.ecsClusterFilterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.ecsClusterFilter)))
	} else if m.ecsClusterFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.ecsClusterFilter)))
	}
	b.WriteString("\n\n")

	if len(m.filteredECSClusters) == 0 {
		b.WriteString(dimStyle.Render("  No clusters found"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-8, 5)
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
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, c.DisplayTitle())))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d clusters", len(m.filteredECSClusters), len(m.ecsClusters))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • r: refresh • enter: select • esc: back • H: home"))
	return b.String()
}

// --- Service List ---

func (m Model) updateECSServiceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.ecsServiceFilterActive {
		newFilter, deactivate, changed := handleFilterKey(key, m.ecsServiceFilter)
		m.ecsServiceFilter = newFilter
		if deactivate {
			m.ecsServiceFilterActive = false
		}
		if changed {
			m.filteredECSServices = applyFilter(m.ecsServices, m.ecsServiceFilter)
			m.ecsServiceIdx = 0
		}
		return m, nil
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
		m.ecsServiceFilterActive = true
	case "r":
		m.screen = screenLoading
		return m, m.loadECSServices()
	case "enter":
		if len(m.filteredECSServices) > 0 && m.ecsServiceIdx < len(m.filteredECSServices) {
			svc := m.filteredECSServices[m.ecsServiceIdx]
			m.selectedECSService = &svc
			m.screen = screenLoading
			return m, m.loadECSTasks()
		}
	}
	return m, nil
}

func (m Model) viewECSServiceList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	clusterName := ""
	if m.selectedECSCluster != nil {
		clusterName = m.selectedECSCluster.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECS Services — %s", clusterName)))
	b.WriteString("\n")

	if m.ecsServiceFilterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.ecsServiceFilter)))
	} else if m.ecsServiceFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.ecsServiceFilter)))
	}
	b.WriteString("\n\n")

	if len(m.filteredECSServices) == 0 {
		b.WriteString(dimStyle.Render("  No services found"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-8, 5)
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
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, s.DisplayTitle())))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d services", len(m.filteredECSServices), len(m.ecsServices))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • r: refresh • enter: select • esc: back • H: home"))
	return b.String()
}

// --- Task List ---

func (m Model) updateECSTaskList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenECSServiceList
	case "up", "k":
		if m.ecsTaskIdx > 0 {
			m.ecsTaskIdx--
		}
	case "down", "j":
		if m.ecsTaskIdx < len(m.filteredECSTasks)-1 {
			m.ecsTaskIdx++
		}
	case "r":
		m.screen = screenLoading
		return m, m.loadECSTasks()
	case "enter":
		if len(m.filteredECSTasks) > 0 && m.ecsTaskIdx < len(m.filteredECSTasks) {
			task := m.filteredECSTasks[m.ecsTaskIdx]
			m.selectedECSTask = &task
			m.screen = screenLoading
			return m, m.loadECSContainers()
		}
	}
	return m, nil
}

func (m Model) viewECSTaskList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	svcName := ""
	if m.selectedECSService != nil {
		svcName = m.selectedECSService.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECS Tasks — %s", svcName)))
	b.WriteString("\n\n")

	if len(m.filteredECSTasks) == 0 {
		b.WriteString(dimStyle.Render("  No running tasks found"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-8, 5)
		start := 0
		if m.ecsTaskIdx >= visibleLines {
			start = m.ecsTaskIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredECSTasks))

		for i := start; i < end; i++ {
			t := m.filteredECSTasks[i]
			cursor := "  "
			style := normalStyle
			if i == m.ecsTaskIdx {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, t.DisplayTitle())))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d tasks", len(m.filteredECSTasks))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • r: refresh • enter: select • esc: back • H: home"))
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
	b.WriteString(m.renderStatusBar())
	taskID := ""
	if m.selectedECSTask != nil {
		taskID = m.selectedECSTask.TaskID
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECS Containers — %s", taskID)))
	b.WriteString("\n\n")

	if len(m.ecsContainers) == 0 {
		b.WriteString(dimStyle.Render("  No containers found"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-8, 5)
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
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, c.DisplayTitle())))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d containers", len(m.ecsContainers))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • enter: exec session • esc: back • H: home"))
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

		cmd := awsservice.BuildECSExecCommand(
			m.selectedECSCluster.ARN,
			m.selectedECSTask.TaskARN,
			container.Name,
			m.cfg.Region,
		)

		execCmd := tea.ExecProcess(cmd, func(err error) tea.Msg {
			return ecsExecDoneMsg{err: err}
		})
		return execCmd()
	}
}
