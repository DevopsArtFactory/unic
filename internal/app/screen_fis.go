package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func (m Model) handleFISMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case fisTemplatesLoadedMsg:
		m.fisTemplates = msg.templates
		m.filteredFISTemplates = msg.templates
		m.fisTemplateIdx = 0
		m.selectedFISTemplate = nil
		m.fisTemplateScroll = 0
		m.screen = screenFISTemplateList
		return m, nil, true
	case fisTemplateDetailLoadedMsg:
		m.selectedFISTemplate = msg.template
		m.fisTemplateScroll = 0
		m.screen = screenFISTemplateDetail
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) updateFISTemplateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterFISTemplates); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterFISTemplates)
	case "up", "k":
		m.fisTemplateIdx = previousListIndex(m.fisTemplateIdx, len(m.filteredFISTemplates))
	case "down", "j":
		m.fisTemplateIdx = nextListIndex(m.fisTemplateIdx, len(m.filteredFISTemplates))
	case "/":
		return m, m.activateFilter(filterFISTemplates)
	case "r":
		return m.startLoading(m.loadFISTemplates())
	case "enter":
		if len(m.filteredFISTemplates) > 0 && m.fisTemplateIdx < len(m.filteredFISTemplates) {
			selected := m.filteredFISTemplates[m.fisTemplateIdx]
			m.selectedFISTemplate = &selected
			return m.startLoading(m.loadFISTemplateDetail(selected.ID))
		}
	}
	return m, nil
}

func (m Model) updateFISTemplateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenFISTemplateList
	case "up", "k":
		if m.fisTemplateScroll > 0 {
			m.fisTemplateScroll--
		}
	case "down", "j":
		if m.selectedFISTemplate != nil {
			m.fisTemplateScroll = min(m.fisTemplateScroll+1, max(len(m.fisTemplateDetailLines(*m.selectedFISTemplate))-1, 0))
		}
	case "pgup":
		m.fisTemplateScroll = max(m.fisTemplateScroll-10, 0)
	case "pgdown":
		if m.selectedFISTemplate != nil {
			m.fisTemplateScroll = min(m.fisTemplateScroll+10, max(len(m.fisTemplateDetailLines(*m.selectedFISTemplate))-1, 0))
		}
	case "r":
		if m.selectedFISTemplate != nil {
			return m.startLoading(m.loadFISTemplateDetail(m.selectedFISTemplate.ID))
		}
	}
	return m, nil
}

func (m Model) loadFISTemplates() tea.Cmd {
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

func (m Model) loadFISTemplateDetail(id string) tea.Cmd {
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

func (m Model) viewFISTemplateList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("FIS Experiment Templates"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterFISTemplates))
	b.WriteString("\n\n")

	if len(m.filteredFISTemplates) == 0 {
		panel.WriteString(dimStyle.Render("  No matching experiment templates"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if m.fisTemplateIdx >= visibleLines {
			start = m.fisTemplateIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredFISTemplates))

		for i := start; i < end; i++ {
			template := m.filteredFISTemplates[i]
			cursor := "  "
			style := normalStyle
			if i == m.fisTemplateIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterFISTemplates, template.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d templates", len(m.filteredFISTemplates), len(m.fisTemplates))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: detail • r: refresh • esc: back • H: home"))
	return b.String()
}

func (m Model) viewFISTemplateDetail() string {
	if m.selectedFISTemplate == nil {
		return ""
	}
	template := *m.selectedFISTemplate
	lines := m.fisTemplateDetailLines(template)
	visibleLines := max(m.height-7, 5)
	start := min(m.fisTemplateScroll, max(len(lines)-1, 0))
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

func (m Model) fisTemplateDetailLines(template awsservice.FISExperimentTemplate) []string {
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
