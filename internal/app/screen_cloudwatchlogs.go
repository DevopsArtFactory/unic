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

func (m Model) handleCloudWatchLogsMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case cwLogGroupsLoadedMsg:
		m.cwLogGroups = msg.groups
		m.filteredCWLogGroups = msg.groups
		m.cwLogGroupIdx = 0
		m.screen = screenCWLogGroupList
		return m, nil, true

	case cwLogStreamsLoadedMsg:
		m.cwLogStreams = msg.streams
		m.filteredCWLogStreams = msg.streams
		m.cwLogStreamIdx = 0
		m.screen = screenCWLogStreamList
		return m, nil, true

	case cwLogEventsLoadedMsg:
		if msg.append {
			m.cwLogEvents = append(m.cwLogEvents, msg.events...)
			// Auto-scroll to bottom when tailing
			if m.cwLogTailing {
				total := len(m.cwLogEvents)
				visibleLines := max(m.height-8, 5)
				m.cwLogScrollOffset = clampCWLogScrollOffset(total-visibleLines, total, visibleLines)
			}
		} else {
			m.cwLogEvents = msg.events
			visibleLines := max(m.height-8, 5)
			m.cwLogScrollOffset = clampCWLogScrollOffset(m.cwLogScrollOffset, len(m.cwLogEvents), visibleLines)
		}
		m.cwLogNextToken = msg.nextToken
		m.cwLogTailToken = msg.nextToken
		m.screen = screenCWLogViewer
		return m, nil, true

	case cwLogTailTickMsg:
		if m.cwLogTailing && m.selectedCWLogGroup != nil {
			return m, m.pollCWLogTail(), true
		}
		return m, nil, true
	}
	return m, nil, false
}

// --- Log Group List ---

func (m Model) updateCWLogGroupList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.cwLogGroupFilterActive {
		newFilter, deactivate, changed := handleFilterKey(key, m.cwLogGroupFilter)
		m.cwLogGroupFilter = newFilter
		if deactivate {
			m.cwLogGroupFilterActive = false
		}
		if changed {
			m.filteredCWLogGroups = applyFilter(m.cwLogGroups, m.cwLogGroupFilter)
			m.cwLogGroupIdx = 0
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.cwLogGroupFilter = ""
		m.filteredCWLogGroups = m.cwLogGroups
		m.cwLogGroupIdx = 0
	case "up", "k":
		if m.cwLogGroupIdx > 0 {
			m.cwLogGroupIdx--
		}
	case "down", "j":
		if m.cwLogGroupIdx < len(m.filteredCWLogGroups)-1 {
			m.cwLogGroupIdx++
		}
	case "/":
		m.cwLogGroupFilterActive = true
	case "enter":
		if len(m.filteredCWLogGroups) > 0 && m.cwLogGroupIdx < len(m.filteredCWLogGroups) {
			selected := m.filteredCWLogGroups[m.cwLogGroupIdx]
			m.selectedCWLogGroup = &selected
			m.screen = screenLoading
			return m, m.loadCWLogStreams(selected.Name)
		}
	}
	return m, nil
}

func (m Model) viewCWLogGroupList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("CloudWatch Log Groups"))
	b.WriteString("\n")

	if m.cwLogGroupFilterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.cwLogGroupFilter)))
	} else if m.cwLogGroupFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.cwLogGroupFilter)))
	}
	b.WriteString("\n\n")

	if len(m.filteredCWLogGroups) == 0 {
		b.WriteString(dimStyle.Render("  No matching log groups"))
		b.WriteString("\n")
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

		b.WriteString(dimStyle.Render("  " + nameCol.Render("NAME") + retCol.Render("RETENTION") + "SIZE"))
		b.WriteString("\n")

		visibleLines := max(m.height-9, 5)
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
			b.WriteString(row)
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d log groups", len(m.filteredCWLogGroups), len(m.cwLogGroups))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • enter: streams • esc: back • H: home"))
	return b.String()
}

// --- Log Stream List ---

func (m Model) updateCWLogStreamList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.cwLogStreamFilterActive {
		newFilter, deactivate, changed := handleFilterKey(key, m.cwLogStreamFilter)
		m.cwLogStreamFilter = newFilter
		if deactivate {
			m.cwLogStreamFilterActive = false
		}
		if changed {
			m.filteredCWLogStreams = applyFilter(m.cwLogStreams, m.cwLogStreamFilter)
			m.cwLogStreamIdx = 0
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.screen = screenCWLogGroupList
		m.cwLogStreamFilter = ""
		m.filteredCWLogStreams = m.cwLogStreams
		m.cwLogStreamIdx = 0
	case "up", "k":
		if m.cwLogStreamIdx > 0 {
			m.cwLogStreamIdx--
		}
	case "down", "j":
		if m.cwLogStreamIdx < len(m.filteredCWLogStreams)-1 {
			m.cwLogStreamIdx++
		}
	case "/":
		m.cwLogStreamFilterActive = true
	case "enter":
		if len(m.filteredCWLogStreams) > 0 && m.cwLogStreamIdx < len(m.filteredCWLogStreams) {
			selected := m.filteredCWLogStreams[m.cwLogStreamIdx]
			m.selectedCWLogStream = &selected
			m.cwLogTimeRange = 2 // default: 1h
			m.cwLogFilterPattern = ""
			m.cwLogTailing = false
			m.cwLogTailToken = nil
			m.screen = screenLoading
			return m, m.loadCWLogEvents(false)
		}
	}
	return m, nil
}

func (m Model) viewCWLogStreamList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	groupName := ""
	if m.selectedCWLogGroup != nil {
		groupName = m.selectedCWLogGroup.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Log Streams — %s", groupName)))
	b.WriteString("\n")

	if m.cwLogStreamFilterActive {
		b.WriteString(filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.cwLogStreamFilter)))
	} else if m.cwLogStreamFilter != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("Filter: %s", m.cwLogStreamFilter)))
	}
	b.WriteString("\n\n")

	if len(m.filteredCWLogStreams) == 0 {
		b.WriteString(dimStyle.Render("  No matching log streams"))
		b.WriteString("\n")
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

		b.WriteString(dimStyle.Render("  " + nameCol.Render("NAME") + "LAST EVENT"))
		b.WriteString("\n")

		visibleLines := max(m.height-9, 5)
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
			b.WriteString(row)
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d streams", len(m.filteredCWLogStreams), len(m.cwLogStreams))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • /: filter • enter: view logs • esc: back • H: home"))
	return b.String()
}

// --- Log Viewer ---

func (m Model) updateCWLogViewer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Filter pattern input mode
	if m.cwLogFilterActive {
		switch key {
		case "esc":
			m.cwLogFilterActive = false
		case "enter":
			m.cwLogFilterActive = false
			// Reload with new filter
			m.screen = screenLoading
			return m, m.loadCWLogEvents(false)
		case "backspace":
			if len(m.cwLogFilterPattern) > 0 {
				m.cwLogFilterPattern = m.cwLogFilterPattern[:len(m.cwLogFilterPattern)-1]
			}
		default:
			if len(key) == 1 {
				m.cwLogFilterPattern += key
			}
		}
		return m, nil
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
		maxOffset := clampCWLogScrollOffset(len(m.cwLogEvents)-visibleLines, len(m.cwLogEvents), visibleLines)
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
		m.cwLogScrollOffset = clampCWLogScrollOffset(m.cwLogScrollOffset, len(m.cwLogEvents), visibleLines)
	case "t": // Toggle live tail
		m.cwLogTailing = !m.cwLogTailing
		if m.cwLogTailing {
			return m, m.tickCWLogTail()
		}
	case "f": // Filter pattern input
		m.cwLogFilterActive = true
	case "n": // Load more (older events)
		if m.cwLogNextToken != nil {
			return m, m.loadCWLogEvents(true)
		}
	case "1", "2", "3", "4", "5", "6": // Time range presets
		idx := int(key[0] - '1')
		if idx >= 0 && idx < len(cwTimeRanges) {
			m.cwLogTimeRange = idx
			m.cwLogTailing = false
			m.screen = screenLoading
			return m, m.loadCWLogEvents(false)
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
	if m.cwLogFilterActive {
		status = append(status, filterStyle.Render(fmt.Sprintf("Filter: %s▏", m.cwLogFilterPattern)))
	} else if m.cwLogFilterPattern != "" {
		status = append(status, dimStyle.Render(fmt.Sprintf("Filter: %s", m.cwLogFilterPattern)))
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
		start := clampCWLogScrollOffset(m.cwLogScrollOffset, len(m.cwLogEvents), visibleLines)
		end := min(start+visibleLines, len(m.cwLogEvents))

		for i := start; i < end; i++ {
			evt := m.cwLogEvents[i]
			ts := dimStyle.Render(evt.Timestamp.Local().Format("15:04:05.000"))
			msg := strings.TrimSpace(evt.Message)

			var levelStr string
			switch evt.Level {
			case "ERROR", "FATAL":
				levelStr = logErrorStyle.Render(fmt.Sprintf("[%s]", evt.Level))
			case "WARN":
				levelStr = logWarnStyle.Render("[WARN]")
			case "INFO":
				levelStr = logInfoStyle.Render("[INFO]")
			case "DEBUG":
				levelStr = logDebugStyle.Render("[DEBUG]")
			}

			if levelStr != "" {
				b.WriteString(fmt.Sprintf("  %s %s %s", ts, levelStr, msg))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s", ts, msg))
			}
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d events (showing %d-%d)", len(m.cwLogEvents), start+1, end)))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: scroll • pgup/pgdn: page • 1-6: time range • f: filter • t: tail • n: load more • esc: back"))
	return b.String()
}

// --- Commands ---

func (m Model) loadCWLogGroups() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		groups, err := repo.ListLogGroups(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(groups) == 0 {
			return errMsg{err: fmt.Errorf("no log groups found")}
		}
		return cwLogGroupsLoadedMsg{groups: groups}
	}
}

func (m Model) loadCWLogStreams(logGroupName string) tea.Cmd {
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

		streams, err := repo.ListLogStreams(ctx, logGroupName)
		if err != nil {
			return errMsg{err: err}
		}
		if len(streams) == 0 {
			return errMsg{err: fmt.Errorf("no log streams found in %s", logGroupName)}
		}
		return cwLogStreamsLoadedMsg{streams: streams}
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

		events, nextToken, err := repo.FilterLogEvents(ctx, m.selectedCWLogGroup.Name, startTime, endTime, m.cwLogFilterPattern, token)
		if err != nil {
			return errMsg{err: err}
		}

		return cwLogEventsLoadedMsg{
			events:    events,
			nextToken: nextToken,
			append:    appendMode,
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

		events, nextToken, err := repo.FilterLogEvents(ctx, m.selectedCWLogGroup.Name, startTime, endTime, m.cwLogFilterPattern, m.cwLogTailToken)
		if err != nil {
			return cwLogEventsLoadedMsg{append: true}
		}

		return cwLogEventsLoadedMsg{
			events:    events,
			nextToken: nextToken,
			append:    true,
		}
	}
}

func (m Model) tickCWLogTail() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return cwLogTailTickMsg{}
	})
}
