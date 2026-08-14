package inspector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/guardduty"
	guarddutytypes "github.com/aws/aws-sdk-go-v2/service/guardduty/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
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

func TestLoadChecklistParsesExpandedSampleFile(t *testing.T) {
	path := filepath.Join("testdata", "checklists", "expanded-readiness.yaml")

	checklist, err := LoadChecklist(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(checklist.Checks) != 9 {
		t.Fatalf("expected 9 checks, got %d", len(checklist.Checks))
	}
	if checklist.Checks[0].Type != ChecklistCheckHostedZone {
		t.Fatalf("expected first expanded check to be hosted_zone, got %s", checklist.Checks[0].Type)
	}
	if checklist.Checks[1].Expect.Zone != "example.internal" {
		t.Fatalf("expected route53_record zone to parse, got %+v", checklist.Checks[1].Expect)
	}
	if checklist.Checks[4].Type != ChecklistCheckCloudWatchLogGroup {
		t.Fatalf("expected fifth expanded check to be cloudwatch_log_group, got %s", checklist.Checks[4].Type)
	}
	if checklist.Checks[8].Type != ChecklistCheckElastiCacheValkeyBaseline {
		t.Fatalf("expected final expanded check to be elasticache_valkey_baseline, got %s", checklist.Checks[8].Type)
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

func TestLoadChecklistRejectsRoute53RecordWithoutZone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-zone.yaml")
	content := strings.Join([]string{
		"name: Invalid",
		"checks:",
		"  - type: route53_record",
		"    resource: api.example.internal",
		"    expect:",
		"      record_type: A",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write checklist: %v", err)
	}

	_, err := LoadChecklist(path)
	if err == nil {
		t.Fatal("expected route53_record zone validation error")
	}
	if !strings.Contains(err.Error(), "expect.zone") {
		t.Fatalf("expected expect.zone validation error, got %v", err)
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

func TestRunChecklistPanicsForUnhandledCheckType(t *testing.T) {
	runner := &checklistRunner{}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic for unhandled checklist type")
		}
		message, ok := recovered.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", recovered)
		}
		if !strings.Contains(message, "unhandled ChecklistCheckType: unsupported") {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()

	runner.runCheck(context.Background(), ChecklistCheck{
		Type: ChecklistCheckType("unsupported"),
	})
}

func TestRunChecklistRoute53Checks(t *testing.T) {
	checklistPath := filepath.Join(t.TempDir(), "route53.yaml")
	content := strings.Join([]string{
		"name: Route53 Readiness",
		"checks:",
		"  - id: private-zone",
		"    type: hosted_zone",
		"    resource: example.internal",
		"    expect:",
		"      private_zone: true",
		"  - id: api-record",
		"    type: route53_record",
		"    resource: api.example.internal",
		"    expect:",
		"      zone: example.internal",
		"      record_type: A",
		"      alias_target: internal-alb-123.ap-northeast-2.elb.amazonaws.com",
		"  - id: txt-record-mismatch",
		"    type: route53_record",
		"    resource: example.internal",
		"    expect:",
		"      zone: example.internal",
		"      record_type: TXT",
		"      values:",
		"        - v=spf1 include:mail.example.internal -all",
	}, "\n")
	if err := os.WriteFile(checklistPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write checklist: %v", err)
	}

	checklist, err := LoadChecklist(checklistPath)
	if err != nil {
		t.Fatalf("unexpected error loading checklist: %v", err)
	}

	repo := &AwsRepository{
		Route53Client: &checklistMockRoute53Client{
			listHostedZonesFunc: func(context.Context, *route53.ListHostedZonesInput, ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
				return &route53.ListHostedZonesOutput{
					HostedZones: []r53types.HostedZone{
						{
							Id:   awssdk.String("/hostedzone/Z123"),
							Name: awssdk.String("example.internal."),
							Config: &r53types.HostedZoneConfig{
								PrivateZone: true,
							},
						},
					},
				}, nil
			},
			listResourceRecordSetsFunc: func(_ context.Context, params *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
				if awssdk.ToString(params.HostedZoneId) != "Z123" {
					t.Fatalf("unexpected hosted zone id %q", awssdk.ToString(params.HostedZoneId))
				}
				return &route53.ListResourceRecordSetsOutput{
					ResourceRecordSets: []r53types.ResourceRecordSet{
						{
							Name: awssdk.String("api.example.internal."),
							Type: r53types.RRTypeA,
							AliasTarget: &r53types.AliasTarget{
								DNSName:      awssdk.String("internal-alb-123.ap-northeast-2.elb.amazonaws.com."),
								HostedZoneId: awssdk.String("ZELB"),
							},
						},
						{
							Name: awssdk.String("example.internal."),
							Type: r53types.RRTypeTxt,
							TTL:  awssdk.Int64(60),
							ResourceRecords: []r53types.ResourceRecord{
								{Value: awssdk.String("v=spf1 include:_spf.example.internal -all")},
							},
						},
					},
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
	if !report.Results[0].Passed || !report.Results[1].Passed {
		t.Fatalf("expected hosted zone and alias record checks to pass, got %+v", report.Results)
	}
	if report.Results[2].Passed {
		t.Fatalf("expected TXT record check to fail, got %+v", report.Results[2])
	}
	if !strings.Contains(report.Results[2].Details[0], "values") {
		t.Fatalf("expected values mismatch detail, got %+v", report.Results[2].Details)
	}
}

func TestRunChecklistVPCAndSubnetChecks(t *testing.T) {
	checklistPath := filepath.Join(t.TempDir(), "network.yaml")
	content := strings.Join([]string{
		"name: Network Readiness",
		"checks:",
		"  - id: main-vpc",
		"    type: vpc",
		"    resource: main-vpc",
		"    expect:",
		"      cidr: 10.0.0.0/16",
		"      default_vpc: false",
		"      subnet_count: 2",
		"  - id: app-subnet-a",
		"    type: subnet",
		"    resource: app-a",
		"    expect:",
		"      vpc: main-vpc",
		"      availability_zone: ap-northeast-2a",
		"      available_ip_count_min: 10",
		"  - id: app-subnet-b-low-ips",
		"    type: subnet",
		"    resource: app-b",
		"    expect:",
		"      vpc: main-vpc",
		"      available_ip_count_min: 10",
	}, "\n")
	if err := os.WriteFile(checklistPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write checklist: %v", err)
	}

	checklist, err := LoadChecklist(checklistPath)
	if err != nil {
		t.Fatalf("unexpected error loading checklist: %v", err)
	}

	repo := &AwsRepository{
		EC2Client: &mockEC2Client{
			describeVpcsFunc: func(context.Context, *ec2.DescribeVpcsInput, ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
				return &ec2.DescribeVpcsOutput{
					Vpcs: []ec2types.Vpc{
						{
							VpcId:     awssdk.String("vpc-123"),
							CidrBlock: awssdk.String("10.0.0.0/16"),
							IsDefault: awssdk.Bool(false),
							Tags:      []ec2types.Tag{{Key: awssdk.String("Name"), Value: awssdk.String("main-vpc")}},
						},
					},
				}, nil
			},
			describeSubnetsFunc: func(_ context.Context, params *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
				if len(params.Filters) == 0 || awssdk.ToString(params.Filters[0].Name) != "vpc-id" {
					t.Fatalf("expected subnet filter by vpc-id, got %+v", params.Filters)
				}
				return &ec2.DescribeSubnetsOutput{
					Subnets: []ec2types.Subnet{
						{
							SubnetId:                awssdk.String("subnet-a"),
							VpcId:                   awssdk.String("vpc-123"),
							CidrBlock:               awssdk.String("10.0.1.0/24"),
							AvailabilityZone:        awssdk.String("ap-northeast-2a"),
							AvailableIpAddressCount: awssdk.Int32(20),
							Tags:                    []ec2types.Tag{{Key: awssdk.String("Name"), Value: awssdk.String("app-a")}},
						},
						{
							SubnetId:                awssdk.String("subnet-b"),
							VpcId:                   awssdk.String("vpc-123"),
							CidrBlock:               awssdk.String("10.0.2.0/24"),
							AvailabilityZone:        awssdk.String("ap-northeast-2b"),
							AvailableIpAddressCount: awssdk.Int32(4),
							Tags:                    []ec2types.Tag{{Key: awssdk.String("Name"), Value: awssdk.String("app-b")}},
						},
					},
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
	if report.Results[2].Passed {
		t.Fatalf("expected subnet low-IP check to fail, got %+v", report.Results[2])
	}
	if !strings.Contains(report.Results[2].Details[0], "available_ip_count") {
		t.Fatalf("expected available_ip_count detail, got %+v", report.Results[2].Details)
	}
}

func TestRunChecklistLogGroupAndBaselineChecks(t *testing.T) {
	checklistPath := filepath.Join(t.TempDir(), "baselines.yaml")
	content := strings.Join([]string{
		"name: Logging and Baselines",
		"checks:",
		"  - id: app-log-group",
		"    type: cloudwatch_log_group",
		"    resource: /aws/ecs/app",
		"    expect:",
		"      retention_days: 30",
		"  - id: cloudtrail-clean",
		"    type: cloudtrail_baseline",
		"    resource: cloudtrail",
		"  - id: guardduty-clean",
		"    type: guardduty_baseline",
		"    resource: guardduty",
		"  - id: config-clean",
		"    type: config_baseline",
		"    resource: config",
		"  - id: valkey-clean",
		"    type: elasticache_valkey_baseline",
		"    resource: valkey",
	}, "\n")
	if err := os.WriteFile(checklistPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write checklist: %v", err)
	}

	checklist, err := LoadChecklist(checklistPath)
	if err != nil {
		t.Fatalf("unexpected error loading checklist: %v", err)
	}

	repo := &AwsRepository{
		CloudWatchLogsClient: &checklistMockCloudWatchLogsClient{
			describeLogGroupsFunc: func(context.Context, *cloudwatchlogs.DescribeLogGroupsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return &cloudwatchlogs.DescribeLogGroupsOutput{
					LogGroups: []cloudwatchlogstypes.LogGroup{
						{
							LogGroupName:    awssdk.String("/aws/ecs/app"),
							Arn:             awssdk.String("arn:aws:logs:ap-northeast-2:123456789012:log-group:/aws/ecs/app"),
							RetentionInDays: awssdk.Int32(30),
						},
					},
				}, nil
			},
		},
		CloudTrailClient: &mockCloudTrailClient{
			describeTrailsFunc: func(context.Context, *cloudtrail.DescribeTrailsInput, ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error) {
				return &cloudtrail.DescribeTrailsOutput{
					TrailList: []cloudtrailtypes.Trail{
						{
							Name:                     awssdk.String("org-trail"),
							TrailARN:                 awssdk.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/org-trail"),
							IsMultiRegionTrail:       awssdk.Bool(true),
							LogFileValidationEnabled: awssdk.Bool(true),
						},
					},
				}, nil
			},
			getTrailStatusFunc: func(context.Context, *cloudtrail.GetTrailStatusInput, ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error) {
				return &cloudtrail.GetTrailStatusOutput{IsLogging: awssdk.Bool(true)}, nil
			},
		},
		GuardDutyClient: &mockGuardDutyClient{
			listDetectorsFunc: func(context.Context, *guardduty.ListDetectorsInput, ...func(*guardduty.Options)) (*guardduty.ListDetectorsOutput, error) {
				return &guardduty.ListDetectorsOutput{DetectorIds: []string{"detector-1"}}, nil
			},
			getDetectorFunc: func(context.Context, *guardduty.GetDetectorInput, ...func(*guardduty.Options)) (*guardduty.GetDetectorOutput, error) {
				return &guardduty.GetDetectorOutput{Status: guarddutytypes.DetectorStatusEnabled}, nil
			},
		},
		ConfigServiceClient: &mockConfigServiceClient{
			describeRecordersFunc: func(context.Context, *configservice.DescribeConfigurationRecordersInput, ...func(*configservice.Options)) (*configservice.DescribeConfigurationRecordersOutput, error) {
				return &configservice.DescribeConfigurationRecordersOutput{}, nil
			},
			describeStatusFunc: func(context.Context, *configservice.DescribeConfigurationRecorderStatusInput, ...func(*configservice.Options)) (*configservice.DescribeConfigurationRecorderStatusOutput, error) {
				return &configservice.DescribeConfigurationRecorderStatusOutput{}, nil
			},
		},
		ElastiCacheClient: &mockElastiCacheClient{
			describeReplicationGroupsFunc: func(context.Context, *elasticache.DescribeReplicationGroupsInput, ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
				return &elasticache.DescribeReplicationGroupsOutput{
					ReplicationGroups: []elasticachetypes.ReplicationGroup{
						{
							ReplicationGroupId:       awssdk.String("valkey-secure"),
							Engine:                   awssdk.String("valkey"),
							TransitEncryptionEnabled: awssdk.Bool(true),
							AtRestEncryptionEnabled:  awssdk.Bool(true),
							SnapshotRetentionLimit:   awssdk.Int32(inspectorValkeyMinSnapshotRetentionDays),
							UserGroupIds:             []string{"ug-123"},
						},
					},
				}, nil
			},
		},
	}

	report, err := RunChecklist(context.Background(), repo, checklist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.PassedCount != 4 || report.FailedCount != 1 {
		t.Fatalf("expected 4 pass / 1 fail, got %d pass / %d fail", report.PassedCount, report.FailedCount)
	}
	if report.Results[3].Passed {
		t.Fatalf("expected config baseline check to fail, got %+v", report.Results[3])
	}
	if !strings.Contains(report.Results[3].Summary, "baseline finding") {
		t.Fatalf("expected baseline finding summary, got %+v", report.Results[3])
	}
	if len(report.Results[3].Details) == 0 || !strings.Contains(report.Results[3].Details[0], "AWS Config") {
		t.Fatalf("expected baseline finding details, got %+v", report.Results[3].Details)
	}
}

func TestAppendCheckCreatesValidatesAndRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.yaml")

	check := ChecklistCheck{
		Type:     ChecklistCheckRDS,
		Resource: "prod-db",
		Expect:   ChecklistExpectations{StorageEncrypted: boolPtr(true)},
	}
	if err := AppendCheck(path, check); err != nil {
		t.Fatalf("unexpected error creating checklist: %v", err)
	}

	second := ChecklistCheck{
		Type:     ChecklistCheckCloudWatchLogGroup,
		Resource: "/aws/ecs/app",
		Expect:   ChecklistExpectations{RetentionDays: intPtr(30)},
	}
	if err := AppendCheck(path, second); err != nil {
		t.Fatalf("unexpected error appending: %v", err)
	}

	loaded, err := LoadChecklist(path)
	if err != nil {
		t.Fatalf("expected generated file to load through validation: %v", err)
	}
	if len(loaded.Checks) != 2 || loaded.Checks[1].Resource != "/aws/ecs/app" {
		t.Fatalf("expected both checks persisted, got %+v", loaded.Checks)
	}
}

func TestAppendCheckRejectsInvalidWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.yaml")
	valid := ChecklistCheck{
		Type:     ChecklistCheckRDS,
		Resource: "prod-db",
		Expect:   ChecklistExpectations{StorageEncrypted: boolPtr(true)},
	}
	if err := AppendCheck(path, valid); err != nil {
		t.Fatal(err)
	}

	invalid := ChecklistCheck{Type: ChecklistCheckRDS, Resource: "no-expectations"}
	if err := AppendCheck(path, invalid); err == nil {
		t.Fatal("expected validation rejection for a check without expectations")
	}

	loaded, err := LoadChecklist(path)
	if err != nil || len(loaded.Checks) != 1 {
		t.Fatalf("expected file untouched after rejection, got %+v err=%v", loaded, err)
	}
}

func boolPtr(v bool) *bool { return &v }
func intPtr(v int) *int    { return &v }
