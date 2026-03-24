package aws

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

// ListDBInstances returns all RDS DB instances in the current account/region.
func (r *AwsRepository) ListDBInstances(ctx context.Context) ([]RDSInstance, error) {
	output, err := r.RDSClient.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe DB instances: %w", err)
	}

	instances := make([]RDSInstance, 0, len(output.DBInstances))
	for _, db := range output.DBInstances {
		inst := RDSInstance{
			DBInstanceID:  awssdk.ToString(db.DBInstanceIdentifier),
			Engine:        awssdk.ToString(db.Engine),
			EngineVersion: awssdk.ToString(db.EngineVersion),
			Status:        awssdk.ToString(db.DBInstanceStatus),
			InstanceClass: awssdk.ToString(db.DBInstanceClass),
			MultiAZ:       awssdk.ToBool(db.MultiAZ),
			StorageGB:     awssdk.ToInt32(db.AllocatedStorage),
			ClusterID:     awssdk.ToString(db.DBClusterIdentifier),
		}

		// Endpoint may be nil for stopped instances
		if db.Endpoint != nil {
			inst.Endpoint = fmt.Sprintf("%s:%d", awssdk.ToString(db.Endpoint.Address), awssdk.ToInt32(db.Endpoint.Port))
		}

		instances = append(instances, inst)
	}
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
		DBInstanceID:  awssdk.ToString(db.DBInstanceIdentifier),
		Engine:        awssdk.ToString(db.Engine),
		EngineVersion: awssdk.ToString(db.EngineVersion),
		Status:        awssdk.ToString(db.DBInstanceStatus),
		InstanceClass: awssdk.ToString(db.DBInstanceClass),
		MultiAZ:       awssdk.ToBool(db.MultiAZ),
		StorageGB:     awssdk.ToInt32(db.AllocatedStorage),
		ClusterID:     awssdk.ToString(db.DBClusterIdentifier),
	}
	if db.Endpoint != nil {
		inst.Endpoint = fmt.Sprintf("%s:%d", awssdk.ToString(db.Endpoint.Address), awssdk.ToInt32(db.Endpoint.Port))
	}
	return inst, nil
}

// StopDBInstance stops a running RDS instance.
func (r *AwsRepository) StopDBInstance(ctx context.Context, dbInstanceID string) error {
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
	_, err := r.RDSClient.RebootDBInstance(ctx, &rds.RebootDBInstanceInput{
		DBInstanceIdentifier: awssdk.String(dbInstanceID),
		ForceFailover:        awssdk.Bool(forceFailover),
	})
	if err != nil {
		return fmt.Errorf("failed to reboot DB instance %s: %w", dbInstanceID, err)
	}
	return nil
}
