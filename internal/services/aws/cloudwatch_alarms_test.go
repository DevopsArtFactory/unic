package aws

import (
	"context"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

func TestListAlarmsSortsFiringFirstAndMapsFields(t *testing.T) {
	updated := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	mock := &mockCloudWatchClient{
		describeAlarmsFunc: func(_ context.Context, _ *cloudwatch.DescribeAlarmsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmsOutput, error) {
			return &cloudwatch.DescribeAlarmsOutput{
				CompositeAlarms: []cwtypes.CompositeAlarm{
					{
						AlarmName:   awssdk.String("service-health"),
						StateValue:  cwtypes.StateValueAlarm,
						StateReason: awssdk.String("Child alarm firing"),
					},
				},
				MetricAlarms: []cwtypes.MetricAlarm{
					{
						AlarmName:  awssdk.String("healthy-alarm"),
						StateValue: cwtypes.StateValueOk,
						MetricName: awssdk.String("CPUUtilization"),
						Namespace:  awssdk.String("AWS/EC2"),
					},
					{
						AlarmName:             awssdk.String("db-cpu-high"),
						StateValue:            cwtypes.StateValueAlarm,
						StateReason:           awssdk.String("Threshold crossed"),
						StateUpdatedTimestamp: &updated,
						MetricName:            awssdk.String("CPUUtilization"),
						Namespace:             awssdk.String("AWS/RDS"),
						Threshold:             awssdk.Float64(80),
						ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
						Dimensions: []cwtypes.Dimension{
							{Name: awssdk.String("DBInstanceIdentifier"), Value: awssdk.String("prod-db")},
						},
					},
					{
						AlarmName:  awssdk.String("no-data-alarm"),
						StateValue: cwtypes.StateValueInsufficientData,
						MetricName: awssdk.String("Invocations"),
						Namespace:  awssdk.String("AWS/Lambda"),
					},
				},
			}, nil
		},
	}
	repo := &AwsRepository{CloudWatchClient: mock}

	alarms, err := repo.ListAlarms(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alarms) != 4 {
		t.Fatalf("expected 4 alarms including the composite, got %d", len(alarms))
	}
	if alarms[0].State != "ALARM" || alarms[1].State != "ALARM" ||
		alarms[2].State != "INSUFFICIENT_DATA" || alarms[3].State != "OK" {
		t.Fatalf("expected firing-first ordering across metric and composite alarms, got %+v", alarms)
	}
	firing := alarms[0]
	if firing.Name != "db-cpu-high" || firing.Dimension("DBInstanceIdentifier") != "prod-db" ||
		firing.Threshold != 80 || !firing.StateUpdated.Equal(updated) {
		t.Fatalf("expected mapped alarm fields, got %+v", firing)
	}
	composite := alarms[1]
	if composite.Name != "service-health" || !composite.Composite || composite.StateReason != "Child alarm firing" {
		t.Fatalf("expected mapped composite alarm, got %+v", composite)
	}
	if !strings.Contains(composite.DisplayTitle(), "(composite)") {
		t.Fatalf("expected composite marker in display title, got %q", composite.DisplayTitle())
	}
}

func TestListAlarmHistoryMapsItems(t *testing.T) {
	ts := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	mock := &mockCloudWatchClient{
		describeAlarmHistoryFunc: func(_ context.Context, params *cloudwatch.DescribeAlarmHistoryInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
			if awssdk.ToString(params.AlarmName) != "db-cpu-high" {
				t.Fatalf("expected alarm name filter, got %+v", params)
			}
			if params.ScanBy != cwtypes.ScanByTimestampDescending {
				t.Fatalf("expected newest-first scan, got %v", params.ScanBy)
			}
			if params.HistoryItemType != cwtypes.HistoryItemTypeStateUpdate {
				t.Fatalf("expected state-update-only history, got %v", params.HistoryItemType)
			}
			return &cloudwatch.DescribeAlarmHistoryOutput{
				AlarmHistoryItems: []cwtypes.AlarmHistoryItem{
					{
						Timestamp:       &ts,
						HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
						HistorySummary:  awssdk.String("Alarm updated from OK to ALARM"),
					},
				},
			}, nil
		},
	}
	repo := &AwsRepository{CloudWatchClient: mock}

	items, err := repo.ListAlarmHistory(context.Background(), "db-cpu-high")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].Summary != "Alarm updated from OK to ALARM" || !items[0].Timestamp.Equal(ts) {
		t.Fatalf("expected mapped history item, got %+v", items)
	}
}
