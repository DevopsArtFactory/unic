package aws

import (
	"fmt"
	"strings"
)

// ElastiCacheResource is either a replication group or a standalone cache cluster.
type ElastiCacheResource struct {
	ID            string            `json:"id"`
	Kind          string            `json:"kind"`
	Engine        string            `json:"engine"`
	EngineVersion string            `json:"engine_version"`
	Status        string            `json:"status"`
	NodeType      string            `json:"node_type"`
	Endpoint      string            `json:"endpoint"`
	Region        string            `json:"region"`
	Nodes         []ElastiCacheNode `json:"nodes"`
}

// FilterText returns a lowercase string for shared list filtering.
func (r ElastiCacheResource) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s %s",
		r.ID, r.Kind, r.Engine, r.EngineVersion, r.Status, r.NodeType, r.Endpoint))
}

// ElastiCacheNode holds node-level connection and placement metadata.
type ElastiCacheNode struct {
	ID        string `json:"id"`
	ClusterID string `json:"cluster_id"`
	ShardID   string `json:"shard_id"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	AZ        string `json:"availability_zone"`
	Endpoint  string `json:"endpoint"`
}
