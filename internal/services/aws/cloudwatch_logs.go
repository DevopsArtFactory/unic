package aws

import (
	"context"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	uniclog "unic/internal/log"
)

// ListLogGroups returns all CloudWatch Logs log groups in the current account/region.
func (r *AwsRepository) ListLogGroups(ctx context.Context) ([]LogGroup, error) {
	uniclog.Debug("aws", "ListLogGroups called")
	var groups []LogGroup
	var nextToken *string

	for {
		output, err := r.CloudWatchLogsClient.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe log groups: %w", err)
		}

		for _, lg := range output.LogGroups {
			group := LogGroup{
				Name:        awssdk.ToString(lg.LogGroupName),
				ARN:         awssdk.ToString(lg.Arn),
				StoredBytes: awssdk.ToInt64(lg.StoredBytes),
			}
			if lg.RetentionInDays != nil {
				group.RetentionDays = *lg.RetentionInDays
			}
			if lg.CreationTime != nil {
				group.CreationTime = time.UnixMilli(*lg.CreationTime)
			}
			groups = append(groups, group)
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return groups, nil
}

// ListLogStreams returns log streams for a given log group, sorted by last event time descending.
func (r *AwsRepository) ListLogStreams(ctx context.Context, logGroupName string) ([]LogStream, error) {
	uniclog.Debug("aws", "ListLogStreams called", "log_group", logGroupName)
	var streams []LogStream
	var nextToken *string

	for {
		output, err := r.CloudWatchLogsClient.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: awssdk.String(logGroupName),
			OrderBy:      "LastEventTime",
			Descending:   awssdk.Bool(true),
			NextToken:    nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe log streams for %s: %w", logGroupName, err)
		}

		for _, ls := range output.LogStreams {
			stream := LogStream{
				Name: awssdk.ToString(ls.LogStreamName),
			}
			if ls.LastEventTimestamp != nil {
				stream.LastEventTime = time.UnixMilli(*ls.LastEventTimestamp)
			}
			if ls.CreationTime != nil {
				stream.CreationTime = time.UnixMilli(*ls.CreationTime)
			}
			streams = append(streams, stream)
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return streams, nil
}

// FilterLogEvents returns log events matching the given criteria.
// Returns events, a next token for pagination, and any error.
func (r *AwsRepository) FilterLogEvents(ctx context.Context, logGroupName string, startTime, endTime int64, filterPattern string, nextToken *string) ([]LogEvent, *string, error) {
	uniclog.Debug("aws", "FilterLogEvents called", "log_group", logGroupName, "start", startTime, "end", endTime)

	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: awssdk.String(logGroupName),
		StartTime:    awssdk.Int64(startTime),
		EndTime:      awssdk.Int64(endTime),
		NextToken:    nextToken,
	}
	if filterPattern != "" {
		input.FilterPattern = awssdk.String(filterPattern)
	}

	output, err := r.CloudWatchLogsClient.FilterLogEvents(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to filter log events for %s: %w", logGroupName, err)
	}

	var events []LogEvent
	for _, e := range output.Events {
		event := LogEvent{
			Message: awssdk.ToString(e.Message),
		}
		if e.Timestamp != nil {
			event.Timestamp = time.UnixMilli(*e.Timestamp)
		}
		event.Level = extractLogLevel(event.Message)
		events = append(events, event)
	}

	return events, output.NextToken, nil
}
