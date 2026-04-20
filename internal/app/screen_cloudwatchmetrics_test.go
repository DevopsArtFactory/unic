package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	awsservice "unic/internal/services/aws"
)

func TestRenderMetricSparklineUsesBlockChars(t *testing.T) {
	points := []awsservice.CloudWatchMetricDatapoint{
		{Timestamp: time.Unix(0, 0), Value: 1},
		{Timestamp: time.Unix(1, 0), Value: 2},
		{Timestamp: time.Unix(2, 0), Value: 3},
		{Timestamp: time.Unix(3, 0), Value: 4},
	}

	sparkline := renderMetricSparkline(points, 4)
	if sparkline == "" {
		t.Fatal("expected non-empty sparkline")
	}
	if !strings.ContainsRune("▁▂▃▄▅▆▇█", []rune(sparkline)[0]) {
		t.Fatalf("expected sparkline to use block characters, got %q", sparkline)
	}
}

func TestRenderMetricSparklineConstantSeries(t *testing.T) {
	points := []awsservice.CloudWatchMetricDatapoint{
		{Timestamp: time.Unix(0, 0), Value: 5},
		{Timestamp: time.Unix(1, 0), Value: 5},
		{Timestamp: time.Unix(2, 0), Value: 5},
		{Timestamp: time.Unix(3, 0), Value: 5},
	}

	sparkline := renderMetricSparkline(points, 4)
	if sparkline != "████" {
		t.Fatalf("expected constant positive series to render as a flat filled sparkline, got %q", sparkline)
	}
}

func TestRenderMetricComparisonOverlayUsesLegendSymbols(t *testing.T) {
	seriesSet := []*awsservice.CloudWatchMetricSeriesData{
		{
			Datapoints: []awsservice.CloudWatchMetricDatapoint{
				{Timestamp: time.Unix(0, 0), Value: 1},
				{Timestamp: time.Unix(1, 0), Value: 2},
				{Timestamp: time.Unix(2, 0), Value: 3},
			},
		},
		{
			Datapoints: []awsservice.CloudWatchMetricDatapoint{
				{Timestamp: time.Unix(0, 0), Value: 3},
				{Timestamp: time.Unix(1, 0), Value: 2},
				{Timestamp: time.Unix(2, 0), Value: 1},
			},
		},
	}

	lines, minValue, maxValue := renderMetricComparisonOverlay(seriesSet, 3, 3)
	if len(lines) != 3 {
		t.Fatalf("expected 3 overlay lines, got %d", len(lines))
	}
	chart := strings.Join(lines, "\n")
	if !strings.Contains(chart, "A") || !strings.Contains(chart, "B") {
		t.Fatalf("expected overlay to contain legend symbols A and B, got %q", chart)
	}
	if minValue != 1 || maxValue != 3 {
		t.Fatalf("expected min/max 1/3, got %v/%v", minValue, maxValue)
	}
}

func TestCloudWatchMetricsHandleMessageLoadsMetrics(t *testing.T) {
	m := New(testConfig(), "", "dev")
	updated, _, handled := m.cwMetrics.HandleMessage(&m, cwMetricsLoadedMsg{
		metrics: []awsservice.CloudWatchMetric{
			{Namespace: "AWS/EC2", MetricName: "CPUUtilization"},
			{Namespace: "AWS/RDS", MetricName: "CPUUtilization"},
		},
	})
	if !handled {
		t.Fatal("expected metrics message to be handled")
	}

	model := updated.(Model)
	if model.screen != screenCWMetricList {
		t.Fatalf("expected metric list screen, got %v", model.screen)
	}
	if len(model.cwMetrics.filteredCWMetrics) != 2 {
		t.Fatalf("expected 2 filtered metrics, got %d", len(model.cwMetrics.filteredCWMetrics))
	}
}

func TestCloudWatchMetricListToggleComparisonRequiresRelatedMetrics(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenCWMetricList
	m.cwMetrics.metrics = []awsservice.CloudWatchMetric{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Dimensions: []awsservice.CloudWatchMetricDimension{{Name: "InstanceId", Value: "i-1"}}},
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Dimensions: []awsservice.CloudWatchMetricDimension{{Name: "InstanceId", Value: "i-2"}}},
		{Namespace: "AWS/EC2", MetricName: "NetworkIn", Dimensions: []awsservice.CloudWatchMetricDimension{{Name: "InstanceId", Value: "i-1"}}},
	}
	m.cwMetrics.filteredCWMetrics = m.cwMetrics.metrics

	updated, _ := m.cwMetrics.updateMetricList(&m, keyMsg(" "))
	model := updated.(Model)
	if len(model.cwMetrics.comparisonMetrics) != 1 {
		t.Fatalf("expected one selected metric, got %d", len(model.cwMetrics.comparisonMetrics))
	}

	model.cwMetrics.metricIdx = 1
	updated, _ = model.cwMetrics.updateMetricList(&model, keyMsg(" "))
	model = updated.(Model)
	if len(model.cwMetrics.comparisonMetrics) != 2 {
		t.Fatalf("expected related metric to be selected, got %d selections", len(model.cwMetrics.comparisonMetrics))
	}

	model.cwMetrics.metricIdx = 2
	updated, _ = model.cwMetrics.updateMetricList(&model, keyMsg(" "))
	model = updated.(Model)
	if len(model.cwMetrics.comparisonMetrics) != 2 {
		t.Fatalf("expected unrelated metric to be rejected, got %d selections", len(model.cwMetrics.comparisonMetrics))
	}
	if !strings.Contains(model.cwMetrics.selectionNotice, "same namespace and metric name") {
		t.Fatalf("expected related-metric notice, got %q", model.cwMetrics.selectionNotice)
	}
}

func TestCloudWatchMetricsApplyFilter(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cwMetrics.metrics = []awsservice.CloudWatchMetric{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization"},
		{Namespace: "AWS/RDS", MetricName: "DatabaseConnections"},
	}
	m.storeFilterValue(filterCWMetrics, "database")

	m.applyFilterTarget(filterCWMetrics)

	if len(m.cwMetrics.filteredCWMetrics) != 1 {
		t.Fatalf("expected 1 filtered metric, got %d", len(m.cwMetrics.filteredCWMetrics))
	}
	if got := m.cwMetrics.filteredCWMetrics[0].MetricName; got != "DatabaseConnections" {
		t.Fatalf("unexpected filtered metric %q", got)
	}
}

func TestCloudWatchMetricsPresetFiltersResourceGroups(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cwMetrics.metrics = []awsservice.CloudWatchMetric{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization"},
		{Namespace: "AWS/EC2", MetricName: "NetworkIn"},
		{Namespace: "AWS/RDS", MetricName: "DatabaseConnections"},
		{Namespace: "AWS/ECS", MetricName: "CPUUtilization"},
		{Namespace: "ECS/ContainerInsights", MetricName: "RunningTaskCount"},
	}

	m.cwMetrics.presetIdx = 1
	m.applyFilterTarget(filterCWMetrics)

	if len(m.cwMetrics.filteredCWMetrics) != 2 {
		t.Fatalf("expected 2 EC2 preset metrics, got %d", len(m.cwMetrics.filteredCWMetrics))
	}
	for _, metric := range m.cwMetrics.filteredCWMetrics {
		if metric.Namespace != "AWS/EC2" {
			t.Fatalf("expected only AWS/EC2 metrics, got %+v", metric)
		}
	}
}

func TestCloudWatchMetricListCyclePreset(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenCWMetricList
	m.cwMetrics.metrics = []awsservice.CloudWatchMetric{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization"},
		{Namespace: "AWS/RDS", MetricName: "DatabaseConnections"},
	}
	m.cwMetrics.filteredCWMetrics = m.cwMetrics.metrics

	updated, _ := m.cwMetrics.updateMetricList(&m, keyMsg("g"))
	model := updated.(Model)

	if model.cwMetrics.presetIdx != 1 {
		t.Fatalf("expected preset index 1, got %d", model.cwMetrics.presetIdx)
	}
	if len(model.cwMetrics.filteredCWMetrics) != 1 {
		t.Fatalf("expected preset-filtered metrics to shrink to 1, got %d", len(model.cwMetrics.filteredCWMetrics))
	}
	if got := model.cwMetrics.filteredCWMetrics[0].Namespace; got != "AWS/EC2" {
		t.Fatalf("expected EC2 metric after preset cycle, got %q", got)
	}
}

func TestCloudWatchMetricDetailViewShowsNoDataMessage(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 90
	m.height = 24
	m.screen = screenCWMetricDetail

	metric := awsservice.CloudWatchMetric{
		Namespace:  "AWS/EC2",
		MetricName: "CPUUtilization",
	}
	m.cwMetrics.selectedMetric = &metric
	m.cwMetrics.selectedSeries = &awsservice.CloudWatchMetricSeriesData{
		Metric:    metric,
		Stat:      "Average",
		StartTime: time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC),
		Period:    time.Minute,
	}

	view := m.cwMetrics.viewMetricDetail(m)
	if !strings.Contains(view, "No datapoints returned for the selected time window.") {
		t.Fatalf("expected no-data message, got %q", view)
	}
	if !strings.Contains(view, "Try t to widen the range, p to change the period, or s to switch the statistic.") {
		t.Fatalf("expected no-data controls hint, got %q", view)
	}
}

func TestCloudWatchMetricDetailViewShowsComparisonOverlay(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 90
	m.height = 24
	m.screen = screenCWMetricDetail

	metricOne := awsservice.CloudWatchMetric{
		Namespace:  "AWS/EC2",
		MetricName: "CPUUtilization",
		Dimensions: []awsservice.CloudWatchMetricDimension{{Name: "InstanceId", Value: "i-1"}},
	}
	metricTwo := awsservice.CloudWatchMetric{
		Namespace:  "AWS/EC2",
		MetricName: "CPUUtilization",
		Dimensions: []awsservice.CloudWatchMetricDimension{{Name: "InstanceId", Value: "i-2"}},
	}
	m.cwMetrics.selectedMetric = &metricOne
	m.cwMetrics.selectedSeriesSet = []*awsservice.CloudWatchMetricSeriesData{
		{
			Metric:    metricOne,
			Label:     "i-1",
			Stat:      "Average",
			StartTime: time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC),
			Period:    time.Minute,
			Datapoints: []awsservice.CloudWatchMetricDatapoint{
				{Timestamp: time.Unix(0, 0), Value: 10},
				{Timestamp: time.Unix(1, 0), Value: 20},
			},
		},
		{
			Metric:    metricTwo,
			Label:     "i-2",
			Stat:      "Average",
			StartTime: time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC),
			Period:    time.Minute,
			Datapoints: []awsservice.CloudWatchMetricDatapoint{
				{Timestamp: time.Unix(0, 0), Value: 20},
				{Timestamp: time.Unix(1, 0), Value: 10},
			},
		},
	}

	view := m.cwMetrics.viewMetricDetail(m)
	for _, expected := range []string{"CloudWatch Metric Comparison", "Legend", "Overlay", "A", "B"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected comparison view to contain %q, got %q", expected, view)
		}
	}
}

func TestCloudWatchMetricDetailControlChangeStartsReload(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenCWMetricDetail
	metric := awsservice.CloudWatchMetric{
		Namespace:  "AWS/EC2",
		MetricName: "CPUUtilization",
	}
	m.cwMetrics.selectedMetric = &metric
	m.cwMetrics.selectedSeries = &awsservice.CloudWatchMetricSeriesData{Metric: metric, Stat: "Average"}

	updated, cmd := m.cwMetrics.updateMetricDetail(&m, keyMsg("s"))
	model := updated.(Model)

	if model.screen != screenLoading {
		t.Fatalf("expected screenLoading after statistic change, got %v", model.screen)
	}
	if model.cwMetrics.statIdx != 1 {
		t.Fatalf("expected stat index to advance to 1, got %d", model.cwMetrics.statIdx)
	}
	if model.loadingTitle != "Refreshing CloudWatch metric series..." {
		t.Fatalf("unexpected loading title %q", model.loadingTitle)
	}
	if cmd == nil {
		t.Fatal("expected reload command after statistic change")
	}
}

func TestCloudWatchMetricListViewShowsGlobalEmptyState(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 90
	m.height = 24
	m.screen = screenCWMetricList

	view := m.cwMetrics.viewMetricList(m)
	if !strings.Contains(view, "No CloudWatch metrics were found in this account/region.") {
		t.Fatalf("expected global empty-state copy, got %q", view)
	}
}

func keyMsg(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value), Alt: false}
}
