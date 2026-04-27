package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

func TestFeatureListEKSBrowserStartsLoading(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenFeatureList
	m.features = []domain.Feature{{Kind: domain.FeatureEKSBrowser, Description: "Browse EKS clusters"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if cmd == nil {
		t.Fatal("expected load command")
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %v", model.screen)
	}
}

func TestHandleEKSClustersLoadedMsgShowsClusterList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	clusters := []awsservice.EKSCluster{{Name: "prod-eks", Version: "1.32", Status: "ACTIVE", EndpointPublicAccess: true}}

	updated, _, handled := m.handleEKSMsg(eksClustersLoadedMsg{clusters: clusters})
	if !handled {
		t.Fatal("expected message to be handled")
	}
	model := updated.(Model)
	if model.screen != screenEKSClusterList {
		t.Fatalf("expected EKS cluster list screen, got %v", model.screen)
	}
	if len(model.filteredEKSClusters) != 1 || model.filteredEKSClusters[0].Name != "prod-eks" {
		t.Fatalf("unexpected clusters: %+v", model.filteredEKSClusters)
	}
}

func TestEKSClusterListEnterLoadsNodeGroups(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenEKSClusterList
	m.eksClusters = []awsservice.EKSCluster{{Name: "prod-eks", ARN: "arn:aws:eks:ap-northeast-2:123456789012:cluster/prod-eks", Version: "1.32", Status: "ACTIVE"}}
	m.filteredEKSClusters = m.eksClusters

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if cmd == nil {
		t.Fatal("expected load command")
	}
	if model.selectedEKSCluster == nil || model.selectedEKSCluster.Name != "prod-eks" {
		t.Fatalf("unexpected selected cluster: %+v", model.selectedEKSCluster)
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %v", model.screen)
	}
}

func TestEKSClusterListALoadsAddons(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenEKSClusterList
	m.eksClusters = []awsservice.EKSCluster{{Name: "prod-eks", ARN: "arn:aws:eks:ap-northeast-2:123456789012:cluster/prod-eks", Version: "1.32", Status: "ACTIVE"}}
	m.filteredEKSClusters = m.eksClusters

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model := updated.(Model)
	if cmd == nil {
		t.Fatal("expected load command")
	}
	if model.selectedEKSCluster == nil || model.selectedEKSCluster.Name != "prod-eks" {
		t.Fatalf("unexpected selected cluster: %+v", model.selectedEKSCluster)
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %v", model.screen)
	}
}

func TestEKSClusterListULoadsUpgradeReadiness(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenEKSClusterList
	m.eksClusters = []awsservice.EKSCluster{{Name: "prod-eks", ARN: "arn:aws:eks:ap-northeast-2:123456789012:cluster/prod-eks", Version: "1.32", Status: "ACTIVE"}}
	m.filteredEKSClusters = m.eksClusters

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'U'}})
	model := updated.(Model)
	if cmd == nil {
		t.Fatal("expected load command")
	}
	if model.selectedEKSCluster == nil || model.selectedEKSCluster.Name != "prod-eks" {
		t.Fatalf("unexpected selected cluster: %+v", model.selectedEKSCluster)
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %v", model.screen)
	}
}

func TestHandleEKSAddonsLoadedMsgShowsAddonList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	addons := []awsservice.EKSAddon{{ClusterName: "prod-eks", Name: "coredns", Version: "v1.11.4-eksbuild.2", Status: "ACTIVE"}}

	updated, _, handled := m.handleEKSMsg(eksAddonsLoadedMsg{addons: addons})
	if !handled {
		t.Fatal("expected message to be handled")
	}
	model := updated.(Model)
	if model.screen != screenEKSAddonList {
		t.Fatalf("expected EKS add-on list screen, got %v", model.screen)
	}
	if len(model.filteredEKSAddons) != 1 || model.filteredEKSAddons[0].Name != "coredns" {
		t.Fatalf("unexpected add-ons: %+v", model.filteredEKSAddons)
	}
}

func TestHandleEKSUpgradeReadinessLoadedMsgShowsReadiness(t *testing.T) {
	m := New(testConfig(), "", "dev")
	readiness := &awsservice.EKSUpgradeReadiness{ClusterName: "prod-eks", ClusterVersion: "1.32"}

	updated, _, handled := m.handleEKSMsg(eksUpgradeReadinessLoadedMsg{readiness: readiness})
	if !handled {
		t.Fatal("expected message to be handled")
	}
	model := updated.(Model)
	if model.screen != screenEKSUpgradeReadiness {
		t.Fatalf("expected readiness screen, got %v", model.screen)
	}
	if model.eksUpgradeReadiness == nil || model.eksUpgradeReadiness.ClusterName != "prod-eks" {
		t.Fatalf("unexpected readiness: %+v", model.eksUpgradeReadiness)
	}
}

func TestEKSUpgradeReadinessViewShowsFindings(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 120
	m.height = 32
	m.screen = screenEKSUpgradeReadiness
	m.eksUpgradeReadiness = &awsservice.EKSUpgradeReadiness{
		ClusterName:    "prod-eks",
		ClusterVersion: "1.32",
		NodeGroups: []awsservice.EKSNodeGroup{
			{ClusterName: "prod-eks", Name: "blue", Version: "1.32", Status: "ACTIVE"},
			{ClusterName: "prod-eks", Name: "green", Version: "1.31", Status: "ACTIVE"},
		},
		Addons: []awsservice.EKSAddon{
			{ClusterName: "prod-eks", Name: "coredns", Version: "v1.11.4-eksbuild.2", Status: "ACTIVE"},
		},
		Insights: []awsservice.EKSUpgradeInsight{
			{ID: "insight-1", Name: "Deprecated API usage", Status: "ERROR", KubernetesVersion: "1.33"},
		},
		Findings: []awsservice.EKSUpgradeFinding{{
			Severity: "blocker",
			Subject:  "node group green",
			Current:  "1.31",
			Expected: "1.32",
			Message:  "node group version differs from control plane",
		}},
	}

	view := stripANSI(m.viewEKSUpgradeReadiness())
	for _, want := range []string{
		"EKS Upgrade Readiness — prod-eks",
		"current version alignment",
		"1 blocker(s), 0 warning(s)",
		"node group green",
		"Deprecated API usage",
		"1.31 -> 1.32",
		"coredns",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}

func TestEKSAddonListHighlightsDegradedAddons(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 120
	m.height = 32
	m.screen = screenEKSAddonList
	m.selectedEKSCluster = &awsservice.EKSCluster{Name: "prod-eks"}
	m.eksAddons = []awsservice.EKSAddon{{
		ClusterName: "prod-eks",
		Name:        "vpc-cni",
		Version:     "v1.16.4-eksbuild.2",
		Status:      "DEGRADED",
		HealthIssues: []awsservice.EKSAddonIssue{{
			Code:    "AddonPermissionFailure",
			Message: "missing IAM permission",
		}},
	}}
	m.filteredEKSAddons = m.eksAddons

	view := stripANSI(m.viewEKSAddonList())
	for _, want := range []string{"EKS Add-ons — prod-eks", "vpc-cni", "v1.16.4-eksbuild.2", "DEGRADED", "1 issue(s)"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}

func TestEKSAddonDetailViewShowsHealthIssues(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 120
	m.height = 32
	m.screen = screenEKSAddonDetail
	m.selectedEKSAddon = &awsservice.EKSAddon{
		ClusterName:           "prod-eks",
		Name:                  "vpc-cni",
		ARN:                   "arn:aws:eks:ap-northeast-2:123456789012:addon/prod-eks/vpc-cni/uuid",
		Version:               "v1.16.4-eksbuild.2",
		Status:                "DEGRADED",
		Owner:                 "aws",
		Publisher:             "eks",
		ServiceAccountRoleARN: "arn:aws:iam::123456789012:role/vpc-cni",
		HealthIssues: []awsservice.EKSAddonIssue{{
			Code:        "AddonPermissionFailure",
			Message:     "missing IAM permission",
			ResourceIDs: []string{"aws-node"},
		}},
	}

	view := stripANSI(m.viewEKSAddonDetail())
	for _, want := range []string{
		"EKS Add-on Detail — vpc-cni",
		"v1.16.4-eksbuild.2",
		"DEGRADED",
		"AddonPermissionFailure",
		"missing IAM permission",
		"aws-node",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}

func TestEKSNodeGroupDetailViewShowsScalingAndHealth(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 120
	m.height = 32
	m.screen = screenEKSNodeGroupDetail
	m.selectedEKSNodeGroup = &awsservice.EKSNodeGroup{
		ClusterName:    "prod-eks",
		Name:           "blue",
		ARN:            "arn:aws:eks:ap-northeast-2:123456789012:nodegroup/prod-eks/blue/uuid",
		Status:         "ACTIVE",
		Version:        "1.32",
		ReleaseVersion: "1.32.3-20260401",
		AmiType:        "AL2_x86_64",
		CapacityType:   "ON_DEMAND",
		InstanceTypes:  []string{"m6i.large", "m6a.large"},
		DesiredSize:    3,
		MinSize:        2,
		MaxSize:        5,
		HealthIssues: []awsservice.EKSHealthIssue{{
			Code:        "ClusterUnreachable",
			Message:     "control plane unreachable",
			ResourceIDs: []string{"subnet-123"},
		}},
	}

	view := stripANSI(m.viewEKSNodeGroupDetail())
	for _, want := range []string{
		"EKS Node Group Detail — blue",
		"desired:3 min:2 max:5",
		"m6i.large, m6a.large",
		"ClusterUnreachable",
		"control plane unreachable",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}
