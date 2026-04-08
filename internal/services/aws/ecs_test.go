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
	listClustersFunc    func(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	describeClustersFunc func(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)
	listServicesFunc    func(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	describeServicesFunc func(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	listTasksFunc       func(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	describeTasksFunc   func(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
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
	if services[0].LaunchType != "FARGATE" {
		t.Errorf("expected FARGATE, got %s", services[0].LaunchType)
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
