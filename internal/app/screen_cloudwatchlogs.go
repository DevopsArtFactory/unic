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

// --- Message handler ---

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

func (m Model) handleCloudWatchLogsMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case cwLogGroupsLoadedMsg:
		if msg.append {
			m.cwLogGroups = append(m.cwLogGroups, msg.groups...)
		} else {
			m.cwLogGroups = msg.groups
			m.cwLogGroupIdx = 0
		}
		m.cwLogGroupNextToken = msg.nextToken
		m.filteredCWLogGroups = applyFilter(m.cwLogGroups, m.filterValue(filterCWLogGroups))
		if len(m.filteredCWLogGroups) == 0 {
			m.cwLogGroupIdx = 0
		} else if m.cwLogGroupIdx >= len(m.filteredCWLogGroups) {
			m.cwLogGroupIdx = len(m.filteredCWLogGroups) - 1
		}
		m.screen = screenCWLogGroupList
		return m, nil, true

	case cwLogStreamsLoadedMsg:
		if msg.append {
			m.cwLogStreams = append(m.cwLogStreams, msg.streams...)
		} else {
			m.cwLogStreams = msg.streams
			m.cwLogStreamIdx = 0
		}
		m.cwLogStreamNextToken = msg.nextToken
		m.filteredCWLogStreams = applyFilter(m.cwLogStreams, m.filterValue(filterCWLogStreams))
		if len(m.filteredCWLogStreams) == 0 {
			m.cwLogStreamIdx = 0
		} else if m.cwLogStreamIdx >= len(m.filteredCWLogStreams) {
			m.cwLogStreamIdx = len(m.filteredCWLogStreams) - 1
		}
		m.screen = screenCWLogStreamList
		return m, nil, true

	case cwLogEventsLoadedMsg:
		if msg.append {
			m.cwLogEvents = appendUniqueCWLogEvents(m.cwLogEvents, msg.events)
			// Auto-scroll to bottom when tailing
			if m.cwLogTailing {
				total := len(m.cwLogViewerLines())
				visibleLines := max(m.height-8, 5)
				m.cwLogScrollOffset = clampCWLogScrollOffset(total-visibleLines, total, visibleLines)
			}
		} else {
			m.cwLogEvents = msg.events
			visibleLines := max(m.height-8, 5)
			m.cwLogScrollOffset = clampCWLogScrollOffset(m.cwLogScrollOffset, len(m.cwLogViewerLines()), visibleLines)
		}
		if msg.updatePaginationToken {
			m.cwLogNextToken = msg.nextToken
		}
		if msg.updateTailToken {
			m.cwLogTailToken = msg.nextToken
		}
		m.screen = screenCWLogViewer
		return m, nil, true

	case cwLogTailTickMsg:
		if m.cwLogTailing && m.selectedCWLogGroup != nil {
			return m, tea.Batch(m.pollCWLogTail(), m.tickCWLogTail()), true
		}
		return m, nil, true
	}
	return m, nil, false
}

// --- Log Group List ---

func (m Model) updateCWLogGroupList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterCWLogGroups); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterCWLogGroups)
	case "up", "k":
		if m.cwLogGroupIdx > 0 {
			m.cwLogGroupIdx--
		}
	case "down", "j":
		if m.cwLogGroupIdx < len(m.filteredCWLogGroups)-1 {
			m.cwLogGroupIdx++
		}
	case "/":
		return m, m.activateFilter(filterCWLogGroups)
	case "n":
		if m.cwLogGroupNextToken != nil {
			return m.startLoading(m.loadCWLogGroups(true))
		}
	case "enter":
		if len(m.filteredCWLogGroups) > 0 && m.cwLogGroupIdx < len(m.filteredCWLogGroups) {
			selected := m.filteredCWLogGroups[m.cwLogGroupIdx]
			m.selectedCWLogGroup = &selected
			return m.startLoading(m.loadCWLogStreams(selected.Name, false))
		}
	}
	return m, nil
}

func (m Model) viewCWLogGroupList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("CloudWatch Log Groups"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterCWLogGroups))
	b.WriteString("\n\n")

	if len(m.filteredCWLogGroups) == 0 {
		panel.WriteString(dimStyle.Render("  No matching log groups"))
		panel.WriteString("\n")
	} else {
		maxName := 4 // "NAME"
		for _, g := range m.filteredCWLogGroups {
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
		if m.cwLogGroupIdx >= visibleLines {
			start = m.cwLogGroupIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredCWLogGroups))

		for i := start; i < end; i++ {
			g := m.filteredCWLogGroups[i]
			cursor := "  "
			style := normalStyle
			if i == m.cwLogGroupIdx {
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
				nameCol.Inherit(style).Render(name) +
				retCol.Inherit(dimStyle).Render(retention) +
				dimStyle.Render(awsservice.FormatBytes(g.StoredBytes))
			panel.WriteString(row)
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		countLine := fmt.Sprintf("  %d/%d log groups", len(m.filteredCWLogGroups), len(m.cwLogGroups))
		if m.cwLogGroupNextToken != nil {
			countLine += " • more available"
		}
		panel.WriteString(dimStyle.Render(countLine))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • n: load more • enter: streams • esc: back • H: home"))
	return b.String()
}

// --- Log Stream List ---

func (m Model) updateCWLogStreamList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterCWLogStreams); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenCWLogGroupList
		m.resetFilter(filterCWLogStreams)
	case "up", "k":
		if m.cwLogStreamIdx > 0 {
			m.cwLogStreamIdx--
		}
	case "down", "j":
		if m.cwLogStreamIdx < len(m.filteredCWLogStreams)-1 {
			m.cwLogStreamIdx++
		}
	case "/":
		return m, m.activateFilter(filterCWLogStreams)
	case "n":
		if m.selectedCWLogGroup != nil && m.cwLogStreamNextToken != nil {
			return m.startLoading(m.loadCWLogStreams(m.selectedCWLogGroup.Name, true))
		}
	case "enter":
		if len(m.filteredCWLogStreams) > 0 && m.cwLogStreamIdx < len(m.filteredCWLogStreams) {
			selected := m.filteredCWLogStreams[m.cwLogStreamIdx]
			m.selectedCWLogStream = &selected
			m.cwLogTimeRange = 2 // default: 1h
			m.resetFilter(filterCWLogViewer)
			m.cwLogTailing = false
			m.cwLogTailToken = nil
			m.cwLogWrap = true
			m.cwLogHorizontalOffset = 0
			m.cwLogScrollOffset = 0
			return m.startLoading(m.loadCWLogEvents(false))
		}
	}
	return m, nil
}

func (m Model) viewCWLogStreamList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	groupName := ""
	if m.selectedCWLogGroup != nil {
		groupName = m.selectedCWLogGroup.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Log Streams — %s", groupName)))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterCWLogStreams))
	b.WriteString("\n\n")

	if len(m.filteredCWLogStreams) == 0 {
		panel.WriteString(dimStyle.Render("  No matching log streams"))
		panel.WriteString("\n")
	} else {
		maxName := 4
		for _, s := range m.filteredCWLogStreams {
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
		if m.cwLogStreamIdx >= visibleLines {
			start = m.cwLogStreamIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredCWLogStreams))

		for i := start; i < end; i++ {
			s := m.filteredCWLogStreams[i]
			cursor := "  "
			style := normalStyle
			if i == m.cwLogStreamIdx {
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
				nameCol.Inherit(style).Render(name) +
				dimStyle.Render(lastEvent)
			panel.WriteString(row)
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		countLine := fmt.Sprintf("  %d/%d streams", len(m.filteredCWLogStreams), len(m.cwLogStreams))
		if m.cwLogStreamNextToken != nil {
			countLine += " • more available"
		}
		panel.WriteString(dimStyle.Render(countLine))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • n: load more • enter: view logs • esc: back • H: home"))
	return b.String()
}

// --- Log Viewer ---

func (m Model) updateCWLogViewer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.isFiltering(filterCWLogViewer) {
		if key == "enter" {
			m.deactivateFilter()
			return m.startLoading(m.loadCWLogEvents(false))
		}
		if cmd, handled := m.updateSharedFilter(msg, filterCWLogViewer); handled {
			return m, cmd
		}
	}

	visibleLines := max(m.height-8, 5)

	switch key {
	case "q", "esc":
		m.cwLogTailing = false
		m.screen = screenCWLogStreamList
	case "up", "k":
		if m.cwLogScrollOffset > 0 {
			m.cwLogScrollOffset--
		}
	case "down", "j":
		totalLines := len(m.cwLogViewerLines())
		maxOffset := clampCWLogScrollOffset(totalLines-visibleLines, totalLines, visibleLines)
		if m.cwLogScrollOffset < maxOffset {
			m.cwLogScrollOffset++
		}
	case "pgup":
		m.cwLogScrollOffset -= visibleLines
		if m.cwLogScrollOffset < 0 {
			m.cwLogScrollOffset = 0
		}
	case "pgdown":
		m.cwLogScrollOffset += visibleLines
		m.cwLogScrollOffset = clampCWLogScrollOffset(m.cwLogScrollOffset, len(m.cwLogViewerLines()), visibleLines)
	case "t": // Toggle live tail
		m.cwLogTailing = !m.cwLogTailing
		if m.cwLogTailing {
			return m, m.tickCWLogTail()
		}
	case "f": // Filter pattern input
		return m, m.activateFilter(filterCWLogViewer)
	case "w":
		m.cwLogWrap = !m.cwLogWrap
		if m.cwLogWrap {
			m.cwLogHorizontalOffset = 0
		}
		m.cwLogScrollOffset = clampCWLogScrollOffset(m.cwLogScrollOffset, len(m.cwLogViewerLines()), visibleLines)
	case "h":
		if !m.cwLogWrap && m.cwLogHorizontalOffset > 0 {
			m.cwLogHorizontalOffset = max(m.cwLogHorizontalOffset-8, 0)
		}
	case "l":
		if !m.cwLogWrap {
			m.cwLogHorizontalOffset = min(m.cwLogHorizontalOffset+8, m.maxCWLogHorizontalOffset())
		}
	case "n": // Load more (older events)
		if m.cwLogNextToken != nil {
			return m, m.loadCWLogEvents(true)
		}
	case "1", "2", "3", "4", "5", "6": // Time range presets
		idx := int(key[0] - '1')
		if idx >= 0 && idx < len(cwTimeRanges) {
			m.cwLogTimeRange = idx
			m.cwLogTailing = false
			return m.startLoading(m.loadCWLogEvents(false))
		}
	}
	return m, nil
}

func (m Model) viewCWLogViewer() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())

	// Header
	groupName := ""
	if m.selectedCWLogGroup != nil {
		groupName = m.selectedCWLogGroup.Name
	}
	streamName := "All streams"
	if m.selectedCWLogStream != nil {
		streamName = m.selectedCWLogStream.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Logs — %s / %s", groupName, streamName)))
	b.WriteString("\n")

	// Status line: time range + filter + tail indicator
	var status []string
	if m.cwLogTimeRange >= 0 && m.cwLogTimeRange < len(cwTimeRanges) {
		// Build time range selector
		var ranges []string
		for i, tr := range cwTimeRanges {
			if i == m.cwLogTimeRange {
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
	if m.cwLogTailing {
		status = append(status, filterStyle.Render("TAILING"))
	}
	if len(status) > 0 {
		b.WriteString(strings.Join(status, "  "))
	}
	b.WriteString("\n\n")

	// Log events
	if len(m.cwLogEvents) == 0 {
		b.WriteString(dimStyle.Render("  No log events found"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-8, 5)
		lines := m.cwLogViewerLines()
		start := clampCWLogScrollOffset(m.cwLogScrollOffset, len(lines), visibleLines)
		end := min(start+visibleLines, len(lines))

		for _, line := range lines[start:end] {
			b.WriteString(line)
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d events (showing lines %d-%d)", len(m.cwLogEvents), start+1, end)))
	}

	b.WriteString("\n")
	wrapStatus := "off"
	if m.cwLogWrap {
		wrapStatus = "on"
	}
	hint := fmt.Sprintf("↑/↓: scroll • pgup/pgdn: page • 1-6: time range • f: filter • t: tail • w: wrap (%s)", wrapStatus)
	if !m.cwLogWrap {
		hint += fmt.Sprintf(" • h/l: horizontal (%d)", m.cwLogHorizontalOffset)
	}
	hint += " • n: load more • esc: back"
	b.WriteString(m.renderHelpBar(hint))
	return b.String()
}

func (m Model) cwLogViewerLines() []string {
	lines := make([]string, 0, len(m.cwLogEvents))
	for _, evt := range m.cwLogEvents {
		lines = append(lines, m.renderCWLogEventLines(evt)...)
	}
	return lines
}

func (m Model) renderCWLogEventLines(evt awsservice.LogEvent) []string {
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
		if m.cwLogWrap {
			prefixPlain := continuationPrefix
			prefixStyled := continuationPrefix
			if firstLine {
				prefixPlain = firstPrefixPlain
				prefixStyled = firstPrefixStyled
			}
			wrapped := wrapCWLogMessage(line, max(m.cwLogContentWidth()-lipgloss.Width(prefixPlain), 8))
			for idx, segment := range wrapped {
				if firstLine && idx == 0 {
					rendered = append(rendered, prefixStyled+segment)
					continue
				}
				rendered = append(rendered, continuationPrefix+segment)
			}
		} else {
			segment := sliceCWLogMessage(line, m.cwLogHorizontalOffset)
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

func (m Model) cwLogContentWidth() int {
	if m.width <= 0 {
		return 120
	}
	return max(m.width-2, 20)
}

func (m Model) maxCWLogHorizontalOffset() int {
	maxWidth := 0
	for _, evt := range m.cwLogEvents {
		for _, line := range strings.Split(strings.TrimRight(evt.Message, "\n"), "\n") {
			maxWidth = max(maxWidth, lipgloss.Width(strings.TrimRight(line, "\r")))
		}
	}
	return max(maxWidth-m.cwLogContentWidth()/2, 0)
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

// --- Commands ---

func (m Model) loadCWLogGroups(appendMode bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		var nextToken *string
		if appendMode {
			nextToken = m.cwLogGroupNextToken
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

func (m Model) loadCWLogStreams(logGroupName string, appendMode bool) tea.Cmd {
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

		var nextToken *string
		if appendMode {
			nextToken = m.cwLogStreamNextToken
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

func (m Model) loadCWLogEvents(appendMode bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return cwLogEventsLoadedMsg{append: appendMode}
			}
		}

		if m.selectedCWLogGroup == nil {
			return cwLogEventsLoadedMsg{append: appendMode}
		}

		now := time.Now()
		timeIdx := m.cwLogTimeRange
		if timeIdx < 0 || timeIdx >= len(cwTimeRanges) {
			timeIdx = 2 // default: 1h
		}
		duration := cwTimeRanges[timeIdx].duration
		startTime := now.Add(-duration).UnixMilli()
		endTime := now.UnixMilli()

		var token *string
		if appendMode {
			token = m.cwLogNextToken
		}

		events, nextToken, err := repo.FilterLogEvents(ctx, m.selectedCWLogGroup.Name, startTime, endTime, m.filterValue(filterCWLogViewer), token)
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

func (m Model) pollCWLogTail() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return cwLogEventsLoadedMsg{append: true}
			}
		}

		if m.selectedCWLogGroup == nil {
			return cwLogEventsLoadedMsg{append: true}
		}

		// Poll from last token or last 30 seconds
		now := time.Now()
		startTime := now.Add(-30 * time.Second).UnixMilli()
		endTime := now.UnixMilli()

		events, nextToken, err := repo.FilterLogEvents(ctx, m.selectedCWLogGroup.Name, startTime, endTime, m.filterValue(filterCWLogViewer), m.cwLogTailToken)
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

func (m Model) tickCWLogTail() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return cwLogTailTickMsg{}
	})
}
