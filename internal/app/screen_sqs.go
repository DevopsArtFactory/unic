package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

// The SQS browser is a backlog-first triage view: queues list deepest-backlog
// first with DLQ markers, detail shows the redrive relationship, and purge /
// DLQ-redrive actions sit behind type-to-confirm.

type sqsModel struct {
	queues       []awsservice.SQSQueue
	filtered     []awsservice.SQSQueue
	idx          int
	selected     *awsservice.SQSQueue
	action       string // "purge", "redrive"
	confirmInput string
	notice       string
	allRegions   bool
	regionErrors []awsservice.RegionError
}

func newSQSModel() sqsModel {
	return sqsModel{}
}

func (sm *sqsModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(sm.loadQueues(*m))
}

func (sm *sqsModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case sqsQueuesLoadedMsg:
		sm.queues = msg.queues
		sm.regionErrors = msg.regionErrors
		sm.filtered = applyFilter(sm.queues, m.filterValue(filterSQSQueues))
		sm.idx = 0
		sm.selected = nil
		m.screen = screenSQSQueueList
		return *m, nil, true
	case sqsActionDoneMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return *m, nil, true
		}
		sm.notice = msg.notice
		m.screen = screenSQSQueueDetail
		return *m, nil, true
	}
	return *m, nil, false
}

func (sm *sqsModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenSQSQueueList:
		newM, cmd := sm.updateList(m, msg)
		return newM, cmd, true
	case screenSQSQueueDetail:
		newM, cmd := sm.updateDetail(m, msg)
		return newM, cmd, true
	case screenSQSConfirm:
		newM, cmd := sm.updateConfirm(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (sm sqsModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenSQSQueueList:
		return sm.viewList(m), true
	case screenSQSQueueDetail:
		return sm.viewDetail(m), true
	case screenSQSConfirm:
		return sm.viewConfirm(m), true
	default:
		return "", false
	}
}

func (sm *sqsModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterSQSQueues {
		return false
	}
	sm.filtered = applyFilter(sm.queues, m.filterValue(target))
	sm.idx = 0
	return true
}

func (sm *sqsModel) updateList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterSQSQueues); handled {
		return *m, cmd
	}

	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterSQSQueues)
	case "up", "k":
		sm.idx = previousListIndex(sm.idx, len(sm.filtered))
	case "down", "j":
		sm.idx = nextListIndex(sm.idx, len(sm.filtered))
	case "/":
		return *m, m.activateFilter(filterSQSQueues)
	case "A":
		if m.hasMultipleRegions() {
			sm.allRegions = !sm.allRegions
			m.resetFilter(filterSQSQueues)
			return m.startLoading(sm.loadQueues(*m))
		}
	case "r":
		m.resetFilter(filterSQSQueues)
		return m.startLoading(sm.loadQueues(*m))
	case "enter":
		if len(sm.filtered) > 0 && sm.idx < len(sm.filtered) {
			selected := sm.filtered[sm.idx]
			sm.selected = &selected
			sm.notice = ""
			m.screen = screenSQSQueueDetail
		}
	}
	return *m, nil
}

func (sm *sqsModel) updateDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenSQSQueueList
	case "r":
		return m.startLoading(sm.loadQueues(*m))
	case "d":
		// Jump from a source queue into its DLQ.
		if sm.selected != nil && sm.selected.DLQTargetARN != "" {
			if dlq := sm.queueByARN(sm.selected.DLQTargetARN); dlq != nil {
				sm.selected = dlq
				sm.notice = ""
			}
		}
	case "s":
		// Jump from a DLQ back to its (first) source queue.
		if sm.selected != nil && len(sm.selected.SourceQueueARNs) > 0 {
			if source := sm.queueByARN(sm.selected.SourceQueueARNs[0]); source != nil {
				sm.selected = source
				sm.notice = ""
			}
		}
	case "x":
		if sm.selected != nil {
			sm.action = "purge"
			sm.confirmInput = ""
			m.screen = screenSQSConfirm
		}
	case "m":
		if sm.selected != nil && sm.selected.IsDLQ() {
			sm.action = "redrive"
			sm.confirmInput = ""
			m.screen = screenSQSConfirm
		}
	}
	return *m, nil
}

func (sm *sqsModel) updateConfirm(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenSQSQueueDetail
	case "enter":
		if sm.selected != nil && sm.confirmInput == sm.selected.Name {
			m.screen = screenSQSQueueDetail
			return m.startLoadingWithMessage(
				fmt.Sprintf("Running %s...", sm.action),
				[]string{sm.selected.Name},
				sm.executeAction(*m, sm.action, *sm.selected),
			)
		}
	case "backspace":
		sm.confirmInput = trimLastRune(sm.confirmInput)
	default:
		sm.confirmInput = appendKeyRunes(sm.confirmInput, msg)
	}
	return *m, nil
}

func (sm sqsModel) queueByARN(arn string) *awsservice.SQSQueue {
	for i := range sm.queues {
		if sm.queues[i].ARN == arn {
			queue := sm.queues[i]
			return &queue
		}
	}
	return nil
}

func (sm sqsModel) loadQueues(m Model) tea.Cmd {
	allRegions := sm.allRegions && m.hasMultipleRegions()
	var regions []string
	if m.cfg != nil {
		regions = append(regions, m.cfg.Regions...)
	}
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		if allRegions {
			queues, regionErrors := repo.ListQueuesAcrossRegions(ctx, regions)
			return sqsQueuesLoadedMsg{queues: queues, regionErrors: regionErrors}
		}
		queues, err := repo.ListQueues(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(queues) == 0 {
			return errMsg{err: fmt.Errorf("no SQS queues found")}
		}
		return sqsQueuesLoadedMsg{queues: queues}
	}
}

func (sm sqsModel) executeAction(m Model, action string, queue awsservice.SQSQueue) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return sqsActionDoneMsg{err: err}
		}
		if queue.Region != "" && repo.Region != queue.Region {
			// Rows loaded through the all-regions scope act against their
			// own region, not the globally active one.
			repo = repo.ForRegion(queue.Region)
		}
		switch action {
		case "purge":
			if err := repo.PurgeQueue(ctx, queue.URL); err != nil {
				return sqsActionDoneMsg{err: err}
			}
			return sqsActionDoneMsg{notice: "Purge started (deletion can take up to 60 seconds)"}
		case "redrive":
			if err := repo.RedriveQueue(ctx, queue.ARN); err != nil {
				return sqsActionDoneMsg{err: err}
			}
			return sqsActionDoneMsg{notice: "Redrive started: messages are moving back to their source queues"}
		}
		return sqsActionDoneMsg{}
	}
}

func (sm sqsModel) viewList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	title := "SQS Queues"
	if sm.allRegions && m.hasMultipleRegions() {
		title += " (all regions)"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterSQSQueues))
	b.WriteString("\n\n")

	for _, regionErr := range sm.regionErrors {
		panel.WriteString(errorStyle.Render(fmt.Sprintf("  %s: %v", regionErr.Region, regionErr.Err)))
		panel.WriteString("\n")
	}
	if len(sm.filtered) == 0 {
		emptyText := "  No queues found"
		if len(sm.queues) > 0 {
			emptyText = "  No matching queues"
		}
		panel.WriteString(dimStyle.Render(emptyText))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-11, 5)
		start := 0
		if sm.idx >= visibleLines {
			start = sm.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(sm.filtered))
		for i := start; i < end; i++ {
			queue := sm.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == sm.idx {
				cursor = "> "
				style = selectedStyle
			}
			row := queue.DisplayTitle()
			if sm.allRegions && m.hasMultipleRegions() {
				row = fmt.Sprintf("[%s] %s", queue.Region, row)
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterSQSQueues, row)))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d queues (! = DLQ, deepest backlog first)", len(sm.filtered), len(sm.queues))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (sm sqsModel) viewDetail(m Model) string {
	if sm.selected == nil {
		return ""
	}
	queue := sm.selected
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("SQS Queue Detail"))
	b.WriteString("\n\n")

	b.WriteString(m.renderEC2DetailLine("Name", queue.Name))
	b.WriteString(m.renderEC2DetailLine("Region", ec2ValueOrDash(queue.Region)))
	b.WriteString(m.renderEC2DetailLine("ARN", queue.ARN))
	depth := fmt.Sprintf("%d", queue.Depth)
	if queue.Depth > 0 {
		b.WriteString(m.renderEC2StyledDetailLine("Depth", errorStyle.Render(depth)))
	} else {
		b.WriteString(m.renderEC2DetailLine("Depth", depth))
	}
	b.WriteString(m.renderEC2DetailLine("In Flight", fmt.Sprintf("%d", queue.InFlight)))
	b.WriteString(m.renderEC2DetailLine("Delayed", fmt.Sprintf("%d", queue.Delayed)))
	b.WriteString(m.renderEC2DetailLine("Visibility", fmt.Sprintf("%ds", queue.VisibilitySec)))
	b.WriteString(m.renderEC2DetailLine("Retention", fmt.Sprintf("%ds", queue.RetentionSec)))
	queueType := "standard"
	if queue.Fifo {
		queueType = "FIFO"
	}
	b.WriteString(m.renderEC2DetailLine("Type", queueType))

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Dead-Letter Relationship"))
	b.WriteString("\n")
	if queue.DLQTargetARN != "" {
		b.WriteString(m.renderEC2DetailLine("DLQ Target", queue.DLQTargetARN))
		b.WriteString(m.renderEC2DetailLine("Max Receives", fmt.Sprintf("%d", queue.MaxReceiveCount)))
	}
	if queue.IsDLQ() {
		b.WriteString(m.renderEC2DetailLine("Is DLQ For", fmt.Sprintf("%d source queue(s)", queue.SourceQueueCount)))
		for _, sourceARN := range queue.SourceQueueARNs {
			b.WriteString(m.renderEC2DetailLine("", sourceARN))
		}
	}
	if queue.DLQTargetARN == "" && !queue.IsDLQ() {
		b.WriteString(dimStyle.Render("  (no redrive policy)"))
		b.WriteString("\n")
	}

	if sm.notice != "" {
		b.WriteString("\n")
		b.WriteString(selectedStyle.Render("  " + sm.notice))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (sm sqsModel) viewConfirm(m Model) string {
	if sm.selected == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(errorStyle.Render("Confirm Action"))
	b.WriteString("\n\n")

	warning := fmt.Sprintf("  You are about to %s queue:", sm.action)
	if sm.action == "purge" {
		warning = "  You are about to PURGE (delete every message in) queue:"
	} else if sm.action == "redrive" {
		warning = "  You are about to redrive this DLQ's messages back to their source queues:"
	}
	b.WriteString(normalStyle.Render(warning))
	b.WriteString("\n")
	b.WriteString(selectedStyle.Render("  " + sm.selected.Name))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("  Type the queue name to confirm:"))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", sm.confirmInput)))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}
