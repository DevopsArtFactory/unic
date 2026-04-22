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
	VPCID            string
	SubnetID         string
	PrivateIP        string
	PublicIP         string
	LaunchTime       time.Time
	PlatformDetails  string
	IAMProfile       string
	Tags             map[string]string
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
		i.VPCID,
		i.SubnetID,
		i.PrivateIP,
		i.PublicIP,
		i.PlatformDetails,
		i.IAMProfile,
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
