package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsservice "unic/internal/services/aws"
)

var (
	inspectorSeverityCriticalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	inspectorSeverityHighStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	inspectorSeverityMediumStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	inspectorSeverityLowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Bold(true)
)

var inspectorSeverityFilters = []awsservice.RuleSeverity{
	"",
	awsservice.RuleSeverityCritical,
	awsservice.RuleSeverityHigh,
	awsservice.RuleSeverityMedium,
	awsservice.RuleSeverityLow,
}

func (m Model) handleInspectorMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case inspectorScanLoadedMsg:
		m.inspectorReport = msg.report
		m.selectedInspectorFinding = nil
		m.applyInspectorSeverityFilter()
		m.screen = screenInspectorResults
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) updateInspectorHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
	case "enter", "r":
		return m.startInspectorScan()
	}
	return m, nil
}

func (m Model) updateInspectorResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.selectedInspectorFinding = nil
		m.screen = screenInspectorHome
	case "up", "k":
		if m.inspectorIdx > 0 {
			m.inspectorIdx--
		}
	case "down", "j":
		if m.inspectorIdx < len(m.inspectorFindings)-1 {
			m.inspectorIdx++
		}
	case "enter":
		if len(m.inspectorFindings) > 0 && m.inspectorIdx < len(m.inspectorFindings) {
			selected := m.inspectorFindings[m.inspectorIdx]
			m.selectedInspectorFinding = &selected
			m.screen = screenInspectorFindingDetail
		}
	case "r":
		return m.startInspectorScan()
	case "1", "2", "3", "4", "5":
		idx := int(msg.String()[0] - '1')
		if idx >= 0 && idx < len(inspectorSeverityFilters) {
			m.inspectorSeverityFilter = inspectorSeverityFilters[idx]
			m.applyInspectorSeverityFilter()
		}
	}
	return m, nil
}

func (m Model) updateInspectorFindingDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenInspectorResults
	case "r":
		return m.startInspectorScan()
	}
	return m, nil
}

func (m Model) startInspectorScan() (tea.Model, tea.Cmd) {
	m.selectedInspectorFinding = nil
	m.screen = screenInspectorScanning
	m.loadingSpinner = newLoadingSpinner()
	return m, tea.Batch(m.loadingSpinner.Tick, m.loadSecurityScan())
}

func (m Model) loadSecurityScan() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}

		report, err := repo.RunSecurityScan(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return inspectorScanLoadedMsg{report: report}
	}
}

func (m *Model) applyInspectorSeverityFilter() {
	m.inspectorIdx = 0
	if m.inspectorReport == nil {
		m.inspectorFindings = nil
		return
	}

	var filtered []awsservice.SecurityFinding
	for _, finding := range m.inspectorReport.Findings {
		if finding.MatchesSeverity(m.inspectorSeverityFilter) {
			filtered = append(filtered, finding)
		}
	}
	m.inspectorFindings = filtered
}

func (m Model) viewInspectorHome() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Inspector"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("  Run built-in security scans against the active AWS context."))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  Built-in rule packs cover Security Groups, RDS, IAM access keys, Secrets Manager, and S3 buckets."))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  Registered rule packs: %d", awsservice.RegisteredSecurityInspectorScannerCount())))
	b.WriteString("\n")
	if m.inspectorReport != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf(
			"  Last scan: %s • findings:%d • warnings:%d",
			m.inspectorReport.ScannedAt.Local().Format("2006-01-02 15:04:05"),
			len(m.inspectorReport.Findings),
			len(m.inspectorReport.Warnings),
		)))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("enter/r: run security scan • esc: back • H: home"))
	return b.String()
}

func (m Model) viewInspectorScanning() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Inspector — Security Scan"))
	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render(fmt.Sprintf("%s Running built-in rule packs...", m.loadingSpinner.View())))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Checking network exposure, backups, key age, secret rotation, and S3 bucket posture."))
	return b.String()
}

func (m Model) viewInspectorResults() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Inspector Findings"))
	b.WriteString("\n")

	if m.inspectorReport != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf(
			"Scanned: %s  Rule Packs: %d  Findings: %d",
			m.inspectorReport.ScannedAt.Local().Format("2006-01-02 15:04:05"),
			m.inspectorReport.ScannerCount,
			len(m.inspectorReport.Findings),
		)))
		b.WriteString("\n")
	}

	b.WriteString(m.renderInspectorSeveritySelector())
	b.WriteString("\n\n")

	if m.inspectorReport != nil && len(m.inspectorReport.Warnings) > 0 {
		panel.WriteString(errorStyle.Render(fmt.Sprintf("Warnings: %d rule pack(s) reported errors", len(m.inspectorReport.Warnings))))
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render("  " + m.inspectorReport.Warnings[0]))
		panel.WriteString("\n\n")
	}

	if len(m.inspectorFindings) == 0 {
		panel.WriteString(dimStyle.Render("  No matching findings"))
		if m.inspectorReport != nil && len(m.inspectorReport.Findings) == 0 && m.inspectorReport.ScannerCount == 0 {
			panel.WriteString("\n")
			panel.WriteString(dimStyle.Render("  No built-in rule packs are registered yet."))
		}
		panel.WriteString("\n")
	} else {
		resourceWidth := 24
		for _, finding := range m.inspectorFindings {
			resourceWidth = max(resourceWidth, len(inspectorFindingResource(finding)))
		}
		if resourceWidth > 36 {
			resourceWidth = 36
		}
		resourceCol := lipgloss.NewStyle().Width(resourceWidth)

		panel.WriteString(dimStyle.Render("  SEVERITY   " + resourceCol.Render("RESOURCE") + "RULE"))
		panel.WriteString("\n")

		visibleLines := max(m.height-13, 5)
		start := 0
		if m.inspectorIdx >= visibleLines {
			start = m.inspectorIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.inspectorFindings))

		for i := start; i < end; i++ {
			finding := m.inspectorFindings[i]
			cursor := "  "
			textStyle := normalStyle
			if i == m.inspectorIdx {
				cursor = "> "
				textStyle = selectedStyle
			}

			resource := inspectorShorten(inspectorFindingResource(finding), resourceWidth)
			row := cursor +
				padInspectorSeverity(renderInspectorSeverity(finding.Severity), 11) +
				" " +
				resourceCol.Inherit(textStyle).Render(resource) +
				textStyle.Render(finding.RuleName)
			panel.WriteString(row)
			panel.WriteString("\n")
		}
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • 1-5: severity • enter: detail • r: rescan • esc: back • H: home"))
	return b.String()
}

func (m Model) viewInspectorFindingDetail() string {
	if m.selectedInspectorFinding == nil {
		return ""
	}

	finding := m.selectedInspectorFinding
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Inspector Finding Detail"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Severity", renderInspectorSeverity(finding.Severity)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Rule", normalStyle.Render(finding.RuleName)))
	b.WriteString("\n")
	if finding.RuleID != "" {
		b.WriteString(renderDetailLine("Rule ID", normalStyle.Render(finding.RuleID)))
		b.WriteString("\n")
	}
	if finding.ResourceType != "" {
		b.WriteString(renderDetailLine("Resource Type", normalStyle.Render(finding.ResourceType)))
		b.WriteString("\n")
	}
	b.WriteString(renderDetailLine("Resource ID", normalStyle.Render(inspectorFindingResource(*finding))))
	b.WriteString("\n\n")

	width := 80
	if m.width > 0 {
		width = max(m.width-4, 40)
	}
	paragraph := lipgloss.NewStyle().Width(width)

	b.WriteString(titleStyle.Render("Summary"))
	b.WriteString("\n\n")
	b.WriteString(paragraph.Render("  " + finding.Summary))
	b.WriteString("\n\n")

	b.WriteString(titleStyle.Render("Recommendation"))
	b.WriteString("\n\n")
	b.WriteString(paragraph.Render("  " + finding.Recommendation))
	b.WriteString("\n\n")

	b.WriteString(m.renderHelpBar("esc: back • r: rescan • H: home"))
	return b.String()
}

func (m Model) renderInspectorSeveritySelector() string {
	parts := make([]string, 0, len(inspectorSeverityFilters))
	for idx, severity := range inspectorSeverityFilters {
		label := fmt.Sprintf("%d:%s", idx+1, severity.Label())
		if severity == m.inspectorSeverityFilter {
			parts = append(parts, selectedStyle.Render("["+label+"]"))
		} else {
			parts = append(parts, dimStyle.Render(" "+label+" "))
		}
	}
	return strings.Join(parts, " ")
}

func inspectorFindingResource(finding awsservice.SecurityFinding) string {
	if finding.ResourceID != "" {
		return finding.ResourceID
	}
	if finding.ResourceType != "" {
		return finding.ResourceType
	}
	return "-"
}

func renderInspectorSeverity(severity awsservice.RuleSeverity) string {
	switch severity {
	case awsservice.RuleSeverityCritical:
		return inspectorSeverityCriticalStyle.Render(string(severity))
	case awsservice.RuleSeverityHigh:
		return inspectorSeverityHighStyle.Render(string(severity))
	case awsservice.RuleSeverityMedium:
		return inspectorSeverityMediumStyle.Render(string(severity))
	case awsservice.RuleSeverityLow:
		return inspectorSeverityLowStyle.Render(string(severity))
	default:
		return dimStyle.Render(string(severity))
	}
}

func padInspectorSeverity(value string, width int) string {
	plainWidth := lipgloss.Width(value)
	if plainWidth >= width {
		return value
	}
	return value + strings.Repeat(" ", width-plainWidth)
}

func inspectorShorten(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}

	suffix := ""
	targetWidth := width
	if width <= 3 {
		targetWidth = width
	} else {
		suffix = "..."
		targetWidth = width - lipgloss.Width(suffix)
	}
	if targetWidth <= 0 {
		return suffix
	}

	var b strings.Builder
	currentWidth := 0
	for _, r := range value {
		runeWidth := lipgloss.Width(string(r))
		if currentWidth+runeWidth > targetWidth {
			break
		}
		b.WriteRune(r)
		currentWidth += runeWidth
	}

	if b.Len() == 0 {
		return suffix
	}
	return b.String() + suffix
}
