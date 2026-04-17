package app

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

const (
	cwMetricDefaultLookback = time.Hour
	cwMetricDefaultPeriod   = time.Minute
	cwMetricDefaultStat     = "Average"
)

var cwMetricSparklineBlocks = []rune("▁▂▃▄▅▆▇█")

type cloudWatchMetricsModel struct {
	metrics           []awsservice.CloudWatchMetric
	filteredCWMetrics []awsservice.CloudWatchMetric
	metricIdx         int
	selectedMetric    *awsservice.CloudWatchMetric
	selectedSeries    *awsservice.CloudWatchMetricSeriesData
	detailScroll      int
}

func newCloudWatchMetricsModel() cloudWatchMetricsModel {
	return cloudWatchMetricsModel{}
}

func (cw *cloudWatchMetricsModel) Start(m *Model) (tea.Model, tea.Cmd) {
	cw.selectedMetric = nil
	cw.selectedSeries = nil
	cw.detailScroll = 0
	return m.startLoadingWithMessage(
		"Loading CloudWatch metrics...",
		[]string{
			"Fetching metric definitions from the current account and region.",
			"This first-pass viewer opens one metric series at a time with a fixed 1h / Average view.",
		},
		cw.loadMetrics(*m),
	)
}

func (cw *cloudWatchMetricsModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case cwMetricsLoadedMsg:
		cw.metrics = msg.metrics
		cw.filteredCWMetrics = applyFilter(cw.metrics, m.filterValue(filterCWMetrics))
		cw.metricIdx = 0
		m.screen = screenCWMetricList
		return *m, nil, true
	case cwMetricDataLoadedMsg:
		metric := msg.metric
		cw.selectedMetric = &metric
		cw.selectedSeries = msg.series
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
	cw.filteredCWMetrics = applyFilter(cw.metrics, m.filterValue(target))
	cw.metricIdx = 0
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
		if cw.metricIdx > 0 {
			cw.metricIdx--
		}
	case "down", "j":
		if cw.metricIdx < len(cw.filteredCWMetrics)-1 {
			cw.metricIdx++
		}
	case "/":
		return *m, m.activateFilter(filterCWMetrics)
	case "r":
		return m.startLoading(cw.loadMetrics(*m))
	case "enter":
		if len(cw.filteredCWMetrics) == 0 || cw.metricIdx >= len(cw.filteredCWMetrics) {
			return *m, nil
		}
		selected := cw.filteredCWMetrics[cw.metricIdx]
		return m.startLoadingWithMessage(
			"Loading CloudWatch metric series...",
			[]string{
				fmt.Sprintf("Metric: %s / %s", selected.Namespace, selected.MetricName),
				fmt.Sprintf("Stat: %s  Range: last %s  Period: %s", cwMetricDefaultStat, cwMetricDefaultLookback, cwMetricDefaultPeriod),
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
		if cw.selectedMetric == nil {
			return *m, nil
		}
		return m.startLoadingWithMessage(
			"Refreshing CloudWatch metric series...",
			[]string{fmt.Sprintf("Metric: %s / %s", cw.selectedMetric.Namespace, cw.selectedMetric.MetricName)},
			cw.loadSeries(*m, *cw.selectedMetric),
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
	b.WriteString(dimStyle.Render("Single-series viewer v1. Use / to filter, then Enter to open one metric series."))
	b.WriteString("\n\n")

	b.WriteString(m.renderFilterValue(filterCWMetrics))
	b.WriteString("\n\n")

	if len(cw.filteredCWMetrics) == 0 {
		panel.WriteString(dimStyle.Render("  No matching metrics"))
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
			style := normalStyle
			if i == cw.metricIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterCWMetrics, metric.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d metric series", len(cw.filteredCWMetrics), len(cw.metrics))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: chart • esc: back • H: home"))
	return b.String()
}

func (cw cloudWatchMetricsModel) viewMetricDetail(m Model) string {
	var b strings.Builder
	var panel strings.Builder

	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("CloudWatch Metric Detail"))
	b.WriteString("\n\n")

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
	b.WriteString(m.renderHelpBar("↑/↓: scroll • pgup/pgdn: page • r: refresh • esc: back • H: home"))
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
		renderDetailLine("Stat", normalStyle.Render(series.Stat)),
		renderDetailLine("Range", normalStyle.Render(fmt.Sprintf("%s to %s", series.StartTime.Local().Format("2006-01-02 15:04"), series.EndTime.Local().Format("2006-01-02 15:04")))),
		renderDetailLine("Period", normalStyle.Render(series.Period.String())),
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
	for _, value := range values {
		position := (value - minValue) / (maxValue - minValue)
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

func (cw cloudWatchMetricsModel) loadMetrics(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}

		metrics, err := repo.ListMetrics(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(metrics) == 0 {
			return errMsg{err: fmt.Errorf("no CloudWatch metrics found")}
		}
		return cwMetricsLoadedMsg{metrics: metrics}
	}
}

func (cw cloudWatchMetricsModel) loadSeries(m Model, metric awsservice.CloudWatchMetric) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}

		endTime := time.Now()
		startTime := endTime.Add(-cwMetricDefaultLookback)
		series, err := repo.GetMetricData(ctx, metric, startTime, endTime, cwMetricDefaultPeriod, cwMetricDefaultStat)
		if err != nil {
			return errMsg{err: err}
		}
		return cwMetricDataLoadedMsg{metric: metric, series: series}
	}
}
