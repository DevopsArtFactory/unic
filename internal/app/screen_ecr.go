package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/clipboard"
	awsservice "unic/internal/services/aws"
)

type ecrModel struct {
	repositories         []awsservice.ECRRepository
	filteredRepositories []awsservice.ECRRepository
	repositoryIdx        int
	selectedRepository   *awsservice.ECRRepository
	images               []awsservice.ECRImage
	filteredImages       []awsservice.ECRImage
	imageIdx             int
	selectedImage        *awsservice.ECRImage
	copyMsg              string
	loginRegistryURI     string
	loginDockerCommand   string
	loginPodmanCommand   string
}

func newECRModel() ecrModel {
	return ecrModel{}
}

func (em *ecrModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(em.loadRepositories(*m))
}

func (em *ecrModel) StartLogin(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(em.loadLoginCommands(*m))
}

func (em *ecrModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case ecrRepositoriesLoadedMsg:
		em.repositories = msg.repositories
		em.filteredRepositories = msg.repositories
		em.repositoryIdx = 0
		em.selectedRepository = nil
		em.images = nil
		em.filteredImages = nil
		em.selectedImage = nil
		em.copyMsg = ""
		m.screen = screenECRRepositoryList
		return *m, nil, true
	case ecrImagesLoadedMsg:
		if em.selectedRepository == nil || em.selectedRepository.Name != msg.repository {
			return *m, nil, true
		}
		em.images = msg.images
		em.filteredImages = msg.images
		em.imageIdx = 0
		em.selectedImage = nil
		em.copyMsg = ""
		m.resetFilter(filterECRImages)
		m.screen = screenECRImageList
		return *m, nil, true
	case ecrLoginResolvedMsg:
		em.loginRegistryURI = msg.registryURI
		em.loginDockerCommand = msg.dockerCommand
		em.loginPodmanCommand = msg.podmanCommand
		em.copyMsg = ""
		m.screen = screenECRLoginHelper
		return *m, nil, true
	}
	return *m, nil, false
}

func (em *ecrModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenECRRepositoryList:
		newM, cmd := em.updateRepositoryList(m, msg)
		return newM, cmd, true
	case screenECRRepositoryDetail:
		newM, cmd := em.updateRepositoryDetail(m, msg)
		return newM, cmd, true
	case screenECRImageList:
		newM, cmd := em.updateImageList(m, msg)
		return newM, cmd, true
	case screenECRImageDetail:
		newM, cmd := em.updateImageDetail(m, msg)
		return newM, cmd, true
	case screenECRLoginHelper:
		newM, cmd := em.updateLoginHelper(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (em ecrModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenECRRepositoryList:
		return em.viewRepositoryList(m), true
	case screenECRRepositoryDetail:
		return em.viewRepositoryDetail(m), true
	case screenECRImageList:
		return em.viewImageList(m), true
	case screenECRImageDetail:
		return em.viewImageDetail(m), true
	case screenECRLoginHelper:
		return em.viewLoginHelper(m), true
	default:
		return "", false
	}
}

func (em *ecrModel) ApplyFilter(m *Model, target filterTarget) bool {
	switch target {
	case filterECRRepositories:
		em.filteredRepositories = applyFilter(em.repositories, m.filterValue(target))
		em.repositoryIdx = 0
	case filterECRImages:
		em.filteredImages = applyFilter(em.images, m.filterValue(target))
		em.imageIdx = 0
	default:
		return false
	}
	return true
}

func (em *ecrModel) updateRepositoryList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterECRRepositories); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterECRRepositories)
	case "up", "k":
		if em.repositoryIdx > 0 {
			em.repositoryIdx--
		}
	case "down", "j":
		if em.repositoryIdx < len(em.filteredRepositories)-1 {
			em.repositoryIdx++
		}
	case "/":
		return *m, m.activateFilter(filterECRRepositories)
	case "r":
		return m.startLoading(em.loadRepositories(*m))
	case "d":
		if len(em.filteredRepositories) > 0 && em.repositoryIdx < len(em.filteredRepositories) {
			selected := em.filteredRepositories[em.repositoryIdx]
			em.selectedRepository = &selected
			m.screen = screenECRRepositoryDetail
		}
	case "enter":
		if len(em.filteredRepositories) > 0 && em.repositoryIdx < len(em.filteredRepositories) {
			selected := em.filteredRepositories[em.repositoryIdx]
			em.selectedRepository = &selected
			return m.startLoading(em.loadImages(*m, selected.Name))
		}
	}
	return *m, nil
}

func (em *ecrModel) updateRepositoryDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		em.selectedRepository = nil
		m.screen = screenECRRepositoryList
	case "r":
		return m.startLoading(em.loadRepositories(*m))
	}
	return *m, nil
}

func (em *ecrModel) updateImageList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterECRImages); handled {
		return *m, cmd
	}

	switch key {
	case "q":
		m.screen = screenFeatureList
		m.resetFilter(filterECRImages)
	case "esc":
		em.selectedImage = nil
		em.copyMsg = ""
		m.screen = screenECRRepositoryList
	case "up", "k":
		em.imageIdx = previousListIndex(em.imageIdx, len(em.filteredImages))
	case "down", "j":
		em.imageIdx = nextListIndex(em.imageIdx, len(em.filteredImages))
	case "/":
		return *m, m.activateFilter(filterECRImages)
	case "r":
		if em.selectedRepository != nil {
			return m.startLoading(em.loadImages(*m, em.selectedRepository.Name))
		}
	case "enter":
		if len(em.filteredImages) > 0 && em.imageIdx < len(em.filteredImages) {
			selected := em.filteredImages[em.imageIdx]
			em.selectedImage = &selected
			em.copyMsg = ""
			m.screen = screenECRImageDetail
		}
	}
	return *m, nil
}

func (em *ecrModel) updateImageDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		em.copyMsg = ""
		m.screen = screenECRImageList
	case "c":
		if em.selectedImage != nil {
			if err := clipboard.Copy(em.selectedImage.Digest); err != nil {
				em.copyMsg = fmt.Sprintf("Clipboard error: %s", err)
			} else {
				em.copyMsg = "Copied digest to clipboard"
			}
		}
	case "t":
		if em.selectedImage != nil {
			tag := em.selectedImage.CopyTagValue()
			if tag == "" {
				em.copyMsg = "No tag to copy"
				return *m, nil
			}
			if err := clipboard.Copy(tag); err != nil {
				em.copyMsg = fmt.Sprintf("Clipboard error: %s", err)
			} else {
				em.copyMsg = "Copied tag to clipboard"
			}
		}
	}
	return *m, nil
}

func (em *ecrModel) updateLoginHelper(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		em.copyMsg = ""
		m.screen = screenFeatureList
	case "r":
		return m.startLoading(em.loadLoginCommands(*m))
	case "c":
		em.copyLoginCommand("Docker", em.loginDockerCommand)
	case "p":
		em.copyLoginCommand("Podman", em.loginPodmanCommand)
	}
	return *m, nil
}

func (em *ecrModel) copyLoginCommand(runtime, command string) {
	if command == "" {
		em.copyMsg = fmt.Sprintf("No %s login command available", runtime)
		return
	}
	if err := clipboard.Copy(command); err != nil {
		em.copyMsg = fmt.Sprintf("Clipboard error: %s", err)
		return
	}
	em.copyMsg = fmt.Sprintf("Copied %s login command to clipboard", runtime)
}

func (em *ecrModel) loadLoginCommands(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		registryURI, _, err := awsservice.ResolvePrivateECRRegistryURI(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		docker, err := awsservice.BuildECRLoginCommand(registryURI, m.cfg.Region, awsservice.ECRRuntimeDocker)
		if err != nil {
			return errMsg{err: err}
		}
		podman, err := awsservice.BuildECRLoginCommand(registryURI, m.cfg.Region, awsservice.ECRRuntimePodman)
		if err != nil {
			return errMsg{err: err}
		}
		return ecrLoginResolvedMsg{
			registryURI:   registryURI,
			dockerCommand: withContextEnv(m.cfg.ContextName, docker),
			podmanCommand: withContextEnv(m.cfg.ContextName, podman),
		}
	}
}

// withContextEnv prefixes a copied command with the active context's shell
// exports. The registry URI is resolved with the unic context's credentials,
// but `aws ecr get-login-password` runs with the shell's ambient credentials —
// without this prefix the copied command could log in to a different account.
func withContextEnv(contextName, command string) string {
	if contextName == "" {
		return command
	}
	return fmt.Sprintf("eval \"$(unic env %s)\" && %s", contextName, command)
}

func (em ecrModel) viewLoginHelper(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("ECR Login Helper"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Registry  "))
	b.WriteString(normalStyle.Render(em.loginRegistryURI))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Docker"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  " + em.loginDockerCommand))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  Podman"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  " + em.loginPodmanCommand))
	b.WriteString("\n")
	if em.copyMsg != "" {
		b.WriteString("\n")
		b.WriteString(selectedStyle.Render("  " + em.copyMsg))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("c: copy docker • p: copy podman • r: refresh • esc: back • H: home"))
	return b.String()
}

func (em *ecrModel) loadRepositories(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
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

func (em *ecrModel) loadImages(m Model, repositoryName string) tea.Cmd {
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

		images, err := repo.ListECRImages(ctx, repositoryName)
		if err != nil {
			return errMsg{err: err}
		}
		return ecrImagesLoadedMsg{repository: repositoryName, images: images}
	}
}

func (em ecrModel) viewRepositoryList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("ECR Repositories"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterECRRepositories))
	b.WriteString("\n\n")

	if len(em.filteredRepositories) == 0 {
		panel.WriteString(dimStyle.Render("  No matching repositories"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if em.repositoryIdx >= visibleLines {
			start = em.repositoryIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filteredRepositories))

		for i := start; i < end; i++ {
			repo := em.filteredRepositories[i]
			cursor := "  "
			style := normalStyle
			if i == em.repositoryIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterECRRepositories, repo.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d repositories", len(em.filteredRepositories), len(em.repositories))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: images • d: detail • r: refresh • esc: back • H: home"))
	return b.String()
}

func (em ecrModel) viewRepositoryDetail(m Model) string {
	if em.selectedRepository == nil {
		return ""
	}
	repo := em.selectedRepository

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

func (em ecrModel) viewImageList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	repositoryName := ""
	if em.selectedRepository != nil {
		repositoryName = em.selectedRepository.Name
	}
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(fmt.Sprintf("ECR Images — %s", repositoryName)))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterECRImages))
	b.WriteString("\n\n")

	if len(em.filteredImages) == 0 {
		panel.WriteString(dimStyle.Render("  No matching images"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if em.imageIdx >= visibleLines {
			start = em.imageIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filteredImages))

		for i := start; i < end; i++ {
			image := em.filteredImages[i]
			cursor := "  "
			style := normalStyle
			if image.IsUntagged() || image.IsStale(time.Now()) {
				style = warningStyle
			}
			if i == em.imageIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterECRImages, image.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d images", len(em.filteredImages), len(em.images))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: detail • r: refresh • esc: repositories • H: home"))
	return b.String()
}

func (em ecrModel) viewImageDetail(m Model) string {
	if em.selectedImage == nil {
		return ""
	}
	image := em.selectedImage

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

	if em.copyMsg != "" {
		b.WriteString("\n")
		b.WriteString(selectedStyle.Render("  " + em.copyMsg))
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
