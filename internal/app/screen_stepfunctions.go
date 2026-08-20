package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

type stepFunctionsModel struct {
	stateMachines         []awsservice.StepFunctionStateMachine
	filteredStateMachines []awsservice.StepFunctionStateMachine
	stateMachineIdx       int
	selectedStateMachine  *awsservice.StepFunctionStateMachine
	executions            []awsservice.StepFunctionExecution
	filteredExecutions    []awsservice.StepFunctionExecution
	executionIdx          int
	selectedExecution     *awsservice.StepFunctionExecutionDetail
	detailScroll          int
	notice                string
}

func newStepFunctionsModel() stepFunctionsModel { return stepFunctionsModel{} }

func (sm *stepFunctionsModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoadingFor(screenStepFunctionStateMachineList, "Loading Step Functions state machines...", nil, sm.loadStateMachines(*m))
}

func (sm *stepFunctionsModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case stepFunctionStateMachinesLoadedMsg:
		sm.stateMachines = msg.stateMachines
		sm.filteredStateMachines = applyFilter(sm.stateMachines, m.filterValue(filterStepFunctionStateMachines))
		sm.stateMachineIdx = 0
		sm.selectedStateMachine = nil
		sm.executions = nil
		sm.filteredExecutions = nil
		sm.selectedExecution = nil
		sm.notice = ""
		finishStepFunctionsLoad(m, screenStepFunctionStateMachineList)
		return *m, nil, true
	case stepFunctionExecutionsLoadedMsg:
		if sm.selectedStateMachine == nil || sm.selectedStateMachine.ARN != msg.stateMachineARN {
			return *m, nil, true
		}
		sm.executions = msg.executions
		sm.filteredExecutions = applyFilter(sm.executions, m.filterValue(filterStepFunctionExecutions))
		sm.executionIdx = 0
		sm.selectedExecution = nil
		sm.notice = ""
		finishStepFunctionsLoad(m, screenStepFunctionExecutionList)
		return *m, nil, true
	case stepFunctionExecutionDetailLoadedMsg:
		sm.selectedExecution = msg.detail
		sm.detailScroll = 0
		finishStepFunctionsLoad(m, screenStepFunctionExecutionDetail)
		return *m, nil, true
	}
	return *m, nil, false
}

func (sm *stepFunctionsModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenStepFunctionStateMachineList:
		newM, cmd := sm.updateStateMachineList(m, msg)
		return newM, cmd, true
	case screenStepFunctionExecutionList:
		newM, cmd := sm.updateExecutionList(m, msg)
		return newM, cmd, true
	case screenStepFunctionExecutionDetail:
		newM, cmd := sm.updateExecutionDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (sm stepFunctionsModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenStepFunctionStateMachineList:
		return sm.viewStateMachineList(m), true
	case screenStepFunctionExecutionList:
		return sm.viewExecutionList(m), true
	case screenStepFunctionExecutionDetail:
		return sm.viewExecutionDetail(m), true
	default:
		return "", false
	}
}

func (sm *stepFunctionsModel) ApplyFilter(m *Model, target filterTarget) bool {
	switch target {
	case filterStepFunctionStateMachines:
		sm.filteredStateMachines = applyFilter(sm.stateMachines, m.filterValue(target))
		sm.stateMachineIdx = 0
		return true
	case filterStepFunctionExecutions:
		sm.filteredExecutions = applyFilter(sm.executions, m.filterValue(target))
		sm.executionIdx = 0
		return true
	default:
		return false
	}
}

func (sm *stepFunctionsModel) updateStateMachineList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterStepFunctionStateMachines); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q", "esc":
		m.resetFilter(filterStepFunctionStateMachines)
		m.resetFilter(filterStepFunctionExecutions)
		m.screen = screenFeatureList
	case "up", "k":
		sm.stateMachineIdx = previousListIndex(sm.stateMachineIdx, len(sm.filteredStateMachines))
		sm.notice = ""
	case "down", "j":
		sm.stateMachineIdx = nextListIndex(sm.stateMachineIdx, len(sm.filteredStateMachines))
		sm.notice = ""
	case "/":
		return *m, m.activateFilter(filterStepFunctionStateMachines)
	case "r":
		return sm.Start(m)
	case "enter":
		if sm.stateMachineIdx >= len(sm.filteredStateMachines) {
			break
		}
		selected := sm.filteredStateMachines[sm.stateMachineIdx]
		if strings.EqualFold(selected.Type, "EXPRESS") {
			sm.notice = "Execution history is unavailable for EXPRESS state machines"
			break
		}
		sm.selectedStateMachine = &selected
		return m.startLoadingFor(screenStepFunctionExecutionList, "Loading Step Functions executions...", []string{selected.Name}, sm.loadExecutions(*m, selected.ARN))
	}
	return *m, nil
}

func (sm *stepFunctionsModel) updateExecutionList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterStepFunctionExecutions); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q":
		m.resetFilter(filterStepFunctionExecutions)
		m.screen = screenFeatureList
	case "esc":
		m.resetFilter(filterStepFunctionExecutions)
		m.screen = screenStepFunctionStateMachineList
	case "up", "k":
		sm.executionIdx = previousListIndex(sm.executionIdx, len(sm.filteredExecutions))
	case "down", "j":
		sm.executionIdx = nextListIndex(sm.executionIdx, len(sm.filteredExecutions))
	case "/":
		return *m, m.activateFilter(filterStepFunctionExecutions)
	case "r":
		if sm.selectedStateMachine != nil {
			return m.startLoadingFor(screenStepFunctionExecutionList, "Loading Step Functions executions...", []string{sm.selectedStateMachine.Name}, sm.loadExecutions(*m, sm.selectedStateMachine.ARN))
		}
	case "enter":
		if sm.executionIdx < len(sm.filteredExecutions) {
			execution := sm.filteredExecutions[sm.executionIdx]
			return m.startLoadingFor(screenStepFunctionExecutionDetail, "Loading Step Functions execution detail...", []string{execution.Name}, sm.loadExecutionDetail(*m, execution.ARN))
		}
	}
	return *m, nil
}

func (sm *stepFunctionsModel) updateExecutionDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := sm.executionDetailLines(*m)
	visibleLines := max(m.height-8, 5)
	maxOffset := max(len(lines)-visibleLines, 0)
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenStepFunctionExecutionList
	case "up", "k":
		sm.detailScroll = max(sm.detailScroll-1, 0)
	case "down", "j":
		sm.detailScroll = min(sm.detailScroll+1, maxOffset)
	case "pgup":
		sm.detailScroll = max(sm.detailScroll-visibleLines, 0)
	case "pgdown":
		sm.detailScroll = min(sm.detailScroll+visibleLines, maxOffset)
	case "r":
		if sm.selectedExecution != nil {
			return m.startLoadingFor(screenStepFunctionExecutionDetail, "Loading Step Functions execution detail...", []string{sm.selectedExecution.Name}, sm.loadExecutionDetail(*m, sm.selectedExecution.ARN))
		}
	}
	return *m, nil
}

func (sm stepFunctionsModel) loadStateMachines(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		stateMachines, err := repo.ListStepFunctionStateMachines(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return stepFunctionStateMachinesLoadedMsg{stateMachines: stateMachines}
	}
}

func (sm stepFunctionsModel) loadExecutions(m Model, stateMachineARN string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		executions, err := repo.ListStepFunctionExecutions(ctx, stateMachineARN)
		if err != nil {
			return errMsg{err: err}
		}
		return stepFunctionExecutionsLoadedMsg{stateMachineARN: stateMachineARN, executions: executions}
	}
}

func (sm stepFunctionsModel) loadExecutionDetail(m Model, executionARN string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		detail, err := repo.DescribeStepFunctionExecution(ctx, executionARN)
		if err != nil {
			return errMsg{err: err}
		}
		return stepFunctionExecutionDetailLoadedMsg{detail: detail}
	}
}

func (sm stepFunctionsModel) viewStateMachineList(m Model) string {
	var b, panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Step Functions State Machines"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterStepFunctionStateMachines))
	b.WriteString("\n\n")

	if len(sm.filteredStateMachines) == 0 {
		message := "  No state machines found"
		if len(sm.stateMachines) > 0 {
			message = "  No matching state machines"
		}
		panel.WriteString(dimStyle.Render(message))
		panel.WriteString("\n")
	} else {
		panel.WriteString(dimStyle.Render("  " + fmt.Sprintf("%-38s  %-8s  %s", "NAME", "TYPE", "CREATED")))
		panel.WriteString("\n")
		visibleLines := max(m.height-12, 5)
		start := max(sm.stateMachineIdx-visibleLines+1, 0)
		for i := start; i < min(start+visibleLines, len(sm.filteredStateMachines)); i++ {
			cursor, style := "  ", normalStyle
			if i == sm.stateMachineIdx {
				cursor, style = "> ", selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterStepFunctionStateMachines, sm.filteredStateMachines[i].DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d state machines", len(sm.filteredStateMachines), len(sm.stateMachines))))
		if sm.notice != "" {
			panel.WriteString("\n")
			panel.WriteString(warningStyle.Render("  " + sm.notice))
		}
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (sm stepFunctionsModel) viewExecutionList(m Model) string {
	var b, panel strings.Builder
	title := "Step Functions Executions"
	if sm.selectedStateMachine != nil {
		title += " — " + sm.selectedStateMachine.Name
	}
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterStepFunctionExecutions))
	b.WriteString("\n\n")

	if len(sm.filteredExecutions) == 0 {
		message := "  No recent executions found"
		if len(sm.executions) > 0 {
			message = "  No matching executions"
		}
		panel.WriteString(dimStyle.Render(message))
		panel.WriteString("\n")
	} else {
		panel.WriteString(dimStyle.Render("  " + fmt.Sprintf("%-34s  %-16s  %s", "NAME", "STATUS", "STARTED")))
		panel.WriteString("\n")
		visibleLines := max(m.height-12, 5)
		start := max(sm.executionIdx-visibleLines+1, 0)
		for i := start; i < min(start+visibleLines, len(sm.filteredExecutions)); i++ {
			execution := sm.filteredExecutions[i]
			cursor, style := "  ", normalStyle
			if execution.NeedsAttention() {
				style = warningStyle
			}
			if i == sm.executionIdx {
				cursor, style = "> ", selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterStepFunctionExecutions, execution.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d executions (latest 200, failures first)", len(sm.filteredExecutions), len(sm.executions))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (sm stepFunctionsModel) viewExecutionDetail(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Step Functions Execution Detail"))
	b.WriteString("\n\n")
	if sm.selectedExecution == nil {
		b.WriteString(dimStyle.Render("  No execution detail loaded"))
		b.WriteString("\n\n")
		b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
		return b.String()
	}
	lines := sm.executionDetailLines(m)
	visibleLines := max(m.height-8, 5)
	start := min(sm.detailScroll, max(len(lines)-visibleLines, 0))
	end := min(start+visibleLines, len(lines))

	for _, line := range lines[start:end] {
		b.WriteString(line)
	}
	if len(lines) > visibleLines {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d-%d/%d lines", start+1, end, len(lines))))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (sm stepFunctionsModel) executionDetailLines(m Model) []string {
	detail := sm.selectedExecution
	if detail == nil {
		return nil
	}
	status := normalStyle.Render(ec2ValueOrDash(detail.Status))
	if detail.NeedsAttention() {
		status = warningStyle.Render(ec2ValueOrDash(detail.Status))
	} else if strings.EqualFold(detail.Status, "SUCCEEDED") {
		status = successStyle.Render(detail.Status)
	}
	lines := []string{
		m.renderEC2DetailLine("Name", detail.Name),
		m.renderEC2StyledDetailLine("Status", status),
		m.renderEC2DetailLine("Failed Step", ec2ValueOrDash(detail.FailedStep)),
		m.renderEC2DetailLine("Execution ARN", detail.ARN),
		m.renderEC2DetailLine("State Machine", detail.StateMachineARN),
	}
	if !detail.StartDate.IsZero() {
		lines = append(lines, m.renderEC2DetailLine("Started", detail.StartDate.Local().Format("2006-01-02 15:04:05 MST")))
	}
	if !detail.StopDate.IsZero() {
		lines = append(lines, m.renderEC2DetailLine("Stopped", detail.StopDate.Local().Format("2006-01-02 15:04:05 MST")))
		if !detail.StartDate.IsZero() {
			lines = append(lines, m.renderEC2DetailLine("Duration", detail.StopDate.Sub(detail.StartDate).Round(time.Second).String()))
		}
	}
	lines = append(lines,
		m.renderEC2DetailLine("Error", ec2ValueOrDash(detail.Error)),
		m.renderEC2DetailLine("Cause", ec2ValueOrDash(detail.Cause)),
		m.renderEC2DetailLine("Input", stepFunctionsPayloadPreview(detail.Input)),
		m.renderEC2DetailLine("Output", stepFunctionsPayloadPreview(detail.Output)),
	)
	return lines
}

const stepFunctionsPayloadPreviewLimit = 512

func stepFunctionsPayloadPreview(payload string) string {
	if strings.TrimSpace(payload) == "" {
		return "-"
	}
	preview := strings.Join(strings.Fields(payload), " ")
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(payload)); err == nil {
		preview = compact.String()
	}
	return truncateEC2DetailValue(preview, stepFunctionsPayloadPreviewLimit)
}

// finishStepFunctionsLoad keeps a completed load behind a global overlay and
// rewrites that overlay's return target instead of stealing the screen.
func finishStepFunctionsLoad(m *Model, target screen) {
	if m.screen == screenLoading {
		m.screen = target
		return
	}
	current := m.screen
	seen := make(map[screen]struct{})
	for range 8 {
		if _, ok := seen[current]; ok {
			return
		}
		seen[current] = struct{}{}
		switch current {
		case screenSettings:
			if m.settingsPrevScreen == screenLoading {
				m.settingsPrevScreen = target
				return
			}
			current = m.settingsPrevScreen
		case screenCommandPalette:
			if m.palette.prevScreen == screenLoading {
				m.palette.prevScreen = target
				return
			}
			current = m.palette.prevScreen
		case screenViewList:
			if m.views.prevScreen == screenLoading {
				m.views.prevScreen = target
				return
			}
			current = m.views.prevScreen
		case screenContextPicker:
			if m.ctxPrevScreen == screenLoading {
				m.ctxPrevScreen = target
				return
			}
			current = m.ctxPrevScreen
		default:
			return
		}
	}
}
