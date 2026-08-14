package app

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type filterTarget int

const (
	filterNone filterTarget = iota
	filterServices
	filterInstances
	filterEC2BrowserInstances
	filterEC2BrowserRelated
	filterSubnetIPs
	filterRDS
	filterRoute53Zones
	filterRoute53Records
	filterIAMUsers
	filterSecrets
	filterSecurityGroups
	filterECSClusters
	filterECSServices
	filterEKSClusters
	filterEKSNodeGroups
	filterEKSAddons
	filterECRRepositories
	filterECRImages
	filterFISTemplates
	filterFISExperiments
	filterCWMetrics
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
	filterBedrockKeys
	filterCWAlarms
	filterCloudTrailEvents
	filterSQSQueues
	filterELBs
	filterELBTargetGroups
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
		return trimLastRune(current), false, true
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
	case filterServices:
		m.applyServiceListFilter()
	case filterInstances:
		m.filtered = applyFilter(m.instances, m.filterValue(target))
		m.instIdx = 0
	case filterContexts:
		m.ctxIdx = 0
		m.applyContextListFilter()
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
