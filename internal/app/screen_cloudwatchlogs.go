package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsservice "unic/internal/services/aws"
)

// Time range presets: label and duration.
var cwTimeRanges = []struct {
	label    string
	duration time.Duration
}{
	{"5m", 5 * time.Minute},
	{"15m", 15 * time.Minute},
	{"1h", 1 * time.Hour},
	{"6h", 6 * time.Hour},
	{"24h", 24 * time.Hour},
	{"7d", 7 * 24 * time.Hour},
}

// Log level styles.
var (
	logErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // red
	logWarnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // yellow
	logInfoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	logDebugStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // dim gray
)

type cloudWatchLogsModel struct {
	groups           []awsservice.LogGroup
	filteredGroups   []awsservice.LogGroup
	groupIdx         int
	groupNextToken   *string
	selectedGroup    *awsservice.LogGroup
	streams          []awsservice.LogStream
	filteredStreams  []awsservice.LogStream
	streamIdx        int
	streamNextToken  *string
	selectedStream   *awsservice.LogStream
	events           []awsservice.LogEvent
	scrollOffset     int
	nextToken        *string
	timeRange        int
	tailing          bool
	tailToken        *string
	wrap             bool
	horizontalOffset int
}

func newCloudWatchLogsModel() cloudWatchLogsModel {
	return cloudWatchLogsModel{
		timeRange: 2,
		wrap:      true,
	}
}

func (cw *cloudWatchLogsModel) Start(m *Model) (tea.Model, tea.Cmd) {
	cw.selectedGroup = nil
	cw.selectedStream = nil
	cw.events = nil
	cw.scrollOffset = 0
	cw.nextToken = nil
	cw.tailToken = nil
	cw.tailing = false
	cw.streamNextToken = nil
	return m.startLoading(cw.loadGroups(*m, false))
}

func (cw *cloudWatchLogsModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case cwLogGroupsLoadedMsg:
		if msg.append {
			cw.groups = append(cw.groups, msg.groups...)
		} else {
			cw.groups = msg.groups
			cw.groupIdx = 0
		}
		cw.groupNextToken = msg.nextToken
		cw.filteredGroups = applyFilter(cw.groups, m.filterValue(filterCWLogGroups))
		if len(cw.filteredGroups) == 0 {
			cw.groupIdx = 0
		} else if cw.groupIdx >= len(cw.filteredGroups) {
			cw.groupIdx = len(cw.filteredGroups) - 1
		}
		m.screen = screenCWLogGroupList
		return *m, nil, true

	case cwLogStreamsLoadedMsg:
		if msg.append {
			cw.streams = append(cw.streams, msg.streams...)
		} else {
			cw.streams = msg.streams
			cw.streamIdx = 0
		}
		cw.streamNextToken = msg.nextToken
		cw.filteredStreams = applyFilter(cw.streams, m.filterValue(filterCWLogStreams))
		if len(cw.filteredStreams) == 0 {
			cw.streamIdx = 0
		} else if cw.streamIdx >= len(cw.filteredStreams) {
			cw.streamIdx = len(cw.filteredStreams) - 1
		}
		m.screen = screenCWLogStreamList
		return *m, nil, true

	case cwLogEventsLoadedMsg:
		if msg.append {
			cw.events = appendUniqueCWLogEvents(cw.events, msg.events)
			if cw.tailing {
				total := len(cw.viewerLines(*m))
				visibleLines := max(m.height-8, 5)
				cw.scrollOffset = clampCWLogScrollOffset(total-visibleLines, total, visibleLines)
			}
		} else {
			cw.events = msg.events
			visibleLines := max(m.height-8, 5)
			cw.scrollOffset = clampCWLogScrollOffset(cw.scrollOffset, len(cw.viewerLines(*m)), visibleLines)
		}
		if msg.updatePaginationToken {
			cw.nextToken = msg.nextToken
		}
		if msg.updateTailToken {
			cw.tailToken = msg.nextToken
		}
		m.screen = screenCWLogViewer
		return *m, nil, true

	case cwLogTailTickMsg:
		if cw.tailing && cw.selectedGroup != nil {
			return *m, tea.Batch(cw.pollTail(*m), cw.tickTail()), true
		}
		return *m, nil, true
	}
	return *m, nil, false
}

func (cw *cloudWatchLogsModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenCWLogGroupList:
		newM, cmd := cw.updateGroupList(m, msg)
		return newM, cmd, true
	case screenCWLogStreamList:
		newM, cmd := cw.updateStreamList(m, msg)
		return newM, cmd, true
	case screenCWLogViewer:
		newM, cmd := cw.updateViewer(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (cw cloudWatchLogsModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenCWLogGroupList:
		return cw.viewGroupList(m), true
	case screenCWLogStreamList:
		return cw.viewStreamList(m), true
	case screenCWLogViewer:
		return cw.viewViewer(m), true
	default:
		return "", false
	}
}

func (cw *cloudWatchLogsModel) ApplyFilter(m *Model, target filterTarget) bool {
	switch target {
	case filterCWLogGroups:
		cw.filteredGroups = applyFilter(cw.groups, m.filterValue(target))
		cw.groupIdx = 0
		return true
	case filterCWLogStreams:
		cw.filteredStreams = applyFilter(cw.streams, m.filterValue(target))
		cw.streamIdx = 0
		return true
	default:
		return false
	}
}

func clampCWLogScrollOffset(offset, totalEvents, visibleLines int) int {
	maxOffset := totalEvents - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		return 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func cwLogEventDedupKey(evt awsservice.LogEvent) string {
	if evt.EventID != "" {
		return "id:" + evt.EventID
	}
	return fmt.Sprintf("fallback:%d:%s", evt.Timestamp.UnixMilli(), strings.TrimSpace(evt.Message))
}

func appendUniqueCWLogEvents(existing, incoming []awsservice.LogEvent) []awsservice.LogEvent {
	if len(incoming) == 0 {
		return existing
	}

	seenEventIDs := make(map[string]struct{}, len(existing))
	for _, evt := range existing {
		seenEventIDs[cwLogEventDedupKey(evt)] = struct{}{}
	}

	result := existing
	for _, evt := range incoming {
		key := cwLogEventDedupKey(evt)
		if _, exists := seenEventIDs[key]; exists {
			continue
		}
		seenEventIDs[key] = struct{}{}
		result = append(result, evt)
	}

	return result
}

func (cw *cloudWatchLogsModel) updateGroupList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterCWLogGroups); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterCWLogGroups)
	case "up", "k":
		if cw.groupIdx > 0 {
			cw.groupIdx--
		}
	case "down", "j":
		if cw.groupIdx < len(cw.filteredGroups)-1 {
			cw.groupIdx++
		}
	case "/":
		return *m, m.activateFilter(filterCWLogGroups)
	case "n":
		if cw.groupNextToken != nil {
			return m.startLoading(cw.loadGroups(*m, true))
		}
	case "enter":
		if len(cw.filteredGroups) > 0 && cw.groupIdx < len(cw.filteredGroups) {
			selected := cw.filteredGroups[cw.groupIdx]
			cw.selectedGroup = &selected
			return m.startLoading(cw.loadStreams(*m, selected.Name, false))
		}
	}
	return *m, nil
}

func (cw cloudWatchLogsModel) viewGroupList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("CloudWatch Log Groups"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterCWLogGroups))
	b.WriteString("\n\n")

	if len(cw.filteredGroups) == 0 {
		panel.WriteString(dimStyle.Render("  No matching log groups"))
		panel.WriteString("\n")
	} else {
		maxName := 4
		for _, g := range cw.filteredGroups {
			if len(g.Name) > maxName {
				maxName = len(g.Name)
			}
		}
		if maxName > 60 {
			maxName = 60
		}
		nameCol := lipgloss.NewStyle().Width(maxName + 2)
		retCol := lipgloss.NewStyle().Width(14)

		panel.WriteString(dimStyle.Render("  " + nameCol.Render("NAME") + retCol.Render("RETENTION") + "SIZE"))
		panel.WriteString("\n")

		visibleLines := max(m.height-11, 5)
		start := 0
		if cw.groupIdx >= visibleLines {
			start = cw.groupIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(cw.filteredGroups))

		for i := start; i < end; i++ {
			g := cw.filteredGroups[i]
			cursor := "  "
			style := normalStyle
			if i == cw.groupIdx {
				cursor = "> "
				style = selectedStyle
			}
			name := g.Name
			if len(name) > maxName {
				name = name[:maxName-3] + "..."
			}
			retention := "Never"
			if g.RetentionDays > 0 {
				retention = fmt.Sprintf("%d days", g.RetentionDays)
			}
			row := cursor +
				nameCol.Inherit(style).Render(m.renderHighlightedValue(filterCWLogGroups, name)) +
				retCol.Inherit(dimStyle).Render(retention) +
				dimStyle.Render(awsservice.FormatBytes(g.StoredBytes))
			panel.WriteString(row)
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		countLine := fmt.Sprintf("  %d/%d log groups", len(cw.filteredGroups), len(cw.groups))
		if cw.groupNextToken != nil {
			countLine += " • more available"
		}
		panel.WriteString(dimStyle.Render(countLine))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • n: load more • enter: streams • esc: back • H: home"))
	return b.String()
}

func (cw *cloudWatchLogsModel) updateStreamList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterCWLogStreams); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenCWLogGroupList
		m.resetFilter(filterCWLogStreams)
	case "up", "k":
		if cw.streamIdx > 0 {
			cw.streamIdx--
		}
	case "down", "j":
		if cw.streamIdx < len(cw.filteredStreams)-1 {
			cw.streamIdx++
		}
	case "/":
		return *m, m.activateFilter(filterCWLogStreams)
	case "n":
		if cw.selectedGroup != nil && cw.streamNextToken != nil {
			return m.startLoading(cw.loadStreams(*m, cw.selectedGroup.Name, true))
		}
	case "enter":
		if len(cw.filteredStreams) > 0 && cw.streamIdx < len(cw.filteredStreams) {
			selected := cw.filteredStreams[cw.streamIdx]
			cw.selectedStream = &selected
			cw.timeRange = 2
			m.resetFilter(filterCWLogViewer)
			cw.tailing = false
			cw.tailToken = nil
			cw.wrap = true
			cw.horizontalOffset = 0
			cw.scrollOffset = 0
			return m.startLoading(cw.loadEvents(*m, false))
		}
	}
	return *m, nil
}

func (cw cloudWatchLogsModel) viewStreamList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	groupName := ""
	if cw.selectedGroup != nil {
		groupName = cw.selectedGroup.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Log Streams — %s", groupName)))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterCWLogStreams))
	b.WriteString("\n\n")

	if len(cw.filteredStreams) == 0 {
		panel.WriteString(dimStyle.Render("  No matching log streams"))
		panel.WriteString("\n")
	} else {
		maxName := 4
		for _, s := range cw.filteredStreams {
			if len(s.Name) > maxName {
				maxName = len(s.Name)
			}
		}
		if maxName > 60 {
			maxName = 60
		}
		nameCol := lipgloss.NewStyle().Width(maxName + 2)

		panel.WriteString(dimStyle.Render("  " + nameCol.Render("NAME") + "LAST EVENT"))
		panel.WriteString("\n")

		visibleLines := max(m.height-11, 5)
		start := 0
		if cw.streamIdx >= visibleLines {
			start = cw.streamIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(cw.filteredStreams))

		for i := start; i < end; i++ {
			s := cw.filteredStreams[i]
			cursor := "  "
			style := normalStyle
			if i == cw.streamIdx {
				cursor = "> "
				style = selectedStyle
			}
			name := s.Name
			if len(name) > maxName {
				name = name[:maxName-3] + "..."
			}
			lastEvent := "No events"
			if !s.LastEventTime.IsZero() {
				lastEvent = s.LastEventTime.Local().Format("2006-01-02 15:04:05")
			}
			row := cursor +
				nameCol.Inherit(style).Render(m.renderHighlightedValue(filterCWLogStreams, name)) +
				dimStyle.Render(lastEvent)
			panel.WriteString(row)
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		countLine := fmt.Sprintf("  %d/%d streams", len(cw.filteredStreams), len(cw.streams))
		if cw.streamNextToken != nil {
			countLine += " • more available"
		}
		panel.WriteString(dimStyle.Render(countLine))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • n: load more • enter: view logs • esc: back • H: home"))
	return b.String()
}

func (cw *cloudWatchLogsModel) updateViewer(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.isFiltering(filterCWLogViewer) {
		if key == "enter" {
			m.deactivateFilter()
			return m.startLoading(cw.loadEvents(*m, false))
		}
		if cmd, handled := m.updateSharedFilter(msg, filterCWLogViewer); handled {
			return *m, cmd
		}
	}

	visibleLines := max(m.height-8, 5)

	switch key {
	case "q", "esc":
		cw.tailing = false
		m.screen = screenCWLogStreamList
	case "up", "k":
		if cw.scrollOffset > 0 {
			cw.scrollOffset--
		}
	case "down", "j":
		totalLines := len(cw.viewerLines(*m))
		maxOffset := clampCWLogScrollOffset(totalLines-visibleLines, totalLines, visibleLines)
		if cw.scrollOffset < maxOffset {
			cw.scrollOffset++
		}
	case "pgup":
		cw.scrollOffset -= visibleLines
		if cw.scrollOffset < 0 {
			cw.scrollOffset = 0
		}
	case "pgdown":
		cw.scrollOffset += visibleLines
		cw.scrollOffset = clampCWLogScrollOffset(cw.scrollOffset, len(cw.viewerLines(*m)), visibleLines)
	case "t":
		cw.tailing = !cw.tailing
		if cw.tailing {
			return *m, cw.tickTail()
		}
	case "f":
		return *m, m.activateFilter(filterCWLogViewer)
	case "w":
		cw.wrap = !cw.wrap
		if cw.wrap {
			cw.horizontalOffset = 0
		}
		cw.scrollOffset = clampCWLogScrollOffset(cw.scrollOffset, len(cw.viewerLines(*m)), visibleLines)
	case "h":
		if !cw.wrap && cw.horizontalOffset > 0 {
			cw.horizontalOffset = max(cw.horizontalOffset-8, 0)
		}
	case "l":
		if !cw.wrap {
			cw.horizontalOffset = min(cw.horizontalOffset+8, cw.maxHorizontalOffset(*m))
		}
	case "n":
		if cw.nextToken != nil {
			return *m, cw.loadEvents(*m, true)
		}
	case "1", "2", "3", "4", "5", "6":
		idx := int(key[0] - '1')
		if idx >= 0 && idx < len(cwTimeRanges) {
			cw.timeRange = idx
			cw.tailing = false
			return m.startLoading(cw.loadEvents(*m, false))
		}
	}
	return *m, nil
}

func (cw cloudWatchLogsModel) viewViewer(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())

	groupName := ""
	if cw.selectedGroup != nil {
		groupName = cw.selectedGroup.Name
	}
	streamName := "All streams"
	if cw.selectedStream != nil {
		streamName = cw.selectedStream.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Logs — %s / %s", groupName, streamName)))
	b.WriteString("\n")

	var status []string
	if cw.timeRange >= 0 && cw.timeRange < len(cwTimeRanges) {
		var ranges []string
		for i, tr := range cwTimeRanges {
			if i == cw.timeRange {
				ranges = append(ranges, selectedStyle.Render(fmt.Sprintf("[%s]", tr.label)))
			} else {
				ranges = append(ranges, dimStyle.Render(fmt.Sprintf(" %s ", tr.label)))
			}
		}
		status = append(status, strings.Join(ranges, ""))
	}
	if filter := m.renderFilterValue(filterCWLogViewer); filter != "" {
		status = append(status, filter)
	}
	if cw.tailing {
		status = append(status, filterStyle.Render("TAILING"))
	}
	if len(status) > 0 {
		b.WriteString(strings.Join(status, "  "))
	}
	b.WriteString("\n\n")

	if len(cw.events) == 0 {
		b.WriteString(dimStyle.Render("  No log events found"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-8, 5)
		lines := cw.viewerLines(m)
		start := clampCWLogScrollOffset(cw.scrollOffset, len(lines), visibleLines)
		end := min(start+visibleLines, len(lines))

		for _, line := range lines[start:end] {
			b.WriteString(line)
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d events (showing lines %d-%d)", len(cw.events), start+1, end)))
	}

	b.WriteString("\n")
	wrapStatus := "off"
	if cw.wrap {
		wrapStatus = "on"
	}
	hint := fmt.Sprintf("↑/↓: scroll • pgup/pgdn: page • 1-6: time range • f: filter • t: tail • w: wrap (%s)", wrapStatus)
	if !cw.wrap {
		hint += fmt.Sprintf(" • h/l: horizontal (%d)", cw.horizontalOffset)
	}
	hint += " • n: load more • esc: back"
	b.WriteString(m.renderHelpBar(hint))
	return b.String()
}

func (cw cloudWatchLogsModel) viewerLines(m Model) []string {
	lines := make([]string, 0, len(cw.events))
	for _, evt := range cw.events {
		lines = append(lines, cw.renderEventLines(m, evt)...)
	}
	return lines
}

func (cw cloudWatchLogsModel) renderEventLines(m Model, evt awsservice.LogEvent) []string {
	tsText := evt.Timestamp.Local().Format("15:04:05.000")
	tsStyled := dimStyle.Render(tsText)

	levelText := ""
	levelStyled := ""
	switch evt.Level {
	case "ERROR", "FATAL":
		levelText = fmt.Sprintf("[%s]", evt.Level)
		levelStyled = logErrorStyle.Render(levelText)
	case "WARN":
		levelText = "[WARN]"
		levelStyled = logWarnStyle.Render(levelText)
	case "INFO":
		levelText = "[INFO]"
		levelStyled = logInfoStyle.Render(levelText)
	case "DEBUG":
		levelText = "[DEBUG]"
		levelStyled = logDebugStyle.Render(levelText)
	}

	firstPrefixPlain := "  " + tsText
	firstPrefixStyled := "  " + tsStyled
	if levelText != "" {
		firstPrefixPlain += " " + levelText
		firstPrefixStyled += " " + levelStyled
	}
	firstPrefixPlain += " "
	firstPrefixStyled += " "

	continuationPrefix := strings.Repeat(" ", lipgloss.Width(firstPrefixPlain))
	messageLines := strings.Split(strings.TrimRight(evt.Message, "\n"), "\n")
	if len(messageLines) == 0 {
		messageLines = []string{""}
	}

	rendered := make([]string, 0, len(messageLines))
	firstLine := true
	for _, line := range messageLines {
		line = strings.TrimRight(line, "\r")
		if cw.wrap {
			prefixPlain := continuationPrefix
			prefixStyled := continuationPrefix
			if firstLine {
				prefixPlain = firstPrefixPlain
				prefixStyled = firstPrefixStyled
			}
			wrapped := wrapCWLogMessage(line, max(cw.contentWidth(m)-lipgloss.Width(prefixPlain), 8))
			for idx, segment := range wrapped {
				if firstLine && idx == 0 {
					rendered = append(rendered, prefixStyled+segment)
					continue
				}
				rendered = append(rendered, continuationPrefix+segment)
			}
		} else {
			segment := sliceCWLogMessage(line, cw.horizontalOffset)
			if firstLine {
				rendered = append(rendered, firstPrefixStyled+segment)
			} else {
				rendered = append(rendered, continuationPrefix+segment)
			}
		}
		firstLine = false
	}

	if len(rendered) == 0 {
		rendered = append(rendered, firstPrefixStyled)
	}
	return rendered
}

func (cw cloudWatchLogsModel) contentWidth(m Model) int {
	if m.width <= 0 {
		return 120
	}
	return max(m.width-2, 20)
}

func (cw cloudWatchLogsModel) maxHorizontalOffset(m Model) int {
	maxWidth := 0
	for _, evt := range cw.events {
		for _, line := range strings.Split(strings.TrimRight(evt.Message, "\n"), "\n") {
			maxWidth = max(maxWidth, lipgloss.Width(strings.TrimRight(line, "\r")))
		}
	}
	return max(maxWidth-cw.contentWidth(m), 0)
}

func wrapCWLogMessage(message string, width int) []string {
	if width <= 0 {
		return []string{message}
	}
	if message == "" {
		return []string{""}
	}

	runes := []rune(message)
	lines := make([]string, 0, len(runes)/max(width, 1)+1)
	for len(runes) > 0 {
		end := min(width, len(runes))
		lines = append(lines, string(runes[:end]))
		runes = runes[end:]
	}
	return lines
}

func sliceCWLogMessage(message string, offset int) string {
	runes := []rune(message)
	if offset >= len(runes) {
		return ""
	}
	if offset < 0 {
		offset = 0
	}
	return string(runes[offset:])
}

func (cw cloudWatchLogsModel) loadGroups(m Model, appendMode bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}

		var nextToken *string
		if appendMode {
			nextToken = cw.groupNextToken
		}

		groups, token, err := repo.ListLogGroupsPage(ctx, nextToken, 10)
		if err != nil {
			return errMsg{err: err}
		}
		if !appendMode && len(groups) == 0 {
			return errMsg{err: fmt.Errorf("no log groups found")}
		}
		return cwLogGroupsLoadedMsg{groups: groups, nextToken: token, append: appendMode}
	}
}

func (cw cloudWatchLogsModel) loadStreams(m Model, logGroupName string, appendMode bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}

		var nextToken *string
		if appendMode {
			nextToken = cw.streamNextToken
		}

		streams, token, err := repo.ListLogStreamsPage(ctx, logGroupName, nextToken, 10)
		if err != nil {
			return errMsg{err: err}
		}
		if !appendMode && len(streams) == 0 {
			return errMsg{err: fmt.Errorf("no log streams found in %s", logGroupName)}
		}
		return cwLogStreamsLoadedMsg{streams: streams, nextToken: token, append: appendMode}
	}
}

func (cw cloudWatchLogsModel) loadEvents(m Model, appendMode bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return cwLogEventsLoadedMsg{append: appendMode}
		}

		if cw.selectedGroup == nil {
			return cwLogEventsLoadedMsg{append: appendMode}
		}

		now := time.Now()
		timeIdx := cw.timeRange
		if timeIdx < 0 || timeIdx >= len(cwTimeRanges) {
			timeIdx = 2
		}
		duration := cwTimeRanges[timeIdx].duration
		startTime := now.Add(-duration).UnixMilli()
		endTime := now.UnixMilli()

		var token *string
		if appendMode {
			token = cw.nextToken
		}

		events, nextToken, err := repo.FilterLogEvents(ctx, cw.selectedGroup.Name, startTime, endTime, m.filterValue(filterCWLogViewer), token)
		if err != nil {
			return errMsg{err: err}
		}

		return cwLogEventsLoadedMsg{
			events:                events,
			nextToken:             nextToken,
			append:                appendMode,
			updatePaginationToken: true,
			updateTailToken:       !appendMode,
		}
	}
}

func (cw cloudWatchLogsModel) pollTail(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return cwLogEventsLoadedMsg{append: true}
		}

		if cw.selectedGroup == nil {
			return cwLogEventsLoadedMsg{append: true}
		}

		now := time.Now()
		startTime := now.Add(-30 * time.Second).UnixMilli()
		endTime := now.UnixMilli()

		events, nextToken, err := repo.FilterLogEvents(ctx, cw.selectedGroup.Name, startTime, endTime, m.filterValue(filterCWLogViewer), cw.tailToken)
		if err != nil {
			return cwLogEventsLoadedMsg{append: true}
		}

		return cwLogEventsLoadedMsg{
			events:          events,
			nextToken:       nextToken,
			append:          true,
			updateTailToken: true,
		}
	}
}

func (cw cloudWatchLogsModel) tickTail() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return cwLogTailTickMsg{}
	})
}

func (m Model) handleCloudWatchLogsMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	return m.cwLogs.HandleMessage(&m, msg)
}

func (m Model) cwLogViewerLines() []string {
	return m.cwLogs.viewerLines(m)
}
