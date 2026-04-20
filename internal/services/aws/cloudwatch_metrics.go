package aws

import (
	"context"
	"fmt"
	"sort"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	uniclog "unic/internal/log"
)

// ListMetrics returns all CloudWatch metrics visible in the current account/region.
func (r *AwsRepository) ListMetrics(ctx context.Context) ([]CloudWatchMetric, error) {
	uniclog.Debug("aws", "ListMetrics called")

	metricsByKey := make(map[string]CloudWatchMetric)
	var nextToken *string

	for {
		output, err := r.CloudWatchClient.ListMetrics(ctx, &cloudwatch.ListMetricsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list CloudWatch metrics: %w", err)
		}

		for _, metric := range output.Metrics {
			item := CloudWatchMetric{
				Namespace:  awssdk.ToString(metric.Namespace),
				MetricName: awssdk.ToString(metric.MetricName),
				Dimensions: make([]CloudWatchMetricDimension, 0, len(metric.Dimensions)),
			}
			for _, dim := range metric.Dimensions {
				item.Dimensions = append(item.Dimensions, CloudWatchMetricDimension{
					Name:  awssdk.ToString(dim.Name),
					Value: awssdk.ToString(dim.Value),
				})
			}
			item.Normalize()
			metricsByKey[item.IdentityKey()] = item
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	metrics := make([]CloudWatchMetric, 0, len(metricsByKey))
	for _, metric := range metricsByKey {
		metrics = append(metrics, metric)
	}

	sort.Slice(metrics, func(i, j int) bool {
		left := metrics[i].IdentityKey()
		right := metrics[j].IdentityKey()
		return left < right
	})
	return metrics, nil
}

// GetMetricData returns a single metric series for the requested time window.
func (r *AwsRepository) GetMetricData(ctx context.Context, metric CloudWatchMetric, startTime, endTime time.Time, period time.Duration, stat string) (*CloudWatchMetricSeriesData, error) {
	uniclog.Debug("aws", "GetMetricData called", "namespace", metric.Namespace, "metric", metric.MetricName, "stat", stat)

	series, err := r.GetMetricDataSeries(ctx, []CloudWatchMetric{metric}, startTime, endTime, period, stat)
	if err != nil {
		return nil, err
	}
	if len(series) == 0 {
		return &CloudWatchMetricSeriesData{
			Metric:    metric,
			Stat:      stat,
			StartTime: startTime,
			EndTime:   endTime,
			Period:    period,
		}, nil
	}
	return series[0], nil
}

// GetMetricDataSeries returns data for a small set of metric series over the same time window.
func (r *AwsRepository) GetMetricDataSeries(ctx context.Context, metrics []CloudWatchMetric, startTime, endTime time.Time, period time.Duration, stat string) ([]*CloudWatchMetricSeriesData, error) {
	if len(metrics) == 0 {
		return nil, nil
	}

	uniclog.Debug("aws", "GetMetricDataSeries called", "series_count", len(metrics), "stat", stat)

	periodSeconds := int32(period / time.Second)
	queries := make([]cwtypes.MetricDataQuery, 0, len(metrics))
	series := make([]*CloudWatchMetricSeriesData, 0, len(metrics))
	indexByID := make(map[string]int, len(metrics))

	for i, metric := range metrics {
		queryID := fmt.Sprintf("m%d", i+1)
		indexByID[queryID] = i
		queries = append(queries, cloudWatchMetricDataQuery(queryID, metric, periodSeconds, stat))
		series = append(series, &CloudWatchMetricSeriesData{
			Metric:    metric,
			Stat:      stat,
			StartTime: startTime,
			EndTime:   endTime,
			Period:    period,
		})
	}

	output, err := r.CloudWatchClient.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
		StartTime:         awssdk.Time(startTime),
		EndTime:           awssdk.Time(endTime),
		MetricDataQueries: queries,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get CloudWatch metric data: %w", err)
	}

	for _, result := range output.MetricDataResults {
		resultID := awssdk.ToString(result.Id)
		if resultID == "" && len(series) == 1 {
			fillMetricSeriesData(series[0], result)
			continue
		}
		index, ok := indexByID[resultID]
		if !ok {
			continue
		}
		fillMetricSeriesData(series[index], result)
	}

	return series, nil
}

func cloudWatchMetricDataQuery(id string, metric CloudWatchMetric, periodSeconds int32, stat string) cwtypes.MetricDataQuery {
	dimensions := make([]cwtypes.Dimension, 0, len(metric.Dimensions))
	for _, dim := range metric.Dimensions {
		dimensions = append(dimensions, cwtypes.Dimension{
			Name:  awssdk.String(dim.Name),
			Value: awssdk.String(dim.Value),
		})
	}

	return cwtypes.MetricDataQuery{
		Id: awssdk.String(id),
		MetricStat: &cwtypes.MetricStat{
			Metric: &cwtypes.Metric{
				Namespace:  awssdk.String(metric.Namespace),
				MetricName: awssdk.String(metric.MetricName),
				Dimensions: dimensions,
			},
			Period: awssdk.Int32(periodSeconds),
			Stat:   awssdk.String(stat),
		},
		Label:      awssdk.String(metric.DisplayTitle()),
		ReturnData: awssdk.Bool(true),
	}
}

func fillMetricSeriesData(series *CloudWatchMetricSeriesData, result cwtypes.MetricDataResult) {
	series.Label = awssdk.ToString(result.Label)
	series.StatusCode = string(result.StatusCode)
	series.Messages = nil
	for _, message := range result.Messages {
		if code := awssdk.ToString(message.Code); code != "" {
			series.Messages = append(series.Messages, fmt.Sprintf("%s: %s", code, awssdk.ToString(message.Value)))
			continue
		}
		series.Messages = append(series.Messages, awssdk.ToString(message.Value))
	}

	pairs := len(result.Timestamps)
	if len(result.Values) < pairs {
		pairs = len(result.Values)
	}
	series.Datapoints = make([]CloudWatchMetricDatapoint, 0, pairs)
	for i := 0; i < pairs; i++ {
		series.Datapoints = append(series.Datapoints, CloudWatchMetricDatapoint{
			Timestamp: result.Timestamps[i],
			Value:     result.Values[i],
		})
	}

	sort.Slice(series.Datapoints, func(i, j int) bool {
		return series.Datapoints[i].Timestamp.Before(series.Datapoints[j].Timestamp)
	})
}
