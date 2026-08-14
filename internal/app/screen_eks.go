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

type eksModel struct {
	clusters         []awsservice.EKSCluster
	filteredClusters []awsservice.EKSCluster
	clusterIdx       int
	selectedCluster  *awsservice.EKSCluster
	upgradeReadiness *awsservice.EKSUpgradeReadiness
	upgradeScroll    int
	accessCopyMsg    string

	nodeGroups         []awsservice.EKSNodeGroup
	filteredNodeGroups []awsservice.EKSNodeGroup
	nodeGroupIdx       int
	selectedNodeGroup  *awsservice.EKSNodeGroup
	nodeGroupScroll    int

	addons         []awsservice.EKSAddon
	filteredAddons []awsservice.EKSAddon
	addonIdx       int
	selectedAddon  *awsservice.EKSAddon
	addonScroll    int
}

func newEKSModel() eksModel {
	return eksModel{}
}

func (em *eksModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(em.loadClusters(*m))
}

func (em *eksModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case eksClustersLoadedMsg:
		em.clusters = msg.clusters
		em.filteredClusters = msg.clusters
		em.clusterIdx = 0
		em.selectedCluster = nil
		em.upgradeReadiness = nil
		em.upgradeScroll = 0
		em.accessCopyMsg = ""
		em.nodeGroups = nil
		em.filteredNodeGroups = nil
		em.selectedNodeGroup = nil
		em.nodeGroupScroll = 0
		em.addons = nil
		em.filteredAddons = nil
		em.selectedAddon = nil
		em.addonScroll = 0
		m.resetFilter(filterEKSClusters)
		m.screen = screenEKSClusterList
		return *m, nil, true
	case eksNodeGroupsLoadedMsg:
		em.nodeGroups = msg.nodeGroups
		em.filteredNodeGroups = msg.nodeGroups
		em.nodeGroupIdx = 0
		em.selectedNodeGroup = nil
		em.nodeGroupScroll = 0
		m.resetFilter(filterEKSNodeGroups)
		m.screen = screenEKSNodeGroupList
		return *m, nil, true
	case eksAddonsLoadedMsg:
		em.addons = msg.addons
		em.filteredAddons = msg.addons
		em.addonIdx = 0
		em.selectedAddon = nil
		em.addonScroll = 0
		m.resetFilter(filterEKSAddons)
		m.screen = screenEKSAddonList
		return *m, nil, true
	case eksUpgradeReadinessLoadedMsg:
		em.upgradeReadiness = msg.readiness
		em.upgradeScroll = 0
		m.screen = screenEKSUpgradeReadiness
		return *m, nil, true
	}
	return *m, nil, false
}

func (em *eksModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenEKSClusterList:
		newM, cmd := em.updateClusterList(m, msg)
		return newM, cmd, true
	case screenEKSUpgradeReadiness:
		newM, cmd := em.updateUpgradeReadiness(m, msg)
		return newM, cmd, true
	case screenEKSAccessHelper:
		newM, cmd := em.updateAccessHelper(m, msg)
		return newM, cmd, true
	case screenEKSNodeGroupList:
		newM, cmd := em.updateNodeGroupList(m, msg)
		return newM, cmd, true
	case screenEKSNodeGroupDetail:
		newM, cmd := em.updateNodeGroupDetail(m, msg)
		return newM, cmd, true
	case screenEKSAddonList:
		newM, cmd := em.updateAddonList(m, msg)
		return newM, cmd, true
	case screenEKSAddonDetail:
		newM, cmd := em.updateAddonDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (em eksModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenEKSClusterList:
		return em.viewClusterList(m), true
	case screenEKSUpgradeReadiness:
		return em.viewUpgradeReadiness(m), true
	case screenEKSAccessHelper:
		return em.viewAccessHelper(m), true
	case screenEKSNodeGroupList:
		return em.viewNodeGroupList(m), true
	case screenEKSNodeGroupDetail:
		return em.viewNodeGroupDetail(m), true
	case screenEKSAddonList:
		return em.viewAddonList(m), true
	case screenEKSAddonDetail:
		return em.viewAddonDetail(m), true
	default:
		return "", false
	}
}

func (em *eksModel) ApplyFilter(m *Model, target filterTarget) bool {
	switch target {
	case filterEKSClusters:
		em.filteredClusters = applyFilter(em.clusters, m.filterValue(target))
		em.clusterIdx = 0
	case filterEKSNodeGroups:
		em.filteredNodeGroups = applyFilter(em.nodeGroups, m.filterValue(target))
		em.nodeGroupIdx = 0
	case filterEKSAddons:
		em.filteredAddons = applyFilter(em.addons, m.filterValue(target))
		em.addonIdx = 0
	default:
		return false
	}
	return true
}

func (em *eksModel) updateClusterList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterEKSClusters); handled {
		return *m, cmd
	}

	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
	case "up", "k":
		em.clusterIdx = previousListIndex(em.clusterIdx, len(em.filteredClusters))
	case "down", "j":
		em.clusterIdx = nextListIndex(em.clusterIdx, len(em.filteredClusters))
	case "/":
		return *m, m.activateFilter(filterEKSClusters)
	case "r":
		return m.startLoading(em.loadClusters(*m))
	case "a":
		if len(em.filteredClusters) > 0 && em.clusterIdx < len(em.filteredClusters) {
			cluster := em.filteredClusters[em.clusterIdx]
			em.selectedCluster = &cluster
			return m.startLoading(em.loadAddons(*m))
		}
	case "U":
		if len(em.filteredClusters) > 0 && em.clusterIdx < len(em.filteredClusters) {
			cluster := em.filteredClusters[em.clusterIdx]
			em.selectedCluster = &cluster
			return m.startLoading(em.loadUpgradeReadiness(*m))
		}
	case "u":
		if len(em.filteredClusters) > 0 && em.clusterIdx < len(em.filteredClusters) {
			cluster := em.filteredClusters[em.clusterIdx]
			em.selectedCluster = &cluster
			em.accessCopyMsg = ""
			m.screen = screenEKSAccessHelper
		}
	case "enter":
		if len(em.filteredClusters) > 0 && em.clusterIdx < len(em.filteredClusters) {
			cluster := em.filteredClusters[em.clusterIdx]
			em.selectedCluster = &cluster
			return m.startLoading(em.loadNodeGroups(*m))
		}
	}
	return *m, nil
}

func (em eksModel) viewClusterList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("EKS Clusters"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterEKSClusters))
	b.WriteString("\n\n")

	if len(em.filteredClusters) == 0 {
		panel.WriteString(dimStyle.Render("  No EKS clusters found"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-16, 5)
		start := 0
		if em.clusterIdx >= visibleLines {
			start = em.clusterIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filteredClusters))
		for i := start; i < end; i++ {
			cluster := em.filteredClusters[i]
			cursor := "  "
			style := normalStyle
			if i == em.clusterIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterEKSClusters, cluster.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d clusters", len(em.filteredClusters), len(em.clusters))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	if cluster := em.currentCluster(); cluster != nil {
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

func (em *eksModel) updateUpgradeReadiness(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := em.upgradeReadinessLines()
	visibleLines := max(m.height-9, 5)
	maxOffset := max(len(lines)-visibleLines, 0)

	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenEKSClusterList
	case "r":
		return m.startLoading(em.loadUpgradeReadiness(*m))
	case "up", "k":
		if em.upgradeScroll > 0 {
			em.upgradeScroll--
		}
	case "down", "j":
		if em.upgradeScroll < maxOffset {
			em.upgradeScroll++
		}
	case "pgup":
		em.upgradeScroll -= visibleLines
		if em.upgradeScroll < 0 {
			em.upgradeScroll = 0
		}
	case "pgdown":
		em.upgradeScroll += visibleLines
		if em.upgradeScroll > maxOffset {
			em.upgradeScroll = maxOffset
		}
	}
	return *m, nil
}

func (em eksModel) viewUpgradeReadiness(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	title := "EKS Upgrade Readiness"
	if em.upgradeReadiness != nil {
		title = fmt.Sprintf("EKS Upgrade Readiness — %s", em.upgradeReadiness.ClusterName)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	lines := em.upgradeReadinessLines()
	visibleLines := max(m.height-9, 5)
	start := min(em.upgradeScroll, max(len(lines)-visibleLines, 0))
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

func (em *eksModel) updateAccessHelper(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		em.accessCopyMsg = ""
		m.screen = screenEKSClusterList
	case "c":
		if err := clipboard.Copy(em.updateKubeconfigCommand(*m)); err != nil {
			em.accessCopyMsg = fmt.Sprintf("Clipboard error: %s", err)
		} else {
			em.accessCopyMsg = "Copied update-kubeconfig command"
		}
	case "k":
		if err := clipboard.Copy(awsservice.BuildEKSKubectlSmokeCommand()); err != nil {
			em.accessCopyMsg = fmt.Sprintf("Clipboard error: %s", err)
		} else {
			em.accessCopyMsg = "Copied kubectl command"
		}
	}
	return *m, nil
}

func (em eksModel) upgradeReadinessLines() []string {
	if em.upgradeReadiness == nil {
		return []string{dimStyle.Render("No readiness data loaded")}
	}
	readiness := em.upgradeReadiness
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

func (em eksModel) viewAccessHelper(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	title := "EKS Access Helper"
	if em.selectedCluster != nil {
		title = fmt.Sprintf("EKS Access Helper — %s", em.selectedCluster.Name)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	cluster := em.selectedCluster
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
	panel.WriteString("  [c] " + em.updateKubeconfigCommand(m))
	panel.WriteString("\n")
	panel.WriteString("  [k] " + awsservice.BuildEKSKubectlSmokeCommand())
	panel.WriteString("\n")
	if em.accessCopyMsg != "" {
		panel.WriteString("\n")
		panel.WriteString(selectedStyle.Render("  " + em.accessCopyMsg))
		panel.WriteString("\n")
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("c: copy kubeconfig • k: copy kubectl • esc: clusters • q: feature list • H: home"))
	return b.String()
}

func (em eksModel) updateKubeconfigCommand(m Model) string {
	if em.selectedCluster == nil {
		return ""
	}
	alias := em.selectedCluster.Name
	if strings.TrimSpace(m.cfg.ContextName) != "" {
		alias = m.cfg.ContextName + "-" + em.selectedCluster.Name
	}
	return awsservice.BuildEKSUpdateKubeconfigCommand(em.selectedCluster.Name, m.cfg.Region, m.cfg.Profile, alias)
}

func (em *eksModel) updateNodeGroupList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterEKSNodeGroups); handled {
		return *m, cmd
	}

	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenEKSClusterList
	case "up", "k":
		em.nodeGroupIdx = previousListIndex(em.nodeGroupIdx, len(em.filteredNodeGroups))
	case "down", "j":
		em.nodeGroupIdx = nextListIndex(em.nodeGroupIdx, len(em.filteredNodeGroups))
	case "/":
		return *m, m.activateFilter(filterEKSNodeGroups)
	case "r":
		return m.startLoading(em.loadNodeGroups(*m))
	case "enter":
		if len(em.filteredNodeGroups) > 0 && em.nodeGroupIdx < len(em.filteredNodeGroups) {
			nodeGroup := em.filteredNodeGroups[em.nodeGroupIdx]
			em.selectedNodeGroup = &nodeGroup
			em.nodeGroupScroll = 0
			m.screen = screenEKSNodeGroupDetail
		}
	}
	return *m, nil
}

func (em eksModel) viewNodeGroupList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	clusterName := ""
	if em.selectedCluster != nil {
		clusterName = em.selectedCluster.Name
	}
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(fmt.Sprintf("EKS Node Groups — %s", clusterName)))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterEKSNodeGroups))
	b.WriteString("\n\n")

	if len(em.filteredNodeGroups) == 0 {
		panel.WriteString(dimStyle.Render("  No managed node groups found"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-17, 5)
		start := 0
		if em.nodeGroupIdx >= visibleLines {
			start = em.nodeGroupIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filteredNodeGroups))
		for i := start; i < end; i++ {
			nodeGroup := em.filteredNodeGroups[i]
			cursor := "  "
			style := normalStyle
			if i == em.nodeGroupIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterEKSNodeGroups, nodeGroup.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d node groups", len(em.filteredNodeGroups), len(em.nodeGroups))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	if nodeGroup := em.currentNodeGroup(); nodeGroup != nil {
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

func (em *eksModel) updateNodeGroupDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := em.nodeGroupDetailLines()
	visibleLines := max(m.height-9, 5)
	maxOffset := max(len(lines)-visibleLines, 0)

	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenEKSNodeGroupList
	case "up", "k":
		if em.nodeGroupScroll > 0 {
			em.nodeGroupScroll--
		}
	case "down", "j":
		if em.nodeGroupScroll < maxOffset {
			em.nodeGroupScroll++
		}
	case "pgup":
		em.nodeGroupScroll -= visibleLines
		if em.nodeGroupScroll < 0 {
			em.nodeGroupScroll = 0
		}
	case "pgdown":
		em.nodeGroupScroll += visibleLines
		if em.nodeGroupScroll > maxOffset {
			em.nodeGroupScroll = maxOffset
		}
	}
	return *m, nil
}

func (em eksModel) viewNodeGroupDetail(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	title := "EKS Node Group Detail"
	if em.selectedNodeGroup != nil {
		title = fmt.Sprintf("EKS Node Group Detail — %s", em.selectedNodeGroup.Name)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	lines := em.nodeGroupDetailLines()
	visibleLines := max(m.height-9, 5)
	start := min(em.nodeGroupScroll, max(len(lines)-visibleLines, 0))
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

func (em *eksModel) updateAddonList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterEKSAddons); handled {
		return *m, cmd
	}

	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenEKSClusterList
	case "up", "k":
		em.addonIdx = previousListIndex(em.addonIdx, len(em.filteredAddons))
	case "down", "j":
		em.addonIdx = nextListIndex(em.addonIdx, len(em.filteredAddons))
	case "/":
		return *m, m.activateFilter(filterEKSAddons)
	case "r":
		return m.startLoading(em.loadAddons(*m))
	case "enter":
		if len(em.filteredAddons) > 0 && em.addonIdx < len(em.filteredAddons) {
			addon := em.filteredAddons[em.addonIdx]
			em.selectedAddon = &addon
			em.addonScroll = 0
			m.screen = screenEKSAddonDetail
		}
	}
	return *m, nil
}

func (em eksModel) viewAddonList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	clusterName := ""
	if em.selectedCluster != nil {
		clusterName = em.selectedCluster.Name
	}
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(fmt.Sprintf("EKS Add-ons — %s", clusterName)))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterEKSAddons))
	b.WriteString("\n\n")

	if len(em.filteredAddons) == 0 {
		panel.WriteString(dimStyle.Render("  No managed add-ons found"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-17, 5)
		start := 0
		if em.addonIdx >= visibleLines {
			start = em.addonIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filteredAddons))
		for i := start; i < end; i++ {
			addon := em.filteredAddons[i]
			cursor := "  "
			style := normalStyle
			if addon.NeedsAttention() {
				style = warningStyle
			}
			if i == em.addonIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterEKSAddons, addon.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d add-ons", len(em.filteredAddons), len(em.addons))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	if addon := em.currentAddon(); addon != nil {
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

func (em *eksModel) updateAddonDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := em.addonDetailLines()
	visibleLines := max(m.height-9, 5)
	maxOffset := max(len(lines)-visibleLines, 0)

	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenEKSAddonList
	case "up", "k":
		if em.addonScroll > 0 {
			em.addonScroll--
		}
	case "down", "j":
		if em.addonScroll < maxOffset {
			em.addonScroll++
		}
	case "pgup":
		em.addonScroll -= visibleLines
		if em.addonScroll < 0 {
			em.addonScroll = 0
		}
	case "pgdown":
		em.addonScroll += visibleLines
		if em.addonScroll > maxOffset {
			em.addonScroll = maxOffset
		}
	}
	return *m, nil
}

func (em eksModel) viewAddonDetail(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	title := "EKS Add-on Detail"
	if em.selectedAddon != nil {
		title = fmt.Sprintf("EKS Add-on Detail — %s", em.selectedAddon.Name)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	lines := em.addonDetailLines()
	visibleLines := max(m.height-9, 5)
	start := min(em.addonScroll, max(len(lines)-visibleLines, 0))
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

func (em eksModel) addonDetailLines() []string {
	if em.selectedAddon == nil {
		return []string{dimStyle.Render("No add-on selected")}
	}
	addon := em.selectedAddon
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

func (em eksModel) nodeGroupDetailLines() []string {
	if em.selectedNodeGroup == nil {
		return []string{dimStyle.Render("No node group selected")}
	}
	nodeGroup := em.selectedNodeGroup
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

func (em eksModel) currentCluster() *awsservice.EKSCluster {
	if len(em.filteredClusters) == 0 || em.clusterIdx < 0 || em.clusterIdx >= len(em.filteredClusters) {
		return nil
	}
	cluster := em.filteredClusters[em.clusterIdx]
	return &cluster
}

func (em eksModel) currentNodeGroup() *awsservice.EKSNodeGroup {
	if len(em.filteredNodeGroups) == 0 || em.nodeGroupIdx < 0 || em.nodeGroupIdx >= len(em.filteredNodeGroups) {
		return nil
	}
	nodeGroup := em.filteredNodeGroups[em.nodeGroupIdx]
	return &nodeGroup
}

func (em eksModel) currentAddon() *awsservice.EKSAddon {
	if len(em.filteredAddons) == 0 || em.addonIdx < 0 || em.addonIdx >= len(em.filteredAddons) {
		return nil
	}
	addon := em.filteredAddons[em.addonIdx]
	return &addon
}

func (em *eksModel) loadClusters(m Model) tea.Cmd {
	return func() tea.Msg {
		cfg := m.cfg
		ctx, cancel := context.WithTimeout(m.commandContext(), eksAPITimeout)
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

func (em *eksModel) loadNodeGroups(m Model) tea.Cmd {
	return func() tea.Msg {
		cfg := m.cfg
		cluster := em.selectedCluster
		if cluster == nil {
			return errMsg{fmt.Errorf("no EKS cluster selected")}
		}
		ctx, cancel := context.WithTimeout(m.commandContext(), eksAPITimeout)
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

func (em *eksModel) loadAddons(m Model) tea.Cmd {
	return func() tea.Msg {
		cfg := m.cfg
		cluster := em.selectedCluster
		if cluster == nil {
			return errMsg{fmt.Errorf("no EKS cluster selected")}
		}
		ctx, cancel := context.WithTimeout(m.commandContext(), eksAPITimeout)
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

func (em *eksModel) loadUpgradeReadiness(m Model) tea.Cmd {
	return func() tea.Msg {
		cfg := m.cfg
		cluster := em.selectedCluster
		if cluster == nil {
			return errMsg{fmt.Errorf("no EKS cluster selected")}
		}
		ctx, cancel := context.WithTimeout(m.commandContext(), eksAPITimeout)
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
