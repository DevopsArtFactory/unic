package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
		m.ctxFilterInput = ""
		m.ctxFilterActive = false
		for i, ctx := range m.filteredCtxList {
			if ctx.Current {
				m.ctxIdx = i
				break
			}
		}
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

	if m.ctxFilterActive {
		newFilter, deactivate, changed := handleFilterKey(key, m.ctxFilterInput)
		m.ctxFilterInput = newFilter
		if deactivate {
			m.ctxFilterActive = false
		}
		if changed {
			m.filteredCtxList = applyFilter(m.ctxList, m.ctxFilterInput)
			m.ctxIdx = 0
		}
		return m, nil
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
			m.ctxFilterInput = ""
			m.filteredCtxList = m.ctxList
		} else {
			m.quitting = true
			return m, tea.Quit
		}
	case "up", "k":
		if m.ctxIdx > 0 {
			m.ctxIdx--
		}
	case "down", "j":
		if m.ctxIdx < len(m.filteredCtxList)-1 {
			m.ctxIdx++
		}
	case "/":
		m.ctxFilterActive = true
	case "enter":
		if len(m.filteredCtxList) > 0 && m.ctxIdx < len(m.filteredCtxList) {
			selected := m.filteredCtxList[m.ctxIdx]
			m.pendingContextName = selected.Name
			return m.startLoading(m.switchContext(selected.Name))
		}
	case "a":
		m.addStep = 0
		m.addAuthIdx = 0
		m.addFields = nil
		m.addFieldIdx = 0
		m.addInput = ""
		m.addValues = make(map[string]string)
		m.screen = screenContextAdd
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
	b.WriteString(titleStyle.Render("Select Context"))
	b.WriteString("\n")

	// Filter bar
	if m.ctxFilterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.ctxFilterInput)))
	} else if m.ctxFilterInput != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.ctxFilterInput)))
	}
	b.WriteString("\n\n")

	if len(m.ctxList) == 0 {
		b.WriteString(normalStyle.Render("  No contexts defined."))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("  Press 'a' to add your first context."))
		b.WriteString("\n")
	} else if len(m.filteredCtxList) == 0 {
		b.WriteString(dimStyle.Render("  No matching contexts"))
		b.WriteString("\n")
	} else {
		// Measure max widths for alignment
		maxName, maxRegion := 4, 6 // "NAME", "REGION"
		for _, ctx := range m.filteredCtxList {
			if len(ctx.Name) > maxName {
				maxName = len(ctx.Name)
			}
			if len(ctx.Region) > maxRegion {
				maxRegion = len(ctx.Region)
			}
		}

		nameCol := lipgloss.NewStyle().Width(maxName + 2)
		regionCol := lipgloss.NewStyle().Width(maxRegion + 2)

		// Header
		b.WriteString(dimStyle.Render("  " + nameCol.Render("NAME") + regionCol.Render("REGION") + "AUTH"))
		b.WriteString("\n")

		// overhead: title (1) + filter (1) + blank (1) + table header (1) + blank (1) + footer (1) = 6
		visibleLines := max(m.height-6, 3)
		start := 0
		if m.ctxIdx >= visibleLines {
			start = m.ctxIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredCtxList))

		for i := start; i < end; i++ {
			ctx := m.filteredCtxList[i]
			cursor := "  "
			style := normalStyle
			if i == m.ctxIdx {
				cursor = "> "
				style = selectedStyle
			}

			row := cursor + nameCol.Inherit(style).Render(ctx.Name) + regionCol.Inherit(style).Render(ctx.Region) + style.Render(ctx.AuthType)
			if ctx.Current {
				row += dimStyle.Render(" *")
			}
			b.WriteString(row)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.cfg.ContextName != "" {
		b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • enter: select • a: add • esc: back • q: quit"))
	} else {
		b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • enter: select • a: add • q: quit"))
	}
	return b.String()
}
