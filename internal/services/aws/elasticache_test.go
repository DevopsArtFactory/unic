package aws

import (
	"context"
	"errors"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
)

type mockElastiCacheBrowserClient struct {
	describeCacheClustersFunc     func(context.Context, *elasticache.DescribeCacheClustersInput, ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error)
	describeReplicationGroupsFunc func(context.Context, *elasticache.DescribeReplicationGroupsInput, ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error)
}

func (m *mockElastiCacheBrowserClient) DescribeCacheClusters(ctx context.Context, input *elasticache.DescribeCacheClustersInput, opts ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
	return m.describeCacheClustersFunc(ctx, input, opts...)
}

func (m *mockElastiCacheBrowserClient) DescribeReplicationGroups(ctx context.Context, input *elasticache.DescribeReplicationGroupsInput, opts ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
	return m.describeReplicationGroupsFunc(ctx, input, opts...)
}

func TestListElastiCacheResourcesMapsReplicationGroupsAndStandaloneClusters(t *testing.T) {
	client := &mockElastiCacheBrowserClient{
		describeCacheClustersFunc: func(_ context.Context, input *elasticache.DescribeCacheClustersInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
			if !awssdk.ToBool(input.ShowCacheNodeInfo) {
				t.Fatal("expected cache node info to be requested")
			}
			return &elasticache.DescribeCacheClustersOutput{CacheClusters: []elasticachetypes.CacheCluster{
				{
					CacheClusterId:     awssdk.String("prod-cache-001"),
					ReplicationGroupId: awssdk.String("prod-rg"),
					Engine:             awssdk.String("valkey"),
					EngineVersion:      awssdk.String("8.0"),
					CacheNodes: []elasticachetypes.CacheNode{{
						CacheNodeId:              awssdk.String("0001"),
						CacheNodeStatus:          awssdk.String("available"),
						CustomerAvailabilityZone: awssdk.String("us-east-1a"),
						Endpoint:                 endpoint("prod-node.cache.amazonaws.com", 6379),
					}},
				},
				{
					CacheClusterId:        awssdk.String("memcached-dev"),
					CacheClusterStatus:    awssdk.String("available"),
					Engine:                awssdk.String("memcached"),
					EngineVersion:         awssdk.String("1.6.22"),
					CacheNodeType:         awssdk.String("cache.t4g.small"),
					ConfigurationEndpoint: endpoint("memcached.cfg.cache.amazonaws.com", 11211),
					CacheNodes: []elasticachetypes.CacheNode{{
						CacheNodeId:              awssdk.String("0001"),
						CacheNodeStatus:          awssdk.String("available"),
						CustomerAvailabilityZone: awssdk.String("us-east-1b"),
						Endpoint:                 endpoint("memcached-1.cache.amazonaws.com", 11211),
					}},
				},
			}}, nil
		},
		describeReplicationGroupsFunc: func(context.Context, *elasticache.DescribeReplicationGroupsInput, ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
			return &elasticache.DescribeReplicationGroupsOutput{ReplicationGroups: []elasticachetypes.ReplicationGroup{{
				ReplicationGroupId: awssdk.String("prod-rg"),
				ARN:                awssdk.String("arn:aws:elasticache:us-east-1:123:replicationgroup:prod-rg"),
				Engine:             awssdk.String("valkey"),
				Status:             awssdk.String("available"),
				CacheNodeType:      awssdk.String("cache.r7g.large"),
				NodeGroups: []elasticachetypes.NodeGroup{{
					NodeGroupId:     awssdk.String("0001"),
					PrimaryEndpoint: endpoint("prod.cache.amazonaws.com", 6379),
					Status:          awssdk.String("available"),
					NodeGroupMembers: []elasticachetypes.NodeGroupMember{{
						CacheClusterId:            awssdk.String("prod-cache-001"),
						CacheNodeId:               awssdk.String("0001"),
						CurrentRole:               awssdk.String("primary"),
						PreferredAvailabilityZone: awssdk.String("us-east-1a"),
					}},
				}},
			}}}, nil
		},
	}
	repo := &AwsRepository{ElastiCacheClient: client}

	resources, err := repo.ListElastiCacheResources(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected replication group and standalone cluster, got %+v", resources)
	}
	standalone, group := resources[0], resources[1]
	if standalone.ID != "memcached-dev" || standalone.Kind != "cluster" || standalone.Endpoint != "memcached.cfg.cache.amazonaws.com:11211" {
		t.Fatalf("unexpected standalone cluster: %+v", standalone)
	}
	if len(standalone.Nodes) != 1 || standalone.Nodes[0].Role != "node" {
		t.Fatalf("unexpected standalone nodes: %+v", standalone.Nodes)
	}
	if group.ID != "prod-rg" || group.Kind != "replication group" || group.EngineVersion != "8.0" || group.Endpoint != "prod.cache.amazonaws.com:6379" {
		t.Fatalf("unexpected replication group: %+v", group)
	}
	if len(group.Nodes) != 1 || group.Nodes[0].Status != "available" || group.Nodes[0].Endpoint != "prod-node.cache.amazonaws.com:6379" {
		t.Fatalf("unexpected replication-group nodes: %+v", group.Nodes)
	}
}

func TestListElastiCacheResourcesWrapsAPIErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cluster error
		group   error
		want    string
	}{
		{name: "clusters", cluster: errors.New("denied"), want: "describe ElastiCache clusters"},
		{name: "replication groups", group: errors.New("denied"), want: "describe ElastiCache replication groups"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &mockElastiCacheBrowserClient{
				describeCacheClustersFunc: func(context.Context, *elasticache.DescribeCacheClustersInput, ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
					if tc.cluster != nil {
						return nil, tc.cluster
					}
					return &elasticache.DescribeCacheClustersOutput{}, nil
				},
				describeReplicationGroupsFunc: func(context.Context, *elasticache.DescribeReplicationGroupsInput, ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
					if tc.group != nil {
						return nil, tc.group
					}
					return &elasticache.DescribeReplicationGroupsOutput{}, nil
				},
			}
			_, err := (&AwsRepository{ElastiCacheClient: client}).ListElastiCacheResources(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
		})
	}
}

func TestElastiCacheResourceFilterText(t *testing.T) {
	resource := ElastiCacheResource{ID: "Prod-RG", Kind: "replication group", Engine: "Valkey", Status: "Available", NodeType: "cache.r7g.large"}
	for _, want := range []string{"prod-rg", "replication group", "valkey", "available", "cache.r7g.large"} {
		if !strings.Contains(resource.FilterText(), want) {
			t.Fatalf("expected filter text to contain %q: %q", want, resource.FilterText())
		}
	}
}

func endpoint(address string, port int32) *elasticachetypes.Endpoint {
	return &elasticachetypes.Endpoint{Address: awssdk.String(address), Port: awssdk.Int32(port)}
}
