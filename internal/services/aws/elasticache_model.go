package aws

import (
	"fmt"
	"strings"
)

// ElastiCacheResource is either a replication group or a standalone cache cluster.
type ElastiCacheResource struct {
	ID            string
	Kind          string
	Engine        string
	EngineVersion string
	Status        string
	NodeType      string
	Endpoint      string
	Nodes         []ElastiCacheNode
}

// FilterText returns a lowercase string for shared list filtering.
func (r ElastiCacheResource) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s %s",
		r.ID, r.Kind, r.Engine, r.EngineVersion, r.Status, r.NodeType, r.Endpoint))
}

// ElastiCacheNode holds node-level connection and placement metadata.
type ElastiCacheNode struct {
	ID        string
	ClusterID string
	ShardID   string
	Role      string
	Status    string
	AZ        string
	Endpoint  string
}
