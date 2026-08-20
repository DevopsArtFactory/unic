package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

func TestCloudFormationStackListRendersFiltersAndOpensDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading
	repo := &awsservice.AwsRepository{}
	stacks := []awsservice.CloudFormationStack{
		{Name: "failed-prod", ID: "failed-id", Status: "CREATE_FAILED", DriftStatus: "DRIFTED"},
		{Name: "healthy-dev", ID: "healthy-id", Status: "CREATE_COMPLETE", DriftStatus: "IN_SYNC"},
	}

	_, _, handled := m.cloudFormation.HandleMessage(&m, cloudFormationStacksLoadedMsg{stacks: stacks, repo: repo})
	if !handled || m.screen != screenCloudFormationStackList || m.awsRepo != repo {
		t.Fatalf("expected stack list and retained repository, got handled=%v screen=%v repo=%p", handled, m.screen, m.awsRepo)
	}
	view := stripANSI(m.cloudFormation.viewStackList(m))
	for _, want := range []string{"failed and rollback states first", "failed-prod", "CREATE_FAILED", "healthy-dev", "IN_SYNC"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected list to contain %q, got:\n%s", want, view)
		}
	}

	m.storeFilterValue(filterCloudFormationStacks, "drifted")
	m.applyFilterTarget(filterCloudFormationStacks)
	if len(m.cloudFormation.filtered) != 1 || m.cloudFormation.filtered[0].Name != "failed-prod" {
		t.Fatalf("expected drift filter result, got %+v", m.cloudFormation.filtered)
	}

	updated, cmd, handled := m.cloudFormation.HandleKey(&m, tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if !handled || cmd == nil || model.screen != screenLoading || model.cloudFormation.selected == nil || model.cloudFormation.selected.ID != "failed-id" {
		t.Fatalf("expected selected stack detail load, got handled=%v screen=%v selected=%+v", handled, model.screen, model.cloudFormation.selected)
	}
}

func TestCloudFormationDetailShowsParametersOutputsEventsAndEscapesControls(t *testing.T) {
	now := time.Date(2026, 8, 20, 4, 0, 0, 0, time.UTC)
	m := New(testConfig(), "", "dev")
	m.height = 80
	m.screen = screenLoading
	stack := &awsservice.CloudFormationStack{
		Name: "prod", ID: "stack-id", Status: "ROLLBACK_FAILED", StatusReason: "failed\x1b[31m",
		DriftStatus: "DRIFTED", LastDriftCheck: now, CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
		TerminationProtection: true,
		Parameters:            []awsservice.CloudFormationValue{{Key: "Environment", Value: "prod"}},
		Outputs:               []awsservice.CloudFormationValue{{Key: "Endpoint", Value: "example.com", ExportName: "prod-endpoint", Description: "service endpoint"}},
		Events:                []awsservice.CloudFormationStackEvent{{Timestamp: now, LogicalResourceID: "Bucket", ResourceType: "AWS::S3::Bucket", Status: "CREATE_FAILED", Reason: "name already exists\nretry"}},
	}

	m.cloudFormation.selected = &awsservice.CloudFormationStack{ID: "stack-id"}
	m.cloudFormation.HandleMessage(&m, cloudFormationStackDetailLoadedMsg{stack: stack})
	view := stripANSI(m.cloudFormation.viewStackDetail(m))
	for _, want := range []string{"ROLLBACK_FAILED", "Environment = prod", "Endpoint = example.com", "prod-endpoint", "Recent Events", "CREATE_FAILED", "name already exists\\nretry"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected detail to contain %q, got:\n%s", want, view)
		}
	}
	if strings.Contains(view, "\x1b[31m") {
		t.Fatalf("expected terminal control to be escaped, got %q", view)
	}
	if !strings.Contains(view, `\x1b`) {
		t.Fatalf("expected visible escape marker, got %q", view)
	}
}

func TestCloudFormationDetailScrollsToOlderEvents(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 10
	m.screen = screenCloudFormationStackDetail
	m.cloudFormation.selected = &awsservice.CloudFormationStack{Name: "prod", ID: "stack-id", DriftStatus: "NOT_CHECKED"}
	for i := 0; i < 12; i++ {
		m.cloudFormation.selected.Events = append(m.cloudFormation.selected.Events, awsservice.CloudFormationStackEvent{
			LogicalResourceID: "Resource" + string(rune('A'+i)), Status: "CREATE_COMPLETE",
		})
	}

	initial := stripANSI(m.cloudFormation.viewStackDetail(m))
	if strings.Contains(initial, "ResourceL") {
		t.Fatalf("expected later events to be windowed, got:\n%s", initial)
	}
	for range 6 {
		_, _, handled := m.cloudFormation.HandleKey(&m, tea.KeyMsg{Type: tea.KeyPgDown})
		if !handled {
			t.Fatal("expected page-down to be handled")
		}
	}
	scrolled := stripANSI(m.cloudFormation.viewStackDetail(m))
	if !strings.Contains(scrolled, "ResourceL") {
		t.Fatalf("expected page-down to reveal later events, got:\n%s", scrolled)
	}
}

func TestCloudFormationDriftPollingUpdatesDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading
	m.cloudFormation.selected = &awsservice.CloudFormationStack{Name: "prod", ID: "stack-id", DriftStatus: "NOT_CHECKED"}

	_, cmd, handled := m.cloudFormation.HandleMessage(&m, cloudFormationDriftStartedMsg{stackID: "stack-id", detectionID: "detect-1"})
	if !handled || cmd == nil || m.cloudFormation.driftDetectionID != "detect-1" || !strings.Contains(m.cloudFormation.driftNotice, "in progress") {
		t.Fatalf("expected active drift polling, got handled=%v id=%q notice=%q", handled, m.cloudFormation.driftDetectionID, m.cloudFormation.driftNotice)
	}
	_, cmd, _ = m.cloudFormation.HandleMessage(&m, cloudFormationDriftStatusMsg{
		stackID: "stack-id", detectionID: "detect-1", status: &awsservice.CloudFormationDriftDetection{DetectionStatus: "DETECTION_IN_PROGRESS"},
	})
	if cmd == nil {
		t.Fatal("expected in-progress detection to schedule another poll")
	}

	checkedAt := time.Date(2026, 8, 20, 5, 0, 0, 0, time.UTC)
	_, cmd, _ = m.cloudFormation.HandleMessage(&m, cloudFormationDriftStatusMsg{
		stackID: "stack-id", detectionID: "detect-1", status: &awsservice.CloudFormationDriftDetection{
			DetectionStatus: "DETECTION_COMPLETE", StackDriftStatus: "DRIFTED", DriftedResources: 2, Timestamp: checkedAt,
		},
	})
	if cmd != nil || m.cloudFormation.driftDetectionID != "" || m.cloudFormation.selected.DriftStatus != "DRIFTED" || !m.cloudFormation.selected.LastDriftCheck.Equal(checkedAt) {
		t.Fatalf("expected completed drift state, got id=%q stack=%+v", m.cloudFormation.driftDetectionID, m.cloudFormation.selected)
	}
	if !strings.Contains(m.cloudFormation.driftNotice, "2 drifted resources") {
		t.Fatalf("expected completion notice, got %q", m.cloudFormation.driftNotice)
	}
}

func TestCloudFormationSavedViewAndHelpRegistration(t *testing.T) {
	if target, ok := featurePrimaryFilter[domain.FeatureCloudFormationBrowser]; !ok || target != filterCloudFormationStacks {
		t.Fatalf("expected CloudFormation saved-view filter, got %v %v", target, ok)
	}
	m := New(testConfig(), "", "dev")
	for screen, want := range map[screen]string{
		screenCloudFormationStackList:   "CloudFormation Stacks",
		screenCloudFormationStackDetail: "CloudFormation Stack Detail",
	} {
		m.screen = screen
		if got := m.helpScreenTitle(); got != want {
			t.Fatalf("screen %v: expected %q, got %q", screen, want, got)
		}
	}
}

func TestCloudFormationIgnoresLoadResultsAfterNavigation(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenServiceList
	m.cloudFormation.selected = &awsservice.CloudFormationStack{ID: "stack-id"}

	m.cloudFormation.HandleMessage(&m, cloudFormationStacksLoadedMsg{stacks: []awsservice.CloudFormationStack{{ID: "other"}}})
	if m.screen != screenServiceList || len(m.cloudFormation.stacks) != 0 {
		t.Fatalf("expected stale list load to be ignored, got screen=%v stacks=%+v", m.screen, m.cloudFormation.stacks)
	}
	m.cloudFormation.HandleMessage(&m, cloudFormationStackDetailLoadedMsg{stack: &awsservice.CloudFormationStack{ID: "stack-id"}})
	if m.screen != screenServiceList {
		t.Fatalf("expected stale detail load to be ignored, got %v", m.screen)
	}
}
