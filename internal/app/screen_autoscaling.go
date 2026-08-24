package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

type autoScalingModel struct {
	groups          []awsservice.AutoScalingGroup
	filtered        []awsservice.AutoScalingGroup
	idx             int
	selected        *awsservice.AutoScalingGroup
	activities      []awsservice.AutoScalingActivity
	detailScroll    int
	capacityInput   string
	capacityError   string
	pendingCapacity int32
	confirmInput    string
	notice          string
}

func newAutoScalingModel() autoScalingModel { return autoScalingModel{} }

func (am *autoScalingModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoadingFor(screenAutoScalingGroupList, "Loading Auto Scaling groups...", nil, am.loadGroups(*m))
}

func (am *autoScalingModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case autoScalingGroupsLoadedMsg:
		if msg.err != nil {
			newM, cmd := m.Update(errMsg{err: msg.err})
			return newM, cmd, true
		}
		am.groups = msg.groups
		am.filtered = applyFilter(am.groups, m.filterValue(filterAutoScalingGroups))
		am.idx = 0
		am.selected = nil
		am.activities = nil
		am.notice = ""
		finishAutoScalingLoad(m, screenAutoScalingGroupList)
		return *m, nil, true
	case autoScalingDetailLoadedMsg:
		if am.selected == nil || am.selected.Name != msg.groupName {
			return *m, nil, true
		}
		if msg.err != nil {
			newM, cmd := m.Update(errMsg{err: msg.err})
			return newM, cmd, true
		}
		am.selected = msg.group
		am.activities = msg.activities
		am.detailScroll = 0
		am.replaceGroup(*msg.group, m.filterValue(filterAutoScalingGroups))
		finishAutoScalingLoad(m, screenAutoScalingGroupDetail)
		return *m, nil, true
	case autoScalingCapacityUpdatedMsg:
		if am.selected == nil || am.selected.Name != msg.groupName {
			return *m, nil, true
		}
		if msg.err != nil {
			newM, cmd := m.Update(errMsg{err: msg.err})
			return newM, cmd, true
		}
		am.selected.DesiredCapacity = msg.desired
		am.replaceGroup(*am.selected, m.filterValue(filterAutoScalingGroups))
		am.capacityInput = ""
		am.confirmInput = ""
		am.notice = fmt.Sprintf("Desired capacity update requested: %d", msg.desired)
		finishAutoScalingLoad(m, screenAutoScalingGroupDetail)
		return *m, nil, true
	}
	return *m, nil, false
}

func (am *autoScalingModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenAutoScalingGroupList:
		newM, cmd := am.updateList(m, msg)
		return newM, cmd, true
	case screenAutoScalingGroupDetail:
		newM, cmd := am.updateDetail(m, msg)
		return newM, cmd, true
	case screenAutoScalingCapacityInput:
		newM, cmd := am.updateCapacityInput(m, msg)
		return newM, cmd, true
	case screenAutoScalingConfirm:
		newM, cmd := am.updateConfirm(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (am autoScalingModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenAutoScalingGroupList:
		return am.viewList(m), true
	case screenAutoScalingGroupDetail:
		return am.viewDetail(m), true
	case screenAutoScalingCapacityInput:
		return am.viewCapacityInput(m), true
	case screenAutoScalingConfirm:
		return am.viewConfirm(m), true
	default:
		return "", false
	}
}

func (am *autoScalingModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterAutoScalingGroups {
		return false
	}
	am.filtered = applyFilter(am.groups, m.filterValue(target))
	am.idx = 0
	return true
}

func (am *autoScalingModel) updateList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterAutoScalingGroups); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterAutoScalingGroups)
	case "up", "k":
		am.idx = previousListIndex(am.idx, len(am.filtered))
	case "down", "j":
		am.idx = nextListIndex(am.idx, len(am.filtered))
	case "/":
		return *m, m.activateFilter(filterAutoScalingGroups)
	case "r":
		return m.startLoadingFor(screenAutoScalingGroupList, "Loading Auto Scaling groups...", nil, am.loadGroups(*m))
	case "enter":
		if am.idx < len(am.filtered) {
			selected := am.filtered[am.idx]
			am.selected = &selected
			am.activities = nil
			am.detailScroll = 0
			am.notice = ""
			return m.startLoadingFor(screenAutoScalingGroupDetail, "Loading Auto Scaling group...", []string{selected.Name}, am.loadDetail(*m, selected.Name))
		}
	}
	return *m, nil
}

func (am *autoScalingModel) updateDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visibleLines := am.detailVisibleLines(*m)
	maxOffset := max(len(am.detailLines(*m))-visibleLines, 0)
	switch msg.String() {
	case "q", "esc":
		am.detailScroll = 0
		m.screen = screenAutoScalingGroupList
	case "up", "k":
		am.detailScroll = max(am.detailScroll-1, 0)
	case "down", "j":
		am.detailScroll = min(am.detailScroll+1, maxOffset)
	case "pgup":
		am.detailScroll = max(am.detailScroll-visibleLines, 0)
	case "pgdown":
		am.detailScroll = min(am.detailScroll+visibleLines, maxOffset)
	case "r":
		if am.selected != nil {
			return m.startLoadingFor(screenAutoScalingGroupDetail, "Refreshing Auto Scaling group...", []string{am.selected.Name}, am.loadDetail(*m, am.selected.Name))
		}
	case "c":
		if am.selected != nil {
			am.capacityInput = ""
			am.capacityError = ""
			am.confirmInput = ""
			m.screen = screenAutoScalingCapacityInput
		}
	}
	return *m, nil
}

func (am *autoScalingModel) updateCapacityInput(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenAutoScalingGroupDetail
	case "enter":
		if am.selected == nil {
			return *m, nil
		}
		capacity, err := strconv.ParseInt(am.capacityInput, 10, 32)
		if err != nil {
			am.capacityError = "Enter a whole number"
			return *m, nil
		}
		if capacity < int64(am.selected.MinSize) || capacity > int64(am.selected.MaxSize) {
			am.capacityError = fmt.Sprintf("Capacity must be between %d and %d", am.selected.MinSize, am.selected.MaxSize)
			return *m, nil
		}
		if int32(capacity) == am.selected.DesiredCapacity {
			am.capacityError = "Capacity is already set to that value"
			return *m, nil
		}
		am.pendingCapacity = int32(capacity)
		am.capacityError = ""
		am.confirmInput = ""
		m.screen = screenAutoScalingConfirm
	case "backspace":
		am.capacityInput = trimLastRune(am.capacityInput)
		am.capacityError = ""
	default:
		for _, r := range msg.Runes {
			if r >= '0' && r <= '9' {
				am.capacityInput += string(r)
				am.capacityError = ""
			}
		}
	}
	return *m, nil
}

func (am *autoScalingModel) updateConfirm(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenAutoScalingCapacityInput
	case "enter":
		if am.selected != nil && am.confirmInput == am.selected.Name {
			name, desired := am.selected.Name, am.pendingCapacity
			return m.startLoadingFor(screenAutoScalingGroupDetail, "Updating desired capacity...", []string{name, fmt.Sprintf("desired=%d", desired)}, am.setDesiredCapacity(*m, name, desired))
		}
	case "backspace":
		am.confirmInput = trimLastRune(am.confirmInput)
	default:
		am.confirmInput = appendKeyRunes(am.confirmInput, msg)
	}
	return *m, nil
}

func (am autoScalingModel) loadGroups(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return autoScalingGroupsLoadedMsg{err: err}
			}
		}
		groups, err := repo.ListAutoScalingGroups(ctx)
		return autoScalingGroupsLoadedMsg{groups: groups, err: err}
	}
}

func (am autoScalingModel) loadDetail(m Model, name string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return autoScalingDetailLoadedMsg{groupName: name, err: err}
			}
		}
		group, activities, err := repo.DescribeAutoScalingGroup(ctx, name)
		return autoScalingDetailLoadedMsg{groupName: name, group: group, activities: activities, err: err}
	}
}

func (am autoScalingModel) setDesiredCapacity(m Model, name string, desired int32) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return autoScalingCapacityUpdatedMsg{groupName: name, desired: desired, err: err}
			}
		}
		err := repo.SetAutoScalingDesiredCapacity(ctx, name, desired)
		return autoScalingCapacityUpdatedMsg{groupName: name, desired: desired, err: err}
	}
}

func (am *autoScalingModel) replaceGroup(group awsservice.AutoScalingGroup, filter string) {
	for i := range am.groups {
		if am.groups[i].Name == group.Name {
			am.groups[i] = group
			break
		}
	}
	am.filtered = applyFilter(am.groups, filter)
	for i := range am.filtered {
		if am.filtered[i].Name == group.Name {
			am.idx = i
			return
		}
	}
	am.idx = clampListIndex(am.idx, len(am.filtered))
}

func finishAutoScalingLoad(m *Model, target screen) {
	if m.loadingReturnScreen != target {
		return
	}
	previousScreens := []*screen{&m.settingsPrevScreen, &m.palette.prevScreen, &m.views.prevScreen, &m.ctxPrevScreen}
	if m.screen == screenLoading {
		m.screen = target
	}
	for _, previous := range previousScreens {
		if *previous == screenLoading {
			*previous = target
		}
	}

	// Reopening global overlays can overwrite their shared predecessor fields
	// and form a cycle. Ensure the active overlay chain still has a path back
	// to the completed Auto Scaling screen.
	current := m.screen
	visited := make(map[screen]struct{}, len(previousScreens))
	for {
		previous := autoScalingOverlayPrevious(m, current)
		if previous == nil || *previous == target {
			break
		}
		if _, seen := visited[*previous]; seen {
			*previous = target
			break
		}
		visited[current] = struct{}{}
		current = *previous
	}
	m.loadingReturnScreen = 0
}

func autoScalingOverlayPrevious(m *Model, current screen) *screen {
	switch current {
	case screenSettings:
		return &m.settingsPrevScreen
	case screenCommandPalette:
		return &m.palette.prevScreen
	case screenViewList:
		return &m.views.prevScreen
	case screenContextPicker:
		return &m.ctxPrevScreen
	default:
		return nil
	}
}

func (am autoScalingModel) viewList(m Model) string {
	var b, panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Auto Scaling Groups"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterAutoScalingGroups))
	b.WriteString("\n\n")
	if len(am.filtered) == 0 {
		empty := "  No Auto Scaling groups found"
		if len(am.groups) > 0 {
			empty = "  No matching Auto Scaling groups"
		}
		panel.WriteString(dimStyle.Render(empty))
		panel.WriteString("\n")
	} else {
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %-34s %7s %5s %5s %9s %9s", "GROUP", "DESIRED", "MIN", "MAX", "INSTANCES", "HEALTHY")))
		panel.WriteString("\n")
		visibleLines := max(m.height-12, 5)
		start := max(am.idx-visibleLines+1, 0)
		for i := start; i < min(start+visibleLines, len(am.filtered)); i++ {
			cursor, style := "  ", normalStyle
			if i == am.idx {
				cursor, style = "> ", selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterAutoScalingGroups, am.filtered[i].DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d groups", len(am.filtered), len(am.groups))))
	}
	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (am autoScalingModel) viewDetail(m Model) string {
	if am.selected == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Auto Scaling Group Detail"))
	b.WriteString("\n\n")
	if am.notice != "" {
		b.WriteString(successStyle.Render("  " + am.notice))
		b.WriteString("\n\n")
	}
	lines := am.detailLines(m)
	visibleLines := am.detailVisibleLines(m)
	start := min(am.detailScroll, max(len(lines)-visibleLines, 0))
	for _, line := range lines[start:min(start+visibleLines, len(lines))] {
		b.WriteString(line)
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (am autoScalingModel) detailVisibleLines(m Model) int {
	overhead := 8
	if am.notice != "" {
		overhead = 10
	}
	return max(m.height-overhead, 5)
}

func (am autoScalingModel) detailLines(m Model) []string {
	group := am.selected
	if group == nil {
		return nil
	}
	lines := []string{
		m.renderEC2DetailLine("Name", group.Name),
		m.renderEC2DetailLine("ARN", group.ARN),
		m.renderEC2DetailLine("Status", ec2ValueOrDash(group.Status)),
		m.renderEC2DetailLine("Health Check", ec2ValueOrDash(group.HealthCheckType)),
		m.renderEC2DetailLine("Capacity", fmt.Sprintf("desired %d  min %d  max %d", group.DesiredCapacity, group.MinSize, group.MaxSize)),
		m.renderEC2DetailLine("Instances", fmt.Sprintf("%d total  %d healthy", len(group.Instances), group.HealthyInstanceCount())),
		"\n",
		titleStyle.Render("Instances") + "\n",
	}
	if len(group.Instances) == 0 {
		lines = append(lines, dimStyle.Render("  No instances")+"\n")
	}
	for _, instance := range group.Instances {
		protection := ""
		if instance.ProtectedFromScaleIn {
			protection = "  protected"
		}
		lines = append(lines, m.renderEC2DetailLine(instance.ID, fmt.Sprintf("%s  %s  %s  %s%s", ec2ValueOrDash(instance.InstanceType), ec2ValueOrDash(instance.AvailabilityZone), ec2ValueOrDash(instance.LifecycleState), ec2ValueOrDash(instance.HealthStatus), protection)))
	}
	lines = append(lines, "\n", titleStyle.Render("Recent Scaling Activity")+"\n")
	if len(am.activities) == 0 {
		lines = append(lines, dimStyle.Render("  No recent scaling activity")+"\n")
	}
	for _, activity := range am.activities {
		started := "-"
		if !activity.StartTime.IsZero() {
			started = activity.StartTime.Local().Format("2006-01-02 15:04:05")
		}
		lines = append(lines, m.renderEC2DetailLine(started, fmt.Sprintf("[%s] %s", ec2ValueOrDash(activity.Status), ec2ValueOrDash(activity.Description))))
		if activity.StatusMessage != "" {
			label := "Message"
			switch strings.ToLower(activity.Status) {
			case "failed", "cancelled":
				label = "Failure"
			}
			lines = append(lines, m.renderEC2DetailLine(label, activity.StatusMessage))
		} else if activity.Cause != "" {
			lines = append(lines, m.renderEC2DetailLine("Cause", activity.Cause))
		}
	}
	return lines
}

func (am autoScalingModel) viewCapacityInput(m Model) string {
	if am.selected == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Set Desired Capacity"))
	b.WriteString("\n\n")
	b.WriteString(m.renderEC2DetailLine("Group", am.selected.Name))
	b.WriteString(m.renderEC2DetailLine("Current", fmt.Sprintf("%d", am.selected.DesiredCapacity)))
	b.WriteString(m.renderEC2DetailLine("Allowed", fmt.Sprintf("%d-%d", am.selected.MinSize, am.selected.MaxSize)))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  New desired capacity:"))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", am.capacityInput)))
	b.WriteString("\n")
	if am.capacityError != "" {
		b.WriteString(errorStyle.Render("  " + am.capacityError))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (am autoScalingModel) viewConfirm(m Model) string {
	if am.selected == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(errorStyle.Render("Confirm Desired Capacity Change"))
	b.WriteString("\n\n")
	b.WriteString(m.renderEC2DetailLine("Group", am.selected.Name))
	b.WriteString(m.renderEC2DetailLine("Current", fmt.Sprintf("%d", am.selected.DesiredCapacity)))
	b.WriteString(m.renderEC2DetailLine("New", fmt.Sprintf("%d", am.pendingCapacity)))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  This can launch or terminate EC2 instances."))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  Type the Auto Scaling group name to confirm:"))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", am.confirmInput)))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}
