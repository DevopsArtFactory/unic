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

// EKSAddon captures managed add-on status for the TUI.
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

// EKSAddon captures managed add-on status for the TUI and upgrade readiness checks.
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

type EKSUpgradeInsight struct {
	ID                string
	Name              string
	Category          string
	KubernetesVersion string
	Status            string
	Reason            string
}

// EKSUpgradeFinding describes one readiness signal.
type EKSUpgradeFinding struct {
	Severity string
	Subject  string
	Current  string
	Expected string
	Message  string
}

func (f EKSUpgradeFinding) Summary() string {
	parts := []string{f.Subject}
	if strings.TrimSpace(f.Current) != "" || strings.TrimSpace(f.Expected) != "" {
		parts = append(parts, fmt.Sprintf("%s -> %s", firstNonEmptyString(f.Current, "-"), firstNonEmptyString(f.Expected, "-")))
	}
	if strings.TrimSpace(f.Message) != "" {
		parts = append(parts, f.Message)
	}
	return strings.Join(parts, " • ")
}

// EKSUpgradeReadiness summarizes control plane, node group, and add-on alignment.
type EKSUpgradeReadiness struct {
	ClusterName    string
	ClusterVersion string
	NodeGroups     []EKSNodeGroup
	Addons         []EKSAddon
	Insights       []EKSUpgradeInsight
	Findings       []EKSUpgradeFinding
}

func (r EKSUpgradeReadiness) Summary() string {
	blockers := 0
	warnings := 0
	for _, finding := range r.Findings {
		switch strings.ToLower(finding.Severity) {
		case "blocker":
			blockers++
		case "warning":
			warnings++
		}
	}
	if blockers > 0 {
		return fmt.Sprintf("%d blocker(s), %d warning(s)", blockers, warnings)
	}
	if warnings > 0 {
		return fmt.Sprintf("%d warning(s)", warnings)
	}
	return "ready"
}

func (r EKSUpgradeReadiness) HasBlockers() bool {
	for _, finding := range r.Findings {
		if strings.EqualFold(finding.Severity, "blocker") {
			return true
		}
	}
	return false
}

func BuildEKSUpgradeReadiness(cluster EKSCluster, nodeGroups []EKSNodeGroup, addons []EKSAddon, insights []EKSUpgradeInsight, addonCompatibility map[string]bool) EKSUpgradeReadiness {
	result := EKSUpgradeReadiness{
		ClusterName:    cluster.Name,
		ClusterVersion: cluster.Version,
		NodeGroups:     append([]EKSNodeGroup(nil), nodeGroups...),
		Addons:         append([]EKSAddon(nil), addons...),
		Insights:       append([]EKSUpgradeInsight(nil), insights...),
	}
	for _, nodeGroup := range nodeGroups {
		if strings.TrimSpace(nodeGroup.Version) == "" {
			result.Findings = append(result.Findings, EKSUpgradeFinding{
				Severity: "warning",
				Subject:  "node group " + nodeGroup.Name,
				Expected: cluster.Version,
				Message:  "node group version unavailable",
			})
			continue
		}
		if nodeGroup.Version != cluster.Version {
			result.Findings = append(result.Findings, EKSUpgradeFinding{
				Severity: "blocker",
				Subject:  "node group " + nodeGroup.Name,
				Current:  nodeGroup.Version,
				Expected: cluster.Version,
				Message:  "node group version differs from control plane",
			})
		}
	}
	for _, addon := range addons {
		if !strings.EqualFold(strings.TrimSpace(addon.Status), "ACTIVE") {
			severity := "warning"
			if isBlockingAddonStatus(addon.Status) {
				severity = "blocker"
			}
			result.Findings = append(result.Findings, EKSUpgradeFinding{
				Severity: severity,
				Subject:  "add-on " + addon.Name,
				Current:  addon.Status,
				Expected: "ACTIVE",
				Message:  "add-on is not active",
			})
		}
		if compatible, ok := addonCompatibility[addon.Name]; ok && !compatible {
			result.Findings = append(result.Findings, EKSUpgradeFinding{
				Severity: "blocker",
				Subject:  "add-on " + addon.Name,
				Current:  addon.Version,
				Expected: "compatible with Kubernetes " + cluster.Version,
				Message:  "installed add-on version is not listed as compatible",
			})
		}
	}
	for _, insight := range insights {
		if strings.EqualFold(insight.Status, "PASSING") {
			continue
		}
		severity := "warning"
		if strings.EqualFold(insight.Status, "ERROR") {
			severity = "blocker"
		}
		result.Findings = append(result.Findings, EKSUpgradeFinding{
			Severity: severity,
			Subject:  "insight " + firstNonEmptyString(insight.Name, insight.ID),
			Current:  firstNonEmptyString(insight.Status, "-"),
			Expected: "PASSING",
			Message:  firstNonEmptyString(insight.Reason, "EKS upgrade insight is not passing"),
		})
	}
	return result
}

func isBlockingAddonStatus(status string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(status))
	return normalized == "DEGRADED" || strings.HasSuffix(normalized, "_FAILED")
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
