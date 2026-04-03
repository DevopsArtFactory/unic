package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"unic/internal/update"
)

var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	normalStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	filterStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	statusBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Background(lipgloss.Color("236"))
)

func (m Model) renderStatusBar() string {
	var parts []string

	if m.cfg.ContextName != "" {
		parts = append(parts, fmt.Sprintf("[%s]", m.cfg.ContextName))
	}
	parts = append(parts, fmt.Sprintf("region:%s", m.cfg.Region))
	if m.cfg.AuthType != "" {
		parts = append(parts, fmt.Sprintf("auth:%s", m.cfg.AuthType))
	}
	if m.callerIdentity != nil && m.callerIdentity.Account != "" {
		parts = append(parts, fmt.Sprintf("account:%s", m.callerIdentity.Account))
	}

	if m.updateAvailable != "" {
		hint := "unic update"
		if m.installMethod == update.InstallBrew {
			hint = "brew upgrade unic"
		}
		parts = append(parts, filterStyle.Render(fmt.Sprintf("%s available — %s", m.updateAvailable, hint)))
	}

	bar := strings.Join(parts, "  ")
	if m.width > 0 {
		if len(bar) < m.width {
			bar += strings.Repeat(" ", m.width-len(bar))
		}
	}
	return statusBarStyle.Render(bar) + "\n\n"
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
	return titleStyle.Render("Loading...")
}

func (m Model) viewError() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(errorStyle.Render("Error"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render(m.errMsg))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("enter/esc: go back • q: quit"))
	return b.String()
}
