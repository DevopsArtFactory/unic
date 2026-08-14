package aws

import (
	"context"
	"errors"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

func TestListLoadBalancersMapsAndSorts(t *testing.T) {
	pages := 0
	mock := &mockELBv2Client{
		describeLoadBalancersFunc: func(_ context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
			pages++
			if pages == 1 {
				if params.Marker != nil {
					t.Fatalf("first page should have no marker, got %v", params.Marker)
				}
				return &elasticloadbalancingv2.DescribeLoadBalancersOutput{
					LoadBalancers: []elbtypes.LoadBalancer{
						{
							LoadBalancerName: awssdk.String("zebra-alb"),
							LoadBalancerArn:  awssdk.String("arn:lb/zebra"),
							DNSName:          awssdk.String("zebra.elb.amazonaws.com"),
							Type:             elbtypes.LoadBalancerTypeEnumApplication,
							Scheme:           elbtypes.LoadBalancerSchemeEnumInternetFacing,
							State:            &elbtypes.LoadBalancerState{Code: elbtypes.LoadBalancerStateEnumActive},
							VpcId:            awssdk.String("vpc-1"),
						},
					},
					NextMarker: awssdk.String("page2"),
				}, nil
			}
			if awssdk.ToString(params.Marker) != "page2" {
				t.Fatalf("second page should carry marker, got %v", params.Marker)
			}
			return &elasticloadbalancingv2.DescribeLoadBalancersOutput{
				LoadBalancers: []elbtypes.LoadBalancer{
					{
						LoadBalancerName: awssdk.String("api-nlb"),
						LoadBalancerArn:  awssdk.String("arn:lb/api"),
						Type:             elbtypes.LoadBalancerTypeEnumNetwork,
						Scheme:           elbtypes.LoadBalancerSchemeEnumInternal,
					},
				},
			}, nil
		},
	}
	repo := &AwsRepository{ELBv2Client: mock, Region: "us-east-1"}

	balancers, err := repo.ListLoadBalancers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(balancers) != 2 {
		t.Fatalf("expected 2 load balancers across pages, got %d", len(balancers))
	}
	if balancers[0].Name != "api-nlb" || balancers[1].Name != "zebra-alb" {
		t.Fatalf("expected name-sorted order, got %s, %s", balancers[0].Name, balancers[1].Name)
	}
	first := balancers[1]
	if first.Type != "application" || first.Scheme != "internet-facing" || first.State != "active" {
		t.Fatalf("unexpected mapping: %+v", first)
	}
	if first.Region != "us-east-1" {
		t.Fatalf("expected region stamped on rows, got %q", first.Region)
	}
}

func TestListLoadBalancersWrapsError(t *testing.T) {
	mock := &mockELBv2Client{
		describeLoadBalancersFunc: func(_ context.Context, _ *elasticloadbalancingv2.DescribeLoadBalancersInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
			return nil, errors.New("denied")
		},
	}
	repo := &AwsRepository{ELBv2Client: mock}
	if _, err := repo.ListLoadBalancers(context.Background()); err == nil || !strings.Contains(err.Error(), "failed to describe load balancers") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestListTargetGroupHealthAggregatesAndSorts(t *testing.T) {
	mock := &mockELBv2Client{
		describeTargetGroupsFunc: func(_ context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error) {
			if awssdk.ToString(params.LoadBalancerArn) != "arn:lb" {
				t.Fatalf("expected lookup scoped to the load balancer, got %v", params.LoadBalancerArn)
			}
			return &elasticloadbalancingv2.DescribeTargetGroupsOutput{
				TargetGroups: []elbtypes.TargetGroup{
					{
						TargetGroupArn:  awssdk.String("arn:tg/healthy"),
						TargetGroupName: awssdk.String("all-healthy"),
						Protocol:        elbtypes.ProtocolEnumHttp,
						Port:            awssdk.Int32(80),
						TargetType:      elbtypes.TargetTypeEnumInstance,
					},
					{
						TargetGroupArn:  awssdk.String("arn:tg/broken"),
						TargetGroupName: awssdk.String("has-unhealthy"),
						Protocol:        elbtypes.ProtocolEnumHttps,
						Port:            awssdk.Int32(443),
						TargetType:      elbtypes.TargetTypeEnumIp,
					},
				},
			}, nil
		},
		describeTargetHealthFunc: func(_ context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
			if awssdk.ToString(params.TargetGroupArn) == "arn:tg/healthy" {
				return &elasticloadbalancingv2.DescribeTargetHealthOutput{
					TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
						{
							Target:       &elbtypes.TargetDescription{Id: awssdk.String("i-1"), Port: awssdk.Int32(80)},
							TargetHealth: &elbtypes.TargetHealth{State: elbtypes.TargetHealthStateEnumHealthy},
						},
					},
				}, nil
			}
			return &elasticloadbalancingv2.DescribeTargetHealthOutput{
				TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
					{
						Target:       &elbtypes.TargetDescription{Id: awssdk.String("i-ok"), Port: awssdk.Int32(443)},
						TargetHealth: &elbtypes.TargetHealth{State: elbtypes.TargetHealthStateEnumHealthy},
					},
					{
						Target: &elbtypes.TargetDescription{Id: awssdk.String("i-bad"), Port: awssdk.Int32(443)},
						TargetHealth: &elbtypes.TargetHealth{
							State:       elbtypes.TargetHealthStateEnumUnhealthy,
							Reason:      elbtypes.TargetHealthReasonEnumTimeout,
							Description: awssdk.String("Request timed out"),
						},
					},
					{
						Target:       &elbtypes.TargetDescription{Id: awssdk.String("i-drain"), Port: awssdk.Int32(443)},
						TargetHealth: &elbtypes.TargetHealth{State: elbtypes.TargetHealthStateEnumDraining},
					},
				},
			}, nil
		},
	}
	repo := &AwsRepository{ELBv2Client: mock}

	groups, err := repo.ListTargetGroupHealth(context.Background(), "arn:lb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 target groups, got %d", len(groups))
	}
	if groups[0].Name != "has-unhealthy" {
		t.Fatalf("expected unhealthiest group first, got %s", groups[0].Name)
	}
	broken := groups[0]
	if broken.HealthyCount != 1 || broken.UnhealthyCount != 1 || broken.OtherCount != 1 {
		t.Fatalf("unexpected health counts: %+v", broken)
	}
	if broken.Targets[0].ID != "i-bad" {
		t.Fatalf("expected unhealthy target first, got %s", broken.Targets[0].ID)
	}
	if broken.Targets[0].Reason != "Target.Timeout" || broken.Targets[0].Description != "Request timed out" {
		t.Fatalf("expected reason code and description preserved, got %+v", broken.Targets[0])
	}
	if broken.Targets[1].ID != "i-drain" {
		t.Fatalf("expected draining before healthy, got %s", broken.Targets[1].ID)
	}
}

func TestListTargetGroupHealthFollowsPagination(t *testing.T) {
	pages := 0
	mock := &mockELBv2Client{
		describeTargetGroupsFunc: func(_ context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error) {
			pages++
			if pages == 1 {
				if params.Marker != nil {
					t.Fatalf("first page should have no marker, got %v", params.Marker)
				}
				return &elasticloadbalancingv2.DescribeTargetGroupsOutput{
					TargetGroups: []elbtypes.TargetGroup{
						{TargetGroupArn: awssdk.String("arn:tg/page1"), TargetGroupName: awssdk.String("page1-tg")},
					},
					NextMarker: awssdk.String("page2"),
				}, nil
			}
			if awssdk.ToString(params.Marker) != "page2" {
				t.Fatalf("second page should carry marker, got %v", params.Marker)
			}
			return &elasticloadbalancingv2.DescribeTargetGroupsOutput{
				TargetGroups: []elbtypes.TargetGroup{
					{TargetGroupArn: awssdk.String("arn:tg/page2"), TargetGroupName: awssdk.String("page2-tg")},
				},
			}, nil
		},
		describeTargetHealthFunc: func(_ context.Context, _ *elasticloadbalancingv2.DescribeTargetHealthInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
			return &elasticloadbalancingv2.DescribeTargetHealthOutput{}, nil
		},
	}
	repo := &AwsRepository{ELBv2Client: mock}

	groups, err := repo.ListTargetGroupHealth(context.Background(), "arn:lb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected target groups from both pages, got %d", len(groups))
	}
	names := []string{groups[0].Name, groups[1].Name}
	if names[0] != "page1-tg" || names[1] != "page2-tg" {
		t.Fatalf("expected both pages' groups included, got %v", names)
	}
}

func TestListTargetGroupHealthWrapsHealthError(t *testing.T) {
	mock := &mockELBv2Client{
		describeTargetGroupsFunc: func(_ context.Context, _ *elasticloadbalancingv2.DescribeTargetGroupsInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error) {
			return &elasticloadbalancingv2.DescribeTargetGroupsOutput{
				TargetGroups: []elbtypes.TargetGroup{
					{TargetGroupArn: awssdk.String("arn:tg"), TargetGroupName: awssdk.String("tg")},
				},
			}, nil
		},
		describeTargetHealthFunc: func(_ context.Context, _ *elasticloadbalancingv2.DescribeTargetHealthInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
			return nil, errors.New("denied")
		},
	}
	repo := &AwsRepository{ELBv2Client: mock}
	if _, err := repo.ListTargetGroupHealth(context.Background(), "arn:lb"); err == nil || !strings.Contains(err.Error(), "failed to describe target health for tg") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}
