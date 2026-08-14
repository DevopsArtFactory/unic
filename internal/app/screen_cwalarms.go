package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

// The CloudWatch alarm browser is the alarm-first incident entry point: list
// alarms firing-first, filter by state or text, inspect a transition history,
// and jump from an alarm's dimensions into the owning resource browser.

var alarmStateFilters = []string{"ALL", "ALARM", "INSUFFICIENT_DATA", "OK"}

type cwAlarmsModel struct {
	alarms      []awsservice.CloudWatchAlarm
	filtered    []awsservice.CloudWatchAlarm
	idx         int
	stateIdx    int // index into alarmStateFilters
	selected    *awsservice.CloudWatchAlarm
	history     []awsservice.CloudWatchAlarmHistoryItem
	historyName string
}

func newCWAlarmsModel() cwAlarmsModel {
	return cwAlarmsModel{}
}

func (am *cwAlarmsModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(am.loadAlarms(*m))
}

func (am *cwAlarmsModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case cwAlarmsLoadedMsg:
		am.alarms = msg.alarms
		am.applyStateAndTextFilter(m)
		am.idx = 0
		am.selected = nil
		m.screen = screenCWAlarmList
		return *m, nil, true
	case cwAlarmHistoryLoadedMsg:
		if am.selected == nil || am.selected.Name != msg.alarmName {
			return *m, nil, true
		}
		am.history = msg.items
		am.historyName = msg.alarmName
		m.screen = screenCWAlarmDetail
		return *m, nil, true
	}
	return *m, nil, false
}

func (am *cwAlarmsModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenCWAlarmList:
		newM, cmd := am.updateList(m, msg)
		return newM, cmd, true
	case screenCWAlarmDetail:
		newM, cmd := am.updateDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (am cwAlarmsModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenCWAlarmList:
		return am.viewList(m), true
	case screenCWAlarmDetail:
		return am.viewDetail(m), true
	default:
		return "", false
	}
}

func (am *cwAlarmsModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterCWAlarms {
		return false
	}
	am.applyStateAndTextFilter(m)
	am.idx = 0
	return true
}

// applyStateAndTextFilter composes the state tab with the shared text filter.
func (am *cwAlarmsModel) applyStateAndTextFilter(m *Model) {
	state := alarmStateFilters[am.stateIdx]
	source := am.alarms
	if state != "ALL" {
		source = nil
		for _, alarm := range am.alarms {
			if alarm.State == state {
				source = append(source, alarm)
			}
		}
	}
	am.filtered = applyFilter(source, m.filterValue(filterCWAlarms))
}

func (am *cwAlarmsModel) updateList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterCWAlarms); handled {
		return *m, cmd
	}

	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterCWAlarms)
	case "up", "k":
		am.idx = previousListIndex(am.idx, len(am.filtered))
	case "down", "j":
		am.idx = nextListIndex(am.idx, len(am.filtered))
	case "/":
		return *m, m.activateFilter(filterCWAlarms)
	case "tab":
		am.stateIdx = (am.stateIdx + 1) % len(alarmStateFilters)
		am.applyStateAndTextFilter(m)
		am.idx = 0
	case "r":
		m.resetFilter(filterCWAlarms)
		return m.startLoading(am.loadAlarms(*m))
	case "enter":
		if len(am.filtered) > 0 && am.idx < len(am.filtered) {
			selected := am.filtered[am.idx]
			am.selected = &selected
			am.history = nil
			return m.startLoadingWithMessage("Loading alarm history...", []string{selected.Name}, am.loadHistory(*m, selected.Name))
		}
	}
	return *m, nil
}

func (am *cwAlarmsModel) updateDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenCWAlarmList
	case "r":
		if am.selected != nil {
			return m.startLoadingWithMessage("Loading alarm history...", []string{am.selected.Name}, am.loadHistory(*m, am.selected.Name))
		}
	case "g":
		if am.selected != nil {
			if feature, target, value, ok := alarmRelatedResource(*am.selected); ok {
				m.enterServiceForPalette(paletteItem{service: featureService(feature), feature: feature})
				m.storeFilterValue(target, value)
				return m.startFeature(feature)
			}
		}
	case "l":
		if am.selected != nil {
			if logGroup, ok := alarmRelatedLogGroup(*am.selected); ok {
				m.enterServiceForPalette(paletteItem{service: domain.ServiceCloudWatchLogs, feature: domain.FeatureCloudWatchLogsBrowser})
				m.storeFilterValue(filterCWLogGroups, logGroup)
				return m.startFeature(domain.FeatureCloudWatchLogsBrowser)
			}
		}
	}
	return *m, nil
}

// alarmRelatedResource maps well-known alarm dimensions to a resource browser
// and its prefilled filter.
func alarmRelatedResource(alarm awsservice.CloudWatchAlarm) (domain.FeatureKind, filterTarget, string, bool) {
	if value := alarm.Dimension("DBInstanceIdentifier"); value != "" {
		return domain.FeatureRDSBrowser, filterRDS, value, true
	}
	if value := alarm.Dimension("InstanceId"); value != "" {
		return domain.FeatureEC2InstanceBrowser, filterEC2BrowserInstances, value, true
	}
	if value := alarm.Dimension("ClusterName"); value != "" {
		return domain.FeatureECSExec, filterECSClusters, value, true
	}
	if value := alarm.Dimension("FunctionName"); value != "" {
		return domain.FeatureLambdaBrowser, filterLambdaFunctions, value, true
	}
	return "", filterNone, "", false
}

// alarmRelatedLogGroup derives an obvious log group from alarm dimensions.
func alarmRelatedLogGroup(alarm awsservice.CloudWatchAlarm) (string, bool) {
	if fn := alarm.Dimension("FunctionName"); fn != "" {
		return "/aws/lambda/" + fn, true
	}
	if lg := alarm.Dimension("LogGroupName"); lg != "" {
		return lg, true
	}
	return "", false
}

// featureService resolves the owning service for a feature from the catalog.
func featureService(kind domain.FeatureKind) domain.AwsService {
	for _, svc := range domain.Catalog() {
		for _, feat := range svc.Features {
			if feat.Kind == kind {
				return svc.Name
			}
		}
	}
	return ""
}

func (am cwAlarmsModel) loadAlarms(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		alarms, err := repo.ListAlarms(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(alarms) == 0 {
			return errMsg{err: fmt.Errorf("no CloudWatch alarms found")}
		}
		return cwAlarmsLoadedMsg{alarms: alarms}
	}
}

func (am cwAlarmsModel) loadHistory(m Model, alarmName string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}
		items, err := repo.ListAlarmHistory(ctx, alarmName)
		if err != nil {
			return errMsg{err: err}
		}
		return cwAlarmHistoryLoadedMsg{alarmName: alarmName, items: items}
	}
}

func renderAlarmState(state string) string {
	switch state {
	case "ALARM":
		return errorStyle.Render(state)
	case "OK":
		return selectedStyle.Render(state)
	case "INSUFFICIENT_DATA":
		return filterStyle.Render(state)
	default:
		return normalStyle.Render(state)
	}
}

func (am cwAlarmsModel) viewList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("CloudWatch Alarms"))
	b.WriteString("\n")

	var tabs []string
	for i, state := range alarmStateFilters {
		if i == am.stateIdx {
			tabs = append(tabs, selectedStyle.Render("["+state+"]"))
		} else {
			tabs = append(tabs, dimStyle.Render(state))
		}
	}
	b.WriteString("  " + strings.Join(tabs, "  "))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterCWAlarms))
	b.WriteString("\n\n")

	if len(am.filtered) == 0 {
		emptyText := "  No alarms found"
		if len(am.alarms) > 0 {
			emptyText = "  No matching alarms"
		}
		panel.WriteString(dimStyle.Render(emptyText))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-12, 5)
		start := 0
		if am.idx >= visibleLines {
			start = am.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(am.filtered))
		for i := start; i < end; i++ {
			alarm := am.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == am.idx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterCWAlarms, alarm.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d alarms", len(am.filtered), len(am.alarms))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • tab: state filter • /: filter • r: refresh • enter: detail • esc: back"))
	return b.String()
}

func (am cwAlarmsModel) viewDetail(m Model) string {
	if am.selected == nil {
		return ""
	}
	alarm := am.selected
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Alarm Detail"))
	b.WriteString("\n\n")

	b.WriteString(m.renderEC2DetailLine("Name", alarm.Name))
	b.WriteString(m.renderEC2StyledDetailLine("State", renderAlarmState(alarm.State)))
	b.WriteString(m.renderEC2DetailLine("State Since", formatAlarmTime(alarm.StateUpdated)))
	b.WriteString(m.renderEC2DetailLine("Reason", ec2ValueOrDash(alarm.StateReason)))
	b.WriteString(m.renderEC2DetailLine("Metric", ec2ValueOrDash(alarm.Namespace+"/"+alarm.MetricName)))
	b.WriteString(m.renderEC2DetailLine("Condition", fmt.Sprintf("%s %g", alarm.ComparisonOperator, alarm.Threshold)))
	actions := "disabled"
	if alarm.ActionsEnabled {
		actions = "enabled"
	}
	b.WriteString(m.renderEC2DetailLine("Actions", actions))

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Dimensions"))
	b.WriteString("\n")
	if len(alarm.Dimensions) == 0 {
		b.WriteString(dimStyle.Render("  (none)"))
		b.WriteString("\n")
	} else {
		for _, dim := range alarm.Dimensions {
			b.WriteString(m.renderEC2DetailLine(dim.Name, dim.Value))
		}
	}

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Recent Transitions"))
	b.WriteString("\n")
	if len(am.history) == 0 {
		b.WriteString(dimStyle.Render("  (no recent history)"))
		b.WriteString("\n")
	} else {
		shown := min(len(am.history), 10)
		for _, item := range am.history[:shown] {
			b.WriteString(dimStyle.Render("  " + formatAlarmTime(item.Timestamp) + "  "))
			b.WriteString(normalStyle.Render(item.Summary))
			b.WriteString("\n")
		}
	}

	help := "r: reload history • esc: back • H: home"
	if _, _, _, ok := alarmRelatedResource(*alarm); ok {
		help = "g: related resource • " + help
	}
	if _, ok := alarmRelatedLogGroup(*alarm); ok {
		help = "l: logs • " + help
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(help))
	return b.String()
}

func formatAlarmTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}
