package aws

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	uniclog "unic/internal/log"
)

// ListVPCs returns all VPCs in the current account/region.
func (r *AwsRepository) ListVPCs(ctx context.Context) ([]VPC, error) {
	uniclog.Debug("aws", "ListVPCs called")
	output, err := r.EC2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, err
	}

	vpcs := make([]VPC, 0, len(output.Vpcs))
	for _, v := range output.Vpcs {
		vpcs = append(vpcs, VPC{
			VPCID:     awssdk.ToString(v.VpcId),
			Name:      extractNameTag(v.Tags),
			CIDR:      awssdk.ToString(v.CidrBlock),
			IsDefault: awssdk.ToBool(v.IsDefault),
		})
	}
	sort.Slice(vpcs, func(i, j int) bool {
		left := normalizedSortKey(vpcs[i].Name, vpcs[i].VPCID)
		right := normalizedSortKey(vpcs[j].Name, vpcs[j].VPCID)
		if left == right {
			return vpcs[i].VPCID < vpcs[j].VPCID
		}
		return left < right
	})
	return vpcs, nil
}

// ListSubnets returns all subnets belonging to the given VPC.
func (r *AwsRepository) ListSubnets(ctx context.Context, vpcID string) ([]Subnet, error) {
	uniclog.Debug("aws", "ListSubnets called", "vpc_id", vpcID)
	input := &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   awssdk.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	}

	output, err := r.EC2Client.DescribeSubnets(ctx, input)
	if err != nil {
		return nil, err
	}

	subnets := make([]Subnet, 0, len(output.Subnets))
	for _, s := range output.Subnets {
		subnets = append(subnets, Subnet{
			SubnetID:         awssdk.ToString(s.SubnetId),
			Name:             extractNameTag(s.Tags),
			CIDR:             awssdk.ToString(s.CidrBlock),
			AvailabilityZone: awssdk.ToString(s.AvailabilityZone),
			AvailableIPCount: awssdk.ToInt32(s.AvailableIpAddressCount),
		})
	}
	sort.Slice(subnets, func(i, j int) bool {
		left := normalizedSortKey(subnets[i].Name, subnets[i].SubnetID)
		right := normalizedSortKey(subnets[j].Name, subnets[j].SubnetID)
		if left == right {
			return subnets[i].SubnetID < subnets[j].SubnetID
		}
		return left < right
	})
	return subnets, nil
}

// ListAvailableIPs returns the list of usable IP addresses in a subnet
// that are not currently assigned to any network interface.
// AWS reserves 5 IPs per subnet: .0 (network), .1 (router), .2 (DNS),
// .3 (future use), .255 (broadcast) — these are always excluded.
func (r *AwsRepository) ListAvailableIPs(ctx context.Context, subnetID, cidr string) ([]string, error) {
	uniclog.Debug("aws", "ListAvailableIPs called", "subnet_id", subnetID, "cidr", cidr)
	// Parse CIDR to get all IPs in range
	allIPs, err := cidrUsableIPs(cidr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CIDR %s: %w", cidr, err)
	}

	// Get IPs already assigned to network interfaces in this subnet
	input := &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{
				Name:   awssdk.String("subnet-id"),
				Values: []string{subnetID},
			},
		},
	}
	output, err := r.EC2Client.DescribeNetworkInterfaces(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe network interfaces: %w", err)
	}

	usedIPs := make(map[string]struct{})
	for _, eni := range output.NetworkInterfaces {
		for _, addr := range eni.PrivateIpAddresses {
			if addr.PrivateIpAddress != nil {
				usedIPs[*addr.PrivateIpAddress] = struct{}{}
			}
		}
	}

	available := make([]string, 0, len(allIPs))
	for _, ip := range allIPs {
		if _, used := usedIPs[ip]; !used {
			available = append(available, ip)
		}
	}
	return available, nil
}

// cidrUsableIPs returns all usable host IPs in a CIDR block,
// excluding the 5 AWS-reserved addresses:
//   - x.x.x.0   network address
//   - x.x.x.1   VPC router
//   - x.x.x.2   Amazon DNS
//   - x.x.x.3   reserved for future use
//   - x.x.x.255 broadcast
func cidrUsableIPs(cidr string) ([]string, error) {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	// Convert network IP to uint32 for arithmetic
	start := binary.BigEndian.Uint32(network.IP.To4())
	mask := binary.BigEndian.Uint32(network.Mask)
	end := start | ^mask

	// Usable range: skip .0, .1, .2, .3 (first 4) and .255 (last 1)
	firstUsable := start + 4
	lastUsable := end - 1

	if firstUsable > lastUsable {
		return nil, nil
	}

	ips := make([]string, 0, lastUsable-firstUsable+1)
	for i := firstUsable; i <= lastUsable; i++ {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, i)
		ips = append(ips, net.IP(b).String())
	}
	return ips, nil
}
