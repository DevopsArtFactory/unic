package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

var newAwsRepositoryForRegionFn = awsservice.NewAwsRepository

// hasMultipleRegions reports whether the active context exposes more than one
// resource region.
func (m Model) hasMultipleRegions() bool {
	return m.cfg != nil && len(m.cfg.Regions) > 1
}

func (m Model) canSwitchResourceRegion() bool {
	if !m.hasMultipleRegions() || m.filterTI.Focused() || m.isTextEntryScreen() {
		return false
	}
	switch m.screen {
	case screenRegionPicker, screenContextPicker, screenLoading, screenInspectorScanning:
		return false
	default:
		return true
	}
}

func (m Model) activeRegionIndex() int {
	if m.cfg == nil {
		return 0
	}
	for i, region := range m.cfg.Regions {
		if region == m.cfg.Region {
			return i
		}
	}
	return 0
}

func (m Model) updateRegionPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	regions := m.cfg.Regions
	switch msg.String() {
	case "q", "esc":
		m.screen = m.regionPrevScreen
	case "up", "k":
		m.regionIdx = previousListIndex(m.regionIdx, len(regions))
	case "down", "j":
		m.regionIdx = nextListIndex(m.regionIdx, len(regions))
	case "enter":
		if m.regionIdx < 0 || m.regionIdx >= len(regions) {
			return m, nil
		}
		region := regions[m.regionIdx]
		if region == m.cfg.Region {
			m.screen = m.regionPrevScreen
			return m, nil
		}
		return m.startLoading(m.switchResourceRegion(region))
	}
	return m, nil
}

func (m Model) switchResourceRegion(region string) tea.Cmd {
	cfg := *m.cfg
	repo := m.awsRepo
	return func() tea.Msg {
		if repo != nil {
			return regionSwitchedMsg{region: region, repo: repo.ForRegion(region)}
		}
		cfg.Region = region
		newRepo, err := newAwsRepositoryForRegionFn(m.commandContext(), &cfg)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to switch resource region to %s: %w", region, err)}
		}
		return regionSwitchedMsg{region: region, repo: newRepo}
	}
}

func (m Model) viewRegionPicker() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Select Resource Region"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Authentication, account, and role remain unchanged."))
	b.WriteString("\n\n")

	visibleLines := max(m.height-10, 5)
	start := 0
	if m.regionIdx >= visibleLines {
		start = m.regionIdx - visibleLines + 1
	}
	end := min(start+visibleLines, len(m.cfg.Regions))
	for i := start; i < end; i++ {
		region := m.cfg.Regions[i]
		cursor := "  "
		style := normalStyle
		marker := ""
		if region == m.cfg.Region {
			marker = "  (active)"
		}
		if i == m.regionIdx {
			cursor = "> "
			style = selectedStyle
		}
		panel.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, region)))
		panel.WriteString(dimStyle.Render(marker))
		panel.WriteString("\n")
	}
	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • enter: switch • esc: cancel"))
	return b.String()
}
