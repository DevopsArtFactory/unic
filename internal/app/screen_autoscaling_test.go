package app

import (
	"context"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

type mockAutoScalingAppClient struct {
	describeGroups     func(context.Context, *autoscaling.DescribeAutoScalingGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
	describeActivities func(context.Context, *autoscaling.DescribeScalingActivitiesInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error)
	setDesired         func(context.Context, *autoscaling.SetDesiredCapacityInput, ...func(*autoscaling.Options)) (*autoscaling.SetDesiredCapacityOutput, error)
}

func (m *mockAutoScalingAppClient) DescribeAutoScalingInstances(context.Context, *autoscaling.DescribeAutoScalingInstancesInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingInstancesOutput, error) {
	return &autoscaling.DescribeAutoScalingInstancesOutput{}, nil
}

func (m *mockAutoScalingAppClient) DescribeAutoScalingGroups(ctx context.Context, input *autoscaling.DescribeAutoScalingGroupsInput, opts ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return m.describeGroups(ctx, input, opts...)
}

func (m *mockAutoScalingAppClient) DescribeScalingActivities(ctx context.Context, input *autoscaling.DescribeScalingActivitiesInput, opts ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	return m.describeActivities(ctx, input, opts...)
}

func (m *mockAutoScalingAppClient) SetDesiredCapacity(ctx context.Context, input *autoscaling.SetDesiredCapacityInput, opts ...func(*autoscaling.Options)) (*autoscaling.SetDesiredCapacityOutput, error) {
	return m.setDesired(ctx, input, opts...)
}

func autoScalingTestGroups() []awsservice.AutoScalingGroup {
	return []awsservice.AutoScalingGroup{
		{Name: "api-prod", ARN: "arn:api", DesiredCapacity: 2, MinSize: 1, MaxSize: 4, HealthCheckType: "ELB", Instances: []awsservice.AutoScalingInstance{{ID: "i-1", HealthStatus: "Healthy"}, {ID: "i-2", HealthStatus: "Unhealthy"}}},
		{Name: "worker-dev", DesiredCapacity: 1, MinSize: 0, MaxSize: 2},
	}
}

func TestAutoScalingGroupsLoadedRendersAndFilters(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading
	m.loadingReturnScreen = screenAutoScalingGroupList
	m.autoScaling.HandleMessage(&m, autoScalingGroupsLoadedMsg{groups: autoScalingTestGroups()})

	if m.screen != screenAutoScalingGroupList {
		t.Fatalf("expected group list, got %v", m.screen)
	}
	view, ok := m.autoScaling.View(m)
	for _, want := range []string{"GROUP", "DESIRED", "INSTANCES", "HEALTHY", "api-prod", "1/2"} {
		if !ok || !strings.Contains(view, want) {
			t.Fatalf("expected group table containing %q, got:\n%s", want, view)
		}
	}

	m.storeFilterValue(filterAutoScalingGroups, "i-1")
	m.applyFilterTarget(filterAutoScalingGroups)
	if len(m.autoScaling.filtered) != 1 || m.autoScaling.filtered[0].Name != "api-prod" {
		t.Fatalf("expected instance metadata filter match, got %+v", m.autoScaling.filtered)
	}
}

func TestAutoScalingDetailLoadExecutesRepositoryCommand(t *testing.T) {
	started := time.Date(2026, 8, 20, 2, 3, 4, 0, time.UTC)
	mock := &mockAutoScalingAppClient{
		describeGroups: func(_ context.Context, input *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			if len(input.AutoScalingGroupNames) != 1 || input.AutoScalingGroupNames[0] != "api-prod" {
				t.Fatalf("unexpected group request: %+v", input)
			}
			return &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: []asgtypes.AutoScalingGroup{{
				AutoScalingGroupName: awssdk.String("api-prod"), DesiredCapacity: awssdk.Int32(2), MinSize: awssdk.Int32(1), MaxSize: awssdk.Int32(4), HealthCheckType: awssdk.String("ELB"),
				Instances: []asgtypes.Instance{{InstanceId: awssdk.String("i-1"), AvailabilityZone: awssdk.String("us-east-1a"), LifecycleState: asgtypes.LifecycleStateInService, HealthStatus: awssdk.String("Healthy")}},
			}}}, nil
		},
		describeActivities: func(context.Context, *autoscaling.DescribeScalingActivitiesInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
			return &autoscaling.DescribeScalingActivitiesOutput{Activities: []asgtypes.Activity{{
				ActivityId: awssdk.String("a-1"), AutoScalingGroupName: awssdk.String("api-prod"), Cause: awssdk.String("manual change"), StartTime: &started,
				StatusCode: asgtypes.ScalingActivityStatusCodeFailed, Description: awssdk.String("Launching instance"), StatusMessage: awssdk.String("launch template invalid"),
			}}}, nil
		},
		setDesired: func(context.Context, *autoscaling.SetDesiredCapacityInput, ...func(*autoscaling.Options)) (*autoscaling.SetDesiredCapacityOutput, error) {
			return &autoscaling.SetDesiredCapacityOutput{}, nil
		},
	}
	m := New(testConfig(), "", "dev")
	m.height = 40
	m.awsRepo = &awsservice.AwsRepository{AutoScalingClient: mock}
	m.screen = screenLoading
	m.loadingReturnScreen = screenAutoScalingGroupList
	m.autoScaling.HandleMessage(&m, autoScalingGroupsLoadedMsg{groups: autoScalingTestGroups()})

	updated, cmd := m.autoScaling.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	result := runBatchedUserCmd(t, cmd)
	updated, _ = m.Update(result)
	m = updated.(Model)

	if m.screen != screenAutoScalingGroupDetail || m.autoScaling.selected == nil || len(m.autoScaling.activities) != 1 {
		t.Fatalf("expected loaded group detail, screen=%v model=%+v", m.screen, m.autoScaling)
	}
	view, _ := m.autoScaling.View(m)
	for _, want := range []string{"i-1", "Recent Scaling Activity", "Failed", "launch template invalid"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected detail containing %q, got:\n%s", want, view)
		}
	}
}

func TestAutoScalingCapacityRequiresRangeAndTypedConfirmation(t *testing.T) {
	var captured *autoscaling.SetDesiredCapacityInput
	mock := &mockAutoScalingAppClient{
		describeGroups: func(context.Context, *autoscaling.DescribeAutoScalingGroupsInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			return &autoscaling.DescribeAutoScalingGroupsOutput{}, nil
		},
		describeActivities: func(context.Context, *autoscaling.DescribeScalingActivitiesInput, ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
			return &autoscaling.DescribeScalingActivitiesOutput{}, nil
		},
		setDesired: func(_ context.Context, input *autoscaling.SetDesiredCapacityInput, _ ...func(*autoscaling.Options)) (*autoscaling.SetDesiredCapacityOutput, error) {
			captured = input
			return &autoscaling.SetDesiredCapacityOutput{}, nil
		},
	}
	m := New(testConfig(), "", "dev")
	m.awsRepo = &awsservice.AwsRepository{AutoScalingClient: mock}
	m.autoScaling.groups = autoScalingTestGroups()
	m.autoScaling.filtered = m.autoScaling.groups
	selected := m.autoScaling.groups[0]
	m.autoScaling.selected = &selected
	m.screen = screenAutoScalingGroupDetail

	m.autoScaling.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.screen != screenAutoScalingCapacityInput || !m.isTextEntryScreen() {
		t.Fatalf("expected text-entry capacity screen, got %v", m.screen)
	}
	m.autoScaling.capacityInput = "5"
	m.autoScaling.updateCapacityInput(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenAutoScalingCapacityInput || !strings.Contains(m.autoScaling.capacityError, "between 1 and 4") {
		t.Fatalf("expected range validation, screen=%v error=%q", m.screen, m.autoScaling.capacityError)
	}
	m.autoScaling.capacityInput = "3"
	m.autoScaling.updateCapacityInput(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenAutoScalingConfirm || !m.isTextEntryScreen() {
		t.Fatalf("expected typed confirmation screen, got %v", m.screen)
	}
	m.autoScaling.confirmInput = "wrong"
	_, cmd := m.autoScaling.updateConfirm(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || m.screen != screenAutoScalingConfirm {
		t.Fatal("wrong group name must not run the capacity change")
	}
	m.autoScaling.confirmInput = "api-prod"
	updated, cmd := m.autoScaling.updateConfirm(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	result := runBatchedUserCmd(t, cmd)
	updated, _ = m.Update(result)
	m = updated.(Model)

	if captured == nil || awssdk.ToString(captured.AutoScalingGroupName) != "api-prod" || awssdk.ToInt32(captured.DesiredCapacity) != 3 {
		t.Fatalf("unexpected capacity request: %+v", captured)
	}
	if m.screen != screenAutoScalingGroupDetail || m.autoScaling.selected.DesiredCapacity != 3 || !strings.Contains(m.autoScaling.notice, "3") {
		t.Fatalf("expected updated detail notice, screen=%v model=%+v", m.screen, m.autoScaling)
	}
}

func TestAutoScalingLoadCompletesBehindSettings(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 40
	selected := autoScalingTestGroups()[0]
	m.autoScaling.selected = &selected
	m.loadingReturnScreen = screenAutoScalingGroupDetail
	m.settingsPrevScreen = screenLoading
	m.screen = screenSettings

	m.autoScaling.HandleMessage(&m, autoScalingDetailLoadedMsg{groupName: selected.Name, group: &selected})
	if m.screen != screenSettings || m.settingsPrevScreen != screenAutoScalingGroupDetail {
		t.Fatalf("expected Settings to stay open over completed detail, screen=%v previous=%v", m.screen, m.settingsPrevScreen)
	}
}

func TestAutoScalingLoadBreaksOverlayCycle(t *testing.T) {
	m := New(testConfig(), "", "dev")
	selected := autoScalingTestGroups()[0]
	m.autoScaling.selected = &selected
	m.loadingReturnScreen = screenAutoScalingGroupDetail
	m.screen = screenSettings
	m.settingsPrevScreen = screenViewList
	m.views.prevScreen = screenSettings

	m.autoScaling.HandleMessage(&m, autoScalingDetailLoadedMsg{groupName: selected.Name, group: &selected})
	if m.screen != screenSettings || m.views.prevScreen != screenAutoScalingGroupDetail {
		t.Fatalf("expected the overlay cycle to terminate at group detail, screen=%v settingsPrev=%v viewsPrev=%v", m.screen, m.settingsPrevScreen, m.views.prevScreen)
	}
}

func TestAutoScalingDetailEscapesFailureControls(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 40
	selected := autoScalingTestGroups()[0]
	m.autoScaling.selected = &selected
	m.autoScaling.activities = []awsservice.AutoScalingActivity{{Status: "Failed", StatusMessage: "bad\x1b[31mvalue\a"}}
	m.screen = screenAutoScalingGroupDetail
	view, _ := m.autoScaling.View(m)
	if strings.Contains(view, "\x1b[31mvalue\a") || !strings.Contains(view, `bad\x1b[31mvalue\a`) {
		t.Fatalf("expected escaped activity failure, got %q", view)
	}
}

func TestAutoScalingSavedViewAndHelpTitles(t *testing.T) {
	if target, ok := featurePrimaryFilter["Auto Scaling Group Browser"]; !ok || target != filterAutoScalingGroups {
		t.Fatalf("expected Auto Scaling saved-view filter, got %v %v", target, ok)
	}
	m := New(testConfig(), "", "dev")
	for screen, want := range map[screen]string{
		screenAutoScalingGroupList:     "Auto Scaling Groups",
		screenAutoScalingGroupDetail:   "Auto Scaling Group Detail",
		screenAutoScalingCapacityInput: "Auto Scaling Desired Capacity",
		screenAutoScalingConfirm:       "Auto Scaling Capacity Confirmation",
	} {
		m.screen = screen
		if got := m.helpScreenTitle(); got != want {
			t.Fatalf("screen %v: expected %q, got %q", screen, want, got)
		}
	}
}
