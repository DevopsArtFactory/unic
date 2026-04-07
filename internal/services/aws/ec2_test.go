package aws

import (
	"context"
	"testing"

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
	if got != "webserver i-1234567890abcdef0 10.0.1.50" {
		t.Errorf("unexpected FilterText: %q", got)
	}
}

func TestFilterTextContainsAllFields(t *testing.T) {
	inst := EC2Instance{
		InstanceID: "i-abc",
		Name:       "MyApp",
		PrivateIP:  "172.16.0.1",
	}

	ft := inst.FilterText()
	for _, keyword := range []string{"myapp", "i-abc", "172.16.0.1"} {
		if !containsStr(ft, keyword) {
			t.Errorf("FilterText %q should contain %q", ft, keyword)
		}
	}
}

func TestDisplayTitle(t *testing.T) {
	inst := EC2Instance{
		InstanceID: "i-abc",
		Name:       "MyApp",
		PrivateIP:  "10.0.0.1",
	}

	expected := "MyApp (i-abc) - 10.0.0.1"
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
