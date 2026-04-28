package aws

import (
	"context"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

type mockAutoScalingClient struct {
	describeInstancesFunc func(ctx context.Context, params *autoscaling.DescribeAutoScalingInstancesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingInstancesOutput, error)
	describeGroupsFunc    func(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
}

func (m *mockAutoScalingClient) DescribeAutoScalingInstances(ctx context.Context, params *autoscaling.DescribeAutoScalingInstancesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingInstancesOutput, error) {
	if m.describeInstancesFunc != nil {
		return m.describeInstancesFunc(ctx, params, optFns...)
	}
	return &autoscaling.DescribeAutoScalingInstancesOutput{}, nil
}

func (m *mockAutoScalingClient) DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	if m.describeGroupsFunc != nil {
		return m.describeGroupsFunc(ctx, params, optFns...)
	}
	return &autoscaling.DescribeAutoScalingGroupsOutput{}, nil
}

type mockELBv2Client struct {
	describeTargetGroupsFunc  func(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error)
	describeTargetHealthFunc  func(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error)
	describeLoadBalancersFunc func(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error)
	describeListenersFunc     func(ctx context.Context, params *elasticloadbalancingv2.DescribeListenersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error)
	describeRulesFunc         func(ctx context.Context, params *elasticloadbalancingv2.DescribeRulesInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeRulesOutput, error)
}

func (m *mockELBv2Client) DescribeTargetGroups(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error) {
	return m.describeTargetGroupsFunc(ctx, params, optFns...)
}

func (m *mockELBv2Client) DescribeTargetHealth(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
	return m.describeTargetHealthFunc(ctx, params, optFns...)
}

func (m *mockELBv2Client) DescribeLoadBalancers(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
	return m.describeLoadBalancersFunc(ctx, params, optFns...)
}

func (m *mockELBv2Client) DescribeListeners(ctx context.Context, params *elasticloadbalancingv2.DescribeListenersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error) {
	return m.describeListenersFunc(ctx, params, optFns...)
}

func (m *mockELBv2Client) DescribeRules(ctx context.Context, params *elasticloadbalancingv2.DescribeRulesInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeRulesOutput, error) {
	return m.describeRulesFunc(ctx, params, optFns...)
}

func TestDescribeEC2InstanceRelationshipsMapsMainPaths(t *testing.T) {
	instance := EC2Instance{
		InstanceID: "i-123",
		SecurityGroups: []EC2InstanceSecurityGroup{
			{GroupID: "sg-web", Name: "web"},
		},
	}
	ec2Mock := &mockEC2Client{
		describeSecurityGroupsFunc: func(_ context.Context, params *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			if len(params.GroupIds) != 1 || params.GroupIds[0] != "sg-web" {
				t.Fatalf("unexpected security group IDs: %+v", params.GroupIds)
			}
			return &ec2.DescribeSecurityGroupsOutput{
				SecurityGroups: []ec2types.SecurityGroup{
					{
						GroupId:     awssdk.String("sg-web"),
						GroupName:   awssdk.String("web"),
						Description: awssdk.String("web tier"),
						VpcId:       awssdk.String("vpc-1"),
					},
				},
			}, nil
		},
	}
	asgMock := &mockAutoScalingClient{
		describeInstancesFunc: func(_ context.Context, params *autoscaling.DescribeAutoScalingInstancesInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingInstancesOutput, error) {
			if len(params.InstanceIds) != 1 || params.InstanceIds[0] != "i-123" {
				t.Fatalf("unexpected ASG instance IDs: %+v", params.InstanceIds)
			}
			return &autoscaling.DescribeAutoScalingInstancesOutput{
				AutoScalingInstances: []asgtypes.AutoScalingInstanceDetails{
					{
						InstanceId:           awssdk.String("i-123"),
						AutoScalingGroupName: awssdk.String("app-asg"),
						LifecycleState:       awssdk.String("InService"),
						HealthStatus:         awssdk.String("Healthy"),
						AvailabilityZone:     awssdk.String("us-east-1a"),
						ProtectedFromScaleIn: awssdk.Bool(false),
					},
				},
			}, nil
		},
		describeGroupsFunc: func(_ context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			if len(params.AutoScalingGroupNames) != 1 || params.AutoScalingGroupNames[0] != "app-asg" {
				t.Fatalf("unexpected ASG names: %+v", params.AutoScalingGroupNames)
			}
			return &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []asgtypes.AutoScalingGroup{
					{
						AutoScalingGroupName: awssdk.String("app-asg"),
						AutoScalingGroupARN:  awssdk.String("arn:asg"),
						DesiredCapacity:      awssdk.Int32(2),
						MinSize:              awssdk.Int32(1),
						MaxSize:              awssdk.Int32(4),
						TargetGroupARNs:      []string{"arn:tg"},
					},
				},
			}, nil
		},
	}
	elbMock := &mockELBv2Client{
		describeTargetGroupsFunc: func(_ context.Context, _ *elasticloadbalancingv2.DescribeTargetGroupsInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error) {
			return &elasticloadbalancingv2.DescribeTargetGroupsOutput{
				TargetGroups: []elbtypes.TargetGroup{
					{
						TargetGroupArn:   awssdk.String("arn:tg"),
						TargetGroupName:  awssdk.String("app-tg"),
						Protocol:         elbtypes.ProtocolEnumHttp,
						Port:             awssdk.Int32(80),
						TargetType:       elbtypes.TargetTypeEnumInstance,
						VpcId:            awssdk.String("vpc-1"),
						LoadBalancerArns: []string{"arn:lb"},
					},
				},
			}, nil
		},
		describeTargetHealthFunc: func(_ context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
			if awssdk.ToString(params.TargetGroupArn) != "arn:tg" {
				t.Fatalf("unexpected target group ARN: %s", awssdk.ToString(params.TargetGroupArn))
			}
			return &elasticloadbalancingv2.DescribeTargetHealthOutput{
				TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
					{
						Target:       &elbtypes.TargetDescription{Id: awssdk.String("i-123")},
						TargetHealth: &elbtypes.TargetHealth{State: elbtypes.TargetHealthStateEnumHealthy},
					},
				},
			}, nil
		},
		describeLoadBalancersFunc: func(_ context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
			if len(params.LoadBalancerArns) != 1 || params.LoadBalancerArns[0] != "arn:lb" {
				t.Fatalf("unexpected load balancer ARNs: %+v", params.LoadBalancerArns)
			}
			return &elasticloadbalancingv2.DescribeLoadBalancersOutput{
				LoadBalancers: []elbtypes.LoadBalancer{
					{
						LoadBalancerArn:  awssdk.String("arn:lb"),
						LoadBalancerName: awssdk.String("app-lb"),
						DNSName:          awssdk.String("app.example.com"),
						Type:             elbtypes.LoadBalancerTypeEnumApplication,
						Scheme:           elbtypes.LoadBalancerSchemeEnumInternetFacing,
						State:            &elbtypes.LoadBalancerState{Code: elbtypes.LoadBalancerStateEnumActive},
						VpcId:            awssdk.String("vpc-1"),
					},
				},
			}, nil
		},
		describeListenersFunc: func(_ context.Context, params *elasticloadbalancingv2.DescribeListenersInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error) {
			if awssdk.ToString(params.LoadBalancerArn) != "arn:lb" {
				t.Fatalf("unexpected listener LB ARN: %s", awssdk.ToString(params.LoadBalancerArn))
			}
			return &elasticloadbalancingv2.DescribeListenersOutput{
				Listeners: []elbtypes.Listener{
					{
						ListenerArn:     awssdk.String("arn:listener"),
						LoadBalancerArn: awssdk.String("arn:lb"),
						Protocol:        elbtypes.ProtocolEnumHttp,
						Port:            awssdk.Int32(80),
						DefaultActions:  []elbtypes.Action{{Type: elbtypes.ActionTypeEnumForward}},
					},
				},
			}, nil
		},
		describeRulesFunc: func(_ context.Context, params *elasticloadbalancingv2.DescribeRulesInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeRulesOutput, error) {
			if awssdk.ToString(params.ListenerArn) != "arn:listener" {
				t.Fatalf("unexpected listener ARN: %s", awssdk.ToString(params.ListenerArn))
			}
			return &elasticloadbalancingv2.DescribeRulesOutput{
				Rules: []elbtypes.Rule{
					{Priority: awssdk.String("default")},
					{Priority: awssdk.String("10")},
				},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: ec2Mock, AutoScalingClient: asgMock, ELBv2Client: elbMock}
	relationships, err := repo.DescribeEC2InstanceRelationships(context.Background(), instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(relationships.Errors) != 0 {
		t.Fatalf("expected no relationship errors, got %+v", relationships.Errors)
	}
	if len(relationships.SecurityGroups) != 1 || relationships.SecurityGroups[0].GroupID != "sg-web" {
		t.Fatalf("unexpected security groups: %+v", relationships.SecurityGroups)
	}
	if relationships.AutoScaling == nil || relationships.AutoScaling.Name != "app-asg" || relationships.AutoScaling.DesiredCapacity != 2 {
		t.Fatalf("unexpected auto scaling group: %+v", relationships.AutoScaling)
	}
	if len(relationships.TargetGroups) != 1 || relationships.TargetGroups[0].HealthState != "healthy" {
		t.Fatalf("unexpected target groups: %+v", relationships.TargetGroups)
	}
	if len(relationships.LoadBalancers) != 1 || relationships.LoadBalancers[0].Name != "app-lb" {
		t.Fatalf("unexpected load balancers: %+v", relationships.LoadBalancers)
	}
	if len(relationships.Listeners) != 1 || relationships.Listeners[0].RuleCount != 2 {
		t.Fatalf("unexpected listeners: %+v", relationships.Listeners)
	}
}
