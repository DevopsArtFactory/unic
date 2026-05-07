package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

type fisModel struct {
	templates         []awsservice.FISExperimentTemplate
	filteredTemplates []awsservice.FISExperimentTemplate
	templateIdx       int
	selectedTemplate  *awsservice.FISExperimentTemplate
	templateScroll    int
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
	default:
		return "", false
	}
}

func (fm *fisModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterFISTemplates {
		return false
	}
	fm.filteredTemplates = applyFilter(fm.templates, m.filterValue(target))
	fm.templateIdx = 0
	return true
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
	case "r":
		if fm.selectedTemplate != nil {
			return m.startLoading(fm.loadTemplateDetail(*m, fm.selectedTemplate.ID))
		}
	}
	return *m, nil
}

func (fm *fisModel) loadTemplates(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		templates, err := repo.ListFISExperimentTemplates(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(templates) == 0 {
			return errMsg{err: fmt.Errorf("no FIS experiment templates found")}
		}
		return fisTemplatesLoadedMsg{templates: templates}
	}
}

func (fm *fisModel) loadTemplateDetail(m Model, id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
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
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: detail • r: refresh • esc: back • H: home"))
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
	b.WriteString(m.renderHelpBar("↑/↓: scroll • pgup/pgdn: page • r: refresh • esc: templates • q: feature list • H: home"))
	return b.String()
}

func (fm fisModel) templateDetailLines(template awsservice.FISExperimentTemplate) []string {
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
