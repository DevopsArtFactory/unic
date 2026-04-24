package aws

import (
	"fmt"
	"strings"
)

// EKSCluster captures the summary needed for browsing clusters from the TUI.
type EKSCluster struct {
	Name                  string
	ARN                   string
	Version               string
	Status                string
	EndpointPublicAccess  bool
	EndpointPrivateAccess bool
}

func (c EKSCluster) DisplayTitle() string {
	return fmt.Sprintf("%-28s  v%-8s  %-10s  %s", c.Name, c.Version, c.Status, c.EndpointVisibility())
}

func (c EKSCluster) FilterText() string {
	return strings.ToLower(strings.Join([]string{c.Name, c.Version, c.Status, c.ARN, c.EndpointVisibility()}, " "))
}

func (c EKSCluster) EndpointVisibility() string {
	return fmt.Sprintf("pub:%s priv:%s", enabledMarker(c.EndpointPublicAccess), enabledMarker(c.EndpointPrivateAccess))
}

// EKSHealthIssue describes one managed node group health issue.
type EKSHealthIssue struct {
	Code        string
	Message     string
	ResourceIDs []string
}

func (i EKSHealthIssue) Summary() string {
	parts := []string{i.Code}
	if strings.TrimSpace(i.Message) != "" {
		parts = append(parts, i.Message)
	}
	if len(i.ResourceIDs) > 0 {
		parts = append(parts, fmt.Sprintf("resources:%s", strings.Join(i.ResourceIDs, ",")))
	}
	return strings.Join(parts, " • ")
}

// EKSNodeGroup captures managed node group state for the TUI.
type EKSNodeGroup struct {
	ClusterName    string
	Name           string
	ARN            string
	Status         string
	Version        string
	ReleaseVersion string
	AmiType        string
	CapacityType   string
	InstanceTypes  []string
	DesiredSize    int32
	MinSize        int32
	MaxSize        int32
	HealthIssues   []EKSHealthIssue
}

func (n EKSNodeGroup) DisplayTitle() string {
	return fmt.Sprintf("%-26s  %-12s  desired:%-3d min:%-3d max:%-3d", n.Name, n.Status, n.DesiredSize, n.MinSize, n.MaxSize)
}

func (n EKSNodeGroup) FilterText() string {
	parts := []string{n.ClusterName, n.Name, n.Status, n.Version, n.ReleaseVersion, n.AmiType, n.CapacityType, n.ARN, strings.Join(n.InstanceTypes, " ")}
	for _, issue := range n.HealthIssues {
		parts = append(parts, issue.Code, issue.Message, strings.Join(issue.ResourceIDs, " "))
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func (n EKSNodeGroup) HealthSummary() string {
	if len(n.HealthIssues) == 0 {
		return "healthy"
	}
	return fmt.Sprintf("%d issue(s)", len(n.HealthIssues))
}

func (n EKSNodeGroup) InstanceTypesLabel() string {
	if len(n.InstanceTypes) == 0 {
		return "-"
	}
	return strings.Join(n.InstanceTypes, ", ")
}

func enabledMarker(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
