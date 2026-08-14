package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

// The load balancer browser is a target-health-first triage view: LB list →
// target groups with aggregated health (unhealthiest first) → per-target
// health with reason codes. Unhealthy targets are two keystrokes from the
// LB list.

type elbModel struct {
	balancers      []awsservice.ELBLoadBalancer
	filteredLBs    []awsservice.ELBLoadBalancer
	lbIdx          int
	selectedLB     *awsservice.ELBLoadBalancer
	groups         []awsservice.ELBTargetGroupHealth
	filteredGroups []awsservice.ELBTargetGroupHealth
	groupIdx       int
	selectedGroup  *awsservice.ELBTargetGroupHealth
	targetIdx      int
	allRegions     bool
	regionErrors   []awsservice.RegionError
}

func newELBModel() elbModel {
	return elbModel{}
}

func (em *elbModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(em.loadBalancers(*m))
}

func (em *elbModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case elbLoadBalancersLoadedMsg:
		em.balancers = msg.balancers
		em.regionErrors = msg.regionErrors
		em.filteredLBs = applyFilter(em.balancers, m.filterValue(filterELBs))
		em.lbIdx = 0
		em.selectedLB = nil
		m.screen = screenELBList
		return *m, nil, true
	case elbTargetGroupsLoadedMsg:
		if em.selectedLB == nil || em.selectedLB.ARN != msg.loadBalancerARN {
			return *m, nil, true
		}
		em.groups = msg.groups
		em.filteredGroups = applyFilter(em.groups, m.filterValue(filterELBTargetGroups))
		em.groupIdx = 0
		em.selectedGroup = nil
		m.screen = screenELBTargetGroupList
		return *m, nil, true
	}
	return *m, nil, false
}

func (em *elbModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenELBList:
		newM, cmd := em.updateLBList(m, msg)
		return newM, cmd, true
	case screenELBTargetGroupList:
		newM, cmd := em.updateGroupList(m, msg)
		return newM, cmd, true
	case screenELBTargetList:
		newM, cmd := em.updateTargetList(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (em elbModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenELBList:
		return em.viewLBList(m), true
	case screenELBTargetGroupList:
		return em.viewGroupList(m), true
	case screenELBTargetList:
		return em.viewTargetList(m), true
	default:
		return "", false
	}
}

func (em *elbModel) ApplyFilter(m *Model, target filterTarget) bool {
	switch target {
	case filterELBs:
		em.filteredLBs = applyFilter(em.balancers, m.filterValue(target))
		em.lbIdx = 0
		return true
	case filterELBTargetGroups:
		em.filteredGroups = applyFilter(em.groups, m.filterValue(target))
		em.groupIdx = 0
		return true
	}
	return false
}

func (em *elbModel) updateLBList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterELBs); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterELBs)
	case "up", "k":
		em.lbIdx = previousListIndex(em.lbIdx, len(em.filteredLBs))
	case "down", "j":
		em.lbIdx = nextListIndex(em.lbIdx, len(em.filteredLBs))
	case "/":
		return *m, m.activateFilter(filterELBs)
	case "A":
		if m.hasMultipleRegions() {
			em.allRegions = !em.allRegions
			m.resetFilter(filterELBs)
			return m.startLoading(em.loadBalancers(*m))
		}
	case "r":
		m.resetFilter(filterELBs)
		return m.startLoading(em.loadBalancers(*m))
	case "enter":
		if len(em.filteredLBs) > 0 && em.lbIdx < len(em.filteredLBs) {
			selected := em.filteredLBs[em.lbIdx]
			em.selectedLB = &selected
			return m.startLoadingWithMessage("Loading target health...", []string{selected.Name}, em.loadGroups(*m, selected))
		}
	}
	return *m, nil
}

func (em *elbModel) updateGroupList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterELBTargetGroups); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
		m.resetFilter(filterELBTargetGroups)
	case "esc":
		m.screen = screenELBList
		m.resetFilter(filterELBTargetGroups)
	case "up", "k":
		em.groupIdx = previousListIndex(em.groupIdx, len(em.filteredGroups))
	case "down", "j":
		em.groupIdx = nextListIndex(em.groupIdx, len(em.filteredGroups))
	case "/":
		return *m, m.activateFilter(filterELBTargetGroups)
	case "r":
		if em.selectedLB != nil {
			return m.startLoadingWithMessage("Loading target health...", []string{em.selectedLB.Name}, em.loadGroups(*m, *em.selectedLB))
		}
	case "enter":
		if len(em.filteredGroups) > 0 && em.groupIdx < len(em.filteredGroups) {
			selected := em.filteredGroups[em.groupIdx]
			em.selectedGroup = &selected
			em.targetIdx = 0
			m.screen = screenELBTargetList
		}
	}
	return *m, nil
}

func (em *elbModel) updateTargetList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenELBTargetGroupList
	case "up", "k":
		if em.selectedGroup != nil {
			em.targetIdx = previousListIndex(em.targetIdx, len(em.selectedGroup.Targets))
		}
	case "down", "j":
		if em.selectedGroup != nil {
			em.targetIdx = nextListIndex(em.targetIdx, len(em.selectedGroup.Targets))
		}
	case "r":
		if em.selectedLB != nil {
			m.screen = screenELBTargetGroupList
			return m.startLoadingWithMessage("Loading target health...", []string{em.selectedLB.Name}, em.loadGroups(*m, *em.selectedLB))
		}
	}
	return *m, nil
}

func (em elbModel) loadBalancers(m Model) tea.Cmd {
	allRegions := em.allRegions && m.hasMultipleRegions()
	var regions []string
	if m.cfg != nil {
		regions = append(regions, m.cfg.Regions...)
	}
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		if allRegions {
			balancers, regionErrors := repo.ListLoadBalancersAcrossRegions(ctx, regions)
			return elbLoadBalancersLoadedMsg{balancers: balancers, regionErrors: regionErrors}
		}
		balancers, err := repo.ListLoadBalancers(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(balancers) == 0 {
			return errMsg{err: fmt.Errorf("no load balancers found")}
		}
		return elbLoadBalancersLoadedMsg{balancers: balancers}
	}
}

func (em elbModel) loadGroups(m Model, lb awsservice.ELBLoadBalancer) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		if lb.Region != "" && repo.Region != lb.Region {
			// Rows loaded through the all-regions scope drill into their own
			// region, not the globally active one.
			repo = repo.ForRegion(lb.Region)
		}
		groups, err := repo.ListTargetGroupHealth(ctx, lb.ARN)
		if err != nil {
			return errMsg{err: err}
		}
		return elbTargetGroupsLoadedMsg{loadBalancerARN: lb.ARN, groups: groups}
	}
}

func (em elbModel) viewLBList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	title := "Load Balancers"
	if em.allRegions && m.hasMultipleRegions() {
		title += " (all regions)"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterELBs))
	b.WriteString("\n\n")

	for _, regionErr := range em.regionErrors {
		panel.WriteString(errorStyle.Render(fmt.Sprintf("  %s: %v", regionErr.Region, regionErr.Err)))
		panel.WriteString("\n")
	}
	if len(em.filteredLBs) == 0 {
		emptyText := "  No load balancers found"
		if len(em.balancers) > 0 {
			emptyText = "  No matching load balancers"
		}
		panel.WriteString(dimStyle.Render(emptyText))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-11, 5)
		start := 0
		if em.lbIdx >= visibleLines {
			start = em.lbIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filteredLBs))
		for i := start; i < end; i++ {
			lb := em.filteredLBs[i]
			cursor := "  "
			style := normalStyle
			if i == em.lbIdx {
				cursor = "> "
				style = selectedStyle
			}
			row := lb.DisplayTitle()
			if em.allRegions && m.hasMultipleRegions() {
				row = fmt.Sprintf("[%s] %s", lb.Region, row)
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterELBs, row)))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d load balancers", len(em.filteredLBs), len(em.balancers))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (em elbModel) viewGroupList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Target Groups"))
	b.WriteString("\n")
	if em.selectedLB != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Load balancer: %s (%s)", em.selectedLB.Name, em.selectedLB.DNSName)))
		b.WriteString("\n")
	}
	b.WriteString(m.renderFilterValue(filterELBTargetGroups))
	b.WriteString("\n\n")

	if len(em.filteredGroups) == 0 {
		emptyText := "  No target groups attached"
		if len(em.groups) > 0 {
			emptyText = "  No matching target groups"
		}
		panel.WriteString(dimStyle.Render(emptyText))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-12, 5)
		start := 0
		if em.groupIdx >= visibleLines {
			start = em.groupIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filteredGroups))
		for i := start; i < end; i++ {
			group := em.filteredGroups[i]
			cursor := "  "
			style := normalStyle
			if group.UnhealthyCount > 0 {
				style = errorStyle
			}
			if i == em.groupIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterELBTargetGroups, group.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d target groups (unhealthiest first)", len(em.filteredGroups), len(em.groups))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (em elbModel) viewTargetList(m Model) string {
	if em.selectedGroup == nil {
		return ""
	}
	group := em.selectedGroup
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Target Health"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  Target group: %s  healthy:%d unhealthy:%d other:%d",
		group.Name, group.HealthyCount, group.UnhealthyCount, group.OtherCount)))
	b.WriteString("\n\n")

	if len(group.Targets) == 0 {
		panel.WriteString(dimStyle.Render("  No registered targets"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-12, 5)
		start := 0
		if em.targetIdx >= visibleLines {
			start = em.targetIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(group.Targets))
		for i := start; i < end; i++ {
			target := group.Targets[i]
			cursor := "  "
			style := normalStyle
			if target.State == "unhealthy" {
				style = errorStyle
			}
			if i == em.targetIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + target.DisplayTitle()))
			panel.WriteString("\n")
		}
		if em.targetIdx < len(group.Targets) {
			target := group.Targets[em.targetIdx]
			if target.Description != "" {
				panel.WriteString("\n")
				panel.WriteString(dimStyle.Render("  " + target.Description))
				panel.WriteString("\n")
			}
		}
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}
