package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingsItem struct {
	name        string
	value       string
	description string
}

func (m Model) settingsItems() []settingsItem {
	return []settingsItem{
		{
			name:        "Boot splash",
			value:       onOff(m.bootSplash),
			description: "Show the retro startup splash on every launch. When off, it still appears once after install/update.",
		},
	}
}

func (m Model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.settingsItems()
	switch msg.String() {
	case "q", "esc":
		if m.settingsPrevScreen == 0 || m.settingsPrevScreen == screenSettings {
			m.screen = screenContextPicker
		} else {
			m.screen = m.settingsPrevScreen
		}
	case "up", "k":
		m.settingsIdx = previousListIndex(m.settingsIdx, len(items))
	case "down", "j":
		m.settingsIdx = nextListIndex(m.settingsIdx, len(items))
	case "enter", " ":
		return m.toggleSelectedSetting()
	}
	return m, nil
}

func (m Model) toggleSelectedSetting() (tea.Model, tea.Cmd) {
	switch m.settingsIdx {
	case 0:
		if err := m.toggleBootSplash(); err != nil {
			m.errMsg = err.Error()
			m.screen = screenError
		}
	}
	return m, nil
}

func (m Model) viewSettings() string {
	var b strings.Builder
	var panel strings.Builder
	items := m.settingsItems()
	m.settingsIdx = clampListIndex(m.settingsIdx, len(items))

	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Settings"))
	b.WriteString("\n\n")

	// Width-aware name column so styled (ANSI-wrapped) cells stay aligned.
	nameCol := lipgloss.NewStyle().Width(18)
	for i, item := range items {
		prefix := "  "
		nameStyle := normalStyle
		valueStyle := dimStyle
		if i == m.settingsIdx {
			prefix = "> "
			nameStyle = selectedStyle
			valueStyle = selectedStyle
		}
		nameCell := nameCol.Render(nameStyle.Render(item.name))
		valueCell := valueStyle.Render(item.value)
		panel.WriteString(prefix + nameCell + " " + valueCell + "\n")
		panel.WriteString("  " + dimStyle.Render(item.description))
		panel.WriteString("\n")
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • enter/space: toggle • esc/q: back"))
	return b.String()
}

func onOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}
