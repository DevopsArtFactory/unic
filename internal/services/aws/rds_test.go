package aws

import (
	"context"
	"fmt"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// mockRDSClient implements RDSClientAPI for testing.
type mockRDSClient struct {
	describeDBInstancesFunc            func(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
	describeDBSnapshotsFunc            func(ctx context.Context, params *rds.DescribeDBSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error)
	describeDBSnapshotAttributesFunc   func(ctx context.Context, params *rds.DescribeDBSnapshotAttributesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error)
	describeDBClusterSnapshotsFunc     func(ctx context.Context, params *rds.DescribeDBClusterSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error)
	describeDBClusterSnapshotAttrsFunc func(ctx context.Context, params *rds.DescribeDBClusterSnapshotAttributesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error)
	stopDBInstanceFunc                 func(ctx context.Context, params *rds.StopDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StopDBInstanceOutput, error)
	startDBInstanceFunc                func(ctx context.Context, params *rds.StartDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StartDBInstanceOutput, error)
	rebootDBInstanceFunc               func(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error)
	stopDBClusterFunc                  func(ctx context.Context, params *rds.StopDBClusterInput, optFns ...func(*rds.Options)) (*rds.StopDBClusterOutput, error)
	startDBClusterFunc                 func(ctx context.Context, params *rds.StartDBClusterInput, optFns ...func(*rds.Options)) (*rds.StartDBClusterOutput, error)
	failoverDBClusterFunc              func(ctx context.Context, params *rds.FailoverDBClusterInput, optFns ...func(*rds.Options)) (*rds.FailoverDBClusterOutput, error)
}

func (m *mockRDSClient) DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return m.describeDBInstancesFunc(ctx, params, optFns...)
}

func (m *mockRDSClient) DescribeDBSnapshots(ctx context.Context, params *rds.DescribeDBSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
	if m.describeDBSnapshotsFunc != nil {
		return m.describeDBSnapshotsFunc(ctx, params, optFns...)
	}
	return &rds.DescribeDBSnapshotsOutput{}, nil
}

func (m *mockRDSClient) DescribeDBSnapshotAttributes(ctx context.Context, params *rds.DescribeDBSnapshotAttributesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error) {
	if m.describeDBSnapshotAttributesFunc != nil {
		return m.describeDBSnapshotAttributesFunc(ctx, params, optFns...)
	}
	return &rds.DescribeDBSnapshotAttributesOutput{}, nil
}

func (m *mockRDSClient) DescribeDBClusterSnapshots(ctx context.Context, params *rds.DescribeDBClusterSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	if m.describeDBClusterSnapshotsFunc != nil {
		return m.describeDBClusterSnapshotsFunc(ctx, params, optFns...)
	}
	return &rds.DescribeDBClusterSnapshotsOutput{}, nil
}

func (m *mockRDSClient) DescribeDBClusterSnapshotAttributes(ctx context.Context, params *rds.DescribeDBClusterSnapshotAttributesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error) {
	if m.describeDBClusterSnapshotAttrsFunc != nil {
		return m.describeDBClusterSnapshotAttrsFunc(ctx, params, optFns...)
	}
	return &rds.DescribeDBClusterSnapshotAttributesOutput{}, nil
}

func (m *mockRDSClient) StopDBInstance(ctx context.Context, params *rds.StopDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StopDBInstanceOutput, error) {
	if m.stopDBInstanceFunc != nil {
		return m.stopDBInstanceFunc(ctx, params, optFns...)
	}
	return &rds.StopDBInstanceOutput{}, nil
}

func (m *mockRDSClient) StartDBInstance(ctx context.Context, params *rds.StartDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StartDBInstanceOutput, error) {
	if m.startDBInstanceFunc != nil {
		return m.startDBInstanceFunc(ctx, params, optFns...)
	}
	return &rds.StartDBInstanceOutput{}, nil
}

func (m *mockRDSClient) RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error) {
	if m.rebootDBInstanceFunc != nil {
		return m.rebootDBInstanceFunc(ctx, params, optFns...)
	}
	return &rds.RebootDBInstanceOutput{}, nil
}

func (m *mockRDSClient) StopDBCluster(ctx context.Context, params *rds.StopDBClusterInput, optFns ...func(*rds.Options)) (*rds.StopDBClusterOutput, error) {
	if m.stopDBClusterFunc != nil {
		return m.stopDBClusterFunc(ctx, params, optFns...)
	}
	return &rds.StopDBClusterOutput{}, nil
}

func (m *mockRDSClient) StartDBCluster(ctx context.Context, params *rds.StartDBClusterInput, optFns ...func(*rds.Options)) (*rds.StartDBClusterOutput, error) {
	if m.startDBClusterFunc != nil {
		return m.startDBClusterFunc(ctx, params, optFns...)
	}
	return &rds.StartDBClusterOutput{}, nil
}

func (m *mockRDSClient) FailoverDBCluster(ctx context.Context, params *rds.FailoverDBClusterInput, optFns ...func(*rds.Options)) (*rds.FailoverDBClusterOutput, error) {
	if m.failoverDBClusterFunc != nil {
		return m.failoverDBClusterFunc(ctx, params, optFns...)
	}
	return &rds.FailoverDBClusterOutput{}, nil
}

// --- ListDBInstances tests ---

func TestListDBInstances_Success(t *testing.T) {
	mock := &mockRDSClient{
		describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier: awssdk.String("my-db"),
						Engine:               awssdk.String("mysql"),
						EngineVersion:        awssdk.String("8.0.35"),
						DBInstanceStatus:     awssdk.String("available"),
						DBInstanceClass:      awssdk.String("db.t3.micro"),
						MultiAZ:              awssdk.Bool(true),
						AllocatedStorage:     awssdk.Int32(20),
						Endpoint: &rdstypes.Endpoint{
							Address: awssdk.String("my-db.abc123.us-east-1.rds.amazonaws.com"),
							Port:    awssdk.Int32(3306),
						},
					},
					{
						DBInstanceIdentifier: awssdk.String("aurora-inst-1"),
						Engine:               awssdk.String("aurora-mysql"),
						EngineVersion:        awssdk.String("8.0.mysql_aurora.3.04.0"),
						DBInstanceStatus:     awssdk.String("available"),
						DBInstanceClass:      awssdk.String("db.r6g.large"),
						MultiAZ:              awssdk.Bool(false),
						AllocatedStorage:     awssdk.Int32(0),
						DBClusterIdentifier:  awssdk.String("my-cluster"),
						Endpoint: &rdstypes.Endpoint{
							Address: awssdk.String("aurora-inst-1.abc123.us-east-1.rds.amazonaws.com"),
							Port:    awssdk.Int32(3306),
						},
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{RDSClient: mock}
	instances, err := repo.ListDBInstances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	inst := instances[0]
	if inst.DBInstanceID != "aurora-inst-1" {
		t.Errorf("expected DBInstanceID 'aurora-inst-1', got %q", inst.DBInstanceID)
	}
	if inst.Engine != "aurora-mysql" {
		t.Errorf("expected Engine 'aurora-mysql', got %q", inst.Engine)
	}
	if inst.EngineVersion != "8.0.mysql_aurora.3.04.0" {
		t.Errorf("expected EngineVersion '8.0.mysql_aurora.3.04.0', got %q", inst.EngineVersion)
	}
	if inst.Status != "available" {
		t.Errorf("expected Status 'available', got %q", inst.Status)
	}
	if inst.InstanceClass != "db.r6g.large" {
		t.Errorf("expected InstanceClass 'db.r6g.large', got %q", inst.InstanceClass)
	}
	if inst.MultiAZ {
		t.Error("expected MultiAZ to be false")
	}
	if inst.StorageGB != 0 {
		t.Errorf("expected StorageGB 0, got %d", inst.StorageGB)
	}
	if inst.Endpoint != "aurora-inst-1.abc123.us-east-1.rds.amazonaws.com:3306" {
		t.Errorf("unexpected Endpoint: %q", inst.Endpoint)
	}
	if inst.ClusterID != "my-cluster" {
		t.Errorf("expected ClusterID 'my-cluster', got %q", inst.ClusterID)
	}

	mysql := instances[1]
	if mysql.DBInstanceID != "my-db" {
		t.Errorf("expected DBInstanceID 'my-db', got %q", mysql.DBInstanceID)
	}
	if mysql.ClusterID != "" {
		t.Errorf("expected empty ClusterID, got %q", mysql.ClusterID)
	}
}

func TestListDBInstances_Empty(t *testing.T) {
	mock := &mockRDSClient{
		describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{DBInstances: []rdstypes.DBInstance{}}, nil
		},
	}

	repo := &AwsRepository{RDSClient: mock}
	instances, err := repo.ListDBInstances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 0 {
		t.Errorf("expected empty slice, got %d", len(instances))
	}
}

func TestListDBInstances_Error(t *testing.T) {
	mock := &mockRDSClient{
		describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}

	repo := &AwsRepository{RDSClient: mock}
	_, err := repo.ListDBInstances(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListDBInstances_NilEndpoint(t *testing.T) {
	mock := &mockRDSClient{
		describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier: awssdk.String("stopped-db"),
						Engine:               awssdk.String("postgres"),
						EngineVersion:        awssdk.String("15.4"),
						DBInstanceStatus:     awssdk.String("stopped"),
						DBInstanceClass:      awssdk.String("db.t3.small"),
						MultiAZ:              awssdk.Bool(false),
						AllocatedStorage:     awssdk.Int32(50),
						Endpoint:             nil,
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{RDSClient: mock}
	instances, err := repo.ListDBInstances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	if instances[0].Endpoint != "" {
		t.Errorf("expected empty Endpoint for stopped instance, got %q", instances[0].Endpoint)
	}
}

func TestListDBInstances_SortedByIdentifier(t *testing.T) {
	mock := &mockRDSClient{
		describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{DBInstanceIdentifier: awssdk.String("zeta-db")},
					{DBInstanceIdentifier: awssdk.String("alpha-db")},
				},
			}, nil
		},
	}

	repo := &AwsRepository{RDSClient: mock}
	instances, err := repo.ListDBInstances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}
	if instances[0].DBInstanceID != "alpha-db" || instances[1].DBInstanceID != "zeta-db" {
		t.Fatalf("expected alphabetical DB identifier order, got %+v", instances)
	}
}

// --- DescribeDBInstance tests ---

func TestDescribeDBInstance_Success(t *testing.T) {
	mock := &mockRDSClient{
		describeDBInstancesFunc: func(_ context.Context, params *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			if awssdk.ToString(params.DBInstanceIdentifier) != "my-db" {
				t.Errorf("expected identifier 'my-db', got %q", awssdk.ToString(params.DBInstanceIdentifier))
			}
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier:  awssdk.String("my-db"),
						Engine:                awssdk.String("mysql"),
						EngineVersion:         awssdk.String("8.0.35"),
						DBInstanceStatus:      awssdk.String("available"),
						DBInstanceClass:       awssdk.String("db.t3.micro"),
						MultiAZ:               awssdk.Bool(false),
						AllocatedStorage:      awssdk.Int32(20),
						StorageEncrypted:      awssdk.Bool(true),
						PubliclyAccessible:    awssdk.Bool(true),
						BackupRetentionPeriod: awssdk.Int32(7),
						Endpoint: &rdstypes.Endpoint{
							Address: awssdk.String("my-db.abc123.us-east-1.rds.amazonaws.com"),
							Port:    awssdk.Int32(3306),
						},
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{RDSClient: mock}
	inst, err := repo.DescribeDBInstance(context.Background(), "my-db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.DBInstanceID != "my-db" {
		t.Errorf("expected 'my-db', got %q", inst.DBInstanceID)
	}
	if !inst.StorageEncrypted {
		t.Error("expected storage encryption to be mapped")
	}
	if !inst.PubliclyAccessible {
		t.Error("expected public accessibility to be mapped")
	}
	if inst.BackupRetentionPeriod != 7 {
		t.Errorf("expected backup retention period 7, got %d", inst.BackupRetentionPeriod)
	}
}

func TestListDBInstances_MapsInspectorFields(t *testing.T) {
	mock := &mockRDSClient{
		describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier:  awssdk.String("my-db"),
						StorageEncrypted:      awssdk.Bool(true),
						PubliclyAccessible:    awssdk.Bool(false),
						BackupRetentionPeriod: awssdk.Int32(14),
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{RDSClient: mock}
	instances, err := repo.ListDBInstances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	if !instances[0].StorageEncrypted {
		t.Error("expected storage encryption to be mapped")
	}
	if instances[0].PubliclyAccessible {
		t.Error("expected public accessibility to be false")
	}
	if instances[0].BackupRetentionPeriod != 14 {
		t.Errorf("expected backup retention period 14, got %d", instances[0].BackupRetentionPeriod)
	}
}

// --- StopDBInstance tests ---

func TestStopDBInstance_Success(t *testing.T) {
	var capturedID string
	mock := &mockRDSClient{
		describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{}, nil
		},
		stopDBInstanceFunc: func(_ context.Context, params *rds.StopDBInstanceInput, _ ...func(*rds.Options)) (*rds.StopDBInstanceOutput, error) {
			capturedID = awssdk.ToString(params.DBInstanceIdentifier)
			return &rds.StopDBInstanceOutput{}, nil
		},
	}

	repo := &AwsRepository{RDSClient: mock}
	err := repo.StopDBInstance(context.Background(), "my-db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID != "my-db" {
		t.Errorf("expected stop to be called with 'my-db', got %q", capturedID)
	}
}

func TestStopDBInstance_Error(t *testing.T) {
	mock := &mockRDSClient{
		describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{}, nil
		},
		stopDBInstanceFunc: func(_ context.Context, _ *rds.StopDBInstanceInput, _ ...func(*rds.Options)) (*rds.StopDBInstanceOutput, error) {
			return nil, fmt.Errorf("cannot stop Aurora cluster member")
		},
	}

	repo := &AwsRepository{RDSClient: mock}
	err := repo.StopDBInstance(context.Background(), "aurora-inst")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- StartDBInstance tests ---

func TestStartDBInstance_Success(t *testing.T) {
	var capturedID string
	mock := &mockRDSClient{
		describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{}, nil
		},
		startDBInstanceFunc: func(_ context.Context, params *rds.StartDBInstanceInput, _ ...func(*rds.Options)) (*rds.StartDBInstanceOutput, error) {
			capturedID = awssdk.ToString(params.DBInstanceIdentifier)
			return &rds.StartDBInstanceOutput{}, nil
		},
	}

	repo := &AwsRepository{RDSClient: mock}
	err := repo.StartDBInstance(context.Background(), "my-db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID != "my-db" {
		t.Errorf("expected start to be called with 'my-db', got %q", capturedID)
	}
}

func TestStartDBInstance_Error(t *testing.T) {
	mock := &mockRDSClient{
		describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{}, nil
		},
		startDBInstanceFunc: func(_ context.Context, _ *rds.StartDBInstanceInput, _ ...func(*rds.Options)) (*rds.StartDBInstanceOutput, error) {
			return nil, fmt.Errorf("instance not in stopped state")
		},
	}

	repo := &AwsRepository{RDSClient: mock}
	err := repo.StartDBInstance(context.Background(), "my-db")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- RebootDBInstance tests ---

func TestRebootDBInstance_ForceFailover(t *testing.T) {
	var capturedFailover bool
	mock := &mockRDSClient{
		describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{}, nil
		},
		rebootDBInstanceFunc: func(_ context.Context, params *rds.RebootDBInstanceInput, _ ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error) {
			capturedFailover = awssdk.ToBool(params.ForceFailover)
			return &rds.RebootDBInstanceOutput{}, nil
		},
	}

	repo := &AwsRepository{RDSClient: mock}
	err := repo.RebootDBInstance(context.Background(), "my-db", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capturedFailover {
		t.Error("expected ForceFailover to be true")
	}
}

// --- Model tests ---

func TestRDSInstanceDisplayTitle(t *testing.T) {
	inst := RDSInstance{
		DBInstanceID: "my-db", InstanceClass: "db.t3.micro",
		Engine: "mysql", EngineVersion: "8.0.35", Status: "available",
	}
	expected := "my-db (db.t3.micro) mysql 8.0.35 [available]"
	if got := inst.DisplayTitle(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestRDSInstanceFilterText(t *testing.T) {
	inst := RDSInstance{
		DBInstanceID: "MyDB", Engine: "MySQL", EngineVersion: "8.0",
		Status: "Available", InstanceClass: "db.t3.micro", ClusterID: "MyCluster",
	}
	ft := inst.FilterText()
	for _, kw := range []string{"mydb", "mysql", "8.0", "available", "db.t3.micro", "mycluster"} {
		if !strings.Contains(ft, kw) {
			t.Errorf("FilterText %q should contain %q", ft, kw)
		}
	}
}

func TestCanStart_StoppedInstance(t *testing.T) {
	inst := RDSInstance{Status: "stopped"}
	if !inst.CanStart() {
		t.Error("stopped instance should be startable")
	}
}

func TestCanStart_RunningInstance(t *testing.T) {
	inst := RDSInstance{Status: "available"}
	if inst.CanStart() {
		t.Error("available instance should not be startable")
	}
}

func TestCanStop_RunningInstance(t *testing.T) {
	inst := RDSInstance{Status: "available", ClusterID: ""}
	if !inst.CanStop() {
		t.Error("available standalone instance should be stoppable")
	}
}

func TestCanStop_ClusterMember(t *testing.T) {
	inst := RDSInstance{Status: "available", ClusterID: "my-cluster"}
	if !inst.CanStop() {
		t.Error("available cluster member should be stoppable (cluster-level stop)")
	}
}

func TestCanStop_StoppedInstance(t *testing.T) {
	inst := RDSInstance{Status: "stopped", ClusterID: ""}
	if inst.CanStop() {
		t.Error("stopped instance should not be stoppable")
	}
}

func TestCanFailover_MultiAZ(t *testing.T) {
	inst := RDSInstance{Status: "available", MultiAZ: true}
	if !inst.CanFailover() {
		t.Error("available Multi-AZ instance should support failover")
	}
}

func TestCanFailover_SingleAZ(t *testing.T) {
	// Standalone single-AZ instance cannot failover
	inst := RDSInstance{Status: "available", MultiAZ: false, ClusterID: ""}
	if inst.CanFailover() {
		t.Error("standalone single-AZ instance should not support failover")
	}
}

func TestCanFailover_ClusterMember(t *testing.T) {
	// Aurora cluster member can failover even without MultiAZ on the instance
	inst := RDSInstance{Status: "available", MultiAZ: false, ClusterID: "my-cluster"}
	if !inst.CanFailover() {
		t.Error("Aurora cluster member should support failover")
	}
}

func TestCanFailover_StoppedMultiAZ(t *testing.T) {
	inst := RDSInstance{Status: "stopped", MultiAZ: true}
	if inst.CanFailover() {
		t.Error("stopped Multi-AZ instance should not support failover")
	}
}

func TestIsTransitionalStatus(t *testing.T) {
	transitional := []string{"starting", "stopping", "rebooting", "modifying", "backing-up"}
	for _, s := range transitional {
		if !IsTransitionalStatus(s) {
			t.Errorf("%q should be transitional", s)
		}
	}

	stable := []string{"available", "stopped", "failed"}
	for _, s := range stable {
		if IsTransitionalStatus(s) {
			t.Errorf("%q should not be transitional", s)
		}
	}
}
