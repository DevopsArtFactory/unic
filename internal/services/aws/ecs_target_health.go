package aws

import (
	"context"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// ECSServiceTargetHealth retains per-association failures alongside successful evidence.
type ECSServiceTargetHealth struct {
	Group   ELBTargetGroupHealth
	Warning string
}

// ListECSServiceTargetHealth queries only target groups explicitly attached to the service.
// Cancellation can return partial evidence with an error; callers must display both.
func (r *AwsRepository) ListECSServiceTargetHealth(ctx context.Context, clusterARN, serviceARN string) ([]ECSServiceTargetHealth, error) {
	out, err := r.ECSClient.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster: awssdk.String(clusterARN), Services: []string{serviceARN},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ECS service: %w", err)
	}
	if len(out.Failures) > 0 {
		return nil, fmt.Errorf("ECS service lookup failed: %s %s", awssdk.ToString(out.Failures[0].Reason), awssdk.ToString(out.Failures[0].Detail))
	}
	if len(out.Services) != 1 || awssdk.ToString(out.Services[0].ServiceArn) != serviceARN {
		return nil, fmt.Errorf("ECS service not found: %s", serviceARN)
	}
	var results []ECSServiceTargetHealth
	seen := make(map[string]bool)
	for _, association := range out.Services[0].LoadBalancers {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		arn := awssdk.ToString(association.TargetGroupArn)
		if arn == "" {
			results = append(results, ECSServiceTargetHealth{Warning: "Classic load balancer association has no ELBv2 target group; health lookup unsupported"})
			continue
		}
		if seen[arn] {
			continue
		}
		seen[arn] = true
		name := arn
		parts := strings.Split(arn, "/")
		if len(parts) == 3 && strings.HasSuffix(parts[0], ":targetgroup") && parts[1] != "" {
			name = parts[1]
		}
		item := ECSServiceTargetHealth{Group: ELBTargetGroupHealth{ARN: arn, Name: name}}
		if err := r.loadTargetHealth(ctx, &item.Group); err != nil {
			item.Warning = err.Error()
		}
		results = append(results, item)
		if err := ctx.Err(); err != nil {
			return results, err
		}
	}
	return results, nil
}
