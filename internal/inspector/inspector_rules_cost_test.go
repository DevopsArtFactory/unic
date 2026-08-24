package inspector

import (
	"context"
	"errors"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

type mockCostELBv2Client struct {
	describeTargetGroupsFunc func(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error)
	describeTargetHealthFunc func(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error)
}

func (m *mockCostELBv2Client) DescribeTargetGroups(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error) {
	return m.describeTargetGroupsFunc(ctx, params, optFns...)
}

func (m *mockCostELBv2Client) DescribeTargetHealth(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
	return m.describeTargetHealthFunc(ctx, params, optFns...)
}

func (m *mockCostELBv2Client) DescribeLoadBalancers(context.Context, *elasticloadbalancingv2.DescribeLoadBalancersInput, ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
	return &elasticloadbalancingv2.DescribeLoadBalancersOutput{}, nil
}

func (m *mockCostELBv2Client) DescribeListeners(context.Context, *elasticloadbalancingv2.DescribeListenersInput, ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error) {
	return &elasticloadbalancingv2.DescribeListenersOutput{}, nil
}

func (m *mockCostELBv2Client) DescribeRules(context.Context, *elasticloadbalancingv2.DescribeRulesInput, ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeRulesOutput, error) {
	return &elasticloadbalancingv2.DescribeRulesOutput{}, nil
}

func TestInspectCostWasteFindsWasteAndMissingRequiredTags(t *testing.T) {
	now := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	volumeCalls := 0
	targetGroupCalls := 0
	targetHealthCalls := 0

	ec2Client := &mockEC2Client{
		describeAddressesFunc: func(context.Context, *ec2.DescribeAddressesInput, ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
			return &ec2.DescribeAddressesOutput{Addresses: []ec2types.Address{
				{
					AllocationId: awssdk.String("eipalloc-unused"),
					PublicIp:     awssdk.String("203.0.113.10"),
					Tags:         []ec2types.Tag{{Key: awssdk.String("aws:managed"), Value: awssdk.String("true")}},
				},
				{
					AllocationId:  awssdk.String("eipalloc-used"),
					AssociationId: awssdk.String("eipassoc-1"),
					Tags:          []ec2types.Tag{{Key: awssdk.String("Owner"), Value: awssdk.String("platform")}},
				},
			}}, nil
		},
		describeVolumesFunc: func(_ context.Context, params *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
			volumeCalls++
			if awssdk.ToString(params.NextToken) == "" {
				return &ec2.DescribeVolumesOutput{
					Volumes: []ec2types.Volume{{
						VolumeId:   awssdk.String("vol-unused"),
						State:      ec2types.VolumeStateAvailable,
						Size:       awssdk.Int32(100),
						VolumeType: ec2types.VolumeTypeGp3,
					}},
					NextToken: awssdk.String("vol-next"),
				}, nil
			}
			return &ec2.DescribeVolumesOutput{Volumes: []ec2types.Volume{{
				VolumeId:    awssdk.String("vol-used"),
				State:       ec2types.VolumeStateInUse,
				Attachments: []ec2types.VolumeAttachment{{InstanceId: awssdk.String("i-running")}},
				Tags:        []ec2types.Tag{{Key: awssdk.String("Owner"), Value: awssdk.String("platform")}},
			}}}, nil
		},
		describeInstancesFunc: func(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{
				{
					InstanceId: awssdk.String("i-stopped"),
					State:      &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopped},
					Tags:       []ec2types.Tag{{Key: awssdk.String("aws:autoscaling:groupName"), Value: awssdk.String("legacy")}},
				},
				{
					InstanceId: awssdk.String("i-running"),
					State:      &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
					Tags:       []ec2types.Tag{{Key: awssdk.String("Environment"), Value: awssdk.String("prod")}},
				},
			}}}}, nil
		},
		describeSnapshotsFunc: func(_ context.Context, params *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
			if len(params.OwnerIds) != 1 || params.OwnerIds[0] != "self" {
				t.Fatalf("expected self-owned snapshot filter, got %+v", params.OwnerIds)
			}
			return &ec2.DescribeSnapshotsOutput{Snapshots: []ec2types.Snapshot{
				{
					SnapshotId: awssdk.String("snap-aged"),
					StartTime:  awssdk.Time(now.Add(-90 * 24 * time.Hour)),
					Tags:       []ec2types.Tag{{Key: awssdk.String("aws:backup:source-resource"), Value: awssdk.String("vol-old")}},
				},
				{
					SnapshotId: awssdk.String("snap-recent"),
					StartTime:  awssdk.Time(now.Add(-89 * 24 * time.Hour)),
					Tags:       []ec2types.Tag{{Key: awssdk.String("Owner"), Value: awssdk.String("platform")}},
				},
			}}, nil
		},
	}

	elbClient := &mockCostELBv2Client{
		describeTargetGroupsFunc: func(_ context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error) {
			targetGroupCalls++
			if awssdk.ToString(params.Marker) == "" {
				return &elasticloadbalancingv2.DescribeTargetGroupsOutput{
					TargetGroups: []elbtypes.TargetGroup{{
						TargetGroupArn:  awssdk.String("arn:aws:elasticloadbalancing:us-east-1:123:targetgroup/empty/1"),
						TargetGroupName: awssdk.String("empty"),
					}},
					NextMarker: awssdk.String("tg-next"),
				}, nil
			}
			return &elasticloadbalancingv2.DescribeTargetGroupsOutput{TargetGroups: []elbtypes.TargetGroup{{
				TargetGroupArn:  awssdk.String("arn:aws:elasticloadbalancing:us-east-1:123:targetgroup/used/2"),
				TargetGroupName: awssdk.String("used"),
			}}}, nil
		},
		describeTargetHealthFunc: func(_ context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
			targetHealthCalls++
			if awssdk.ToString(params.TargetGroupArn) == "arn:aws:elasticloadbalancing:us-east-1:123:targetgroup/empty/1" {
				return &elasticloadbalancingv2.DescribeTargetHealthOutput{}, nil
			}
			return &elasticloadbalancingv2.DescribeTargetHealthOutput{
				TargetHealthDescriptions: []elbtypes.TargetHealthDescription{{Target: &elbtypes.TargetDescription{Id: awssdk.String("i-running")}}},
			}, nil
		},
	}

	findings, err := inspectCostWaste(context.Background(), &AwsRepository{EC2Client: ec2Client, ELBv2Client: elbClient}, now, []string{"Owner"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if volumeCalls != 2 || targetGroupCalls != 2 || targetHealthCalls != 2 {
		t.Fatalf("expected paginator and sequential health coverage, got volume calls %d, target group calls %d, and target health calls %d", volumeCalls, targetGroupCalls, targetHealthCalls)
	}
	if len(findings) != 10 {
		t.Fatalf("expected 10 findings, got %d: %+v", len(findings), findings)
	}

	byRuleAndResource := make(map[string]SecurityFinding, len(findings))
	for _, finding := range findings {
		byRuleAndResource[finding.RuleID+"/"+finding.ResourceID] = finding
	}
	for key, severity := range map[string]RuleSeverity{
		inspectorRuleIDCostEIPUnattached + "/eipalloc-unused":                                                   RuleSeverityMedium,
		inspectorRuleIDCostResourceUntagged + "/eipalloc-unused":                                                RuleSeverityLow,
		inspectorRuleIDCostVolumeUnattached + "/vol-unused":                                                     RuleSeverityMedium,
		inspectorRuleIDCostResourceUntagged + "/vol-unused":                                                     RuleSeverityLow,
		inspectorRuleIDCostInstanceStopped + "/i-stopped":                                                       RuleSeverityMedium,
		inspectorRuleIDCostResourceUntagged + "/i-stopped":                                                      RuleSeverityLow,
		inspectorRuleIDCostResourceUntagged + "/i-running":                                                      RuleSeverityLow,
		inspectorRuleIDCostSnapshotAged + "/snap-aged":                                                          RuleSeverityLow,
		inspectorRuleIDCostResourceUntagged + "/snap-aged":                                                      RuleSeverityLow,
		inspectorRuleIDCostTargetGroupEmpty + "/arn:aws:elasticloadbalancing:us-east-1:123:targetgroup/empty/1": RuleSeverityLow,
	} {
		finding, ok := byRuleAndResource[key]
		if !ok {
			t.Errorf("missing finding %s", key)
			continue
		}
		if finding.Severity != severity {
			t.Errorf("finding %s severity = %s, want %s", key, finding.Severity, severity)
		}
	}
	if got := byRuleAndResource[inspectorRuleIDCostResourceUntagged+"/eipalloc-unused"].Summary; got != "ElasticIP eipalloc-unused is missing required tags: Owner." {
		t.Fatalf("unexpected required-tag summary: %q", got)
	}
}

func TestUntaggedCostFindingUsesConfiguredRequiredTags(t *testing.T) {
	tags := []ec2types.Tag{{Key: awssdk.String("Owner"), Value: awssdk.String("platform")}}
	finding, ok := untaggedCostFinding(
		"EC2Instance",
		"i-123",
		tags,
		[]string{"Owner", "Environment", "CostCenter", "Environment", " "},
	)
	if !ok {
		t.Fatal("expected missing required tags to produce a finding")
	}
	if finding.Summary != "EC2Instance i-123 is missing required tags: Environment, CostCenter." {
		t.Fatalf("unexpected summary: %q", finding.Summary)
	}

	if _, ok := untaggedCostFinding("EC2Instance", "i-123", nil, nil); ok {
		t.Fatal("expected an empty required-tag policy to disable the rule")
	}

	whitespaceTag := []ec2types.Tag{{Key: awssdk.String(" Owner ")}}
	if finding, ok := untaggedCostFinding("EC2Instance", "i-123", whitespaceTag, []string{"Owner"}); !ok || finding.Summary != "EC2Instance i-123 is missing required tags: Owner." {
		t.Fatalf("expected exact AWS tag-key matching, got finding %+v, ok %t", finding, ok)
	}
}

func TestInspectCostWasteReturnsContextualEC2Error(t *testing.T) {
	repo := &AwsRepository{EC2Client: &mockEC2Client{
		describeVolumesFunc: func(context.Context, *ec2.DescribeVolumesInput, ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
			return nil, errors.New("denied")
		},
	}}

	_, err := inspectCostWaste(context.Background(), repo, time.Now().UTC(), nil)
	if err == nil || err.Error() != "failed to inspect EBS volumes: denied" {
		t.Fatalf("expected contextual EBS error, got %v", err)
	}
}
