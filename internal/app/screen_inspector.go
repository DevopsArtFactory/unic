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

type inspectorModel struct {
	workflows               []inspector.Workflow
	workflowIdx             int
	checklistPath           string
	checklistDir            string
	checklistFiles          []checklistPickerEntry
	filteredChecklistFiles  []checklistPickerEntry
	checklistFileIdx        int
	checklistError          string
	report                  *inspector.SecurityScanReport
	findings                []inspector.SecurityFinding
	idx                     int
	severityFilter          inspector.RuleSeverity
	selectedFinding         *inspector.SecurityFinding
	checklistReport         *inspector.ChecklistReport
	checklistIdx            int
	selectedChecklistResult *inspector.ChecklistResult

	// Add-check wizard
	addStep     int
	addTypeIdx  int
	addFields   []checkFieldDef
	addFieldIdx int
	addInput    string
	addValues   map[string]string
	addError    string
}

func newInspectorModel(checklistPath string) inspectorModel {
	return inspectorModel{
		checklistPath: checklistPath,
		workflows:     inspector.Workflows(checklistPath),
	}
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

func (im *inspectorModel) refreshWorkflows() {
	im.workflows = inspector.Workflows(im.checklistPath)
	if im.workflowIdx >= len(im.workflows) {
		im.workflowIdx = 0
	}
}

func (im *inspectorModel) ensureWorkflows() {
	if len(im.workflows) == 0 {
		im.refreshWorkflows()
	}
}

func (im *inspectorModel) Enter(m *Model) {
	im.ensureWorkflows()
	m.screen = screenInspectorHome
	im.selectedFinding = nil
	im.selectedChecklistResult = nil
	im.idx = 0
	im.checklistIdx = 0
}

func (im inspectorModel) currentWorkflow() inspector.Workflow {
	if len(im.workflows) == 0 {
		return inspector.Workflow{}
	}
	if im.workflowIdx < 0 || im.workflowIdx >= len(im.workflows) {
		return im.workflows[0]
	}
	return im.workflows[im.workflowIdx]
}

func (im inspectorModel) checklistWorkflowIndex() int {
	for i, workflow := range im.workflows {
		if workflow.Kind == inspector.WorkflowChecklist {
			return i
		}
	}
	return 0
}

func (im inspectorModel) initialChecklistPickerDir() string {
	if dir := strings.TrimSpace(im.checklistDir); dir != "" && checklistPickerDirExists(dir) {
		return dir
	}

	checklistPath := strings.TrimSpace(im.checklistPath)
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

func (im *inspectorModel) loadChecklistPickerEntries(m *Model, dir string) error {
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

	im.checklistDir = absDir
	im.checklistFiles = items
	im.checklistError = ""
	im.checklistFileIdx = 0
	m.storeFilterValue(filterInspectorChecklistFiles, "")
	if m.activeFilter == filterInspectorChecklistFiles {
		m.filterTI.Reset()
		m.deactivateFilter()
	}
	m.applyFilterTarget(filterInspectorChecklistFiles)
	return nil
}

func (im *inspectorModel) openChecklistPicker(m *Model) (tea.Model, tea.Cmd) {
	if err := im.loadChecklistPickerEntries(m, im.initialChecklistPickerDir()); err != nil {
		m.errMsg = err.Error()
		m.screen = screenError
		return *m, nil
	}

	im.workflowIdx = im.checklistWorkflowIndex()
	m.screen = screenInspectorChecklistPicker
	return *m, nil
}

func (im *inspectorModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case inspectorScanLoadedMsg:
		im.report = msg.report
		im.selectedFinding = nil
		im.applySeverityFilter()
		m.screen = screenInspectorResults
		return *m, nil, true
	case inspectorChecklistLoadedMsg:
		im.checklistReport = msg.report
		im.selectedChecklistResult = nil
		im.checklistIdx = 0
		m.screen = screenInspectorChecklistResults
		return *m, nil, true
	}
	return *m, nil, false
}

func (im *inspectorModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenInspectorHome:
		newM, cmd := im.updateHome(m, msg)
		return newM, cmd, true
	case screenInspectorWorkflowPlaceholder:
		newM, cmd := im.updateWorkflowPlaceholder(m, msg)
		return newM, cmd, true
	case screenInspectorChecklistPicker:
		newM, cmd := im.updateChecklistPicker(m, msg)
		return newM, cmd, true
	case screenInspectorChecklistAdd:
		newM, cmd := im.updateChecklistAdd(m, msg)
		return newM, cmd, true
	case screenInspectorResults:
		newM, cmd := im.updateResults(m, msg)
		return newM, cmd, true
	case screenInspectorFindingDetail:
		newM, cmd := im.updateFindingDetail(m, msg)
		return newM, cmd, true
	case screenInspectorChecklistResults:
		newM, cmd := im.updateChecklistResults(m, msg)
		return newM, cmd, true
	case screenInspectorChecklistDetail:
		newM, cmd := im.updateChecklistDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (im inspectorModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenInspectorHome:
		return im.viewHome(m), true
	case screenInspectorWorkflowPlaceholder:
		return im.viewWorkflowPlaceholder(m), true
	case screenInspectorChecklistPicker:
		return im.viewChecklistPicker(m), true
	case screenInspectorChecklistAdd:
		return im.viewChecklistAdd(m), true
	case screenInspectorScanning:
		return im.viewScanning(m), true
	case screenInspectorResults:
		return im.viewResults(m), true
	case screenInspectorFindingDetail:
		return im.viewFindingDetail(m), true
	case screenInspectorChecklistResults:
		return im.viewChecklistResults(m), true
	case screenInspectorChecklistDetail:
		return im.viewChecklistDetail(m), true
	default:
		return "", false
	}
}

func (im *inspectorModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterInspectorChecklistFiles {
		return false
	}
	im.filteredChecklistFiles = applyFilter(im.checklistFiles, m.filterValue(target))
	im.checklistFileIdx = 0
	return true
}

func (im *inspectorModel) updateHome(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenServiceList
	case "up", "k":
		im.workflowIdx = previousListIndex(im.workflowIdx, len(im.workflows))
	case "down", "j":
		im.workflowIdx = nextListIndex(im.workflowIdx, len(im.workflows))
	case "r":
		if im.currentWorkflow().Available {
			return im.startWorkflow(m, im.currentWorkflow().Kind)
		}
	case "l":
		if im.currentWorkflow().Kind == inspector.WorkflowChecklist {
			return im.openChecklistPicker(m)
		}
	case "enter":
		return im.openWorkflow(m)
	}
	return *m, nil
}

func (im *inspectorModel) openWorkflow(m *Model) (tea.Model, tea.Cmd) {
	workflow := im.currentWorkflow()
	if workflow.Kind == inspector.WorkflowChecklist && !workflow.Available {
		return im.openChecklistPicker(m)
	}
	if !workflow.Available {
		m.screen = screenInspectorWorkflowPlaceholder
		return *m, nil
	}

	switch workflow.Kind {
	case inspector.WorkflowSecurity:
		if im.report != nil {
			im.applySeverityFilter()
			m.screen = screenInspectorResults
			return *m, nil
		}
	case inspector.WorkflowChecklist:
		if im.checklistReport != nil {
			im.checklistIdx = 0
			m.screen = screenInspectorChecklistResults
			return *m, nil
		}
	}
	return im.startWorkflow(m, workflow.Kind)
}

func (im *inspectorModel) updateWorkflowPlaceholder(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "l":
		if im.currentWorkflow().Kind == inspector.WorkflowChecklist {
			return im.openChecklistPicker(m)
		}
	case "enter":
		if im.currentWorkflow().Kind == inspector.WorkflowChecklist {
			return im.openChecklistPicker(m)
		}
		m.screen = screenInspectorHome
	case "q", "esc":
		m.screen = screenInspectorHome
	}
	return *m, nil
}

func (im *inspectorModel) updateResults(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		im.selectedFinding = nil
		m.screen = screenInspectorHome
	case "up", "k":
		im.idx = previousListIndex(im.idx, len(im.findings))
	case "down", "j":
		im.idx = nextListIndex(im.idx, len(im.findings))
	case "enter":
		if len(im.findings) > 0 && im.idx < len(im.findings) {
			selected := im.findings[im.idx]
			im.selectedFinding = &selected
			m.screen = screenInspectorFindingDetail
		}
	case "r":
		return im.startWorkflow(m, inspector.WorkflowSecurity)
	case "1", "2", "3", "4", "5":
		idx := int(msg.String()[0] - '1')
		if idx >= 0 && idx < len(inspectorSeverityFilters) {
			im.severityFilter = inspectorSeverityFilters[idx]
			im.applySeverityFilter()
		}
	}
	return *m, nil
}

func (im *inspectorModel) updateFindingDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenInspectorResults
	case "r":
		return im.startWorkflow(m, inspector.WorkflowSecurity)
	}
	return *m, nil
}

func (im *inspectorModel) updateChecklistResults(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		im.selectedChecklistResult = nil
		m.screen = screenInspectorHome
	case "l":
		return im.openChecklistPicker(m)
	case "a":
		return im.openChecklistAdd(m)
	case "up", "k":
		if im.checklistReport != nil {
			im.checklistIdx = previousListIndex(im.checklistIdx, len(im.checklistReport.Results))
		}
	case "down", "j":
		if im.checklistReport != nil {
			im.checklistIdx = nextListIndex(im.checklistIdx, len(im.checklistReport.Results))
		}
	case "enter":
		if im.checklistReport != nil && len(im.checklistReport.Results) > 0 && im.checklistIdx < len(im.checklistReport.Results) {
			selected := im.checklistReport.Results[im.checklistIdx]
			im.selectedChecklistResult = &selected
			m.screen = screenInspectorChecklistDetail
		}
	case "r":
		return im.startWorkflow(m, inspector.WorkflowChecklist)
	}
	return *m, nil
}

func (im *inspectorModel) updateChecklistDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenInspectorChecklistResults
	case "l":
		return im.openChecklistPicker(m)
	case "a":
		return im.openChecklistAdd(m)
	case "r":
		return im.startWorkflow(m, inspector.WorkflowChecklist)
	}
	return *m, nil
}

func (im *inspectorModel) updateChecklistPicker(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterInspectorChecklistFiles); handled {
		return *m, cmd
	}

	switch msg.String() {
	case "q", "esc":
		m.screen = screenInspectorHome
	case "up", "k":
		im.checklistFileIdx = previousListIndex(im.checklistFileIdx, len(im.filteredChecklistFiles))
	case "down", "j":
		im.checklistFileIdx = nextListIndex(im.checklistFileIdx, len(im.filteredChecklistFiles))
	case "/":
		return *m, m.activateFilter(filterInspectorChecklistFiles)
	case "enter":
		if len(im.filteredChecklistFiles) == 0 || im.checklistFileIdx >= len(im.filteredChecklistFiles) {
			return *m, nil
		}

		selected := im.filteredChecklistFiles[im.checklistFileIdx]
		if selected.IsDir {
			if err := im.loadChecklistPickerEntries(m, selected.Path); err != nil {
				im.checklistError = err.Error()
			}
			return *m, nil
		}

		if _, err := inspector.LoadChecklist(selected.Path); err != nil {
			im.checklistError = err.Error()
			return *m, nil
		}

		im.checklistPath = selected.Path
		im.checklistDir = filepath.Dir(selected.Path)
		im.checklistReport = nil
		im.selectedChecklistResult = nil
		im.checklistIdx = 0
		im.checklistError = ""
		im.refreshWorkflows()
		im.workflowIdx = im.checklistWorkflowIndex()
		return im.startChecklistScan(m)
	}

	return *m, nil
}

func (im *inspectorModel) startWorkflow(m *Model, kind inspector.WorkflowKind) (tea.Model, tea.Cmd) {
	switch kind {
	case inspector.WorkflowChecklist:
		return im.startChecklistScan(m)
	case inspector.WorkflowSecurity:
		fallthrough
	default:
		return im.startScan(m)
	}
}

func (im *inspectorModel) startScan(m *Model) (tea.Model, tea.Cmd) {
	im.selectedFinding = nil
	m.screen = screenInspectorScanning
	m.loadingSpinner = newLoadingSpinner()
	return *m, tea.Batch(m.loadingSpinner.Tick, im.loadSecurityScan(*m))
}

func (im *inspectorModel) startChecklistScan(m *Model) (tea.Model, tea.Cmd) {
	im.selectedChecklistResult = nil
	m.screen = screenInspectorScanning
	m.loadingSpinner = newLoadingSpinner()
	return *m, tea.Batch(m.loadingSpinner.Tick, im.loadChecklistScan(*m))
}

func (im *inspectorModel) loadSecurityScan(m Model) tea.Cmd {
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

func (im *inspectorModel) loadChecklistScan(m Model) tea.Cmd {
	return func() tea.Msg {
		checklistPath := strings.TrimSpace(im.checklistPath)
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

func (im *inspectorModel) applySeverityFilter() {
	im.idx = 0
	if im.report == nil {
		im.findings = nil
		return
	}

	var filtered []inspector.SecurityFinding
	for _, finding := range im.report.Findings {
		if finding.MatchesSeverity(im.severityFilter) {
			filtered = append(filtered, finding)
		}
	}
	im.findings = filtered
}

func (im inspectorModel) viewHome(m Model) string {
	selected := im.currentWorkflow()

	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle("Inspector Mode"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Cross-service workflows with inspection-focused chrome and shared AWS context."))
	b.WriteString("\n\n")

	for i, workflow := range im.workflows {
		cursor := "  "
		nameStyle := normalStyle
		badgeStyle := inspectorReadyStyle
		if !workflow.Available {
			badgeStyle = inspectorPlannedStyle
		}
		if i == im.workflowIdx {
			cursor = "> "
			nameStyle = inspectorSelectedStyle
		}

		row := fmt.Sprintf("%s%-22s %s", cursor, workflow.Title, badgeStyle.Render("["+workflow.StatusLabel()+"]"))
		panel.WriteString(nameStyle.Render(row))
		panel.WriteString("\n")
		if i == im.workflowIdx {
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
		if im.report != nil {
			panel.WriteString(dimStyle.Render(fmt.Sprintf(
				"  Last scan: %s • findings:%d • warnings:%d",
				im.report.ScannedAt.Local().Format("2006-01-02 15:04:05"),
				len(im.report.Findings),
				len(im.report.Warnings),
			)))
			panel.WriteString("\n")
		}
		panel.WriteString(inspectorAccentStyle.Render("  Enter opens the latest findings or starts a fresh scan. Press r to force a new scan."))
	case inspector.WorkflowChecklist:
		if selected.Available {
			panel.WriteString(normalStyle.Render("  Run the configured YAML checklist against the current AWS context."))
			panel.WriteString("\n")
			panel.WriteString(dimStyle.Render(fmt.Sprintf("  Checklist file: %s", im.checklistPath)))
			panel.WriteString("\n")
			if im.checklistReport != nil {
				panel.WriteString(dimStyle.Render(fmt.Sprintf(
					"  Last run: %s • pass:%d • fail:%d",
					im.checklistReport.ScannedAt.Local().Format("2006-01-02 15:04:05"),
					im.checklistReport.PassedCount,
					im.checklistReport.FailedCount,
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

func (im inspectorModel) viewWorkflowPlaceholder(m Model) string {
	workflow := im.currentWorkflow()

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

func (im inspectorModel) viewChecklistPicker(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle("Checklist File Picker"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Browse folders and choose a .yaml or .yml file to load into Checklist Inspector."))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Directory: " + im.checklistDir))
	if strings.TrimSpace(im.checklistPath) != "" {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("Loaded: " + im.checklistPath))
	}
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterInspectorChecklistFiles))
	b.WriteString("\n\n")

	if im.checklistError != "" {
		panel.WriteString(errorStyle.Render("  " + im.checklistError))
		panel.WriteString("\n\n")
	}

	if len(im.filteredChecklistFiles) == 0 {
		panel.WriteString(dimStyle.Render("  No checklist files or folders match the current filter"))
		panel.WriteString("\n")
	} else {
		typeCol := lipgloss.NewStyle().Width(8)
		panel.WriteString(dimStyle.Render("  " + typeCol.Render("TYPE") + "NAME"))
		panel.WriteString("\n")

		visibleLines := max(m.height-14, 5)
		start := 0
		if im.checklistFileIdx >= visibleLines {
			start = im.checklistFileIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(im.filteredChecklistFiles))

		for i := start; i < end; i++ {
			entry := im.filteredChecklistFiles[i]
			cursor := "  "
			textStyle := normalStyle
			if i == im.checklistFileIdx {
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
			if entry.Path == im.checklistPath {
				name += " [loaded]"
			}

			row := cursor +
				typeCol.Inherit(dimStyle).Render(entryType) +
				textStyle.Render(m.renderHighlightedValue(filterInspectorChecklistFiles, name))
			panel.WriteString(row)
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d entries", len(im.filteredChecklistFiles), len(im.checklistFiles))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: open/load • esc: Inspector mode • H: home"))
	return b.String()
}

func (im inspectorModel) viewScanning(m Model) string {
	var title string
	var description string

	switch im.currentWorkflow().Kind {
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
	switch im.currentWorkflow().Kind {
	case inspector.WorkflowChecklist:
		b.WriteString(inspectorTitleStyle.Render(fmt.Sprintf("%s Running checklist expectations...", m.loadingSpinner.View())))
	default:
		b.WriteString(inspectorTitleStyle.Render(fmt.Sprintf("%s Running built-in rule packs...", m.loadingSpinner.View())))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(description))
	return b.String()
}

func (im inspectorModel) viewResults(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle("Security Inspector Findings"))
	b.WriteString("\n")

	if im.report != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf(
			"Scanned: %s  Rule Packs: %d  Findings: %d",
			im.report.ScannedAt.Local().Format("2006-01-02 15:04:05"),
			im.report.ScannerCount,
			len(im.report.Findings),
		)))
		b.WriteString("\n")
	}

	b.WriteString(im.renderSeveritySelector())
	b.WriteString("\n\n")

	if im.report != nil && len(im.report.Warnings) > 0 {
		panel.WriteString(errorStyle.Render(fmt.Sprintf("Warnings: %d rule pack(s) reported errors", len(im.report.Warnings))))
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render("  " + im.report.Warnings[0]))
		panel.WriteString("\n\n")
	}

	if len(im.findings) == 0 {
		panel.WriteString(dimStyle.Render("  No matching findings"))
		if im.report != nil && len(im.report.Findings) == 0 && im.report.ScannerCount == 0 {
			panel.WriteString("\n")
			panel.WriteString(dimStyle.Render("  No built-in rule packs are registered yet."))
		}
		panel.WriteString("\n")
	} else {
		resourceWidth := 24
		for _, finding := range im.findings {
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
		if im.idx >= visibleLines {
			start = im.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(im.findings))

		for i := start; i < end; i++ {
			finding := im.findings[i]
			cursor := "  "
			textStyle := normalStyle
			if i == im.idx {
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

func (im inspectorModel) viewFindingDetail(m Model) string {
	if im.selectedFinding == nil {
		return ""
	}

	finding := im.selectedFinding
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

func (im inspectorModel) viewChecklistResults(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle("Checklist Inspector Results"))
	b.WriteString("\n")

	if im.checklistReport != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf(
			"Checklist: %s  Scanned: %s  Pass: %d  Fail: %d",
			im.checklistReport.ChecklistName,
			im.checklistReport.ScannedAt.Local().Format("2006-01-02 15:04:05"),
			im.checklistReport.PassedCount,
			im.checklistReport.FailedCount,
		)))
		b.WriteString("\n")
		if im.checklistReport.SourcePath != "" {
			b.WriteString(dimStyle.Render("Source: " + im.checklistReport.SourcePath))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	if im.checklistReport == nil || len(im.checklistReport.Results) == 0 {
		panel.WriteString(dimStyle.Render("  No checklist results"))
		panel.WriteString("\n")
	} else {
		typeWidth := 16
		resourceWidth := 24
		for _, result := range im.checklistReport.Results {
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
		if im.checklistIdx >= visibleLines {
			start = im.checklistIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(im.checklistReport.Results))

		for i := start; i < end; i++ {
			result := im.checklistReport.Results[i]
			cursor := "  "
			textStyle := normalStyle
			if i == im.checklistIdx {
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

func (im inspectorModel) viewChecklistDetail(m Model) string {
	if im.selectedChecklistResult == nil {
		return ""
	}

	result := im.selectedChecklistResult
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

func (im inspectorModel) renderSeveritySelector() string {
	parts := make([]string, 0, len(inspectorSeverityFilters))
	for idx, severity := range inspectorSeverityFilters {
		label := fmt.Sprintf("%d:%s", idx+1, severity.Label())
		if severity == im.severityFilter {
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
