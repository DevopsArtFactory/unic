package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/auth"
	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func (m Model) handleContextMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case contextsLoadedMsg:
		m.ctxList = msg.contexts
		m.filteredCtxList = msg.contexts
		m.ctxIdx = 0
		m.contextSSOBase = config.ContextInfo{}
		m.contextSSOAccounts = nil
		m.contextSSOAccountIdx = 0
		m.contextSSOAccount = awsservice.SSOAccount{}
		m.contextSSORoles = nil
		m.contextSSORoleIdx = 0
		m.resetFilter(filterContexts)
		envContext := contextDetectEnvFn(msg.contexts, os.Getenv)
		m.envContextName = envContext.Name
		m.envContextSource = envContext.Source
		m.envContextKnown = envContext.Known
		for i, ctx := range m.filteredCtxList {
			if ctx.Current {
				m.ctxIdx = i
				break
			}
		}
		m.syncContextTable()
		m.screen = screenContextPicker
		return m, nil, true

	case ssoLoginDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("SSO login failed: %s", msg.err)
			m.screen = screenError
			return m, tea.ClearScreen, true
		}
		return m, m.finalizeContextSwitch(), true

	case contextSwitchedMsg:
		m.cfg = msg.cfg
		m.callerIdentity = msg.identity
		m.awsRepo = nil
		m.screen = m.ctxPrevScreen
		return m, tea.ClearScreen, true

	case contextSSOAccountsLoadedMsg:
		m.contextSSOBase = msg.base
		m.contextSSOAccounts = msg.accounts
		m.contextSSOAccountIdx = 0
		m.screen = screenContextSSOAccountList
		return m, nil, true

	case contextSSORolesLoadedMsg:
		m.contextSSOBase = msg.base
		m.contextSSOAccount = msg.account
		m.contextSSORoles = msg.roles
		m.contextSSORoleIdx = 0
		m.screen = screenContextSSORoleList
		return m, nil, true

	case contextTerminalActionDoneMsg:
		m.exitTitle = msg.title
		m.exitMessage = msg.message
		m.screen = screenExitNotice
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) loadContexts() tea.Cmd {
	return func() tea.Msg {
		contexts, err := config.Contexts(m.configPath)
		if err != nil || len(contexts) == 0 {
			return contextsLoadedMsg{}
		}
		return contextsLoadedMsg{contexts: contexts}
	}
}

func (m Model) updateContextPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterContexts); handled {
		return m, cmd
	}

	if handled := m.updateIncrementalContextFilter(msg); handled {
		return m, nil
	}

	switch key {
	case "Q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		// If we have a valid config (mid-session C key), go back.
		// If initial launch, quit.
		if m.cfg.ContextName != "" {
			m.screen = m.ctxPrevScreen
			m.resetFilter(filterContexts)
		} else {
			m.quitting = true
			return m, tea.Quit
		}
	case "/":
		return m, m.activateFilter(filterContexts)
	case "enter":
		cursor := m.contextTable.Cursor()
		if len(m.filteredCtxList) > 0 && cursor >= 0 && cursor < len(m.filteredCtxList) {
			selected := m.filteredCtxList[cursor]
			m.pendingContextName = selected.Name
			return m.startLoading(m.switchContext(selected.Name))
		}
	case "S":
		selected, ok := m.selectedContextInfo()
		if !ok {
			return m, nil
		}
		return m.beginContextSetup(selected)
	case "Y":
		selected, ok := m.selectedContextInfo()
		if !ok {
			return m, nil
		}
		return m.beginContextExport(selected)
	case "U":
		return m.beginContextUnset()
	case "A":
		m.addStep = 0
		m.addAuthIdx = 0
		m.addFields = nil
		m.addFieldIdx = 0
		m.addInput = ""
		m.addValues = make(map[string]string)
		m.screen = screenContextAdd
	default:
		if len(m.filteredCtxList) == 0 {
			return m, nil
		}
		var cmd tea.Cmd
		m.contextTable, cmd = m.contextTable.Update(msg)
		if cursor := m.contextTable.Cursor(); cursor >= 0 {
			m.ctxIdx = cursor
		}
		return m, cmd
	}
	return m, nil
}

func (m *Model) updateIncrementalContextFilter(msg tea.KeyMsg) bool {
	query := m.filterValue(filterContexts)

	switch msg.String() {
	case "backspace":
		if query == "" {
			return false
		}
		m.storeFilterValue(filterContexts, trimLastRune(query))
		m.applyFilterTarget(filterContexts)
		return true
	case "esc":
		if query == "" {
			return false
		}
		m.resetFilter(filterContexts)
		return true
	}

	if !shouldTypeFilterContext(msg) {
		return false
	}

	m.storeFilterValue(filterContexts, query+string(msg.Runes))
	m.applyFilterTarget(filterContexts)
	return true
}

func shouldTypeFilterContext(msg tea.KeyMsg) bool {
	if len(msg.Runes) == 0 {
		return false
	}

	switch msg.String() {
	case "/", "A", "Q", "S", "U", "Y", "j", "k":
		return false
	default:
		return true
	}
}

func (m Model) switchContext(name string) tea.Cmd {
	return func() tea.Msg {
		if err := config.SetCurrent(m.configPath, name); err != nil {
			return errMsg{err: err}
		}

		cfg, err := config.Load(nil, nil, m.configPath)
		if err != nil {
			return errMsg{err: err}
		}

		// SSO needs interactive terminal — hand off via tea.ExecProcess
		if cfg.AuthType == config.AuthTypeSSO {
			cmd, cleanup, err := awsservice.BuildSSOLoginCmd(cfg)
			if err != nil {
				return errMsg{err: err}
			}
			return tea.ExecProcess(cmd, func(err error) tea.Msg {
				cleanup()
				return ssoLoginDoneMsg{err: err}
			})()
		}

		// Non-SSO: perform auth + finalize in one shot
		return m.doFinalizeContextSwitch()()
	}
}

func (m Model) finalizeContextSwitch() tea.Cmd {
	return m.doFinalizeContextSwitch()
}

func (m Model) doFinalizeContextSwitch() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load(nil, nil, m.configPath)
		if err != nil {
			return errMsg{err: err}
		}

		// Perform non-SSO auth action (credential check, assume role, etc.)
		if cfg.AuthType != config.AuthTypeSSO {
			if _, err := auth.PostSwitch(cfg); err != nil {
				return errMsg{err: err}
			}
		}

		// Get caller identity with new credentials
		ctx := context.Background()
		var identity *awsservice.CallerIdentity
		repo, err := awsservice.NewAwsRepository(ctx, cfg)
		if err == nil {
			identity, _ = repo.GetCallerIdentity(ctx)
		}

		return contextSwitchedMsg{
			cfg:      cfg,
			identity: identity,
		}
	}
}

func (m Model) viewContextPicker() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(titleStyle.Render("Select Context"))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("UNIC current", displayContextName(m.cfg.ContextName)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Shell env", m.displayShellEnvContext()))
	b.WriteString("\n\n")

	b.WriteString(m.renderFilterValue(filterContexts))
	b.WriteString("\n\n")

	if len(m.ctxList) == 0 {
		panel.WriteString(normalStyle.Render("  No contexts defined."))
		panel.WriteString("\n\n")
		panel.WriteString(dimStyle.Render("  Press 'a' to add your first context."))
		panel.WriteString("\n")
	} else if len(m.filteredCtxList) == 0 {
		panel.WriteString(dimStyle.Render("  No matching contexts"))
		panel.WriteString("\n")
	} else {
		panel.WriteString(m.contextTable.View())
		panel.WriteString("\n")
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	if m.cfg.ContextName != "" {
		b.WriteString(m.renderHelpBar("↑/↓: navigate • type: filter • /: edit filter • enter: switch • S: setup • Y: copy env • U: unset • A: add • esc: clear/back • Q: quit"))
	} else {
		b.WriteString(m.renderHelpBar("↑/↓: navigate • type: filter • /: edit filter • enter: switch • S: setup • Y: copy env • U: unset • A: add • esc: clear/quit • Q: quit"))
	}
	return b.String()
}

func displayContextName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "none"
	}
	return name
}

func (m Model) displayShellEnvContext() string {
	if strings.TrimSpace(m.envContextName) == "" {
		return "not detected"
	}

	summary := m.envContextName
	switch m.envContextSource {
	case auth.ContextEnvVar:
		if !m.envContextKnown {
			return summary + " (UNIC_CONTEXT, not in config)"
		}
		return summary + " (UNIC_CONTEXT)"
	case "AWS_PROFILE":
		if !m.envContextKnown {
			return summary + " (best effort from AWS_PROFILE)"
		}
		return summary + " (AWS_PROFILE)"
	default:
		return summary
	}
}
