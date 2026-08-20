package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

type mockCloudFormationClient struct {
	describeStacks func(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error)
	describeEvents func(context.Context, *cloudformation.DescribeStackEventsInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error)
	detectDrift    func(context.Context, *cloudformation.DetectStackDriftInput, ...func(*cloudformation.Options)) (*cloudformation.DetectStackDriftOutput, error)
	describeDrift  func(context.Context, *cloudformation.DescribeStackDriftDetectionStatusInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStackDriftDetectionStatusOutput, error)
}

func (m *mockCloudFormationClient) DescribeStacks(ctx context.Context, input *cloudformation.DescribeStacksInput, opts ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	return m.describeStacks(ctx, input, opts...)
}

func (m *mockCloudFormationClient) DescribeStackEvents(ctx context.Context, input *cloudformation.DescribeStackEventsInput, opts ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error) {
	return m.describeEvents(ctx, input, opts...)
}

func (m *mockCloudFormationClient) DetectStackDrift(ctx context.Context, input *cloudformation.DetectStackDriftInput, opts ...func(*cloudformation.Options)) (*cloudformation.DetectStackDriftOutput, error) {
	return m.detectDrift(ctx, input, opts...)
}

func (m *mockCloudFormationClient) DescribeStackDriftDetectionStatus(ctx context.Context, input *cloudformation.DescribeStackDriftDetectionStatusInput, opts ...func(*cloudformation.Options)) (*cloudformation.DescribeStackDriftDetectionStatusOutput, error) {
	return m.describeDrift(ctx, input, opts...)
}

func TestListCloudFormationStacksPaginatesMapsAndPrioritizesFailures(t *testing.T) {
	now := time.Date(2026, 8, 20, 4, 0, 0, 0, time.UTC)
	calls := 0
	client := &mockCloudFormationClient{
		describeStacks: func(_ context.Context, input *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			calls++
			if calls == 1 {
				if input.NextToken != nil {
					t.Fatalf("expected empty first-page token, got %q", awssdk.ToString(input.NextToken))
				}
				return &cloudformation.DescribeStacksOutput{
					NextToken: awssdk.String("page-2"),
					Stacks: []cloudformationtypes.Stack{
						cloudFormationSDKStack("healthy", cloudformationtypes.StackStatusCreateComplete, now),
						cloudFormationSDKStack("rollback", cloudformationtypes.StackStatusUpdateRollbackComplete, now),
					},
				}, nil
			}
			if awssdk.ToString(input.NextToken) != "page-2" {
				t.Fatalf("expected second-page token, got %q", awssdk.ToString(input.NextToken))
			}
			return &cloudformation.DescribeStacksOutput{Stacks: []cloudformationtypes.Stack{
				cloudFormationSDKStack("failed", cloudformationtypes.StackStatusCreateFailed, now),
				cloudFormationSDKStack("updating", cloudformationtypes.StackStatusUpdateInProgress, now),
			}}, nil
		},
	}
	repo := &AwsRepository{CloudFormationClient: client, Region: "ap-northeast-2"}

	stacks, err := repo.ListCloudFormationStacks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(stacks) != 4 {
		t.Fatalf("expected two pages and four stacks, got calls=%d stacks=%+v", calls, stacks)
	}
	if stacks[0].Name != "failed" || stacks[1].Name != "rollback" || stacks[2].Name != "updating" || stacks[3].Name != "healthy" {
		t.Fatalf("expected failed/rollback then in-progress then healthy, got %+v", stacks)
	}
	if stacks[0].DriftStatus != "IN_SYNC" || stacks[0].Region != "ap-northeast-2" || len(stacks[0].Parameters) != 1 || len(stacks[0].Outputs) != 1 {
		t.Fatalf("expected stack metadata to be mapped, got %+v", stacks[0])
	}
	for _, value := range []string{"failed", "create_failed", "in_sync", "ap-northeast-2"} {
		if !strings.Contains(stacks[0].FilterText(), value) {
			t.Fatalf("expected filter text to contain %q: %q", value, stacks[0].FilterText())
		}
	}
}

func TestGetCloudFormationStackMapsNewestThirtyEventsAndReasons(t *testing.T) {
	now := time.Date(2026, 8, 20, 4, 0, 0, 0, time.UTC)
	events := make([]cloudformationtypes.StackEvent, 35)
	for i := range events {
		events[i] = cloudformationtypes.StackEvent{
			EventId: awssdk.String(fmt.Sprintf("event-%02d", i)), StackId: awssdk.String("stack-id"), StackName: awssdk.String("prod"),
			Timestamp: awssdk.Time(now.Add(-time.Duration(i) * time.Minute)), LogicalResourceId: awssdk.String(fmt.Sprintf("Resource%d", i)),
			ResourceType: awssdk.String("AWS::S3::Bucket"), ResourceStatus: cloudformationtypes.ResourceStatusCreateFailed,
			ResourceStatusReason: awssdk.String("bucket name already exists"),
		}
	}
	eventCalls := 0
	client := &mockCloudFormationClient{
		describeStacks: func(_ context.Context, input *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			if awssdk.ToString(input.StackName) != "stack-id" {
				t.Fatalf("expected stack ID lookup, got %q", awssdk.ToString(input.StackName))
			}
			return &cloudformation.DescribeStacksOutput{Stacks: []cloudformationtypes.Stack{
				cloudFormationSDKStack("prod", cloudformationtypes.StackStatusCreateFailed, now),
			}}, nil
		},
		describeEvents: func(_ context.Context, input *cloudformation.DescribeStackEventsInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error) {
			if awssdk.ToString(input.StackName) != "stack-id" {
				t.Fatalf("expected event lookup by stack ID, got %q", awssdk.ToString(input.StackName))
			}
			eventCalls++
			switch eventCalls {
			case 1:
				if input.NextToken != nil {
					t.Fatalf("expected empty first-page token, got %q", awssdk.ToString(input.NextToken))
				}
				return &cloudformation.DescribeStackEventsOutput{StackEvents: events[:10], NextToken: awssdk.String("page-2")}, nil
			case 2:
				if awssdk.ToString(input.NextToken) != "page-2" {
					t.Fatalf("expected second-page token, got %q", awssdk.ToString(input.NextToken))
				}
				return &cloudformation.DescribeStackEventsOutput{StackEvents: events[10:]}, nil
			default:
				t.Fatalf("unexpected event page %d", eventCalls)
				return nil, nil
			}
		},
	}

	stack, err := (&AwsRepository{CloudFormationClient: client}).GetCloudFormationStack(context.Background(), "stack-id")
	if err != nil {
		t.Fatal(err)
	}
	if eventCalls != 2 || len(stack.Events) != cloudFormationRecentEventLimit {
		t.Fatalf("expected two pages and %d bounded recent events, got calls=%d events=%d", cloudFormationRecentEventLimit, eventCalls, len(stack.Events))
	}
	if stack.Events[0].LogicalResourceID != "Resource0" || stack.Events[0].Reason != "bucket name already exists" || stack.Events[29].LogicalResourceID != "Resource29" {
		t.Fatalf("unexpected mapped events: first=%+v last=%+v", stack.Events[0], stack.Events[29])
	}
}

func TestCloudFormationDriftDetectionStartsAndMapsStatus(t *testing.T) {
	now := time.Date(2026, 8, 20, 4, 0, 0, 0, time.UTC)
	client := &mockCloudFormationClient{
		detectDrift: func(_ context.Context, input *cloudformation.DetectStackDriftInput, _ ...func(*cloudformation.Options)) (*cloudformation.DetectStackDriftOutput, error) {
			if awssdk.ToString(input.StackName) != "stack-id" {
				t.Fatalf("expected drift lookup by stack ID, got %q", awssdk.ToString(input.StackName))
			}
			return &cloudformation.DetectStackDriftOutput{StackDriftDetectionId: awssdk.String("detect-1")}, nil
		},
		describeDrift: func(_ context.Context, input *cloudformation.DescribeStackDriftDetectionStatusInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackDriftDetectionStatusOutput, error) {
			if awssdk.ToString(input.StackDriftDetectionId) != "detect-1" {
				t.Fatalf("expected detection ID, got %q", awssdk.ToString(input.StackDriftDetectionId))
			}
			return &cloudformation.DescribeStackDriftDetectionStatusOutput{
				DetectionStatus:           cloudformationtypes.StackDriftDetectionStatusDetectionComplete,
				StackDriftStatus:          cloudformationtypes.StackDriftStatusDrifted,
				DriftedStackResourceCount: awssdk.Int32(2), Timestamp: awssdk.Time(now),
				StackDriftDetectionId: awssdk.String("detect-1"), StackId: awssdk.String("stack-id"),
			}, nil
		},
	}
	repo := &AwsRepository{CloudFormationClient: client}

	id, err := repo.DetectCloudFormationStackDrift(context.Background(), "stack-id")
	if err != nil || id != "detect-1" {
		t.Fatalf("expected detection ID, got id=%q err=%v", id, err)
	}
	status, err := repo.GetCloudFormationStackDriftDetection(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if status.DetectionStatus != "DETECTION_COMPLETE" || status.StackDriftStatus != "DRIFTED" || status.DriftedResources != 2 || !status.Timestamp.Equal(now) {
		t.Fatalf("unexpected drift status: %+v", status)
	}
}

func TestCloudFormationOperationsWrapErrors(t *testing.T) {
	client := &mockCloudFormationClient{
		describeStacks: func(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return nil, errors.New("denied")
		},
		detectDrift: func(context.Context, *cloudformation.DetectStackDriftInput, ...func(*cloudformation.Options)) (*cloudformation.DetectStackDriftOutput, error) {
			return nil, errors.New("denied")
		},
		describeDrift: func(context.Context, *cloudformation.DescribeStackDriftDetectionStatusInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStackDriftDetectionStatusOutput, error) {
			return nil, errors.New("denied")
		},
	}
	repo := &AwsRepository{CloudFormationClient: client}

	if _, err := repo.ListCloudFormationStacks(context.Background()); err == nil || !strings.Contains(err.Error(), "describe CloudFormation stacks") {
		t.Fatalf("expected list error context, got %v", err)
	}
	if _, err := repo.DetectCloudFormationStackDrift(context.Background(), "stack-id"); err == nil || !strings.Contains(err.Error(), "start drift detection") {
		t.Fatalf("expected detect error context, got %v", err)
	}
	if _, err := repo.GetCloudFormationStackDriftDetection(context.Background(), "detect-1"); err == nil || !strings.Contains(err.Error(), "describe CloudFormation drift detection") {
		t.Fatalf("expected status error context, got %v", err)
	}
}

func TestGetCloudFormationStackWrapsEventErrorsAndHandlesMissingStack(t *testing.T) {
	client := &mockCloudFormationClient{
		describeStacks: func(_ context.Context, input *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			if awssdk.ToString(input.StackName) == "missing" {
				return &cloudformation.DescribeStacksOutput{}, nil
			}
			return &cloudformation.DescribeStacksOutput{Stacks: []cloudformationtypes.Stack{
				cloudFormationSDKStack("prod", cloudformationtypes.StackStatusCreateComplete, time.Now()),
			}}, nil
		},
		describeEvents: func(context.Context, *cloudformation.DescribeStackEventsInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error) {
			return nil, errors.New("denied")
		},
	}
	repo := &AwsRepository{CloudFormationClient: client}

	if _, err := repo.GetCloudFormationStack(context.Background(), "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing-stack error, got %v", err)
	}
	if _, err := repo.GetCloudFormationStack(context.Background(), "prod-id"); err == nil || !strings.Contains(err.Error(), "describe events") {
		t.Fatalf("expected event error context, got %v", err)
	}
}

func cloudFormationSDKStack(name string, status cloudformationtypes.StackStatus, now time.Time) cloudformationtypes.Stack {
	return cloudformationtypes.Stack{
		StackId: awssdk.String(name + "-id"), StackName: awssdk.String(name), StackStatus: status,
		StackStatusReason: awssdk.String("status reason"), CreationTime: awssdk.Time(now.Add(-time.Hour)), LastUpdatedTime: awssdk.Time(now),
		EnableTerminationProtection: awssdk.Bool(true),
		DriftInformation:            &cloudformationtypes.StackDriftInformation{StackDriftStatus: cloudformationtypes.StackDriftStatusInSync, LastCheckTimestamp: awssdk.Time(now)},
		Parameters:                  []cloudformationtypes.Parameter{{ParameterKey: awssdk.String("Environment"), ParameterValue: awssdk.String("prod")}},
		Outputs:                     []cloudformationtypes.Output{{OutputKey: awssdk.String("Endpoint"), OutputValue: awssdk.String("example.com"), ExportName: awssdk.String("prod-endpoint")}},
	}
}
