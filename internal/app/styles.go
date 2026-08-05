package app

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"unic/internal/update"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

var (
	titleStyle              = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	selectedStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	normalStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle                = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle              = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	successStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	warningStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	infoStyle               = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	pathNodeStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	pathLineStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("67"))
	filterStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	matchStyle              = lipgloss.NewStyle().Bold(true).Underline(true)
	favoriteServiceStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	statusBarStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Background(lipgloss.Color("236"))
	listPanelStyle          = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	helpStyle               = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Background(lipgloss.Color("237"))
	inspectorTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	inspectorSelectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("216")).Bold(true)
	inspectorAccentStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("209"))
	inspectorSectionStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("173")).Bold(true)
	inspectorReadyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("215")).Bold(true)
	inspectorPlannedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("95")).Bold(true)
	inspectorStatusBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("52"))
	inspectorListPanelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("160")).Padding(0, 1)
	inspectorHelpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("53"))
	detailLabelStyle        = dimStyle.Copy().Width(14)
	exitPanelStyle          = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("39")).
				Foreground(lipgloss.Color("252")).
				Background(lipgloss.Color("236")).
				Padding(1, 2)
	exitTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)
	exitBodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
	exitPromptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)
)

func (m Model) inspectorModeActive() bool {
	switch m.screen {
	case screenInspectorHome, screenInspectorWorkflowPlaceholder, screenInspectorChecklistPicker, screenInspectorScanning, screenInspectorResults, screenInspectorFindingDetail, screenInspectorChecklistResults, screenInspectorChecklistDetail:
		return true
	default:
		return false
	}
}

func (m Model) currentStatusBarStyle() lipgloss.Style {
	if m.inspectorModeActive() {
		return inspectorStatusBarStyle
	}
	return statusBarStyle
}

func (m Model) currentListPanelStyle() lipgloss.Style {
	if m.inspectorModeActive() {
		return inspectorListPanelStyle
	}
	return listPanelStyle
}

func (m Model) currentHelpStyle() lipgloss.Style {
	if m.inspectorModeActive() {
		return inspectorHelpStyle
	}
	return helpStyle
}

func (m Model) renderModeTitle(title string) string {
	if m.inspectorModeActive() {
		return inspectorTitleStyle.Render(title)
	}
	return titleStyle.Render(title)
}

func (m Model) renderStatusBar() string {
	var leftParts []string
	var rightParts []string

	if m.cfg.ContextName != "" {
		leftParts = append(leftParts, fmt.Sprintf("[%s]", m.cfg.ContextName))
	}
	activeRegion := m.cfg.Region
	if m.screen == screenReachabilityRegionList ||
		m.screen == screenReachabilitySourceList ||
		m.screen == screenReachabilityDestinationList ||
		m.screen == screenReachabilityConfig ||
		m.screen == screenReachabilityResult {
		activeRegion = m.activeReachabilityRegion()
	}
	regionLabel := fmt.Sprintf("region:%s", activeRegion)
	if len(m.cfg.Regions) > 1 {
		regionLabel += " [R switch]"
	}
	leftParts = append(leftParts, regionLabel)
	if m.cfg.AuthType != "" {
		leftParts = append(leftParts, fmt.Sprintf("auth:%s", m.cfg.AuthType))
	}
	if m.callerIdentity != nil && m.callerIdentity.Account != "" {
		leftParts = append(leftParts, fmt.Sprintf("account:%s", m.callerIdentity.Account))
	}
	if m.inspectorModeActive() {
		leftParts = append(leftParts, inspectorAccentStyle.Render("mode:inspector"))
	}

	if m.updateAvailable != "" {
		hint := "unic update"
		if m.installMethod == update.InstallBrew {
			hint = "brew upgrade unic"
		}
		rightParts = append(rightParts, filterStyle.Render(fmt.Sprintf("%s available • %s", m.updateAvailable, hint)))
	}

	leftText := " " + strings.Join(leftParts, "  ")
	rightText := ""
	if len(rightParts) > 0 {
		rightText = strings.Join(rightParts, "  ") + " "
	}

	width := m.width
	leftMinWidth := lipgloss.Width(leftText)
	rightMinWidth := lipgloss.Width(rightText)
	if width <= 0 || width < leftMinWidth+rightMinWidth {
		width = leftMinWidth + rightMinWidth
	}
	if rightText == "" {
		return m.currentStatusBarStyle().Copy().Width(width).Align(lipgloss.Left).Render(leftText) + "\n\n"
	}

	leftWidth := width - rightMinWidth
	if leftWidth < leftMinWidth {
		leftWidth = leftMinWidth
	}
	rightWidth := width - leftWidth

	bar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.currentStatusBarStyle().Copy().Width(leftWidth).Align(lipgloss.Left).Render(leftText),
		m.currentStatusBarStyle().Copy().Width(rightWidth).Align(lipgloss.Right).Render(rightText),
	)
	return bar + "\n\n"
}

func (m Model) renderListPanel(content string) string {
	content = strings.TrimRight(content, "\n")
	style := m.currentListPanelStyle()
	if m.width > 0 {
		style = style.MaxWidth(max(m.width, 1))
	}
	return style.Render(content)
}

func (m Model) renderHelpBar(content string) string {
	content = " " + strings.TrimSpace(content)
	style := m.currentHelpStyle()
	if m.width > 0 {
		style = style.Width(m.width)
	}
	return style.Render(content)
}

func renderDetailLine(label, value string) string {
	return "  " + detailLabelStyle.Render(label) + value
}

// fitToHeight ensures the rendered output is exactly m.height lines.
// It pads short content with blank lines and truncates long content,
// keeping both the header (top) and footer (bottom) visible by trimming
// from the middle of the content area.
func (m Model) fitToHeight(s string) string {
	if m.height <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	// Remove trailing empty line if present (common from trailing \n)
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= m.height {
		// Pad to exact height so the terminal doesn't shift
		for len(lines) < m.height {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}
	if selectedLine := selectedRenderedLine(lines); selectedLine >= 0 {
		return strings.Join(trimLinesAroundAnchor(lines, m.height, selectedLine), "\n")
	}
	// Content overflows: keep first (height-2) lines + last 1 line (footer)
	// with a "..." indicator
	footerLines := 1
	headerLines := m.height - footerLines - 1 // -1 for the "..." line
	if headerLines < 1 {
		headerLines = 1
	}
	result := make([]string, 0, m.height)
	result = append(result, lines[:headerLines]...)
	result = append(result, dimStyle.Render("  ..."))
	result = append(result, lines[len(lines)-footerLines:]...)
	return strings.Join(result, "\n")
}

func selectedRenderedLine(lines []string) int {
	for i, line := range lines {
		line = strings.TrimSpace(ansiEscapePattern.ReplaceAllString(line, ""))
		line = strings.TrimPrefix(line, "│")
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "> ") {
			return i
		}
	}
	return -1
}

func trimLinesAroundAnchor(lines []string, height, anchor int) []string {
	if height <= 0 || len(lines) <= height {
		return lines
	}

	if height == 1 {
		return []string{lines[anchor]}
	}
	if height == 2 {
		return []string{lines[anchor], lines[len(lines)-1]}
	}
	if height == 3 {
		return []string{lines[0], lines[anchor], lines[len(lines)-1]}
	}

	if anchor <= 0 || anchor >= len(lines)-1 {
		return trimLinesWithFixedEdges(lines, height)
	}

	windowSize := height - 2 // first line + footer
	if anchor > 1 {
		windowSize--
	}
	if anchor < len(lines)-2 {
		windowSize--
	}

	contextBefore := windowSize / 2
	contextAfter := windowSize - contextBefore - 1
	start := anchor - contextBefore
	end := anchor + contextAfter + 1

	if start < 1 {
		end += 1 - start
		start = 1
	}
	if end > len(lines)-1 {
		start -= end - (len(lines) - 1)
		end = len(lines) - 1
	}
	if start < 1 {
		start = 1
	}

	for {
		resultLen := 2 + (end - start)
		if start > 1 {
			resultLen++
		}
		if end < len(lines)-1 {
			resultLen++
		}
		if resultLen >= height {
			break
		}
		switch {
		case end < len(lines)-1:
			end++
		case start > 1:
			start--
		default:
			break
		}
	}

	return buildAnchoredLines(lines, height, start, end)
}

func trimLinesWithFixedEdges(lines []string, height int) []string {
	result := make([]string, 0, height)
	headerLines := height - 2
	if headerLines < 1 {
		headerLines = 1
	}
	if headerLines > len(lines)-1 {
		headerLines = len(lines) - 1
	}
	result = append(result, lines[:headerLines]...)
	result = append(result, dimStyle.Render("  ..."))
	result = append(result, lines[len(lines)-1])
	if len(result) > height {
		return result[:height]
	}
	for len(result) < height {
		result = append(result, "")
	}
	return result
}

func buildAnchoredLines(lines []string, height, start, end int) []string {
	result := make([]string, 0, height)
	result = append(result, lines[0])
	if start > 1 {
		result = append(result, dimStyle.Render("  ..."))
	}
	result = append(result, lines[start:end]...)
	if end < len(lines)-1 {
		result = append(result, dimStyle.Render("  ..."))
	}
	result = append(result, lines[len(lines)-1])
	if len(result) > height {
		result = result[:height]
	}
	for len(result) < height {
		result = append(result, "")
	}
	return result
}

func (m Model) viewLoading() string {
	title := m.loadingTitle
	if strings.TrimSpace(title) == "" {
		title = "Loading..."
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("%s %s", m.loadingSpinner.View(), title)))
	if len(m.loadingDetails) > 0 {
		b.WriteString("\n\n")
		for _, detail := range m.loadingDetails {
			if strings.TrimSpace(detail) == "" {
				continue
			}
			b.WriteString(detail)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model) viewError() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(errorStyle.Render("Error"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render(m.errMsg))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("enter/esc: go back • q: quit"))
	return b.String()
}

func (m Model) viewExitNotice() string {
	title := strings.TrimSpace(m.exitTitle)
	if title == "" {
		title = "SYSTEM NOTICE"
	}
	message := strings.TrimSpace(m.exitMessage)
	if message == "" {
		message = "Operation complete."
	}

	boxWidth := 60
	if m.width > 0 {
		boxWidth = min(max(m.width-10, 48), 72)
	}
	contentWidth := max(boxWidth-6, 32)

	var body strings.Builder
	body.WriteString(exitTitleStyle.Copy().Width(contentWidth).Align(lipgloss.Center).Render(title))
	body.WriteString("\n\n")
	body.WriteString(exitBodyStyle.Copy().Width(contentWidth).Align(lipgloss.Center).Render(message))
	body.WriteString("\n\n")
	body.WriteString(exitPromptStyle.Copy().Width(contentWidth).Align(lipgloss.Center).Render("Press any key to exit unic"))

	modal := exitPanelStyle.Copy().Width(boxWidth).Render(body.String())
	if m.width <= 0 || m.height <= 0 {
		return modal
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}
