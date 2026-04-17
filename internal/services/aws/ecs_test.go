package aws

import (
	"context"
	"fmt"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// mockECSClient implements ECSClientAPI for testing.
type mockECSClient struct {
	listClustersFunc     func(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	describeClustersFunc func(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)
	listServicesFunc     func(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	describeServicesFunc func(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	describeTaskDefFunc  func(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
	listTasksFunc        func(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	describeTasksFunc    func(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

func (m *mockECSClient) ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return m.listClustersFunc(ctx, params, optFns...)
}

func (m *mockECSClient) DescribeClusters(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	return m.describeClustersFunc(ctx, params, optFns...)
}

func (m *mockECSClient) ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	return m.listServicesFunc(ctx, params, optFns...)
}

func (m *mockECSClient) DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	return m.describeServicesFunc(ctx, params, optFns...)
}

func (m *mockECSClient) DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	return m.describeTaskDefFunc(ctx, params, optFns...)
}

func (m *mockECSClient) ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	return m.listTasksFunc(ctx, params, optFns...)
}

func (m *mockECSClient) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	return m.describeTasksFunc(ctx, params, optFns...)
}

func TestListClusters_success(t *testing.T) {
	repo := &AwsRepository{
		ECSClient: &mockECSClient{
			listClustersFunc: func(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
				return &ecs.ListClustersOutput{
					ClusterArns: []string{
						"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
						"arn:aws:ecs:us-east-1:123456789012:cluster/staging-cluster",
					},
				}, nil
			},
			describeClustersFunc: func(_ context.Context, params *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
				return &ecs.DescribeClustersOutput{
					Clusters: []ecstypes.Cluster{
						{
							ClusterName:         awssdk.String("prod-cluster"),
							ClusterArn:          awssdk.String("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
							Status:              awssdk.String("ACTIVE"),
							ActiveServicesCount: 5,
							RunningTasksCount:   12,
						},
						{
							ClusterName:         awssdk.String("staging-cluster"),
							ClusterArn:          awssdk.String("arn:aws:ecs:us-east-1:123456789012:cluster/staging-cluster"),
							Status:              awssdk.String("ACTIVE"),
							ActiveServicesCount: 2,
							RunningTasksCount:   3,
						},
					},
				}, nil
			},
		},
	}

	clusters, err := repo.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}
	if clusters[0].Name != "prod-cluster" {
		t.Errorf("expected prod-cluster, got %s", clusters[0].Name)
	}
	if clusters[0].ActiveServices != 5 {
		t.Errorf("expected 5 active services, got %d", clusters[0].ActiveServices)
	}
	if clusters[1].Name != "staging-cluster" {
		t.Errorf("expected staging-cluster, got %s", clusters[1].Name)
	}
}

func TestListClusters_empty(t *testing.T) {
	repo := &AwsRepository{
		ECSClient: &mockECSClient{
			listClustersFunc: func(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
				return &ecs.ListClustersOutput{ClusterArns: nil}, nil
			},
			describeClustersFunc: func(_ context.Context, _ *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
				return &ecs.DescribeClustersOutput{}, nil
			},
		},
	}

	clusters, err := repo.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clusters != nil {
		t.Errorf("expected nil clusters, got %v", clusters)
	}
}

func TestListClusters_error(t *testing.T) {
	repo := &AwsRepository{
		ECSClient: &mockECSClient{
			listClustersFunc: func(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
				return nil, fmt.Errorf("AccessDenied")
			},
			describeClustersFunc: nil,
		},
	}

	_, err := repo.ListClusters(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListServices_success(t *testing.T) {
	repo := &AwsRepository{
		ECSClient: &mockECSClient{
			listServicesFunc: func(_ context.Context, _ *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
				return &ecs.ListServicesOutput{
					ServiceArns: []string{"arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/api-service"},
				}, nil
			},
			describeServicesFunc: func(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
				return &ecs.DescribeServicesOutput{
					Services: []ecstypes.Service{
						{
							ServiceName:  awssdk.String("api-service"),
							ServiceArn:   awssdk.String("arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/api-service"),
							Status:       awssdk.String("ACTIVE"),
							RunningCount: 3,
							DesiredCount: 3,
							PendingCount: 1,
							LaunchType:   ecstypes.LaunchTypeFargate,
						},
					},
				}, nil
			},
		},
	}

	services, err := repo.ListServices(context.Background(), "arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].Name != "api-service" {
		t.Errorf("expected api-service, got %s", services[0].Name)
	}
	if services[0].RunningCount != 3 {
		t.Errorf("expected running count 3, got %d", services[0].RunningCount)
	}
	if services[0].PendingCount != 1 {
		t.Errorf("expected pending count 1, got %d", services[0].PendingCount)
	}
	if services[0].LaunchType != "FARGATE" {
		t.Errorf("expected FARGATE, got %s", services[0].LaunchType)
	}
}

func TestDescribeServiceDetail_success(t *testing.T) {
	repo := &AwsRepository{
		ECSClient: &mockECSClient{
			describeServicesFunc: func(_ context.Context, params *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
				if got := awssdk.ToString(params.Cluster); got != "arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster" {
					t.Fatalf("unexpected cluster ARN: %s", got)
				}
				return &ecs.DescribeServicesOutput{
					Services: []ecstypes.Service{
						{
							ServiceName:          awssdk.String("api-service"),
							ServiceArn:           awssdk.String("arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/api-service"),
							Status:               awssdk.String("ACTIVE"),
							LaunchType:           ecstypes.LaunchTypeFargate,
							SchedulingStrategy:   ecstypes.SchedulingStrategyReplica,
							DesiredCount:         3,
							RunningCount:         2,
							PendingCount:         1,
							EnableExecuteCommand: true,
							PlatformVersion:      awssdk.String("1.4.0"),
							TaskDefinition:       awssdk.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api:42"),
							DeploymentController: &ecstypes.DeploymentController{Type: ecstypes.DeploymentControllerTypeEcs},
							Deployments: []ecstypes.Deployment{
								{
									Id:                 awssdk.String("ecs-svc/123"),
									Status:             awssdk.String("PRIMARY"),
									RolloutState:       ecstypes.DeploymentRolloutStateInProgress,
									RolloutStateReason: awssdk.String("deployment in progress"),
									TaskDefinition:     awssdk.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api:42"),
									DesiredCount:       3,
									RunningCount:       2,
									PendingCount:       1,
									FailedTasks:        2,
								},
							},
							Events: []ecstypes.ServiceEvent{
								{
									Id:      awssdk.String("event-1"),
									Message: awssdk.String("(service api-service) has started 1 tasks: task abc123"),
								},
							},
						},
					},
				}, nil
			},
			describeTaskDefFunc: func(_ context.Context, params *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
				if got := awssdk.ToString(params.TaskDefinition); got != "arn:aws:ecs:us-east-1:123456789012:task-definition/api:42" {
					t.Fatalf("unexpected task definition: %s", got)
				}
				return &ecs.DescribeTaskDefinitionOutput{
					TaskDefinition: &ecstypes.TaskDefinition{
						Family:                  awssdk.String("api"),
						Revision:                42,
						NetworkMode:             ecstypes.NetworkModeAwsvpc,
						RequiresCompatibilities: []ecstypes.Compatibility{ecstypes.CompatibilityFargate},
						ContainerDefinitions: []ecstypes.ContainerDefinition{
							{Name: awssdk.String("app"), Image: awssdk.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/api:2026-04-17")},
							{Name: awssdk.String("nginx"), Image: awssdk.String("nginx:1.27")},
						},
					},
				}, nil
			},
		},
	}

	detail, err := repo.DescribeServiceDetail(context.Background(), "arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster", "arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/api-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Name != "api-service" {
		t.Fatalf("expected api-service, got %s", detail.Name)
	}
	if detail.TaskDefinitionLabel() != "api:42" {
		t.Fatalf("expected task definition label api:42, got %q", detail.TaskDefinitionLabel())
	}
	if detail.CompatibilityLabel() != "FARGATE" {
		t.Fatalf("expected FARGATE compatibility, got %q", detail.CompatibilityLabel())
	}
	if len(detail.Deployments) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(detail.Deployments))
	}
	if detail.Deployments[0].TaskDefinition != "api:42" {
		t.Fatalf("expected short deployment task definition api:42, got %q", detail.Deployments[0].TaskDefinition)
	}
	if detail.Deployments[0].FailedTasks != 2 {
		t.Fatalf("expected failed tasks 2, got %d", detail.Deployments[0].FailedTasks)
	}
	if len(detail.ContainerImages) != 2 {
		t.Fatalf("expected 2 container images, got %d", len(detail.ContainerImages))
	}
	if detail.ContainerImages[0].Name != "app" {
		t.Fatalf("expected app container first after sorting, got %q", detail.ContainerImages[0].Name)
	}
	if len(detail.Events) != 1 || detail.Events[0].ID != "event-1" {
		t.Fatalf("expected 1 service event, got %+v", detail.Events)
	}
}

func TestDescribeServiceDetail_taskDefinitionError(t *testing.T) {
	repo := &AwsRepository{
		ECSClient: &mockECSClient{
			describeServicesFunc: func(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
				return &ecs.DescribeServicesOutput{
					Services: []ecstypes.Service{
						{
							ServiceName:    awssdk.String("api-service"),
							ServiceArn:     awssdk.String("arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/api-service"),
							TaskDefinition: awssdk.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api:42"),
						},
					},
				}, nil
			},
			describeTaskDefFunc: func(_ context.Context, _ *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
				return nil, fmt.Errorf("AccessDenied")
			},
		},
	}

	_, err := repo.DescribeServiceDetail(context.Background(), "cluster", "service")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListTasks_success(t *testing.T) {
	taskARN := "arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/abc123def456"
	repo := &AwsRepository{
		ECSClient: &mockECSClient{
			listTasksFunc: func(_ context.Context, _ *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
				return &ecs.ListTasksOutput{
					TaskArns: []string{taskARN},
				}, nil
			},
			describeTasksFunc: func(_ context.Context, _ *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
				return &ecs.DescribeTasksOutput{
					Tasks: []ecstypes.Task{
						{
							TaskArn:    awssdk.String(taskARN),
							LastStatus: awssdk.String("RUNNING"),
							Group:      awssdk.String("service:api-service"),
						},
					},
				}, nil
			},
		},
	}

	tasks, err := repo.ListTasks(context.Background(), "arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].TaskID != "abc123def456" {
		t.Errorf("expected task ID abc123def456, got %s", tasks[0].TaskID)
	}
	if tasks[0].LastStatus != "RUNNING" {
		t.Errorf("expected RUNNING, got %s", tasks[0].LastStatus)
	}
}
