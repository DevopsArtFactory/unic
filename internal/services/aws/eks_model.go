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

// EKSAddonIssue describes one managed add-on health issue.
type EKSAddonIssue struct {
	Code        string
	Message     string
	ResourceIDs []string
}

func (i EKSAddonIssue) Summary() string {
	parts := []string{i.Code}
	if strings.TrimSpace(i.Message) != "" {
		parts = append(parts, i.Message)
	}
	if len(i.ResourceIDs) > 0 {
		parts = append(parts, fmt.Sprintf("resources:%s", strings.Join(i.ResourceIDs, ",")))
	}
	return strings.Join(parts, " • ")
}

// EKSAddon captures managed add-on status for the TUI.
type EKSAddon struct {
	ClusterName           string
	Name                  string
	ARN                   string
	Version               string
	Status                string
	Owner                 string
	Publisher             string
	ServiceAccountRoleARN string
	HealthIssues          []EKSAddonIssue
}

func (a EKSAddon) DisplayTitle() string {
	return fmt.Sprintf("%-26s  %-18s  %-12s  %s", a.Name, firstNonEmptyString(a.Version, "-"), firstNonEmptyString(a.Status, "-"), a.HealthSummary())
}

func (a EKSAddon) FilterText() string {
	parts := []string{a.ClusterName, a.Name, a.ARN, a.Version, a.Status, a.Owner, a.Publisher, a.ServiceAccountRoleARN}
	for _, issue := range a.HealthIssues {
		parts = append(parts, issue.Code, issue.Message, strings.Join(issue.ResourceIDs, " "))
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func (a EKSAddon) HealthSummary() string {
	if len(a.HealthIssues) == 0 {
		return "healthy"
	}
	return fmt.Sprintf("%d issue(s)", len(a.HealthIssues))
}

func (a EKSAddon) NeedsAttention() bool {
	return !strings.EqualFold(strings.TrimSpace(a.Status), "ACTIVE") || len(a.HealthIssues) > 0
}

func (a EKSAddon) StatusSummary() string {
	if len(a.HealthIssues) > 0 {
		return a.HealthSummary()
	}
	if strings.TrimSpace(a.Status) == "" {
		return "unknown status"
	}
	if !strings.EqualFold(a.Status, "ACTIVE") {
		return strings.ToLower(a.Status)
	}
	return "healthy"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func enabledMarker(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
