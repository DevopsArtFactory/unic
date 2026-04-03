package aws

import (
	"context"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestListSecurityGroups_Success(t *testing.T) {
	mock := &mockEC2Client{
		describeSecurityGroupsFunc: func(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			return &ec2.DescribeSecurityGroupsOutput{
				SecurityGroups: []types.SecurityGroup{
					{
						GroupId:     awssdk.String("sg-aaa"),
						GroupName:   awssdk.String("web-sg"),
						Description: awssdk.String("Web servers"),
						VpcId:       awssdk.String("vpc-111"),
						IpPermissions: []types.IpPermission{
							{
								IpProtocol: awssdk.String("tcp"),
								FromPort:   awssdk.Int32(443),
								ToPort:     awssdk.Int32(443),
								IpRanges: []types.IpRange{
									{CidrIp: awssdk.String("0.0.0.0/0"), Description: awssdk.String("HTTPS")},
								},
							},
							{
								IpProtocol: awssdk.String("tcp"),
								FromPort:   awssdk.Int32(22),
								ToPort:     awssdk.Int32(22),
								UserIdGroupPairs: []types.UserIdGroupPair{
									{GroupId: awssdk.String("sg-bastion"), Description: awssdk.String("SSH from bastion")},
								},
							},
						},
						IpPermissionsEgress: []types.IpPermission{
							{
								IpProtocol: awssdk.String("-1"),
								IpRanges: []types.IpRange{
									{CidrIp: awssdk.String("0.0.0.0/0")},
								},
							},
						},
					},
					{
						GroupId:   awssdk.String("sg-bbb"),
						GroupName: awssdk.String("default"),
						VpcId:     awssdk.String("vpc-111"),
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	sgs, err := repo.ListSecurityGroups(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sgs) != 2 {
		t.Fatalf("expected 2 security groups, got %d", len(sgs))
	}

	// First SG
	sg := sgs[0]
	if sg.GroupID != "sg-aaa" {
		t.Errorf("expected GroupID sg-aaa, got %s", sg.GroupID)
	}
	if sg.Name != "web-sg" {
		t.Errorf("expected Name web-sg, got %s", sg.Name)
	}
	if sg.IsDefault {
		t.Error("expected IsDefault false")
	}
	if len(sg.IngressRules) != 2 {
		t.Fatalf("expected 2 ingress rules, got %d", len(sg.IngressRules))
	}
	if sg.IngressRules[0].CIDRV4 != "0.0.0.0/0" {
		t.Errorf("expected first ingress CIDR 0.0.0.0/0, got %s", sg.IngressRules[0].CIDRV4)
	}
	if sg.IngressRules[1].ReferencedSGID != "sg-bastion" {
		t.Errorf("expected second ingress ref SG sg-bastion, got %s", sg.IngressRules[1].ReferencedSGID)
	}
	if len(sg.EgressRules) != 1 {
		t.Fatalf("expected 1 egress rule, got %d", len(sg.EgressRules))
	}

	// Second SG (default)
	if !sgs[1].IsDefault {
		t.Error("expected default SG to have IsDefault true")
	}
}

func TestListSecurityGroups_Error(t *testing.T) {
	mock := &mockEC2Client{
		describeSecurityGroupsFunc: func(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			return nil, context.DeadlineExceeded
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	_, err := repo.ListSecurityGroups(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSecurityGroupDisplayTitle(t *testing.T) {
	sg := SecurityGroup{GroupID: "sg-aaa", Name: "web-sg", VPCID: "vpc-111"}
	title := sg.DisplayTitle()
	if title != "web-sg (sg-aaa) - vpc-111" {
		t.Errorf("unexpected DisplayTitle: %s", title)
	}

	sgDefault := SecurityGroup{GroupID: "sg-bbb", Name: "default", VPCID: "vpc-111", IsDefault: true}
	titleDefault := sgDefault.DisplayTitle()
	if titleDefault != "default (sg-bbb) - vpc-111 [default]" {
		t.Errorf("unexpected DisplayTitle for default: %s", titleDefault)
	}
}

func TestAddSecurityGroupRule_Ingress(t *testing.T) {
	var captured *ec2.AuthorizeSecurityGroupIngressInput
	mock := &mockEC2Client{
		authorizeSGIngressFunc: func(_ context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, _ ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
			captured = params
			return &ec2.AuthorizeSecurityGroupIngressOutput{}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	err := repo.AddSecurityGroupRule(context.Background(), "sg-aaa", "ingress", SecurityGroupRule{
		Protocol:    "tcp",
		FromPort:    443,
		ToPort:      443,
		CIDRV4:      "10.0.0.0/8",
		Description: "HTTPS from internal",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected AuthorizeSecurityGroupIngress to be called")
	}
	if awssdk.ToString(captured.GroupId) != "sg-aaa" {
		t.Errorf("expected GroupId sg-aaa, got %s", awssdk.ToString(captured.GroupId))
	}
	if len(captured.IpPermissions) != 1 {
		t.Fatalf("expected 1 IpPermission, got %d", len(captured.IpPermissions))
	}
	perm := captured.IpPermissions[0]
	if awssdk.ToString(perm.IpProtocol) != "tcp" {
		t.Errorf("expected protocol tcp, got %s", awssdk.ToString(perm.IpProtocol))
	}
	if len(perm.IpRanges) != 1 || awssdk.ToString(perm.IpRanges[0].CidrIp) != "10.0.0.0/8" {
		t.Errorf("expected CIDR 10.0.0.0/8")
	}
}

func TestAddSecurityGroupRule_Egress_SGRef(t *testing.T) {
	var captured *ec2.AuthorizeSecurityGroupEgressInput
	mock := &mockEC2Client{
		authorizeSGEgressFunc: func(_ context.Context, params *ec2.AuthorizeSecurityGroupEgressInput, _ ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupEgressOutput, error) {
			captured = params
			return &ec2.AuthorizeSecurityGroupEgressOutput{}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	err := repo.AddSecurityGroupRule(context.Background(), "sg-bbb", "egress", SecurityGroupRule{
		Protocol:       "tcp",
		FromPort:       5432,
		ToPort:         5432,
		ReferencedSGID: "sg-db",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected AuthorizeSecurityGroupEgress to be called")
	}
	perm := captured.IpPermissions[0]
	if len(perm.UserIdGroupPairs) != 1 || awssdk.ToString(perm.UserIdGroupPairs[0].GroupId) != "sg-db" {
		t.Error("expected SG reference sg-db in UserIdGroupPairs")
	}
}

func TestDeleteSecurityGroupRule_Ingress(t *testing.T) {
	var captured *ec2.RevokeSecurityGroupIngressInput
	mock := &mockEC2Client{
		revokeSGIngressFunc: func(_ context.Context, params *ec2.RevokeSecurityGroupIngressInput, _ ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error) {
			captured = params
			return &ec2.RevokeSecurityGroupIngressOutput{}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	err := repo.DeleteSecurityGroupRule(context.Background(), "sg-aaa", "ingress", SecurityGroupRule{
		Protocol: "tcp",
		FromPort: 22,
		ToPort:   22,
		CIDRV4:   "0.0.0.0/0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected RevokeSecurityGroupIngress to be called")
	}
	if awssdk.ToString(captured.GroupId) != "sg-aaa" {
		t.Errorf("expected GroupId sg-aaa, got %s", awssdk.ToString(captured.GroupId))
	}
}

func TestDeleteSecurityGroupRule_Egress(t *testing.T) {
	var captured *ec2.RevokeSecurityGroupEgressInput
	mock := &mockEC2Client{
		revokeSGEgressFunc: func(_ context.Context, params *ec2.RevokeSecurityGroupEgressInput, _ ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupEgressOutput, error) {
			captured = params
			return &ec2.RevokeSecurityGroupEgressOutput{}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	err := repo.DeleteSecurityGroupRule(context.Background(), "sg-aaa", "egress", SecurityGroupRule{
		Protocol: "-1",
		CIDRV4:   "0.0.0.0/0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected RevokeSecurityGroupEgress to be called")
	}
}

func TestRefreshSecurityGroup_Success(t *testing.T) {
	mock := &mockEC2Client{
		describeSecurityGroupsFunc: func(_ context.Context, params *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			if len(params.GroupIds) != 1 || params.GroupIds[0] != "sg-aaa" {
				t.Errorf("expected GroupIds [sg-aaa], got %v", params.GroupIds)
			}
			return &ec2.DescribeSecurityGroupsOutput{
				SecurityGroups: []types.SecurityGroup{
					{
						GroupId:   awssdk.String("sg-aaa"),
						GroupName: awssdk.String("web-sg"),
						VpcId:     awssdk.String("vpc-111"),
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	sg, err := repo.RefreshSecurityGroup(context.Background(), "sg-aaa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sg.GroupID != "sg-aaa" {
		t.Errorf("expected GroupID sg-aaa, got %s", sg.GroupID)
	}
}

func TestRefreshSecurityGroup_NotFound(t *testing.T) {
	mock := &mockEC2Client{
		describeSecurityGroupsFunc: func(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: []types.SecurityGroup{}}, nil
		},
	}

	repo := &AwsRepository{EC2Client: mock}
	_, err := repo.RefreshSecurityGroup(context.Background(), "sg-missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSecurityGroupRuleDisplayTitle(t *testing.T) {
	tests := []struct {
		name     string
		rule     SecurityGroupRule
		expected string
	}{
		{
			name:     "TCP single port with CIDR",
			rule:     SecurityGroupRule{Protocol: "tcp", FromPort: 443, ToPort: 443, CIDRV4: "0.0.0.0/0"},
			expected: "tcp  443  0.0.0.0/0",
		},
		{
			name:     "TCP port range",
			rule:     SecurityGroupRule{Protocol: "tcp", FromPort: 1024, ToPort: 65535, CIDRV4: "10.0.0.0/8"},
			expected: "tcp  1024-65535  10.0.0.0/8",
		},
		{
			name:     "All traffic",
			rule:     SecurityGroupRule{Protocol: "-1", CIDRV4: "0.0.0.0/0"},
			expected: "All  All  0.0.0.0/0",
		},
		{
			name:     "SG reference",
			rule:     SecurityGroupRule{Protocol: "tcp", FromPort: 22, ToPort: 22, ReferencedSGID: "sg-bastion"},
			expected: "tcp  22  sg-bastion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rule.DisplayTitle()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
