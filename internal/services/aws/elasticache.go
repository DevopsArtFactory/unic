package aws

import (
	"context"
	"fmt"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
)

// ListElastiCacheResources returns replication groups and standalone clusters,
// enriched with their cache-node status and connection endpoints.
func (r *AwsRepository) ListElastiCacheResources(ctx context.Context) ([]ElastiCacheResource, error) {
	clusters, err := r.listElastiCacheClusters(ctx)
	if err != nil {
		return nil, err
	}
	groups, err := r.listElastiCacheReplicationGroups(ctx)
	if err != nil {
		return nil, err
	}

	clustersByID := make(map[string]elasticachetypes.CacheCluster, len(clusters))
	for _, cluster := range clusters {
		clustersByID[awssdk.ToString(cluster.CacheClusterId)] = cluster
	}

	resources := make([]ElastiCacheResource, 0, len(groups)+len(clusters))
	for _, group := range groups {
		resources = append(resources, mapElastiCacheReplicationGroup(group, clustersByID))
	}
	for _, cluster := range clusters {
		if awssdk.ToString(cluster.ReplicationGroupId) != "" {
			continue
		}
		resources = append(resources, mapElastiCacheCluster(cluster))
	}

	sort.Slice(resources, func(i, j int) bool {
		left := normalizedSortKey(resources[i].ID)
		right := normalizedSortKey(resources[j].ID)
		if left == right {
			return resources[i].Kind < resources[j].Kind
		}
		return left < right
	})
	return resources, nil
}

func (r *AwsRepository) listElastiCacheClusters(ctx context.Context) ([]elasticachetypes.CacheCluster, error) {
	paginator := elasticache.NewDescribeCacheClustersPaginator(r.ElastiCacheClient, &elasticache.DescribeCacheClustersInput{
		ShowCacheNodeInfo: awssdk.Bool(true),
	})
	var clusters []elasticachetypes.CacheCluster
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe ElastiCache clusters: %w", err)
		}
		clusters = append(clusters, page.CacheClusters...)
	}
	return clusters, nil
}

func (r *AwsRepository) listElastiCacheReplicationGroups(ctx context.Context) ([]elasticachetypes.ReplicationGroup, error) {
	paginator := elasticache.NewDescribeReplicationGroupsPaginator(r.ElastiCacheClient, &elasticache.DescribeReplicationGroupsInput{})
	var groups []elasticachetypes.ReplicationGroup
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe ElastiCache replication groups: %w", err)
		}
		groups = append(groups, page.ReplicationGroups...)
	}
	return groups, nil
}

func mapElastiCacheReplicationGroup(group elasticachetypes.ReplicationGroup, clusters map[string]elasticachetypes.CacheCluster) ElastiCacheResource {
	resource := ElastiCacheResource{
		ID:       awssdk.ToString(group.ReplicationGroupId),
		Kind:     "replication group",
		Engine:   awssdk.ToString(group.Engine),
		Status:   awssdk.ToString(group.Status),
		NodeType: awssdk.ToString(group.CacheNodeType),
		Endpoint: formatElastiCacheEndpoint(group.ConfigurationEndpoint),
	}
	for _, nodeGroup := range group.NodeGroups {
		if resource.Endpoint == "" {
			resource.Endpoint = formatElastiCacheEndpoint(nodeGroup.PrimaryEndpoint)
		}
		for _, member := range nodeGroup.NodeGroupMembers {
			node := ElastiCacheNode{
				ID:        awssdk.ToString(member.CacheNodeId),
				ClusterID: awssdk.ToString(member.CacheClusterId),
				ShardID:   awssdk.ToString(nodeGroup.NodeGroupId),
				Role:      awssdk.ToString(member.CurrentRole),
				Status:    awssdk.ToString(nodeGroup.Status),
				AZ:        awssdk.ToString(member.PreferredAvailabilityZone),
				Endpoint:  formatElastiCacheEndpoint(member.ReadEndpoint),
			}
			if cluster, ok := clusters[node.ClusterID]; ok {
				if resource.EngineVersion == "" {
					resource.EngineVersion = awssdk.ToString(cluster.EngineVersion)
				}
				enrichElastiCacheNode(&node, cluster)
			}
			resource.Nodes = append(resource.Nodes, node)
		}
	}
	sortElastiCacheNodes(resource.Nodes)
	return resource
}

func mapElastiCacheCluster(cluster elasticachetypes.CacheCluster) ElastiCacheResource {
	resource := ElastiCacheResource{
		ID:            awssdk.ToString(cluster.CacheClusterId),
		Kind:          "cluster",
		Engine:        awssdk.ToString(cluster.Engine),
		EngineVersion: awssdk.ToString(cluster.EngineVersion),
		Status:        awssdk.ToString(cluster.CacheClusterStatus),
		NodeType:      awssdk.ToString(cluster.CacheNodeType),
		Endpoint:      formatElastiCacheEndpoint(cluster.ConfigurationEndpoint),
	}
	for _, cacheNode := range cluster.CacheNodes {
		role := "primary"
		if resource.Engine == "memcached" {
			role = "node"
		} else if awssdk.ToString(cacheNode.SourceCacheNodeId) != "" {
			role = "replica"
		}
		node := ElastiCacheNode{
			ID:        awssdk.ToString(cacheNode.CacheNodeId),
			ClusterID: resource.ID,
			Role:      role,
			Status:    awssdk.ToString(cacheNode.CacheNodeStatus),
			AZ:        awssdk.ToString(cacheNode.CustomerAvailabilityZone),
			Endpoint:  formatElastiCacheEndpoint(cacheNode.Endpoint),
		}
		resource.Nodes = append(resource.Nodes, node)
		if resource.Endpoint == "" && node.Endpoint != "" {
			resource.Endpoint = node.Endpoint
		}
	}
	sortElastiCacheNodes(resource.Nodes)
	return resource
}

func enrichElastiCacheNode(node *ElastiCacheNode, cluster elasticachetypes.CacheCluster) {
	for _, cacheNode := range cluster.CacheNodes {
		if awssdk.ToString(cacheNode.CacheNodeId) != node.ID {
			continue
		}
		if status := awssdk.ToString(cacheNode.CacheNodeStatus); status != "" {
			node.Status = status
		}
		if az := awssdk.ToString(cacheNode.CustomerAvailabilityZone); az != "" {
			node.AZ = az
		}
		if endpoint := formatElastiCacheEndpoint(cacheNode.Endpoint); endpoint != "" {
			node.Endpoint = endpoint
		}
		return
	}
}

func sortElastiCacheNodes(nodes []ElastiCacheNode) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].ClusterID == nodes[j].ClusterID {
			return nodes[i].ID < nodes[j].ID
		}
		return normalizedSortKey(nodes[i].ClusterID) < normalizedSortKey(nodes[j].ClusterID)
	})
}

func formatElastiCacheEndpoint(endpoint *elasticachetypes.Endpoint) string {
	if endpoint == nil || awssdk.ToString(endpoint.Address) == "" {
		return ""
	}
	if port := awssdk.ToInt32(endpoint.Port); port > 0 {
		return fmt.Sprintf("%s:%d", awssdk.ToString(endpoint.Address), port)
	}
	return awssdk.ToString(endpoint.Address)
}
