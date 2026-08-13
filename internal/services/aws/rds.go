package aws

import (
	"context"
	"fmt"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"

	uniclog "unic/internal/log"
)

// ListDBInstances returns all RDS DB instances in the current account/region.
func (r *AwsRepository) ListDBInstances(ctx context.Context) ([]RDSInstance, error) {
	uniclog.Debug("aws", "ListDBInstances called")
	output, err := r.RDSClient.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe DB instances: %w", err)
	}

	instances := make([]RDSInstance, 0, len(output.DBInstances))
	for _, db := range output.DBInstances {
		inst := RDSInstance{
			DBInstanceID:          awssdk.ToString(db.DBInstanceIdentifier),
			Engine:                awssdk.ToString(db.Engine),
			EngineVersion:         awssdk.ToString(db.EngineVersion),
			Status:                awssdk.ToString(db.DBInstanceStatus),
			InstanceClass:         awssdk.ToString(db.DBInstanceClass),
			MultiAZ:               awssdk.ToBool(db.MultiAZ),
			StorageGB:             awssdk.ToInt32(db.AllocatedStorage),
			StorageEncrypted:      awssdk.ToBool(db.StorageEncrypted),
			PubliclyAccessible:    awssdk.ToBool(db.PubliclyAccessible),
			BackupRetentionPeriod: awssdk.ToInt32(db.BackupRetentionPeriod),
			ClusterID:             awssdk.ToString(db.DBClusterIdentifier),
		}

		// Endpoint may be nil for stopped instances
		if db.Endpoint != nil {
			inst.Endpoint = fmt.Sprintf("%s:%d", awssdk.ToString(db.Endpoint.Address), awssdk.ToInt32(db.Endpoint.Port))
		}

		instances = append(instances, inst)
	}
	sort.Slice(instances, func(i, j int) bool {
		left := normalizedSortKey(instances[i].DBInstanceID)
		right := normalizedSortKey(instances[j].DBInstanceID)
		if left == right {
			return instances[i].Endpoint < instances[j].Endpoint
		}
		return left < right
	})
	return instances, nil
}

// DescribeDBInstance returns a single refreshed RDS instance by identifier.
func (r *AwsRepository) DescribeDBInstance(ctx context.Context, dbInstanceID string) (*RDSInstance, error) {
	output, err := r.RDSClient.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: awssdk.String(dbInstanceID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe DB instance %s: %w", dbInstanceID, err)
	}

	if len(output.DBInstances) == 0 {
		return nil, fmt.Errorf("DB instance %s not found", dbInstanceID)
	}

	db := output.DBInstances[0]
	inst := &RDSInstance{
		DBInstanceID:          awssdk.ToString(db.DBInstanceIdentifier),
		Engine:                awssdk.ToString(db.Engine),
		EngineVersion:         awssdk.ToString(db.EngineVersion),
		Status:                awssdk.ToString(db.DBInstanceStatus),
		InstanceClass:         awssdk.ToString(db.DBInstanceClass),
		MultiAZ:               awssdk.ToBool(db.MultiAZ),
		StorageGB:             awssdk.ToInt32(db.AllocatedStorage),
		StorageEncrypted:      awssdk.ToBool(db.StorageEncrypted),
		PubliclyAccessible:    awssdk.ToBool(db.PubliclyAccessible),
		BackupRetentionPeriod: awssdk.ToInt32(db.BackupRetentionPeriod),
		ClusterID:             awssdk.ToString(db.DBClusterIdentifier),
	}
	if db.Endpoint != nil {
		inst.Endpoint = fmt.Sprintf("%s:%d", awssdk.ToString(db.Endpoint.Address), awssdk.ToInt32(db.Endpoint.Port))
	}
	return inst, nil
}

// StopDBInstance stops a running RDS instance.
func (r *AwsRepository) StopDBInstance(ctx context.Context, dbInstanceID string) error {
	uniclog.Info("aws", "StopDBInstance called", "instance", dbInstanceID)
	_, err := r.RDSClient.StopDBInstance(ctx, &rds.StopDBInstanceInput{
		DBInstanceIdentifier: awssdk.String(dbInstanceID),
	})
	if err != nil {
		return fmt.Errorf("failed to stop DB instance %s: %w", dbInstanceID, err)
	}
	return nil
}

// StartDBInstance starts a stopped RDS instance.
func (r *AwsRepository) StartDBInstance(ctx context.Context, dbInstanceID string) error {
	uniclog.Info("aws", "StartDBInstance called", "instance", dbInstanceID)
	_, err := r.RDSClient.StartDBInstance(ctx, &rds.StartDBInstanceInput{
		DBInstanceIdentifier: awssdk.String(dbInstanceID),
	})
	if err != nil {
		return fmt.Errorf("failed to start DB instance %s: %w", dbInstanceID, err)
	}
	return nil
}

// RebootDBInstance reboots an RDS instance. If forceFailover is true,
// a Multi-AZ failover is triggered.
func (r *AwsRepository) RebootDBInstance(ctx context.Context, dbInstanceID string, forceFailover bool) error {
	uniclog.Info("aws", "RebootDBInstance called", "instance", dbInstanceID, "force_failover", forceFailover)
	_, err := r.RDSClient.RebootDBInstance(ctx, &rds.RebootDBInstanceInput{
		DBInstanceIdentifier: awssdk.String(dbInstanceID),
		ForceFailover:        awssdk.Bool(forceFailover),
	})
	if err != nil {
		return fmt.Errorf("failed to reboot DB instance %s: %w", dbInstanceID, err)
	}
	return nil
}

// StopDBCluster stops an Aurora DB cluster.
func (r *AwsRepository) StopDBCluster(ctx context.Context, clusterID string) error {
	uniclog.Info("aws", "StopDBCluster called", "cluster", clusterID)
	_, err := r.RDSClient.StopDBCluster(ctx, &rds.StopDBClusterInput{
		DBClusterIdentifier: awssdk.String(clusterID),
	})
	if err != nil {
		return fmt.Errorf("failed to stop DB cluster %s: %w", clusterID, err)
	}
	return nil
}

// StartDBCluster starts a stopped Aurora DB cluster.
func (r *AwsRepository) StartDBCluster(ctx context.Context, clusterID string) error {
	uniclog.Info("aws", "StartDBCluster called", "cluster", clusterID)
	_, err := r.RDSClient.StartDBCluster(ctx, &rds.StartDBClusterInput{
		DBClusterIdentifier: awssdk.String(clusterID),
	})
	if err != nil {
		return fmt.Errorf("failed to start DB cluster %s: %w", clusterID, err)
	}
	return nil
}

// FailoverDBCluster triggers a failover for an Aurora DB cluster.
func (r *AwsRepository) FailoverDBCluster(ctx context.Context, clusterID string) error {
	uniclog.Info("aws", "FailoverDBCluster called", "cluster", clusterID)
	_, err := r.RDSClient.FailoverDBCluster(ctx, &rds.FailoverDBClusterInput{
		DBClusterIdentifier: awssdk.String(clusterID),
	})
	if err != nil {
		return fmt.Errorf("failed to failover DB cluster %s: %w", clusterID, err)
	}
	return nil
}

// ListOrderableDBInstanceClasses returns the distinct DB instance classes
// orderable for the given engine (and optional engine version) in the active
// region, sorted alphabetically.
func (r *AwsRepository) ListOrderableDBInstanceClasses(ctx context.Context, engine, engineVersion string) ([]string, error) {
	uniclog.Debug("aws", "ListOrderableDBInstanceClasses called", "engine", engine, "engine_version", engineVersion)

	input := &rds.DescribeOrderableDBInstanceOptionsInput{Engine: awssdk.String(engine)}
	if engineVersion != "" {
		input.EngineVersion = awssdk.String(engineVersion)
	}

	seen := make(map[string]struct{})
	var classes []string
	paginator := rds.NewDescribeOrderableDBInstanceOptionsPaginator(r.RDSClient, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list orderable DB instance classes: %w", err)
		}
		for _, option := range page.OrderableDBInstanceOptions {
			class := awssdk.ToString(option.DBInstanceClass)
			if class == "" {
				continue
			}
			if _, ok := seen[class]; ok {
				continue
			}
			seen[class] = struct{}{}
			classes = append(classes, class)
		}
	}
	sort.Strings(classes)
	return classes, nil
}

// ModifyDBInstanceClass changes the DB instance class of an instance.
func (r *AwsRepository) ModifyDBInstanceClass(ctx context.Context, dbInstanceID, instanceClass string, applyImmediately bool) error {
	uniclog.Info("aws", "ModifyDBInstanceClass called", "instance", dbInstanceID, "class", instanceClass, "apply_immediately", applyImmediately)
	_, err := r.RDSClient.ModifyDBInstance(ctx, &rds.ModifyDBInstanceInput{
		DBInstanceIdentifier: awssdk.String(dbInstanceID),
		DBInstanceClass:      awssdk.String(instanceClass),
		ApplyImmediately:     awssdk.Bool(applyImmediately),
	})
	if err != nil {
		return fmt.Errorf("failed to modify DB instance class for %s: %w", dbInstanceID, err)
	}
	return nil
}
