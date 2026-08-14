package aws

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func TestFilterText(t *testing.T) {
	inst := EC2Instance{
		InstanceID: "i-1234567890abcdef0",
		Name:       "WebServer",
		PrivateIP:  "10.0.1.50",
	}

	got := inst.FilterText()
	for _, keyword := range []string{"webserver", "i-1234567890abcdef0", "10.0.1.50"} {
		if !containsStr(got, keyword) {
			t.Errorf("FilterText %q should contain %q", got, keyword)
		}
	}
}

func TestFilterTextContainsAllFields(t *testing.T) {
	inst := EC2Instance{
		InstanceID:       "i-abc",
		Name:             "MyApp",
		State:            "running",
		InstanceType:     "t3.micro",
		AvailabilityZone: "us-east-1a",
		VPCID:            "vpc-123",
		SubnetID:         "subnet-123",
		PrivateIP:        "172.16.0.1",
		PublicIP:         "203.0.113.10",
		PlatformDetails:  "Linux/UNIX",
		IAMProfile:       "arn:aws:iam::123456789012:instance-profile/app",
		Tags:             map[string]string{"Environment": "prod"},
	}

	ft := inst.FilterText()
	for _, keyword := range []string{
		"myapp",
		"i-abc",
		"running",
		"t3.micro",
		"us-east-1a",
		"vpc-123",
		"subnet-123",
		"172.16.0.1",
		"203.0.113.10",
		"linux/unix",
		"instance-profile/app",
		"environment",
		"prod",
	} {
		if !containsStr(ft, keyword) {
			t.Errorf("FilterText %q should contain %q", ft, keyword)
		}
	}
}

func TestDisplayTitle(t *testing.T) {
	inst := EC2Instance{
		InstanceID:   "i-abc",
		Name:         "MyApp",
		State:        "running",
		InstanceType: "t3.micro",
		PrivateIP:    "10.0.0.1",
	}

	expected := "MyApp (i-abc) t3.micro [running] - 10.0.0.1"
	if got := inst.DisplayTitle(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestExtractNameTag(t *testing.T) {
	tests := []struct {
		name     string
		tags     []types.Tag
		expected string
	}{
		{
			name: "has name tag",
			tags: []types.Tag{
				{Key: aws.String("Name"), Value: aws.String("production-web")},
				{Key: aws.String("Env"), Value: aws.String("prod")},
			},
			expected: "production-web",
		},
		{
			name:     "no tags",
			tags:     nil,
			expected: "Unknown",
		},
		{
			name: "no name tag",
			tags: []types.Tag{
				{Key: aws.String("Env"), Value: aws.String("dev")},
			},
			expected: "Unknown",
		},
		{
			name: "empty name tag",
			tags: []types.Tag{
				{Key: aws.String("Name"), Value: aws.String("")},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNameTag(tt.tags)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestDerefString(t *testing.T) {
	s := "hello"
	if derefString(&s) != "hello" {
		t.Error("should dereference non-nil string")
	}
	if derefString(nil) != "" {
		t.Error("should return empty string for nil")
	}
}

func TestListRunningInstances_SortedByName(t *testing.T) {
	ec2Mock := &mockEC2Client{
		describeInstancesFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:       aws.String("i-2"),
								PrivateIpAddress: aws.String("10.0.0.2"),
								State:            &types.InstanceState{Name: types.InstanceStateNameRunning},
								Tags:             []types.Tag{{Key: aws.String("Name"), Value: aws.String("zeta-web")}},
							},
							{
								InstanceId:       aws.String("i-1"),
								PrivateIpAddress: aws.String("10.0.0.1"),
								State:            &types.InstanceState{Name: types.InstanceStateNameRunning},
								Tags:             []types.Tag{{Key: aws.String("Name"), Value: aws.String("alpha-web")}},
							},
						},
					},
				},
			}, nil
		},
	}
	ssmMock := &mockSSMClient{
		describeInstanceInfoFunc: func(_ context.Context, _ *ssm.DescribeInstanceInformationInput, _ ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
			return &ssm.DescribeInstanceInformationOutput{
				InstanceInformationList: []ssmtypes.InstanceInformation{
					{InstanceId: aws.String("i-2")},
					{InstanceId: aws.String("i-1")},
				},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: ec2Mock, SSMClient: ssmMock}
	instances, err := repo.ListRunningInstances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}
	if instances[0].Name != "alpha-web" || instances[1].Name != "zeta-web" {
		t.Fatalf("expected alphabetical EC2 order, got %+v", instances)
	}
}

func TestListEC2Instances_MapsInventoryFields(t *testing.T) {
	launch := time.Date(2026, 4, 22, 12, 30, 0, 0, time.UTC)
	ec2Mock := &mockEC2Client{
		describeInstancesFunc: func(_ context.Context, params *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			if len(params.Filters) != 0 {
				t.Fatalf("expected no filters for inventory browser, got %+v", params.Filters)
			}
			if len(params.InstanceIds) != 0 {
				t.Fatalf("expected no instance IDs for inventory browser, got %+v", params.InstanceIds)
			}
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:       aws.String("i-2"),
								InstanceType:     types.InstanceTypeM6iLarge,
								PrivateIpAddress: aws.String("10.0.0.2"),
								PublicIpAddress:  aws.String("203.0.113.2"),
								State:            &types.InstanceState{Name: types.InstanceStateNameStopped},
								Placement:        &types.Placement{AvailabilityZone: aws.String("us-east-1b")},
								VpcId:            aws.String("vpc-2"),
								SubnetId:         aws.String("subnet-2"),
								LaunchTime:       &launch,
								PlatformDetails:  aws.String("Linux/UNIX"),
								IamInstanceProfile: &types.IamInstanceProfile{
									Arn: aws.String("arn:aws:iam::123456789012:instance-profile/app"),
								},
								Tags: []types.Tag{
									{Key: aws.String("Name"), Value: aws.String("zeta-web")},
									{Key: aws.String("Environment"), Value: aws.String("prod")},
								},
							},
							{
								InstanceId:       aws.String("i-1"),
								InstanceType:     types.InstanceTypeT3Micro,
								PrivateIpAddress: aws.String("10.0.0.1"),
								State:            &types.InstanceState{Name: types.InstanceStateNameRunning},
								Tags:             []types.Tag{{Key: aws.String("Name"), Value: aws.String("alpha-web")}},
							},
						},
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: ec2Mock}
	instances, err := repo.ListEC2Instances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}
	if instances[0].InstanceID != "i-1" {
		t.Fatalf("expected alpha-web to sort first, got %+v", instances)
	}

	inst := instances[1]
	if inst.Name != "zeta-web" {
		t.Fatalf("expected Name zeta-web, got %q", inst.Name)
	}
	if inst.State != "stopped" {
		t.Fatalf("expected stopped state, got %q", inst.State)
	}
	if inst.InstanceType != "m6i.large" {
		t.Fatalf("expected m6i.large instance type, got %q", inst.InstanceType)
	}
	if inst.AvailabilityZone != "us-east-1b" {
		t.Fatalf("expected AZ us-east-1b, got %q", inst.AvailabilityZone)
	}
	if inst.VPCID != "vpc-2" || inst.SubnetID != "subnet-2" {
		t.Fatalf("expected VPC/subnet IDs, got %q/%q", inst.VPCID, inst.SubnetID)
	}
	if inst.PrivateIP != "10.0.0.2" || inst.PublicIP != "203.0.113.2" {
		t.Fatalf("expected private/public IPs, got %q/%q", inst.PrivateIP, inst.PublicIP)
	}
	if !inst.LaunchTime.Equal(launch) {
		t.Fatalf("expected launch time %v, got %v", launch, inst.LaunchTime)
	}
	if inst.PlatformDetails != "Linux/UNIX" {
		t.Fatalf("expected platform details, got %q", inst.PlatformDetails)
	}
	if inst.IAMProfile != "arn:aws:iam::123456789012:instance-profile/app" {
		t.Fatalf("expected IAM profile ARN, got %q", inst.IAMProfile)
	}
	if inst.Tags["Environment"] != "prod" {
		t.Fatalf("expected Environment tag prod, got %+v", inst.Tags)
	}
}

func TestEC2InstanceFromSDKHandlesNilState(t *testing.T) {
	inst := ec2InstanceFromSDK(types.Instance{
		InstanceId: aws.String("i-no-state"),
	})

	if inst.InstanceID != "i-no-state" {
		t.Fatalf("expected instance ID to be mapped, got %q", inst.InstanceID)
	}
	if inst.State != "" {
		t.Fatalf("expected empty state for nil SDK state, got %q", inst.State)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func regionEC2Mock(region, instanceID string) *mockEC2Client {
	return &mockEC2Client{
		describeInstancesFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId: aws.String(instanceID),
								Tags:       []types.Tag{{Key: aws.String("Name"), Value: aws.String(region + "-web")}},
							},
						},
					},
				},
			}, nil
		},
	}
}

func TestListEC2InstancesAcrossRegions_MergesAndTagsRegions(t *testing.T) {
	base := &AwsRepository{Region: "us-east-1", EC2Client: regionEC2Mock("us-east-1", "i-east")}
	west := &AwsRepository{Region: "eu-west-1", EC2Client: regionEC2Mock("eu-west-1", "i-west")}

	original := ec2RepoForRegion
	defer func() { ec2RepoForRegion = original }()
	ec2RepoForRegion = func(r *AwsRepository, region string) *AwsRepository {
		if region != "eu-west-1" {
			t.Fatalf("unexpected region requested: %s", region)
		}
		return west
	}

	instances, regionErrors := base.ListEC2InstancesAcrossRegions(context.Background(), []string{"us-east-1", "eu-west-1"})
	if len(regionErrors) != 0 {
		t.Fatalf("unexpected region errors: %+v", regionErrors)
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}
	regions := map[string]string{}
	for _, inst := range instances {
		regions[inst.InstanceID] = inst.Region
	}
	if regions["i-east"] != "us-east-1" || regions["i-west"] != "eu-west-1" {
		t.Fatalf("expected per-region tagging, got %+v", regions)
	}
}

func TestListEC2InstancesAcrossRegions_KeepsPartialResultsOnFailure(t *testing.T) {
	base := &AwsRepository{Region: "us-east-1", EC2Client: regionEC2Mock("us-east-1", "i-east")}
	broken := &AwsRepository{Region: "eu-west-1", EC2Client: &mockEC2Client{
		describeInstancesFunc: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return nil, errRegionDown
		},
	}}

	original := ec2RepoForRegion
	defer func() { ec2RepoForRegion = original }()
	ec2RepoForRegion = func(_ *AwsRepository, _ string) *AwsRepository { return broken }

	instances, regionErrors := base.ListEC2InstancesAcrossRegions(context.Background(), []string{"us-east-1", "eu-west-1"})
	if len(instances) != 1 || instances[0].InstanceID != "i-east" {
		t.Fatalf("expected the healthy region's instances to survive, got %+v", instances)
	}
	if len(regionErrors) != 1 || regionErrors[0].Region != "eu-west-1" || regionErrors[0].Err != errRegionDown {
		t.Fatalf("expected one eu-west-1 error, got %+v", regionErrors)
	}
}

var errRegionDown = errors.New("region down")
