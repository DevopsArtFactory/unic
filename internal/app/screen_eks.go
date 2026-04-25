package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

const eksAPITimeout = 30 * time.Second

func (m Model) handleEKSMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case eksClustersLoadedMsg:
		m.eksClusters = msg.clusters
		m.filteredEKSClusters = msg.clusters
		m.eksClusterIdx = 0
		m.selectedEKSCluster = nil
		m.eksNodeGroups = nil
		m.filteredEKSNodeGroups = nil
		m.selectedEKSNodeGroup = nil
		m.eksNodeGroupScroll = 0
		m.resetFilter(filterEKSClusters)
		m.screen = screenEKSClusterList
		return m, nil, true
	case eksNodeGroupsLoadedMsg:
		m.eksNodeGroups = msg.nodeGroups
		m.filteredEKSNodeGroups = msg.nodeGroups
		m.eksNodeGroupIdx = 0
		m.selectedEKSNodeGroup = nil
		m.eksNodeGroupScroll = 0
		m.resetFilter(filterEKSNodeGroups)
		m.screen = screenEKSNodeGroupList
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) updateEKSClusterList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterEKSClusters); handled {
		return m, cmd
	}

	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
	case "up", "k":
		m.eksClusterIdx = previousListIndex(m.eksClusterIdx, len(m.filteredEKSClusters))
	case "down", "j":
		m.eksClusterIdx = nextListIndex(m.eksClusterIdx, len(m.filteredEKSClusters))
	case "/":
		return m, m.activateFilter(filterEKSClusters)
	case "r":
		return m.startLoading(m.loadEKSClusters())
	case "enter":
		if len(m.filteredEKSClusters) > 0 && m.eksClusterIdx < len(m.filteredEKSClusters) {
			cluster := m.filteredEKSClusters[m.eksClusterIdx]
			m.selectedEKSCluster = &cluster
			return m.startLoading(m.loadEKSNodeGroups())
		}
	}
	return m, nil
}

func (m Model) viewEKSClusterList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("EKS Clusters"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterEKSClusters))
	b.WriteString("\n\n")

	if len(m.filteredEKSClusters) == 0 {
		panel.WriteString(dimStyle.Render("  No EKS clusters found"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-16, 5)
		start := 0
		if m.eksClusterIdx >= visibleLines {
			start = m.eksClusterIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredEKSClusters))
		for i := start; i < end; i++ {
			cluster := m.filteredEKSClusters[i]
			cursor := "  "
			style := normalStyle
			if i == m.eksClusterIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterEKSClusters, cluster.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d clusters", len(m.filteredEKSClusters), len(m.eksClusters))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	if cluster := m.currentEKSCluster(); cluster != nil {
		b.WriteString(titleStyle.Render("Selected Cluster"))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Version", cluster.Version))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Status", cluster.Status))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Endpoint", cluster.EndpointVisibility()))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("ARN", cluster.ARN))
		b.WriteString("\n\n")
	}
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: node groups • esc: back • H: home"))
	return b.String()
}

func (m Model) updateEKSNodeGroupList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterEKSNodeGroups); handled {
		return m, cmd
	}

	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenEKSClusterList
	case "up", "k":
		m.eksNodeGroupIdx = previousListIndex(m.eksNodeGroupIdx, len(m.filteredEKSNodeGroups))
	case "down", "j":
		m.eksNodeGroupIdx = nextListIndex(m.eksNodeGroupIdx, len(m.filteredEKSNodeGroups))
	case "/":
		return m, m.activateFilter(filterEKSNodeGroups)
	case "r":
		return m.startLoading(m.loadEKSNodeGroups())
	case "enter":
		if len(m.filteredEKSNodeGroups) > 0 && m.eksNodeGroupIdx < len(m.filteredEKSNodeGroups) {
			nodeGroup := m.filteredEKSNodeGroups[m.eksNodeGroupIdx]
			m.selectedEKSNodeGroup = &nodeGroup
			m.eksNodeGroupScroll = 0
			m.screen = screenEKSNodeGroupDetail
		}
	}
	return m, nil
}

func (m Model) viewEKSNodeGroupList() string {
	var b strings.Builder
	var panel strings.Builder
	clusterName := ""
	if m.selectedEKSCluster != nil {
		clusterName = m.selectedEKSCluster.Name
	}
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(fmt.Sprintf("EKS Node Groups — %s", clusterName)))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterEKSNodeGroups))
	b.WriteString("\n\n")

	if len(m.filteredEKSNodeGroups) == 0 {
		panel.WriteString(dimStyle.Render("  No managed node groups found"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-17, 5)
		start := 0
		if m.eksNodeGroupIdx >= visibleLines {
			start = m.eksNodeGroupIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredEKSNodeGroups))
		for i := start; i < end; i++ {
			nodeGroup := m.filteredEKSNodeGroups[i]
			cursor := "  "
			style := normalStyle
			if i == m.eksNodeGroupIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterEKSNodeGroups, nodeGroup.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d node groups", len(m.filteredEKSNodeGroups), len(m.eksNodeGroups))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	if nodeGroup := m.currentEKSNodeGroup(); nodeGroup != nil {
		b.WriteString(titleStyle.Render("Selected Node Group"))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Scaling", fmt.Sprintf("desired:%d min:%d max:%d", nodeGroup.DesiredSize, nodeGroup.MinSize, nodeGroup.MaxSize)))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Version", firstNonEmpty(nodeGroup.Version, nodeGroup.ReleaseVersion, "-")))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Health", nodeGroup.HealthSummary()))
		b.WriteString("\n\n")
	}
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: detail • esc: back • q: feature list • H: home"))
	return b.String()
}

func (m Model) updateEKSNodeGroupDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := m.eksNodeGroupDetailLines()
	visibleLines := max(m.height-9, 5)
	maxOffset := max(len(lines)-visibleLines, 0)

	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenEKSNodeGroupList
	case "up", "k":
		if m.eksNodeGroupScroll > 0 {
			m.eksNodeGroupScroll--
		}
	case "down", "j":
		if m.eksNodeGroupScroll < maxOffset {
			m.eksNodeGroupScroll++
		}
	case "pgup":
		m.eksNodeGroupScroll -= visibleLines
		if m.eksNodeGroupScroll < 0 {
			m.eksNodeGroupScroll = 0
		}
	case "pgdown":
		m.eksNodeGroupScroll += visibleLines
		if m.eksNodeGroupScroll > maxOffset {
			m.eksNodeGroupScroll = maxOffset
		}
	}
	return m, nil
}

func (m Model) viewEKSNodeGroupDetail() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	title := "EKS Node Group Detail"
	if m.selectedEKSNodeGroup != nil {
		title = fmt.Sprintf("EKS Node Group Detail — %s", m.selectedEKSNodeGroup.Name)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	lines := m.eksNodeGroupDetailLines()
	visibleLines := max(m.height-9, 5)
	start := min(m.eksNodeGroupScroll, max(len(lines)-visibleLines, 0))
	end := min(start+visibleLines, len(lines))

	var panel strings.Builder
	for _, line := range lines[start:end] {
		panel.WriteString(line)
		panel.WriteString("\n")
	}
	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: scroll • pgup/pgdn: page • esc: back • q: feature list • H: home"))
	return b.String()
}

func (m Model) eksNodeGroupDetailLines() []string {
	if m.selectedEKSNodeGroup == nil {
		return []string{dimStyle.Render("No node group selected")}
	}
	nodeGroup := m.selectedEKSNodeGroup
	lines := []string{
		renderDetailLine("Cluster", nodeGroup.ClusterName),
		renderDetailLine("Status", nodeGroup.Status),
		renderDetailLine("Version", firstNonEmpty(nodeGroup.Version, "-")),
		renderDetailLine("Release", firstNonEmpty(nodeGroup.ReleaseVersion, "-")),
		renderDetailLine("AMI Type", firstNonEmpty(nodeGroup.AmiType, "-")),
		renderDetailLine("Capacity", firstNonEmpty(nodeGroup.CapacityType, "-")),
		renderDetailLine("Scaling", fmt.Sprintf("desired:%d min:%d max:%d", nodeGroup.DesiredSize, nodeGroup.MinSize, nodeGroup.MaxSize)),
		renderDetailLine("Instances", nodeGroup.InstanceTypesLabel()),
		renderDetailLine("ARN", nodeGroup.ARN),
		"",
		titleStyle.Render("Health"),
	}
	if len(nodeGroup.HealthIssues) == 0 {
		lines = append(lines, "  "+successStyle.Render("No active health issues"))
		return lines
	}
	for _, issue := range nodeGroup.HealthIssues {
		lines = append(lines, "  "+warningStyle.Render(issue.Summary()))
	}
	return lines
}

func (m Model) currentEKSCluster() *awsservice.EKSCluster {
	if len(m.filteredEKSClusters) == 0 || m.eksClusterIdx < 0 || m.eksClusterIdx >= len(m.filteredEKSClusters) {
		return nil
	}
	cluster := m.filteredEKSClusters[m.eksClusterIdx]
	return &cluster
}

func (m Model) currentEKSNodeGroup() *awsservice.EKSNodeGroup {
	if len(m.filteredEKSNodeGroups) == 0 || m.eksNodeGroupIdx < 0 || m.eksNodeGroupIdx >= len(m.filteredEKSNodeGroups) {
		return nil
	}
	nodeGroup := m.filteredEKSNodeGroups[m.eksNodeGroupIdx]
	return &nodeGroup
}

func (m Model) loadEKSClusters() tea.Cmd {
	return func() tea.Msg {
		cfg := m.cfg
		ctx, cancel := context.WithTimeout(context.Background(), eksAPITimeout)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), eksAPITimeout)
		defer cancel()

		repo, err := awsservice.NewAwsRepository(ctx, cfg)
		if err != nil {
			return errMsg{err}
		}
		clusters, err := repo.ListEKSClusters(ctx)
		if err != nil {
			return errMsg{err}
		}
		return eksClustersLoadedMsg{clusters: clusters}
	}
}

func (m Model) loadEKSNodeGroups() tea.Cmd {
	return func() tea.Msg {
		cfg := m.cfg
		cluster := m.selectedEKSCluster
		if cluster == nil {
			return errMsg{fmt.Errorf("no EKS cluster selected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), eksAPITimeout)
	if cluster == nil {
		return func() tea.Msg { return errMsg{fmt.Errorf("no EKS cluster selected")} }
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), eksAPITimeout)
		defer cancel()

		repo, err := awsservice.NewAwsRepository(ctx, cfg)
		if err != nil {
			return errMsg{err}
		}
		nodeGroups, err := repo.ListEKSNodeGroups(ctx, cluster.Name)
		if err != nil {
			return errMsg{err}
		}
		return eksNodeGroupsLoadedMsg{nodeGroups: nodeGroups}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
