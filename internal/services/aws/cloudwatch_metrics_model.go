package aws

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// CloudWatchMetricDimension is a name/value pair identifying a metric series.
type CloudWatchMetricDimension struct {
	Name  string
	Value string
}

// CloudWatchMetric identifies one concrete metric series.
type CloudWatchMetric struct {
	Namespace  string
	MetricName string
	Dimensions []CloudWatchMetricDimension
}

// DisplayTitle returns a formatted string for list display.
func (m CloudWatchMetric) DisplayTitle() string {
	if len(m.Dimensions) == 0 {
		return fmt.Sprintf("%s  %s", m.Namespace, m.MetricName)
	}
	return fmt.Sprintf("%s  %s  %s", m.Namespace, m.MetricName, m.DimensionsText())
}

// FilterText returns a lowercase string for fuzzy filtering.
func (m CloudWatchMetric) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s", m.Namespace, m.MetricName, m.DimensionsText()))
}

// DimensionsText returns dimensions joined into a stable display string.
func (m CloudWatchMetric) DimensionsText() string {
	if len(m.Dimensions) == 0 {
		return ""
	}

	parts := make([]string, 0, len(m.Dimensions))
	for _, dim := range m.Dimensions {
		parts = append(parts, fmt.Sprintf("%s=%s", dim.Name, dim.Value))
	}
	return strings.Join(parts, ", ")
}

// IdentityKey returns a stable key for sorting and deduplication.
func (m CloudWatchMetric) IdentityKey() string {
	return strings.ToLower(fmt.Sprintf("%s|%s|%s", m.Namespace, m.MetricName, m.DimensionsText()))
}

// Normalize sorts metric dimensions into a stable order.
func (m *CloudWatchMetric) Normalize() {
	sort.Slice(m.Dimensions, func(i, j int) bool {
		left := normalizedSortKey(m.Dimensions[i].Name)
		right := normalizedSortKey(m.Dimensions[j].Name)
		if left == right {
			return normalizedSortKey(m.Dimensions[i].Value) < normalizedSortKey(m.Dimensions[j].Value)
		}
		return left < right
	})
}

// CloudWatchMetricDatapoint holds one point in a metric time series.
type CloudWatchMetricDatapoint struct {
	Timestamp time.Time
	Value     float64
}

// CloudWatchMetricSeriesData holds fetched datapoints for one metric series.
type CloudWatchMetricSeriesData struct {
	Metric     CloudWatchMetric
	Label      string
	Stat       string
	StatusCode string
	StartTime  time.Time
	EndTime    time.Time
	Period     time.Duration
	Datapoints []CloudWatchMetricDatapoint
	Messages   []string
}

// LatestValue returns the newest value in the series.
func (d CloudWatchMetricSeriesData) LatestValue() (float64, bool) {
	if len(d.Datapoints) == 0 {
		return 0, false
	}
	return d.Datapoints[len(d.Datapoints)-1].Value, true
}

// MinValue returns the minimum datapoint value in the series.
func (d CloudWatchMetricSeriesData) MinValue() (float64, bool) {
	if len(d.Datapoints) == 0 {
		return 0, false
	}
	minValue := d.Datapoints[0].Value
	for _, point := range d.Datapoints[1:] {
		if point.Value < minValue {
			minValue = point.Value
		}
	}
	return minValue, true
}

// MaxValue returns the maximum datapoint value in the series.
func (d CloudWatchMetricSeriesData) MaxValue() (float64, bool) {
	if len(d.Datapoints) == 0 {
		return 0, false
	}
	maxValue := d.Datapoints[0].Value
	for _, point := range d.Datapoints[1:] {
		if point.Value > maxValue {
			maxValue = point.Value
		}
	}
	return maxValue, true
}

// AverageValue returns the arithmetic mean of the series.
func (d CloudWatchMetricSeriesData) AverageValue() (float64, bool) {
	if len(d.Datapoints) == 0 {
		return 0, false
	}
	var total float64
	for _, point := range d.Datapoints {
		total += point.Value
	}
	return total / float64(len(d.Datapoints)), true
}
