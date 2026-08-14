package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

// The CloudTrail event lookup answers "who changed what, and when": recent
// events newest-first with time-window presets, a mutations-only toggle, an
// optional server-side resource-name lookup, and a raw-JSON detail view.

type cloudTrailWindow struct {
	label string
	since time.Duration
}

var cloudTrailWindows = []cloudTrailWindow{
	{"1h", time.Hour},
	{"6h", 6 * time.Hour},
	{"24h", 24 * time.Hour},
	{"3d", 72 * time.Hour},
	{"7d", 168 * time.Hour},
}

type cloudTrailModel struct {
	events        []awsservice.CloudTrailEvent
	filtered      []awsservice.CloudTrailEvent
	idx           int
	windowIdx     int
	mutationsOnly bool
	resourceName  string
	lookupInput   bool
	lookupText    string
	selected      *awsservice.CloudTrailEvent
	scrollOffset  int
}

func newCloudTrailModel() cloudTrailModel {
	return cloudTrailModel{windowIdx: 2} // default 24h
}

func (cm *cloudTrailModel) Start(m *Model) (tea.Model, tea.Cmd) {
	cm.resourceName = ""
	cm.mutationsOnly = false
	return m.startLoading(cm.loadEvents(*m))
}

func (cm *cloudTrailModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case cloudTrailEventsLoadedMsg:
		cm.events = msg.events
		cm.filtered = applyFilter(cm.events, m.filterValue(filterCloudTrailEvents))
		cm.idx = 0
		cm.selected = nil
		m.screen = screenCloudTrailEventList
		return *m, nil, true
	}
	return *m, nil, false
}

func (cm *cloudTrailModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenCloudTrailEventList:
		newM, cmd := cm.updateList(m, msg)
		return newM, cmd, true
	case screenCloudTrailEventDetail:
		newM, cmd := cm.updateDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (cm cloudTrailModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenCloudTrailEventList:
		return cm.viewList(m), true
	case screenCloudTrailEventDetail:
		return cm.viewDetail(m), true
	default:
		return "", false
	}
}

func (cm *cloudTrailModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterCloudTrailEvents {
		return false
	}
	cm.filtered = applyFilter(cm.events, m.filterValue(target))
	cm.idx = 0
	return true
}

func (cm *cloudTrailModel) updateList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cm.lookupInput {
		switch key {
		case "esc":
			cm.lookupInput = false
			cm.lookupText = ""
		case "enter":
			cm.resourceName = strings.TrimSpace(cm.lookupText)
			cm.lookupInput = false
			cm.lookupText = ""
			return m.startLoading(cm.loadEvents(*m))
		case "backspace":
			if runes := []rune(cm.lookupText); len(runes) > 0 {
				cm.lookupText = string(runes[:len(runes)-1])
			}
		default:
			if runes := msg.Runes; len(runes) > 0 {
				cm.lookupText += string(runes)
			}
		}
		return *m, nil
	}

	if cmd, handled := m.updateSharedFilter(msg, filterCloudTrailEvents); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterCloudTrailEvents)
	case "up", "k":
		cm.idx = previousListIndex(cm.idx, len(cm.filtered))
	case "down", "j":
		cm.idx = nextListIndex(cm.idx, len(cm.filtered))
	case "/":
		return *m, m.activateFilter(filterCloudTrailEvents)
	case "1", "2", "3", "4", "5":
		cm.windowIdx = int(key[0] - '1')
		return m.startLoading(cm.loadEvents(*m))
	case "m":
		cm.mutationsOnly = !cm.mutationsOnly
		return m.startLoading(cm.loadEvents(*m))
	case "n":
		cm.lookupInput = true
		cm.lookupText = cm.resourceName
	case "r":
		return m.startLoading(cm.loadEvents(*m))
	case "enter":
		if len(cm.filtered) > 0 && cm.idx < len(cm.filtered) {
			selected := cm.filtered[cm.idx]
			cm.selected = &selected
			cm.scrollOffset = 0
			m.screen = screenCloudTrailEventDetail
		}
	}
	return *m, nil
}

func (cm *cloudTrailModel) updateDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenCloudTrailEventList
	case "up", "k":
		if cm.scrollOffset > 0 {
			cm.scrollOffset--
		}
	case "down", "j":
		cm.scrollOffset++
	}
	return *m, nil
}

func (cm cloudTrailModel) loadEvents(m Model) tea.Cmd {
	lookup := awsservice.CloudTrailLookup{
		Since:         cloudTrailWindows[cm.windowIdx].since,
		ResourceName:  cm.resourceName,
		MutationsOnly: cm.mutationsOnly,
	}
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
		events, err := repo.LookupEvents(ctx, lookup)
		if err != nil {
			return errMsg{err: err}
		}
		return cloudTrailEventsLoadedMsg{events: events}
	}
}

func (cm cloudTrailModel) viewList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("CloudTrail Events"))
	b.WriteString("\n")

	var windows []string
	for i, window := range cloudTrailWindows {
		label := fmt.Sprintf("%d:%s", i+1, window.label)
		if i == cm.windowIdx {
			windows = append(windows, selectedStyle.Render("["+label+"]"))
		} else {
			windows = append(windows, dimStyle.Render(label))
		}
	}
	mode := dimStyle.Render("all calls")
	if cm.mutationsOnly {
		mode = errorStyle.Render("mutations only")
	}
	b.WriteString("  " + strings.Join(windows, " ") + "  " + mode)
	if cm.resourceName != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  resource:%s", cm.resourceName)))
	}
	b.WriteString("\n")
	if cm.lookupInput {
		b.WriteString(normalStyle.Render("  Resource name lookup: "))
		b.WriteString(filterStyle.Render(fmt.Sprintf("%s▏", cm.lookupText)))
		b.WriteString("\n")
	} else {
		b.WriteString(m.renderFilterValue(filterCloudTrailEvents))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if len(cm.filtered) == 0 {
		emptyText := "  No events in this window"
		if len(cm.events) > 0 {
			emptyText = "  No matching events"
		}
		panel.WriteString(dimStyle.Render(emptyText))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-12, 5)
		start := 0
		if cm.idx >= visibleLines {
			start = cm.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(cm.filtered))
		for i := start; i < end; i++ {
			event := cm.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == cm.idx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterCloudTrailEvents, event.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d events (* = mutation)", len(cm.filtered), len(cm.events))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	help := "↑/↓: navigate • 1-5: window • m: mutations • n: resource lookup • /: filter • enter: detail • esc: back"
	if cm.lookupInput {
		help = "type: resource name • enter: look up • esc: cancel"
	}
	b.WriteString(m.renderHelpBar(help))
	return b.String()
}

func (cm cloudTrailModel) viewDetail(m Model) string {
	if cm.selected == nil {
		return ""
	}
	event := cm.selected
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("CloudTrail Event Detail"))
	b.WriteString("\n\n")

	b.WriteString(m.renderEC2DetailLine("Event", event.Name))
	b.WriteString(m.renderEC2DetailLine("Time", event.Time.Local().Format("2006-01-02 15:04:05 MST")))
	b.WriteString(m.renderEC2DetailLine("Actor", ec2ValueOrDash(event.Username)))
	b.WriteString(m.renderEC2DetailLine("Source", ec2ValueOrDash(event.Source)))
	b.WriteString(m.renderEC2DetailLine("Region", ec2ValueOrDash(event.Region)))
	b.WriteString(m.renderEC2DetailLine("Source IP", ec2ValueOrDash(event.SourceIP)))
	readOnly := "yes"
	if !event.ReadOnly {
		readOnly = errorStyle.Render("no (mutation)")
		b.WriteString(m.renderEC2StyledDetailLine("Read Only", readOnly))
	} else {
		b.WriteString(m.renderEC2DetailLine("Read Only", readOnly))
	}
	b.WriteString(m.renderEC2DetailLine("Event ID", event.ID))

	if len(event.Resources) > 0 {
		b.WriteString("\n")
		b.WriteString(titleStyle.Render("Resources"))
		b.WriteString("\n")
		for _, res := range event.Resources {
			b.WriteString(m.renderEC2DetailLine(ec2ValueOrDash(res.Type), ec2ValueOrDash(res.Name)))
		}
	}

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Raw Event"))
	b.WriteString("\n")
	lines := strings.Split(event.PrettyRawJSON(), "\n")
	visibleLines := max(m.height-20, 5)
	maxOffset := max(len(lines)-visibleLines, 0)
	offset := min(cm.scrollOffset, maxOffset)
	end := min(offset+visibleLines, len(lines))
	for _, line := range lines[offset:end] {
		b.WriteString(dimStyle.Render("  " + line))
		b.WriteString("\n")
	}
	if len(lines) > visibleLines {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ... %d/%d lines", end, len(lines))))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("↑/↓: scroll raw event • esc: back • H: home"))
	return b.String()
}
