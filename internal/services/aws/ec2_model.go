package aws

import (
	"fmt"
	"strings"
	"time"
)

// EC2Instance holds the essential information about an EC2 instance.
type EC2Instance struct {
	InstanceID       string
	Name             string
	State            string
	InstanceType     string
	AvailabilityZone string
	Region           string
	VPCID            string
	SubnetID         string
	PrivateIP        string
	PublicIP         string
	SecurityGroups   []EC2InstanceSecurityGroup
	LaunchTime       time.Time
	PlatformDetails  string
	IAMProfile       string
	Tags             map[string]string
}

// EC2InstanceSecurityGroup identifies a security group attached to an instance.
type EC2InstanceSecurityGroup struct {
	GroupID string
	Name    string
}

// FilterText returns a lowercase string combining name, instance ID, and IP
// for keyword matching.
func (i EC2Instance) FilterText() string {
	parts := []string{
		i.Name,
		i.InstanceID,
		i.State,
		i.InstanceType,
		i.AvailabilityZone,
		i.Region,
		i.VPCID,
		i.SubnetID,
		i.PrivateIP,
		i.PublicIP,
		i.PlatformDetails,
		i.IAMProfile,
	}
	for _, sg := range i.SecurityGroups {
		parts = append(parts, sg.GroupID, sg.Name)
	}
	for key, value := range i.Tags {
		parts = append(parts, key, value)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// DisplayTitle returns a formatted string for list display.
func (i EC2Instance) DisplayTitle() string {
	name := i.Name
	if name == "" || name == "Unknown" {
		name = "-"
	}
	network := i.PrivateIP
	if network == "" {
		network = i.PublicIP
	}
	if network == "" {
		network = "-"
	}
	instanceType := i.InstanceType
	if instanceType == "" {
		instanceType = "-"
	}
	state := i.State
	if state == "" {
		state = "-"
	}
	return fmt.Sprintf("%s (%s) %s [%s] - %s", name, i.InstanceID, instanceType, state, network)
}

// EC2InstanceRelationships groups resources connected to an EC2 instance.
type EC2InstanceRelationships struct {
	InstanceID     string
	SecurityGroups []SecurityGroup
	AutoScaling    *EC2AutoScalingGroup
	TargetGroups   []EC2TargetGroup
	LoadBalancers  []EC2LoadBalancer
	Listeners      []EC2Listener
	Errors         []EC2RelationshipError
}

// EC2RelationshipError records a failed relationship lookup without discarding partial results.
type EC2RelationshipError struct {
	Section string
	Err     error
}

// EC2AutoScalingGroup describes the Auto Scaling group that owns an instance.
type EC2AutoScalingGroup struct {
	Name              string
	ARN               string
	LifecycleState    string
	HealthStatus      string
	DesiredCapacity   int32
	MinSize           int32
	MaxSize           int32
	TargetGroupARNs   []string
	LoadBalancerNames []string
}

func (a EC2AutoScalingGroup) DisplayTitle() string {
	return fmt.Sprintf("%s desired:%d min:%d max:%d [%s/%s]", a.Name, a.DesiredCapacity, a.MinSize, a.MaxSize, valueOrDash(a.LifecycleState), valueOrDash(a.HealthStatus))
}

func (a EC2AutoScalingGroup) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %v %v", a.Name, a.ARN, a.LifecycleState, a.HealthStatus, a.TargetGroupARNs, a.LoadBalancerNames))
}

// EC2TargetGroup describes a target group where the instance is registered.
type EC2TargetGroup struct {
	ARN               string
	Name              string
	Protocol          string
	Port              int32
	VPCID             string
	TargetType        string
	HealthState       string
	HealthReason      string
	HealthDescription string
	LoadBalancerARNs  []string
}

func (t EC2TargetGroup) DisplayTitle() string {
	return fmt.Sprintf("%s %s:%d [%s]", valueOrDash(t.Name), valueOrDash(t.Protocol), t.Port, valueOrDash(t.HealthState))
}

func (t EC2TargetGroup) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s %v", t.Name, t.ARN, t.Protocol, t.VPCID, t.TargetType, t.HealthState, t.LoadBalancerARNs))
}

// EC2LoadBalancer describes a load balancer associated through a target group.
type EC2LoadBalancer struct {
	ARN     string
	Name    string
	DNSName string
	Type    string
	Scheme  string
	State   string
	VPCID   string
}

func (l EC2LoadBalancer) DisplayTitle() string {
	return fmt.Sprintf("%s %s [%s] - %s", valueOrDash(l.Name), valueOrDash(l.Type), valueOrDash(l.State), valueOrDash(l.DNSName))
}

func (l EC2LoadBalancer) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s %s", l.Name, l.ARN, l.DNSName, l.Type, l.Scheme, l.State, l.VPCID))
}

// EC2Listener describes a load balancer listener and its rule count.
type EC2Listener struct {
	ARN              string
	LoadBalancerARN  string
	LoadBalancerName string
	Protocol         string
	Port             int32
	RuleCount        int
	DefaultAction    string
}

func (l EC2Listener) DisplayTitle() string {
	return fmt.Sprintf("%s:%d on %s (%d rules)", valueOrDash(l.Protocol), l.Port, valueOrDash(l.LoadBalancerName), l.RuleCount)
}

func (l EC2Listener) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %d %s", l.ARN, l.LoadBalancerARN, l.LoadBalancerName, l.Protocol, l.Port, l.DefaultAction))
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
