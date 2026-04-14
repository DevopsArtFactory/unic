package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func (m Model) handleEC2VPCMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case instancesLoadedMsg:
		m.instances = msg.instances
		m.filtered = msg.instances
		m.instIdx = 0
		m.screen = screenInstanceList
		return m, nil, true

	case vpcsLoadedMsg:
		m.vpcs = msg.vpcs
		m.filteredVPCs = msg.vpcs
		m.vpcIdx = 0
		m.screen = screenVPCList
		return m, nil, true

	case subnetsLoadedMsg:
		m.subnets = msg.subnets
		m.subnetIdx = 0
		m.screen = screenSubnetList
		return m, nil, true

	case availableIPsLoadedMsg:
		m.availableIPs = msg.ips
		m.resetFilter(filterSubnetIPs)
		m.screen = screenSubnetDetail
		return m, nil, true

	case reachabilityTargetsLoadedMsg:
		m.reachabilityTargets = msg.targets
		m.reachabilitySourceTypes = buildReachabilityTargetTypes(msg.targets, false)
		m.reachabilitySourceTypeIdx = 0
		m.reachabilityDestTypes = nil
		m.reachabilityDestTypeIdx = 0
		m.filteredReachabilityTargets = applyReachabilityTargetFilter(msg.targets, m.selectedReachabilitySourceType(), "")
		m.reachabilityIdx = 0
		m.reachabilityFilter = ""
		m.reachabilityFilterActive = false
		m.reachabilitySource = nil
		m.reachabilityDestination = nil
		m.reachabilityDestinationIP = ""
		m.reachabilityProtocolIdx = 0
		m.reachabilityPortInput = "443"
		m.reachabilityConfigField = 0
		m.reachabilityResult = nil
		m.reachabilityScrollOffset = 0
		m.screen = screenReachabilitySourceList
		return m, nil, true

	case reachabilityAnalysisLoadedMsg:
		m.reachabilityResult = msg.result
		m.reachabilityScrollOffset = 0
		m.screen = screenReachabilityResult
		return m, nil, true

	case ssmSessionDoneMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil, true
		}
		m.screen = screenInstanceList
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) updateInstanceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterInstances); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterInstances)
	case "up", "k":
		if m.instIdx > 0 {
			m.instIdx--
		}
	case "down", "j":
		if m.instIdx < len(m.filtered)-1 {
			m.instIdx++
		}
	case "/":
		return m, m.activateFilter(filterInstances)
	case "r":
		m.resetFilter(filterInstances)
		return m.startLoading(m.loadInstances())
	case "enter":
		if len(m.filtered) > 0 && m.instIdx < len(m.filtered) {
			return m, m.startSSMSession(m.filtered[m.instIdx])
		}
	}
	return m, nil
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
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("EC2 Instances (Running)"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterInstances))
	b.WriteString("\n\n")

	if len(m.filtered) == 0 {
		panel.WriteString(dimStyle.Render("  No matching instances"))
		panel.WriteString("\n")
	} else {
		// Calculate visible range for scrolling
		visibleLines := max(m.height-10, 5)
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
			panel.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, inst.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d instances", len(m.filtered), len(m.instances))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: connect • esc: back • H: home"))
	return b.String()
}
