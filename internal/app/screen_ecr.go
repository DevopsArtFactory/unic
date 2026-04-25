package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func (m Model) handleECRMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case ecrRepositoriesLoadedMsg:
		m.ecrRepositories = msg.repositories
		m.filteredECRRepositories = msg.repositories
		m.ecrRepositoryIdx = 0
		m.screen = screenECRRepositoryList
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) updateECRRepositoryList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterECRRepositories); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterECRRepositories)
	case "up", "k":
		if m.ecrRepositoryIdx > 0 {
			m.ecrRepositoryIdx--
		}
	case "down", "j":
		if m.ecrRepositoryIdx < len(m.filteredECRRepositories)-1 {
			m.ecrRepositoryIdx++
		}
	case "/":
		return m, m.activateFilter(filterECRRepositories)
	case "r":
		return m.startLoading(m.loadECRRepositories())
	case "enter":
		if len(m.filteredECRRepositories) > 0 && m.ecrRepositoryIdx < len(m.filteredECRRepositories) {
			selected := m.filteredECRRepositories[m.ecrRepositoryIdx]
			m.selectedECRRepository = &selected
			m.screen = screenECRRepositoryDetail
		}
	}
	return m, nil
}

func (m Model) updateECRRepositoryDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.selectedECRRepository = nil
		m.screen = screenECRRepositoryList
	case "r":
		return m.startLoading(m.loadECRRepositories())
	}
	return m, nil
}

func (m Model) loadECRRepositories() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		repositories, err := repo.ListECRRepositories(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(repositories) == 0 {
			return errMsg{err: fmt.Errorf("no ECR repositories found")}
		}
		return ecrRepositoriesLoadedMsg{repositories: repositories}
	}
}

func (m Model) viewECRRepositoryList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("ECR Repositories"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterECRRepositories))
	b.WriteString("\n\n")

	if len(m.filteredECRRepositories) == 0 {
		panel.WriteString(dimStyle.Render("  No matching repositories"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if m.ecrRepositoryIdx >= visibleLines {
			start = m.ecrRepositoryIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredECRRepositories))

		for i := start; i < end; i++ {
			repo := m.filteredECRRepositories[i]
			cursor := "  "
			style := normalStyle
			if i == m.ecrRepositoryIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterECRRepositories, repo.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d repositories", len(m.filteredECRRepositories), len(m.ecrRepositories))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: detail • r: refresh • esc: back • H: home"))
	return b.String()
}

func (m Model) viewECRRepositoryDetail() string {
	if m.selectedECRRepository == nil {
		return ""
	}
	repo := m.selectedECRRepository

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("ECR Repository Detail"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Name", normalStyle.Render(repo.Name)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("URI", normalStyle.Render(repo.URI)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Registry", normalStyle.Render(repo.RegistryID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Scan on Push", normalStyle.Render(yesNoLabel(repo.ScanOnPush))))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Tag Mutability", normalStyle.Render(repo.TagMutability)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Encryption", normalStyle.Render(repo.Encryption)))
	b.WriteString("\n")
	if repo.ARN != "" {
		b.WriteString(renderDetailLine("ARN", normalStyle.Render(repo.ARN)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("r: refresh • esc: back • H: home"))
	return b.String()
}

func yesNoLabel(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}
