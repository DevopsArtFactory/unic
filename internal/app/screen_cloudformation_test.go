package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

func TestCloudFormationStackListRendersFiltersAndOpensDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading
	repo := &awsservice.AwsRepository{CloudFormationClient: &cloudFormationAppTestClient{
		describeStacks: func(_ context.Context, input *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			if awssdk.ToString(input.StackName) != "failed-id" {
				t.Fatalf("expected failed stack detail lookup, got %q", awssdk.ToString(input.StackName))
			}
			return &cloudformation.DescribeStacksOutput{Stacks: []cloudformationtypes.Stack{{
				StackId: awssdk.String("failed-id"), StackName: awssdk.String("failed-prod"),
				StackStatus: cloudformationtypes.StackStatusCreateFailed,
			}}}, nil
		},
	}}
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
	result := runBatchedUserCmd(t, cmd)
	bound, ok := result.(genBoundMsg)
	if !ok {
		t.Fatalf("expected generation-bound detail result, got %#v", result)
	}
	loaded, ok := bound.msg.(cloudFormationStackDetailLoadedMsg)
	if !ok || loaded.stack == nil || loaded.stack.ID != "failed-id" {
		t.Fatalf("expected failed stack detail message, got %#v", result)
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

func TestCloudFormationDetailRefreshReconcilesStackList(t *testing.T) {
	tests := []struct {
		name         string
		filter       string
		wantFiltered int
	}{
		{name: "preserves selected stack after reordering", wantFiltered: 2},
		{name: "reapplies active status filter", filter: "create_complete", wantFiltered: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(testConfig(), "", "dev")
			m.screen = screenLoading
			m.storeFilterValue(filterCloudFormationStacks, tt.filter)
			m.cloudFormation.stacks = []awsservice.CloudFormationStack{
				{ID: "other-id", Name: "other", Status: "UPDATE_IN_PROGRESS"},
				{ID: "stack-id", Name: "prod", Status: "CREATE_COMPLETE"},
			}
			m.cloudFormation.filtered = applyFilter(m.cloudFormation.stacks, tt.filter)
			m.cloudFormation.idx = len(m.cloudFormation.filtered) - 1
			m.cloudFormation.selected = &awsservice.CloudFormationStack{ID: "stack-id", Name: "prod", Status: "CREATE_COMPLETE"}

			m.cloudFormation.HandleMessage(&m, cloudFormationStackDetailLoadedMsg{stack: &awsservice.CloudFormationStack{
				ID: "stack-id", Name: "prod", Status: "CREATE_FAILED",
			}})

			if m.cloudFormation.stacks[0].ID != "stack-id" || m.cloudFormation.stacks[0].Status != "CREATE_FAILED" {
				t.Fatalf("expected refreshed failed stack first, got %+v", m.cloudFormation.stacks)
			}
			if len(m.cloudFormation.filtered) != tt.wantFiltered {
				t.Fatalf("expected %d filtered stacks, got %+v", tt.wantFiltered, m.cloudFormation.filtered)
			}
			if tt.filter == "" && (m.cloudFormation.idx != 0 || m.cloudFormation.filtered[m.cloudFormation.idx].ID != "stack-id") {
				t.Fatalf("expected cursor to follow refreshed stack, got idx=%d filtered=%+v", m.cloudFormation.idx, m.cloudFormation.filtered)
			}
		})
	}
}

func TestCloudFormationDetailRefreshWaitsForActiveDriftDetection(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenCloudFormationStackDetail
	m.cloudFormation.selected = &awsservice.CloudFormationStack{ID: "stack-id", Name: "prod"}
	m.cloudFormation.driftDetectionID = "detect-1"
	generation := m.commands.CurrentGen()

	updated, cmd, handled := m.cloudFormation.HandleKey(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model := updated.(Model)
	if !handled || cmd != nil || model.screen != screenCloudFormationStackDetail || model.cloudFormation.driftDetectionID != "detect-1" {
		t.Fatalf("expected refresh to leave active drift polling intact, got handled=%v cmd=%v screen=%v id=%q", handled, cmd, model.screen, model.cloudFormation.driftDetectionID)
	}
	if model.commands.CurrentGen() != generation {
		t.Fatalf("expected refresh to preserve command generation %d, got %d", generation, model.commands.CurrentGen())
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
	m.cloudFormation.stacks = []awsservice.CloudFormationStack{{ID: "stack-id", DriftStatus: "NOT_CHECKED"}}
	m.cloudFormation.filtered = []awsservice.CloudFormationStack{{ID: "stack-id", DriftStatus: "NOT_CHECKED"}}

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
	if m.cloudFormation.stacks[0].DriftStatus != "DRIFTED" || m.cloudFormation.filtered[0].DriftStatus != "DRIFTED" ||
		!m.cloudFormation.stacks[0].LastDriftCheck.Equal(checkedAt) || !m.cloudFormation.filtered[0].LastDriftCheck.Equal(checkedAt) {
		t.Fatalf("expected cached drift state to be updated, got stacks=%+v filtered=%+v", m.cloudFormation.stacks, m.cloudFormation.filtered)
	}
	if !strings.Contains(m.cloudFormation.driftNotice, "2 drifted resources") {
		t.Fatalf("expected completion notice, got %q", m.cloudFormation.driftNotice)
	}
}

func TestCloudFormationDriftPollingStopsOnFailuresAndTimeout(t *testing.T) {
	tests := []struct {
		name     string
		msg      cloudFormationDriftStatusMsg
		attempts int
		want     string
	}{
		{name: "API error", msg: cloudFormationDriftStatusMsg{err: errors.New("access denied")}, want: "access denied"},
		{name: "terminal failure", msg: cloudFormationDriftStatusMsg{status: &awsservice.CloudFormationDriftDetection{DetectionStatus: "DETECTION_FAILED", Reason: "unsupported resource"}}, want: "unsupported resource"},
		{name: "timeout", msg: cloudFormationDriftStatusMsg{status: &awsservice.CloudFormationDriftDetection{DetectionStatus: "DETECTION_IN_PROGRESS"}}, attempts: cloudFormationDriftPollLimit - 1, want: "timed out"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(testConfig(), "", "dev")
			m.screen = screenCloudFormationStackDetail
			m.cloudFormation.selected = &awsservice.CloudFormationStack{ID: "stack-id"}
			m.cloudFormation.driftDetectionID = "detect-1"
			m.cloudFormation.driftPollAttempts = tt.attempts
			tt.msg.stackID = "stack-id"
			tt.msg.detectionID = "detect-1"

			_, cmd, handled := m.cloudFormation.HandleMessage(&m, tt.msg)
			if !handled || cmd != nil || m.cloudFormation.driftDetectionID != "" || !strings.Contains(m.cloudFormation.driftNotice, tt.want) {
				t.Fatalf("expected polling to stop with %q, got handled=%v cmd=%v id=%q notice=%q", tt.want, handled, cmd, m.cloudFormation.driftDetectionID, m.cloudFormation.driftNotice)
			}
		})
	}
}

func TestCloudFormationDriftPollingSurvivesSettingsOverlay(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenCloudFormationStackDetail
	m.cloudFormation.selected = &awsservice.CloudFormationStack{ID: "stack-id"}
	m.cloudFormation.driftDetectionID = "detect-1"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	model := updated.(Model)
	if model.screen != screenSettings || model.settingsPrevScreen != screenCloudFormationStackDetail {
		t.Fatalf("expected Settings over stack detail, got screen=%v previous=%v", model.screen, model.settingsPrevScreen)
	}
	updated, cmd := model.Update(cloudFormationDriftPollTickMsg{stackID: "stack-id", detectionID: "detect-1"})
	model = updated.(Model)
	if cmd == nil || model.cloudFormation.driftDetectionID != "detect-1" {
		t.Fatalf("expected active poll to continue behind Settings, got cmd=%v id=%q", cmd, model.cloudFormation.driftDetectionID)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.screen != screenCloudFormationStackDetail || model.cloudFormation.driftDetectionID != "detect-1" {
		t.Fatalf("expected return to live stack detail, got screen=%v id=%q", model.screen, model.cloudFormation.driftDetectionID)
	}
}

func TestCloudFormationLoadsCompleteBehindGlobalOverlays(t *testing.T) {
	loads := []struct {
		name    string
		setup   func(*Model)
		msg     tea.Msg
		next    screen
		wantCmd bool
		verify  func(*testing.T, Model)
	}{
		{
			name: "stack list",
			msg:  cloudFormationStacksLoadedMsg{stacks: []awsservice.CloudFormationStack{{ID: "stack-id"}}},
			next: screenCloudFormationStackList,
			verify: func(t *testing.T, m Model) {
				if len(m.cloudFormation.stacks) != 1 {
					t.Fatalf("expected loaded stack list, got %+v", m.cloudFormation.stacks)
				}
			},
		},
		{
			name: "stack detail",
			setup: func(m *Model) {
				m.cloudFormation.selected = &awsservice.CloudFormationStack{ID: "stack-id"}
			},
			msg:  cloudFormationStackDetailLoadedMsg{stack: &awsservice.CloudFormationStack{ID: "stack-id", Name: "prod"}},
			next: screenCloudFormationStackDetail,
			verify: func(t *testing.T, m Model) {
				if m.cloudFormation.selected == nil || m.cloudFormation.selected.Name != "prod" {
					t.Fatalf("expected loaded stack detail, got %+v", m.cloudFormation.selected)
				}
			},
		},
		{
			name: "drift start",
			setup: func(m *Model) {
				m.cloudFormation.selected = &awsservice.CloudFormationStack{ID: "stack-id"}
			},
			msg:     cloudFormationDriftStartedMsg{stackID: "stack-id", detectionID: "detect-1"},
			next:    screenCloudFormationStackDetail,
			wantCmd: true,
			verify: func(t *testing.T, m Model) {
				if m.cloudFormation.driftDetectionID != "detect-1" {
					t.Fatalf("expected active drift detection, got %q", m.cloudFormation.driftDetectionID)
				}
			},
		},
	}
	overlays := []struct {
		name     string
		open     func(*Model)
		previous func(Model) screen
	}{
		{
			name: "settings",
			open: func(m *Model) {
				m.screen = screenSettings
				m.settingsPrevScreen = screenLoading
			},
			previous: func(m Model) screen { return m.settingsPrevScreen },
		},
		{
			name: "command palette",
			open: func(m *Model) {
				m.screen = screenCommandPalette
				m.palette.prevScreen = screenLoading
			},
			previous: func(m Model) screen { return m.palette.prevScreen },
		},
		{
			name: "saved views",
			open: func(m *Model) {
				m.screen = screenViewList
				m.views.prevScreen = screenLoading
			},
			previous: func(m Model) screen { return m.views.prevScreen },
		},
		{
			name: "context picker",
			open: func(m *Model) {
				m.cfg.ContextName = "dev"
				m.screen = screenContextPicker
				m.ctxPrevScreen = screenLoading
			},
			previous: func(m Model) screen { return m.ctxPrevScreen },
		},
	}

	for _, load := range loads {
		for _, overlay := range overlays {
			t.Run(load.name+"/"+overlay.name, func(t *testing.T) {
				m := New(testConfig(), "", "dev")
				overlay.open(&m)
				if load.setup != nil {
					load.setup(&m)
				}

				updated, cmd := m.Update(load.msg)
				model := updated.(Model)
				if model.screen != m.screen || overlay.previous(model) != load.next || (cmd != nil) != load.wantCmd {
					t.Fatalf("expected %s to retain completed load target %v, got screen=%v previous=%v cmd=%v", overlay.name, load.next, model.screen, overlay.previous(model), cmd)
				}
				load.verify(t, model)

				updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
				if model = updated.(Model); model.screen != load.next {
					t.Fatalf("expected %s return to %v, got %v", overlay.name, load.next, model.screen)
				}
			})
		}
	}
}

func TestCloudFormationLoadActiveStopsOnOverlayCycle(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenSettings
	m.settingsPrevScreen = screenContextPicker
	m.ctxPrevScreen = screenSettings

	if cloudFormationLoadActive(m) {
		t.Fatal("expected cyclic overlay chain not to report an active load")
	}
}

func TestCloudFormationLoadCompletesBeforeContextPickerOpens(t *testing.T) {
	cfg := testConfig()
	cfg.ContextName = "dev"
	m := New(cfg, "", "dev")
	m.screen = screenLoading

	updated, contextsCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	model := updated.(Model)
	if contextsCmd == nil || model.screen != screenLoading || model.ctxPrevScreen != screenLoading {
		t.Fatalf("expected context picker load over CloudFormation load, got screen=%v previous=%v cmd=%v", model.screen, model.ctxPrevScreen, contextsCmd)
	}

	updated, _ = model.Update(cloudFormationStacksLoadedMsg{stacks: []awsservice.CloudFormationStack{{ID: "stack-id"}}})
	model = updated.(Model)
	if model.screen != screenCloudFormationStackList || model.ctxPrevScreen != screenCloudFormationStackList {
		t.Fatalf("expected completed load to rewrite pending context return, got screen=%v previous=%v", model.screen, model.ctxPrevScreen)
	}

	updated, _ = model.Update(contextsLoadedMsg{})
	model = updated.(Model)
	if model.screen != screenContextPicker || model.ctxPrevScreen != screenCloudFormationStackList {
		t.Fatalf("expected context picker over completed stack list, got screen=%v previous=%v", model.screen, model.ctxPrevScreen)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model = updated.(Model); model.screen != screenCloudFormationStackList {
		t.Fatalf("expected context picker return to completed stack list, got %v", model.screen)
	}
}

func TestCloudFormationDriftPollingStopsAfterHome(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading
	m.cloudFormation.selected = &awsservice.CloudFormationStack{ID: "stack-id"}

	_, tickCmd, _ := m.cloudFormation.HandleMessage(&m, cloudFormationDriftStartedMsg{stackID: "stack-id", detectionID: "detect-1"})
	_, pollCmd, _ := m.cloudFormation.HandleMessage(&m, cloudFormationDriftPollTickMsg{stackID: "stack-id", detectionID: "detect-1"})
	if tickCmd == nil || pollCmd == nil {
		t.Fatal("expected scheduled tick and poll commands")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	model := updated.(Model)
	if model.screen != screenServiceList {
		t.Fatalf("expected Home navigation, got %v", model.screen)
	}
	for name, cmd := range map[string]tea.Cmd{"tick": tickCmd, "poll": pollCmd} {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected abandoned %s command to be dropped, got %#v", name, msg)
		}
	}
}

func TestCloudFormationDriftStatusReappliesFilter(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenCloudFormationStackDetail
	m.storeFilterValue(filterCloudFormationStacks, "not_checked")
	m.cloudFormation.selected = &awsservice.CloudFormationStack{ID: "stack-id", DriftStatus: "NOT_CHECKED"}
	m.cloudFormation.stacks = []awsservice.CloudFormationStack{{ID: "stack-id", DriftStatus: "NOT_CHECKED"}}
	m.cloudFormation.filtered = applyFilter(m.cloudFormation.stacks, m.filterValue(filterCloudFormationStacks))
	m.cloudFormation.driftDetectionID = "detect-1"

	m.cloudFormation.HandleMessage(&m, cloudFormationDriftStatusMsg{
		stackID: "stack-id", detectionID: "detect-1", status: &awsservice.CloudFormationDriftDetection{
			DetectionStatus: "DETECTION_COMPLETE", StackDriftStatus: "IN_SYNC",
		},
	})

	if len(m.cloudFormation.filtered) != 0 {
		t.Fatalf("expected completed stack to leave NOT_CHECKED filter, got %+v", m.cloudFormation.filtered)
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

type cloudFormationAppTestClient struct {
	describeStacks func(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error)
}

func (c *cloudFormationAppTestClient) DescribeStacks(ctx context.Context, input *cloudformation.DescribeStacksInput, opts ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	return c.describeStacks(ctx, input, opts...)
}

func (*cloudFormationAppTestClient) DescribeStackEvents(context.Context, *cloudformation.DescribeStackEventsInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error) {
	return &cloudformation.DescribeStackEventsOutput{}, nil
}

func (*cloudFormationAppTestClient) DetectStackDrift(context.Context, *cloudformation.DetectStackDriftInput, ...func(*cloudformation.Options)) (*cloudformation.DetectStackDriftOutput, error) {
	return &cloudformation.DetectStackDriftOutput{}, nil
}

func (*cloudFormationAppTestClient) DescribeStackDriftDetectionStatus(context.Context, *cloudformation.DescribeStackDriftDetectionStatusInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStackDriftDetectionStatusOutput, error) {
	return &cloudformation.DescribeStackDriftDetectionStatusOutput{}, nil
}
