package aws

import (
	"context"
	"fmt"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type mockCloudWatchClient struct {
	listMetricsFunc   func(ctx context.Context, params *cloudwatch.ListMetricsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error)
	getMetricDataFunc func(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

func (m *mockCloudWatchClient) ListMetrics(ctx context.Context, params *cloudwatch.ListMetricsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
	return m.listMetricsFunc(ctx, params, optFns...)
}

func (m *mockCloudWatchClient) GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	return m.getMetricDataFunc(ctx, params, optFns...)
}

func TestListMetrics_Success(t *testing.T) {
	mock := &mockCloudWatchClient{
		listMetricsFunc: func(_ context.Context, params *cloudwatch.ListMetricsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
			if token := awssdk.ToString(params.NextToken); token == "" {
				return &cloudwatch.ListMetricsOutput{
					Metrics: []cwtypes.Metric{
						{
							Namespace:  awssdk.String("AWS/EC2"),
							MetricName: awssdk.String("CPUUtilization"),
							Dimensions: []cwtypes.Dimension{
								{Name: awssdk.String("InstanceId"), Value: awssdk.String("i-456")},
								{Name: awssdk.String("AutoScalingGroupName"), Value: awssdk.String("web")},
							},
						},
					},
					NextToken: awssdk.String("page-2"),
				}, nil
			}
			if token := awssdk.ToString(params.NextToken); token != "page-2" {
				t.Fatalf("expected next token page-2, got %q", token)
			}
			return &cloudwatch.ListMetricsOutput{
				Metrics: []cwtypes.Metric{
					{
						Namespace:  awssdk.String("AWS/EC2"),
						MetricName: awssdk.String("CPUUtilization"),
						Dimensions: []cwtypes.Dimension{
							{Name: awssdk.String("AutoScalingGroupName"), Value: awssdk.String("web")},
							{Name: awssdk.String("InstanceId"), Value: awssdk.String("i-456")},
						},
					},
					{
						Namespace:  awssdk.String("AWS/RDS"),
						MetricName: awssdk.String("CPUUtilization"),
						Dimensions: []cwtypes.Dimension{
							{Name: awssdk.String("DBInstanceIdentifier"), Value: awssdk.String("db-prod")},
						},
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{CloudWatchClient: mock}
	metrics, err := repo.ListMetrics(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 2 {
		t.Fatalf("expected 2 deduplicated metrics, got %d", len(metrics))
	}
	if metrics[0].Namespace != "AWS/EC2" || metrics[0].MetricName != "CPUUtilization" {
		t.Fatalf("unexpected first metric: %+v", metrics[0])
	}
	if got := metrics[0].DimensionsText(); got != "AutoScalingGroupName=web, InstanceId=i-456" {
		t.Fatalf("unexpected dimension text: %q", got)
	}
	if metrics[1].Namespace != "AWS/RDS" {
		t.Fatalf("expected AWS/RDS metric second, got %+v", metrics[1])
	}
}

func TestListMetrics_Error(t *testing.T) {
	mock := &mockCloudWatchClient{
		listMetricsFunc: func(_ context.Context, _ *cloudwatch.ListMetricsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}

	repo := &AwsRepository{CloudWatchClient: mock}
	_, err := repo.ListMetrics(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetMetricData_Success(t *testing.T) {
	startTime := time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC)
	endTime := startTime.Add(time.Hour)
	older := startTime.Add(10 * time.Minute)
	newer := startTime.Add(20 * time.Minute)

	mock := &mockCloudWatchClient{
		getMetricDataFunc: func(_ context.Context, params *cloudwatch.GetMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
			if got := awssdk.ToTime(params.StartTime); !got.Equal(startTime) {
				t.Fatalf("expected start time %v, got %v", startTime, got)
			}
			if got := awssdk.ToTime(params.EndTime); !got.Equal(endTime) {
				t.Fatalf("expected end time %v, got %v", endTime, got)
			}
			if len(params.MetricDataQueries) != 1 {
				t.Fatalf("expected 1 metric query, got %d", len(params.MetricDataQueries))
			}
			query := params.MetricDataQueries[0]
			if got := awssdk.ToString(query.Label); got != "AWS/EC2  CPUUtilization  InstanceId=i-123" {
				t.Fatalf("unexpected label %q", got)
			}
			if got := awssdk.ToString(query.MetricStat.Metric.Namespace); got != "AWS/EC2" {
				t.Fatalf("unexpected namespace %q", got)
			}
			if got := awssdk.ToString(query.MetricStat.Metric.MetricName); got != "CPUUtilization" {
				t.Fatalf("unexpected metric name %q", got)
			}
			if got := awssdk.ToString(query.MetricStat.Stat); got != "Average" {
				t.Fatalf("unexpected stat %q", got)
			}
			if got := awssdk.ToInt32(query.MetricStat.Period); got != 60 {
				t.Fatalf("unexpected period %d", got)
			}
			if got := awssdk.ToString(query.MetricStat.Metric.Dimensions[0].Name); got != "InstanceId" {
				t.Fatalf("unexpected dimension name %q", got)
			}

			return &cloudwatch.GetMetricDataOutput{
				MetricDataResults: []cwtypes.MetricDataResult{
					{
						Label:      awssdk.String("CPUUtilization"),
						StatusCode: cwtypes.StatusCode("Complete"),
						Messages: []cwtypes.MessageData{
							{Code: awssdk.String("Info"), Value: awssdk.String("aggregated")},
						},
						Timestamps: []time.Time{newer, older},
						Values:     []float64{80.0, 40.0},
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{CloudWatchClient: mock}
	series, err := repo.GetMetricData(context.Background(), CloudWatchMetric{
		Namespace:  "AWS/EC2",
		MetricName: "CPUUtilization",
		Dimensions: []CloudWatchMetricDimension{
			{Name: "InstanceId", Value: "i-123"},
		},
	}, startTime, endTime, time.Minute, "Average")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if series.Label != "CPUUtilization" {
		t.Fatalf("unexpected label %q", series.Label)
	}
	if len(series.Datapoints) != 2 {
		t.Fatalf("expected 2 datapoints, got %d", len(series.Datapoints))
	}
	if !series.Datapoints[0].Timestamp.Equal(older) || series.Datapoints[0].Value != 40.0 {
		t.Fatalf("expected older datapoint first, got %+v", series.Datapoints[0])
	}
	if !series.Datapoints[1].Timestamp.Equal(newer) || series.Datapoints[1].Value != 80.0 {
		t.Fatalf("expected newer datapoint second, got %+v", series.Datapoints[1])
	}
	if len(series.Messages) != 1 || series.Messages[0] != "Info: aggregated" {
		t.Fatalf("unexpected messages: %#v", series.Messages)
	}
}

func TestGetMetricData_Error(t *testing.T) {
	mock := &mockCloudWatchClient{
		getMetricDataFunc: func(_ context.Context, _ *cloudwatch.GetMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
			return nil, fmt.Errorf("throttled")
		},
	}

	repo := &AwsRepository{CloudWatchClient: mock}
	_, err := repo.GetMetricData(context.Background(), CloudWatchMetric{
		Namespace:  "AWS/EC2",
		MetricName: "CPUUtilization",
	}, time.Now().Add(-time.Hour), time.Now(), time.Minute, "Average")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
