package aws

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cttypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func TestECSInvestigationRelationshipsAndPartialEvidence(t *testing.T) {
	const service = "arn:aws:ecs:us-east-1:000000000000:service/cluster/checkout"
	stopped := time.Now().UTC().Add(-time.Minute)
	logCalls := 0
	repo := &AwsRepository{Region: "us-east-1"}
	repo.ECSClient = &mockECSClient{
		describeServicesFunc: func(_ context.Context, in *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
			if awssdk.ToString(in.Cluster) != "cluster" || len(in.Services) != 1 || in.Services[0] != service {
				t.Fatalf("wrong service scope: %+v", in)
			}
			return &ecs.DescribeServicesOutput{Services: []ecstypes.Service{{ServiceArn: awssdk.String(service), ServiceName: awssdk.String("checkout"), TaskDefinition: awssdk.String("taskdef")}}}, nil
		},
		describeTaskDefFunc: func(_ context.Context, in *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
			if awssdk.ToString(in.TaskDefinition) != "taskdef" {
				t.Fatal("lost task definition association")
			}
			var containers []ecstypes.ContainerDefinition
			for _, group := range []string{"good", "good", "denied", "empty", "remote"} {
				region := "us-east-1"
				if group == "remote" {
					region = "eu-west-1"
				}
				containers = append(containers, ecstypes.ContainerDefinition{LogConfiguration: &ecstypes.LogConfiguration{LogDriver: ecstypes.LogDriverAwslogs, Options: map[string]string{"awslogs-group": group, "awslogs-region": region}}})
			}
			return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &ecstypes.TaskDefinition{ContainerDefinitions: containers}}, nil
		},
		listTasksFunc: func(_ context.Context, in *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
			if awssdk.ToString(in.Cluster) != "cluster" || awssdk.ToString(in.ServiceName) != "checkout" || in.DesiredStatus != ecstypes.DesiredStatusStopped || awssdk.ToInt32(in.MaxResults) != 20 {
				t.Fatalf("unbounded or wrong task query: %+v", in)
			}
			return &ecs.ListTasksOutput{TaskArns: []string{"task"}, NextToken: awssdk.String("more")}, nil
		},
		describeTasksFunc: func(_ context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
			if len(in.Tasks) != 1 || in.Tasks[0] != "task" {
				t.Fatal("unrelated task query")
			}
			return &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{{TaskArn: awssdk.String("task"), Group: awssdk.String("service:checkout"), StoppedAt: &stopped, StoppedReason: awssdk.String("CannotPullContainerError"), Containers: []ecstypes.Container{{Name: awssdk.String("app"), ExitCode: awssdk.Int32(1)}}}, {Group: awssdk.String("service:other")}}, Failures: []ecstypes.Failure{{Reason: awssdk.String("MISSING")}}}, nil
		},
	}
	repo.CloudWatchLogsClient = &mockCloudWatchLogsClient{filterLogEventsFunc: func(_ context.Context, in *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
		logCalls++
		if awssdk.ToInt32(in.Limit) != 50 || *in.EndTime-*in.StartTime != int64(6*time.Hour/time.Millisecond) {
			t.Fatal("log query unbounded")
		}
		switch awssdk.ToString(in.LogGroupName) {
		case "good":
			return &cloudwatchlogs.FilterLogEventsOutput{Events: []cwltypes.FilteredLogEvent{{Message: awssdk.String("sample"), Timestamp: awssdk.Int64(stopped.UnixMilli())}}, NextToken: awssdk.String("more")}, nil
		case "denied":
			return nil, errors.New("AccessDenied")
		case "empty":
			return &cloudwatchlogs.FilterLogEventsOutput{NextToken: awssdk.String("more")}, nil
		default:
			t.Fatal("queried unassociated or cross-region log group")
			return nil, nil
		}
	}}
	repo.CloudTrailClient = &mockCloudTrailEventsClient{lookupEventsFunc: func(_ context.Context, in *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
		if len(in.LookupAttributes) != 1 || awssdk.ToString(in.LookupAttributes[0].AttributeValue) != service {
			t.Fatal("CloudTrail not scoped to service ARN")
		}
		return &cloudtrail.LookupEventsOutput{Events: []cttypes.Event{{EventName: awssdk.String("UpdateService"), ReadOnly: awssdk.String("false")}}}, nil
	}}
	result := repo.InvestigateECSService(context.Background(), "cluster", service)
	if result.Rollout == nil || len(result.StoppedTasks) != 1 || !result.StoppedTasks[0].StoppedAt.Equal(stopped) || len(result.Changes) != 1 || logCalls != 3 || len(result.Logs) != 3 {
		t.Fatalf("lost partial evidence: %+v calls=%d", result, logCalls)
	}
	if len(result.Logs[0].Events) != 1 || result.Logs[1].Warning != "AccessDenied" || result.Logs[2].Warning == "" {
		t.Fatalf("lost log evidence/empty-page warning: %+v", result.Logs)
	}
	warnings := strings.Join(result.Warnings, " ")
	for _, want := range []string{"one page", "MISSING", "awslogs-region"} {
		if !strings.Contains(warnings, want) {
			t.Fatalf("missing %q: %s", want, warnings)
		}
	}
}

func TestECSInvestigationCancellationAndSectionFailures(t *testing.T) {
	for _, cancelEarly := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		repo := &AwsRepository{}
		calls := 0
		repo.ECSClient = &mockECSClient{describeServicesFunc: func(context.Context, *ecs.DescribeServicesInput, ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
			calls++
			return nil, errors.New("AccessDenied")
		}, listTasksFunc: func(context.Context, *ecs.ListTasksInput, ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
			cancel()
			return nil, ctx.Err()
		}}
		if cancelEarly {
			cancel()
		}
		result := repo.InvestigateECSService(ctx, "cluster", "service")
		cancel()
		warnings := strings.Join(result.Warnings, " ")
		if !strings.Contains(warnings, "incomplete") || (!cancelEarly && !strings.Contains(warnings, "AccessDenied")) || (cancelEarly && calls != 0) {
			t.Fatalf("failure/cancellation hidden: %+v", result)
		}
	}
}
