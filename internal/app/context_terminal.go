package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/auth"
	"unic/internal/clipboard"
	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

type contextSSOAccountsLoadedMsg struct {
	base     config.ContextInfo
	accounts []awsservice.SSOAccount
}

type contextSSORolesLoadedMsg struct {
	base    config.ContextInfo
	account awsservice.SSOAccount
	roles   []awsservice.SSORole
}

type contextTerminalActionDoneMsg struct {
	title   string
	message string
}

var (
	contextBuildEnvExportsFn     = auth.BuildEnvExports
	contextBuildEnvCleanupFn     = auth.BuildEnvCleanupCommands
	contextSetCurrentFn          = config.SetCurrent
	contextUnsetCurrentFn        = config.UnsetCurrent
	contextLoadNamedContextFn    = config.LoadNamedContext
	contextCopyClipboardFn       = clipboard.Copy
	contextListSSOAccountsFn     = auth.ListSSOContextAccounts
	contextListSSORolesFn        = auth.ListSSOContextRoles
	contextResolveSSOSelectionFn = auth.ResolveSSOContextSelection
	contextDetectEnvFn           = auth.DetectEnvContext
	contextCheckSSOSessionFn     = awsservice.CheckSSOSession
	contextBuildSSOLoginCmdFn    = awsservice.BuildSSOLoginCmd
	contextFinalizeSwitchFn      = func(m Model) tea.Cmd { return m.doFinalizeContextSwitch() }
)

func (m Model) selectedContextInfo() (config.ContextInfo, bool) {
	cursor := m.contextTable.Cursor()
	if len(m.filteredCtxList) == 0 || cursor < 0 || cursor >= len(m.filteredCtxList) {
		return config.ContextInfo{}, false
	}
	return m.filteredCtxList[cursor], true
}

func (m Model) beginContextSetup(selected config.ContextInfo) (tea.Model, tea.Cmd) {
	if auth.IsBaseSSOContext(selected) {
		details := []string{
			renderDetailLine("Context", selected.Name),
			renderDetailLine("Auth", "sso"),
		}
		return m.startLoadingWithMessage("Loading SSO accounts...", details, m.loadSSOContextAccounts(selected))
	}

	return m.startLoadingWithMessage(
		"Preparing terminal exports...",
		[]string{renderDetailLine("Context", selected.Name)},
		m.setupSelectedContextForTerminal(selected.Name),
	)
}

func (m Model) beginContextExport(selected config.ContextInfo) (tea.Model, tea.Cmd) {
	return m.startLoadingWithMessage(
		"Copying environment exports...",
		[]string{renderDetailLine("Context", selected.Name)},
		m.copySelectedContextExports(selected.Name),
	)
}

func (m Model) beginContextUnset() (tea.Model, tea.Cmd) {
	return m.startLoadingWithMessage(
		"Clearing terminal context...",
		[]string{renderDetailLine("Action", "Unset shell exports and current context")},
		m.unsetTerminalContext(),
	)
}

func (m Model) loadSSOContextAccounts(selected config.ContextInfo) tea.Cmd {
	return func() tea.Msg {
		accounts, err := contextListSSOAccountsFn(context.Background(), m.configPath, selected)
		if err != nil {
			return errMsg{err: err}
		}
		if len(accounts) == 0 {
			return errMsg{err: fmt.Errorf("no SSO accounts available for %q", selected.Name)}
		}
		return contextSSOAccountsLoadedMsg{base: selected, accounts: accounts}
	}
}

func (m Model) loadSSOContextRoles(base config.ContextInfo, account awsservice.SSOAccount) tea.Cmd {
	return func() tea.Msg {
		roles, err := contextListSSORolesFn(context.Background(), m.configPath, base, account.ID)
		if err != nil {
			return errMsg{err: err}
		}
		if len(roles) == 0 {
			return errMsg{err: fmt.Errorf("no SSO roles available for account %s", account.ID)}
		}
		return contextSSORolesLoadedMsg{base: base, account: account, roles: roles}
	}
}

func (m Model) setupResolvedSSOContextForTerminal() tea.Cmd {
	return func() tea.Msg {
		if m.contextSSORoleIdx < 0 || m.contextSSORoleIdx >= len(m.contextSSORoles) {
			return errMsg{err: fmt.Errorf("invalid role selection")}
		}
		finalName, err := contextResolveSSOSelectionFn(m.configPath, m.contextSSOBase, m.contextSSOAccount, m.contextSSORoles[m.contextSSORoleIdx])
		if err != nil {
			return errMsg{err: err}
		}
		return m.setupSelectedContextForTerminal(finalName)()
	}
}

func (m Model) setupSelectedContextForTerminal(name string) tea.Cmd {
	return func() tea.Msg {
		if err := contextSetCurrentFn(m.configPath, name); err != nil {
			return errMsg{err: err}
		}

		cfg, err := contextLoadNamedContextFn(m.configPath, name)
		if err != nil {
			return errMsg{err: err}
		}

		exports, err := contextBuildEnvExportsFn(context.Background(), cfg)
		if err != nil {
			return errMsg{err: err}
		}
		if err := contextCopyClipboardFn(exports); err != nil {
			return errMsg{err: fmt.Errorf("failed to copy exports to clipboard: %w", err)}
		}
		return contextTerminalActionDoneMsg{
			title:   "CONTEXT SET UP",
			message: fmt.Sprintf("[%s] has been set up and copied. Goodbye!", name),
		}
	}
}

func (m Model) copySelectedContextExports(name string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := contextLoadNamedContextFn(m.configPath, name)
		if err != nil {
			return errMsg{err: err}
		}

		exports, err := contextBuildEnvExportsFn(context.Background(), cfg)
		if err != nil {
			return errMsg{err: err}
		}
		if err := contextCopyClipboardFn(exports); err != nil {
			return errMsg{err: fmt.Errorf("failed to copy exports to clipboard: %w", err)}
		}
		return contextTerminalActionDoneMsg{
			title:   "EXPORTS COPIED",
			message: fmt.Sprintf("[%s] exports have been copied. Goodbye!", name),
		}
	}
}

func (m Model) unsetTerminalContext() tea.Cmd {
	return func() tea.Msg {
		if err := contextUnsetCurrentFn(m.configPath); err != nil {
			return errMsg{err: err}
		}

		if err := contextCopyClipboardFn(contextBuildEnvCleanupFn()); err != nil {
			return errMsg{err: fmt.Errorf("failed to copy cleanup commands to clipboard: %w", err)}
		}
		return contextTerminalActionDoneMsg{
			title:   "CONTEXT CLEARED",
			message: "Shell context has been cleared and cleanup commands copied. Goodbye!",
		}
	}
}

func (m Model) updateContextSSOAccountList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenContextPicker
	case "esc":
		m.screen = screenContextPicker
	case "up", "k":
		m.contextSSOAccountIdx = previousListIndex(m.contextSSOAccountIdx, len(m.contextSSOAccounts))
	case "down", "j":
		m.contextSSOAccountIdx = nextListIndex(m.contextSSOAccountIdx, len(m.contextSSOAccounts))
	case "enter":
		if len(m.contextSSOAccounts) == 0 {
			return m, nil
		}
		account := m.contextSSOAccounts[m.contextSSOAccountIdx]
		details := []string{
			renderDetailLine("Context", m.contextSSOBase.Name),
			renderDetailLine("Account", formatSSOAccountLabel(account)),
		}
		return m.startLoadingWithMessage("Loading SSO roles...", details, m.loadSSOContextRoles(m.contextSSOBase, account))
	}
	return m, nil
}

func (m Model) updateContextSSORoleList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenContextPicker
	case "esc":
		m.screen = screenContextSSOAccountList
	case "up", "k":
		m.contextSSORoleIdx = previousListIndex(m.contextSSORoleIdx, len(m.contextSSORoles))
	case "down", "j":
		m.contextSSORoleIdx = nextListIndex(m.contextSSORoleIdx, len(m.contextSSORoles))
	case "enter":
		if len(m.contextSSORoles) == 0 {
			return m, nil
		}
		details := []string{
			renderDetailLine("Context", m.contextSSOBase.Name),
			renderDetailLine("Account", formatSSOAccountLabel(m.contextSSOAccount)),
			renderDetailLine("Role", m.contextSSORoles[m.contextSSORoleIdx].Name),
		}
		return m.startLoadingWithMessage("Preparing terminal exports...", details, m.setupResolvedSSOContextForTerminal())
	}
	return m, nil
}

func (m Model) viewContextSSOAccountList() string {
	var b strings.Builder
	var panel strings.Builder

	b.WriteString(titleStyle.Render("Select SSO Account"))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Context", m.contextSSOBase.Name))
	b.WriteString("\n\n")

	visibleLines := max(m.height-9, 3)
	start := 0
	if m.contextSSOAccountIdx >= visibleLines {
		start = m.contextSSOAccountIdx - visibleLines + 1
	}
	end := min(start+visibleLines, len(m.contextSSOAccounts))

	for i := start; i < end; i++ {
		account := m.contextSSOAccounts[i]
		cursor := "  "
		style := normalStyle
		if i == m.contextSSOAccountIdx {
			cursor = "> "
			style = selectedStyle
		}
		panel.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, formatSSOAccountLabel(account))))
		panel.WriteString("\n")
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • enter: select • esc: back • q: cancel"))
	return b.String()
}

func (m Model) viewContextSSORoleList() string {
	var b strings.Builder
	var panel strings.Builder

	b.WriteString(titleStyle.Render("Select SSO Role"))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Context", m.contextSSOBase.Name))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Account", formatSSOAccountLabel(m.contextSSOAccount)))
	b.WriteString("\n\n")

	visibleLines := max(m.height-10, 3)
	start := 0
	if m.contextSSORoleIdx >= visibleLines {
		start = m.contextSSORoleIdx - visibleLines + 1
	}
	end := min(start+visibleLines, len(m.contextSSORoles))

	for i := start; i < end; i++ {
		role := m.contextSSORoles[i]
		cursor := "  "
		style := normalStyle
		if i == m.contextSSORoleIdx {
			cursor = "> "
			style = selectedStyle
		}
		panel.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, role.Name)))
		panel.WriteString("\n")
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • enter: select • esc: back • q: cancel"))
	return b.String()
}

func formatSSOAccountLabel(account awsservice.SSOAccount) string {
	label := account.Name
	if strings.TrimSpace(label) == "" {
		label = account.ID
	}
	if account.Email != "" {
		return fmt.Sprintf("%s <%s> (%s)", label, account.Email, account.ID)
	}
	if label == account.ID {
		return label
	}
	return fmt.Sprintf("%s (%s)", label, account.ID)
}
