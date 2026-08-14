package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	uniclog "unic/internal/log"
)

// CloudWatchAlarm is a metric alarm with the state fields operators triage by.
type CloudWatchAlarm struct {
	Name               string
	State              string
	StateReason        string
	StateUpdated       time.Time
	MetricName         string
	Namespace          string
	Dimensions         []CloudWatchAlarmDimension
	Threshold          float64
	ComparisonOperator string
	ActionsEnabled     bool
	// Composite marks alarms whose state derives from a rule over other
	// alarms; they carry no metric, dimensions, or threshold.
	Composite bool
}

type CloudWatchAlarmDimension struct {
	Name  string
	Value string
}

// DisplayTitle returns a formatted string for list display.
func (a CloudWatchAlarm) DisplayTitle() string {
	metric := a.MetricName
	if a.Namespace != "" {
		metric = a.Namespace + "/" + a.MetricName
	}
	if a.Composite {
		metric = "(composite)"
	}
	return fmt.Sprintf("%-17s %-40s %s", a.State, a.Name, metric)
}

// FilterText returns a lowercase string for keyword matching.
func (a CloudWatchAlarm) FilterText() string {
	parts := []string{a.Name, a.State, a.Namespace, a.MetricName, a.StateReason}
	if a.Composite {
		parts = append(parts, "composite")
	}
	for _, dim := range a.Dimensions {
		parts = append(parts, dim.Name, dim.Value)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// Dimension returns the value of the named dimension, or "".
func (a CloudWatchAlarm) Dimension(name string) string {
	for _, dim := range a.Dimensions {
		if dim.Name == name {
			return dim.Value
		}
	}
	return ""
}

// CloudWatchAlarmHistoryItem is one recorded alarm transition or update.
type CloudWatchAlarmHistoryItem struct {
	Timestamp time.Time
	Type      string
	Summary   string
}

// alarmStatePriority orders triage-first: firing alarms above unknowns above healthy.
func alarmStatePriority(state string) int {
	switch state {
	case "ALARM":
		return 0
	case "INSUFFICIENT_DATA":
		return 1
	default:
		return 2
	}
}

// ListAlarms returns all metric alarms in the account/region, firing first.
func (r *AwsRepository) ListAlarms(ctx context.Context) ([]CloudWatchAlarm, error) {
	uniclog.Debug("aws", "ListAlarms called")

	var alarms []CloudWatchAlarm
	paginator := cloudwatch.NewDescribeAlarmsPaginator(r.CloudWatchClient, &cloudwatch.DescribeAlarmsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe alarms: %w", err)
		}
		for _, alarm := range page.MetricAlarms {
			mapped := CloudWatchAlarm{
				Name:               derefString(alarm.AlarmName),
				State:              string(alarm.StateValue),
				StateReason:        derefString(alarm.StateReason),
				MetricName:         derefString(alarm.MetricName),
				Namespace:          derefString(alarm.Namespace),
				ComparisonOperator: string(alarm.ComparisonOperator),
				ActionsEnabled:     alarm.ActionsEnabled != nil && *alarm.ActionsEnabled,
			}
			if alarm.Threshold != nil {
				mapped.Threshold = *alarm.Threshold
			}
			if alarm.StateUpdatedTimestamp != nil {
				mapped.StateUpdated = *alarm.StateUpdatedTimestamp
			}
			for _, dim := range alarm.Dimensions {
				mapped.Dimensions = append(mapped.Dimensions, CloudWatchAlarmDimension{
					Name:  derefString(dim.Name),
					Value: derefString(dim.Value),
				})
			}
			alarms = append(alarms, mapped)
		}
		for _, alarm := range page.CompositeAlarms {
			mapped := CloudWatchAlarm{
				Name:           derefString(alarm.AlarmName),
				State:          string(alarm.StateValue),
				StateReason:    derefString(alarm.StateReason),
				ActionsEnabled: alarm.ActionsEnabled != nil && *alarm.ActionsEnabled,
				Composite:      true,
			}
			if alarm.StateUpdatedTimestamp != nil {
				mapped.StateUpdated = *alarm.StateUpdatedTimestamp
			}
			alarms = append(alarms, mapped)
		}
	}

	sort.SliceStable(alarms, func(i, j int) bool {
		left, right := alarmStatePriority(alarms[i].State), alarmStatePriority(alarms[j].State)
		if left != right {
			return left < right
		}
		return normalizedSortKey(alarms[i].Name) < normalizedSortKey(alarms[j].Name)
	})
	return alarms, nil
}

// ListAlarmHistory returns the most recent history items for an alarm, newest first.
func (r *AwsRepository) ListAlarmHistory(ctx context.Context, alarmName string) ([]CloudWatchAlarmHistoryItem, error) {
	uniclog.Debug("aws", "ListAlarmHistory called", "alarm", alarmName)

	maxRecords := int32(20)
	out, err := r.CloudWatchClient.DescribeAlarmHistory(ctx, &cloudwatch.DescribeAlarmHistoryInput{
		AlarmName:       &alarmName,
		MaxRecords:      &maxRecords,
		ScanBy:          cwtypes.ScanByTimestampDescending,
		HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe alarm history for %s: %w", alarmName, err)
	}

	items := make([]CloudWatchAlarmHistoryItem, 0, len(out.AlarmHistoryItems))
	for _, item := range out.AlarmHistoryItems {
		mapped := CloudWatchAlarmHistoryItem{
			Type:    string(item.HistoryItemType),
			Summary: derefString(item.HistorySummary),
		}
		if item.Timestamp != nil {
			mapped.Timestamp = *item.Timestamp
		}
		items = append(items, mapped)
	}
	return items, nil
}
