package aws

import (
	"context"
	"fmt"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// DescribeEC2InstanceRelationships returns resources connected to an EC2 instance.
// Permission failures are returned in result.Errors so the TUI can render partial data.
func (r *AwsRepository) DescribeEC2InstanceRelationships(ctx context.Context, inst EC2Instance) (*EC2InstanceRelationships, error) {
	if inst.InstanceID == "" {
		return nil, fmt.Errorf("instance ID is required")
	}
	result := &EC2InstanceRelationships{InstanceID: inst.InstanceID}

	if len(inst.SecurityGroups) > 0 {
		securityGroups, err := r.describeRelatedSecurityGroups(ctx, inst.SecurityGroups)
		if err != nil {
			result.Errors = append(result.Errors, EC2RelationshipError{Section: "security groups", Err: err})
		} else {
			result.SecurityGroups = securityGroups
		}
	}

	asg, err := r.describeRelatedAutoScalingGroup(ctx, inst)
	result.AutoScaling = asg
	if err != nil {
		result.Errors = append(result.Errors, EC2RelationshipError{Section: "auto scaling", Err: err})
	}

	targetGroups, err := r.describeRelatedTargetGroups(ctx, inst.InstanceID)
	result.TargetGroups = targetGroups
	if err != nil {
		result.Errors = append(result.Errors, EC2RelationshipError{Section: "target groups", Err: err})
	}

	loadBalancers, listeners, err := r.describeRelatedLoadBalancersAndListeners(ctx, targetGroups)
	if err != nil {
		result.Errors = append(result.Errors, EC2RelationshipError{Section: "load balancers/listeners", Err: err})
	}
	result.LoadBalancers = loadBalancers
	result.Listeners = listeners

	return result, nil
}

func (r *AwsRepository) describeRelatedSecurityGroups(ctx context.Context, attached []EC2InstanceSecurityGroup) ([]SecurityGroup, error) {
	ids := make([]string, 0, len(attached))
	for _, sg := range attached {
		if sg.GroupID != "" {
			ids = append(ids, sg.GroupID)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	out, err := r.EC2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{GroupIds: ids})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance security groups: %w", err)
	}
	groups := make([]SecurityGroup, 0, len(out.SecurityGroups))
	for _, sg := range out.SecurityGroups {
		group := SecurityGroup{
			GroupID:      awssdk.ToString(sg.GroupId),
			Name:         awssdk.ToString(sg.GroupName),
			Description:  awssdk.ToString(sg.Description),
			VPCID:        awssdk.ToString(sg.VpcId),
			IsDefault:    awssdk.ToString(sg.GroupName) == "default",
			IngressRules: parseRules(sg.IpPermissions),
			EgressRules:  parseRules(sg.IpPermissionsEgress),
		}
		groups = append(groups, group)
	}
	sortSecurityGroups(groups)
	return groups, nil
}

func (r *AwsRepository) describeRelatedAutoScalingGroup(ctx context.Context, inst EC2Instance) (*EC2AutoScalingGroup, error) {
	if r.AutoScalingClient == nil {
		return autoScalingGroupFromTag(inst), nil
	}
	out, err := r.AutoScalingClient.DescribeAutoScalingInstances(ctx, &autoscaling.DescribeAutoScalingInstancesInput{
		InstanceIds: []string{inst.InstanceID},
	})
	if err != nil {
		if tagGroup := autoScalingGroupFromTag(inst); tagGroup != nil {
			return tagGroup, fmt.Errorf("failed to describe Auto Scaling instance: %w", err)
		}
		return nil, fmt.Errorf("failed to describe Auto Scaling instance: %w", err)
	}
	if len(out.AutoScalingInstances) == 0 {
		return autoScalingGroupFromTag(inst), nil
	}

	instance := out.AutoScalingInstances[0]
	group := EC2AutoScalingGroup{
		Name:           awssdk.ToString(instance.AutoScalingGroupName),
		LifecycleState: awssdk.ToString(instance.LifecycleState),
		HealthStatus:   awssdk.ToString(instance.HealthStatus),
	}
	if group.Name == "" {
		return &group, nil
	}

	groupOut, err := r.AutoScalingClient.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{group.Name},
	})
	if err != nil {
		return &group, fmt.Errorf("failed to describe Auto Scaling group %s: %w", group.Name, err)
	}
	if len(groupOut.AutoScalingGroups) > 0 {
		mergeAutoScalingGroupDetails(&group, groupOut.AutoScalingGroups[0])
	}
	return &group, nil
}

func autoScalingGroupFromTag(inst EC2Instance) *EC2AutoScalingGroup {
	if inst.Tags == nil {
		return nil
	}
	name := inst.Tags["aws:autoscaling:groupName"]
	if name == "" {
		return nil
	}
	return &EC2AutoScalingGroup{Name: name}
}

func mergeAutoScalingGroupDetails(group *EC2AutoScalingGroup, sdkGroup asgtypes.AutoScalingGroup) {
	group.ARN = awssdk.ToString(sdkGroup.AutoScalingGroupARN)
	group.DesiredCapacity = awssdk.ToInt32(sdkGroup.DesiredCapacity)
	group.MinSize = awssdk.ToInt32(sdkGroup.MinSize)
	group.MaxSize = awssdk.ToInt32(sdkGroup.MaxSize)
	group.TargetGroupARNs = append([]string(nil), sdkGroup.TargetGroupARNs...)
	group.LoadBalancerNames = append([]string(nil), sdkGroup.LoadBalancerNames...)
	sort.Strings(group.TargetGroupARNs)
	sort.Strings(group.LoadBalancerNames)
}

func (r *AwsRepository) describeRelatedTargetGroups(ctx context.Context, instanceID string) ([]EC2TargetGroup, error) {
	if r.ELBv2Client == nil {
		return nil, nil
	}
	var groups []EC2TargetGroup
	var nextToken *string
	for {
		out, err := r.ELBv2Client.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
			PageSize: awssdk.Int32(400),
			Marker:   nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe target groups: %w", err)
		}
		for _, tg := range out.TargetGroups {
			if string(tg.TargetType) != "instance" {
				continue
			}
			matched, healthState, healthReason, healthDescription, err := r.targetGroupHasInstance(ctx, awssdk.ToString(tg.TargetGroupArn), instanceID)
			if err != nil {
				return groups, err
			}
			if matched {
				groups = append(groups, targetGroupFromSDK(tg, healthState, healthReason, healthDescription))
			}
		}
		if out.NextMarker == nil || awssdk.ToString(out.NextMarker) == "" {
			break
		}
		nextToken = out.NextMarker
	}
	sort.Slice(groups, func(i, j int) bool {
		left := normalizedSortKey(groups[i].Name, groups[i].ARN)
		right := normalizedSortKey(groups[j].Name, groups[j].ARN)
		if left == right {
			return groups[i].ARN < groups[j].ARN
		}
		return left < right
	})
	return groups, nil
}

func (r *AwsRepository) targetGroupHasInstance(ctx context.Context, targetGroupARN, instanceID string) (bool, string, string, string, error) {
	out, err := r.ELBv2Client.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: awssdk.String(targetGroupARN),
	})
	if err != nil {
		return false, "", "", "", fmt.Errorf("failed to describe target health for %s: %w", targetGroupARN, err)
	}
	for _, target := range out.TargetHealthDescriptions {
		if target.Target == nil || awssdk.ToString(target.Target.Id) != instanceID {
			continue
		}
		if target.TargetHealth == nil {
			return true, "", "", "", nil
		}
		return true,
			string(target.TargetHealth.State),
			string(target.TargetHealth.Reason),
			awssdk.ToString(target.TargetHealth.Description),
			nil
	}
	return false, "", "", "", nil
}

func targetGroupFromSDK(tg elbtypes.TargetGroup, healthState, healthReason, healthDescription string) EC2TargetGroup {
	return EC2TargetGroup{
		ARN:               awssdk.ToString(tg.TargetGroupArn),
		Name:              awssdk.ToString(tg.TargetGroupName),
		Protocol:          string(tg.Protocol),
		Port:              awssdk.ToInt32(tg.Port),
		VPCID:             awssdk.ToString(tg.VpcId),
		TargetType:        string(tg.TargetType),
		HealthState:       healthState,
		HealthReason:      healthReason,
		HealthDescription: healthDescription,
		LoadBalancerARNs:  append([]string(nil), tg.LoadBalancerArns...),
	}
}

func (r *AwsRepository) describeRelatedLoadBalancersAndListeners(ctx context.Context, targetGroups []EC2TargetGroup) ([]EC2LoadBalancer, []EC2Listener, error) {
	if r.ELBv2Client == nil || len(targetGroups) == 0 {
		return nil, nil, nil
	}
	arns := uniqueLoadBalancerARNs(targetGroups)
	if len(arns) == 0 {
		return nil, nil, nil
	}
	out, err := r.ELBv2Client.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{
		LoadBalancerArns: arns,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to describe load balancers: %w", err)
	}

	loadBalancers := make([]EC2LoadBalancer, 0, len(out.LoadBalancers))
	listeners := make([]EC2Listener, 0)
	for _, lb := range out.LoadBalancers {
		mapped := loadBalancerFromSDK(lb)
		loadBalancers = append(loadBalancers, mapped)
		lbListeners, err := r.describeListenersForLoadBalancer(ctx, mapped)
		if err != nil {
			return loadBalancers, listeners, err
		}
		listeners = append(listeners, lbListeners...)
	}
	sortLoadBalancers(loadBalancers)
	sortListeners(listeners)
	return loadBalancers, listeners, nil
}

func uniqueLoadBalancerARNs(targetGroups []EC2TargetGroup) []string {
	seen := make(map[string]struct{})
	for _, tg := range targetGroups {
		for _, arn := range tg.LoadBalancerARNs {
			if arn != "" {
				seen[arn] = struct{}{}
			}
		}
	}
	arns := make([]string, 0, len(seen))
	for arn := range seen {
		arns = append(arns, arn)
	}
	sort.Strings(arns)
	return arns
}

func loadBalancerFromSDK(lb elbtypes.LoadBalancer) EC2LoadBalancer {
	state := ""
	if lb.State != nil {
		state = string(lb.State.Code)
	}
	return EC2LoadBalancer{
		ARN:     awssdk.ToString(lb.LoadBalancerArn),
		Name:    awssdk.ToString(lb.LoadBalancerName),
		DNSName: awssdk.ToString(lb.DNSName),
		Type:    string(lb.Type),
		Scheme:  string(lb.Scheme),
		State:   state,
		VPCID:   awssdk.ToString(lb.VpcId),
	}
}

func (r *AwsRepository) describeListenersForLoadBalancer(ctx context.Context, lb EC2LoadBalancer) ([]EC2Listener, error) {
	out, err := r.ELBv2Client.DescribeListeners(ctx, &elasticloadbalancingv2.DescribeListenersInput{
		LoadBalancerArn: awssdk.String(lb.ARN),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe listeners for %s: %w", lb.Name, err)
	}
	listeners := make([]EC2Listener, 0, len(out.Listeners))
	for _, listener := range out.Listeners {
		ruleCount, err := r.listenerRuleCount(ctx, awssdk.ToString(listener.ListenerArn))
		if err != nil {
			return listeners, err
		}
		listeners = append(listeners, listenerFromSDK(listener, lb.Name, ruleCount))
	}
	return listeners, nil
}

func (r *AwsRepository) listenerRuleCount(ctx context.Context, listenerARN string) (int, error) {
	out, err := r.ELBv2Client.DescribeRules(ctx, &elasticloadbalancingv2.DescribeRulesInput{
		ListenerArn: awssdk.String(listenerARN),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to describe listener rules for %s: %w", listenerARN, err)
	}
	return len(out.Rules), nil
}

func listenerFromSDK(listener elbtypes.Listener, loadBalancerName string, ruleCount int) EC2Listener {
	defaultAction := ""
	if len(listener.DefaultActions) > 0 {
		defaultAction = string(listener.DefaultActions[0].Type)
	}
	return EC2Listener{
		ARN:              awssdk.ToString(listener.ListenerArn),
		LoadBalancerARN:  awssdk.ToString(listener.LoadBalancerArn),
		LoadBalancerName: loadBalancerName,
		Protocol:         string(listener.Protocol),
		Port:             awssdk.ToInt32(listener.Port),
		RuleCount:        ruleCount,
		DefaultAction:    defaultAction,
	}
}

func sortSecurityGroups(groups []SecurityGroup) {
	sort.Slice(groups, func(i, j int) bool {
		left := normalizedSortKey(groups[i].Name, groups[i].GroupID)
		right := normalizedSortKey(groups[j].Name, groups[j].GroupID)
		if left == right {
			return groups[i].GroupID < groups[j].GroupID
		}
		return left < right
	})
}

func sortLoadBalancers(loadBalancers []EC2LoadBalancer) {
	sort.Slice(loadBalancers, func(i, j int) bool {
		left := normalizedSortKey(loadBalancers[i].Name, loadBalancers[i].ARN)
		right := normalizedSortKey(loadBalancers[j].Name, loadBalancers[j].ARN)
		if left == right {
			return loadBalancers[i].ARN < loadBalancers[j].ARN
		}
		return left < right
	})
}

func sortListeners(listeners []EC2Listener) {
	sort.Slice(listeners, func(i, j int) bool {
		left := normalizedSortKey(listeners[i].LoadBalancerName, listeners[i].Protocol, fmt.Sprint(listeners[i].Port))
		right := normalizedSortKey(listeners[j].LoadBalancerName, listeners[j].Protocol, fmt.Sprint(listeners[j].Port))
		if left == right {
			return listeners[i].ARN < listeners[j].ARN
		}
		return left < right
	})
}
