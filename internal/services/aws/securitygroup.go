package aws

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

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

		// Check if this is the default security group
		if group.Name == "default" {
			group.IsDefault = true
		}

		for _, perm := range sg.IpPermissions {
			base := SecurityGroupRule{
				Protocol: awssdk.ToString(perm.IpProtocol),
				FromPort: awssdk.ToInt32(perm.FromPort),
				ToPort:   awssdk.ToInt32(perm.ToPort),
			}
			for _, ipRange := range perm.IpRanges {
				rule := base
				rule.CIDRV4 = awssdk.ToString(ipRange.CidrIp)
				rule.Description = awssdk.ToString(ipRange.Description)
				group.IngressRules = append(group.IngressRules, rule)
			}
			for _, ipv6Range := range perm.Ipv6Ranges {
				rule := base
				rule.CIDRV6 = awssdk.ToString(ipv6Range.CidrIpv6)
				rule.Description = awssdk.ToString(ipv6Range.Description)
				group.IngressRules = append(group.IngressRules, rule)
			}
			for _, sgRef := range perm.UserIdGroupPairs {
				rule := base
				rule.ReferencedSGID = awssdk.ToString(sgRef.GroupId)
				rule.Description = awssdk.ToString(sgRef.Description)
				group.IngressRules = append(group.IngressRules, rule)
			}
			// If no specific source, add the base rule
			if len(perm.IpRanges) == 0 && len(perm.Ipv6Ranges) == 0 && len(perm.UserIdGroupPairs) == 0 {
				group.IngressRules = append(group.IngressRules, base)
			}
		}

		for _, perm := range sg.IpPermissionsEgress {
			base := SecurityGroupRule{
				Protocol: awssdk.ToString(perm.IpProtocol),
				FromPort: awssdk.ToInt32(perm.FromPort),
				ToPort:   awssdk.ToInt32(perm.ToPort),
			}
			for _, ipRange := range perm.IpRanges {
				rule := base
				rule.CIDRV4 = awssdk.ToString(ipRange.CidrIp)
				rule.Description = awssdk.ToString(ipRange.Description)
				group.EgressRules = append(group.EgressRules, rule)
			}
			for _, ipv6Range := range perm.Ipv6Ranges {
				rule := base
				rule.CIDRV6 = awssdk.ToString(ipv6Range.CidrIpv6)
				rule.Description = awssdk.ToString(ipv6Range.Description)
				group.EgressRules = append(group.EgressRules, rule)
			}
			for _, sgRef := range perm.UserIdGroupPairs {
				rule := base
				rule.ReferencedSGID = awssdk.ToString(sgRef.GroupId)
				rule.Description = awssdk.ToString(sgRef.Description)
				group.EgressRules = append(group.EgressRules, rule)
			}
			if len(perm.IpRanges) == 0 && len(perm.Ipv6Ranges) == 0 && len(perm.UserIdGroupPairs) == 0 {
				group.EgressRules = append(group.EgressRules, base)
			}
		}

		sgs = append(sgs, group)
	}
	return sgs, nil
}
