package app

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var watchIntervals = []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}

type watchModel struct {
	enabled     bool
	target      screen
	intervalIdx int
	token       int
	refreshing  bool
}

type watchTickMsg struct {
	target screen
	token  int
}

type watchRefreshMsg struct {
	target screen
	msg    tea.Msg
}

func newWatchModel() watchModel {
	return watchModel{}
}

func isWatchableScreen(s screen) bool {
	switch s {
	case screenCWAlarmList,
		screenECSServiceDetail,
		screenSQSQueueList,
		screenSQSQueueDetail,
		screenELBTargetGroupList,
		screenELBTargetList:
		return true
	default:
		return false
	}
}

func (w watchModel) interval() time.Duration {
	if w.intervalIdx < 0 || w.intervalIdx >= len(watchIntervals) {
		return watchIntervals[0]
	}
	return watchIntervals[w.intervalIdx]
}

func (w watchModel) tickCmd() tea.Cmd {
	target, token, interval := w.target, w.token, w.interval()
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return watchTickMsg{target: target, token: token}
	})
}

func (m Model) toggleWatch() (tea.Model, tea.Cmd) {
	if m.watch.enabled {
		m.stopWatch()
		return m, nil
	}
	m.watch.enabled = true
	m.watch.target = m.screen
	m.watch.token++
	return m, m.watch.tickCmd()
}

func (m Model) cycleWatchInterval() (tea.Model, tea.Cmd) {
	m.watch.intervalIdx = (m.watch.intervalIdx + 1) % len(watchIntervals)
	if !m.watch.enabled {
		return m, nil
	}
	m.watch.token++
	if m.commands != nil {
		m.commands.CancelAll()
	}
	return m, m.watch.tickCmd()
}

func (m *Model) stopWatch() {
	if !m.watch.enabled && !m.watch.refreshing {
		return
	}
	m.watch.enabled = false
	m.watch.refreshing = false
	m.watch.token++
	if m.commands != nil {
		m.commands.CancelAll()
	}
}

func (m Model) handleWatchTick(msg watchTickMsg) (tea.Model, tea.Cmd) {
	if !m.watch.enabled || m.watch.token != msg.token || m.watch.target != msg.target || m.screen != msg.target {
		return m, nil
	}
	refresh := m.watchRefreshCmd()
	if refresh == nil {
		m.stopWatch()
		return m, nil
	}
	if m.commands != nil {
		gen := m.commands.Renew()
		refresh = m.commands.BindCmd(gen, refresh)
	}
	return m, tea.Batch(refresh, m.watch.tickCmd())
}

func (m Model) handleWatchRefresh(msg watchRefreshMsg) (tea.Model, tea.Cmd) {
	if !m.watch.enabled || m.watch.target != msg.target || m.screen != msg.target {
		return m, nil
	}
	m.watch.refreshing = true
	updated, cmd := m.Update(msg.msg)
	model := updated.(Model)
	model.watch.refreshing = false
	return model, cmd
}

func (m Model) watchRefreshCmd() tea.Cmd {
	var refresh tea.Cmd
	switch m.watch.target {
	case screenCWAlarmList:
		refresh = m.cwAlarms.loadAlarms(m)
	case screenECSServiceDetail:
		refresh = m.ecs.loadServiceDetail(m)
	case screenSQSQueueList, screenSQSQueueDetail:
		refresh = m.sqs.loadQueues(m)
	case screenELBTargetGroupList, screenELBTargetList:
		if m.elb.selectedLB != nil {
			refresh = m.elb.loadGroups(m, *m.elb.selectedLB)
		}
	}
	if refresh == nil {
		return nil
	}
	target := m.watch.target
	return func() tea.Msg {
		return watchRefreshMsg{target: target, msg: refresh()}
	}
}

func (m Model) watchBadge() string {
	if !m.watch.enabled || m.watch.target != m.screen {
		return ""
	}
	return dimStyle.Render(fmt.Sprintf("  [watch %s]", m.watch.interval()))
}
