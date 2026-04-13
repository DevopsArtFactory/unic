package aws

import (
	"fmt"
	"strings"
)

// VPC holds essential information about an AWS VPC.
type VPC struct {
	VPCID     string
	Name      string
	CIDR      string
	IsDefault bool
}

// DisplayTitle returns a formatted string for list display.
func (v VPC) DisplayTitle() string {
	defaultMark := ""
	if v.IsDefault {
		defaultMark = " [default]"
	}
	return fmt.Sprintf("%s (%s) - %s%s", v.Name, v.VPCID, v.CIDR, defaultMark)
}

// FilterText returns a lowercase string for keyword matching.
func (v VPC) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s", v.Name, v.VPCID, v.CIDR))
}

// Subnet holds essential information about a VPC subnet.
type Subnet struct {
	SubnetID         string
	Name             string
	CIDR             string
	AvailabilityZone string
	AvailableIPCount int32
	TotalIPCount     int32
	AvailableIPs     []string
}

// DisplayTitle returns a formatted string for list display.
func (s Subnet) DisplayTitle() string {
	return fmt.Sprintf("%s (%s) - %s | %s | %d IPs available",
		s.Name, s.SubnetID, s.CIDR, s.AvailabilityZone, s.AvailableIPCount)
}

// FilterText returns a lowercase string for keyword matching.
func (s Subnet) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s", s.Name, s.SubnetID, s.CIDR, s.AvailabilityZone))
}

// ReachabilityTarget represents a source or destination candidate for network analysis.
type ReachabilityTarget struct {
	ID          string
	Name        string
	Type        string
	VPCID       string
	SubnetID    string
	PrivateIP   string
	Description string
	ManualIP    bool
}

func (t ReachabilityTarget) DisplayTitle() string {
	if t.ManualIP {
		return t.Name
	}

	parts := []string{fmt.Sprintf("%s (%s)", t.Name, t.ID)}
	if t.PrivateIP != "" {
		parts = append(parts, t.PrivateIP)
	}
	if t.Description != "" {
		parts = append(parts, t.Description)
	}
	return strings.Join(parts, " | ")
}

func (t ReachabilityTarget) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s", t.Name, t.ID, t.Type, t.VPCID, t.SubnetID, t.PrivateIP))
}

type ReachabilityPathComponent struct {
	Sequence     int32
	Title        string
	Details      []string
	Explanations []string
}

type ReachabilityExplanation struct {
	Code    string
	Summary string
	Details []string
}

type ReachabilityAnalysisResult struct {
	PathID           string
	AnalysisID       string
	Status           string
	StatusMessage    string
	NetworkPathFound bool
	WarningMessage   string
	Source           ReachabilityTarget
	Destination      ReachabilityTarget
	DestinationIP    string
	Protocol         string
	DestinationPort  int32
	ForwardPath      []ReachabilityPathComponent
	Explanations     []ReachabilityExplanation
}
