package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

type fisModel struct {
	templates         []awsservice.FISExperimentTemplate
	filteredTemplates []awsservice.FISExperimentTemplate
	templateIdx       int
	selectedTemplate  *awsservice.FISExperimentTemplate
	templateScroll    int

	experiments         []awsservice.FISExperiment
	filteredExperiments []awsservice.FISExperiment
	experimentIdx       int
	selectedExperiment  *awsservice.FISExperiment
	experimentScroll    int
	historyTemplateID   string
}

func newFISModel() fisModel {
	return fisModel{}
}

func (fm *fisModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(fm.loadTemplates(*m))
}

func (fm *fisModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case fisTemplatesLoadedMsg:
		fm.templates = msg.templates
		fm.filteredTemplates = msg.templates
		fm.templateIdx = 0
		fm.selectedTemplate = nil
		fm.templateScroll = 0
		m.screen = screenFISTemplateList
		return *m, nil, true
	case fisTemplateDetailLoadedMsg:
		fm.selectedTemplate = msg.template
		fm.templateScroll = 0
		m.screen = screenFISTemplateDetail
		return *m, nil, true
	case fisExperimentsLoadedMsg:
		fm.historyTemplateID = msg.templateID
		fm.experiments = msg.experiments
		fm.filteredExperiments = msg.experiments
		fm.experimentIdx = 0
		fm.selectedExperiment = nil
		fm.experimentScroll = 0
		m.resetFilter(filterFISExperiments)
		m.screen = screenFISExperimentList
		return *m, nil, true
	case fisExperimentDetailLoadedMsg:
		fm.selectedExperiment = msg.experiment
		fm.experimentScroll = 0
		m.screen = screenFISExperimentDetail
		return *m, nil, true
	}
	return *m, nil, false
}

func (fm *fisModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenFISTemplateList:
		newM, cmd := fm.updateTemplateList(m, msg)
		return newM, cmd, true
	case screenFISTemplateDetail:
		newM, cmd := fm.updateTemplateDetail(m, msg)
		return newM, cmd, true
	case screenFISExperimentList:
		newM, cmd := fm.updateExperimentList(m, msg)
		return newM, cmd, true
	case screenFISExperimentDetail:
		newM, cmd := fm.updateExperimentDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (fm fisModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenFISTemplateList:
		return fm.viewTemplateList(m), true
	case screenFISTemplateDetail:
		return fm.viewTemplateDetail(m), true
	case screenFISExperimentList:
		return fm.viewExperimentList(m), true
	case screenFISExperimentDetail:
		return fm.viewExperimentDetail(m), true
	default:
		return "", false
	}
}

func (fm *fisModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target == filterFISTemplates {
		fm.filteredTemplates = applyFilter(fm.templates, m.filterValue(target))
		fm.templateIdx = 0
		return true
	}
	if target == filterFISExperiments {
		fm.filteredExperiments = applyFilter(fm.experiments, m.filterValue(target))
		fm.experimentIdx = 0
		return true
	}
	return false
}

func (fm *fisModel) selectedTemplateID() string {
	if fm.selectedTemplate != nil {
		return fm.selectedTemplate.ID
	}
	if len(fm.filteredTemplates) > 0 && fm.templateIdx < len(fm.filteredTemplates) {
		return fm.filteredTemplates[fm.templateIdx].ID
	}
	return ""
}

func (fm *fisModel) startExperimentHistory(m *Model, templateID string) (tea.Model, tea.Cmd) {
	if templateID == "" {
		return *m, nil
	}
	return m.startLoading(fm.loadExperiments(*m, templateID))
}

func (fm *fisModel) startAllExperimentHistory(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(fm.loadExperiments(*m, ""))
}

func (fm *fisModel) currentExperiment() *awsservice.FISExperiment {
	if len(fm.filteredExperiments) == 0 || fm.experimentIdx < 0 || fm.experimentIdx >= len(fm.filteredExperiments) {
		return nil
	}
	experiment := fm.filteredExperiments[fm.experimentIdx]
	return &experiment
}

func (fm *fisModel) updateTemplateList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterFISTemplates); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterFISTemplates)
	case "up", "k":
		fm.templateIdx = previousListIndex(fm.templateIdx, len(fm.filteredTemplates))
	case "down", "j":
		fm.templateIdx = nextListIndex(fm.templateIdx, len(fm.filteredTemplates))
	case "/":
		return *m, m.activateFilter(filterFISTemplates)
	case "r":
		return m.startLoading(fm.loadTemplates(*m))
	case "H":
		return fm.startAllExperimentHistory(m)
	case "h":
		return fm.startExperimentHistory(m, fm.selectedTemplateID())
	case "enter":
		if len(fm.filteredTemplates) > 0 && fm.templateIdx < len(fm.filteredTemplates) {
			selected := fm.filteredTemplates[fm.templateIdx]
			fm.selectedTemplate = &selected
			return m.startLoading(fm.loadTemplateDetail(*m, selected.ID))
		}
	}
	return *m, nil
}

func (fm *fisModel) updateTemplateDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenFISTemplateList
	case "up", "k":
		if fm.templateScroll > 0 {
			fm.templateScroll--
		}
	case "down", "j":
		if fm.selectedTemplate != nil {
			fm.templateScroll = min(fm.templateScroll+1, max(len(fm.templateDetailLines(*fm.selectedTemplate))-1, 0))
		}
	case "pgup":
		fm.templateScroll = max(fm.templateScroll-10, 0)
	case "pgdown":
		if fm.selectedTemplate != nil {
			fm.templateScroll = min(fm.templateScroll+10, max(len(fm.templateDetailLines(*fm.selectedTemplate))-1, 0))
		}
	case "h":
		return fm.startExperimentHistory(m, fm.selectedTemplateID())
	case "r":
		if fm.selectedTemplate != nil {
			return m.startLoading(fm.loadTemplateDetail(*m, fm.selectedTemplate.ID))
		}
	}
	return *m, nil
}

func (fm *fisModel) updateExperimentList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterFISExperiments); handled {
		return *m, cmd
	}

	switch key {
	case "q":
		m.screen = screenFeatureList
		m.resetFilter(filterFISExperiments)
	case "esc":
		fm.selectedExperiment = nil
		m.screen = screenFISTemplateList
	case "up", "k":
		fm.experimentIdx = previousListIndex(fm.experimentIdx, len(fm.filteredExperiments))
	case "down", "j":
		fm.experimentIdx = nextListIndex(fm.experimentIdx, len(fm.filteredExperiments))
	case "/":
		return *m, m.activateFilter(filterFISExperiments)
	case "r":
		return m.startLoading(fm.loadExperiments(*m, fm.historyTemplateID))
	case "enter":
		if experiment := fm.currentExperiment(); experiment != nil {
			fm.selectedExperiment = experiment
			return m.startLoading(fm.loadExperimentDetail(*m, experiment.ID))
		}
	}
	return *m, nil
}

func (fm *fisModel) updateExperimentDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := fm.experimentDetailLines()
	visibleLines := max(m.height-7, 5)
	maxOffset := max(len(lines)-visibleLines, 0)

	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenFISExperimentList
	case "up", "k":
		if fm.experimentScroll > 0 {
			fm.experimentScroll--
		}
	case "down", "j":
		if fm.experimentScroll < maxOffset {
			fm.experimentScroll++
		}
	case "pgup":
		fm.experimentScroll = max(fm.experimentScroll-visibleLines, 0)
	case "pgdown":
		fm.experimentScroll = min(fm.experimentScroll+visibleLines, maxOffset)
	case "r":
		if fm.selectedExperiment != nil {
			return m.startLoading(fm.loadExperimentDetail(*m, fm.selectedExperiment.ID))
		}
	}
	return *m, nil
}

func (fm *fisModel) loadTemplates(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		templates, err := repo.ListFISExperimentTemplates(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return fisTemplatesLoadedMsg{templates: templates}
	}
}

func (fm *fisModel) loadTemplateDetail(m Model, id string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}

		template, err := repo.GetFISExperimentTemplate(ctx, id)
		if err != nil {
			return errMsg{err: err}
		}
		return fisTemplateDetailLoadedMsg{template: template}
	}
}

func (fm *fisModel) loadExperiments(m Model, templateID string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}

		experiments, err := repo.ListFISExperiments(ctx, templateID)
		if err != nil {
			return errMsg{err: err}
		}
		return fisExperimentsLoadedMsg{templateID: templateID, experiments: experiments}
	}
}

func (fm *fisModel) loadExperimentDetail(m Model, id string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}

		experiment, err := repo.GetFISExperiment(ctx, id)
		if err != nil {
			return errMsg{err: err}
		}
		return fisExperimentDetailLoadedMsg{experiment: experiment}
	}
}

func (fm fisModel) viewTemplateList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("FIS Experiment Templates"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterFISTemplates))
	b.WriteString("\n\n")

	if len(fm.filteredTemplates) == 0 {
		panel.WriteString(dimStyle.Render("  No matching experiment templates"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if fm.templateIdx >= visibleLines {
			start = fm.templateIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(fm.filteredTemplates))

		for i := start; i < end; i++ {
			template := fm.filteredTemplates[i]
			cursor := "  "
			style := normalStyle
			if i == fm.templateIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterFISTemplates, template.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d templates", len(fm.filteredTemplates), len(fm.templates))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: detail • h: template history • H: all history • r: refresh • esc: back"))
	return b.String()
}

func (fm fisModel) viewTemplateDetail(m Model) string {
	if fm.selectedTemplate == nil {
		return ""
	}
	template := *fm.selectedTemplate
	lines := fm.templateDetailLines(template)
	visibleLines := max(m.height-7, 5)
	start := min(fm.templateScroll, max(len(lines)-1, 0))
	end := min(start+visibleLines, len(lines))

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("FIS Experiment Template Detail"))
	b.WriteString("\n\n")
	for _, line := range lines[start:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}
	if len(lines) > visibleLines {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d-%d/%d lines", start+1, end, len(lines))))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("↑/↓: scroll • pgup/pgdn: page • h: history • r: refresh • esc: templates • q: feature list"))
	return b.String()
}

func (fm fisModel) viewExperimentList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	title := "FIS Experiment History"
	if fm.historyTemplateID != "" {
		title = fmt.Sprintf("FIS Experiment History — %s", fm.historyTemplateID)
	}
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterFISExperiments))
	b.WriteString("\n\n")

	if len(fm.filteredExperiments) == 0 {
		panel.WriteString(dimStyle.Render("  No matching experiment runs"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if fm.experimentIdx >= visibleLines {
			start = fm.experimentIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(fm.filteredExperiments))

		for i := start; i < end; i++ {
			experiment := fm.filteredExperiments[i]
			cursor := "  "
			style := normalStyle
			if experiment.NeedsAttention() {
				style = warningStyle
			}
			if i == fm.experimentIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterFISExperiments, experiment.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d experiments", len(fm.filteredExperiments), len(fm.experiments))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: detail • r: refresh • esc: templates • q: feature list"))
	return b.String()
}

func (fm fisModel) viewExperimentDetail(m Model) string {
	lines := fm.experimentDetailLines()
	visibleLines := max(m.height-7, 5)
	start := min(fm.experimentScroll, max(len(lines)-visibleLines, 0))
	end := min(start+visibleLines, len(lines))

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("FIS Experiment Detail"))
	b.WriteString("\n\n")
	for _, line := range lines[start:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}
	if len(lines) > visibleLines {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d-%d/%d lines", start+1, end, len(lines))))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("↑/↓: scroll • pgup/pgdn: page • r: refresh • esc: history • q: feature list"))
	return b.String()
}

func (fm fisModel) templateDetailLines(template awsservice.FISExperimentTemplate) []string {
	preview := template.SafeRunPreview()
	risk := successStyle.Render(preview.RiskLevel)
	if preview.HasWarnings() {
		risk = warningStyle.Render(preview.RiskLevel)
	}

	lines := []string{
		renderDetailLine("ID", normalStyle.Render(template.ID)),
		renderDetailLine("Description", normalStyle.Render(defaultDash(template.Description))),
		renderDetailLine("Role ARN", normalStyle.Render(defaultDash(template.RoleARN))),
		renderDetailLine("ARN", normalStyle.Render(defaultDash(template.ARN))),
	}
	if !template.CreatedAt.IsZero() {
		lines = append(lines, renderDetailLine("Created", normalStyle.Render(template.CreatedAt.Format("2006-01-02 15:04:05 MST"))))
	}
	if !template.LastUpdatedAt.IsZero() {
		lines = append(lines, renderDetailLine("Updated", normalStyle.Render(template.LastUpdatedAt.Format("2006-01-02 15:04:05 MST"))))
	}
	lines = append(lines, renderDetailLine("Tags", normalStyle.Render(defaultDash(formatDetailMap(template.Tags)))))

	lines = append(lines, "", selectedStyle.Render("Safe Run Preview"))
	lines = append(lines, renderDetailLine("Risk", risk))
	lines = append(lines, renderDetailLine("Targets", normalStyle.Render(fmt.Sprintf("%d target group(s)", preview.TargetCount))))
	lines = append(lines, renderDetailLine("Actions", normalStyle.Render(fmt.Sprintf("%d action(s)", preview.ActionCount))))
	lines = append(lines, renderDetailLine("Stop Conditions", normalStyle.Render(fmt.Sprintf("%d active", preview.StopConditionCount))))
	lines = append(lines, renderDetailLine("IAM Role", normalStyle.Render(defaultDash(template.RoleARN))))
	if len(preview.TargetModes) > 0 {
		lines = append(lines, renderDetailLine("Selection Modes", normalStyle.Render(strings.Join(preview.TargetModes, ", "))))
	}
	if len(preview.TargetSummaries) > 0 {
		lines = append(lines, "  "+dimStyle.Render("Blast radius"))
		for _, summary := range preview.TargetSummaries {
			lines = append(lines, "    "+normalStyle.Render(summary))
		}
	}
	if len(preview.Warnings) == 0 {
		lines = append(lines, "  "+successStyle.Render("No obvious missing safeguards detected"))
	} else {
		lines = append(lines, "  "+warningStyle.Render("Review before any future run"))
		for _, warning := range preview.Warnings {
			lines = append(lines, "    "+warningStyle.Render(warning))
		}
	}
	lines = append(lines, "  "+dimStyle.Render(fmt.Sprintf("Future execution must type %q to confirm.", preview.ConfirmationToken)))

	lines = append(lines, "", selectedStyle.Render("Targets"))
	if len(template.Targets) == 0 {
		lines = append(lines, dimStyle.Render("  No targets"))
	} else {
		for _, target := range template.Targets {
			lines = append(lines, "  "+normalStyle.Render(target.Summary()))
			if len(target.ResourceARNs) > 0 {
				lines = append(lines, "    "+dimStyle.Render("Resource ARNs: "+strings.Join(target.ResourceARNs, ", ")))
			}
			if len(target.Parameters) > 0 {
				lines = append(lines, "    "+dimStyle.Render("Parameters: "+formatDetailMap(target.Parameters)))
			}
			for _, filter := range target.Filters {
				lines = append(lines, "    "+dimStyle.Render("Filter: "+filter.Summary()))
			}
		}
	}

	lines = append(lines, "", selectedStyle.Render("Actions"))
	if len(template.Actions) == 0 {
		lines = append(lines, dimStyle.Render("  No actions"))
	} else {
		for _, action := range template.Actions {
			lines = append(lines, "  "+normalStyle.Render(action.Summary()))
			if action.Description != "" {
				lines = append(lines, "    "+dimStyle.Render(action.Description))
			}
			if len(action.Parameters) > 0 {
				lines = append(lines, "    "+dimStyle.Render("Parameters: "+formatDetailMap(action.Parameters)))
			}
		}
	}

	lines = append(lines, "", selectedStyle.Render("Stop Conditions"))
	if len(template.StopConditions) == 0 {
		lines = append(lines, dimStyle.Render("  No stop conditions"))
	} else {
		for _, condition := range template.StopConditions {
			lines = append(lines, "  "+normalStyle.Render(condition.Summary()))
		}
	}
	return lines
}

func (fm fisModel) experimentDetailLines() []string {
	if fm.selectedExperiment == nil {
		return []string{dimStyle.Render("No experiment selected")}
	}
	experiment := *fm.selectedExperiment
	status := normalStyle.Render(defaultDash(experiment.Status))
	if experiment.NeedsAttention() {
		status = warningStyle.Render(defaultDash(experiment.Status))
	} else if strings.EqualFold(experiment.Status, "completed") {
		status = successStyle.Render(defaultDash(experiment.Status))
	}
	lines := []string{
		renderDetailLine("ID", normalStyle.Render(experiment.ID)),
		renderDetailLine("Template", normalStyle.Render(defaultDash(experiment.TemplateID))),
		renderDetailLine("Status", status),
		renderDetailLine("Reason", normalStyle.Render(defaultDash(experiment.StopSummary()))),
		renderDetailLine("Duration", normalStyle.Render(experiment.DurationLabel())),
		renderDetailLine("ARN", normalStyle.Render(defaultDash(experiment.ARN))),
	}
	if !experiment.CreatedAt.IsZero() {
		lines = append(lines, renderDetailLine("Created", normalStyle.Render(experiment.CreatedAt.Format("2006-01-02 15:04:05 MST"))))
	}
	if !experiment.StartedAt.IsZero() {
		lines = append(lines, renderDetailLine("Started", normalStyle.Render(experiment.StartedAt.Format("2006-01-02 15:04:05 MST"))))
	}
	if !experiment.EndedAt.IsZero() {
		lines = append(lines, renderDetailLine("Ended", normalStyle.Render(experiment.EndedAt.Format("2006-01-02 15:04:05 MST"))))
	}
	if experiment.ErrorCode != "" || experiment.ErrorLocation != "" || experiment.ErrorAccountID != "" {
		lines = append(lines, "", selectedStyle.Render("Failure"))
		lines = append(lines, renderDetailLine("Code", warningStyle.Render(defaultDash(experiment.ErrorCode))))
		lines = append(lines, renderDetailLine("Location", normalStyle.Render(defaultDash(experiment.ErrorLocation))))
		lines = append(lines, renderDetailLine("Account", normalStyle.Render(defaultDash(experiment.ErrorAccountID))))
	}
	lines = append(lines, renderDetailLine("Tags", normalStyle.Render(defaultDash(formatDetailMap(experiment.Tags)))))

	lines = append(lines, "", selectedStyle.Render("Actions"))
	if len(experiment.Actions) == 0 {
		lines = append(lines, dimStyle.Render("  No actions"))
	} else {
		for _, action := range experiment.Actions {
			line := action.Summary()
			if strings.EqualFold(action.Status, "failed") || strings.EqualFold(action.Status, "cancelled") {
				lines = append(lines, "  "+warningStyle.Render(line))
			} else {
				lines = append(lines, "  "+normalStyle.Render(line))
			}
			if !action.StartedAt.IsZero() || !action.EndedAt.IsZero() {
				lines = append(lines, "    "+dimStyle.Render(fmt.Sprintf("Started:%s  Ended:%s", formatOptionalTime(action.StartedAt), formatOptionalTime(action.EndedAt))))
			}
			if len(action.Parameters) > 0 {
				lines = append(lines, "    "+dimStyle.Render("Parameters: "+formatDetailMap(action.Parameters)))
			}
		}
	}

	lines = append(lines, "", selectedStyle.Render("Targets"))
	if len(experiment.Targets) == 0 {
		lines = append(lines, dimStyle.Render("  No targets"))
	} else {
		for _, target := range experiment.Targets {
			lines = append(lines, "  "+normalStyle.Render(target.Summary()))
		}
	}

	lines = append(lines, "", selectedStyle.Render("Stop Conditions"))
	if len(experiment.StopConditions) == 0 {
		lines = append(lines, dimStyle.Render("  No stop conditions"))
	} else {
		for _, condition := range experiment.StopConditions {
			lines = append(lines, "  "+normalStyle.Render(condition.Summary()))
		}
	}
	return lines
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02 15:04:05 MST")
}

func defaultDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func formatDetailMap(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for key, value := range values {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
