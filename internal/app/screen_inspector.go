package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"unic/internal/inspector"
	awsservice "unic/internal/services/aws"
)

var (
	inspectorSeverityCriticalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	inspectorSeverityHighStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	inspectorSeverityMediumStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	inspectorSeverityLowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Bold(true)
	inspectorChecklistPassStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Bold(true)
	inspectorChecklistFailStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
)

var inspectorSeverityFilters = []inspector.RuleSeverity{
	"",
	inspector.RuleSeverityCritical,
	inspector.RuleSeverityHigh,
	inspector.RuleSeverityMedium,
	inspector.RuleSeverityLow,
}

type checklistPickerEntry struct {
	Name     string
	Path     string
	IsDir    bool
	IsParent bool
}

func (e checklistPickerEntry) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s", e.Name, e.Path))
}

func (m *Model) refreshInspectorWorkflows() {
	m.inspectorWorkflows = inspector.Workflows(m.inspectorChecklistPath)
	if m.inspectorWorkflowIdx >= len(m.inspectorWorkflows) {
		m.inspectorWorkflowIdx = 0
	}
}

func (m *Model) ensureInspectorWorkflows() {
	if len(m.inspectorWorkflows) == 0 {
		m.refreshInspectorWorkflows()
		return
	}
	m.refreshInspectorWorkflows()
}

func (m *Model) enterInspectorMode() {
	m.ensureInspectorWorkflows()
	m.screen = screenInspectorHome
	m.selectedInspectorFinding = nil
	m.selectedChecklistResult = nil
	m.inspectorIdx = 0
	m.inspectorChecklistIdx = 0
}

func (m Model) currentInspectorWorkflow() inspector.Workflow {
	if len(m.inspectorWorkflows) == 0 {
		return inspector.Workflow{}
	}
	if m.inspectorWorkflowIdx < 0 || m.inspectorWorkflowIdx >= len(m.inspectorWorkflows) {
		return m.inspectorWorkflows[0]
	}
	return m.inspectorWorkflows[m.inspectorWorkflowIdx]
}

func (m Model) checklistWorkflowIndex() int {
	for i, workflow := range m.inspectorWorkflows {
		if workflow.Kind == inspector.WorkflowChecklist {
			return i
		}
	}
	return 0
}

func (m Model) initialChecklistPickerDir() string {
	if dir := strings.TrimSpace(m.inspectorChecklistDir); dir != "" && checklistPickerDirExists(dir) {
		return dir
	}

	checklistPath := strings.TrimSpace(m.inspectorChecklistPath)
	if checklistPath != "" {
		dir := filepath.Dir(checklistPath)
		if checklistPickerDirExists(dir) {
			return dir
		}
	}

	cwd, err := os.Getwd()
	if err == nil {
		candidate := filepath.Join(cwd, "checklists")
		if checklistPickerDirExists(candidate) {
			return candidate
		}
		if checklistPickerDirExists(cwd) {
			return cwd
		}
	}

	return "."
}

func checklistPickerDirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (m *Model) loadChecklistPickerEntries(dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve checklist directory %s: %w", dir, err)
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return fmt.Errorf("failed to read checklist directory %s: %w", absDir, err)
	}

	items := make([]checklistPickerEntry, 0, len(entries)+1)
	parent := filepath.Dir(absDir)
	if parent != absDir {
		items = append(items, checklistPickerEntry{
			Name:     "..",
			Path:     parent,
			IsDir:    true,
			IsParent: true,
		})
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		if entry.IsDir() {
			items = append(items, checklistPickerEntry{
				Name:  name,
				Path:  filepath.Join(absDir, name),
				IsDir: true,
			})
			continue
		}

		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		items = append(items, checklistPickerEntry{
			Name: name,
			Path: filepath.Join(absDir, name),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		switch {
		case left.IsParent != right.IsParent:
			return left.IsParent
		case left.IsDir != right.IsDir:
			return left.IsDir
		default:
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
		}
	})

	m.inspectorChecklistDir = absDir
	m.inspectorChecklistFiles = items
	m.inspectorChecklistError = ""
	m.inspectorChecklistFileIdx = 0
	m.storeFilterValue(filterInspectorChecklistFiles, "")
	if m.activeFilter == filterInspectorChecklistFiles {
		m.filterTI.Reset()
		m.deactivateFilter()
	}
	m.applyFilterTarget(filterInspectorChecklistFiles)
	return nil
}

func (m Model) openChecklistPicker() (Model, tea.Cmd) {
	if err := m.loadChecklistPickerEntries(m.initialChecklistPickerDir()); err != nil {
		m.errMsg = err.Error()
		m.screen = screenError
		return m, nil
	}

	m.inspectorWorkflowIdx = m.checklistWorkflowIndex()
	m.screen = screenInspectorChecklistPicker
	return m, nil
}

func (m Model) handleInspectorMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case inspectorScanLoadedMsg:
		m.inspectorReport = msg.report
		m.selectedInspectorFinding = nil
		m.applyInspectorSeverityFilter()
		m.screen = screenInspectorResults
		return m, nil, true
	case inspectorChecklistLoadedMsg:
		m.inspectorChecklistReport = msg.report
		m.selectedChecklistResult = nil
		m.inspectorChecklistIdx = 0
		m.screen = screenInspectorChecklistResults
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) updateInspectorHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenServiceList
	case "up", "k":
		m.inspectorWorkflowIdx = previousListIndex(m.inspectorWorkflowIdx, len(m.inspectorWorkflows))
	case "down", "j":
		m.inspectorWorkflowIdx = nextListIndex(m.inspectorWorkflowIdx, len(m.inspectorWorkflows))
	case "r":
		if m.currentInspectorWorkflow().Available {
			return m.startInspectorWorkflow(m.currentInspectorWorkflow().Kind)
		}
	case "l":
		if m.currentInspectorWorkflow().Kind == inspector.WorkflowChecklist {
			return m.openChecklistPicker()
		}
	case "enter":
		return m.openInspectorWorkflow()
	}
	return m, nil
}

func (m Model) openInspectorWorkflow() (tea.Model, tea.Cmd) {
	workflow := m.currentInspectorWorkflow()
	if workflow.Kind == inspector.WorkflowChecklist && !workflow.Available {
		return m.openChecklistPicker()
	}
	if !workflow.Available {
		m.screen = screenInspectorWorkflowPlaceholder
		return m, nil
	}

	switch workflow.Kind {
	case inspector.WorkflowSecurity:
		if m.inspectorReport != nil {
			m.applyInspectorSeverityFilter()
			m.screen = screenInspectorResults
			return m, nil
		}
	case inspector.WorkflowChecklist:
		if m.inspectorChecklistReport != nil {
			m.inspectorChecklistIdx = 0
			m.screen = screenInspectorChecklistResults
			return m, nil
		}
	}
	return m.startInspectorWorkflow(workflow.Kind)
}

func (m Model) updateInspectorWorkflowPlaceholder(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "l":
		if m.currentInspectorWorkflow().Kind == inspector.WorkflowChecklist {
			return m.openChecklistPicker()
		}
	case "enter":
		if m.currentInspectorWorkflow().Kind == inspector.WorkflowChecklist {
			return m.openChecklistPicker()
		}
		m.screen = screenInspectorHome
	case "q", "esc":
		m.screen = screenInspectorHome
	}
	return m, nil
}

func (m Model) updateInspectorResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.selectedInspectorFinding = nil
		m.screen = screenInspectorHome
	case "up", "k":
		m.inspectorIdx = previousListIndex(m.inspectorIdx, len(m.inspectorFindings))
	case "down", "j":
		m.inspectorIdx = nextListIndex(m.inspectorIdx, len(m.inspectorFindings))
	case "enter":
		if len(m.inspectorFindings) > 0 && m.inspectorIdx < len(m.inspectorFindings) {
			selected := m.inspectorFindings[m.inspectorIdx]
			m.selectedInspectorFinding = &selected
			m.screen = screenInspectorFindingDetail
		}
	case "r":
		return m.startInspectorWorkflow(inspector.WorkflowSecurity)
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
		return m.startInspectorWorkflow(inspector.WorkflowSecurity)
	}
	return m, nil
}

func (m Model) updateInspectorChecklistResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.selectedChecklistResult = nil
		m.screen = screenInspectorHome
	case "l":
		return m.openChecklistPicker()
	case "up", "k":
		if m.inspectorChecklistReport != nil {
			m.inspectorChecklistIdx = previousListIndex(m.inspectorChecklistIdx, len(m.inspectorChecklistReport.Results))
		}
	case "down", "j":
		if m.inspectorChecklistReport != nil {
			m.inspectorChecklistIdx = nextListIndex(m.inspectorChecklistIdx, len(m.inspectorChecklistReport.Results))
		}
	case "enter":
		if m.inspectorChecklistReport != nil && len(m.inspectorChecklistReport.Results) > 0 && m.inspectorChecklistIdx < len(m.inspectorChecklistReport.Results) {
			selected := m.inspectorChecklistReport.Results[m.inspectorChecklistIdx]
			m.selectedChecklistResult = &selected
			m.screen = screenInspectorChecklistDetail
		}
	case "r":
		return m.startInspectorWorkflow(inspector.WorkflowChecklist)
	}
	return m, nil
}

func (m Model) updateInspectorChecklistDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenInspectorChecklistResults
	case "l":
		return m.openChecklistPicker()
	case "r":
		return m.startInspectorWorkflow(inspector.WorkflowChecklist)
	}
	return m, nil
}

func (m Model) updateInspectorChecklistPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterInspectorChecklistFiles); handled {
		return m, cmd
	}

	switch msg.String() {
	case "q", "esc":
		m.screen = screenInspectorHome
	case "up", "k":
		m.inspectorChecklistFileIdx = previousListIndex(m.inspectorChecklistFileIdx, len(m.filteredChecklistFiles))
	case "down", "j":
		m.inspectorChecklistFileIdx = nextListIndex(m.inspectorChecklistFileIdx, len(m.filteredChecklistFiles))
	case "/":
		return m, m.activateFilter(filterInspectorChecklistFiles)
	case "enter":
		if len(m.filteredChecklistFiles) == 0 || m.inspectorChecklistFileIdx >= len(m.filteredChecklistFiles) {
			return m, nil
		}

		selected := m.filteredChecklistFiles[m.inspectorChecklistFileIdx]
		if selected.IsDir {
			if err := m.loadChecklistPickerEntries(selected.Path); err != nil {
				m.inspectorChecklistError = err.Error()
			}
			return m, nil
		}

		if _, err := inspector.LoadChecklist(selected.Path); err != nil {
			m.inspectorChecklistError = err.Error()
			return m, nil
		}

		m.inspectorChecklistPath = selected.Path
		m.inspectorChecklistDir = filepath.Dir(selected.Path)
		m.inspectorChecklistReport = nil
		m.selectedChecklistResult = nil
		m.inspectorChecklistIdx = 0
		m.inspectorChecklistError = ""
		m.refreshInspectorWorkflows()
		m.inspectorWorkflowIdx = m.checklistWorkflowIndex()
		return m.startChecklistScan()
	}

	return m, nil
}

func (m Model) startInspectorWorkflow(kind inspector.WorkflowKind) (tea.Model, tea.Cmd) {
	switch kind {
	case inspector.WorkflowChecklist:
		return m.startChecklistScan()
	case inspector.WorkflowSecurity:
		fallthrough
	default:
		return m.startInspectorScan()
	}
}

func (m Model) startInspectorScan() (tea.Model, tea.Cmd) {
	m.selectedInspectorFinding = nil
	m.screen = screenInspectorScanning
	m.loadingSpinner = newLoadingSpinner()
	return m, tea.Batch(m.loadingSpinner.Tick, m.loadSecurityScan())
}

func (m Model) startChecklistScan() (tea.Model, tea.Cmd) {
	m.selectedChecklistResult = nil
	m.screen = screenInspectorScanning
	m.loadingSpinner = newLoadingSpinner()
	return m, tea.Batch(m.loadingSpinner.Tick, m.loadChecklistScan())
}

func (m Model) loadSecurityScan() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}

		report, err := inspector.RunSecurityScan(ctx, repo)
		if err != nil {
			return errMsg{err: err}
		}
		return inspectorScanLoadedMsg{report: report}
	}
}

func (m Model) loadChecklistScan() tea.Cmd {
	return func() tea.Msg {
		checklistPath := strings.TrimSpace(m.inspectorChecklistPath)
		if checklistPath == "" {
			return errMsg{err: fmt.Errorf("Checklist Inspector requires a loaded checklist file")}
		}

		checklist, err := inspector.LoadChecklist(checklistPath)
		if err != nil {
			return errMsg{err: err}
		}

		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}

		report, err := inspector.RunChecklist(ctx, repo, checklist)
		if err != nil {
			return errMsg{err: err}
		}
		return inspectorChecklistLoadedMsg{report: report}
	}
}

func (m *Model) applyInspectorSeverityFilter() {
	m.inspectorIdx = 0
	if m.inspectorReport == nil {
		m.inspectorFindings = nil
		return
	}

	var filtered []inspector.SecurityFinding
	for _, finding := range m.inspectorReport.Findings {
		if finding.MatchesSeverity(m.inspectorSeverityFilter) {
			filtered = append(filtered, finding)
		}
	}
	m.inspectorFindings = filtered
}

func (m Model) viewInspectorHome() string {
	selected := m.currentInspectorWorkflow()

	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle("Inspector Mode"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Cross-service workflows with inspection-focused chrome and shared AWS context."))
	b.WriteString("\n\n")

	for i, workflow := range m.inspectorWorkflows {
		cursor := "  "
		nameStyle := normalStyle
		badgeStyle := inspectorReadyStyle
		if !workflow.Available {
			badgeStyle = inspectorPlannedStyle
		}
		if i == m.inspectorWorkflowIdx {
			cursor = "> "
			nameStyle = inspectorSelectedStyle
		}

		row := fmt.Sprintf("%s%-22s %s", cursor, workflow.Title, badgeStyle.Render("["+workflow.StatusLabel()+"]"))
		panel.WriteString(nameStyle.Render(row))
		panel.WriteString("\n")
		if i == m.inspectorWorkflowIdx {
			panel.WriteString(dimStyle.Render("    " + workflow.Description))
			panel.WriteString("\n")
		}
	}

	panel.WriteString("\n")
	panel.WriteString(inspectorSectionStyle.Render("Selected Workflow"))
	panel.WriteString("\n")
	switch selected.Kind {
	case inspector.WorkflowSecurity:
		panel.WriteString(normalStyle.Render("  Run the built-in security rule packs across the active AWS context."))
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  Registered rule packs: %d", inspector.RegisteredSecurityInspectorScannerCount())))
		panel.WriteString("\n")
		if m.inspectorReport != nil {
			panel.WriteString(dimStyle.Render(fmt.Sprintf(
				"  Last scan: %s • findings:%d • warnings:%d",
				m.inspectorReport.ScannedAt.Local().Format("2006-01-02 15:04:05"),
				len(m.inspectorReport.Findings),
				len(m.inspectorReport.Warnings),
			)))
			panel.WriteString("\n")
		}
		panel.WriteString(inspectorAccentStyle.Render("  Enter opens the latest findings or starts a fresh scan. Press r to force a new scan."))
	case inspector.WorkflowChecklist:
		if selected.Available {
			panel.WriteString(normalStyle.Render("  Run the configured YAML checklist against the current AWS context."))
			panel.WriteString("\n")
			panel.WriteString(dimStyle.Render(fmt.Sprintf("  Checklist file: %s", m.inspectorChecklistPath)))
			panel.WriteString("\n")
			if m.inspectorChecklistReport != nil {
				panel.WriteString(dimStyle.Render(fmt.Sprintf(
					"  Last run: %s • pass:%d • fail:%d",
					m.inspectorChecklistReport.ScannedAt.Local().Format("2006-01-02 15:04:05"),
					m.inspectorChecklistReport.PassedCount,
					m.inspectorChecklistReport.FailedCount,
				)))
				panel.WriteString("\n")
			}
			panel.WriteString(inspectorAccentStyle.Render("  Enter opens the latest report or starts a fresh scan. Press l to choose another file or r to force a new scan."))
		} else {
			panel.WriteString(normalStyle.Render("  Choose a YAML checklist file inside the TUI to enable checklist-driven readiness checks."))
			panel.WriteString("\n")
			panel.WriteString(dimStyle.Render("  Press enter or l to open the checklist file picker. You can still pass --checklist <path> at launch if you prefer."))
		}
	default:
		panel.WriteString(dimStyle.Render("  No inspector workflows are registered."))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: choose workflow • enter: open • l: load checklist • r: run selected workflow • esc: services • H: home"))
	return b.String()
}

func (m Model) viewInspectorWorkflowPlaceholder() string {
	workflow := m.currentInspectorWorkflow()

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle(workflow.Title))
	b.WriteString("\n\n")
	if workflow.Kind == inspector.WorkflowChecklist {
		b.WriteString(normalStyle.Render("  Checklist Inspector needs a checklist file before it can run."))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Press enter or l to open the file picker and load a YAML checklist for resource readiness and baseline posture checks."))
		b.WriteString("\n\n")
		b.WriteString(renderDetailLine("Status", inspectorPlannedStyle.Render(workflow.StatusLabel())))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Description", normalStyle.Render(workflow.Description)))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Optional CLI", normalStyle.Render("unic --checklist ./checklists/readiness.yaml")))
	} else {
		b.WriteString(normalStyle.Render("  This workflow is not available yet."))
		b.WriteString("\n\n")
		b.WriteString(renderDetailLine("Status", inspectorPlannedStyle.Render(workflow.StatusLabel())))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Description", normalStyle.Render(workflow.Description)))
	}
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("enter: load checklist • esc: back • H: home"))
	return b.String()
}

func (m Model) viewInspectorChecklistPicker() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle("Checklist File Picker"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Browse folders and choose a .yaml or .yml file to load into Checklist Inspector."))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Directory: " + m.inspectorChecklistDir))
	if strings.TrimSpace(m.inspectorChecklistPath) != "" {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("Loaded: " + m.inspectorChecklistPath))
	}
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterInspectorChecklistFiles))
	b.WriteString("\n\n")

	if m.inspectorChecklistError != "" {
		panel.WriteString(errorStyle.Render("  " + m.inspectorChecklistError))
		panel.WriteString("\n\n")
	}

	if len(m.filteredChecklistFiles) == 0 {
		panel.WriteString(dimStyle.Render("  No checklist files or folders match the current filter"))
		panel.WriteString("\n")
	} else {
		typeCol := lipgloss.NewStyle().Width(8)
		panel.WriteString(dimStyle.Render("  " + typeCol.Render("TYPE") + "NAME"))
		panel.WriteString("\n")

		visibleLines := max(m.height-14, 5)
		start := 0
		if m.inspectorChecklistFileIdx >= visibleLines {
			start = m.inspectorChecklistFileIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredChecklistFiles))

		for i := start; i < end; i++ {
			entry := m.filteredChecklistFiles[i]
			cursor := "  "
			textStyle := normalStyle
			if i == m.inspectorChecklistFileIdx {
				cursor = "> "
				textStyle = inspectorSelectedStyle
			}

			entryType := "file"
			if entry.IsDir {
				entryType = "dir"
			}

			name := entry.Name
			if entry.IsDir && !entry.IsParent {
				name += "/"
			}
			if entry.Path == m.inspectorChecklistPath {
				name += " [loaded]"
			}

			row := cursor +
				typeCol.Inherit(dimStyle).Render(entryType) +
				textStyle.Render(m.renderHighlightedValue(filterInspectorChecklistFiles, name))
			panel.WriteString(row)
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d entries", len(m.filteredChecklistFiles), len(m.inspectorChecklistFiles))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: open/load • esc: Inspector mode • H: home"))
	return b.String()
}

func (m Model) viewInspectorScanning() string {
	var title string
	var description string

	switch m.currentInspectorWorkflow().Kind {
	case inspector.WorkflowChecklist:
		title = "Inspector Mode — Checklist Scan"
		description = "  Verifying RDS instances, security groups, and secrets against the supplied checklist YAML."
	default:
		title = "Inspector Mode — Security Scan"
		description = "  Checking network exposure, backups, key age, secret rotation, logging baselines, and bucket posture."
	}

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle(title))
	b.WriteString("\n\n")
	switch m.currentInspectorWorkflow().Kind {
	case inspector.WorkflowChecklist:
		b.WriteString(inspectorTitleStyle.Render(fmt.Sprintf("%s Running checklist expectations...", m.loadingSpinner.View())))
	default:
		b.WriteString(inspectorTitleStyle.Render(fmt.Sprintf("%s Running built-in rule packs...", m.loadingSpinner.View())))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(description))
	return b.String()
}

func (m Model) viewInspectorResults() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle("Security Inspector Findings"))
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
				textStyle = inspectorSelectedStyle
			}

			resource := inspectorShorten(inspectorFindingResource(finding), resourceWidth)
			row := cursor +
				padInspectorText(renderInspectorSeverity(finding.Severity), 11) +
				" " +
				resourceCol.Inherit(textStyle).Render(resource) +
				textStyle.Render(finding.RuleName)
			panel.WriteString(row)
			panel.WriteString("\n")
		}
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • 1-5: severity • enter: detail • r: rescan • esc: Inspector mode • H: home"))
	return b.String()
}

func (m Model) viewInspectorFindingDetail() string {
	if m.selectedInspectorFinding == nil {
		return ""
	}

	finding := m.selectedInspectorFinding
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle("Security Inspector Finding"))
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

	b.WriteString(inspectorSectionStyle.Render("Summary"))
	b.WriteString("\n\n")
	b.WriteString(paragraph.Render("  " + finding.Summary))
	b.WriteString("\n\n")

	b.WriteString(inspectorSectionStyle.Render("Recommendation"))
	b.WriteString("\n\n")
	b.WriteString(paragraph.Render("  " + finding.Recommendation))
	b.WriteString("\n\n")

	b.WriteString(m.renderHelpBar("esc: back • r: rescan • H: home"))
	return b.String()
}

func (m Model) viewInspectorChecklistResults() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle("Checklist Inspector Results"))
	b.WriteString("\n")

	if m.inspectorChecklistReport != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf(
			"Checklist: %s  Scanned: %s  Pass: %d  Fail: %d",
			m.inspectorChecklistReport.ChecklistName,
			m.inspectorChecklistReport.ScannedAt.Local().Format("2006-01-02 15:04:05"),
			m.inspectorChecklistReport.PassedCount,
			m.inspectorChecklistReport.FailedCount,
		)))
		b.WriteString("\n")
		if m.inspectorChecklistReport.SourcePath != "" {
			b.WriteString(dimStyle.Render("Source: " + m.inspectorChecklistReport.SourcePath))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	if m.inspectorChecklistReport == nil || len(m.inspectorChecklistReport.Results) == 0 {
		panel.WriteString(dimStyle.Render("  No checklist results"))
		panel.WriteString("\n")
	} else {
		typeWidth := 16
		resourceWidth := 24
		for _, result := range m.inspectorChecklistReport.Results {
			resourceWidth = max(resourceWidth, len(checklistResultResource(result)))
		}
		if resourceWidth > 34 {
			resourceWidth = 34
		}

		typeCol := lipgloss.NewStyle().Width(typeWidth)
		resourceCol := lipgloss.NewStyle().Width(resourceWidth)
		panel.WriteString(dimStyle.Render("  STATUS " + typeCol.Render("TYPE") + resourceCol.Render("RESOURCE") + "CHECK"))
		panel.WriteString("\n")

		visibleLines := max(m.height-14, 5)
		start := 0
		if m.inspectorChecklistIdx >= visibleLines {
			start = m.inspectorChecklistIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.inspectorChecklistReport.Results))

		for i := start; i < end; i++ {
			result := m.inspectorChecklistReport.Results[i]
			cursor := "  "
			textStyle := normalStyle
			if i == m.inspectorChecklistIdx {
				cursor = "> "
				textStyle = inspectorSelectedStyle
			}

			row := cursor +
				padInspectorText(renderChecklistStatus(result.Passed), 6) +
				" " +
				typeCol.Inherit(textStyle).Render(string(result.Type)) +
				resourceCol.Inherit(textStyle).Render(inspectorShorten(checklistResultResource(result), resourceWidth)) +
				textStyle.Render(result.Title)
			panel.WriteString(row)
			panel.WriteString("\n")
		}
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • enter: detail • l: load checklist • r: rerun checklist • esc: Inspector mode • H: home"))
	return b.String()
}

func (m Model) viewInspectorChecklistDetail() string {
	if m.selectedChecklistResult == nil {
		return ""
	}

	result := m.selectedChecklistResult
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle("Checklist Inspector Detail"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Status", renderChecklistStatus(result.Passed)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Check", normalStyle.Render(result.Title)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Check ID", normalStyle.Render(result.CheckID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Type", normalStyle.Render(string(result.Type))))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Target", normalStyle.Render(result.Resource)))
	b.WriteString("\n")
	if result.ResourceContext != "" && result.ResourceContext != result.Resource {
		b.WriteString(renderDetailLine("Matched", normalStyle.Render(result.ResourceContext)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	width := 80
	if m.width > 0 {
		width = max(m.width-4, 40)
	}
	paragraph := lipgloss.NewStyle().Width(width)

	b.WriteString(inspectorSectionStyle.Render("Summary"))
	b.WriteString("\n\n")
	b.WriteString(paragraph.Render("  " + result.Summary))
	b.WriteString("\n\n")

	b.WriteString(inspectorSectionStyle.Render("Details"))
	b.WriteString("\n\n")
	if len(result.Details) == 0 {
		b.WriteString(paragraph.Render("  No mismatches. All expectations matched."))
	} else {
		for _, detail := range result.Details {
			b.WriteString(paragraph.Render("  - " + detail))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	b.WriteString(m.renderHelpBar("esc: back • l: load checklist • r: rerun checklist • H: home"))
	return b.String()
}

func (m Model) renderInspectorSeveritySelector() string {
	parts := make([]string, 0, len(inspectorSeverityFilters))
	for idx, severity := range inspectorSeverityFilters {
		label := fmt.Sprintf("%d:%s", idx+1, severity.Label())
		if severity == m.inspectorSeverityFilter {
			parts = append(parts, inspectorSelectedStyle.Render("["+label+"]"))
		} else {
			parts = append(parts, dimStyle.Render(" "+label+" "))
		}
	}
	return strings.Join(parts, " ")
}

func inspectorFindingResource(finding inspector.SecurityFinding) string {
	if finding.ResourceID != "" {
		return finding.ResourceID
	}
	if finding.ResourceType != "" {
		return finding.ResourceType
	}
	return "-"
}

func checklistResultResource(result inspector.ChecklistResult) string {
	if result.ResourceContext != "" {
		return result.ResourceContext
	}
	if result.Resource != "" {
		return result.Resource
	}
	return "-"
}

func renderInspectorSeverity(severity inspector.RuleSeverity) string {
	switch severity {
	case inspector.RuleSeverityCritical:
		return inspectorSeverityCriticalStyle.Render(string(severity))
	case inspector.RuleSeverityHigh:
		return inspectorSeverityHighStyle.Render(string(severity))
	case inspector.RuleSeverityMedium:
		return inspectorSeverityMediumStyle.Render(string(severity))
	case inspector.RuleSeverityLow:
		return inspectorSeverityLowStyle.Render(string(severity))
	default:
		return dimStyle.Render(string(severity))
	}
}

func renderChecklistStatus(passed bool) string {
	if passed {
		return inspectorChecklistPassStyle.Render("PASS")
	}
	return inspectorChecklistFailStyle.Render("FAIL")
}

func padInspectorText(value string, width int) string {
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
