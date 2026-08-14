package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

var cwMetricSparklineBlocks = []rune("▁▂▃▄▅▆▇█")

const maxCWMetricComparisonSeries = 4

type cwMetricRangeOption struct {
	label    string
	duration time.Duration
}

type cwMetricPeriodOption struct {
	label    string
	duration time.Duration
}

type cwMetricPresetRule struct {
	namespace   string
	metricNames []string
}

type cwMetricPreset struct {
	label       string
	description string
	rules       []cwMetricPresetRule
}

var cwMetricRangeOptions = []cwMetricRangeOption{
	{label: "15m", duration: 15 * time.Minute},
	{label: "1h", duration: time.Hour},
	{label: "6h", duration: 6 * time.Hour},
	{label: "24h", duration: 24 * time.Hour},
	{label: "7d", duration: 7 * 24 * time.Hour},
}

var cwMetricPeriodOptions = []cwMetricPeriodOption{
	{label: "1m", duration: time.Minute},
	{label: "5m", duration: 5 * time.Minute},
	{label: "15m", duration: 15 * time.Minute},
	{label: "1h", duration: time.Hour},
}

var cwMetricStatOptions = []string{
	"Average",
	"Maximum",
	"Minimum",
	"Sum",
}

var cwMetricPresets = []cwMetricPreset{
	{
		label:       "All Metrics",
		description: "Show every discovered CloudWatch metric series in the current region.",
	},
	{
		label:       "EC2 CPU / Network",
		description: "Focus on instance CPU, traffic, and status-check signals for EC2 triage.",
		rules: []cwMetricPresetRule{
			{namespace: "AWS/EC2", metricNames: []string{"CPUUtilization", "NetworkIn", "NetworkOut", "StatusCheckFailed", "StatusCheckFailed_Instance", "StatusCheckFailed_System"}},
		},
	},
	{
		label:       "RDS Connections / Storage",
		description: "Show connection pressure and storage headroom metrics for DB instances.",
		rules: []cwMetricPresetRule{
			{namespace: "AWS/RDS", metricNames: []string{"DatabaseConnections", "FreeStorageSpace", "FreeableMemory", "ReadIOPS", "WriteIOPS", "CPUUtilization"}},
		},
	},
	{
		label:       "ECS Service",
		description: "Show service-level utilization metrics from the standard AWS/ECS namespace.",
		rules: []cwMetricPresetRule{
			{namespace: "AWS/ECS", metricNames: []string{"CPUUtilization", "MemoryUtilization", "CPUReservation", "MemoryReservation"}},
		},
	},
	{
		label:       "ECS Task",
		description: "Show task/container-level metrics where Container Insights publishes them cleanly.",
		rules: []cwMetricPresetRule{
			{namespace: "ECS/ContainerInsights", metricNames: []string{"CpuUtilized", "CpuReserved", "MemoryUtilized", "MemoryReserved", "RunningTaskCount", "PendingTaskCount", "RestartCount"}},
		},
	},
}

type cloudWatchMetricsModel struct {
	metrics           []awsservice.CloudWatchMetric
	filteredCWMetrics []awsservice.CloudWatchMetric
	metricIdx         int
	selectedMetric    *awsservice.CloudWatchMetric
	selectedSeries    *awsservice.CloudWatchMetricSeriesData
	selectedSeriesSet []*awsservice.CloudWatchMetricSeriesData
	comparisonMetrics []awsservice.CloudWatchMetric
	selectionNotice   string
	detailScroll      int
	presetIdx         int
	timeRangeIdx      int
	periodIdx         int
	statIdx           int
}

func newCloudWatchMetricsModel() cloudWatchMetricsModel {
	return cloudWatchMetricsModel{
		presetIdx:    0,
		timeRangeIdx: 1,
		periodIdx:    0,
		statIdx:      0,
	}
}

func (cw *cloudWatchMetricsModel) Start(m *Model) (tea.Model, tea.Cmd) {
	cw.selectedMetric = nil
	cw.selectedSeries = nil
	cw.selectedSeriesSet = nil
	cw.comparisonMetrics = nil
	cw.selectionNotice = ""
	cw.detailScroll = 0
	return m.startLoadingWithMessage(
		"Loading CloudWatch metrics...",
		[]string{
			"Fetching metric definitions from the current account and region.",
			cw.controlsSummary(),
		},
		cw.loadMetrics(*m),
	)
}

func (cw *cloudWatchMetricsModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case cwMetricsLoadedMsg:
		cw.metrics = msg.metrics
		cw.applyMetricFilters(m)
		cw.comparisonMetrics = nil
		cw.selectionNotice = ""
		m.screen = screenCWMetricList
		return *m, nil, true
	case cwMetricDataLoadedMsg:
		cw.selectedMetric = nil
		cw.selectedSeries = nil
		cw.selectedSeriesSet = msg.series
		if len(msg.metrics) > 0 {
			metric := msg.metrics[0]
			cw.selectedMetric = &metric
		}
		if len(msg.series) > 0 {
			cw.selectedSeries = msg.series[0]
			if cw.selectedMetric == nil {
				metric := msg.series[0].Metric
				cw.selectedMetric = &metric
			}
		}
		cw.detailScroll = 0
		m.screen = screenCWMetricDetail
		return *m, nil, true
	default:
		return *m, nil, false
	}
}

func (cw *cloudWatchMetricsModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenCWMetricList:
		newM, cmd := cw.updateMetricList(m, msg)
		return newM, cmd, true
	case screenCWMetricDetail:
		newM, cmd := cw.updateMetricDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (cw cloudWatchMetricsModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenCWMetricList:
		return cw.viewMetricList(m), true
	case screenCWMetricDetail:
		return cw.viewMetricDetail(m), true
	default:
		return "", false
	}
}

func (cw *cloudWatchMetricsModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterCWMetrics {
		return false
	}
	cw.applyMetricFilters(m)
	return true
}

func (cw *cloudWatchMetricsModel) updateMetricList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterCWMetrics); handled {
		return *m, cmd
	}

	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterCWMetrics)
	case "up", "k":
		cw.metricIdx = previousListIndex(cw.metricIdx, len(cw.filteredCWMetrics))
	case "down", "j":
		cw.metricIdx = nextListIndex(cw.metricIdx, len(cw.filteredCWMetrics))
	case "/":
		return *m, m.activateFilter(filterCWMetrics)
	case "r":
		return m.startLoading(cw.loadMetrics(*m))
	case "g":
		cw.presetIdx = (cw.presetIdx + 1) % len(cwMetricPresets)
		cw.comparisonMetrics = nil
		cw.selectionNotice = ""
		cw.applyMetricFilters(m)
	case "t":
		cw.timeRangeIdx = (cw.timeRangeIdx + 1) % len(cwMetricRangeOptions)
	case "p":
		cw.periodIdx = (cw.periodIdx + 1) % len(cwMetricPeriodOptions)
	case "s":
		cw.statIdx = (cw.statIdx + 1) % len(cwMetricStatOptions)
	case " ":
		if len(cw.filteredCWMetrics) == 0 || cw.metricIdx >= len(cw.filteredCWMetrics) {
			return *m, nil
		}
		cw.toggleComparisonMetric(cw.filteredCWMetrics[cw.metricIdx])
	case "enter":
		if len(cw.filteredCWMetrics) == 0 || cw.metricIdx >= len(cw.filteredCWMetrics) {
			return *m, nil
		}
		selected := []awsservice.CloudWatchMetric{cw.filteredCWMetrics[cw.metricIdx]}
		if len(cw.comparisonMetrics) > 0 {
			selected = append([]awsservice.CloudWatchMetric(nil), cw.comparisonMetrics...)
		}
		title := "Loading CloudWatch metric series..."
		if len(selected) > 1 {
			title = "Loading CloudWatch metric comparison..."
		}
		return m.startLoadingWithMessage(
			title,
			[]string{
				cw.loadingMetricSummary(selected),
				cw.controlsSummary(),
			},
			cw.loadSeries(*m, selected),
		)
	}
	return *m, nil
}

func (cw *cloudWatchMetricsModel) updateMetricDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lineCount := len(cw.detailLines(m.width))
	visibleLines := max(m.height-10, 5)
	maxOffset := max(lineCount-visibleLines, 0)

	switch msg.String() {
	case "q", "esc":
		cw.detailScroll = 0
		m.screen = screenCWMetricList
	case "up", "k":
		if cw.detailScroll > 0 {
			cw.detailScroll--
		}
	case "down", "j":
		if cw.detailScroll < maxOffset {
			cw.detailScroll++
		}
	case "pgup":
		cw.detailScroll -= visibleLines
		if cw.detailScroll < 0 {
			cw.detailScroll = 0
		}
	case "pgdn":
		cw.detailScroll += visibleLines
		if cw.detailScroll > maxOffset {
			cw.detailScroll = maxOffset
		}
	case "r":
		selected := cw.detailMetrics()
		if len(selected) == 0 {
			return *m, nil
		}
		return m.startLoadingWithMessage(
			cw.detailLoadingTitle(),
			[]string{cw.loadingMetricSummary(selected), cw.controlsSummary()},
			cw.loadSeries(*m, selected),
		)
	case "t":
		cw.timeRangeIdx = (cw.timeRangeIdx + 1) % len(cwMetricRangeOptions)
		selected := cw.detailMetrics()
		if len(selected) == 0 {
			return *m, nil
		}
		return m.startLoadingWithMessage(
			cw.detailLoadingTitle(),
			[]string{cw.loadingMetricSummary(selected), cw.controlsSummary()},
			cw.loadSeries(*m, selected),
		)
	case "p":
		cw.periodIdx = (cw.periodIdx + 1) % len(cwMetricPeriodOptions)
		selected := cw.detailMetrics()
		if len(selected) == 0 {
			return *m, nil
		}
		return m.startLoadingWithMessage(
			cw.detailLoadingTitle(),
			[]string{cw.loadingMetricSummary(selected), cw.controlsSummary()},
			cw.loadSeries(*m, selected),
		)
	case "s":
		cw.statIdx = (cw.statIdx + 1) % len(cwMetricStatOptions)
		selected := cw.detailMetrics()
		if len(selected) == 0 {
			return *m, nil
		}
		return m.startLoadingWithMessage(
			cw.detailLoadingTitle(),
			[]string{cw.loadingMetricSummary(selected), cw.controlsSummary()},
			cw.loadSeries(*m, selected),
		)
	}
	return *m, nil
}

func (cw cloudWatchMetricsModel) viewMetricList(m Model) string {
	var b strings.Builder
	var panel strings.Builder

	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("CloudWatch Metrics"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(cw.controlsSummary()))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(cw.selectedPreset().description))
	if len(cw.comparisonMetrics) > 0 {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("Selected for comparison: %d/%d", len(cw.comparisonMetrics), maxCWMetricComparisonSeries)))
	}
	if cw.selectionNotice != "" {
		b.WriteString("\n")
		b.WriteString(warningStyle.Render(cw.selectionNotice))
	}
	b.WriteString("\n\n")

	b.WriteString(m.renderFilterValue(filterCWMetrics))
	b.WriteString("\n\n")

	if len(cw.filteredCWMetrics) == 0 {
		switch {
		case len(cw.metrics) == 0:
			panel.WriteString(dimStyle.Render("  No CloudWatch metrics were found in this account/region."))
			panel.WriteString("\n")
			panel.WriteString(dimStyle.Render("  Verify that the current region has emitted metrics and press r to refresh."))
		case m.filterValue(filterCWMetrics) != "":
			panel.WriteString(dimStyle.Render("  No metrics matched the current preset and filter query."))
			panel.WriteString("\n")
			panel.WriteString(dimStyle.Render("  Press g to switch presets or clear the filter with esc."))
		default:
			panel.WriteString(dimStyle.Render("  No metrics matched the current preset."))
			panel.WriteString("\n")
			panel.WriteString(dimStyle.Render("  Press g to cycle to another preset group."))
		}
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-11, 5)
		start := 0
		if cw.metricIdx >= visibleLines {
			start = cw.metricIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(cw.filteredCWMetrics))

		for i := start; i < end; i++ {
			metric := cw.filteredCWMetrics[i]
			cursor := "  "
			marker := "[ ] "
			if cw.isComparisonMetricSelected(metric) {
				marker = "[x] "
			}
			style := normalStyle
			if i == cw.metricIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + marker + m.renderHighlightedValue(filterCWMetrics, metric.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d metric series", len(cw.filteredCWMetrics), len(cw.metrics))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • space: select • enter: chart • /: filter • g/t/p/s: controls • r: refresh • esc: back • H: home"))
	return b.String()
}

func (cw cloudWatchMetricsModel) viewMetricDetail(m Model) string {
	var b strings.Builder
	var panel strings.Builder

	b.WriteString(m.renderStatusBar())
	title := "CloudWatch Metric Detail"
	if len(cw.selectedSeriesSet) > 1 {
		title = "CloudWatch Metric Comparison"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render(cw.controlsSummary()))
	b.WriteString("\n")
	if description := cw.selectedPreset().description; description != "" {
		b.WriteString(dimStyle.Render(description))
		b.WriteString("\n\n")
	}

	lines := cw.detailLines(m.width)
	visibleLines := max(m.height-10, 5)
	scroll := clampCWMetricDetailScroll(cw.detailScroll, len(lines), visibleLines)
	end := min(scroll+visibleLines, len(lines))

	if len(lines) == 0 {
		panel.WriteString(dimStyle.Render("  No metric series selected"))
		panel.WriteString("\n")
	} else {
		for _, line := range lines[scroll:end] {
			panel.WriteString(line)
			panel.WriteString("\n")
		}
		if len(lines) > visibleLines {
			panel.WriteString("\n")
			panel.WriteString(dimStyle.Render(fmt.Sprintf("  Lines %d-%d of %d", scroll+1, end, len(lines))))
		}
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: scroll • pgup/pgdn: page • t: range • p: period • s: stat • r: refresh • esc: back • H: home"))
	return b.String()
}

func clampCWMetricDetailScroll(offset, total, visible int) int {
	maxOffset := total - visible
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

func (cw cloudWatchMetricsModel) detailLines(width int) []string {
	if cw.selectedMetric == nil {
		return nil
	}
	if len(cw.selectedSeriesSet) > 1 {
		return cw.comparisonDetailLines(width)
	}

	series := cw.selectedSeries
	lines := []string{
		renderDetailLine("Namespace", normalStyle.Render(cw.selectedMetric.Namespace)),
		renderDetailLine("Metric", normalStyle.Render(cw.selectedMetric.MetricName)),
	}
	if dims := cw.selectedMetric.DimensionsText(); dims != "" {
		lines = append(lines, renderDetailLine("Dimensions", normalStyle.Render(dims)))
	}
	if series == nil {
		lines = append(lines, "", dimStyle.Render("  Metric data not loaded"))
		return lines
	}

	lines = append(lines,
		renderDetailLine("Preset", normalStyle.Render(cw.selectedPreset().label)),
		renderDetailLine("Stat", normalStyle.Render(series.Stat)),
		renderDetailLine("Range", normalStyle.Render(fmt.Sprintf("%s to %s", series.StartTime.Local().Format("2006-01-02 15:04"), series.EndTime.Local().Format("2006-01-02 15:04")))),
		renderDetailLine("Period", normalStyle.Render(cw.selectedPeriod().label)),
		renderDetailLine("Points", normalStyle.Render(fmt.Sprintf("%d", len(series.Datapoints)))),
	)

	if series.StatusCode != "" {
		status := normalStyle.Render(series.StatusCode)
		if series.StatusCode != "Complete" {
			status = warningStyle.Render(series.StatusCode)
		}
		lines = append(lines, renderDetailLine("Status", status))
	}

	if latestValue, ok := series.LatestValue(); ok {
		lines = append(lines, renderDetailLine("Latest", selectedStyle.Render(formatMetricValue(latestValue))))
	}

	lines = append(lines, "", titleStyle.Render("Series"))
	if len(series.Datapoints) == 0 {
		lines = append(lines, dimStyle.Render("  No datapoints returned for the selected time window."))
		lines = append(lines, dimStyle.Render("  Try t to widen the range, p to change the period, or s to switch the statistic."))
	} else {
		chartWidth := max(width-8, 16)
		lines = append(lines, selectedStyle.Render("  "+renderMetricSparkline(series.Datapoints, chartWidth)))

		statsLine := []string{}
		if minValue, ok := series.MinValue(); ok {
			statsLine = append(statsLine, fmt.Sprintf("min %s", formatMetricValue(minValue)))
		}
		if avgValue, ok := series.AverageValue(); ok {
			statsLine = append(statsLine, fmt.Sprintf("avg %s", formatMetricValue(avgValue)))
		}
		if maxValue, ok := series.MaxValue(); ok {
			statsLine = append(statsLine, fmt.Sprintf("max %s", formatMetricValue(maxValue)))
		}
		if len(statsLine) > 0 {
			lines = append(lines, dimStyle.Render("  "+strings.Join(statsLine, "  ")))
		}
	}

	if len(series.Messages) > 0 {
		lines = append(lines, "", titleStyle.Render("Messages"))
		for _, message := range series.Messages {
			lines = append(lines, warningStyle.Render("  - "+message))
		}
	}

	lines = append(lines, "", titleStyle.Render("Datapoints"))
	if len(series.Datapoints) == 0 {
		lines = append(lines, dimStyle.Render("  No datapoints available"))
		return lines
	}

	for i := len(series.Datapoints) - 1; i >= 0; i-- {
		point := series.Datapoints[i]
		lines = append(lines, fmt.Sprintf("  %s  %s", dimStyle.Render(point.Timestamp.Local().Format("2006-01-02 15:04:05")), normalStyle.Render(formatMetricValue(point.Value))))
	}
	return lines
}

func (cw cloudWatchMetricsModel) comparisonDetailLines(width int) []string {
	seriesSet := cw.selectedSeriesSet
	if len(seriesSet) == 0 {
		return nil
	}

	first := seriesSet[0]
	lines := []string{
		renderDetailLine("Namespace", normalStyle.Render(first.Metric.Namespace)),
		renderDetailLine("Metric", normalStyle.Render(first.Metric.MetricName)),
		renderDetailLine("Preset", normalStyle.Render(cw.selectedPreset().label)),
		renderDetailLine("Stat", normalStyle.Render(first.Stat)),
		renderDetailLine("Range", normalStyle.Render(fmt.Sprintf("%s to %s", first.StartTime.Local().Format("2006-01-02 15:04"), first.EndTime.Local().Format("2006-01-02 15:04")))),
		renderDetailLine("Period", normalStyle.Render(cw.selectedPeriod().label)),
		renderDetailLine("Series", normalStyle.Render(fmt.Sprintf("%d", len(seriesSet)))),
	}

	lines = append(lines, "", titleStyle.Render("Legend"))
	for i, series := range seriesSet {
		symbol := cwMetricLegendSymbol(i)
		label := cloudWatchSeriesDisplayName(series)
		statsLine := []string{}
		if latestValue, ok := series.LatestValue(); ok {
			statsLine = append(statsLine, fmt.Sprintf("latest %s", formatMetricValue(latestValue)))
		}
		if avgValue, ok := series.AverageValue(); ok {
			statsLine = append(statsLine, fmt.Sprintf("avg %s", formatMetricValue(avgValue)))
		}
		detail := ""
		if len(statsLine) > 0 {
			detail = "  " + strings.Join(statsLine, "  ")
		}
		lines = append(lines, fmt.Sprintf("  %s  %s%s", selectedStyle.Render(symbol), normalStyle.Render(label), dimStyle.Render(detail)))
	}

	lines = append(lines, "", titleStyle.Render("Overlay"))
	chartWidth := max(width-10, 16)
	chartLines, minValue, maxValue := renderMetricComparisonOverlay(seriesSet, chartWidth, 8)
	if len(chartLines) == 0 {
		lines = append(lines, dimStyle.Render("  No datapoints returned for the selected time window."))
		lines = append(lines, dimStyle.Render("  Try t to widen the range, p to change the period, or s to switch the statistic."))
	} else {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  max %s", formatMetricValue(maxValue))))
		for _, line := range chartLines {
			lines = append(lines, selectedStyle.Render(line))
		}
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  min %s", formatMetricValue(minValue))))
	}

	lines = append(lines, "", titleStyle.Render("Stats"))
	for i, series := range seriesSet {
		statsLine := []string{}
		if minValue, ok := series.MinValue(); ok {
			statsLine = append(statsLine, fmt.Sprintf("min %s", formatMetricValue(minValue)))
		}
		if avgValue, ok := series.AverageValue(); ok {
			statsLine = append(statsLine, fmt.Sprintf("avg %s", formatMetricValue(avgValue)))
		}
		if maxValue, ok := series.MaxValue(); ok {
			statsLine = append(statsLine, fmt.Sprintf("max %s", formatMetricValue(maxValue)))
		}
		if len(statsLine) == 0 {
			statsLine = append(statsLine, "no datapoints")
		}
		lines = append(lines, fmt.Sprintf("  %s  %s", selectedStyle.Render(cwMetricLegendSymbol(i)), dimStyle.Render(strings.Join(statsLine, "  "))))
	}

	var messages []string
	for i, series := range seriesSet {
		for _, message := range series.Messages {
			messages = append(messages, fmt.Sprintf("%s %s", cwMetricLegendSymbol(i), message))
		}
	}
	if len(messages) > 0 {
		lines = append(lines, "", titleStyle.Render("Messages"))
		for _, message := range messages {
			lines = append(lines, warningStyle.Render("  - "+message))
		}
	}

	return lines
}

func renderMetricSparkline(points []awsservice.CloudWatchMetricDatapoint, width int) string {
	if len(points) == 0 {
		return ""
	}
	if width <= 0 {
		width = len(points)
	}

	values := bucketMetricValues(points, width)
	if len(values) == 0 {
		return ""
	}

	minValue := values[0]
	maxValue := values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}

	if maxValue == minValue {
		block := cwMetricSparklineBlocks[0]
		if maxValue > 0 {
			block = cwMetricSparklineBlocks[len(cwMetricSparklineBlocks)-1]
		}
		return strings.Repeat(string(block), len(values))
	}

	var b strings.Builder
	scale := float64(len(cwMetricSparklineBlocks) - 1)
	denominator := maxValue - minValue
	for _, value := range values {
		position := (value - minValue) / denominator
		idx := int(math.Round(position * scale))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(cwMetricSparklineBlocks) {
			idx = len(cwMetricSparklineBlocks) - 1
		}
		b.WriteRune(cwMetricSparklineBlocks[idx])
	}
	return b.String()
}

func renderMetricComparisonOverlay(seriesSet []*awsservice.CloudWatchMetricSeriesData, width, height int) ([]string, float64, float64) {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	bucketed := make([][]float64, 0, len(seriesSet))
	var minValue, maxValue float64
	haveValue := false
	for _, series := range seriesSet {
		if series == nil {
			bucketed = append(bucketed, nil)
			continue
		}
		values := bucketMetricValues(series.Datapoints, width)
		bucketed = append(bucketed, values)
		for _, value := range values {
			if !haveValue {
				minValue = value
				maxValue = value
				haveValue = true
				continue
			}
			if value < minValue {
				minValue = value
			}
			if value > maxValue {
				maxValue = value
			}
		}
	}
	if !haveValue {
		return nil, 0, 0
	}

	grid := make([][]rune, height)
	for i := range grid {
		grid[i] = make([]rune, width)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	denominator := maxValue - minValue
	for seriesIdx, values := range bucketed {
		symbol := []rune(cwMetricLegendSymbol(seriesIdx))[0]
		for x, value := range values {
			position := 0.0
			if denominator > 0 {
				position = (value - minValue) / denominator
			}
			y := height - 1 - int(math.Round(position*float64(height-1)))
			if y < 0 {
				y = 0
			}
			if y >= height {
				y = height - 1
			}
			if grid[y][x] != ' ' && grid[y][x] != symbol {
				grid[y][x] = '*'
				continue
			}
			grid[y][x] = symbol
		}
	}

	lines := make([]string, 0, height)
	for _, row := range grid {
		lines = append(lines, "  |"+string(row)+"|")
	}
	return lines, minValue, maxValue
}

func bucketMetricValues(points []awsservice.CloudWatchMetricDatapoint, width int) []float64 {
	if len(points) == 0 {
		return nil
	}
	if width <= 0 || width >= len(points) {
		values := make([]float64, len(points))
		for i, point := range points {
			values[i] = point.Value
		}
		return values
	}

	values := make([]float64, 0, width)
	for i := 0; i < width; i++ {
		start := i * len(points) / width
		end := (i + 1) * len(points) / width
		if end <= start {
			end = start + 1
		}
		if end > len(points) {
			end = len(points)
		}

		var total float64
		for _, point := range points[start:end] {
			total += point.Value
		}
		values = append(values, total/float64(end-start))
	}
	return values
}

func formatMetricValue(value float64) string {
	absValue := math.Abs(value)
	switch {
	case absValue >= 1000:
		return fmt.Sprintf("%.0f", value)
	case absValue >= 100:
		return fmt.Sprintf("%.1f", value)
	case absValue >= 1:
		return fmt.Sprintf("%.2f", value)
	default:
		return fmt.Sprintf("%.4f", value)
	}
}

func (cw *cloudWatchMetricsModel) toggleComparisonMetric(metric awsservice.CloudWatchMetric) {
	cw.selectionNotice = ""
	for i, selected := range cw.comparisonMetrics {
		if selected.IdentityKey() == metric.IdentityKey() {
			cw.comparisonMetrics = append(cw.comparisonMetrics[:i], cw.comparisonMetrics[i+1:]...)
			return
		}
	}
	if len(cw.comparisonMetrics) > 0 && !sameCloudWatchMetricComparisonGroup(cw.comparisonMetrics[0], metric) {
		cw.selectionNotice = "Select metrics with the same namespace and metric name for comparison."
		return
	}
	if len(cw.comparisonMetrics) >= maxCWMetricComparisonSeries {
		cw.selectionNotice = fmt.Sprintf("Comparison is limited to %d metric series.", maxCWMetricComparisonSeries)
		return
	}
	cw.comparisonMetrics = append(cw.comparisonMetrics, metric)
}

func (cw cloudWatchMetricsModel) isComparisonMetricSelected(metric awsservice.CloudWatchMetric) bool {
	for _, selected := range cw.comparisonMetrics {
		if selected.IdentityKey() == metric.IdentityKey() {
			return true
		}
	}
	return false
}

func (cw cloudWatchMetricsModel) detailMetrics() []awsservice.CloudWatchMetric {
	if len(cw.selectedSeriesSet) > 0 {
		metrics := make([]awsservice.CloudWatchMetric, 0, len(cw.selectedSeriesSet))
		for _, series := range cw.selectedSeriesSet {
			if series != nil {
				metrics = append(metrics, series.Metric)
			}
		}
		return metrics
	}
	if cw.selectedMetric != nil {
		return []awsservice.CloudWatchMetric{*cw.selectedMetric}
	}
	return nil
}

func (cw cloudWatchMetricsModel) detailLoadingTitle() string {
	if len(cw.selectedSeriesSet) > 1 {
		return "Refreshing CloudWatch metric comparison..."
	}
	return "Refreshing CloudWatch metric series..."
}

func (cw cloudWatchMetricsModel) loadingMetricSummary(metrics []awsservice.CloudWatchMetric) string {
	if len(metrics) == 0 {
		return "Metric: none"
	}
	first := metrics[0]
	if len(metrics) == 1 {
		return fmt.Sprintf("Metric: %s / %s", first.Namespace, first.MetricName)
	}
	return fmt.Sprintf("Metrics: %s / %s (%d series)", first.Namespace, first.MetricName, len(metrics))
}

func sameCloudWatchMetricComparisonGroup(left, right awsservice.CloudWatchMetric) bool {
	return strings.EqualFold(left.Namespace, right.Namespace) && strings.EqualFold(left.MetricName, right.MetricName)
}

func cwMetricLegendSymbol(index int) string {
	if index < 0 {
		return "?"
	}
	return string(rune('A' + index))
}

func cloudWatchSeriesDisplayName(series *awsservice.CloudWatchMetricSeriesData) string {
	if series == nil {
		return "unknown"
	}
	if series.Label != "" {
		return series.Label
	}
	if dims := series.Metric.DimensionsText(); dims != "" {
		return dims
	}
	return series.Metric.DisplayTitle()
}

func (cw cloudWatchMetricsModel) loadMetrics(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}

		metrics, err := repo.ListMetrics(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return cwMetricsLoadedMsg{metrics: metrics}
	}
}

func (cw cloudWatchMetricsModel) loadSeries(m Model, metrics []awsservice.CloudWatchMetric) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}

		endTime := time.Now()
		startTime := endTime.Add(-cw.selectedTimeRange().duration)
		series, err := repo.GetMetricDataSeries(ctx, metrics, startTime, endTime, cw.selectedPeriod().duration, cw.selectedStat())
		if err != nil {
			return errMsg{err: err}
		}
		return cwMetricDataLoadedMsg{metrics: metrics, series: series}
	}
}

func (cw *cloudWatchMetricsModel) applyMetricFilters(m *Model) {
	base := cw.presetMetrics()
	cw.filteredCWMetrics = applyFilter(base, m.filterValue(filterCWMetrics))
	if len(cw.filteredCWMetrics) == 0 {
		cw.metricIdx = 0
		return
	}
	if cw.metricIdx >= len(cw.filteredCWMetrics) {
		cw.metricIdx = len(cw.filteredCWMetrics) - 1
	}
}

func (cw cloudWatchMetricsModel) presetMetrics() []awsservice.CloudWatchMetric {
	preset := cw.selectedPreset()
	if len(preset.rules) == 0 {
		return cw.metrics
	}

	filtered := make([]awsservice.CloudWatchMetric, 0, len(cw.metrics))
	for _, metric := range cw.metrics {
		if preset.matches(metric) {
			filtered = append(filtered, metric)
		}
	}
	return filtered
}

func (cw cloudWatchMetricsModel) selectedPreset() cwMetricPreset {
	if cw.presetIdx < 0 || cw.presetIdx >= len(cwMetricPresets) {
		return cwMetricPresets[0]
	}
	return cwMetricPresets[cw.presetIdx]
}

func (cw cloudWatchMetricsModel) selectedTimeRange() cwMetricRangeOption {
	if cw.timeRangeIdx < 0 || cw.timeRangeIdx >= len(cwMetricRangeOptions) {
		return cwMetricRangeOptions[0]
	}
	return cwMetricRangeOptions[cw.timeRangeIdx]
}

func (cw cloudWatchMetricsModel) selectedPeriod() cwMetricPeriodOption {
	if cw.periodIdx < 0 || cw.periodIdx >= len(cwMetricPeriodOptions) {
		return cwMetricPeriodOptions[0]
	}
	return cwMetricPeriodOptions[cw.periodIdx]
}

func (cw cloudWatchMetricsModel) selectedStat() string {
	if cw.statIdx < 0 || cw.statIdx >= len(cwMetricStatOptions) {
		return cwMetricStatOptions[0]
	}
	return cwMetricStatOptions[cw.statIdx]
}

func (cw cloudWatchMetricsModel) controlsSummary() string {
	return fmt.Sprintf("Preset: %s  Range: %s  Period: %s  Stat: %s", cw.selectedPreset().label, cw.selectedTimeRange().label, cw.selectedPeriod().label, cw.selectedStat())
}

func (p cwMetricPreset) matches(metric awsservice.CloudWatchMetric) bool {
	for _, rule := range p.rules {
		if !strings.EqualFold(rule.namespace, metric.Namespace) {
			continue
		}
		for _, metricName := range rule.metricNames {
			if strings.EqualFold(metricName, metric.MetricName) {
				return true
			}
		}
	}
	return false
}
