package inspector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

func TestLoadChecklistParsesSampleFile(t *testing.T) {
	path := filepath.Join("testdata", "checklists", "readiness.yaml")

	checklist, err := LoadChecklist(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checklist.SourcePath != path {
		t.Fatalf("expected source path %q, got %q", path, checklist.SourcePath)
	}
	if checklist.DisplayName() != "Production Readiness" {
		t.Fatalf("unexpected checklist name: %q", checklist.DisplayName())
	}
	if len(checklist.Checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(checklist.Checks))
	}
	if checklist.Checks[0].Type != ChecklistCheckRDS {
		t.Fatalf("expected first check to be RDS, got %s", checklist.Checks[0].Type)
	}
	if checklist.Checks[1].Type != ChecklistCheckSecurityGroup {
		t.Fatalf("expected second check to be security_group, got %s", checklist.Checks[1].Type)
	}
	if checklist.Checks[2].Expect.RotationEnabled == nil || !*checklist.Checks[2].Expect.RotationEnabled {
		t.Fatalf("expected secret rotation expectation to be true, got %+v", checklist.Checks[2].Expect)
	}
}

func TestLoadChecklistRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.yaml")
	content := strings.Join([]string{
		"name: Invalid",
		"checks:",
		"  - type: rds",
		"    resource: prod-db",
		"    unexpected: true",
		"    expect:",
		"      publicly_accessible: false",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write checklist: %v", err)
	}

	_, err := LoadChecklist(path)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("expected unknown field mention, got %v", err)
	}
}

func TestRunChecklistReportsPassAndFailPerCheck(t *testing.T) {
	checklist, err := LoadChecklist(filepath.Join("testdata", "checklists", "readiness.yaml"))
	if err != nil {
		t.Fatalf("unexpected error loading checklist: %v", err)
	}

	repo := &AwsRepository{
		RDSClient: &mockRDSClient{
			describeDBInstancesFunc: func(context.Context, *rds.DescribeDBInstancesInput, ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
				return &rds.DescribeDBInstancesOutput{
					DBInstances: []rdstypes.DBInstance{
						{
							DBInstanceIdentifier:  awssdk.String("prod-db"),
							DBInstanceStatus:      awssdk.String("available"),
							Engine:                awssdk.String("postgres"),
							EngineVersion:         awssdk.String("16.2"),
							DBInstanceClass:       awssdk.String("db.t4g.medium"),
							MultiAZ:               awssdk.Bool(true),
							StorageEncrypted:      awssdk.Bool(true),
							PubliclyAccessible:    awssdk.Bool(false),
							BackupRetentionPeriod: awssdk.Int32(7),
						},
					},
				}, nil
			},
		},
		EC2Client: &mockEC2Client{
			describeSecurityGroupsFunc: func(context.Context, *ec2.DescribeSecurityGroupsInput, ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
				return &ec2.DescribeSecurityGroupsOutput{
					SecurityGroups: []ec2types.SecurityGroup{
						{
							GroupId:   awssdk.String("sg-web"),
							GroupName: awssdk.String("web"),
							VpcId:     awssdk.String("vpc-123"),
							IpPermissions: []ec2types.IpPermission{
								{
									IpProtocol: awssdk.String("tcp"),
									FromPort:   awssdk.Int32(443),
									ToPort:     awssdk.Int32(443),
									UserIdGroupPairs: []ec2types.UserIdGroupPair{
										{GroupId: awssdk.String("sg-alb")},
									},
								},
							},
						},
					},
				}, nil
			},
		},
		SecretsManagerClient: &inspectorSecretsMockClient{
			listSecretsFunc: func(context.Context, *secretsmanager.ListSecretsInput, ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
				return &secretsmanager.ListSecretsOutput{
					SecretList: []smtypes.SecretListEntry{
						{
							Name:            awssdk.String("prod/app"),
							ARN:             awssdk.String("arn:aws:secretsmanager:ap-northeast-2:123456789012:secret:prod/app"),
							KmsKeyId:        awssdk.String("alias/prod-secrets"),
							RotationEnabled: awssdk.Bool(true),
						},
					},
				}, nil
			},
			getSecretValueFunc: func(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return &secretsmanager.GetSecretValueOutput{
					Name:         awssdk.String("prod/app"),
					SecretString: awssdk.String(`{"username":"app-user"}`),
				}, nil
			},
		},
	}

	report, err := RunChecklist(context.Background(), repo, checklist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.PassedCount != 2 || report.FailedCount != 1 {
		t.Fatalf("expected 2 pass / 1 fail, got %d pass / %d fail", report.PassedCount, report.FailedCount)
	}
	if len(report.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(report.Results))
	}
	if !report.Results[0].Passed {
		t.Fatalf("expected RDS check to pass, got %+v", report.Results[0])
	}
	if !report.Results[1].Passed {
		t.Fatalf("expected security group check to pass, got %+v", report.Results[1])
	}
	if report.Results[2].Passed {
		t.Fatalf("expected secret check to fail, got %+v", report.Results[2])
	}
	if !strings.Contains(report.Results[2].Details[0], "password") {
		t.Fatalf("expected missing password detail, got %+v", report.Results[2].Details)
	}
}
