package app

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"unic/internal/config"
)

const (
	defaultContextTableWidth  = 52
	defaultContextTableHeight = 4
	contextTableColumnCount   = 4
	contextTableCellPadding   = 2
)

func newContextTable() table.Model {
	km := table.DefaultKeyMap()
	km.PageUp.Unbind()
	km.PageDown.Unbind()
	km.HalfPageUp.Unbind()
	km.HalfPageDown.Unbind()
	km.GotoTop.Unbind()
	km.GotoBottom.Unbind()
	km.LineUp = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	)
	km.LineDown = key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	)

	styles := table.DefaultStyles()
	styles.Header = dimStyle.Copy().Bold(true).Padding(0, 1)
	styles.Cell = table.DefaultStyles().Cell.Copy()
	styles.Selected = contextTableSelectedStyle()

	t := table.New(
		table.WithColumns(contextTableColumns(0)),
		table.WithHeight(defaultContextTableHeight),
		table.WithWidth(defaultContextTableWidth),
		table.WithFocused(true),
		table.WithKeyMap(km),
		table.WithStyles(styles),
	)
	t.Focus()
	return t
}

func contextTableSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("16")).
		Background(lipgloss.Color("220"))
}

func (m *Model) syncContextTable() {
	m.contextTable.SetColumns(contextTableColumns(m.width))
	m.contextTable.SetWidth(contextTableWidth(m.width))
	m.contextTable.SetHeight(m.contextPickerTableHeight())
	m.contextTable.SetRows(contextTableRows(m.filteredCtxList))
	m.contextTable.Focus()
	m.setContextTableCursor(m.ctxIdx)
	if cursor := m.contextTable.Cursor(); cursor >= 0 {
		m.ctxIdx = cursor
	}
}

func (m *Model) setContextTableCursor(index int) {
	m.ctxIdx = clampListIndex(index, len(m.filteredCtxList))
	m.contextTable.GotoTop()
	if m.ctxIdx > 0 {
		m.contextTable.MoveDown(m.ctxIdx)
	}
}

func (m *Model) moveContextTableUp() {
	if len(m.filteredCtxList) == 0 {
		return
	}
	if m.ctxIdx <= 0 {
		m.setContextTableCursor(len(m.filteredCtxList) - 1)
		return
	}
	m.contextTable.MoveUp(1)
	m.ctxIdx = m.contextTable.Cursor()
}

func (m *Model) moveContextTableDown() {
	if len(m.filteredCtxList) == 0 {
		return
	}
	if m.ctxIdx >= len(m.filteredCtxList)-1 {
		m.setContextTableCursor(0)
		return
	}
	m.contextTable.MoveDown(1)
	m.ctxIdx = m.contextTable.Cursor()
}

func contextTableRows(contexts []config.ContextInfo) []table.Row {
	rows := make([]table.Row, 0, len(contexts))
	for _, ctx := range contexts {
		current := ""
		if ctx.Current {
			current = "*"
		}
		authType := ctx.AuthType
		if authType == "" {
			authType = "default"
		}
		rows = append(rows, table.Row{
			ctx.Name,
			ctx.Region,
			authType,
			current,
		})
	}
	return rows
}

func contextTableWidth(terminalWidth int) int {
	if terminalWidth <= 0 {
		return defaultContextTableWidth
	}
	return max(terminalWidth-4, 28)
}

func contextTableHeight(terminalHeight int) int {
	if terminalHeight <= 0 {
		return defaultContextTableHeight
	}
	return max(terminalHeight-10, 2)
}

func (m Model) contextPickerTableHeight() int {
	if m.contextPickerCompact() {
		height := max(m.height-6, 2)
		if m.contextPickerFilterVisible() {
			height -= 2
		}
		return max(height, 2)
	}

	height := contextTableHeight(m.height)
	if m.contextPickerFilterVisible() {
		height -= 2
	}
	return max(height, 2)
}

func (m Model) contextPickerCompact() bool {
	return m.height > 0 && m.height <= 14
}

func (m Model) contextPickerFilterVisible() bool {
	if m.isFiltering(filterContexts) && m.renderFilterValue(filterContexts) != "" {
		return true
	}
	return false
}

func contextTableColumns(terminalWidth int) []table.Column {
	available := contextTableWidth(terminalWidth) - contextTableColumnCount*contextTableCellPadding
	if available < 16 {
		available = 16
	}

	currentWidth := 7
	regionWidth := 12
	authWidth := 12
	minNameWidth := 12

	nameWidth := available - regionWidth - authWidth - currentWidth
	if nameWidth < minNameWidth {
		deficit := minNameWidth - nameWidth
		if authWidth > 8 {
			shrink := min(deficit, authWidth-8)
			authWidth -= shrink
			deficit -= shrink
		}
		if deficit > 0 && regionWidth > 8 {
			shrink := min(deficit, regionWidth-8)
			regionWidth -= shrink
			deficit -= shrink
		}
		if deficit > 0 && currentWidth > 3 {
			currentWidth = max(currentWidth-deficit, 3)
		}
		nameWidth = max(available-regionWidth-authWidth-currentWidth, 6)
	}

	return []table.Column{
		{Title: lipgloss.NewStyle().Render("NAME"), Width: nameWidth},
		{Title: lipgloss.NewStyle().Render("REGION"), Width: regionWidth},
		{Title: lipgloss.NewStyle().Render("AUTH TYPE"), Width: authWidth},
		{Title: lipgloss.NewStyle().Render("CURRENT"), Width: currentWidth},
	}
}
