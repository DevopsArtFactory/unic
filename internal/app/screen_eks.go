package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/clipboard"
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
		m.eksUpgradeReadiness = nil
		m.eksUpgradeScroll = 0
		m.eksAccessCopyMsg = ""
		m.eksNodeGroups = nil
		m.filteredEKSNodeGroups = nil
		m.selectedEKSNodeGroup = nil
		m.eksNodeGroupScroll = 0
		m.eksAddons = nil
		m.filteredEKSAddons = nil
		m.selectedEKSAddon = nil
		m.eksAddonScroll = 0
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
	case eksAddonsLoadedMsg:
		m.eksAddons = msg.addons
		m.filteredEKSAddons = msg.addons
		m.eksAddonIdx = 0
		m.selectedEKSAddon = nil
		m.eksAddonScroll = 0
		m.resetFilter(filterEKSAddons)
		m.screen = screenEKSAddonList
		return m, nil, true
	case eksUpgradeReadinessLoadedMsg:
		m.eksUpgradeReadiness = msg.readiness
		m.eksUpgradeScroll = 0
		m.screen = screenEKSUpgradeReadiness
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
	case "a":
		if len(m.filteredEKSClusters) > 0 && m.eksClusterIdx < len(m.filteredEKSClusters) {
			cluster := m.filteredEKSClusters[m.eksClusterIdx]
			m.selectedEKSCluster = &cluster
			return m.startLoading(m.loadEKSAddons())
		}
	case "U":
		if len(m.filteredEKSClusters) > 0 && m.eksClusterIdx < len(m.filteredEKSClusters) {
			cluster := m.filteredEKSClusters[m.eksClusterIdx]
			m.selectedEKSCluster = &cluster
			return m.startLoading(m.loadEKSUpgradeReadiness())
		}
	case "u":
		if len(m.filteredEKSClusters) > 0 && m.eksClusterIdx < len(m.filteredEKSClusters) {
			cluster := m.filteredEKSClusters[m.eksClusterIdx]
			m.selectedEKSCluster = &cluster
			m.eksAccessCopyMsg = ""
			m.screen = screenEKSAccessHelper
		}
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
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: node groups • a: add-ons • U: readiness • u: access helper • r: refresh • esc: back • H: home"))
	return b.String()
}

func (m Model) updateEKSUpgradeReadiness(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := m.eksUpgradeReadinessLines()
	visibleLines := max(m.height-9, 5)
	maxOffset := max(len(lines)-visibleLines, 0)

	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenEKSClusterList
	case "r":
		return m.startLoading(m.loadEKSUpgradeReadiness())
	case "up", "k":
		if m.eksUpgradeScroll > 0 {
			m.eksUpgradeScroll--
		}
	case "down", "j":
		if m.eksUpgradeScroll < maxOffset {
			m.eksUpgradeScroll++
		}
	case "pgup":
		m.eksUpgradeScroll -= visibleLines
		if m.eksUpgradeScroll < 0 {
			m.eksUpgradeScroll = 0
		}
	case "pgdown":
		m.eksUpgradeScroll += visibleLines
		if m.eksUpgradeScroll > maxOffset {
			m.eksUpgradeScroll = maxOffset
		}
	}
	return m, nil
}

func (m Model) viewEKSUpgradeReadiness() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	title := "EKS Upgrade Readiness"
	if m.eksUpgradeReadiness != nil {
		title = fmt.Sprintf("EKS Upgrade Readiness — %s", m.eksUpgradeReadiness.ClusterName)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	lines := m.eksUpgradeReadinessLines()
	visibleLines := max(m.height-9, 5)
	start := min(m.eksUpgradeScroll, max(len(lines)-visibleLines, 0))
	end := min(start+visibleLines, len(lines))

	var panel strings.Builder
	for _, line := range lines[start:end] {
		panel.WriteString(line)
		panel.WriteString("\n")
	}
	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: scroll • pgup/pgdn: page • r: refresh • esc: clusters • q: feature list • H: home"))
	return b.String()
}

func (m Model) updateEKSAccessHelper(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.eksAccessCopyMsg = ""
		m.screen = screenEKSClusterList
	case "c":
		if err := clipboard.Copy(m.eksUpdateKubeconfigCommand()); err != nil {
			m.eksAccessCopyMsg = fmt.Sprintf("Clipboard error: %s", err)
		} else {
			m.eksAccessCopyMsg = "Copied update-kubeconfig command"
		}
	case "k":
		if err := clipboard.Copy(awsservice.BuildEKSKubectlSmokeCommand()); err != nil {
			m.eksAccessCopyMsg = fmt.Sprintf("Clipboard error: %s", err)
		} else {
			m.eksAccessCopyMsg = "Copied kubectl command"
		}
	}
	return m, nil
}

func (m Model) eksUpgradeReadinessLines() []string {
	if m.eksUpgradeReadiness == nil {
		return []string{dimStyle.Render("No readiness data loaded")}
	}
	readiness := m.eksUpgradeReadiness
	summary := readiness.Summary()
	if readiness.HasBlockers() {
		summary = warningStyle.Render(summary)
	} else if summary == "ready" {
		summary = successStyle.Render(summary)
	} else {
		summary = warningStyle.Render(summary)
	}
	lines := []string{
		renderDetailLine("Cluster", readiness.ClusterName),
		renderDetailLine("Control Plane", firstNonEmpty(readiness.ClusterVersion, "-")),
		renderDetailLine("Check", "current version alignment"),
		renderDetailLine("Summary", summary),
		"",
		titleStyle.Render("Node Groups"),
	}
	if len(readiness.NodeGroups) == 0 {
		lines = append(lines, "  "+dimStyle.Render("No managed node groups found"))
	} else {
		for _, nodeGroup := range readiness.NodeGroups {
			line := fmt.Sprintf("  %s  version:%s  status:%s", nodeGroup.Name, firstNonEmpty(nodeGroup.Version, "-"), firstNonEmpty(nodeGroup.Status, "-"))
			if nodeGroup.Version != readiness.ClusterVersion {
				line = warningStyle.Render(line)
			}
			lines = append(lines, line)
		}
	}
	lines = append(lines, "", titleStyle.Render("Add-ons"))
	if len(readiness.Addons) == 0 {
		lines = append(lines, "  "+dimStyle.Render("No managed add-ons found"))
	} else {
		for _, addon := range readiness.Addons {
			lines = append(lines, fmt.Sprintf("  %s  version:%s  status:%s", addon.Name, firstNonEmpty(addon.Version, "-"), firstNonEmpty(addon.Status, "-")))
		}
	}
	lines = append(lines, "", titleStyle.Render("Upgrade Insights"))
	if len(readiness.Insights) == 0 {
		lines = append(lines, "  "+dimStyle.Render("No EKS upgrade insights returned"))
	} else {
		for _, insight := range readiness.Insights {
			line := fmt.Sprintf("  %s  status:%s  version:%s", firstNonEmpty(insight.Name, insight.ID), firstNonEmpty(insight.Status, "-"), firstNonEmpty(insight.KubernetesVersion, "-"))
			if !strings.EqualFold(insight.Status, "PASSING") {
				line = warningStyle.Render(line)
			}
			lines = append(lines, line)
		}
	}
	lines = append(lines, "", titleStyle.Render("Findings"))
	if len(readiness.Findings) == 0 {
		lines = append(lines, "  "+successStyle.Render("No version alignment blockers found"))
		return lines
	}
	for _, finding := range readiness.Findings {
		line := fmt.Sprintf("  [%s] %s", strings.ToUpper(finding.Severity), finding.Summary())
		if strings.EqualFold(finding.Severity, "blocker") || strings.EqualFold(finding.Severity, "warning") {
			line = warningStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

func (m Model) viewEKSAccessHelper() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	title := "EKS Access Helper"
	if m.selectedEKSCluster != nil {
		title = fmt.Sprintf("EKS Access Helper — %s", m.selectedEKSCluster.Name)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	cluster := m.selectedEKSCluster
	if cluster == nil {
		b.WriteString(dimStyle.Render("No EKS cluster selected"))
		b.WriteString("\n\n")
		b.WriteString(m.renderHelpBar("esc: clusters • H: home"))
		return b.String()
	}

	var panel strings.Builder
	panel.WriteString(renderDetailLine("Cluster", cluster.Name))
	panel.WriteString("\n")
	panel.WriteString(renderDetailLine("Region", firstNonEmpty(m.cfg.Region, "-")))
	panel.WriteString("\n")
	panel.WriteString(renderDetailLine("Profile", firstNonEmpty(m.cfg.Profile, "-")))
	panel.WriteString("\n")
	panel.WriteString(renderDetailLine("Context", firstNonEmpty(m.cfg.ContextName, "-")))
	panel.WriteString("\n")
	panel.WriteString(renderDetailLine("Endpoint", firstNonEmpty(cluster.Endpoint, "-")))
	panel.WriteString("\n")
	panel.WriteString(renderDetailLine("ARN", cluster.ARN))
	panel.WriteString("\n\n")
	panel.WriteString(titleStyle.Render("Commands"))
	panel.WriteString("\n")
	panel.WriteString("  [c] " + m.eksUpdateKubeconfigCommand())
	panel.WriteString("\n")
	panel.WriteString("  [k] " + awsservice.BuildEKSKubectlSmokeCommand())
	panel.WriteString("\n")
	if m.eksAccessCopyMsg != "" {
		panel.WriteString("\n")
		panel.WriteString(selectedStyle.Render("  " + m.eksAccessCopyMsg))
		panel.WriteString("\n")
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("c: copy kubeconfig • k: copy kubectl • esc: clusters • q: feature list • H: home"))
	return b.String()
}

func (m Model) eksUpdateKubeconfigCommand() string {
	if m.selectedEKSCluster == nil {
		return ""
	}
	alias := m.selectedEKSCluster.Name
	if strings.TrimSpace(m.cfg.ContextName) != "" {
		alias = m.cfg.ContextName + "-" + m.selectedEKSCluster.Name
	}
	return awsservice.BuildEKSUpdateKubeconfigCommand(m.selectedEKSCluster.Name, m.cfg.Region, m.cfg.Profile, alias)
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

func (m Model) updateEKSAddonList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterEKSAddons); handled {
		return m, cmd
	}

	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenEKSClusterList
	case "up", "k":
		m.eksAddonIdx = previousListIndex(m.eksAddonIdx, len(m.filteredEKSAddons))
	case "down", "j":
		m.eksAddonIdx = nextListIndex(m.eksAddonIdx, len(m.filteredEKSAddons))
	case "/":
		return m, m.activateFilter(filterEKSAddons)
	case "r":
		return m.startLoading(m.loadEKSAddons())
	case "enter":
		if len(m.filteredEKSAddons) > 0 && m.eksAddonIdx < len(m.filteredEKSAddons) {
			addon := m.filteredEKSAddons[m.eksAddonIdx]
			m.selectedEKSAddon = &addon
			m.eksAddonScroll = 0
			m.screen = screenEKSAddonDetail
		}
	}
	return m, nil
}

func (m Model) viewEKSAddonList() string {
	var b strings.Builder
	var panel strings.Builder
	clusterName := ""
	if m.selectedEKSCluster != nil {
		clusterName = m.selectedEKSCluster.Name
	}
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(fmt.Sprintf("EKS Add-ons — %s", clusterName)))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterEKSAddons))
	b.WriteString("\n\n")

	if len(m.filteredEKSAddons) == 0 {
		panel.WriteString(dimStyle.Render("  No managed add-ons found"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-17, 5)
		start := 0
		if m.eksAddonIdx >= visibleLines {
			start = m.eksAddonIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredEKSAddons))
		for i := start; i < end; i++ {
			addon := m.filteredEKSAddons[i]
			cursor := "  "
			style := normalStyle
			if addon.NeedsAttention() {
				style = warningStyle
			}
			if i == m.eksAddonIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterEKSAddons, addon.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d add-ons", len(m.filteredEKSAddons), len(m.eksAddons))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	if addon := m.currentEKSAddon(); addon != nil {
		status := addon.StatusSummary()
		if addon.NeedsAttention() {
			status = warningStyle.Render(status)
		} else {
			status = successStyle.Render(status)
		}
		b.WriteString(titleStyle.Render("Selected Add-on"))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Version", firstNonEmpty(addon.Version, "-")))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Status", firstNonEmpty(addon.Status, "-")))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Health", status))
		b.WriteString("\n\n")
	}
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: detail • esc: clusters • q: feature list • H: home"))
	return b.String()
}

func (m Model) updateEKSAddonDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := m.eksAddonDetailLines()
	visibleLines := max(m.height-9, 5)
	maxOffset := max(len(lines)-visibleLines, 0)

	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenEKSAddonList
	case "up", "k":
		if m.eksAddonScroll > 0 {
			m.eksAddonScroll--
		}
	case "down", "j":
		if m.eksAddonScroll < maxOffset {
			m.eksAddonScroll++
		}
	case "pgup":
		m.eksAddonScroll -= visibleLines
		if m.eksAddonScroll < 0 {
			m.eksAddonScroll = 0
		}
	case "pgdown":
		m.eksAddonScroll += visibleLines
		if m.eksAddonScroll > maxOffset {
			m.eksAddonScroll = maxOffset
		}
	}
	return m, nil
}

func (m Model) viewEKSAddonDetail() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	title := "EKS Add-on Detail"
	if m.selectedEKSAddon != nil {
		title = fmt.Sprintf("EKS Add-on Detail — %s", m.selectedEKSAddon.Name)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	lines := m.eksAddonDetailLines()
	visibleLines := max(m.height-9, 5)
	start := min(m.eksAddonScroll, max(len(lines)-visibleLines, 0))
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

func (m Model) eksAddonDetailLines() []string {
	if m.selectedEKSAddon == nil {
		return []string{dimStyle.Render("No add-on selected")}
	}
	addon := m.selectedEKSAddon
	status := addon.StatusSummary()
	if addon.NeedsAttention() {
		status = warningStyle.Render(status)
	} else {
		status = successStyle.Render(status)
	}
	lines := []string{
		renderDetailLine("Cluster", addon.ClusterName),
		renderDetailLine("Status", firstNonEmpty(addon.Status, "-")),
		renderDetailLine("Version", firstNonEmpty(addon.Version, "-")),
		renderDetailLine("Health", status),
		renderDetailLine("Owner", firstNonEmpty(addon.Owner, "-")),
		renderDetailLine("Publisher", firstNonEmpty(addon.Publisher, "-")),
		renderDetailLine("Role ARN", firstNonEmpty(addon.ServiceAccountRoleARN, "-")),
		renderDetailLine("ARN", firstNonEmpty(addon.ARN, "-")),
		"",
		titleStyle.Render("Health Issues"),
	}
	if len(addon.HealthIssues) == 0 {
		lines = append(lines, "  "+successStyle.Render("No active health issues"))
		return lines
	}
	for _, issue := range addon.HealthIssues {
		lines = append(lines, "  "+warningStyle.Render(issue.Summary()))
	}
	return lines
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

func (m Model) currentEKSAddon() *awsservice.EKSAddon {
	if len(m.filteredEKSAddons) == 0 || m.eksAddonIdx < 0 || m.eksAddonIdx >= len(m.filteredEKSAddons) {
		return nil
	}
	addon := m.filteredEKSAddons[m.eksAddonIdx]
	return &addon
}

func (m Model) loadEKSClusters() tea.Cmd {
	return func() tea.Msg {
		cfg := m.cfg
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

func (m Model) loadEKSAddons() tea.Cmd {
	return func() tea.Msg {
		cfg := m.cfg
		cluster := m.selectedEKSCluster
		if cluster == nil {
			return errMsg{fmt.Errorf("no EKS cluster selected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), eksAPITimeout)
		defer cancel()

		repo, err := awsservice.NewAwsRepository(ctx, cfg)
		if err != nil {
			return errMsg{err}
		}
		addons, err := repo.ListEKSAddons(ctx, cluster.Name)
		if err != nil {
			return errMsg{err}
		}
		return eksAddonsLoadedMsg{addons: addons}
	}
}

func (m Model) loadEKSUpgradeReadiness() tea.Cmd {
	return func() tea.Msg {
		cfg := m.cfg
		cluster := m.selectedEKSCluster
		if cluster == nil {
			return errMsg{fmt.Errorf("no EKS cluster selected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), eksAPITimeout)
		defer cancel()

		repo, err := awsservice.NewAwsRepository(ctx, cfg)
		if err != nil {
			return errMsg{err}
		}
		readiness, err := repo.ListEKSUpgradeReadiness(ctx, *cluster)
		if err != nil {
			return errMsg{err}
		}
		return eksUpgradeReadinessLoadedMsg{readiness: readiness}
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
