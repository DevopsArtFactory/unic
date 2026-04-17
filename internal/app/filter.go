package app

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type filterTarget int

const (
	filterNone filterTarget = iota
	filterInstances
	filterSubnetIPs
	filterRDS
	filterRoute53Zones
	filterRoute53Records
	filterIAMUsers
	filterSecrets
	filterSecurityGroups
	filterECSClusters
	filterECSServices
	filterCWLogGroups
	filterCWLogStreams
	filterCWLogViewer
	filterS3Buckets
	filterS3Objects
	filterContexts
	filterVPCs
	filterSubnets
	filterInspectorChecklistFiles
	filterLambdaFunctions
)

// Filterable is implemented by any type that supports text-based filtering.
type Filterable interface {
	FilterText() string
}

func newFilterInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 256
	return ti
}

func handleFilterKey(key, current string) (next string, deactivate bool, changed bool) {
	switch key {
	case "esc", "enter":
		return current, true, false
	case "backspace":
		if len(current) == 0 {
			return current, false, false
		}
		return current[:len(current)-1], false, true
	}

	if len(key) == 1 {
		return current + key, false, true
	}
	return current, false, false
}

func isFilterNavigationKey(key string) bool {
	switch key {
	case "up", "down":
		return true
	default:
		return false
	}
}

func (m *Model) syncFilterInputWidth() {
	width := m.width - len("Filter: ") - 4
	if width < 10 {
		width = 10
	}
	m.filterTI.Width = width
}

func (m Model) filterValue(target filterTarget) string {
	if m.filters == nil {
		return ""
	}
	return m.filters[target]
}

func (m *Model) storeFilterValue(target filterTarget, value string) {
	if m.filters == nil {
		m.filters = make(map[filterTarget]string)
	}
	if value == "" {
		delete(m.filters, target)
		return
	}
	m.filters[target] = value
}

func (m *Model) activateFilter(target filterTarget) tea.Cmd {
	m.activeFilter = target
	m.filterTI.SetValue(m.filterValue(target))
	m.filterTI.CursorEnd()
	m.syncFilterInputWidth()
	return m.filterTI.Focus()
}

func (m *Model) deactivateFilter() {
	m.filterTI.Blur()
	m.activeFilter = filterNone
}

func (m Model) isFiltering(target filterTarget) bool {
	return m.filterTI.Focused() && m.activeFilter == target
}

func (m *Model) resetFilter(target filterTarget) {
	m.storeFilterValue(target, "")
	if m.activeFilter == target {
		m.filterTI.Reset()
		m.deactivateFilter()
	}
	m.applyFilterTarget(target)
}

func (m *Model) updateSharedFilter(msg tea.KeyMsg, target filterTarget) (tea.Cmd, bool) {
	if !m.isFiltering(target) {
		return nil, false
	}

	if isFilterNavigationKey(msg.String()) {
		return nil, false
	}

	switch msg.String() {
	case "esc", "enter":
		m.deactivateFilter()
		return nil, true
	}

	prev := m.filterTI.Value()
	var cmd tea.Cmd
	m.filterTI, cmd = m.filterTI.Update(msg)
	if next := m.filterTI.Value(); next != prev {
		m.storeFilterValue(target, next)
		m.applyFilterTarget(target)
	}
	return cmd, true
}

func (m *Model) applyFilterTarget(target filterTarget) {
	for _, submodel := range m.featureSubmodels() {
		if submodel.ApplyFilter(m, target) {
			return
		}
	}

	switch target {
	case filterInstances:
		m.filtered = applyFilter(m.instances, m.filterValue(target))
		m.instIdx = 0
	case filterSubnetIPs:
		m.applyIPFilter()
	case filterRDS:
		m.filteredRDS = applyFilter(m.rdsInstances, m.filterValue(target))
		m.rdsIdx = 0
	case filterRoute53Zones:
		m.filteredRoute53Zones = applyFilter(m.route53Zones, m.filterValue(target))
		m.route53ZoneIdx = 0
	case filterRoute53Records:
		m.filteredRoute53Records = applyFilter(m.route53Records, m.filterValue(target))
		m.route53RecordIdx = 0
	case filterIAMUsers:
		m.refreshIAMUserFilter()
	case filterSecrets:
		m.filteredSecrets = applyFilter(m.secrets, m.filterValue(target))
		m.secretIdx = 0
	case filterSecurityGroups:
		m.filteredSecurityGroups = applyFilter(m.securityGroups, m.filterValue(target))
		m.sgIdx = 0
	case filterECSClusters:
		m.filteredECSClusters = applyFilter(m.ecsClusters, m.filterValue(target))
		m.ecsClusterIdx = 0
	case filterECSServices:
		m.filteredECSServices = applyFilter(m.ecsServices, m.filterValue(target))
		m.ecsServiceIdx = 0
	case filterS3Buckets:
		m.filteredS3Buckets = applyFilter(m.s3Buckets, m.filterValue(target))
		m.s3BucketIdx = 0
	case filterS3Objects:
		m.filteredS3Objects = applyFilter(m.s3Objects, m.filterValue(target))
		m.s3ObjectIdx = 0
	case filterContexts:
		m.filteredCtxList = applyFilter(m.ctxList, m.filterValue(target))
		m.ctxIdx = 0
		m.syncContextTable()
	case filterVPCs:
		m.filteredVPCs = applyFilter(m.vpcs, m.filterValue(target))
		m.vpcIdx = 0
	case filterSubnets:
		m.filteredSubnets = applyFilter(m.subnets, m.filterValue(target))
		m.subnetIdx = 0
	case filterInspectorChecklistFiles:
		m.filteredChecklistFiles = applyFilter(m.inspectorChecklistFiles, m.filterValue(target))
		m.inspectorChecklistFileIdx = 0
	case filterLambdaFunctions:
		m.filteredLambdaFunctions = applyFilter(m.lambdaFunctions, m.filterValue(target))
		m.lambdaFunctionIdx = 0
	}
}

func (m Model) renderFilterValue(target filterTarget) string {
	if m.isFiltering(target) {
		return filterStyle.Render("Filter: " + m.filterTI.View())
	}
	if value := m.filterValue(target); value != "" {
		return dimStyle.Render("Filter: " + value)
	}
	return ""
}

func (m Model) renderHighlightedValue(target filterTarget, value string) string {
	return renderHighlightedMatch(value, m.filterValue(target))
}
