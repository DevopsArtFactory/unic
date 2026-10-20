package aws

import (
	"context"
	"errors"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

func TestECSServiceTargetHealthAssociationsAndPartialFailure(t *testing.T) {
	calls := []string{}
	repo := &AwsRepository{
		ECSClient: &mockECSClient{describeServicesFunc: func(_ context.Context, in *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
			if awssdk.ToString(in.Cluster) != "cluster" || len(in.Services) != 1 || in.Services[0] != "service" {
				t.Fatalf("wrong ECS scope: %+v", in)
			}
			return &ecs.DescribeServicesOutput{Services: []ecstypes.Service{{ServiceArn: awssdk.String("service"), LoadBalancers: []ecstypes.LoadBalancer{
				{TargetGroupArn: awssdk.String("good")}, {TargetGroupArn: awssdk.String("denied")}, {TargetGroupArn: awssdk.String("good")}, {TargetGroupArn: awssdk.String("empty")}, {LoadBalancerName: awssdk.String("classic")},
			}}}}, nil
		}},
		ELBv2Client: &mockELBv2Client{describeTargetHealthFunc: func(_ context.Context, in *elasticloadbalancingv2.DescribeTargetHealthInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
			arn := awssdk.ToString(in.TargetGroupArn)
			calls = append(calls, arn)
			if arn == "denied" {
				return nil, errors.New("AccessDenied")
			}
			if arn == "empty" {
				return &elasticloadbalancingv2.DescribeTargetHealthOutput{}, nil
			}
			return &elasticloadbalancingv2.DescribeTargetHealthOutput{TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
				{Target: &elbtypes.TargetDescription{Id: awssdk.String("healthy"), Port: awssdk.Int32(80)}, TargetHealth: &elbtypes.TargetHealth{State: elbtypes.TargetHealthStateEnumHealthy}},
				{Target: &elbtypes.TargetDescription{Id: awssdk.String("bad"), Port: awssdk.Int32(8080)}, TargetHealth: &elbtypes.TargetHealth{State: elbtypes.TargetHealthStateEnumUnhealthy, Reason: elbtypes.TargetHealthReasonEnum("Target.Timeout"), Description: awssdk.String("timed out")}},
			}}, nil
		}},
	}
	results, err := repo.ListECSServiceTargetHealth(context.Background(), "cluster", "service")
	if err != nil || len(results) != 4 || strings.Join(calls, ",") != "good,denied,empty" {
		t.Fatalf("unexpected results=%+v err=%v calls=%v", results, err, calls)
	}
	good := results[0].Group
	if good.HealthyCount != 1 || good.UnhealthyCount != 1 || good.Targets[0].ID != "bad" || good.Targets[0].Reason != "Target.Timeout" || good.Targets[0].Description != "timed out" {
		t.Fatalf("lost health evidence: %+v", good)
	}
	if !strings.Contains(results[1].Warning, "AccessDenied") || len(results[2].Group.Targets) != 0 || !strings.Contains(results[3].Warning, "unsupported") {
		t.Fatalf("lost partial/empty/classic states: %+v", results)
	}
}

func TestECSServiceTargetHealthLookupStates(t *testing.T) {
	for _, name := range []string{"none", "missing", "denied", "failure", "cancelled"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			repo := &AwsRepository{ECSClient: &mockECSClient{describeServicesFunc: func(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
				switch name {
				case "missing":
					return &ecs.DescribeServicesOutput{}, nil
				case "denied":
					return nil, errors.New("AccessDenied")
				case "failure":
					return &ecs.DescribeServicesOutput{Failures: []ecstypes.Failure{{Reason: awssdk.String("MISSING")}}}, nil
				case "cancelled":
					cancel()
					return &ecs.DescribeServicesOutput{Services: []ecstypes.Service{{ServiceArn: awssdk.String("service"), LoadBalancers: []ecstypes.LoadBalancer{{TargetGroupArn: awssdk.String("unused")}}}}}, nil
				}
				return &ecs.DescribeServicesOutput{Services: []ecstypes.Service{{ServiceArn: awssdk.String("service")}}}, nil
			}}}
			result, err := repo.ListECSServiceTargetHealth(ctx, "cluster", "service")
			if name == "none" {
				if err != nil || len(result) != 0 {
					t.Fatalf("expected no associations: %+v %v", result, err)
				}
			} else if err == nil {
				t.Fatal("expected explicit error")
			}
		})
	}
}
