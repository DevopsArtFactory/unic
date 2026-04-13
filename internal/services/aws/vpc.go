package aws

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

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

// ListReachabilityTargets returns the Reachability Analyzer source and destination
// resource types currently supported by AWS and surfaced by unic.
func (r *AwsRepository) ListReachabilityTargets(ctx context.Context) ([]ReachabilityTarget, error) {
	uniclog.Debug("aws", "ListReachabilityTargets called")

	targets := make([]ReachabilityTarget, 0)

	instPaginator := ec2.NewDescribeInstancesPaginator(r.EC2Client, &ec2.DescribeInstancesInput{})
	for instPaginator.HasMorePages() {
		page, err := instPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}
		for _, reservation := range page.Reservations {
			for _, inst := range reservation.Instances {
				targets = append(targets, ReachabilityTarget{
					ID:          derefString(inst.InstanceId),
					Name:        extractNameTag(inst.Tags),
					Type:        "EC2 instances",
					VPCID:       derefString(inst.VpcId),
					SubnetID:    derefString(inst.SubnetId),
					PrivateIP:   derefString(inst.PrivateIpAddress),
					Description: fmt.Sprintf("state=%s", inst.State.Name),
				})
			}
		}
	}

	eniPaginator := ec2.NewDescribeNetworkInterfacesPaginator(r.EC2Client, &ec2.DescribeNetworkInterfacesInput{})
	for eniPaginator.HasMorePages() {
		page, err := eniPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe network interfaces: %w", err)
		}
		for _, eni := range page.NetworkInterfaces {
			name := extractNameTag(eni.TagSet)
			if name == "Unknown" {
				name = derefString(eni.Description)
			}
			if name == "" {
				name = derefString(eni.NetworkInterfaceId)
			}
			targets = append(targets, ReachabilityTarget{
				ID:          derefString(eni.NetworkInterfaceId),
				Name:        name,
				Type:        "Network interfaces",
				VPCID:       derefString(eni.VpcId),
				SubnetID:    derefString(eni.SubnetId),
				PrivateIP:   derefString(eni.PrivateIpAddress),
				Description: fmt.Sprintf("status=%s", eni.Status),
			})
		}
	}

	igwPaginator := ec2.NewDescribeInternetGatewaysPaginator(r.EC2Client, &ec2.DescribeInternetGatewaysInput{})
	for igwPaginator.HasMorePages() {
		page, err := igwPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe internet gateways: %w", err)
		}
		for _, igw := range page.InternetGateways {
			name := extractNameTag(igw.Tags)
			if name == "Unknown" {
				name = derefString(igw.InternetGatewayId)
			}
			vpcID := ""
			if len(igw.Attachments) > 0 {
				vpcID = derefString(igw.Attachments[0].VpcId)
			}
			targets = append(targets, ReachabilityTarget{
				ID:          derefString(igw.InternetGatewayId),
				Name:        name,
				Type:        "Internet gateways",
				VPCID:       vpcID,
				Description: describeIGWAttachment(igw.Attachments),
			})
		}
	}

	vpcEndpointPaginator := ec2.NewDescribeVpcEndpointsPaginator(r.EC2Client, &ec2.DescribeVpcEndpointsInput{})
	for vpcEndpointPaginator.HasMorePages() {
		page, err := vpcEndpointPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe VPC endpoints: %w", err)
		}
		for _, endpoint := range page.VpcEndpoints {
			name := extractNameTag(endpoint.Tags)
			if name == "Unknown" {
				name = derefString(endpoint.ServiceName)
			}
			if strings.TrimSpace(name) == "" {
				name = derefString(endpoint.VpcEndpointId)
			}
			subnetID := ""
			if len(endpoint.SubnetIds) > 0 {
				subnetID = endpoint.SubnetIds[0]
			}
			targets = append(targets, ReachabilityTarget{
				ID:          derefString(endpoint.VpcEndpointId),
				Name:        name,
				Type:        "VPC endpoints",
				VPCID:       derefString(endpoint.VpcId),
				SubnetID:    subnetID,
				Description: fmt.Sprintf("type=%s state=%s", endpoint.VpcEndpointType, endpoint.State),
			})
		}
	}

	vpcPeeringPaginator := ec2.NewDescribeVpcPeeringConnectionsPaginator(r.EC2Client, &ec2.DescribeVpcPeeringConnectionsInput{})
	for vpcPeeringPaginator.HasMorePages() {
		page, err := vpcPeeringPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe VPC peering connections: %w", err)
		}
		for _, peering := range page.VpcPeeringConnections {
			name := extractNameTag(peering.Tags)
			if name == "Unknown" {
				name = derefString(peering.VpcPeeringConnectionId)
			}
			requester := ""
			if peering.RequesterVpcInfo != nil {
				requester = derefString(peering.RequesterVpcInfo.VpcId)
			}
			accepter := ""
			if peering.AccepterVpcInfo != nil {
				accepter = derefString(peering.AccepterVpcInfo.VpcId)
			}
			targets = append(targets, ReachabilityTarget{
				ID:          derefString(peering.VpcPeeringConnectionId),
				Name:        name,
				Type:        "VPC peering connections",
				VPCID:       fallbackString(requester, accepter),
				Description: fmt.Sprintf("requester=%s accepter=%s status=%s", fallbackString(requester, "-"), fallbackString(accepter, "-"), fallbackString(derefStateReason(peering.Status), "unknown")),
			})
		}
	}

	transitGatewayPaginator := ec2.NewDescribeTransitGatewaysPaginator(r.EC2Client, &ec2.DescribeTransitGatewaysInput{})
	for transitGatewayPaginator.HasMorePages() {
		page, err := transitGatewayPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe transit gateways: %w", err)
		}
		for _, gateway := range page.TransitGateways {
			name := extractNameTag(gateway.Tags)
			if name == "Unknown" {
				name = derefString(gateway.TransitGatewayId)
			}
			targets = append(targets, ReachabilityTarget{
				ID:          derefString(gateway.TransitGatewayId),
				Name:        name,
				Type:        "Transit gateways",
				Description: fmt.Sprintf("state=%s", fallbackString(string(gateway.State), "unknown")),
			})
		}
	}

	transitAttachmentPaginator := ec2.NewDescribeTransitGatewayAttachmentsPaginator(r.EC2Client, &ec2.DescribeTransitGatewayAttachmentsInput{})
	for transitAttachmentPaginator.HasMorePages() {
		page, err := transitAttachmentPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe transit gateway attachments: %w", err)
		}
		for _, attachment := range page.TransitGatewayAttachments {
			name := extractNameTag(attachment.Tags)
			if name == "Unknown" {
				name = derefString(attachment.TransitGatewayAttachmentId)
			}
			vpcID := ""
			if attachment.ResourceType == types.TransitGatewayAttachmentResourceTypeVpc {
				vpcID = derefString(attachment.ResourceId)
			}
			targets = append(targets, ReachabilityTarget{
				ID:          derefString(attachment.TransitGatewayAttachmentId),
				Name:        name,
				Type:        "Transit gateway attachments",
				VPCID:       vpcID,
				Description: fmt.Sprintf("resource=%s/%s state=%s", fallbackString(string(attachment.ResourceType), "unknown"), fallbackString(derefString(attachment.ResourceId), "-"), fallbackString(string(attachment.State), "unknown")),
			})
		}
	}

	vpnGatewaysOut, err := r.EC2Client.DescribeVpnGateways(ctx, &ec2.DescribeVpnGatewaysInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe virtual private gateways: %w", err)
	}
	for _, gateway := range vpnGatewaysOut.VpnGateways {
		name := extractNameTag(gateway.Tags)
		if name == "Unknown" {
			name = derefString(gateway.VpnGatewayId)
		}
		vpcID := ""
		if len(gateway.VpcAttachments) > 0 {
			vpcID = derefString(gateway.VpcAttachments[0].VpcId)
		}
		targets = append(targets, ReachabilityTarget{
			ID:          derefString(gateway.VpnGatewayId),
			Name:        name,
			Type:        "Virtual private gateways",
			VPCID:       vpcID,
			Description: fmt.Sprintf("state=%s", fallbackString(string(gateway.State), "unknown")),
		})
	}

	serviceConfigInput := &ec2.DescribeVpcEndpointServiceConfigurationsInput{}
	for {
		serviceConfigsOut, err := r.EC2Client.DescribeVpcEndpointServiceConfigurations(ctx, serviceConfigInput)
		if err != nil {
			return nil, fmt.Errorf("failed to describe VPC endpoint services: %w", err)
		}
		for _, service := range serviceConfigsOut.ServiceConfigurations {
			name := extractNameTag(service.Tags)
			if name == "Unknown" {
				name = derefString(service.ServiceName)
			}
			if strings.TrimSpace(name) == "" {
				name = derefString(service.ServiceId)
			}
			targets = append(targets, ReachabilityTarget{
				ID:          derefString(service.ServiceId),
				Name:        name,
				Type:        "VPC endpoint services",
				Description: fmt.Sprintf("service=%s state=%s", fallbackString(derefString(service.ServiceName), "-"), fallbackString(string(service.ServiceState), "unknown")),
			})
		}
		if strings.TrimSpace(derefString(serviceConfigsOut.NextToken)) == "" {
			break
		}
		serviceConfigInput.NextToken = serviceConfigsOut.NextToken
	}

	sort.Slice(targets, func(i, j int) bool {
		left := normalizedSortKey(targets[i].Name, targets[i].ID)
		right := normalizedSortKey(targets[j].Name, targets[j].ID)
		if left == right {
			return targets[i].ID < targets[j].ID
		}
		return left < right
	})

	return targets, nil
}

func (r *AwsRepository) RunReachabilityAnalysis(
	ctx context.Context,
	source ReachabilityTarget,
	destination ReachabilityTarget,
	destinationIP string,
	protocol string,
	destinationPort int32,
) (*ReachabilityAnalysisResult, error) {
	uniclog.Debug("aws", "RunReachabilityAnalysis called", "source", source.ID, "destination", destination.ID, "destination_ip", destinationIP)

	if source.ID == "" {
		return nil, fmt.Errorf("source is required")
	}
	if destination.ID == "" && destinationIP == "" {
		return nil, fmt.Errorf("destination or destination IP is required")
	}
	if destinationIP != "" {
		ip := net.ParseIP(destinationIP)
		if ip == nil || ip.To4() == nil {
			return nil, fmt.Errorf("destination IP must be a valid IPv4 address")
		}
	}
	if destinationPort <= 0 {
		return nil, fmt.Errorf("destination port must be positive")
	}

	pathInput := &ec2.CreateNetworkInsightsPathInput{
		ClientToken: awssdk.String(strconv.FormatInt(time.Now().UnixNano(), 10)),
		Protocol:    types.Protocol(strings.ToLower(protocol)),
		Source:      awssdk.String(source.ID),
	}
	if destination.ID != "" {
		pathInput.Destination = awssdk.String(destination.ID)
		pathInput.DestinationPort = awssdk.Int32(destinationPort)
	} else {
		pathInput.FilterAtSource = &types.PathRequestFilter{
			DestinationAddress: awssdk.String(destinationIP),
			DestinationPortRange: &types.RequestFilterPortRange{
				FromPort: awssdk.Int32(destinationPort),
				ToPort:   awssdk.Int32(destinationPort),
			},
		}
	}

	pathOut, err := r.EC2Client.CreateNetworkInsightsPath(ctx, pathInput)
	if err != nil {
		return nil, formatReachabilityError("failed to create network insights path", err)
	}
	pathID := awssdk.ToString(pathOut.NetworkInsightsPath.NetworkInsightsPathId)
	analysisID := ""
	defer func() {
		r.cleanupReachabilityResources(pathID, analysisID)
	}()

	analysisOut, err := r.EC2Client.StartNetworkInsightsAnalysis(ctx, &ec2.StartNetworkInsightsAnalysisInput{
		NetworkInsightsPathId: awssdk.String(pathID),
	})
	if err != nil {
		return nil, formatReachabilityError("failed to start network insights analysis", err)
	}
	analysisID = awssdk.ToString(analysisOut.NetworkInsightsAnalysis.NetworkInsightsAnalysisId)

	for {
		describeOut, err := r.EC2Client.DescribeNetworkInsightsAnalyses(ctx, &ec2.DescribeNetworkInsightsAnalysesInput{
			NetworkInsightsAnalysisIds: []string{analysisID},
		})
		if err != nil {
			return nil, formatReachabilityError("failed to describe network insights analysis", err)
		}
		if len(describeOut.NetworkInsightsAnalyses) == 0 {
			return nil, fmt.Errorf("network insights analysis %s not found", analysisID)
		}

		analysis := describeOut.NetworkInsightsAnalyses[0]
		switch analysis.Status {
		case types.AnalysisStatusRunning:
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(750 * time.Millisecond):
				continue
			}
		case types.AnalysisStatusFailed:
			return nil, fmt.Errorf("network insights analysis failed: %s", fallbackString(awssdk.ToString(analysis.StatusMessage), "analysis failed"))
		default:
			return mapReachabilityAnalysis(pathID, analysisID, source, destination, destinationIP, string(protocol), destinationPort, analysis), nil
		}
	}
}

func (r *AwsRepository) cleanupReachabilityResources(pathID, analysisID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if analysisID != "" {
		if _, err := r.EC2Client.DeleteNetworkInsightsAnalysis(ctx, &ec2.DeleteNetworkInsightsAnalysisInput{
			NetworkInsightsAnalysisId: awssdk.String(analysisID),
		}); err != nil {
			uniclog.Error("aws", "failed to delete network insights analysis", "analysis_id", analysisID, "error", err)
		}
	}

	if pathID != "" {
		if _, err := r.EC2Client.DeleteNetworkInsightsPath(ctx, &ec2.DeleteNetworkInsightsPathInput{
			NetworkInsightsPathId: awssdk.String(pathID),
		}); err != nil {
			uniclog.Error("aws", "failed to delete network insights path", "path_id", pathID, "error", err)
		}
	}
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatReachabilityError(prefix string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: timed out waiting for Reachability Analyzer; try narrowing the target or retrying", prefix)
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s: analysis was cancelled", prefix)
	}

	statusPart := ""
	if respErr := (*smithyhttp.ResponseError)(nil); errors.As(err, &respErr) {
		statusPart = fmt.Sprintf(" [status:%d", respErr.HTTPStatusCode())
		if requestID := reachabilityRequestID(respErr.HTTPResponse()); requestID != "" {
			statusPart += fmt.Sprintf(" request-id:%s", requestID)
		}
		statusPart += "]"
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		message := fallbackString(apiErr.ErrorMessage(), err.Error())
		switch code {
		case "UnauthorizedOperation", "AccessDenied", "AccessDeniedException", "AuthFailure":
			return fmt.Errorf("%s: %s: %s%s (required EC2 permissions: CreateNetworkInsightsPath, StartNetworkInsightsAnalysis, DescribeNetworkInsightsAnalyses, DeleteNetworkInsightsAnalysis, DeleteNetworkInsightsPath)", prefix, code, message, statusPart)
		case "InvalidParameterValue", "InvalidParameterCombination", "InvalidParameter":
			return fmt.Errorf("%s: %s: %s%s (check that the selected source and destination are supported Reachability Analyzer resources in the selected region)", prefix, code, message, statusPart)
		}
		return fmt.Errorf("%s: %s: %s%s", prefix, code, message, statusPart)
	}

	if statusPart != "" {
		return fmt.Errorf("%s: %v%s", prefix, err, statusPart)
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

func derefStateReason(reason *types.VpcPeeringConnectionStateReason) string {
	if reason == nil {
		return ""
	}
	return string(reason.Code)
}

func describeIGWAttachment(attachments []types.InternetGatewayAttachment) string {
	if len(attachments) == 0 {
		return "detached"
	}
	attachment := attachments[0]
	return fmt.Sprintf("vpc=%s state=%s", fallbackString(derefString(attachment.VpcId), "-"), fallbackString(string(attachment.State), "unknown"))
}

func reachabilityRequestID(resp *smithyhttp.Response) string {
	if resp == nil || resp.Response == nil {
		return ""
	}
	for _, key := range []string{"x-amzn-RequestId", "x-amzn-requestid", "x-amz-request-id"} {
		if value := resp.Response.Header.Get(key); value != "" {
			return value
		}
	}
	return ""
}

func mapReachabilityAnalysis(
	pathID, analysisID string,
	source ReachabilityTarget,
	destination ReachabilityTarget,
	destinationIP, protocol string,
	destinationPort int32,
	analysis types.NetworkInsightsAnalysis,
) *ReachabilityAnalysisResult {
	result := &ReachabilityAnalysisResult{
		PathID:           pathID,
		AnalysisID:       analysisID,
		Status:           string(analysis.Status),
		StatusMessage:    awssdk.ToString(analysis.StatusMessage),
		NetworkPathFound: awssdk.ToBool(analysis.NetworkPathFound),
		WarningMessage:   awssdk.ToString(analysis.WarningMessage),
		Source:           source,
		Destination:      destination,
		DestinationIP:    destinationIP,
		Protocol:         strings.ToUpper(protocol),
		DestinationPort:  destinationPort,
	}

	for _, component := range analysis.ForwardPathComponents {
		result.ForwardPath = append(result.ForwardPath, mapPathComponent(component))
	}
	for _, explanation := range analysis.Explanations {
		result.Explanations = append(result.Explanations, mapExplanation(explanation))
	}
	return result
}

func mapPathComponent(component types.PathComponent) ReachabilityPathComponent {
	item := ReachabilityPathComponent{
		Sequence: awssdk.ToInt32(component.SequenceNumber),
		Title:    analysisComponentTitle(component.Component),
	}

	if title := analysisComponentTitle(component.AttachedTo); title != "" {
		item.Details = append(item.Details, "attached to "+title)
	}
	if title := analysisComponentTitle(component.Subnet); title != "" {
		item.Details = append(item.Details, "subnet "+title)
	}
	if title := analysisComponentTitle(component.Vpc); title != "" {
		item.Details = append(item.Details, "vpc "+title)
	}
	if component.InboundHeader != nil {
		item.Details = append(item.Details, formatPacketHeader("in", component.InboundHeader))
	}
	if component.OutboundHeader != nil {
		item.Details = append(item.Details, formatPacketHeader("out", component.OutboundHeader))
	}
	for _, explanation := range component.Explanations {
		item.Explanations = append(item.Explanations, mapExplanation(explanation).Summary)
	}
	return item
}

func mapExplanation(explanation types.Explanation) ReachabilityExplanation {
	code := awssdk.ToString(explanation.ExplanationCode)
	summaryParts := []string{code}
	if title := analysisComponentTitle(explanation.Component); title != "" {
		summaryParts = append(summaryParts, "at "+title)
	}

	details := make([]string, 0, 6)
	if title := analysisComponentTitle(explanation.SecurityGroup); title != "" {
		details = append(details, "security group: "+title)
	}
	if title := analysisComponentTitle(explanation.RouteTable); title != "" {
		details = append(details, "route table: "+title)
	}
	if title := analysisComponentTitle(explanation.NetworkInterface); title != "" {
		details = append(details, "network interface: "+title)
	}
	if title := analysisComponentTitle(explanation.Subnet); title != "" {
		details = append(details, "subnet: "+title)
	}
	if explanation.Direction != nil {
		details = append(details, "direction: "+awssdk.ToString(explanation.Direction))
	}
	if explanation.Port != nil {
		details = append(details, fmt.Sprintf("port: %d", awssdk.ToInt32(explanation.Port)))
	}
	if explanation.Address != nil {
		details = append(details, "address: "+awssdk.ToString(explanation.Address))
	}
	if explanation.State != nil {
		details = append(details, "state: "+awssdk.ToString(explanation.State))
	}
	if explanation.MissingComponent != nil {
		details = append(details, "missing: "+awssdk.ToString(explanation.MissingComponent))
	}

	return ReachabilityExplanation{
		Code:    code,
		Summary: strings.Join(summaryParts, " "),
		Details: details,
	}
}

func analysisComponentTitle(component *types.AnalysisComponent) string {
	if component == nil {
		return ""
	}
	name := awssdk.ToString(component.Name)
	id := awssdk.ToString(component.Id)
	switch {
	case name != "" && id != "":
		return fmt.Sprintf("%s (%s)", name, id)
	case name != "":
		return name
	default:
		return id
	}
}

func formatPacketHeader(direction string, header *types.AnalysisPacketHeader) string {
	if header == nil {
		return ""
	}
	var parts []string
	if header.Protocol != nil {
		parts = append(parts, strings.ToUpper(awssdk.ToString(header.Protocol)))
	}
	if len(header.SourcePortRanges) > 0 {
		parts = append(parts, "src:"+formatPortRange(header.SourcePortRanges[0]))
	}
	if len(header.DestinationPortRanges) > 0 {
		parts = append(parts, "dst:"+formatPortRange(header.DestinationPortRanges[0]))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("%sbound %s", direction, strings.Join(parts, " "))
}

func formatPortRange(portRange types.PortRange) string {
	from := awssdk.ToInt32(portRange.From)
	to := awssdk.ToInt32(portRange.To)
	if from == to {
		return fmt.Sprintf("%d", from)
	}
	return fmt.Sprintf("%d-%d", from, to)
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
