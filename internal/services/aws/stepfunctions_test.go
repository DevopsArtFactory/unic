package aws

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
)

type mockStepFunctionsClient struct {
	listStateMachines func(context.Context, *sfn.ListStateMachinesInput, ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error)
	listExecutions    func(context.Context, *sfn.ListExecutionsInput, ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error)
	describeExecution func(context.Context, *sfn.DescribeExecutionInput, ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error)
	executionHistory  func(context.Context, *sfn.GetExecutionHistoryInput, ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error)
}

func (m *mockStepFunctionsClient) ListStateMachines(ctx context.Context, in *sfn.ListStateMachinesInput, opts ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error) {
	return m.listStateMachines(ctx, in, opts...)
}

func (m *mockStepFunctionsClient) ListExecutions(ctx context.Context, in *sfn.ListExecutionsInput, opts ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error) {
	return m.listExecutions(ctx, in, opts...)
}

func (m *mockStepFunctionsClient) DescribeExecution(ctx context.Context, in *sfn.DescribeExecutionInput, opts ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error) {
	return m.describeExecution(ctx, in, opts...)
}

func (m *mockStepFunctionsClient) GetExecutionHistory(ctx context.Context, in *sfn.GetExecutionHistoryInput, opts ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error) {
	return m.executionHistory(ctx, in, opts...)
}

func TestListStepFunctionStateMachinesPaginatesAndSorts(t *testing.T) {
	created := time.Date(2026, 8, 20, 1, 2, 0, 0, time.UTC)
	calls := 0
	client := &mockStepFunctionsClient{listStateMachines: func(_ context.Context, in *sfn.ListStateMachinesInput, _ ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error) {
		calls++
		if calls == 1 {
			if in.NextToken != nil {
				t.Fatalf("expected first page without token, got %q", awssdk.ToString(in.NextToken))
			}
			return &sfn.ListStateMachinesOutput{
				StateMachines: []sfntypes.StateMachineListItem{{
					StateMachineArn: awssdk.String("arn:zeta"), Name: awssdk.String("zeta"), Type: sfntypes.StateMachineTypeExpress,
					CreationDate: awssdk.Time(created),
				}},
				NextToken: awssdk.String("page-2"),
			}, nil
		}
		if awssdk.ToString(in.NextToken) != "page-2" {
			t.Fatalf("expected second-page token, got %q", awssdk.ToString(in.NextToken))
		}
		return &sfn.ListStateMachinesOutput{StateMachines: []sfntypes.StateMachineListItem{{
			StateMachineArn: awssdk.String("arn:alpha"), Name: awssdk.String("Alpha"), Type: sfntypes.StateMachineTypeStandard,
			CreationDate: awssdk.Time(created.Add(-time.Hour)),
		}}}, nil
	}}

	items, err := (&AwsRepository{StepFunctionsClient: client, Region: "us-east-1"}).ListStepFunctionStateMachines(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(items) != 2 || items[0].Name != "Alpha" || items[0].Region != "us-east-1" || items[0].Type != "STANDARD" {
		t.Fatalf("unexpected state machines: calls=%d items=%+v", calls, items)
	}
}

func TestListStepFunctionExecutionsSortsFailuresFirst(t *testing.T) {
	now := time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC)
	client := &mockStepFunctionsClient{listExecutions: func(_ context.Context, in *sfn.ListExecutionsInput, _ ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error) {
		if awssdk.ToString(in.StateMachineArn) != "arn:machine" || in.MaxResults != stepFunctionMaxExecutions {
			t.Fatalf("unexpected list input: %+v", in)
		}
		return &sfn.ListExecutionsOutput{Executions: []sfntypes.ExecutionListItem{
			{ExecutionArn: awssdk.String("arn:succeeded"), Name: awssdk.String("succeeded"), StateMachineArn: in.StateMachineArn, Status: sfntypes.ExecutionStatusSucceeded, StartDate: awssdk.Time(now)},
			{ExecutionArn: awssdk.String("arn:old-failure"), Name: awssdk.String("old-failure"), StateMachineArn: in.StateMachineArn, Status: sfntypes.ExecutionStatusFailed, StartDate: awssdk.Time(now.Add(-time.Hour))},
			{ExecutionArn: awssdk.String("arn:new-failure"), Name: awssdk.String("new-failure"), StateMachineArn: in.StateMachineArn, Status: sfntypes.ExecutionStatusFailed, StartDate: awssdk.Time(now)},
			{ExecutionArn: awssdk.String("arn:running"), Name: awssdk.String("running"), StateMachineArn: in.StateMachineArn, Status: sfntypes.ExecutionStatusRunning, StartDate: awssdk.Time(now.Add(time.Hour))},
		}}, nil
	}}

	executions, err := (&AwsRepository{StepFunctionsClient: client}).ListStepFunctionExecutions(context.Background(), "arn:machine")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"new-failure", "old-failure", "running", "succeeded"}
	for i, name := range want {
		if executions[i].Name != name {
			t.Fatalf("expected order %v, got %+v", want, executions)
		}
	}
	if !executions[0].NeedsAttention() || executions[3].NeedsAttention() || !strings.Contains(executions[0].FilterText(), "failed") {
		t.Fatalf("unexpected execution helpers: %+v", executions)
	}
}

func TestDescribeStepFunctionExecutionFindsFailedStateAcrossHistoryPages(t *testing.T) {
	historyCalls := 0
	client := &mockStepFunctionsClient{
		describeExecution: func(_ context.Context, in *sfn.DescribeExecutionInput, _ ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error) {
			if awssdk.ToString(in.ExecutionArn) != "arn:execution" {
				t.Fatalf("unexpected execution ARN: %q", awssdk.ToString(in.ExecutionArn))
			}
			return &sfn.DescribeExecutionOutput{
				ExecutionArn: awssdk.String("arn:execution"), StateMachineArn: awssdk.String("arn:machine"), Name: awssdk.String("run-1"),
				Status: sfntypes.ExecutionStatusFailed, StartDate: awssdk.Time(time.Now()), Input: awssdk.String(`{"order":42}`),
				Error: awssdk.String("States.TaskFailed"), Cause: awssdk.String("payment rejected"),
			}, nil
		},
		executionHistory: func(_ context.Context, in *sfn.GetExecutionHistoryInput, _ ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error) {
			historyCalls++
			if !in.ReverseOrder || awssdk.ToBool(in.IncludeExecutionData) || in.MaxResults != 1000 {
				t.Fatalf("expected reverse metadata-only history, got %+v", in)
			}
			if historyCalls == 1 {
				return &sfn.GetExecutionHistoryOutput{Events: []sfntypes.HistoryEvent{
					{Id: 4, Type: sfntypes.HistoryEventTypeExecutionFailed, PreviousEventId: 3},
					{Id: 3, Type: sfntypes.HistoryEventTypeTaskFailed, PreviousEventId: 2},
				}, NextToken: awssdk.String("older")}, nil
			}
			if awssdk.ToString(in.NextToken) != "older" {
				t.Fatalf("expected history token, got %q", awssdk.ToString(in.NextToken))
			}
			return &sfn.GetExecutionHistoryOutput{Events: []sfntypes.HistoryEvent{
				{Id: 2, Type: sfntypes.HistoryEventTypeTaskStarted, PreviousEventId: 1},
				{Id: 1, Type: sfntypes.HistoryEventTypeTaskStateEntered, StateEnteredEventDetails: &sfntypes.StateEnteredEventDetails{Name: awssdk.String("ChargeCard")}},
			}}, nil
		},
	}

	detail, err := (&AwsRepository{StepFunctionsClient: client}).DescribeStepFunctionExecution(context.Background(), "arn:execution")
	if err != nil {
		t.Fatal(err)
	}
	if historyCalls != 2 || detail.FailedStep != "ChargeCard" || detail.Error != "States.TaskFailed" || detail.Cause != "payment rejected" || detail.Input != `{"order":42}` {
		t.Fatalf("unexpected detail: calls=%d detail=%+v", historyCalls, detail)
	}
}

func TestDescribeStepFunctionExecutionSkipsHistoryForSuccess(t *testing.T) {
	client := &mockStepFunctionsClient{
		describeExecution: func(context.Context, *sfn.DescribeExecutionInput, ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error) {
			return &sfn.DescribeExecutionOutput{ExecutionArn: awssdk.String("arn:ok"), StateMachineArn: awssdk.String("arn:machine"), Name: awssdk.String("ok"), Status: sfntypes.ExecutionStatusSucceeded, StartDate: awssdk.Time(time.Now()), Output: awssdk.String(`{"ok":true}`)}, nil
		},
		executionHistory: func(context.Context, *sfn.GetExecutionHistoryInput, ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error) {
			t.Fatal("successful execution should not load failure history")
			return nil, nil
		},
	}

	detail, err := (&AwsRepository{StepFunctionsClient: client}).DescribeStepFunctionExecution(context.Background(), "arn:ok")
	if err != nil || detail.Output != `{"ok":true}` {
		t.Fatalf("unexpected success detail: detail=%+v err=%v", detail, err)
	}
}

func TestDescribeStepFunctionExecutionReturnsHistoryError(t *testing.T) {
	client := &mockStepFunctionsClient{
		describeExecution: func(context.Context, *sfn.DescribeExecutionInput, ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error) {
			return &sfn.DescribeExecutionOutput{ExecutionArn: awssdk.String("arn:failed"), StateMachineArn: awssdk.String("arn:machine"), Name: awssdk.String("failed"), Status: sfntypes.ExecutionStatusFailed, StartDate: awssdk.Time(time.Now())}, nil
		},
		executionHistory: func(context.Context, *sfn.GetExecutionHistoryInput, ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	_, err := (&AwsRepository{StepFunctionsClient: client}).DescribeStepFunctionExecution(context.Background(), "arn:failed")
	if err == nil || !strings.Contains(err.Error(), "failed to get Step Functions execution history") || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("expected contextual history error, got %v", err)
	}
}
