package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/clipboard"
	awsservice "unic/internal/services/aws"
)

func (m Model) handleECRMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case ecrRepositoriesLoadedMsg:
		m.ecrRepositories = msg.repositories
		m.filteredECRRepositories = msg.repositories
		m.ecrRepositoryIdx = 0
		m.selectedECRRepository = nil
		m.ecrImages = nil
		m.filteredECRImages = nil
		m.selectedECRImage = nil
		m.ecrCopyMsg = ""
		m.screen = screenECRRepositoryList
		return m, nil, true
	case ecrImagesLoadedMsg:
		if m.selectedECRRepository == nil || m.selectedECRRepository.Name != msg.repository {
			return m, nil, true
		}
		m.ecrImages = msg.images
		m.filteredECRImages = msg.images
		m.ecrImageIdx = 0
		m.selectedECRImage = nil
		m.ecrCopyMsg = ""
		m.resetFilter(filterECRImages)
		m.screen = screenECRImageList
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
	case "d":
		if len(m.filteredECRRepositories) > 0 && m.ecrRepositoryIdx < len(m.filteredECRRepositories) {
			selected := m.filteredECRRepositories[m.ecrRepositoryIdx]
			m.selectedECRRepository = &selected
			m.screen = screenECRRepositoryDetail
		}
	case "enter":
		if len(m.filteredECRRepositories) > 0 && m.ecrRepositoryIdx < len(m.filteredECRRepositories) {
			selected := m.filteredECRRepositories[m.ecrRepositoryIdx]
			m.selectedECRRepository = &selected
			return m.startLoading(m.loadECRImages(selected.Name))
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

func (m Model) updateECRImageList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterECRImages); handled {
		return m, cmd
	}

	switch key {
	case "q":
		m.screen = screenFeatureList
		m.resetFilter(filterECRImages)
	case "esc":
		m.selectedECRImage = nil
		m.ecrCopyMsg = ""
		m.screen = screenECRRepositoryList
	case "up", "k":
		m.ecrImageIdx = previousListIndex(m.ecrImageIdx, len(m.filteredECRImages))
	case "down", "j":
		m.ecrImageIdx = nextListIndex(m.ecrImageIdx, len(m.filteredECRImages))
	case "/":
		return m, m.activateFilter(filterECRImages)
	case "r":
		if m.selectedECRRepository != nil {
			return m.startLoading(m.loadECRImages(m.selectedECRRepository.Name))
		}
	case "enter":
		if len(m.filteredECRImages) > 0 && m.ecrImageIdx < len(m.filteredECRImages) {
			selected := m.filteredECRImages[m.ecrImageIdx]
			m.selectedECRImage = &selected
			m.ecrCopyMsg = ""
			m.screen = screenECRImageDetail
		}
	}
	return m, nil
}

func (m Model) updateECRImageDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.ecrCopyMsg = ""
		m.screen = screenECRImageList
	case "c":
		if m.selectedECRImage != nil {
			if err := clipboard.Copy(m.selectedECRImage.Digest); err != nil {
				m.ecrCopyMsg = fmt.Sprintf("Clipboard error: %s", err)
			} else {
				m.ecrCopyMsg = "Copied digest to clipboard"
			}
		}
	case "t":
		if m.selectedECRImage != nil {
			tag := m.selectedECRImage.CopyTagValue()
			if tag == "" {
				m.ecrCopyMsg = "No tag to copy"
				return m, nil
			}
			if err := clipboard.Copy(tag); err != nil {
				m.ecrCopyMsg = fmt.Sprintf("Clipboard error: %s", err)
			} else {
				m.ecrCopyMsg = "Copied tag to clipboard"
			}
		}
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

func (m Model) loadECRImages(repositoryName string) tea.Cmd {
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

		images, err := repo.ListECRImages(ctx, repositoryName)
		if err != nil {
			return errMsg{err: err}
		}
		return ecrImagesLoadedMsg{repository: repositoryName, images: images}
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
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: images • d: detail • r: refresh • esc: back • H: home"))
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

func (m Model) viewECRImageList() string {
	var b strings.Builder
	var panel strings.Builder
	repositoryName := ""
	if m.selectedECRRepository != nil {
		repositoryName = m.selectedECRRepository.Name
	}
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECR Images — %s", repositoryName)))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterECRImages))
	b.WriteString("\n\n")

	if len(m.filteredECRImages) == 0 {
		panel.WriteString(dimStyle.Render("  No matching images"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if m.ecrImageIdx >= visibleLines {
			start = m.ecrImageIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredECRImages))

		for i := start; i < end; i++ {
			image := m.filteredECRImages[i]
			cursor := "  "
			style := normalStyle
			if image.IsUntagged() || image.IsStale(time.Now()) {
				style = warningStyle
			}
			if i == m.ecrImageIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterECRImages, image.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d images", len(m.filteredECRImages), len(m.ecrImages))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: detail • r: refresh • esc: repositories • H: home"))
	return b.String()
}

func (m Model) viewECRImageDetail() string {
	if m.selectedECRImage == nil {
		return ""
	}
	image := m.selectedECRImage

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("ECR Image Detail"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Repository", normalStyle.Render(image.RepositoryName)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Tags", normalStyle.Render(image.TagsText())))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Digest", normalStyle.Render(image.Digest)))
	b.WriteString("\n")
	pushed := "-"
	if !image.PushedAt.IsZero() {
		pushed = image.PushedAt.Format("2006-01-02 15:04:05 MST")
	}
	b.WriteString(renderDetailLine("Pushed", normalStyle.Render(pushed)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Size", normalStyle.Render(awsservice.FormatBytes(image.SizeBytes))))
	b.WriteString("\n")
	signal := image.CleanupSignal(time.Now())
	if signal == "current" {
		signal = successStyle.Render(signal)
	} else {
		signal = warningStyle.Render(signal)
	}
	b.WriteString(renderDetailLine("Cleanup", signal))
	b.WriteString("\n")

	if m.ecrCopyMsg != "" {
		b.WriteString("\n")
		b.WriteString(selectedStyle.Render("  " + m.ecrCopyMsg))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("c: copy digest • t: copy tag • esc: images • H: home"))
	return b.String()
}

func yesNoLabel(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}
