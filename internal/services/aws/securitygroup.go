package aws

import (
	"context"
	"fmt"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// parseRules converts EC2 IpPermissions into SecurityGroupRule slices.
func parseRules(perms []types.IpPermission) []SecurityGroupRule {
	var rules []SecurityGroupRule
	for _, perm := range perms {
		base := SecurityGroupRule{
			Protocol: awssdk.ToString(perm.IpProtocol),
			FromPort: awssdk.ToInt32(perm.FromPort),
			ToPort:   awssdk.ToInt32(perm.ToPort),
		}
		for _, ipRange := range perm.IpRanges {
			rule := base
			rule.CIDRV4 = awssdk.ToString(ipRange.CidrIp)
			rule.Description = awssdk.ToString(ipRange.Description)
			rules = append(rules, rule)
		}
		for _, ipv6Range := range perm.Ipv6Ranges {
			rule := base
			rule.CIDRV6 = awssdk.ToString(ipv6Range.CidrIpv6)
			rule.Description = awssdk.ToString(ipv6Range.Description)
			rules = append(rules, rule)
		}
		for _, sgRef := range perm.UserIdGroupPairs {
			rule := base
			rule.ReferencedSGID = awssdk.ToString(sgRef.GroupId)
			rule.Description = awssdk.ToString(sgRef.Description)
			rules = append(rules, rule)
		}
		if len(perm.IpRanges) == 0 && len(perm.Ipv6Ranges) == 0 && len(perm.UserIdGroupPairs) == 0 {
			rules = append(rules, base)
		}
	}
	return rules
}

// ListSecurityGroups returns all security groups in the current account/region.
func (r *AwsRepository) ListSecurityGroups(ctx context.Context) ([]SecurityGroup, error) {
	output, err := r.EC2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return nil, err
	}

	sgs := make([]SecurityGroup, 0, len(output.SecurityGroups))
	for _, sg := range output.SecurityGroups {
		group := SecurityGroup{
			GroupID:     awssdk.ToString(sg.GroupId),
			Name:        awssdk.ToString(sg.GroupName),
			Description: awssdk.ToString(sg.Description),
			VPCID:       awssdk.ToString(sg.VpcId),
		}
		if group.Name == "default" {
			group.IsDefault = true
		}
		group.IngressRules = parseRules(sg.IpPermissions)
		group.EgressRules = parseRules(sg.IpPermissionsEgress)
		sgs = append(sgs, group)
	}
	sort.Slice(sgs, func(i, j int) bool {
		left := normalizedSortKey(sgs[i].Name, sgs[i].GroupID)
		right := normalizedSortKey(sgs[j].Name, sgs[j].GroupID)
		if left == right {
			return sgs[i].GroupID < sgs[j].GroupID
		}
		return left < right
	})
	return sgs, nil
}

// buildIpPermission constructs an ec2 IpPermission from a SecurityGroupRule.
func buildIpPermission(rule SecurityGroupRule) types.IpPermission {
	perm := types.IpPermission{
		IpProtocol: awssdk.String(rule.Protocol),
	}
	if rule.Protocol != "-1" {
		perm.FromPort = awssdk.Int32(rule.FromPort)
		perm.ToPort = awssdk.Int32(rule.ToPort)
	}
	if rule.ReferencedSGID != "" {
		pair := types.UserIdGroupPair{GroupId: awssdk.String(rule.ReferencedSGID)}
		if rule.Description != "" {
			pair.Description = awssdk.String(rule.Description)
		}
		perm.UserIdGroupPairs = []types.UserIdGroupPair{pair}
	} else if rule.CIDRV6 != "" {
		r := types.Ipv6Range{CidrIpv6: awssdk.String(rule.CIDRV6)}
		if rule.Description != "" {
			r.Description = awssdk.String(rule.Description)
		}
		perm.Ipv6Ranges = []types.Ipv6Range{r}
	} else {
		cidr := rule.CIDRV4
		if cidr == "" {
			cidr = "0.0.0.0/0"
		}
		r := types.IpRange{CidrIp: awssdk.String(cidr)}
		if rule.Description != "" {
			r.Description = awssdk.String(rule.Description)
		}
		perm.IpRanges = []types.IpRange{r}
	}
	return perm
}

// AddSecurityGroupRule adds an inbound or outbound rule to a security group.
// direction must be "ingress" or "egress".
func (r *AwsRepository) AddSecurityGroupRule(ctx context.Context, groupID, direction string, rule SecurityGroupRule) error {
	perm := buildIpPermission(rule)
	if direction == "ingress" {
		_, err := r.EC2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:       awssdk.String(groupID),
			IpPermissions: []types.IpPermission{perm},
		})
		return err
	}
	_, err := r.EC2Client.AuthorizeSecurityGroupEgress(ctx, &ec2.AuthorizeSecurityGroupEgressInput{
		GroupId:       awssdk.String(groupID),
		IpPermissions: []types.IpPermission{perm},
	})
	return err
}

// DeleteSecurityGroupRule removes an inbound or outbound rule from a security group.
// direction must be "ingress" or "egress".
func (r *AwsRepository) DeleteSecurityGroupRule(ctx context.Context, groupID, direction string, rule SecurityGroupRule) error {
	perm := buildIpPermission(rule)
	if direction == "ingress" {
		_, err := r.EC2Client.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
			GroupId:       awssdk.String(groupID),
			IpPermissions: []types.IpPermission{perm},
		})
		return err
	}
	_, err := r.EC2Client.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{
		GroupId:       awssdk.String(groupID),
		IpPermissions: []types.IpPermission{perm},
	})
	return err
}

// RefreshSecurityGroup fetches a single security group by ID and parses it.
func (r *AwsRepository) RefreshSecurityGroup(ctx context.Context, groupID string) (*SecurityGroup, error) {
	output, err := r.EC2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{groupID},
	})
	if err != nil {
		return nil, err
	}
	if len(output.SecurityGroups) == 0 {
		return nil, fmt.Errorf("security group %s not found", groupID)
	}
	sg := output.SecurityGroups[0]
	group := SecurityGroup{
		GroupID:     awssdk.ToString(sg.GroupId),
		Name:        awssdk.ToString(sg.GroupName),
		Description: awssdk.ToString(sg.Description),
		VPCID:       awssdk.ToString(sg.VpcId),
	}
	if group.Name == "default" {
		group.IsDefault = true
	}
	group.IngressRules = parseRules(sg.IpPermissions)
	group.EgressRules = parseRules(sg.IpPermissionsEgress)
	return &group, nil
}
