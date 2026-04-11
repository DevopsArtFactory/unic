package aws

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// mockEC2Client implements EC2ClientAPI for testing.
type mockEC2Client struct {
	describeVpcsFunc                    func(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	describeSubnetsFunc                 func(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	describeInstancesFunc               func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	describeNetworkInterfacesFunc       func(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
	createNetworkInsightsPathFunc       func(ctx context.Context, params *ec2.CreateNetworkInsightsPathInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkInsightsPathOutput, error)
	startNetworkInsightsAnalysisFunc    func(ctx context.Context, params *ec2.StartNetworkInsightsAnalysisInput, optFns ...func(*ec2.Options)) (*ec2.StartNetworkInsightsAnalysisOutput, error)
	describeNetworkInsightsAnalysesFunc func(ctx context.Context, params *ec2.DescribeNetworkInsightsAnalysesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInsightsAnalysesOutput, error)
	deleteNetworkInsightsAnalysisFunc   func(ctx context.Context, params *ec2.DeleteNetworkInsightsAnalysisInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkInsightsAnalysisOutput, error)
	deleteNetworkInsightsPathFunc       func(ctx context.Context, params *ec2.DeleteNetworkInsightsPathInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkInsightsPathOutput, error)
	describeSecurityGroupsFunc          func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	authorizeSGIngressFunc              func(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	authorizeSGEgressFunc               func(ctx context.Context, params *ec2.AuthorizeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupEgressOutput, error)
	revokeSGIngressFunc                 func(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error)
	revokeSGEgressFunc                  func(ctx context.Context, params *ec2.RevokeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupEgressOutput, error)
}

func (m *mockEC2Client) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return m.describeVpcsFunc(ctx, params, optFns...)
}

func (m *mockEC2Client) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return m.describeSubnetsFunc(ctx, params, optFns...)
}

func (m *mockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.describeInstancesFunc != nil {
		return m.describeInstancesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInstancesOutput{}, nil
}

func (m *mockEC2Client) DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	if m.describeNetworkInterfacesFunc != nil {
		return m.describeNetworkInterfacesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}

func (m *mockEC2Client) CreateNetworkInsightsPath(ctx context.Context, params *ec2.CreateNetworkInsightsPathInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkInsightsPathOutput, error) {
	if m.createNetworkInsightsPathFunc != nil {
		return m.createNetworkInsightsPathFunc(ctx, params, optFns...)
	}
	return &ec2.CreateNetworkInsightsPathOutput{}, nil
}

func (m *mockEC2Client) StartNetworkInsightsAnalysis(ctx context.Context, params *ec2.StartNetworkInsightsAnalysisInput, optFns ...func(*ec2.Options)) (*ec2.StartNetworkInsightsAnalysisOutput, error) {
	if m.startNetworkInsightsAnalysisFunc != nil {
		return m.startNetworkInsightsAnalysisFunc(ctx, params, optFns...)
	}
	return &ec2.StartNetworkInsightsAnalysisOutput{}, nil
}

func (m *mockEC2Client) DescribeNetworkInsightsAnalyses(ctx context.Context, params *ec2.DescribeNetworkInsightsAnalysesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInsightsAnalysesOutput, error) {
	if m.describeNetworkInsightsAnalysesFunc != nil {
		return m.describeNetworkInsightsAnalysesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeNetworkInsightsAnalysesOutput{}, nil
}

func (m *mockEC2Client) DeleteNetworkInsightsAnalysis(ctx context.Context, params *ec2.DeleteNetworkInsightsAnalysisInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkInsightsAnalysisOutput, error) {
	if m.deleteNetworkInsightsAnalysisFunc != nil {
		return m.deleteNetworkInsightsAnalysisFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteNetworkInsightsAnalysisOutput{}, nil
}

func (m *mockEC2Client) DeleteNetworkInsightsPath(ctx context.Context, params *ec2.DeleteNetworkInsightsPathInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkInsightsPathOutput, error) {
	if m.deleteNetworkInsightsPathFunc != nil {
		return m.deleteNetworkInsightsPathFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteNetworkInsightsPathOutput{}, nil
}

func (m *mockEC2Client) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	if m.describeSecurityGroupsFunc != nil {
		return m.describeSecurityGroupsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeSecurityGroupsOutput{}, nil
}

func (m *mockEC2Client) AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	if m.authorizeSGIngressFunc != nil {
		return m.authorizeSGIngressFunc(ctx, params, optFns...)
	}
	return &ec2.AuthorizeSecurityGroupIngressOutput{}, nil
}

func (m *mockEC2Client) AuthorizeSecurityGroupEgress(ctx context.Context, params *ec2.AuthorizeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupEgressOutput, error) {
	if m.authorizeSGEgressFunc != nil {
		return m.authorizeSGEgressFunc(ctx, params, optFns...)
	}
	return &ec2.AuthorizeSecurityGroupEgressOutput{}, nil
}

func (m *mockEC2Client) RevokeSecurityGroupIngress(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error) {
	if m.revokeSGIngressFunc != nil {
		return m.revokeSGIngressFunc(ctx, params, optFns...)
	}
	return &ec2.RevokeSecurityGroupIngressOutput{}, nil
}

func (m *mockEC2Client) RevokeSecurityGroupEgress(ctx context.Context, params *ec2.RevokeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupEgressOutput, error) {
	if m.revokeSGEgressFunc != nil {
		return m.revokeSGEgressFunc(ctx, params, optFns...)
	}
	return &ec2.RevokeSecurityGroupEgressOutput{}, nil
}

// --- VPC tests ---

func TestListVPCs_Success(t *testing.T) {
	mock := &mockEC2Client{
		describeVpcsFunc: func(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
			return &ec2.DescribeVpcsOutput{
				Vpcs: []types.Vpc{
					{
						VpcId:     awssdk.String("vpc-aaa"),
						CidrBlock: awssdk.String("10.0.0.0/16"),
						IsDefault: awssdk.Bool(false),
						Tags:      []types.Tag{{Key: awssdk.String("Name"), Value: awssdk.String("prod-vpc")}},
					},
					{
						VpcId:     awssdk.String("vpc-bbb"),
						CidrBlock: awssdk.String("172.31.0.0/16"),
						IsDefault: awssdk.Bool(true),
						Tags:      nil,
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	vpcs, err := repo.ListVPCs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vpcs) != 2 {
		t.Fatalf("expected 2 VPCs, got %d", len(vpcs))
	}

	if vpcs[0].VPCID != "vpc-aaa" || vpcs[0].Name != "prod-vpc" || vpcs[0].CIDR != "10.0.0.0/16" || vpcs[0].IsDefault {
		t.Errorf("unexpected first VPC: %+v", vpcs[0])
	}
	if vpcs[1].VPCID != "vpc-bbb" || vpcs[1].Name != "Unknown" || !vpcs[1].IsDefault {
		t.Errorf("unexpected second VPC: %+v", vpcs[1])
	}
}

func TestListVPCs_Error(t *testing.T) {
	mock := &mockEC2Client{
		describeVpcsFunc: func(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	_, err := repo.ListVPCs(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListVPCs_Empty(t *testing.T) {
	mock := &mockEC2Client{
		describeVpcsFunc: func(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
			return &ec2.DescribeVpcsOutput{Vpcs: []types.Vpc{}}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	vpcs, err := repo.ListVPCs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vpcs) != 0 {
		t.Errorf("expected empty slice, got %d", len(vpcs))
	}
}

func TestListVPCs_SortedByName(t *testing.T) {
	mock := &mockEC2Client{
		describeVpcsFunc: func(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
			return &ec2.DescribeVpcsOutput{
				Vpcs: []types.Vpc{
					{VpcId: awssdk.String("vpc-2"), Tags: []types.Tag{{Key: awssdk.String("Name"), Value: awssdk.String("zeta")}}},
					{VpcId: awssdk.String("vpc-1"), Tags: []types.Tag{{Key: awssdk.String("Name"), Value: awssdk.String("alpha")}}},
				},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	vpcs, err := repo.ListVPCs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vpcs) != 2 {
		t.Fatalf("expected 2 VPCs, got %d", len(vpcs))
	}
	if vpcs[0].Name != "alpha" || vpcs[1].Name != "zeta" {
		t.Fatalf("expected alphabetical VPC order, got %+v", vpcs)
	}
}

// --- Subnet tests ---

func TestListSubnets_Success(t *testing.T) {
	mock := &mockEC2Client{
		describeSubnetsFunc: func(_ context.Context, params *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
			// Verify filter is set correctly
			if len(params.Filters) != 1 || awssdk.ToString(params.Filters[0].Name) != "vpc-id" {
				t.Errorf("expected vpc-id filter, got %+v", params.Filters)
			}
			return &ec2.DescribeSubnetsOutput{
				Subnets: []types.Subnet{
					{
						SubnetId:                awssdk.String("subnet-111"),
						CidrBlock:               awssdk.String("10.0.1.0/24"),
						AvailabilityZone:        awssdk.String("ap-northeast-2a"),
						AvailableIpAddressCount: awssdk.Int32(251),
						Tags:                    []types.Tag{{Key: awssdk.String("Name"), Value: awssdk.String("public-a")}},
					},
					{
						SubnetId:                awssdk.String("subnet-222"),
						CidrBlock:               awssdk.String("10.0.2.0/24"),
						AvailabilityZone:        awssdk.String("ap-northeast-2b"),
						AvailableIpAddressCount: awssdk.Int32(100),
						Tags:                    nil,
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	subnets, err := repo.ListSubnets(context.Background(), "vpc-aaa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subnets) != 2 {
		t.Fatalf("expected 2 subnets, got %d", len(subnets))
	}

	s := subnets[0]
	if s.SubnetID != "subnet-111" || s.Name != "public-a" || s.CIDR != "10.0.1.0/24" ||
		s.AvailabilityZone != "ap-northeast-2a" || s.AvailableIPCount != 251 {
		t.Errorf("unexpected subnet: %+v", s)
	}
}

func TestListSubnets_Error(t *testing.T) {
	mock := &mockEC2Client{
		describeSubnetsFunc: func(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
			return nil, fmt.Errorf("network error")
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	_, err := repo.ListSubnets(context.Background(), "vpc-aaa")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListSubnets_Empty(t *testing.T) {
	mock := &mockEC2Client{
		describeSubnetsFunc: func(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
			return &ec2.DescribeSubnetsOutput{Subnets: []types.Subnet{}}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	subnets, err := repo.ListSubnets(context.Background(), "vpc-empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subnets) != 0 {
		t.Errorf("expected empty slice, got %d", len(subnets))
	}
}

func TestListSubnets_SortedByName(t *testing.T) {
	mock := &mockEC2Client{
		describeSubnetsFunc: func(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
			return &ec2.DescribeSubnetsOutput{
				Subnets: []types.Subnet{
					{SubnetId: awssdk.String("subnet-2"), Tags: []types.Tag{{Key: awssdk.String("Name"), Value: awssdk.String("z-private")}}},
					{SubnetId: awssdk.String("subnet-1"), Tags: []types.Tag{{Key: awssdk.String("Name"), Value: awssdk.String("a-public")}}},
				},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	subnets, err := repo.ListSubnets(context.Background(), "vpc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subnets) != 2 {
		t.Fatalf("expected 2 subnets, got %d", len(subnets))
	}
	if subnets[0].Name != "a-public" || subnets[1].Name != "z-private" {
		t.Fatalf("expected alphabetical subnet order, got %+v", subnets)
	}
}

func TestListReachabilityTargets_Success(t *testing.T) {
	mock := &mockEC2Client{
		describeInstancesFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:       awssdk.String("i-123"),
								PrivateIpAddress: awssdk.String("10.0.1.10"),
								VpcId:            awssdk.String("vpc-1"),
								SubnetId:         awssdk.String("subnet-1"),
								State:            &types.InstanceState{Name: types.InstanceStateNameRunning},
								Tags:             []types.Tag{{Key: awssdk.String("Name"), Value: awssdk.String("app-a")}},
							},
						},
					},
				},
			}, nil
		},
		describeNetworkInterfacesFunc: func(_ context.Context, _ *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []types.NetworkInterface{
					{
						NetworkInterfaceId: awssdk.String("eni-123"),
						PrivateIpAddress:   awssdk.String("10.0.1.20"),
						VpcId:              awssdk.String("vpc-1"),
						SubnetId:           awssdk.String("subnet-1"),
						Status:             types.NetworkInterfaceStatusInUse,
						Description:        awssdk.String("db-eni"),
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	targets, err := repo.ListReachabilityTargets(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
	if targets[0].Type != "EC2 Instance" || targets[1].Type != "ENI" {
		t.Fatalf("unexpected targets: %+v", targets)
	}
}

func TestRunReachabilityAnalysis_Success(t *testing.T) {
	var deletedAnalysis string
	var deletedPath string
	mock := &mockEC2Client{
		createNetworkInsightsPathFunc: func(_ context.Context, params *ec2.CreateNetworkInsightsPathInput, _ ...func(*ec2.Options)) (*ec2.CreateNetworkInsightsPathOutput, error) {
			if awssdk.ToString(params.Source) != "i-src" {
				t.Fatalf("expected source i-src, got %s", awssdk.ToString(params.Source))
			}
			if awssdk.ToInt32(params.DestinationPort) != 443 {
				t.Fatalf("expected port 443, got %d", awssdk.ToInt32(params.DestinationPort))
			}
			return &ec2.CreateNetworkInsightsPathOutput{
				NetworkInsightsPath: &types.NetworkInsightsPath{
					NetworkInsightsPathId: awssdk.String("nip-123"),
				},
			}, nil
		},
		startNetworkInsightsAnalysisFunc: func(_ context.Context, params *ec2.StartNetworkInsightsAnalysisInput, _ ...func(*ec2.Options)) (*ec2.StartNetworkInsightsAnalysisOutput, error) {
			if awssdk.ToString(params.NetworkInsightsPathId) != "nip-123" {
				t.Fatalf("expected path ID nip-123, got %s", awssdk.ToString(params.NetworkInsightsPathId))
			}
			return &ec2.StartNetworkInsightsAnalysisOutput{
				NetworkInsightsAnalysis: &types.NetworkInsightsAnalysis{
					NetworkInsightsAnalysisId: awssdk.String("nia-123"),
				},
			}, nil
		},
		describeNetworkInsightsAnalysesFunc: func(_ context.Context, _ *ec2.DescribeNetworkInsightsAnalysesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInsightsAnalysesOutput, error) {
			return &ec2.DescribeNetworkInsightsAnalysesOutput{
				NetworkInsightsAnalyses: []types.NetworkInsightsAnalysis{
					{
						NetworkInsightsAnalysisId: awssdk.String("nia-123"),
						Status:                    types.AnalysisStatusSucceeded,
						StatusMessage:             awssdk.String("Analysis completed successfully"),
						NetworkPathFound:          awssdk.Bool(false),
						ForwardPathComponents: []types.PathComponent{
							{
								SequenceNumber: awssdk.Int32(1),
								Component:      &types.AnalysisComponent{Id: awssdk.String("eni-src"), Name: awssdk.String("source-eni")},
								Explanations: []types.Explanation{
									{
										ExplanationCode: awssdk.String("ENI_SG_RULES_MISMATCH"),
										SecurityGroup:   &types.AnalysisComponent{Id: awssdk.String("sg-123"), Name: awssdk.String("web-sg")},
									},
								},
							},
						},
						Explanations: []types.Explanation{
							{
								ExplanationCode: awssdk.String("ENI_SG_RULES_MISMATCH"),
								Component:       &types.AnalysisComponent{Id: awssdk.String("eni-dst"), Name: awssdk.String("dst-eni")},
								SecurityGroup:   &types.AnalysisComponent{Id: awssdk.String("sg-123"), Name: awssdk.String("web-sg")},
								Port:            awssdk.Int32(443),
							},
						},
						StartDate: awssdk.Time(time.Now()),
					},
				},
			}, nil
		},
		deleteNetworkInsightsAnalysisFunc: func(_ context.Context, params *ec2.DeleteNetworkInsightsAnalysisInput, _ ...func(*ec2.Options)) (*ec2.DeleteNetworkInsightsAnalysisOutput, error) {
			deletedAnalysis = awssdk.ToString(params.NetworkInsightsAnalysisId)
			return &ec2.DeleteNetworkInsightsAnalysisOutput{}, nil
		},
		deleteNetworkInsightsPathFunc: func(_ context.Context, params *ec2.DeleteNetworkInsightsPathInput, _ ...func(*ec2.Options)) (*ec2.DeleteNetworkInsightsPathOutput, error) {
			deletedPath = awssdk.ToString(params.NetworkInsightsPathId)
			return &ec2.DeleteNetworkInsightsPathOutput{}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	result, err := repo.RunReachabilityAnalysis(context.Background(),
		ReachabilityTarget{ID: "i-src", Name: "source", Type: "EC2 Instance"},
		ReachabilityTarget{ID: "i-dst", Name: "dest", Type: "EC2 Instance"},
		"",
		"TCP",
		443,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PathID != "nip-123" || result.AnalysisID != "nia-123" {
		t.Fatalf("unexpected IDs: %+v", result)
	}
	if result.NetworkPathFound {
		t.Fatalf("expected unreachable result")
	}
	if len(result.ForwardPath) != 1 || len(result.Explanations) != 1 {
		t.Fatalf("expected mapped path and explanations, got %+v", result)
	}
	if result.Explanations[0].Code != "ENI_SG_RULES_MISMATCH" {
		t.Fatalf("unexpected explanation: %+v", result.Explanations[0])
	}
	if deletedAnalysis != "nia-123" {
		t.Fatalf("expected analysis cleanup, got %q", deletedAnalysis)
	}
	if deletedPath != "nip-123" {
		t.Fatalf("expected path cleanup, got %q", deletedPath)
	}
}

func TestRunReachabilityAnalysis_ManualIPv4UsesDestinationIP(t *testing.T) {
	mock := &mockEC2Client{
		createNetworkInsightsPathFunc: func(_ context.Context, params *ec2.CreateNetworkInsightsPathInput, _ ...func(*ec2.Options)) (*ec2.CreateNetworkInsightsPathOutput, error) {
			if awssdk.ToString(params.DestinationIp) != "10.0.2.15" {
				t.Fatalf("expected destination IP, got %q", awssdk.ToString(params.DestinationIp))
			}
			if params.FilterAtSource != nil {
				t.Fatalf("expected no source filter for manual IPv4 destination")
			}
			return &ec2.CreateNetworkInsightsPathOutput{
				NetworkInsightsPath: &types.NetworkInsightsPath{NetworkInsightsPathId: awssdk.String("nip-123")},
			}, nil
		},
		startNetworkInsightsAnalysisFunc: func(_ context.Context, _ *ec2.StartNetworkInsightsAnalysisInput, _ ...func(*ec2.Options)) (*ec2.StartNetworkInsightsAnalysisOutput, error) {
			return &ec2.StartNetworkInsightsAnalysisOutput{
				NetworkInsightsAnalysis: &types.NetworkInsightsAnalysis{NetworkInsightsAnalysisId: awssdk.String("nia-123")},
			}, nil
		},
		describeNetworkInsightsAnalysesFunc: func(_ context.Context, _ *ec2.DescribeNetworkInsightsAnalysesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInsightsAnalysesOutput, error) {
			return &ec2.DescribeNetworkInsightsAnalysesOutput{
				NetworkInsightsAnalyses: []types.NetworkInsightsAnalysis{{
					Status:           types.AnalysisStatusSucceeded,
					NetworkPathFound: awssdk.Bool(true),
				}},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	_, err := repo.RunReachabilityAnalysis(context.Background(),
		ReachabilityTarget{ID: "i-src", Name: "source", Type: "EC2 Instance"},
		ReachabilityTarget{},
		"10.0.2.15",
		"TCP",
		443,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunReachabilityAnalysis_FailedStatusReturnsError(t *testing.T) {
	mock := &mockEC2Client{
		createNetworkInsightsPathFunc: func(_ context.Context, _ *ec2.CreateNetworkInsightsPathInput, _ ...func(*ec2.Options)) (*ec2.CreateNetworkInsightsPathOutput, error) {
			return &ec2.CreateNetworkInsightsPathOutput{
				NetworkInsightsPath: &types.NetworkInsightsPath{NetworkInsightsPathId: awssdk.String("nip-123")},
			}, nil
		},
		startNetworkInsightsAnalysisFunc: func(_ context.Context, _ *ec2.StartNetworkInsightsAnalysisInput, _ ...func(*ec2.Options)) (*ec2.StartNetworkInsightsAnalysisOutput, error) {
			return &ec2.StartNetworkInsightsAnalysisOutput{
				NetworkInsightsAnalysis: &types.NetworkInsightsAnalysis{NetworkInsightsAnalysisId: awssdk.String("nia-123")},
			}, nil
		},
		describeNetworkInsightsAnalysesFunc: func(_ context.Context, _ *ec2.DescribeNetworkInsightsAnalysesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInsightsAnalysesOutput, error) {
			return &ec2.DescribeNetworkInsightsAnalysesOutput{
				NetworkInsightsAnalyses: []types.NetworkInsightsAnalysis{{
					Status:        types.AnalysisStatusFailed,
					StatusMessage: awssdk.String("unsupported destination"),
				}},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	_, err := repo.RunReachabilityAnalysis(context.Background(),
		ReachabilityTarget{ID: "i-src"},
		ReachabilityTarget{ID: "i-dst"},
		"",
		"TCP",
		443,
	)
	if err == nil || !strings.Contains(err.Error(), "unsupported destination") {
		t.Fatalf("expected failed analysis error, got %v", err)
	}
}

// --- Model tests ---

func TestVPCDisplayTitle(t *testing.T) {
	v := VPC{VPCID: "vpc-aaa", Name: "prod-vpc", CIDR: "10.0.0.0/16", IsDefault: false}
	expected := "prod-vpc (vpc-aaa) - 10.0.0.0/16"
	if got := v.DisplayTitle(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestVPCDisplayTitle_Default(t *testing.T) {
	v := VPC{VPCID: "vpc-bbb", Name: "Unknown", CIDR: "172.31.0.0/16", IsDefault: true}
	expected := "Unknown (vpc-bbb) - 172.31.0.0/16 [default]"
	if got := v.DisplayTitle(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestVPCFilterText(t *testing.T) {
	v := VPC{VPCID: "vpc-aaa", Name: "ProdVPC", CIDR: "10.0.0.0/16"}
	ft := v.FilterText()
	for _, kw := range []string{"prodvpc", "vpc-aaa", "10.0.0.0/16"} {
		if !containsSubstr(ft, kw) {
			t.Errorf("FilterText %q should contain %q", ft, kw)
		}
	}
}

func TestSubnetDisplayTitle(t *testing.T) {
	s := Subnet{
		SubnetID: "subnet-111", Name: "public-a", CIDR: "10.0.1.0/24",
		AvailabilityZone: "ap-northeast-2a", AvailableIPCount: 251,
	}
	expected := "public-a (subnet-111) - 10.0.1.0/24 | ap-northeast-2a | 251 IPs available"
	if got := s.DisplayTitle(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestSubnetFilterText(t *testing.T) {
	s := Subnet{SubnetID: "subnet-111", Name: "PublicA", CIDR: "10.0.1.0/24", AvailabilityZone: "ap-northeast-2a"}
	ft := s.FilterText()
	for _, kw := range []string{"publica", "subnet-111", "10.0.1.0/24", "ap-northeast-2a"} {
		if !containsSubstr(ft, kw) {
			t.Errorf("FilterText %q should contain %q", ft, kw)
		}
	}
}

// --- cidrUsableIPs tests ---

func TestCIDRUsableIPs_Slash28(t *testing.T) {
	// /28 = 16 total, 5 reserved = 11 usable
	ips, err := cidrUsableIPs("10.0.1.0/28")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ips) != 11 {
		t.Errorf("expected 11 usable IPs for /28, got %d", len(ips))
	}
	// First usable should be .4, last should be .14
	if ips[0] != "10.0.1.4" {
		t.Errorf("expected first IP 10.0.1.4, got %s", ips[0])
	}
	if ips[len(ips)-1] != "10.0.1.14" {
		t.Errorf("expected last IP 10.0.1.14, got %s", ips[len(ips)-1])
	}
}

func TestCIDRUsableIPs_Slash24(t *testing.T) {
	// /24 = 256 total, 5 reserved = 251 usable
	ips, err := cidrUsableIPs("10.0.1.0/24")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ips) != 251 {
		t.Errorf("expected 251 usable IPs for /24, got %d", len(ips))
	}
	if ips[0] != "10.0.1.4" {
		t.Errorf("expected first IP 10.0.1.4, got %s", ips[0])
	}
	if ips[len(ips)-1] != "10.0.1.254" {
		t.Errorf("expected last IP 10.0.1.254, got %s", ips[len(ips)-1])
	}
}

func TestCIDRUsableIPs_ReservedExcluded(t *testing.T) {
	ips, err := cidrUsableIPs("192.168.0.0/24")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	reserved := []string{"192.168.0.0", "192.168.0.1", "192.168.0.2", "192.168.0.3", "192.168.0.255"}
	ipSet := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		ipSet[ip] = struct{}{}
	}
	for _, r := range reserved {
		if _, found := ipSet[r]; found {
			t.Errorf("reserved IP %s should not be in usable list", r)
		}
	}
}

func TestCIDRUsableIPs_InvalidCIDR(t *testing.T) {
	_, err := cidrUsableIPs("not-a-cidr")
	if err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
}

// --- ListAvailableIPs tests ---

func TestListAvailableIPs_Success(t *testing.T) {
	mock := &mockEC2Client{
		describeNetworkInterfacesFunc: func(_ context.Context, params *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
			// Simulate .4 and .5 being in use
			return &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []types.NetworkInterface{
					{
						PrivateIpAddresses: []types.NetworkInterfacePrivateIpAddress{
							{PrivateIpAddress: awssdk.String("10.0.1.4")},
						},
					},
					{
						PrivateIpAddresses: []types.NetworkInterfacePrivateIpAddress{
							{PrivateIpAddress: awssdk.String("10.0.1.5")},
						},
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	ips, err := repo.ListAvailableIPs(context.Background(), "subnet-111", "10.0.1.0/28")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// /28 has 11 usable, minus 2 in use = 9
	if len(ips) != 9 {
		t.Errorf("expected 9 available IPs, got %d", len(ips))
	}
	for _, ip := range ips {
		if ip == "10.0.1.4" || ip == "10.0.1.5" {
			t.Errorf("used IP %s should not appear in available list", ip)
		}
	}
}

func TestListAvailableIPs_AllFree(t *testing.T) {
	mock := &mockEC2Client{
		describeNetworkInterfacesFunc: func(_ context.Context, _ *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return &ec2.DescribeNetworkInterfacesOutput{}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	ips, err := repo.ListAvailableIPs(context.Background(), "subnet-111", "10.0.1.0/28")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ips) != 11 {
		t.Errorf("expected 11 available IPs, got %d", len(ips))
	}
}

func TestListAvailableIPs_NetworkInterfaceError(t *testing.T) {
	mock := &mockEC2Client{
		describeNetworkInterfacesFunc: func(_ context.Context, _ *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return nil, fmt.Errorf("api error")
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	_, err := repo.ListAvailableIPs(context.Background(), "subnet-111", "10.0.1.0/28")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
