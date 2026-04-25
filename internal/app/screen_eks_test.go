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
