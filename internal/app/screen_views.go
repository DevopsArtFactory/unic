package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	"unic/internal/domain"
)

// Saved views capture a repeatable operational jump: service feature, the
// context it ran in, and the browser filter that was active. Applying a view
// replays that in one step, switching context first when needed.

// featurePrimaryFilter maps a feature to the shared filter its list screen
// applies on load, so views (and the palette) can prefill it.
var featurePrimaryFilter = map[domain.FeatureKind]filterTarget{
	domain.FeatureEC2InstanceBrowser:    filterEC2BrowserInstances,
	domain.FeatureAutoScalingBrowser:    filterAutoScalingGroups,
	domain.FeatureSSMSession:            filterInstances,
	domain.FeatureRDSBrowser:            filterRDS,
	domain.FeatureCloudFormationBrowser: filterCloudFormationStacks,
	domain.FeatureRoute53Browser:        filterRoute53Zones,
	domain.FeatureSecretsBrowser:        filterSecrets,
	domain.FeatureSecurityGroupBrowser:  filterSecurityGroups,
	domain.FeatureIAMUsersBrowser:       filterIAMUsers,
	domain.FeatureCloudWatchMetrics:     filterCWMetrics,
	domain.FeatureCloudWatchAlarms:      filterCWAlarms,
	domain.FeatureCloudTrailEvents:      filterCloudTrailEvents,
	domain.FeatureEventBridgeRules:      filterEventBridgeRules,
	domain.FeatureCloudWatchLogsBrowser: filterCWLogGroups,
	domain.FeatureECSExec:               filterECSClusters,
	domain.FeatureEKSBrowser:            filterEKSClusters,
	domain.FeatureECRRepositoryBrowser:  filterECRRepositories,
	domain.FeatureFISTemplateBrowser:    filterFISTemplates,
	domain.FeatureS3Browser:             filterS3Buckets,
	domain.FeatureSNSBrowser:            filterSNSTopics,
	domain.FeatureSQSBrowser:            filterSQSQueues,
	domain.FeatureELBBrowser:            filterELBs,
	domain.FeatureACMCertificateBrowser: filterACMCertificates,
	domain.FeatureStepFunctionsBrowser:  filterStepFunctionStateMachines,
	domain.FeatureSSMParameterBrowser:   filterSSMParameters,
	domain.FeatureElastiCacheBrowser:    filterElastiCacheResources,
	domain.FeatureKMSKeyBrowser:         filterKMSKeys,
	domain.FeatureLambdaBrowser:         filterLambdaFunctions,
	domain.FeatureDynamoDBBrowser:       filterDynamoDBTables,
	domain.FeatureBedrockAPIKeys:        filterBedrockKeys,
	domain.FeatureVPCBrowser:            filterVPCs,
}

type viewsModel struct {
	views      []config.ViewEntry
	idx        int
	naming     bool
	nameInput  string
	notice     string
	prevScreen screen
}

// pendingView carries a view application across an async context switch.

func (m Model) openViews() (tea.Model, tea.Cmd) {
	views, err := config.Views(m.configPath)
	if err != nil {
		return m, func() tea.Msg { return errMsg{err: err} }
	}
	m.views.views = views
	m.views.idx = 0
	m.views.naming = false
	m.views.nameInput = ""
	m.views.notice = ""
	if !overlayChainMatches(m, m.screen, func(current screen) bool {
		return current == screenViewList
	}) {
		m.views.prevScreen = m.screen
	}
	m.screen = screenViewList
	return m, nil
}

// captureCurrentView snapshots the last drilled service feature, its primary
// filter value, and the active context.
func (m Model) captureCurrentView() (config.ViewEntry, bool) {
	// activeService is the identity recorded when the feature list was
	// entered; svcIdx indexes the sorted/filtered display list and must not
	// be used to resolve the service.
	if m.activeService == "" || m.featIdx >= len(m.features) {
		return config.ViewEntry{}, false
	}
	feat := m.features[m.featIdx]
	view := config.ViewEntry{
		Service: string(m.activeService),
		Feature: string(feat.Kind),
	}
	if m.cfg != nil {
		view.Context = m.cfg.ContextName
	}
	if target, ok := featurePrimaryFilter[feat.Kind]; ok {
		view.Filter = m.filterValue(target)
	}
	return view, true
}

func (m Model) updateViews(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.views.naming {
		switch key {
		case "esc":
			m.views.naming = false
			m.views.nameInput = ""
		case "enter":
			name := strings.TrimSpace(m.views.nameInput)
			if name == "" {
				return m, nil
			}
			view, ok := m.captureCurrentView()
			if !ok {
				m.views.naming = false
				m.views.notice = "Nothing to save: open a service feature first"
				return m, nil
			}
			view.Name = name
			if err := config.SaveView(m.configPath, view); err != nil {
				return m, func() tea.Msg { return errMsg{err: err} }
			}
			views, err := config.Views(m.configPath)
			if err != nil {
				return m, func() tea.Msg { return errMsg{err: err} }
			}
			m.views.views = views
			m.views.naming = false
			m.views.nameInput = ""
			m.views.notice = fmt.Sprintf("Saved view %q", name)
		case "backspace":
			m.views.nameInput = trimLastRune(m.views.nameInput)
		default:
			m.views.nameInput = appendKeyRunes(m.views.nameInput, msg)
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		return m.restoreScreenAfterOverlay(m.views.prevScreen)
	case "up", "k":
		m.views.idx = previousListIndex(m.views.idx, len(m.views.views))
	case "down", "j":
		m.views.idx = nextListIndex(m.views.idx, len(m.views.views))
	case "s":
		m.views.naming = true
		m.views.nameInput = ""
		m.views.notice = ""
	case "d":
		if len(m.views.views) == 0 || m.views.idx >= len(m.views.views) {
			return m, nil
		}
		name := m.views.views[m.views.idx].Name
		if err := config.DeleteView(m.configPath, name); err != nil {
			return m, func() tea.Msg { return errMsg{err: err} }
		}
		views, err := config.Views(m.configPath)
		if err != nil {
			return m, func() tea.Msg { return errMsg{err: err} }
		}
		m.views.views = views
		if m.views.idx >= len(m.views.views) && m.views.idx > 0 {
			m.views.idx--
		}
		m.views.notice = fmt.Sprintf("Deleted view %q", name)
	case "enter":
		if len(m.views.views) == 0 || m.views.idx >= len(m.views.views) {
			return m, nil
		}
		return m.applyView(m.views.views[m.views.idx])
	}
	return m, nil
}

// applyView replays a saved view: switch context first when the view names a
// different one (the jump continues from contextSwitchedMsg), otherwise jump
// straight into the feature with the filter prefilled.
func (m Model) applyView(view config.ViewEntry) (tea.Model, tea.Cmd) {
	m.ctxPrevWasLoading = false
	if view.Context != "" && (m.cfg == nil || m.cfg.ContextName != view.Context) {
		m.pendingView = &view
		m.ctxPrevScreen = screenServiceList
		return m.startLoading(m.switchContext(view.Context))
	}
	return m.jumpToView(view)
}

func (m Model) jumpToView(view config.ViewEntry) (tea.Model, tea.Cmd) {
	kind := domain.FeatureKind(view.Feature)
	m.enterServiceForPalette(paletteItem{
		service: domain.AwsService(view.Service),
		feature: kind,
	})
	if view.Filter != "" {
		if target, ok := featurePrimaryFilter[kind]; ok {
			m.storeFilterValue(target, view.Filter)
		}
	}
	return m.startFeature(kind)
}

func (m Model) viewViews() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Saved Views"))
	b.WriteString("\n")

	if m.views.naming {
		b.WriteString(normalStyle.Render("  Save current view as: "))
		b.WriteString(filterStyle.Render(fmt.Sprintf("%s▏", m.views.nameInput)))
		b.WriteString("\n")
		if view, ok := m.captureCurrentView(); ok {
			detail := fmt.Sprintf("  %s / %s", view.Service, view.Feature)
			if view.Filter != "" {
				detail += fmt.Sprintf("  filter:%q", view.Filter)
			}
			if view.Context != "" {
				detail += fmt.Sprintf("  context:%s", view.Context)
			}
			b.WriteString(dimStyle.Render(detail))
			b.WriteString("\n")
		}
	} else if m.views.notice != "" {
		b.WriteString(dimStyle.Render("  " + m.views.notice))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if len(m.views.views) == 0 {
		panel.WriteString(dimStyle.Render("  No saved views yet — press s to save the current one"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if m.views.idx >= visibleLines {
			start = m.views.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.views.views))
		for i := start; i < end; i++ {
			view := m.views.views[i]
			cursor := "  "
			style := normalStyle
			if i == m.views.idx {
				cursor = "> "
				style = selectedStyle
			}
			label := fmt.Sprintf("%-24s %s / %s", view.Name, view.Service, view.Feature)
			if view.Filter != "" {
				label += fmt.Sprintf("  filter:%q", view.Filter)
			}
			if view.Context != "" {
				label += fmt.Sprintf("  [%s]", view.Context)
			}
			panel.WriteString(style.Render(cursor + label))
			panel.WriteString("\n")
		}
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	help := "enter: open • s: save current • d: delete • esc: back"
	if m.views.naming {
		help = "type: name • enter: save • esc: cancel"
	}
	b.WriteString(m.renderHelpBar(help))
	return b.String()
}
