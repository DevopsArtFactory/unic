package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

const (
	cloudFormationDriftPollDelay   = 2 * time.Second
	cloudFormationDriftPollTimeout = 5 * time.Minute
	cloudFormationDriftPollLimit   = int(cloudFormationDriftPollTimeout / cloudFormationDriftPollDelay)
)

type cloudFormationModel struct {
	stacks            []awsservice.CloudFormationStack
	filtered          []awsservice.CloudFormationStack
	idx               int
	selected          *awsservice.CloudFormationStack
	detailScroll      int
	driftDetectionID  string
	driftPollAttempts int
	driftNotice       string
}

func newCloudFormationModel() cloudFormationModel { return cloudFormationModel{} }

func (cm *cloudFormationModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(cm.loadStacks(*m))
}

func (cm *cloudFormationModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case cloudFormationStacksLoadedMsg:
		if !cloudFormationLoadActive(*m) {
			return *m, nil, true
		}
		m.awsRepo = msg.repo
		cm.stacks = msg.stacks
		cm.filtered = applyFilter(cm.stacks, m.filterValue(filterCloudFormationStacks))
		cm.idx = 0
		cm.selected = nil
		cm.detailScroll = 0
		cm.driftDetectionID = ""
		cm.driftNotice = ""
		finishCloudFormationLoad(m, screenCloudFormationStackList)
		return *m, nil, true
	case cloudFormationStackDetailLoadedMsg:
		if !cloudFormationLoadActive(*m) || cm.selected == nil || msg.stack == nil || cm.selected.ID != msg.stack.ID {
			return *m, nil, true
		}
		cm.selected = msg.stack
		cm.reconcileStack(*m, msg.stack)
		cm.detailScroll = 0
		cm.driftDetectionID = ""
		cm.driftNotice = ""
		finishCloudFormationLoad(m, screenCloudFormationStackDetail)
		return *m, nil, true
	case cloudFormationDriftStartedMsg:
		if !cloudFormationLoadActive(*m) || cm.selected == nil || cm.selected.ID != msg.stackID {
			return *m, nil, true
		}
		cm.driftDetectionID = msg.detectionID
		cm.driftPollAttempts = 0
		cm.driftNotice = "Drift detection in progress..."
		finishCloudFormationLoad(m, screenCloudFormationStackDetail)
		return *m, cm.scheduleDriftPoll(*m, msg.stackID, msg.detectionID), true
	case cloudFormationDriftPollTickMsg:
		if cm.selected == nil || cm.selected.ID != msg.stackID || cm.driftDetectionID != msg.detectionID {
			return *m, nil, true
		}
		return *m, cm.pollDrift(*m, msg.stackID, msg.detectionID), true
	case cloudFormationDriftStatusMsg:
		if cm.selected == nil || cm.selected.ID != msg.stackID || cm.driftDetectionID != msg.detectionID {
			return *m, nil, true
		}
		if msg.err != nil {
			cm.driftNotice = fmt.Sprintf("Drift status error: %v", msg.err)
			cm.driftDetectionID = ""
			cm.driftPollAttempts = 0
			return *m, nil, true
		}
		if msg.status == nil {
			cm.driftNotice = "Drift detection returned no status"
			cm.driftDetectionID = ""
			cm.driftPollAttempts = 0
			return *m, nil, true
		}
		switch msg.status.DetectionStatus {
		case "DETECTION_IN_PROGRESS":
			cm.driftPollAttempts++
			if cm.driftPollAttempts >= cloudFormationDriftPollLimit {
				cm.driftNotice = fmt.Sprintf("Drift detection timed out after %s", cloudFormationDriftPollTimeout)
				cm.driftDetectionID = ""
				cm.driftPollAttempts = 0
				return *m, nil, true
			}
			cm.driftNotice = "Drift detection in progress..."
			return *m, cm.scheduleDriftPoll(*m, msg.stackID, msg.detectionID), true
		case "DETECTION_COMPLETE":
			cm.selected.DriftStatus = msg.status.StackDriftStatus
			cm.selected.LastDriftCheck = msg.status.Timestamp
			cm.applyDriftStatus(msg.stackID, msg.status.StackDriftStatus, msg.status.Timestamp)
			cm.filtered = applyFilter(cm.stacks, m.filterValue(filterCloudFormationStacks))
			cm.idx = clampListIndex(cm.idx, len(cm.filtered))
			cm.driftNotice = fmt.Sprintf("Drift detection complete: %s (%d drifted resources)", msg.status.StackDriftStatus, msg.status.DriftedResources)
		default:
			cm.driftNotice = "Drift detection failed"
			if msg.status.Reason != "" {
				cm.driftNotice += ": " + msg.status.Reason
			}
		}
		cm.driftDetectionID = ""
		cm.driftPollAttempts = 0
		return *m, nil, true
	}
	return *m, nil, false
}

func cloudFormationLoadActive(m Model) bool {
	current := m.screen
	for {
		if current == screenLoading {
			return true
		}
		switch current {
		case screenSettings:
			current = m.settingsPrevScreen
		case screenCommandPalette:
			current = m.palette.prevScreen
		case screenViewList:
			current = m.views.prevScreen
		case screenContextPicker:
			current = m.ctxPrevScreen
		default:
			return false
		}
	}
}

func finishCloudFormationLoad(m *Model, next screen) {
	for _, target := range []*screen{
		&m.screen,
		&m.settingsPrevScreen,
		&m.palette.prevScreen,
		&m.views.prevScreen,
		&m.ctxPrevScreen,
	} {
		if *target == screenLoading {
			*target = next
		}
	}
}

func (cm *cloudFormationModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenCloudFormationStackList:
		if cmd, handled := m.updateSharedFilter(msg, filterCloudFormationStacks); handled {
			return *m, cmd, true
		}
		switch msg.String() {
		case "q", "esc":
			m.screen = screenFeatureList
			m.resetFilter(filterCloudFormationStacks)
		case "up", "k":
			cm.idx = previousListIndex(cm.idx, len(cm.filtered))
		case "down", "j":
			cm.idx = nextListIndex(cm.idx, len(cm.filtered))
		case "/":
			return *m, m.activateFilter(filterCloudFormationStacks), true
		case "r":
			newM, cmd := m.startLoading(cm.loadStacks(*m))
			return newM, cmd, true
		case "enter":
			if cm.idx < len(cm.filtered) {
				selected := cm.filtered[cm.idx]
				cm.selected = &selected
				newM, cmd := m.startLoadingWithMessage("Loading stack detail...", []string{selected.Name}, cm.loadStack(*m, selected.ID))
				return newM, cmd, true
			}
		}
		return *m, nil, true
	case screenCloudFormationStackDetail:
		lines := cm.detailLines(*m)
		visibleLines := max(m.height-9, 5)
		maxOffset := max(len(lines)-visibleLines, 0)
		switch msg.String() {
		case "q":
			cm.driftDetectionID = ""
			m.screen = screenFeatureList
		case "esc":
			cm.detailScroll = 0
			cm.driftDetectionID = ""
			m.screen = screenCloudFormationStackList
		case "up", "k":
			cm.detailScroll = max(cm.detailScroll-1, 0)
		case "down", "j":
			cm.detailScroll = min(cm.detailScroll+1, maxOffset)
		case "pgup":
			cm.detailScroll = max(cm.detailScroll-visibleLines, 0)
		case "pgdown":
			cm.detailScroll = min(cm.detailScroll+visibleLines, maxOffset)
		case "r":
			if cm.selected != nil && cm.driftDetectionID == "" {
				newM, cmd := m.startLoadingWithMessage("Refreshing stack detail...", []string{cm.selected.Name}, cm.loadStack(*m, cm.selected.ID))
				return newM, cmd, true
			}
		case "d":
			if cm.selected != nil && cm.driftDetectionID == "" {
				newM, cmd := m.startLoadingWithMessage("Starting drift detection...", []string{cm.selected.Name}, cm.detectDrift(*m, cm.selected.ID))
				return newM, cmd, true
			}
		}
		return *m, nil, true
	}
	return *m, nil, false
}

func (cm cloudFormationModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenCloudFormationStackList:
		return cm.viewStackList(m), true
	case screenCloudFormationStackDetail:
		return cm.viewStackDetail(m), true
	}
	return "", false
}

func (cm *cloudFormationModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterCloudFormationStacks {
		return false
	}
	cm.filtered = applyFilter(cm.stacks, m.filterValue(target))
	cm.idx = 0
	return true
}

func (cm cloudFormationModel) loadStacks(m Model) tea.Cmd {
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
		stacks, err := repo.ListCloudFormationStacks(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return cloudFormationStacksLoadedMsg{stacks: stacks, repo: repo}
	}
}

func (cm cloudFormationModel) loadStack(m Model, stackID string) tea.Cmd {
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
		stack, err := repo.GetCloudFormationStack(ctx, stackID)
		if err != nil {
			return errMsg{err: err}
		}
		return cloudFormationStackDetailLoadedMsg{stack: stack}
	}
}

func (cm cloudFormationModel) detectDrift(m Model, stackID string) tea.Cmd {
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
		detectionID, err := repo.DetectCloudFormationStackDrift(ctx, stackID)
		if err != nil {
			return errMsg{err: err}
		}
		return cloudFormationDriftStartedMsg{stackID: stackID, detectionID: detectionID}
	}
}

func (cm cloudFormationModel) pollDrift(m Model, stackID, detectionID string) tea.Cmd {
	cmd := func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return cloudFormationDriftStatusMsg{stackID: stackID, detectionID: detectionID, err: err}
			}
		}
		status, err := repo.GetCloudFormationStackDriftDetection(ctx, detectionID)
		return cloudFormationDriftStatusMsg{stackID: stackID, detectionID: detectionID, status: status, err: err}
	}
	return bindCloudFormationCommand(m, cmd)
}

func (cm cloudFormationModel) scheduleDriftPoll(m Model, stackID, detectionID string) tea.Cmd {
	cmd := tea.Tick(cloudFormationDriftPollDelay, func(time.Time) tea.Msg {
		return cloudFormationDriftPollTickMsg{stackID: stackID, detectionID: detectionID}
	})
	return bindCloudFormationCommand(m, cmd)
}

func bindCloudFormationCommand(m Model, cmd tea.Cmd) tea.Cmd {
	if m.commands == nil {
		return cmd
	}
	return m.commands.BindCmd(m.commands.CurrentGen(), cmd)
}

func (cm *cloudFormationModel) applyDriftStatus(stackID, driftStatus string, checked time.Time) {
	for i := range cm.stacks {
		if cm.stacks[i].ID == stackID {
			cm.stacks[i].DriftStatus = driftStatus
			cm.stacks[i].LastDriftCheck = checked
		}
	}
}

func (cm *cloudFormationModel) reconcileStack(m Model, stack *awsservice.CloudFormationStack) {
	for i := range cm.stacks {
		if cm.stacks[i].ID == stack.ID {
			cm.stacks[i] = *stack
			break
		}
	}
	awsservice.SortCloudFormationStacks(cm.stacks)
	cm.filtered = applyFilter(cm.stacks, m.filterValue(filterCloudFormationStacks))
	cm.idx = clampListIndex(cm.idx, len(cm.filtered))
	for i := range cm.filtered {
		if cm.filtered[i].ID == stack.ID {
			cm.idx = i
			break
		}
	}
}

func (cm cloudFormationModel) viewStackList(m Model) string {
	var b, panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("CloudFormation Stacks — failed and rollback states first"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterCloudFormationStacks))
	b.WriteString("\n\n")
	if len(cm.filtered) == 0 {
		empty := "  No CloudFormation stacks found"
		if len(cm.stacks) > 0 {
			empty = "  No matching CloudFormation stacks"
		}
		panel.WriteString(dimStyle.Render(empty))
		panel.WriteString("\n")
	} else {
		panel.WriteString(dimStyle.Render("  " + cloudFormationStackRow("NAME", "STATUS", "DRIFT", "UPDATED")))
		panel.WriteString("\n")
		visibleLines := max(m.height-12, 5)
		start := max(cm.idx-visibleLines+1, 0)
		for i := start; i < min(start+visibleLines, len(cm.filtered)); i++ {
			cursor, style := "  ", normalStyle
			if i == cm.idx {
				cursor, style = "> ", selectedStyle
			}
			stack := cm.filtered[i]
			updated := stack.UpdatedAt
			if updated.IsZero() {
				updated = stack.CreatedAt
			}
			row := cloudFormationStackRow(stack.Name, stack.Status, stack.DriftStatus, formatCloudFormationTime(updated))
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterCloudFormationStacks, row)))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d stacks", len(cm.filtered), len(cm.stacks))))
	}
	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func cloudFormationStackRow(name, status, drift, updated string) string {
	return fmt.Sprintf("%-32s  %-42s  %-11s  %s",
		inspectorShorten(name, 32), inspectorShorten(status, 42), inspectorShorten(drift, 11), updated)
}

func (cm cloudFormationModel) viewStackDetail(m Model) string {
	if cm.selected == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("CloudFormation Stack Detail"))
	b.WriteString("\n\n")
	lines := cm.detailLines(m)
	visibleLines := max(m.height-9, 5)
	start := min(cm.detailScroll, max(len(lines)-visibleLines, 0))
	for _, line := range lines[start:min(start+visibleLines, len(lines))] {
		b.WriteString(line)
	}
	if cm.driftNotice != "" {
		b.WriteString("\n")
		b.WriteString(selectedStyle.Render("  " + escapeTerminalControls(cm.driftNotice)))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (cm cloudFormationModel) detailLines(m Model) []string {
	stack := cm.selected
	if stack == nil {
		return nil
	}
	lines := []string{
		m.renderEC2DetailLine("Name", stack.Name),
		m.renderEC2DetailLine("Status", stack.Status),
		m.renderEC2DetailLine("Status Reason", displayCloudFormationValue(stack.StatusReason)),
		m.renderEC2DetailLine("Drift", stack.DriftStatus),
		m.renderEC2DetailLine("Drift Checked", formatCloudFormationTime(stack.LastDriftCheck)),
		m.renderEC2DetailLine("Created", formatCloudFormationTime(stack.CreatedAt)),
		m.renderEC2DetailLine("Updated", formatCloudFormationTime(stack.UpdatedAt)),
		m.renderEC2DetailLine("Termination", fmt.Sprintf("protected: %t", stack.TerminationProtection)),
		m.renderEC2DetailLine("Description", displayCloudFormationValue(stack.Description)),
		m.renderEC2DetailLine("Stack ID", stack.ID),
		cloudFormationSectionLine("Parameters"),
	}
	if len(stack.Parameters) == 0 {
		lines = append(lines, m.renderEC2DetailLine("Parameter", "-"))
	}
	for _, parameter := range stack.Parameters {
		lines = append(lines, m.renderEC2DetailLine("Parameter", fmt.Sprintf("%s = %s", parameter.Key, displayCloudFormationValue(parameter.Value))))
	}
	lines = append(lines, cloudFormationSectionLine("Outputs"))
	if len(stack.Outputs) == 0 {
		lines = append(lines, m.renderEC2DetailLine("Output", "-"))
	}
	for _, output := range stack.Outputs {
		value := displayCloudFormationValue(output.Value)
		if output.ExportName != "" {
			value += "  (export: " + output.ExportName + ")"
		}
		lines = append(lines, m.renderEC2DetailLine("Output", fmt.Sprintf("%s = %s", output.Key, value)))
		if output.Description != "" {
			lines = append(lines, m.renderEC2DetailLine("Description", output.Description))
		}
	}
	lines = append(lines, cloudFormationSectionLine("Recent Events (newest first)"))
	if len(stack.Events) == 0 {
		lines = append(lines, m.renderEC2DetailLine("Event", "-"))
	}
	for _, event := range stack.Events {
		resource := strings.TrimSpace(event.LogicalResourceID + " " + event.ResourceType)
		lines = append(lines, m.renderEC2DetailLine("Event", fmt.Sprintf("%s  %s  %s", formatCloudFormationTime(event.Timestamp), event.Status, resource)))
		if event.Reason != "" {
			lines = append(lines, m.renderEC2DetailLine("Reason", event.Reason))
		}
	}
	return lines
}

func cloudFormationSectionLine(title string) string {
	return inspectorSectionStyle.Render("  "+title) + "\n"
}

func displayCloudFormationValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func formatCloudFormationTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Local().Format("2006-01-02 15:04:05")
}
