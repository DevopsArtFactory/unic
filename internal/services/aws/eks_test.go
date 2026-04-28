package aws

import (
	"context"
	"fmt"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
)

type mockEKSClient struct {
	listClustersFunc      func(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error)
	describeClusterFunc   func(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
	listNodegroupsFunc    func(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error)
	describeNodegroupFunc func(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error)
	listAddonsFunc        func(ctx context.Context, params *eks.ListAddonsInput, optFns ...func(*eks.Options)) (*eks.ListAddonsOutput, error)
	describeAddonFunc     func(ctx context.Context, params *eks.DescribeAddonInput, optFns ...func(*eks.Options)) (*eks.DescribeAddonOutput, error)
}

func (m *mockEKSClient) ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	return m.listClustersFunc(ctx, params, optFns...)
}

func (m *mockEKSClient) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	return m.describeClusterFunc(ctx, params, optFns...)
}

func (m *mockEKSClient) ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	return m.listNodegroupsFunc(ctx, params, optFns...)
}

func (m *mockEKSClient) DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	return m.describeNodegroupFunc(ctx, params, optFns...)
}

func (m *mockEKSClient) ListAddons(ctx context.Context, params *eks.ListAddonsInput, optFns ...func(*eks.Options)) (*eks.ListAddonsOutput, error) {
	return m.listAddonsFunc(ctx, params, optFns...)
}

func (m *mockEKSClient) DescribeAddon(ctx context.Context, params *eks.DescribeAddonInput, optFns ...func(*eks.Options)) (*eks.DescribeAddonOutput, error) {
	return m.describeAddonFunc(ctx, params, optFns...)
}

func TestListEKSClusters(t *testing.T) {
	mock := &mockEKSClient{
		listClustersFunc: func(_ context.Context, _ *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
			return &eks.ListClustersOutput{Clusters: []string{"prod-eks", "dev-eks"}}, nil
		},
		describeClusterFunc: func(_ context.Context, params *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
			name := awssdk.ToString(params.Name)
			return &eks.DescribeClusterOutput{Cluster: &ekstypes.Cluster{
				Name:    awssdk.String(name),
				Arn:     awssdk.String("arn:aws:eks:ap-northeast-2:123456789012:cluster/" + name),
				Version: awssdk.String("1.32"),
				Status:  ekstypes.ClusterStatusActive,
				ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
					EndpointPublicAccess:  true,
					EndpointPrivateAccess: false,
				},
			}}, nil
		},
	}

	repo := &AwsRepository{EKSClient: mock}
	clusters, err := repo.ListEKSClusters(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}
	if clusters[0].Name != "dev-eks" {
		t.Fatalf("expected sorted cluster dev-eks first, got %s", clusters[0].Name)
	}
	if got := clusters[0].EndpointVisibility(); got != "pub:yes priv:no" {
		t.Fatalf("unexpected endpoint visibility: %s", got)
	}
}

func TestListEKSNodeGroups(t *testing.T) {
	mock := &mockEKSClient{
		listNodegroupsFunc: func(_ context.Context, params *eks.ListNodegroupsInput, _ ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
			if got := awssdk.ToString(params.ClusterName); got != "prod-eks" {
				t.Fatalf("expected cluster name prod-eks, got %s", got)
			}
			return &eks.ListNodegroupsOutput{Nodegroups: []string{"blue", "green"}}, nil
		},
		describeNodegroupFunc: func(_ context.Context, params *eks.DescribeNodegroupInput, _ ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
			name := awssdk.ToString(params.NodegroupName)
			return &eks.DescribeNodegroupOutput{Nodegroup: &ekstypes.Nodegroup{
				ClusterName:    awssdk.String("prod-eks"),
				NodegroupName:  awssdk.String(name),
				NodegroupArn:   awssdk.String("arn:aws:eks:ap-northeast-2:123456789012:nodegroup/prod-eks/" + name + "/uuid"),
				Status:         ekstypes.NodegroupStatusActive,
				Version:        awssdk.String("1.32"),
				ReleaseVersion: awssdk.String("1.32.3-20260401"),
				AmiType:        ekstypes.AMITypesAl2X8664,
				CapacityType:   ekstypes.CapacityTypesOnDemand,
				InstanceTypes:  []string{"m6i.large"},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					DesiredSize: awssdk.Int32(3),
					MinSize:     awssdk.Int32(2),
					MaxSize:     awssdk.Int32(5),
				},
				Health: &ekstypes.NodegroupHealth{Issues: []ekstypes.Issue{{
					Code:        ekstypes.NodegroupIssueCodeClusterUnreachable,
					Message:     awssdk.String("control plane unreachable"),
					ResourceIds: []string{"subnet-123"},
				}}},
			}}, nil
		},
	}

	repo := &AwsRepository{EKSClient: mock}
	nodeGroups, err := repo.ListEKSNodeGroups(context.Background(), "prod-eks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodeGroups) != 2 {
		t.Fatalf("expected 2 node groups, got %d", len(nodeGroups))
	}
	if nodeGroups[0].Name != "blue" {
		t.Fatalf("expected sorted node group blue first, got %s", nodeGroups[0].Name)
	}
	if nodeGroups[0].HealthSummary() != "1 issue(s)" {
		t.Fatalf("unexpected health summary: %s", nodeGroups[0].HealthSummary())
	}
	if nodeGroups[0].HealthIssues[0].Summary() == "" {
		t.Fatal("expected health issue summary")
	}
}

func TestListEKSAddons(t *testing.T) {
	mock := &mockEKSClient{
		listAddonsFunc: func(_ context.Context, params *eks.ListAddonsInput, _ ...func(*eks.Options)) (*eks.ListAddonsOutput, error) {
			if got := awssdk.ToString(params.ClusterName); got != "prod-eks" {
				t.Fatalf("expected cluster name prod-eks, got %s", got)
			}
			return &eks.ListAddonsOutput{Addons: []string{"vpc-cni", "coredns"}}, nil
		},
		describeAddonFunc: func(_ context.Context, params *eks.DescribeAddonInput, _ ...func(*eks.Options)) (*eks.DescribeAddonOutput, error) {
			name := awssdk.ToString(params.AddonName)
			status := ekstypes.AddonStatusActive
			var issues []ekstypes.AddonIssue
			if name == "vpc-cni" {
				status = ekstypes.AddonStatusDegraded
				issues = []ekstypes.AddonIssue{{
					Code:        ekstypes.AddonIssueCodeAddonPermissionFailure,
					Message:     awssdk.String("missing IAM permission"),
					ResourceIds: []string{"aws-node"},
				}}
			}
			return &eks.DescribeAddonOutput{Addon: &ekstypes.Addon{
				ClusterName:           awssdk.String("prod-eks"),
				AddonName:             awssdk.String(name),
				AddonArn:              awssdk.String("arn:aws:eks:ap-northeast-2:123456789012:addon/prod-eks/" + name + "/uuid"),
				AddonVersion:          awssdk.String("v1.16.4-eksbuild.2"),
				Status:                status,
				Owner:                 awssdk.String("aws"),
				Publisher:             awssdk.String("eks"),
				ServiceAccountRoleArn: awssdk.String("arn:aws:iam::123456789012:role/" + name),
				Health:                &ekstypes.AddonHealth{Issues: issues},
			}}, nil
		},
	}

	repo := &AwsRepository{EKSClient: mock}
	addons, err := repo.ListEKSAddons(context.Background(), "prod-eks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(addons) != 2 {
		t.Fatalf("expected 2 add-ons, got %d", len(addons))
	}
	if addons[0].Name != "coredns" {
		t.Fatalf("expected sorted add-on coredns first, got %s", addons[0].Name)
	}
	if !addons[1].NeedsAttention() {
		t.Fatal("expected degraded add-on to need attention")
	}
	if addons[1].HealthIssues[0].Summary() == "" {
		t.Fatal("expected add-on health issue summary")
	}
}

func TestListEKSClustersReturnsError(t *testing.T) {
	mock := &mockEKSClient{
		listClustersFunc: func(_ context.Context, _ *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
		describeClusterFunc: func(_ context.Context, _ *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
			return nil, nil
		},
	}

	repo := &AwsRepository{EKSClient: mock}
	if _, err := repo.ListEKSClusters(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
