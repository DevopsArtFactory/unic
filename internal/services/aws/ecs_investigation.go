package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// ECSInvestigation is a bounded collection, not an atomic snapshot or root-cause diagnosis.
type ECSInvestigation struct {
	ObservedAt       time.Time
	Rollout          *ECSServiceDetail
	Health           []ECSServiceTargetHealth
	StoppedTasks     []ECSStoppedTask
	Logs             []ECSInvestigationLog
	Changes          []CloudTrailEvent
	ChangesCollected bool
	Warnings         []string
}

type ECSStoppedTask struct {
	ARN, StopCode, Reason string
	StoppedAt             time.Time
	Containers            []string
}

type ECSInvestigationLog struct {
	Group   string
	Events  []LogEvent
	Warning string
}

// InvestigateECSService retains successful sections when another lookup fails.
// The caller supplies a deadline; each section also checks cancellation before starting.
func (r *AwsRepository) InvestigateECSService(ctx context.Context, cluster, service string) ECSInvestigation {
	result := ECSInvestigation{ObservedAt: time.Now().UTC()}
	if err := ctx.Err(); err != nil {
		result.Warnings = append(result.Warnings, "Investigation incomplete: "+err.Error())
		return result
	}
	var err error
	result.Rollout, err = r.DescribeServiceDetail(ctx, cluster, service)
	if err != nil {
		result.Warnings = append(result.Warnings, "Rollout unavailable: "+err.Error())
	}
	if ctx.Err() == nil {
		result.Health, err = r.ListECSServiceTargetHealth(ctx, cluster, service)
		if err != nil {
			result.Warnings = append(result.Warnings, "Target health unavailable: "+err.Error())
		}
	}
	if ctx.Err() == nil {
		r.collectECSStoppedTasks(ctx, cluster, service, &result)
	}
	if ctx.Err() == nil {
		r.collectECSLogs(ctx, &result)
	}
	if ctx.Err() == nil {
		var complete bool
		result.Changes, complete, err = r.LookupEventsWithStatus(ctx, CloudTrailLookup{Since: 6 * time.Hour, ResourceName: service, MutationsOnly: true})
		result.ChangesCollected = err == nil
		if err != nil {
			result.Warnings = append(result.Warnings, "CloudTrail unavailable: "+err.Error())
		} else if !complete {
			result.Warnings = append(result.Warnings, "CloudTrail truncated at 100 events; narrow the query in CloudTrail Event Lookup.")
		}
	}
	if err := ctx.Err(); err != nil {
		result.Warnings = append(result.Warnings, "Investigation incomplete; later sections may not have run: "+err.Error())
	}
	return result
}

func (r *AwsRepository) collectECSStoppedTasks(ctx context.Context, cluster, service string, result *ECSInvestigation) {
	name := shortECSResourceRef(service)
	out, err := r.ECSClient.ListTasks(ctx, &ecs.ListTasksInput{Cluster: awssdk.String(cluster), ServiceName: &name, DesiredStatus: ecstypes.DesiredStatusStopped, MaxResults: awssdk.Int32(20)})
	if err != nil {
		result.Warnings = append(result.Warnings, "Stopped tasks unavailable: "+err.Error())
		return
	}
	if out.NextToken != nil {
		result.Warnings = append(result.Warnings, "Stopped tasks limited to one page (up to 20); not a complete failure history.")
	}
	if len(out.TaskArns) == 0 {
		return
	}
	described, err := r.ECSClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{Cluster: awssdk.String(cluster), Tasks: out.TaskArns})
	if err != nil {
		result.Warnings = append(result.Warnings, "Stopped-task details unavailable: "+err.Error())
		return
	}
	for _, failure := range described.Failures {
		result.Warnings = append(result.Warnings, "Stopped-task lookup failed: "+awssdk.ToString(failure.Reason))
	}
	for _, task := range described.Tasks {
		// DescribeTasks may return failures or stale tasks; keep only this service's evidence.
		if awssdk.ToString(task.Group) != "service:"+name {
			continue
		}
		item := ECSStoppedTask{ARN: awssdk.ToString(task.TaskArn), StopCode: string(task.StopCode), Reason: awssdk.ToString(task.StoppedReason)}
		if task.StoppedAt != nil {
			item.StoppedAt = *task.StoppedAt
		}
		for _, container := range task.Containers {
			if container.Reason != nil || container.ExitCode != nil {
				text := awssdk.ToString(container.Name) + ": " + awssdk.ToString(container.Reason)
				if container.ExitCode != nil {
					text += fmt.Sprintf(" (exit %d)", *container.ExitCode)
				}
				item.Containers = append(item.Containers, text)
			}
		}
		result.StoppedTasks = append(result.StoppedTasks, item)
	}
}

func (r *AwsRepository) collectECSLogs(ctx context.Context, result *ECSInvestigation) {
	if result.Rollout == nil || result.Rollout.TaskDefinitionARN == "" {
		result.Warnings = append(result.Warnings, "Logs unavailable: no resolved task definition.")
		return
	}
	out, err := r.ECSClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{TaskDefinition: awssdk.String(result.Rollout.TaskDefinitionARN)})
	if err != nil {
		result.Warnings = append(result.Warnings, "Log associations unavailable: "+err.Error())
		return
	}
	if out.TaskDefinition == nil {
		result.Warnings = append(result.Warnings, "Logs unavailable: task definition missing.")
		return
	}
	seen := map[string]bool{}
	for _, container := range out.TaskDefinition.ContainerDefinitions {
		cfg := container.LogConfiguration
		if cfg == nil || cfg.LogDriver != ecstypes.LogDriverAwslogs {
			result.Warnings = append(result.Warnings, "A container has no supported awslogs association.")
			continue
		}
		group := cfg.Options["awslogs-group"]
		if group == "" {
			result.Warnings = append(result.Warnings, "Logs unavailable: awslogs-group missing.")
			continue
		}
		if region := cfg.Options["awslogs-region"]; region != "" && region != r.Region {
			result.Warnings = append(result.Warnings, "Logs skipped: awslogs-region differs from the selected resource region.")
			continue
		}
		if seen[group] {
			continue
		}
		if len(result.Logs) >= 5 {
			result.Warnings = append(result.Warnings, "Log collection limited to five associated groups.")
			break
		}
		seen[group] = true
		if ctx.Err() != nil {
			return
		}
		item := ECSInvestigationLog{Group: group}
		// Group-wide evidence may include other services; never infer task ownership from its name.
		page, err := r.CloudWatchLogsClient.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{LogGroupName: &group, StartTime: awssdk.Int64(result.ObservedAt.Add(-6 * time.Hour).UnixMilli()), EndTime: awssdk.Int64(result.ObservedAt.UnixMilli()), Limit: awssdk.Int32(50)})
		if err != nil {
			item.Warning = err.Error()
		} else {
			for _, event := range page.Events {
				item.Events = append(item.Events, LogEvent{Timestamp: time.UnixMilli(awssdk.ToInt64(event.Timestamp)), Message: awssdk.ToString(event.Message)})
			}
			if page.NextToken != nil && strings.TrimSpace(*page.NextToken) != "" {
				item.Warning = "One page only (up to 50 events); more events may exist, including after an empty page."
			}
		}
		result.Logs = append(result.Logs, item)
	}
}
