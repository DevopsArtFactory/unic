package app

import (
	"strings"
	"testing"
	"time"

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
}
