package aws

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

type mockAutoScalingBrowserClient struct {
	describeGroups     func(context.Context, *autoscaling.DescribeAutoScalingGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
	describeActivities func(context.Context, *autoscaling.DescribeScalingActivitiesInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error)
	setDesired         func(context.Context, *autoscaling.SetDesiredCapacityInput, ...func(*autoscaling.Options)) (*autoscaling.SetDesiredCapacityOutput, error)
}

func (m *mockAutoScalingBrowserClient) DescribeAutoScalingInstances(context.Context, *autoscaling.DescribeAutoScalingInstancesInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingInstancesOutput, error) {
	return &autoscaling.DescribeAutoScalingInstancesOutput{}, nil
}

func (m *mockAutoScalingBrowserClient) DescribeAutoScalingGroups(ctx context.Context, input *autoscaling.DescribeAutoScalingGroupsInput, opts ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return m.describeGroups(ctx, input, opts...)
}

func (m *mockAutoScalingBrowserClient) DescribeScalingActivities(ctx context.Context, input *autoscaling.DescribeScalingActivitiesInput, opts ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	return m.describeActivities(ctx, input, opts...)
}

func (m *mockAutoScalingBrowserClient) SetDesiredCapacity(ctx context.Context, input *autoscaling.SetDesiredCapacityInput, opts ...func(*autoscaling.Options)) (*autoscaling.SetDesiredCapacityOutput, error) {
	return m.setDesired(ctx, input, opts...)
}

func TestListAutoScalingGroupsPaginatesMapsAndSorts(t *testing.T) {
	calls := 0
	mock := &mockAutoScalingBrowserClient{
		describeGroups: func(_ context.Context, input *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			calls++
			if calls == 1 {
				if input.NextToken != nil {
					t.Fatalf("unexpected first-page token: %q", awssdk.ToString(input.NextToken))
				}
				return &autoscaling.DescribeAutoScalingGroupsOutput{
					AutoScalingGroups: []asgtypes.AutoScalingGroup{{
						AutoScalingGroupName: awssdk.String("zeta"), DesiredCapacity: awssdk.Int32(2), MinSize: awssdk.Int32(1), MaxSize: awssdk.Int32(4), HealthCheckType: awssdk.String("EC2"),
						Instances: []asgtypes.Instance{{InstanceId: awssdk.String("i-2"), AvailabilityZone: awssdk.String("us-east-1b"), LifecycleState: asgtypes.LifecycleStateInService, HealthStatus: awssdk.String("Unhealthy"), ProtectedFromScaleIn: awssdk.Bool(true)}},
					}},
					NextToken: awssdk.String("next"),
				}, nil
			}
			if awssdk.ToString(input.NextToken) != "next" {
				t.Fatalf("expected pagination token, got %q", awssdk.ToString(input.NextToken))
			}
			return &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: []asgtypes.AutoScalingGroup{{
				AutoScalingGroupName: awssdk.String("alpha"), DesiredCapacity: awssdk.Int32(1), MinSize: awssdk.Int32(0), MaxSize: awssdk.Int32(2),
				Instances: []asgtypes.Instance{{InstanceId: awssdk.String("i-1"), LifecycleState: asgtypes.LifecycleStateInService, HealthStatus: awssdk.String("Healthy")}},
			}}}, nil
		},
		describeActivities: unusedAutoScalingActivities,
		setDesired:         unusedSetDesiredCapacity,
	}

	groups, err := (&AwsRepository{AutoScalingClient: mock}).ListAutoScalingGroups(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(groups) != 2 || groups[0].Name != "alpha" || groups[1].Name != "zeta" {
		t.Fatalf("unexpected groups: calls=%d groups=%+v", calls, groups)
	}
	if groups[0].HealthyInstanceCount() != 1 || groups[1].Instances[0].LifecycleState != "InService" || !groups[1].Instances[0].ProtectedFromScaleIn {
		t.Fatalf("unexpected instance mapping: %+v", groups)
	}
	if !strings.Contains(groups[1].FilterText(), "i-2") || !strings.Contains(groups[1].DisplayTitle(), "0/1") {
		t.Fatalf("expected searchable display metadata: %+v", groups[1])
	}
}

func TestDescribeAutoScalingGroupIncludesRecentFailureCause(t *testing.T) {
	started := time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC)
	mock := &mockAutoScalingBrowserClient{
		describeGroups: func(_ context.Context, input *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			if len(input.AutoScalingGroupNames) != 1 || input.AutoScalingGroupNames[0] != "api" {
				t.Fatalf("unexpected group input: %+v", input)
			}
			return &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: []asgtypes.AutoScalingGroup{{AutoScalingGroupName: awssdk.String("api")}}}, nil
		},
		describeActivities: func(_ context.Context, input *autoscaling.DescribeScalingActivitiesInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
			if awssdk.ToString(input.AutoScalingGroupName) != "api" || awssdk.ToInt32(input.MaxRecords) != autoScalingActivityLimit {
				t.Fatalf("unexpected activity input: %+v", input)
			}
			return &autoscaling.DescribeScalingActivitiesOutput{Activities: []asgtypes.Activity{{
				ActivityId: awssdk.String("activity-1"), AutoScalingGroupName: awssdk.String("api"), Cause: awssdk.String("capacity update"), StartTime: &started,
				StatusCode: asgtypes.ScalingActivityStatusCodeFailed, StatusMessage: awssdk.String("launch template is invalid"), Description: awssdk.String("Launching instance"),
			}}}, nil
		},
		setDesired: unusedSetDesiredCapacity,
	}

	group, activities, err := (&AwsRepository{AutoScalingClient: mock}).DescribeAutoScalingGroup(context.Background(), "api")
	if err != nil {
		t.Fatal(err)
	}
	if group.Name != "api" || len(activities) != 1 || activities[0].Status != "Failed" || activities[0].StatusMessage != "launch template is invalid" || !activities[0].StartTime.Equal(started) {
		t.Fatalf("unexpected detail: group=%+v activities=%+v", group, activities)
	}
}

func TestDescribeAutoScalingGroupErrors(t *testing.T) {
	mock := &mockAutoScalingBrowserClient{
		describeGroups: func(context.Context, *autoscaling.DescribeAutoScalingGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			return nil, errors.New("denied")
		},
		describeActivities: unusedAutoScalingActivities,
		setDesired:         unusedSetDesiredCapacity,
	}
	if _, _, err := (&AwsRepository{AutoScalingClient: mock}).DescribeAutoScalingGroup(context.Background(), "api"); err == nil || !strings.Contains(err.Error(), "api") {
		t.Fatalf("expected contextual error, got %v", err)
	}
}

func TestAutoScalingRepositoryErrorPaths(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		mock := &mockAutoScalingBrowserClient{
			describeGroups: func(context.Context, *autoscaling.DescribeAutoScalingGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
				return nil, errors.New("denied")
			},
			describeActivities: unusedAutoScalingActivities,
			setDesired:         unusedSetDesiredCapacity,
		}
		if _, err := (&AwsRepository{AutoScalingClient: mock}).ListAutoScalingGroups(context.Background()); err == nil || !strings.Contains(err.Error(), "describe Auto Scaling groups") {
			t.Fatalf("expected contextual list error, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock := &mockAutoScalingBrowserClient{
			describeGroups:     unusedAutoScalingGroups,
			describeActivities: unusedAutoScalingActivities,
			setDesired:         unusedSetDesiredCapacity,
		}
		if _, _, err := (&AwsRepository{AutoScalingClient: mock}).DescribeAutoScalingGroup(context.Background(), "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected not-found error, got %v", err)
		}
	})

	t.Run("activities", func(t *testing.T) {
		mock := &mockAutoScalingBrowserClient{
			describeGroups: func(context.Context, *autoscaling.DescribeAutoScalingGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
				return &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: []asgtypes.AutoScalingGroup{{AutoScalingGroupName: awssdk.String("api")}}}, nil
			},
			describeActivities: func(context.Context, *autoscaling.DescribeScalingActivitiesInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
				return nil, errors.New("throttled")
			},
			setDesired: unusedSetDesiredCapacity,
		}
		if _, _, err := (&AwsRepository{AutoScalingClient: mock}).DescribeAutoScalingGroup(context.Background(), "api"); err == nil || !strings.Contains(err.Error(), "scaling activity") {
			t.Fatalf("expected contextual activity error, got %v", err)
		}
	})

	t.Run("capacity", func(t *testing.T) {
		mock := &mockAutoScalingBrowserClient{
			describeGroups:     unusedAutoScalingGroups,
			describeActivities: unusedAutoScalingActivities,
			setDesired: func(context.Context, *autoscaling.SetDesiredCapacityInput, ...func(*autoscaling.Options)) (*autoscaling.SetDesiredCapacityOutput, error) {
				return nil, errors.New("denied")
			},
		}
		if err := (&AwsRepository{AutoScalingClient: mock}).SetAutoScalingDesiredCapacity(context.Background(), "api", 3); err == nil || !strings.Contains(err.Error(), "api") {
			t.Fatalf("expected contextual capacity error, got %v", err)
		}
	})
}

func TestSetAutoScalingDesiredCapacityPassesParameters(t *testing.T) {
	var captured *autoscaling.SetDesiredCapacityInput
	mock := &mockAutoScalingBrowserClient{
		describeGroups:     unusedAutoScalingGroups,
		describeActivities: unusedAutoScalingActivities,
		setDesired: func(_ context.Context, input *autoscaling.SetDesiredCapacityInput, _ ...func(*autoscaling.Options)) (*autoscaling.SetDesiredCapacityOutput, error) {
			captured = input
			return &autoscaling.SetDesiredCapacityOutput{}, nil
		},
	}
	if err := (&AwsRepository{AutoScalingClient: mock}).SetAutoScalingDesiredCapacity(context.Background(), "api", 3); err != nil {
		t.Fatal(err)
	}
	if awssdk.ToString(captured.AutoScalingGroupName) != "api" || awssdk.ToInt32(captured.DesiredCapacity) != 3 {
		t.Fatalf("unexpected desired-capacity input: %+v", captured)
	}
}

func unusedAutoScalingGroups(context.Context, *autoscaling.DescribeAutoScalingGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return &autoscaling.DescribeAutoScalingGroupsOutput{}, nil
}

func unusedAutoScalingActivities(context.Context, *autoscaling.DescribeScalingActivitiesInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	return &autoscaling.DescribeScalingActivitiesOutput{}, nil
}

func unusedSetDesiredCapacity(context.Context, *autoscaling.SetDesiredCapacityInput, ...func(*autoscaling.Options)) (*autoscaling.SetDesiredCapacityOutput, error) {
	return &autoscaling.SetDesiredCapacityOutput{}, nil
}
