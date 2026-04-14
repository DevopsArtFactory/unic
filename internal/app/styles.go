package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"unic/internal/update"
)

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	selectedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	normalStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	successStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	warningStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	infoStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	pathNodeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	pathLineStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("67"))
	filterStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	statusBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Background(lipgloss.Color("236"))
	listPanelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Background(lipgloss.Color("237"))
	detailLabelStyle = dimStyle.Copy().Width(14)
)

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
	leftParts = append(leftParts, fmt.Sprintf("region:%s", activeRegion))
	if m.cfg.AuthType != "" {
		leftParts = append(leftParts, fmt.Sprintf("auth:%s", m.cfg.AuthType))
	}
	if m.callerIdentity != nil && m.callerIdentity.Account != "" {
		leftParts = append(leftParts, fmt.Sprintf("account:%s", m.callerIdentity.Account))
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
		return statusBarStyle.Copy().Width(width).Align(lipgloss.Left).Render(leftText) + "\n\n"
	}

	leftWidth := width - rightMinWidth
	if leftWidth < leftMinWidth {
		leftWidth = leftMinWidth
	}
	rightWidth := width - leftWidth

	bar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		statusBarStyle.Copy().Width(leftWidth).Align(lipgloss.Left).Render(leftText),
		statusBarStyle.Copy().Width(rightWidth).Align(lipgloss.Right).Render(rightText),
	)
	return bar + "\n\n"
}

func (m Model) renderListPanel(content string) string {
	content = strings.TrimRight(content, "\n")
	style := listPanelStyle
	if m.width > 0 {
		style = style.MaxWidth(max(m.width, 1))
	}
	return style.Render(content)
}

func (m Model) renderHelpBar(content string) string {
	content = " " + strings.TrimSpace(content)
	style := helpStyle
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
