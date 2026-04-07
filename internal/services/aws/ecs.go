package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	uniclog "unic/internal/log"
)

// ListClusters returns all ECS clusters in the current account/region.
func (r *AwsRepository) ListClusters(ctx context.Context) ([]ECSCluster, error) {
	uniclog.Debug("aws", "ListClusters called")

	var arns []string
	var nextToken *string
	for {
		out, err := r.ECSClient.ListClusters(ctx, &ecs.ListClustersInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("failed to list ECS clusters: %w", err)
		}
		arns = append(arns, out.ClusterArns...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	if len(arns) == 0 {
		return nil, nil
	}

	out, err := r.ECSClient.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: arns,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ECS clusters: %w", err)
	}

	clusters := make([]ECSCluster, 0, len(out.Clusters))
	for _, c := range out.Clusters {
		clusters = append(clusters, ECSCluster{
			Name:           awssdk.ToString(c.ClusterName),
			ARN:            awssdk.ToString(c.ClusterArn),
			Status:         awssdk.ToString(c.Status),
			ActiveServices: c.ActiveServicesCount,
			RunningTasks:   c.RunningTasksCount,
		})
	}
	return clusters, nil
}

// ListServices returns all ECS services in the given cluster.
func (r *AwsRepository) ListServices(ctx context.Context, clusterARN string) ([]ECSService, error) {
	uniclog.Debug("aws", "ListServices called", "cluster", clusterARN)

	var arns []string
	var nextToken *string
	for {
		out, err := r.ECSClient.ListServices(ctx, &ecs.ListServicesInput{
			Cluster:   awssdk.String(clusterARN),
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list ECS services: %w", err)
		}
		arns = append(arns, out.ServiceArns...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	if len(arns) == 0 {
		return nil, nil
	}

	// DescribeServices accepts max 10 at a time
	var services []ECSService
	for i := 0; i < len(arns); i += 10 {
		end := i + 10
		if end > len(arns) {
			end = len(arns)
		}
		out, err := r.ECSClient.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  awssdk.String(clusterARN),
			Services: arns[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe ECS services: %w", err)
		}
		for _, s := range out.Services {
			svc := ECSService{
				Name:         awssdk.ToString(s.ServiceName),
				ARN:          awssdk.ToString(s.ServiceArn),
				Status:       awssdk.ToString(s.Status),
				RunningCount: s.RunningCount,
				DesiredCount: s.DesiredCount,
				LaunchType:   string(s.LaunchType),
			}
			services = append(services, svc)
		}
	}
	return services, nil
}

// ListTasks returns running tasks in the given cluster and service.
func (r *AwsRepository) ListTasks(ctx context.Context, clusterARN, serviceARN string) ([]ECSTask, error) {
	uniclog.Debug("aws", "ListTasks called", "cluster", clusterARN, "service", serviceARN)

	var taskARNs []string
	var nextToken *string
	for {
		input := &ecs.ListTasksInput{
			Cluster:   awssdk.String(clusterARN),
			NextToken: nextToken,
		}
		if serviceARN != "" {
			// Extract service name from ARN for the ServiceName filter
			parts := strings.Split(serviceARN, "/")
			input.ServiceName = awssdk.String(parts[len(parts)-1])
		}
		out, err := r.ECSClient.ListTasks(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list ECS tasks: %w", err)
		}
		taskARNs = append(taskARNs, out.TaskArns...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	if len(taskARNs) == 0 {
		return nil, nil
	}

	return r.describeTasksFromARNs(ctx, clusterARN, taskARNs)
}

// DescribeTaskContainers returns the containers for a specific task.
func (r *AwsRepository) DescribeTaskContainers(ctx context.Context, clusterARN, taskARN string) ([]ECSContainer, error) {
	uniclog.Debug("aws", "DescribeTaskContainers called", "task", taskARN)

	out, err := r.ECSClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: awssdk.String(clusterARN),
		Tasks:   []string{taskARN},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ECS task: %w", err)
	}

	if len(out.Tasks) == 0 {
		return nil, fmt.Errorf("task not found: %s", taskARN)
	}

	task := out.Tasks[0]
	containers := make([]ECSContainer, 0, len(task.Containers))
	for _, c := range task.Containers {
		containers = append(containers, ECSContainer{
			Name:        awssdk.ToString(c.Name),
			RuntimeID:   awssdk.ToString(c.RuntimeId),
			ExecEnabled: task.EnableExecuteCommand,
		})
	}
	return containers, nil
}

// describeTasksFromARNs fetches task details for a slice of ARNs (max 100 per call).
func (r *AwsRepository) describeTasksFromARNs(ctx context.Context, clusterARN string, arns []string) ([]ECSTask, error) {
	var tasks []ECSTask
	for i := 0; i < len(arns); i += 100 {
		end := i + 100
		if end > len(arns) {
			end = len(arns)
		}
		out, err := r.ECSClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: awssdk.String(clusterARN),
			Tasks:   arns[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe ECS tasks: %w", err)
		}
		for _, t := range out.Tasks {
			taskARN := awssdk.ToString(t.TaskArn)
			taskID := taskARN
			if idx := strings.LastIndex(taskARN, "/"); idx >= 0 {
				taskID = taskARN[idx+1:]
			}
			var startedAt time.Time
			if t.StartedAt != nil {
				startedAt = *t.StartedAt
			}
			tasks = append(tasks, ECSTask{
				TaskARN:    taskARN,
				TaskID:     taskID,
				LastStatus: awssdk.ToString(t.LastStatus),
				Group:      awssdk.ToString(t.Group),
				StartedAt:  startedAt,
			})
		}
	}
	return tasks, nil
}
