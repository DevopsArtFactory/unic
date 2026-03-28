package aws

import (
	"fmt"
	"strings"
)

// SecurityGroup holds essential information about an EC2 security group.
type SecurityGroup struct {
	GroupID      string
	Name         string
	Description  string
	VPCID        string
	IsDefault    bool
	IngressRules []SecurityGroupRule
	EgressRules  []SecurityGroupRule
}

// DisplayTitle returns a formatted string for list display.
func (sg SecurityGroup) DisplayTitle() string {
	defaultMark := ""
	if sg.IsDefault {
		defaultMark = " [default]"
	}
	return fmt.Sprintf("%s (%s) - %s%s", sg.Name, sg.GroupID, sg.VPCID, defaultMark)
}

// FilterText returns a lowercase string for keyword matching.
func (sg SecurityGroup) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s", sg.Name, sg.GroupID, sg.VPCID, sg.Description))
}

// SecurityGroupRule represents an inbound or outbound rule.
type SecurityGroupRule struct {
	Protocol       string
	FromPort       int32
	ToPort         int32
	CIDRV4         string
	CIDRV6         string
	ReferencedSGID string
	Description    string
}

// DisplayTitle returns a formatted string for rule display.
func (r SecurityGroupRule) DisplayTitle() string {
	proto := r.Protocol
	if proto == "-1" {
		proto = "All"
	}

	portRange := "All"
	if r.Protocol != "-1" {
		if r.FromPort == r.ToPort {
			portRange = fmt.Sprintf("%d", r.FromPort)
		} else {
			portRange = fmt.Sprintf("%d-%d", r.FromPort, r.ToPort)
		}
	}

	source := r.CIDRV4
	if source == "" {
		source = r.CIDRV6
	}
	if source == "" && r.ReferencedSGID != "" {
		source = r.ReferencedSGID
	}
	if source == "" {
		source = "-"
	}

	return fmt.Sprintf("%s  %s  %s", proto, portRange, source)
}
