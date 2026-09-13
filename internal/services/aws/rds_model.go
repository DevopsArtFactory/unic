package aws

import (
	"fmt"
	"strings"
)

// RDSInstance holds essential information about an RDS database instance.
type RDSInstance struct {
	DBInstanceID          string `json:"db_instance_id"`
	Engine                string `json:"engine"`
	EngineVersion         string `json:"engine_version"`
	Status                string `json:"status"`
	InstanceClass         string `json:"instance_class"`
	PendingInstanceClass  string `json:"pending_instance_class"`
	Region                string `json:"region"`
	MultiAZ               bool   `json:"multi_az"`
	StorageGB             int32  `json:"storage_gb"`
	StorageEncrypted      bool   `json:"storage_encrypted"`
	PubliclyAccessible    bool   `json:"publicly_accessible"`
	BackupRetentionPeriod int32  `json:"backup_retention_period"`
	Endpoint              string `json:"endpoint"`
	ClusterID             string `json:"cluster_id"`
}

// DisplayTitle returns a formatted string for list display.
func (r RDSInstance) DisplayTitle() string {
	return fmt.Sprintf("%s (%s) %s %s [%s]",
		r.DBInstanceID, r.InstanceClass, r.Engine, r.EngineVersion, r.Status)
}

// FilterText returns a lowercase string for keyword matching.
func (r RDSInstance) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s %s",
		r.DBInstanceID, r.Engine, r.EngineVersion, r.Status, r.InstanceClass, r.ClusterID, r.Region))
}

// IsClusterMember returns true if this instance belongs to an Aurora cluster.
func (r RDSInstance) IsClusterMember() bool {
	return r.ClusterID != ""
}

// CanStop returns true if the instance (or its cluster) can be stopped.
func (r RDSInstance) CanStop() bool {
	if r.IsClusterMember() {
		return r.Status == "available"
	}
	return r.Status == "available"
}

// CanStart returns true if the instance (or its cluster) can be started.
func (r RDSInstance) CanStart() bool {
	return r.Status == "stopped"
}

// CanFailover returns true if the instance supports failover.
// Aurora cluster members use cluster-level failover; standalone instances need Multi-AZ.
func (r RDSInstance) CanFailover() bool {
	if r.IsClusterMember() {
		return r.Status == "available"
	}
	return r.Status == "available" && r.MultiAZ
}

// IsTransitionalStatus returns true if the instance is in a transitional state.
func IsTransitionalStatus(status string) bool {
	switch status {
	case "starting", "stopping", "rebooting", "modifying", "backing-up",
		"configuring-enhanced-monitoring", "maintenance", "renaming",
		"resetting-master-credentials":
		return true
	}
	return false
}
