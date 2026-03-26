package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func (m Model) updateInstanceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// If filter is active, handle text input
	if m.filterActive {
		switch key {
		case "esc":
			m.filterActive = false
		case "enter":
			m.filterActive = false
		case "backspace":
			if len(m.filterInput) > 0 {
				m.filterInput = m.filterInput[:len(m.filterInput)-1]
				m.applyFilter()
			}
		default:
			if len(key) == 1 {
				m.filterInput += key
				m.applyFilter()
			}
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.filterInput = ""
		m.filtered = m.instances
		m.instIdx = 0
	case "up", "k":
		if m.instIdx > 0 {
			m.instIdx--
		}
	case "down", "j":
		if m.instIdx < len(m.filtered)-1 {
			m.instIdx++
		}
	case "/":
		m.filterActive = true
	case "enter":
		if len(m.filtered) > 0 && m.instIdx < len(m.filtered) {
			return m, m.startSSMSession(m.filtered[m.instIdx])
		}
	}
	return m, nil
}

func (m *Model) applyFilter() {
	if m.filterInput == "" {
		m.filtered = m.instances
	} else {
		query := strings.ToLower(m.filterInput)
		var result []awsservice.EC2Instance
		for _, inst := range m.instances {
			if strings.Contains(inst.FilterText(), query) {
				result = append(result, inst)
			}
		}
		m.filtered = result
	}
	m.instIdx = 0
}

func (m Model) loadInstances() tea.Cmd {
	return func() tea.Msg {
		if err := awsservice.CheckPluginInstalled(); err != nil {
			return errMsg{err: err}
		}

		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		instances, err := repo.ListRunningInstances(ctx)
		if err != nil {
			return errMsg{err: err}
		}

		if len(instances) == 0 {
			return errMsg{err: fmt.Errorf("no running EC2 instances found")}
		}

		return instancesLoadedMsg{instances: instances}
	}
}

func (m Model) startSSMSession(inst awsservice.EC2Instance) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Initialize AWS repo if needed
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}

		sess, endpoint, err := repo.StartSession(ctx, inst.InstanceID)
		if err != nil {
			return errMsg{err: err}
		}

		cmd, err := awsservice.BuildPluginCommand(sess, repo.Region, repo.Profile, inst.InstanceID, endpoint)
		if err != nil {
			return errMsg{err: err}
		}

		execCmd := tea.ExecProcess(cmd, func(err error) tea.Msg {
			// Terminate session after plugin exits
			if sess.SessionId != nil {
				_ = repo.TerminateSession(context.Background(), *sess.SessionId)
			}
			return ssmSessionDoneMsg{err: err}
		})
		return execCmd()
	}
}

func (m Model) viewInstanceList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("EC2 Instances (Running)"))
	b.WriteString("\n")

	// Filter bar
	if m.filterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.filterInput)))
	} else if m.filterInput != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.filterInput)))
	}
	b.WriteString("\n\n")

	if len(m.filtered) == 0 {
		b.WriteString(dimStyle.Render("  No matching instances"))
		b.WriteString("\n")
	} else {
		// Calculate visible range for scrolling
		visibleLines := max(m.height-8, 5)
		start := 0
		if m.instIdx >= visibleLines {
			start = m.instIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filtered))

		for i := start; i < end; i++ {
			inst := m.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == m.instIdx {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, inst.DisplayTitle())))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d instances", len(m.filtered), len(m.instances))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • enter: connect • esc: back • H: home"))
	return b.String()
}
