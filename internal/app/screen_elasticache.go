package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/clipboard"
	awsservice "unic/internal/services/aws"
)

var elasticacheCopyFn = clipboard.Copy

type elasticacheModel struct {
	resources        []awsservice.ElastiCacheResource
	filtered         []awsservice.ElastiCacheResource
	resourceIdx      int
	selectedResource *awsservice.ElastiCacheResource
	nodeIdx          int
	selectedNode     *awsservice.ElastiCacheNode
	notice           string
}

func newElastiCacheModel() elasticacheModel {
	return elasticacheModel{}
}

func (em *elasticacheModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(em.loadResources(*m))
}

func (em *elasticacheModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case elasticacheResourcesLoadedMsg:
		em.resources = msg.resources
		em.filtered = applyFilter(em.resources, m.filterValue(filterElastiCacheResources))
		em.resourceIdx = 0
		em.selectedResource = nil
		em.selectedNode = nil
		em.notice = ""
		m.screen = screenElastiCacheResourceList
		return *m, nil, true
	}
	return *m, nil, false
}

func (em *elasticacheModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenElastiCacheResourceList:
		newM, cmd := em.updateResourceList(m, msg)
		return newM, cmd, true
	case screenElastiCacheNodeList:
		newM, cmd := em.updateNodeList(m, msg)
		return newM, cmd, true
	case screenElastiCacheNodeDetail:
		newM, cmd := em.updateNodeDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (em elasticacheModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenElastiCacheResourceList:
		return em.viewResourceList(m), true
	case screenElastiCacheNodeList:
		return em.viewNodeList(m), true
	case screenElastiCacheNodeDetail:
		return em.viewNodeDetail(m), true
	default:
		return "", false
	}
}

func (em *elasticacheModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterElastiCacheResources {
		return false
	}
	em.filtered = applyFilter(em.resources, m.filterValue(target))
	em.resourceIdx = 0
	return true
}

func (em *elasticacheModel) updateResourceList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterElastiCacheResources); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterElastiCacheResources)
	case "up", "k":
		em.resourceIdx = previousListIndex(em.resourceIdx, len(em.filtered))
	case "down", "j":
		em.resourceIdx = nextListIndex(em.resourceIdx, len(em.filtered))
	case "/":
		return *m, m.activateFilter(filterElastiCacheResources)
	case "r":
		return m.startLoading(em.loadResources(*m))
	case "enter":
		if em.resourceIdx < len(em.filtered) {
			selected := em.filtered[em.resourceIdx]
			em.selectedResource = &selected
			em.nodeIdx = 0
			em.selectedNode = nil
			em.notice = ""
			m.screen = screenElastiCacheNodeList
		}
	}
	return *m, nil
}

func (em *elasticacheModel) updateNodeList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenElastiCacheResourceList
	case "up", "k":
		if em.selectedResource != nil {
			em.nodeIdx = previousListIndex(em.nodeIdx, len(em.selectedResource.Nodes))
		}
	case "down", "j":
		if em.selectedResource != nil {
			em.nodeIdx = nextListIndex(em.nodeIdx, len(em.selectedResource.Nodes))
		}
	case "enter":
		if em.selectedResource != nil && em.nodeIdx < len(em.selectedResource.Nodes) {
			selected := em.selectedResource.Nodes[em.nodeIdx]
			em.selectedNode = &selected
			em.notice = ""
			m.screen = screenElastiCacheNodeDetail
		}
	}
	return *m, nil
}

func (em *elasticacheModel) updateNodeDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenElastiCacheNodeList
	case "c":
		if em.selectedNode == nil || em.selectedNode.Endpoint == "" {
			return *m, nil
		}
		if err := elasticacheCopyFn(em.selectedNode.Endpoint); err != nil {
			em.notice = fmt.Sprintf("Clipboard error: %v", err)
		} else {
			em.notice = "Copied endpoint to clipboard"
		}
	}
	return *m, nil
}

func (em elasticacheModel) loadResources(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		resources, err := repo.ListElastiCacheResources(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return elasticacheResourcesLoadedMsg{resources: resources}
	}
}

func (em elasticacheModel) viewResourceList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("ElastiCache Resources"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterElastiCacheResources))
	b.WriteString("\n\n")

	if len(em.filtered) == 0 {
		empty := "  No ElastiCache resources found"
		if len(em.resources) > 0 {
			empty = "  No matching ElastiCache resources"
		}
		panel.WriteString(dimStyle.Render(empty))
		panel.WriteString("\n")
	} else {
		panel.WriteString(dimStyle.Render("  " + elasticacheResourceRow("ID", "TYPE", "ENGINE", "STATUS", "NODES", "NODE TYPE")))
		panel.WriteString("\n")
		visibleLines := max(m.height-12, 5)
		start := 0
		if em.resourceIdx >= visibleLines {
			start = em.resourceIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filtered))
		for i := start; i < end; i++ {
			resource := em.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == em.resourceIdx {
				cursor = "> "
				style = selectedStyle
			}
			engine := strings.TrimSpace(resource.Engine + " " + resource.EngineVersion)
			row := elasticacheResourceRow(resource.ID, resource.Kind, engine, resource.Status, fmt.Sprintf("%d", len(resource.Nodes)), resource.NodeType)
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterElastiCacheResources, row)))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d resources", len(em.filtered), len(em.resources))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func elasticacheResourceRow(id, kind, engine, status, nodes, nodeType string) string {
	return fmt.Sprintf("%-28s  %-18s  %-18s  %-14s  %5s  %s",
		inspectorShorten(id, 28), inspectorShorten(kind, 18), inspectorShorten(engine, 18), inspectorShorten(status, 14), nodes, nodeType)
}

func (em elasticacheModel) viewNodeList(m Model) string {
	if em.selectedResource == nil {
		return ""
	}
	resource := em.selectedResource
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("ElastiCache Nodes"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %s: %s  endpoint: %s", resource.Kind, resource.ID, displayElastiCacheValue(resource.Endpoint))))
	b.WriteString("\n\n")

	if len(resource.Nodes) == 0 {
		panel.WriteString(dimStyle.Render("  No cache nodes reported"))
		panel.WriteString("\n")
	} else {
		panel.WriteString(dimStyle.Render("  " + elasticacheNodeRow("CLUSTER", "NODE", "SHARD", "ROLE", "STATUS", "AZ")))
		panel.WriteString("\n")
		visibleLines := max(m.height-11, 5)
		start := 0
		if em.nodeIdx >= visibleLines {
			start = em.nodeIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(resource.Nodes))
		for i := start; i < end; i++ {
			node := resource.Nodes[i]
			cursor := "  "
			style := normalStyle
			if i == em.nodeIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + elasticacheNodeRow(node.ClusterID, node.ID, node.ShardID, node.Role, node.Status, node.AZ)))
			panel.WriteString("\n")
		}
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func elasticacheNodeRow(cluster, node, shard, role, status, az string) string {
	return fmt.Sprintf("%-28s  %-7s  %-7s  %-9s  %-14s  %s",
		inspectorShorten(cluster, 28), inspectorShorten(node, 7), inspectorShorten(shard, 7), inspectorShorten(role, 9), inspectorShorten(status, 14), az)
}

func (em elasticacheModel) viewNodeDetail(m Model) string {
	if em.selectedNode == nil || em.selectedResource == nil {
		return ""
	}
	node := em.selectedNode
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("ElastiCache Node Detail"))
	b.WriteString("\n\n")
	b.WriteString(renderDetailLine("Resource", normalStyle.Render(em.selectedResource.ID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Cluster", normalStyle.Render(node.ClusterID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Node", normalStyle.Render(node.ID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Shard", normalStyle.Render(displayElastiCacheValue(node.ShardID))))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Role", normalStyle.Render(displayElastiCacheValue(node.Role))))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Status", normalStyle.Render(displayElastiCacheValue(node.Status))))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("AZ", normalStyle.Render(displayElastiCacheValue(node.AZ))))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Endpoint", normalStyle.Render(displayElastiCacheValue(node.Endpoint))))
	b.WriteString("\n")
	if em.notice != "" {
		b.WriteString("\n")
		b.WriteString(selectedStyle.Render("  " + em.notice))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func displayElastiCacheValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
