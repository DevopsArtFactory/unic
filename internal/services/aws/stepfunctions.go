package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	uniclog "unic/internal/log"
)

const (
	stepFunctionMaxExecutions   = 200
	stepFunctionMaxHistoryPages = 5
)

// ListStepFunctionStateMachines returns every state machine in the active region.
func (r *AwsRepository) ListStepFunctionStateMachines(ctx context.Context) ([]StepFunctionStateMachine, error) {
	uniclog.Debug("aws", "ListStepFunctionStateMachines called")

	var stateMachines []StepFunctionStateMachine
	var nextToken *string
	for {
		out, err := r.StepFunctionsClient.ListStateMachines(ctx, &sfn.ListStateMachinesInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("failed to list Step Functions state machines: %w", err)
		}
		for _, item := range out.StateMachines {
			stateMachines = append(stateMachines, StepFunctionStateMachine{
				ARN:          awssdk.ToString(item.StateMachineArn),
				Name:         awssdk.ToString(item.Name),
				Type:         string(item.Type),
				CreationDate: awssdk.ToTime(item.CreationDate),
				Region:       r.Region,
			})
		}
		if awssdk.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}

	sort.Slice(stateMachines, func(i, j int) bool {
		left, right := normalizedSortKey(stateMachines[i].Name), normalizedSortKey(stateMachines[j].Name)
		if left == right {
			return stateMachines[i].ARN < stateMachines[j].ARN
		}
		return left < right
	})
	return stateMachines, nil
}

// ListStepFunctionExecutions returns up to 200 recent executions, failures first.
func (r *AwsRepository) ListStepFunctionExecutions(ctx context.Context, stateMachineARN string) ([]StepFunctionExecution, error) {
	uniclog.Debug("aws", "ListStepFunctionExecutions called", "state_machine", stateMachineARN)

	executions := make([]StepFunctionExecution, 0, stepFunctionMaxExecutions)
	var nextToken *string
	for len(executions) < stepFunctionMaxExecutions {
		out, err := r.StepFunctionsClient.ListExecutions(ctx, &sfn.ListExecutionsInput{
			StateMachineArn: awssdk.String(stateMachineARN),
			MaxResults:      stepFunctionMaxExecutions,
			NextToken:       nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list Step Functions executions for %s: %w", stateMachineARN, err)
		}
		for _, item := range out.Executions {
			executions = append(executions, mapStepFunctionExecution(item))
			if len(executions) == stepFunctionMaxExecutions {
				break
			}
		}
		if len(executions) == stepFunctionMaxExecutions || awssdk.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}

	sort.SliceStable(executions, func(i, j int) bool {
		left, right := stepFunctionStatusRank(executions[i].Status), stepFunctionStatusRank(executions[j].Status)
		if left != right {
			return left < right
		}
		if executions[i].StartDate.Equal(executions[j].StartDate) {
			return normalizedSortKey(executions[i].Name) < normalizedSortKey(executions[j].Name)
		}
		return executions[i].StartDate.After(executions[j].StartDate)
	})
	return executions, nil
}

// DescribeStepFunctionExecution returns execution data and the failed state when available.
func (r *AwsRepository) DescribeStepFunctionExecution(ctx context.Context, executionARN string) (*StepFunctionExecutionDetail, error) {
	uniclog.Debug("aws", "DescribeStepFunctionExecution called", "execution", executionARN)

	out, err := r.StepFunctionsClient.DescribeExecution(ctx, &sfn.DescribeExecutionInput{ExecutionArn: awssdk.String(executionARN)})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Step Functions execution %s: %w", executionARN, err)
	}
	detail := &StepFunctionExecutionDetail{
		StepFunctionExecution: StepFunctionExecution{
			ARN:             awssdk.ToString(out.ExecutionArn),
			Name:            awssdk.ToString(out.Name),
			StateMachineARN: awssdk.ToString(out.StateMachineArn),
			Status:          string(out.Status),
			StartDate:       awssdk.ToTime(out.StartDate),
			StopDate:        awssdk.ToTime(out.StopDate),
		},
		Input:  awssdk.ToString(out.Input),
		Output: awssdk.ToString(out.Output),
		Error:  awssdk.ToString(out.Error),
		Cause:  awssdk.ToString(out.Cause),
	}
	if detail.NeedsAttention() {
		failedStep, historyErr := r.stepFunctionFailedStep(ctx, executionARN)
		if historyErr != nil {
			uniclog.Debug("aws", "Step Functions failed state unavailable", "execution", executionARN, "error", historyErr.Error())
		} else {
			detail.FailedStep = failedStep
		}
	}
	return detail, nil
}

func mapStepFunctionExecution(item sfntypes.ExecutionListItem) StepFunctionExecution {
	return StepFunctionExecution{
		ARN:             awssdk.ToString(item.ExecutionArn),
		Name:            awssdk.ToString(item.Name),
		StateMachineARN: awssdk.ToString(item.StateMachineArn),
		Status:          string(item.Status),
		StartDate:       awssdk.ToTime(item.StartDate),
		StopDate:        awssdk.ToTime(item.StopDate),
	}
}

func stepFunctionStatusRank(status string) int {
	switch strings.ToUpper(status) {
	case "FAILED":
		return 0
	case "TIMED_OUT":
		return 1
	case "ABORTED":
		return 2
	case "PENDING_REDRIVE":
		return 3
	case "RUNNING":
		return 4
	case "SUCCEEDED":
		return 5
	default:
		return 6
	}
}

func (r *AwsRepository) stepFunctionFailedStep(ctx context.Context, executionARN string) (string, error) {
	events := make(map[int64]sfntypes.HistoryEvent)
	var failureID int64
	var nextToken *string
	for range stepFunctionMaxHistoryPages {
		out, err := r.StepFunctionsClient.GetExecutionHistory(ctx, &sfn.GetExecutionHistoryInput{
			ExecutionArn:         awssdk.String(executionARN),
			IncludeExecutionData: awssdk.Bool(false),
			MaxResults:           1000,
			NextToken:            nextToken,
			ReverseOrder:         true,
		})
		if err != nil {
			return "", fmt.Errorf("failed to get Step Functions execution history for %s: %w", executionARN, err)
		}
		for _, event := range out.Events {
			events[event.Id] = event
			if failureID == 0 && isStepFunctionFailureEvent(event.Type) {
				failureID = event.Id
			}
		}
		if failureID != 0 {
			if step, complete := traceStepFunctionFailedStep(events, failureID); complete {
				return step, nil
			}
		}
		if awssdk.ToString(out.NextToken) == "" {
			return "", nil
		}
		nextToken = out.NextToken
	}
	return "", nil
}

func isStepFunctionFailureEvent(eventType sfntypes.HistoryEventType) bool {
	name := string(eventType)
	return strings.HasSuffix(name, "Failed") || strings.HasSuffix(name, "TimedOut") || strings.HasSuffix(name, "Aborted")
}

func traceStepFunctionFailedStep(events map[int64]sfntypes.HistoryEvent, failureID int64) (string, bool) {
	visited := make(map[int64]struct{})
	for eventID := failureID; eventID != 0; {
		if _, seen := visited[eventID]; seen {
			return "", true
		}
		visited[eventID] = struct{}{}

		event, ok := events[eventID]
		if !ok {
			return "", false
		}
		if event.EvaluationFailedEventDetails != nil {
			return awssdk.ToString(event.EvaluationFailedEventDetails.State), true
		}
		if event.StateEnteredEventDetails != nil {
			return awssdk.ToString(event.StateEnteredEventDetails.Name), true
		}
		eventID = event.PreviousEventId
	}
	return "", true
}
