package app

import (
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
		m.ctxList = m.contextsWithFavoriteState(msg.contexts)
		m.filteredCtxList = append([]config.ContextInfo(nil), m.ctxList...)
		m.sortFavoriteContextsFirst(m.filteredCtxList)
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
		// Startup loads only populate the context list; screen transitions are
		// owned by the Init sequence (screenReadyMsg / the boot splash flow), so
		// a late background load never overrides a screen the user navigated to
		// in the meantime (e.g. Settings). Explicit loads (C shortcut, post-add
		// reloads) surface the picker directly, but never interrupt the splash.
		if !msg.startup && m.screen != screenBootup {
			m.screen = screenContextPicker
		}
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
		m.cloudFormation = newCloudFormationModel()
		m.resetFilter(filterCloudFormationStacks)
		if m.pendingView != nil {
			// A saved view triggered this switch: continue the jump now that
			// the new context is active.
			view := *m.pendingView
			m.pendingView = nil
			newM, cmd := m.jumpToView(view)
			return newM, tea.Batch(tea.ClearScreen, cmd), true
		}
		if m.ctxPrevScreen == screenCloudFormationStackList || m.ctxPrevScreen == screenCloudFormationStackDetail {
			m.ctxPrevScreen = screenFeatureList
		}
		m.screen = m.ctxPrevScreen
		return m, tea.ClearScreen, true

	case regionSwitchedMsg:
		m.cfg.Region = msg.region
		m.awsRepo = msg.repo
		// Region-scoped feature state may contain resources from the previous
		// region, so return to the service catalog after switching.
		m.screen = screenServiceList
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
	return m.loadContextsCmd(false)
}

// loadStartupContexts loads contexts during the initial Init sequence. Unlike
// loadContexts, the resulting message is flagged as a startup load so its
// handler won't bounce the user away from a screen they navigated to while the
// load was in flight.
func (m Model) loadStartupContexts() tea.Cmd {
	return m.loadContextsCmd(true)
}

func (m Model) loadContextsCmd(startup bool) tea.Cmd {
	return func() tea.Msg {
		contexts, err := config.Contexts(m.configPath)
		if err != nil || len(contexts) == 0 {
			return contextsLoadedMsg{startup: startup}
		}
		return contextsLoadedMsg{contexts: contexts, startup: startup}
	}
}

func (m Model) updateContextPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.isFiltering(filterContexts) && key == "esc" {
		if strings.TrimSpace(m.filterValue(filterContexts)) != "" {
			m.resetFilter(filterContexts)
			m.syncContextTable()
			return m, nil
		}
		m.deactivateFilter()
		return m, nil
	}

	if m.isFiltering(filterContexts) {
		switch key {
		case "ctrl+s":
			selected, ok := m.selectedContextInfo()
			if !ok {
				return m, nil
			}
			return m.beginContextSetup(selected)
		case "ctrl+y":
			selected, ok := m.selectedContextInfo()
			if !ok {
				return m, nil
			}
			return m.beginContextExport(selected)
		}
	}

	if cmd, handled := m.updateSharedFilter(msg, filterContexts); handled {
		m.syncContextTable()
		return m, cmd
	}

	if shouldStartContextIncrementalFilter(msg) {
		return m.startContextIncrementalFilter(msg)
	}

	switch key {
	case "q":
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
	case "up", "k":
		m.moveContextTableUp()
	case "down", "j":
		m.moveContextTableDown()
	case "enter":
		cursor := m.contextTable.Cursor()
		if len(m.filteredCtxList) > 0 && cursor >= 0 && cursor < len(m.filteredCtxList) {
			selected := m.filteredCtxList[cursor]
			if m.ctxPrevScreen == screenLoading &&
				(m.loadingReturnScreen == screenCloudFormationStackList || m.loadingReturnScreen == screenCloudFormationStackDetail) {
				m.ctxPrevScreen = screenFeatureList
			}
			m.pendingContextName = selected.Name
			return m.startLoading(m.switchContext(selected.Name))
		}
	case "s":
		selected, ok := m.selectedContextInfo()
		if !ok {
			return m, nil
		}
		return m.beginContextSetup(selected)
	case "y":
		selected, ok := m.selectedContextInfo()
		if !ok {
			return m, nil
		}
		return m.beginContextExport(selected)
	case "f":
		selected, ok := m.selectedContextInfo()
		if !ok {
			return m, nil
		}
		if err := m.toggleFavoriteContext(selected.Name); err != nil {
			m.errMsg = err.Error()
			m.screen = screenError
		}
	case "u":
		return m.beginContextUnset()
	case "a":
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
			check, err := contextCheckSSOSessionFn(cfg)
			if err != nil {
				return errMsg{err: err}
			}
			if !check.LoginRequired {
				return contextFinalizeSwitchFn(m)()
			}

			cmd, cleanup, err := contextBuildSSOLoginCmdFn(cfg)
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
		ctx := m.commandContext()
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
	compact := m.contextPickerCompact()
	b.WriteString(titleStyle.Render("Select Context"))
	b.WriteString("\n")
	if compact {
		b.WriteString("\n")
	} else {
		b.WriteString(renderDetailLine("UNIC current", displayContextName(m.cfg.ContextName)))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Shell env", m.displayShellEnvContext()))
		b.WriteString("\n\n")
	}

	if filter := m.renderFilterValue(filterContexts); filter != "" {
		b.WriteString(filter)
		b.WriteString("\n\n")
	}

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
	if m.isFiltering(filterContexts) {
		if compact {
			b.WriteString(m.renderHelpBar("filter • ↑/↓ choose • enter finish • esc clear"))
		} else {
			b.WriteString(m.renderHelpBar("filtering: type search • ↑/↓ choose • enter finish • esc clear • ctrl+y copy env • ctrl+s setup"))
		}
		return b.String()
	}
	if compact {
		b.WriteString(m.renderHelpBar("↑/↓ nav • enter switch • / filter • f fav • a add • q: quit"))
		return b.String()
	}
	if m.cfg.ContextName != "" {
		b.WriteString(m.renderHelpBar("↑/↓: navigate • type: filter • /: filter • enter: switch • s: setup • y: copy env • f: favorite • u: unset • a: add • S: settings • esc: clear/back • q: quit"))
	} else {
		b.WriteString(m.renderHelpBar("↑/↓: navigate • type: filter • /: filter • enter: switch • s: setup • y: copy env • f: favorite • u: unset • a: add • S: settings • q: quit"))
	}
	return b.String()
}

func shouldStartContextIncrementalFilter(msg tea.KeyMsg) bool {
	if len(msg.Runes) != 1 {
		return false
	}
	switch msg.String() {
	case "/", "q", "s", "y", "f", "u", "a", "S", "j", "k":
		return false
	}
	r := msg.Runes[0]
	return r >= 32 && r != 127
}

func (m Model) startContextIncrementalFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	value := string(msg.Runes)
	m.activeFilter = filterContexts
	m.filterTI.SetValue(value)
	m.filterTI.CursorEnd()
	m.syncFilterInputWidth()
	m.storeFilterValue(filterContexts, value)
	m.applyFilterTarget(filterContexts)
	m.syncContextTable()
	return m, m.filterTI.Focus()
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
