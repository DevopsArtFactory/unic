package aws

import (
	"context"
	"fmt"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

const autoScalingActivityLimit int32 = 20

// ListAutoScalingGroups returns all groups in the active account and region.
func (r *AwsRepository) ListAutoScalingGroups(ctx context.Context) ([]AutoScalingGroup, error) {
	var groups []AutoScalingGroup
	paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(r.AutoScalingClient, &autoscaling.DescribeAutoScalingGroupsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe Auto Scaling groups: %w", err)
		}
		for _, group := range page.AutoScalingGroups {
			groups = append(groups, mapAutoScalingGroup(group))
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		return normalizedSortKey(groups[i].Name) < normalizedSortKey(groups[j].Name)
	})
	return groups, nil
}

// DescribeAutoScalingGroup returns a refreshed group and its recent activity.
func (r *AwsRepository) DescribeAutoScalingGroup(ctx context.Context, name string) (*AutoScalingGroup, []AutoScalingActivity, error) {
	output, err := r.AutoScalingClient.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{name},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to describe Auto Scaling group %s: %w", name, err)
	}
	if len(output.AutoScalingGroups) == 0 {
		return nil, nil, fmt.Errorf("Auto Scaling group %s not found", name)
	}

	activityOutput, err := r.AutoScalingClient.DescribeScalingActivities(ctx, &autoscaling.DescribeScalingActivitiesInput{
		AutoScalingGroupName: awssdk.String(name),
		MaxRecords:           awssdk.Int32(autoScalingActivityLimit),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to describe scaling activity for %s: %w", name, err)
	}
	activities := make([]AutoScalingActivity, 0, len(activityOutput.Activities))
	for _, activity := range activityOutput.Activities {
		activities = append(activities, mapAutoScalingActivity(activity))
	}
	sort.SliceStable(activities, func(i, j int) bool {
		return activities[i].StartTime.After(activities[j].StartTime)
	})
	group := mapAutoScalingGroup(output.AutoScalingGroups[0])
	return &group, activities, nil
}

// SetAutoScalingDesiredCapacity requests a new desired capacity for a group.
func (r *AwsRepository) SetAutoScalingDesiredCapacity(ctx context.Context, name string, desired int32) error {
	_, err := r.AutoScalingClient.SetDesiredCapacity(ctx, &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: awssdk.String(name),
		DesiredCapacity:      awssdk.Int32(desired),
	})
	if err != nil {
		return fmt.Errorf("failed to set desired capacity for %s: %w", name, err)
	}
	return nil
}

func mapAutoScalingGroup(group asgtypes.AutoScalingGroup) AutoScalingGroup {
	mapped := AutoScalingGroup{
		Name:            awssdk.ToString(group.AutoScalingGroupName),
		ARN:             awssdk.ToString(group.AutoScalingGroupARN),
		Status:          awssdk.ToString(group.Status),
		HealthCheckType: awssdk.ToString(group.HealthCheckType),
		DesiredCapacity: awssdk.ToInt32(group.DesiredCapacity),
		MinSize:         awssdk.ToInt32(group.MinSize),
		MaxSize:         awssdk.ToInt32(group.MaxSize),
		Instances:       make([]AutoScalingInstance, 0, len(group.Instances)),
	}
	for _, instance := range group.Instances {
		mapped.Instances = append(mapped.Instances, AutoScalingInstance{
			ID:                   awssdk.ToString(instance.InstanceId),
			AvailabilityZone:     awssdk.ToString(instance.AvailabilityZone),
			InstanceType:         awssdk.ToString(instance.InstanceType),
			LifecycleState:       string(instance.LifecycleState),
			HealthStatus:         awssdk.ToString(instance.HealthStatus),
			ProtectedFromScaleIn: awssdk.ToBool(instance.ProtectedFromScaleIn),
		})
	}
	sort.Slice(mapped.Instances, func(i, j int) bool {
		return normalizedSortKey(mapped.Instances[i].ID) < normalizedSortKey(mapped.Instances[j].ID)
	})
	return mapped
}

func mapAutoScalingActivity(activity asgtypes.Activity) AutoScalingActivity {
	mapped := AutoScalingActivity{
		Status:        string(activity.StatusCode),
		Description:   awssdk.ToString(activity.Description),
		Cause:         awssdk.ToString(activity.Cause),
		StatusMessage: awssdk.ToString(activity.StatusMessage),
	}
	if activity.StartTime != nil {
		mapped.StartTime = *activity.StartTime
	}
	return mapped
}
