package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func TestECSServiceListEnterLoadsServiceDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenECSServiceList
	m.selectedECSCluster = &awsservice.ECSCluster{Name: "prod-cluster", ARN: "arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"}
	m.ecsServices = []awsservice.ECSService{
		{Name: "api-service", ARN: "arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/api-service", Status: "ACTIVE", RunningCount: 2, DesiredCount: 3, PendingCount: 1, LaunchType: "FARGATE"},
	}
	m.filteredECSServices = m.ecsServices

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if cmd == nil {
		t.Fatal("expected load command for ECS service detail")
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %v", model.screen)
	}
	if model.selectedECSService == nil || model.selectedECSService.Name != "api-service" {
		t.Fatalf("expected selected ECS service api-service, got %+v", model.selectedECSService)
	}
}

func TestHandleECSServiceDetailLoadedMsgShowsDetailScreen(t *testing.T) {
	m := New(testConfig(), "", "dev")
	detail := &awsservice.ECSServiceDetail{
		Name:                   "api-service",
		ARN:                    "arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/api-service",
		Status:                 "ACTIVE",
		LaunchType:             "FARGATE",
		DesiredCount:           3,
		RunningCount:           2,
		PendingCount:           1,
		TaskDefinitionFamily:   "api",
		TaskDefinitionRevision: 42,
	}

	updated, _, handled := m.handleECSMsg(ecsServiceDetailLoadedMsg{detail: detail})
	if !handled {
		t.Fatal("expected detail message to be handled")
	}

	model := updated.(Model)
	if model.screen != screenECSServiceDetail {
		t.Fatalf("expected ECS service detail screen, got %v", model.screen)
	}
	if model.selectedECSDetail == nil || model.selectedECSDetail.Name != "api-service" {
		t.Fatalf("expected selected ECS detail api-service, got %+v", model.selectedECSDetail)
	}
	if model.selectedECSService == nil || model.selectedECSService.PendingCount != 1 {
		t.Fatalf("expected service summary sync, got %+v", model.selectedECSService)
	}
}

func TestECSServiceDetailViewShowsRolloutAndImages(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 120
	m.height = 36
	m.screen = screenECSServiceDetail
	m.selectedECSDetail = &awsservice.ECSServiceDetail{
		Name:                     "api-service",
		Status:                   "ACTIVE",
		LaunchType:               "FARGATE",
		SchedulingStrategy:       "REPLICA",
		DeploymentControllerType: "ECS",
		DesiredCount:             3,
		RunningCount:             2,
		PendingCount:             1,
		EnableExecuteCommand:     true,
		PlatformVersion:          "1.4.0",
		TaskDefinitionFamily:     "api",
		TaskDefinitionRevision:   42,
		NetworkMode:              "awsvpc",
		RequiresCompatibilities:  []string{"FARGATE"},
		Deployments: []awsservice.ECSDeployment{
			{
				Status:             "PRIMARY",
				RolloutState:       "IN_PROGRESS",
				RolloutStateReason: "deployment in progress",
				TaskDefinition:     "api:42",
				RunningCount:       2,
				DesiredCount:       3,
				PendingCount:       1,
				FailedTasks:        2,
				UpdatedAt:          time.Date(2026, 4, 17, 12, 30, 0, 0, time.UTC),
			},
		},
		ContainerImages: []awsservice.ECSContainerImage{
			{Name: "app", Image: "123456789012.dkr.ecr.us-east-1.amazonaws.com/api:2026-04-17"},
			{Name: "nginx", Image: "nginx:1.27"},
		},
		Events: []awsservice.ECSServiceEvent{
			{
				CreatedAt: time.Date(2026, 4, 17, 12, 29, 0, 0, time.UTC),
				Message:   "(service api-service) has started 1 tasks: task abc123",
			},
		},
	}

	view := stripANSI(m.viewECSServiceDetail())
	for _, want := range []string{
		"ECS Service Rollout",
		"running:2 desired:3 pending:1",
		"api:42",
		"deployment in progress",
		"nginx:1.27",
		"Recent Service Events",
		"has started 1 tasks",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}

func TestECSServiceDetailEscReturnsToServiceList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenECSServiceDetail
	m.selectedECSDetail = &awsservice.ECSServiceDetail{Name: "api-service"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenECSServiceList {
		t.Fatalf("expected esc to return to service list, got %v", model.screen)
	}
}

func TestECSTaskListEscReturnsToServiceDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenECSTaskList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenECSServiceDetail {
		t.Fatalf("expected esc to return to service detail, got %v", model.screen)
	}
}
