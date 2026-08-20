package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

const cloudFormationRecentEventLimit = 30

// ListCloudFormationStacks returns stacks ordered with failed and rollback states first.
func (r *AwsRepository) ListCloudFormationStacks(ctx context.Context) ([]CloudFormationStack, error) {
	paginator := cloudformation.NewDescribeStacksPaginator(r.CloudFormationClient, &cloudformation.DescribeStacksInput{})
	var stacks []CloudFormationStack
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe CloudFormation stacks: %w", err)
		}
		for _, stack := range page.Stacks {
			stacks = append(stacks, mapCloudFormationStack(stack, r.Region))
		}
	}
	sortCloudFormationStacks(stacks)
	return stacks, nil
}

// GetCloudFormationStack returns refreshed stack metadata and its newest events.
func (r *AwsRepository) GetCloudFormationStack(ctx context.Context, stackID string) (*CloudFormationStack, error) {
	out, err := r.CloudFormationClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: awssdk.String(stackID)})
	if err != nil {
		return nil, fmt.Errorf("failed to describe CloudFormation stack %s: %w", stackID, err)
	}
	if len(out.Stacks) == 0 {
		return nil, fmt.Errorf("CloudFormation stack %s not found", stackID)
	}

	stack := mapCloudFormationStack(out.Stacks[0], r.Region)
	paginator := cloudformation.NewDescribeStackEventsPaginator(r.CloudFormationClient, &cloudformation.DescribeStackEventsInput{StackName: awssdk.String(stackID)})
	for paginator.HasMorePages() && len(stack.Events) < cloudFormationRecentEventLimit {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe events for CloudFormation stack %s: %w", stackID, err)
		}
		for _, event := range page.StackEvents {
			if len(stack.Events) == cloudFormationRecentEventLimit {
				break
			}
			stack.Events = append(stack.Events, mapCloudFormationStackEvent(event))
		}
	}
	sort.SliceStable(stack.Events, func(i, j int) bool {
		return stack.Events[i].Timestamp.After(stack.Events[j].Timestamp)
	})
	return &stack, nil
}

// DetectCloudFormationStackDrift starts drift detection and returns its operation ID.
func (r *AwsRepository) DetectCloudFormationStackDrift(ctx context.Context, stackID string) (string, error) {
	out, err := r.CloudFormationClient.DetectStackDrift(ctx, &cloudformation.DetectStackDriftInput{StackName: awssdk.String(stackID)})
	if err != nil {
		return "", fmt.Errorf("failed to start drift detection for CloudFormation stack %s: %w", stackID, err)
	}
	if awssdk.ToString(out.StackDriftDetectionId) == "" {
		return "", fmt.Errorf("CloudFormation returned no drift detection ID for stack %s", stackID)
	}
	return awssdk.ToString(out.StackDriftDetectionId), nil
}

// GetCloudFormationStackDriftDetection returns the current drift operation status.
func (r *AwsRepository) GetCloudFormationStackDriftDetection(ctx context.Context, detectionID string) (*CloudFormationDriftDetection, error) {
	out, err := r.CloudFormationClient.DescribeStackDriftDetectionStatus(ctx, &cloudformation.DescribeStackDriftDetectionStatusInput{
		StackDriftDetectionId: awssdk.String(detectionID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe CloudFormation drift detection %s: %w", detectionID, err)
	}
	status := &CloudFormationDriftDetection{
		DetectionStatus:  string(out.DetectionStatus),
		StackDriftStatus: string(out.StackDriftStatus),
		Reason:           awssdk.ToString(out.DetectionStatusReason),
		DriftedResources: awssdk.ToInt32(out.DriftedStackResourceCount),
	}
	if out.Timestamp != nil {
		status.Timestamp = *out.Timestamp
	}
	return status, nil
}

func mapCloudFormationStack(stack cloudformationtypes.Stack, region string) CloudFormationStack {
	mapped := CloudFormationStack{
		ID:                    awssdk.ToString(stack.StackId),
		Name:                  awssdk.ToString(stack.StackName),
		Description:           awssdk.ToString(stack.Description),
		Status:                string(stack.StackStatus),
		StatusReason:          awssdk.ToString(stack.StackStatusReason),
		Region:                region,
		TerminationProtection: awssdk.ToBool(stack.EnableTerminationProtection),
	}
	if stack.CreationTime != nil {
		mapped.CreatedAt = *stack.CreationTime
	}
	if stack.LastUpdatedTime != nil {
		mapped.UpdatedAt = *stack.LastUpdatedTime
	}
	if stack.DriftInformation != nil {
		mapped.DriftStatus = string(stack.DriftInformation.StackDriftStatus)
		if stack.DriftInformation.LastCheckTimestamp != nil {
			mapped.LastDriftCheck = *stack.DriftInformation.LastCheckTimestamp
		}
	}
	if mapped.DriftStatus == "" {
		mapped.DriftStatus = "NOT_CHECKED"
	}
	for _, parameter := range stack.Parameters {
		mapped.Parameters = append(mapped.Parameters, CloudFormationValue{
			Key: awssdk.ToString(parameter.ParameterKey), Value: awssdk.ToString(parameter.ParameterValue),
		})
	}
	for _, output := range stack.Outputs {
		mapped.Outputs = append(mapped.Outputs, CloudFormationValue{
			Key: awssdk.ToString(output.OutputKey), Value: awssdk.ToString(output.OutputValue),
			Description: awssdk.ToString(output.Description), ExportName: awssdk.ToString(output.ExportName),
		})
	}
	sort.Slice(mapped.Parameters, func(i, j int) bool { return mapped.Parameters[i].Key < mapped.Parameters[j].Key })
	sort.Slice(mapped.Outputs, func(i, j int) bool { return mapped.Outputs[i].Key < mapped.Outputs[j].Key })
	return mapped
}

func mapCloudFormationStackEvent(event cloudformationtypes.StackEvent) CloudFormationStackEvent {
	mapped := CloudFormationStackEvent{
		LogicalResourceID:  awssdk.ToString(event.LogicalResourceId),
		PhysicalResourceID: awssdk.ToString(event.PhysicalResourceId),
		ResourceType:       awssdk.ToString(event.ResourceType),
		Status:             string(event.ResourceStatus),
		Reason:             awssdk.ToString(event.ResourceStatusReason),
	}
	if event.Timestamp != nil {
		mapped.Timestamp = *event.Timestamp
	}
	return mapped
}

func sortCloudFormationStacks(stacks []CloudFormationStack) {
	sort.SliceStable(stacks, func(i, j int) bool {
		left, right := cloudFormationStatusPriority(stacks[i].Status), cloudFormationStatusPriority(stacks[j].Status)
		if left != right {
			return left < right
		}
		if stacks[i].Status != stacks[j].Status {
			return stacks[i].Status < stacks[j].Status
		}
		return normalizedSortKey(stacks[i].Name) < normalizedSortKey(stacks[j].Name)
	})
}

func cloudFormationStatusPriority(status string) int {
	status = strings.ToUpper(status)
	if strings.Contains(status, "FAILED") || strings.Contains(status, "ROLLBACK") {
		return 0
	}
	if strings.Contains(status, "IN_PROGRESS") {
		return 1
	}
	return 2
}
