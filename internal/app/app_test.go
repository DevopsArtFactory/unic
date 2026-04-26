package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	"unic/internal/domain"
	"unic/internal/inspector"
	awsservice "unic/internal/services/aws"
)

func testConfig() *config.Config {
	return &config.Config{Profile: "default", Region: "us-east-1"}
}

func writeChecklistFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write checklist file %s: %v", path, err)
	}
	return path
}

func TestNewModelNotQuitting(t *testing.T) {
	m := New(testConfig(), "", "dev")
	if m.quitting {
		t.Error("new model should not be quitting")
	}
}

func TestNewModelStartsOnContextPicker(t *testing.T) {
	m := New(testConfig(), "", "dev")
	if m.screen != screenContextPicker {
		t.Error("new model should start on context picker screen")
	}
}

func TestNewModelInitializesLoadingSpinner(t *testing.T) {
	m := New(testConfig(), "", "dev")

	if got, want := m.loadingSpinner.Spinner.FPS, spinner.MiniDot.FPS; got != want {
		t.Fatalf("expected spinner FPS %v, got %v", want, got)
	}
	if got, want := len(m.loadingSpinner.Spinner.Frames), len(spinner.MiniDot.Frames); got != want {
		t.Fatalf("expected %d spinner frames, got %d", want, got)
	}
	if got, want := m.loadingSpinner.Spinner.Frames[0], spinner.MiniDot.Frames[0]; got != want {
		t.Fatalf("expected first spinner frame %q, got %q", want, got)
	}
}

func TestQuitOnCtrlC(t *testing.T) {
	m := New(testConfig(), "", "dev")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := updated.(Model)
	if !model.quitting {
		t.Error("model should be quitting after ctrl+c")
	}
	if cmd == nil {
		t.Error("expected a quit command")
	}
}

func TestQuitOnQ(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenServiceList
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model := updated.(Model)
	if !model.quitting {
		t.Error("model should be quitting after 'q' on service list")
	}
	if cmd == nil {
		t.Error("expected a quit command")
	}
}

func TestViewNotEmpty(t *testing.T) {
	m := New(testConfig(), "", "dev")
	v := m.View()
	if v == "" {
		t.Error("view should not be empty when not quitting")
	}
}

func TestLoadingViewShowsSpinner(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading

	v := m.View()
	if !strings.Contains(v, "Loading...") {
		t.Fatalf("expected loading text, got %q", v)
	}
	if !strings.Contains(v, m.loadingSpinner.View()) {
		t.Fatalf("expected spinner frame in loading view, got %q", v)
	}
}

func TestLoadingViewShowsCustomLoadingDetails(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading
	m.loadingTitle = "Finding Network Path"
	m.loadingDetails = []string{"src-eni", "->", "dst-eni", "Intent: TCP/443"}

	v := m.View()
	for _, want := range []string{"Finding Network Path", "src-eni", "dst-eni", "Intent: TCP/443"} {
		if !strings.Contains(v, want) {
			t.Fatalf("expected loading view to contain %q, got %q", want, v)
		}
	}
}

func TestViewEmptyWhenQuitting(t *testing.T) {
	m := Model{quitting: true}
	v := m.View()
	if v != "" {
		t.Error("view should be empty when quitting")
	}
}

func TestLoadingSpinnerTickUpdatesOnlyOnLoadingScreen(t *testing.T) {
	m := New(testConfig(), "", "dev")
	initialFrame := m.loadingSpinner.View()

	updated, cmd := m.Update(m.loadingSpinner.Tick())
	model := updated.(Model)
	if cmd != nil {
		t.Fatal("expected no follow-up spinner tick off loading screen")
	}
	if model.loadingSpinner.View() != initialFrame {
		t.Fatalf("expected spinner frame to stay %q off loading screen, got %q", initialFrame, model.loadingSpinner.View())
	}

	m.screen = screenLoading
	updated, cmd = m.Update(m.loadingSpinner.Tick())
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected follow-up spinner tick on loading screen")
	}
	if model.loadingSpinner.View() == initialFrame {
		t.Fatalf("expected spinner frame to advance from %q", initialFrame)
	}
}

func TestListIndexHelpersWrapAndClamp(t *testing.T) {
	if got := previousListIndex(0, 3); got != 2 {
		t.Fatalf("expected previous from first to wrap to 2, got %d", got)
	}
	if got := nextListIndex(2, 3); got != 0 {
		t.Fatalf("expected next from last to wrap to 0, got %d", got)
	}
	if got := previousListIndex(4, 0); got != 0 {
		t.Fatalf("expected empty previous list to stay 0, got %d", got)
	}
	if got := clampListIndex(9, 3); got != 2 {
		t.Fatalf("expected clamp to last index 2, got %d", got)
	}
}

func TestServiceListNavigation(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenServiceList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.svcIdx != 1 {
		t.Errorf("expected svcIdx 1 after pressing j, got %d", model.svcIdx)
	}
}

func TestServiceListNavigationWraps(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenServiceList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model := updated.(Model)
	if got, want := model.svcIdx, len(model.serviceList())-1; got != want {
		t.Fatalf("expected up from first service to wrap to %d, got %d", want, got)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)
	if model.svcIdx != 0 {
		t.Fatalf("expected down from last service to wrap to 0, got %d", model.svcIdx)
	}
}

func TestFeatureListNavigationWraps(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenFeatureList
	m.features = []domain.Feature{
		{Kind: domain.FeatureSSMSession},
		{Kind: domain.FeatureSecurityGroupBrowser},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model := updated.(Model)
	if model.featIdx != 1 {
		t.Fatalf("expected up from first feature to wrap to last, got %d", model.featIdx)
	}
}

func TestServiceListFiltersByFeatureDescription(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenServiceList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model := updated.(Model)
	for _, ch := range []rune("long-term") {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(Model)
	}

	if got := len(model.filteredServices); got != 1 {
		t.Fatalf("expected 1 service matching feature description, got %d", got)
	}
	if got := model.filteredServices[0].Name; got != domain.ServiceBedrock {
		t.Fatalf("expected Bedrock to match feature description, got %s", got)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenFeatureList {
		t.Fatalf("expected filtered service enter to open feature list, got %v", model.screen)
	}
	if got := model.features[0].Kind; got != domain.FeatureBedrockAPIKeys {
		t.Fatalf("expected Bedrock feature after filtered enter, got %s", got)
	}
}

func TestServiceListDefaultsToAlphabeticalOrder(t *testing.T) {
	m := New(testConfig(), "", "dev")

	if got := m.filteredServices[0].Name; got != domain.ServiceBedrock {
		t.Fatalf("expected Bedrock first in name sort, got %s", got)
	}
}

func TestServiceListFavoritesSortFirst(t *testing.T) {
	cfg := testConfig()
	cfg.FavoriteServices = []string{string(domain.ServiceRDS)}
	m := New(cfg, "", "dev")

	if got := m.filteredServices[0].Name; got != domain.ServiceRDS {
		t.Fatalf("expected favorite service first, got %s", got)
	}
	if !m.isFavoriteService(domain.ServiceRDS) {
		t.Fatal("expected RDS to be tracked as a favorite")
	}
}

func TestServiceListFavoriteTogglePersists(t *testing.T) {
	originalSetFavoriteServicesFn := configSetFavoriteServicesFn
	t.Cleanup(func() {
		configSetFavoriteServicesFn = originalSetFavoriteServicesFn
	})

	var gotPath string
	var gotFavorites []string
	configSetFavoriteServicesFn = func(path string, services []string) error {
		gotPath = path
		gotFavorites = append([]string(nil), services...)
		return nil
	}

	cfg := testConfig()
	m := New(cfg, "/tmp/unic-test-config.yaml", "dev")
	m.screen = screenServiceList
	for i, svc := range m.filteredServices {
		if svc.Name == domain.ServiceRDS {
			m.svcIdx = i
			break
		}
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model := updated.(Model)
	if gotPath != "/tmp/unic-test-config.yaml" {
		t.Fatalf("expected favorite persistence path, got %q", gotPath)
	}
	if len(gotFavorites) != 1 || gotFavorites[0] != string(domain.ServiceRDS) {
		t.Fatalf("expected persisted RDS favorite, got %v", gotFavorites)
	}
	if model.filteredServices[0].Name != domain.ServiceRDS {
		t.Fatalf("expected toggled favorite to move first, got %s", model.filteredServices[0].Name)
	}
	if len(cfg.FavoriteServices) != 1 || cfg.FavoriteServices[0] != string(domain.ServiceRDS) {
		t.Fatalf("expected config favorite services to update, got %v", cfg.FavoriteServices)
	}
}

func TestServiceListEnterGoesToFeatures(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenServiceList
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenFeatureList {
		t.Errorf("expected feature list screen, got %d", model.screen)
	}
}

func TestRDSListNavigationWraps(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rds.filtered = []awsservice.RDSInstance{
		{DBInstanceID: "db-a"},
		{DBInstanceID: "db-b"},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model := updated.(Model)
	if model.rds.idx != 1 {
		t.Fatalf("expected up from first RDS instance to wrap to last, got %d", model.rds.idx)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)
	if model.rds.idx != 0 {
		t.Fatalf("expected down from last RDS instance to wrap to first, got %d", model.rds.idx)
	}
}

// --- EC2 instance browser tests ---

func TestFeatureListEC2InstanceBrowserGoesToLoading(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenFeatureList
	m.features = []domain.Feature{
		{Kind: domain.FeatureEC2InstanceBrowser, Description: "Browse EC2 instances"},
	}
	m.featIdx = 0

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenLoading {
		t.Errorf("expected loading screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Error("expected a command to load EC2 instances")
	}
}

func TestEC2BrowserInstancesLoadedMsg(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading

	instances := []awsservice.EC2Instance{
		{InstanceID: "i-1", Name: "app-a", State: "running", InstanceType: "t3.micro"},
	}
	updated, _ := m.Update(ec2BrowserInstancesLoadedMsg{instances: instances})
	model := updated.(Model)
	if model.screen != screenEC2InstanceBrowserList {
		t.Errorf("expected EC2 browser list screen, got %d", model.screen)
	}
	if len(model.ec2Browser.instances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(model.ec2Browser.instances))
	}
}

func TestEC2BrowserListNavigationWraps(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenEC2InstanceBrowserList
	m.ec2Browser.filtered = []awsservice.EC2Instance{
		{InstanceID: "i-a"},
		{InstanceID: "i-b"},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model := updated.(Model)
	if model.ec2Browser.idx != 1 {
		t.Fatalf("expected up from first EC2 instance to wrap to last, got %d", model.ec2Browser.idx)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)
	if model.ec2Browser.idx != 0 {
		t.Fatalf("expected down from last EC2 instance to wrap to first, got %d", model.ec2Browser.idx)
	}
}

func TestEC2BrowserListFilter(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenEC2InstanceBrowserList
	m.ec2Browser.instances = []awsservice.EC2Instance{
		{InstanceID: "i-prod", Name: "prod-web", State: "running", InstanceType: "t3.micro"},
		{InstanceID: "i-dev", Name: "dev-web", State: "stopped", InstanceType: "t3.small"},
	}
	m.ec2Browser.filtered = m.ec2Browser.instances

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model := updated.(Model)
	for _, ch := range []rune("prod") {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(Model)
	}

	if len(model.ec2Browser.filtered) != 1 {
		t.Errorf("expected 1 filtered instance, got %d", len(model.ec2Browser.filtered))
	}
	if model.ec2Browser.filtered[0].InstanceID != "i-prod" {
		t.Errorf("expected i-prod, got %q", model.ec2Browser.filtered[0].InstanceID)
	}
}

func TestEC2BrowserListEnterGoesToDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenEC2InstanceBrowserList
	m.ec2Browser.instances = []awsservice.EC2Instance{
		{InstanceID: "i-1", Name: "app-a"},
	}
	m.ec2Browser.filtered = m.ec2Browser.instances

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenEC2InstanceBrowserDetail {
		t.Errorf("expected EC2 browser detail screen, got %d", model.screen)
	}
	if model.ec2Browser.selected == nil {
		t.Error("ec2Browser.selected should not be nil")
	}
}

func TestEC2BrowserDetailViewShowsMetadata(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenEC2InstanceBrowserDetail
	m.ec2Browser.selected = &awsservice.EC2Instance{
		InstanceID:       "i-123",
		Name:             "prod-web",
		State:            "running",
		InstanceType:     "m6i.large",
		AvailabilityZone: "us-east-1a",
		VPCID:            "vpc-123",
		SubnetID:         "subnet-123",
		PrivateIP:        "10.0.0.10",
		PublicIP:         "203.0.113.10",
		LaunchTime:       time.Date(2026, 4, 22, 12, 30, 0, 0, time.UTC),
		PlatformDetails:  "Linux/UNIX",
		IAMProfile:       "arn:aws:iam::123456789012:instance-profile/app",
		Tags:             map[string]string{"Environment": "prod"},
	}

	view := m.View()
	for _, want := range []string{
		"EC2 Instance Detail",
		"i-123",
		"prod-web",
		"running",
		"m6i.large",
		"us-east-1a",
		"vpc-123",
		"subnet-123",
		"10.0.0.10",
		"203.0.113.10",
		"Linux/UNIX",
		"instance-profile/app",
		"Environment",
		"prod",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected EC2 detail view to contain %q, got %q", want, view)
		}
	}
}

func TestReachabilityFeatureOpensRegionSelection(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenFeatureList
	m.features = []domain.Feature{
		{Kind: domain.FeatureVPCBrowser},
		{Kind: domain.FeatureReachabilityAnalyzer},
	}
	m.featIdx = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenReachabilityRegionList {
		t.Fatalf("expected region selection screen, got %v", model.screen)
	}
	if model.reachabilityRegion != "us-east-1" {
		t.Fatalf("expected default reachability region us-east-1, got %q", model.reachabilityRegion)
	}
}

func TestReachabilityStatusBarUsesOverrideRegion(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenReachabilitySourceList
	m.reachabilityRegion = "ap-northeast-2"

	bar := m.renderStatusBar()
	if !strings.Contains(bar, "region:ap-northeast-2") {
		t.Fatalf("expected reachability override region in status bar, got %q", bar)
	}
}

func TestReachabilityTargetsLoadedBuildsSourceTypeFilter(t *testing.T) {
	m := New(testConfig(), "", "dev")
	msg := reachabilityTargetsLoadedMsg{
		targets: []awsservice.ReachabilityTarget{
			{ID: "i-1", Name: "app", Type: "EC2 instances"},
			{ID: "eni-1", Name: "db", Type: "Network interfaces"},
		},
	}

	updated, _, handled := m.handleEC2VPCMsg(msg)
	if !handled {
		t.Fatal("expected message to be handled")
	}
	model := updated.(Model)
	if got := strings.Join(model.reachabilitySourceTypes, ","); got != "EC2 instances,Network interfaces" {
		t.Fatalf("unexpected source types: %q", got)
	}
	if len(model.filteredReachabilityTargets) != 1 {
		t.Fatalf("expected only EC2 instances to be visible initially, got %d", len(model.filteredReachabilityTargets))
	}
	if model.filteredReachabilityTargets[0].Type != "EC2 instances" {
		t.Fatalf("expected EC2 instances to be prioritized, got %+v", model.filteredReachabilityTargets)
	}
}

func TestFeatureListEscGoesBack(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenFeatureList
	m.features = m.services[0].Features
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenServiceList {
		t.Errorf("expected service list screen, got %d", model.screen)
	}
}

func TestHelpToggleAndClose(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenServiceList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model := updated.(Model)
	if !model.helpVisible {
		t.Fatal("expected help overlay to open")
	}
	if model.screen != screenServiceList {
		t.Fatalf("expected screen to stay on service list, got %v", model.screen)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.helpVisible {
		t.Fatal("expected help overlay to close on esc")
	}
}

func TestHelpBlocksNavigationWhileOpen(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenServiceList
	m.svcIdx = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model := updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)

	if model.svcIdx != 0 {
		t.Fatalf("expected selection to stay unchanged while help is open, got %d", model.svcIdx)
	}
	if !model.helpVisible {
		t.Fatal("expected help overlay to remain open on non-close keys")
	}
}

func TestHelpViewShowsContextAwareRDSActions(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.rds.selected = &awsservice.RDSInstance{
		DBInstanceID:  "db-1",
		Status:        "available",
		MultiAZ:       true,
		ClusterID:     "",
		InstanceClass: "db.t3.micro",
	}
	m.helpVisible = true

	view := m.View()
	for _, want := range []string{
		"Keyboard Shortcuts",
		"Screen: RDS Detail",
		"Stop the selected instance or cluster",
		"Trigger failover for the selected instance or cluster",
		"Refresh the selected instance status",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected help view to contain %q, got %q", want, view)
		}
	}
}

func TestHelpViewShowsFilterModeShortcuts(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenSecretList
	_ = m.activateFilter(filterSecrets)
	m.helpVisible = true

	view := m.View()
	for _, want := range []string{
		"Current Mode",
		"Update the filter query",
		"Delete the previous character",
		"Move through filtered results",
		"Close filter mode",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected filter help to contain %q, got %q", want, view)
		}
	}
}

// --- RDS screen tests ---

func TestRDSListNavigation(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rds.instances = []awsservice.RDSInstance{
		{DBInstanceID: "db-1", Engine: "mysql", Status: "available", InstanceClass: "db.t3.micro"},
		{DBInstanceID: "db-2", Engine: "postgres", Status: "stopped", InstanceClass: "db.t3.small"},
	}
	m.rds.filtered = m.rds.instances

	// Press down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.rds.idx != 1 {
		t.Errorf("expected rds.idx 1 after pressing j, got %d", model.rds.idx)
	}

	// Press up
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(Model)
	if model.rds.idx != 0 {
		t.Errorf("expected rds.idx 0 after pressing k, got %d", model.rds.idx)
	}
}

func TestRDSListEnterGoesToDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rds.instances = []awsservice.RDSInstance{
		{DBInstanceID: "db-1", Engine: "mysql", Status: "available", InstanceClass: "db.t3.micro"},
	}
	m.rds.filtered = m.rds.instances

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected RDS detail screen, got %d", model.screen)
	}
	if model.rds.selected == nil {
		t.Error("rds.selected should not be nil")
	}
}

func TestRDSListEscGoesBack(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rds.instances = []awsservice.RDSInstance{}
	m.rds.filtered = m.rds.instances

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenFeatureList {
		t.Errorf("expected feature list screen, got %d", model.screen)
	}
}

func TestRDSDetailEscGoesBack(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "available"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenRDSList {
		t.Errorf("expected RDS list screen, got %d", model.screen)
	}
}

func TestRDSDetailStopGoesToConfirm(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "available", ClusterID: ""}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Errorf("expected confirm screen, got %d", model.screen)
	}
	if model.rds.action != "stop" {
		t.Errorf("expected action 'stop', got %q", model.rds.action)
	}
}

func TestRDSDetailStartGoesToConfirm(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "stopped"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model := updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Errorf("expected confirm screen, got %d", model.screen)
	}
	if model.rds.action != "start" {
		t.Errorf("expected action 'start', got %q", model.rds.action)
	}
}

func TestRDSDetailFailoverGoesToConfirm(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "available", MultiAZ: true}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model := updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Errorf("expected confirm screen, got %d", model.screen)
	}
	if model.rds.action != "failover" {
		t.Errorf("expected action 'failover', got %q", model.rds.action)
	}
}

func TestRDSDetailStopClusterMember(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "available", ClusterID: "my-cluster"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := updated.(Model)
	// Aurora cluster members can be stopped (cluster-level stop)
	if model.screen != screenRDSConfirm {
		t.Errorf("expected confirm screen for cluster stop, got %d", model.screen)
	}
	if model.rds.action != "stop" {
		t.Errorf("expected action 'stop', got %q", model.rds.action)
	}
}

func TestRDSConfirmNoGoesBack(t *testing.T) {
	// For start action, 'n' cancels back to detail
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSConfirm
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1"}
	m.rds.action = "start"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected detail screen after cancel, got %d", model.screen)
	}
}

func TestRDSConfirmEscGoesBack(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSConfirm
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1"}
	m.rds.action = "stop"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected detail screen after esc, got %d", model.screen)
	}
}

func TestRDSListFilter(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rds.instances = []awsservice.RDSInstance{
		{DBInstanceID: "prod-db", Engine: "mysql", Status: "available", InstanceClass: "db.t3.micro"},
		{DBInstanceID: "dev-db", Engine: "postgres", Status: "stopped", InstanceClass: "db.t3.small"},
	}
	m.rds.filtered = m.rds.instances

	// Activate filter
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model := updated.(Model)
	if !model.isFiltering(filterRDS) {
		t.Error("filter should be active")
	}

	// Type 'p', 'r', 'o', 'd'
	for _, ch := range "prod" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(Model)
	}

	if len(model.rds.filtered) != 1 {
		t.Errorf("expected 1 filtered instance, got %d", len(model.rds.filtered))
	}
	if model.rds.filtered[0].DBInstanceID != "prod-db" {
		t.Errorf("expected 'prod-db', got %q", model.rds.filtered[0].DBInstanceID)
	}
}

func TestRDSListFilterAllowsNavigationWhileFocused(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rds.instances = []awsservice.RDSInstance{
		{DBInstanceID: "prod-api-db", Engine: "mysql", Status: "available", InstanceClass: "db.t3.micro"},
		{DBInstanceID: "prod-worker-db", Engine: "mysql", Status: "available", InstanceClass: "db.t3.micro"},
		{DBInstanceID: "dev-db", Engine: "postgres", Status: "stopped", InstanceClass: "db.t3.small"},
	}
	m.rds.filtered = m.rds.instances

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model := updated.(Model)

	for _, ch := range "prod" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(Model)
	}

	if !model.isFiltering(filterRDS) {
		t.Fatal("expected RDS filter to stay active after typing")
	}
	if got := len(model.rds.filtered); got != 2 {
		t.Fatalf("expected 2 filtered instances, got %d", got)
	}
	if model.rds.idx != 0 {
		t.Fatalf("expected selection to reset to the first filtered row, got %d", model.rds.idx)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)

	if !model.isFiltering(filterRDS) {
		t.Fatal("expected RDS filter to remain active while navigating")
	}
	if model.rds.idx != 1 {
		t.Fatalf("expected down arrow to move selection to index 1, got %d", model.rds.idx)
	}
	if got := model.rds.filtered[model.rds.idx].DBInstanceID; got != "prod-worker-db" {
		t.Fatalf("expected selection to move to prod-worker-db, got %q", got)
	}
}

func TestRDSListFilterStillAcceptsJKCharacters(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rds.instances = []awsservice.RDSInstance{
		{DBInstanceID: "proj-jk-db", Engine: "mysql", Status: "available", InstanceClass: "db.t3.micro"},
		{DBInstanceID: "prod-db", Engine: "mysql", Status: "available", InstanceClass: "db.t3.micro"},
	}
	m.rds.filtered = m.rds.instances

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model := updated.(Model)

	for _, ch := range "jk" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(Model)
	}

	if got := model.filterValue(filterRDS); got != "jk" {
		t.Fatalf("expected filter query to accept j/k characters, got %q", got)
	}
	if got := len(model.rds.filtered); got != 1 {
		t.Fatalf("expected 1 filtered instance after typing jk, got %d", got)
	}
	if got := model.rds.filtered[0].DBInstanceID; got != "proj-jk-db" {
		t.Fatalf("expected proj-jk-db to match typed jk filter, got %q", got)
	}
}

func TestRDSActionDoneMsg_Success(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1"}

	updated, cmd := m.Update(rdsActionDoneMsg{action: "stop", instanceID: "db-1", err: nil})
	model := updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected detail screen after action done, got %d", model.screen)
	}
	if !model.rds.polling {
		t.Error("polling should be active after successful action")
	}
	if cmd == nil {
		t.Error("expected a tick command for polling")
	}
}

func TestRDSInstancesLoadedMsg(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading

	instances := []awsservice.RDSInstance{
		{DBInstanceID: "db-1", Engine: "mysql"},
	}
	updated, _ := m.Update(rdsInstancesLoadedMsg{instances: instances})
	model := updated.(Model)
	if model.screen != screenRDSList {
		t.Errorf("expected RDS list screen, got %d", model.screen)
	}
	if len(model.rds.instances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(model.rds.instances))
	}
}

func TestFeatureListRDSBrowserGoesToLoading(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenFeatureList
	m.features = []domain.Feature{
		{Kind: domain.FeatureRDSBrowser, Description: "Browse RDS instances"},
	}
	m.featIdx = 0

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenLoading {
		t.Errorf("expected loading screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Error("expected a command to load RDS instances")
	}
}

func TestFeatureListECRRepositoryBrowserGoesToLoading(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenFeatureList
	m.features = []domain.Feature{
		{Kind: domain.FeatureECRRepositoryBrowser, Description: "Browse ECR repositories"},
	}
	m.featIdx = 0

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenLoading {
		t.Errorf("expected loading screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Error("expected a command to load ECR repositories")
	}
}

func TestECRRepositoriesLoadedGoesToRepositoryList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading

	repositories := []awsservice.ECRRepository{
		{Name: "app", URI: "123456789012.dkr.ecr.us-east-1.amazonaws.com/app"},
	}
	updated, _ := m.Update(ecrRepositoriesLoadedMsg{repositories: repositories})
	model := updated.(Model)
	if model.screen != screenECRRepositoryList {
		t.Errorf("expected ECR repository list screen, got %d", model.screen)
	}
	if len(model.ecrRepositories) != 1 {
		t.Errorf("expected 1 repository, got %d", len(model.ecrRepositories))
	}
}

func TestECRRepositoryEnterLoadsImages(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenECRRepositoryList
	m.ecrRepositories = []awsservice.ECRRepository{
		{Name: "app", URI: "123456789012.dkr.ecr.us-east-1.amazonaws.com/app"},
	}
	m.filteredECRRepositories = m.ecrRepositories

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.selectedECRRepository == nil || model.selectedECRRepository.Name != "app" {
		t.Fatalf("expected selected ECR repository app, got %#v", model.selectedECRRepository)
	}
	if model.screen != screenLoading {
		t.Errorf("expected loading screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Fatal("expected a command to load ECR images")
	}
}

func TestECRImagesLoadedGoesToImageList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading
	m.selectedECRRepository = &awsservice.ECRRepository{Name: "app"}

	images := []awsservice.ECRImage{
		{RepositoryName: "app", Digest: "sha256:abc", Tags: []string{"latest"}},
	}
	updated, _ := m.Update(ecrImagesLoadedMsg{repository: "app", images: images})
	model := updated.(Model)
	if model.screen != screenECRImageList {
		t.Errorf("expected ECR image list screen, got %d", model.screen)
	}
	if len(model.ecrImages) != 1 {
		t.Errorf("expected 1 image, got %d", len(model.ecrImages))
	}
}

func TestECRImageViewHighlightsCleanupSignals(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenECRImageList
	m.height = 30
	m.selectedECRRepository = &awsservice.ECRRepository{Name: "app"}
	m.ecrImages = []awsservice.ECRImage{
		{RepositoryName: "app", Digest: "sha256:untagged"},
	}
	m.filteredECRImages = m.ecrImages

	view := m.View()
	for _, want := range []string{"ECR Images", "(untagged)", "sha256:untagged"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}

func TestRDSViewNotEmpty(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rds.instances = []awsservice.RDSInstance{
		{DBInstanceID: "db-1", Engine: "mysql", Status: "available", InstanceClass: "db.t3.micro", EngineVersion: "8.0"},
	}
	m.rds.filtered = m.rds.instances
	m.height = 30

	v := m.View()
	if v == "" {
		t.Error("RDS list view should not be empty")
	}
}

func TestRDSDetailViewNotEmpty(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.rds.selected = &awsservice.RDSInstance{
		DBInstanceID: "db-1", Engine: "mysql", EngineVersion: "8.0",
		Status: "available", InstanceClass: "db.t3.micro", MultiAZ: true, StorageGB: 20,
		Endpoint: "db-1.abc.us-east-1.rds.amazonaws.com:3306",
	}

	v := m.View()
	if v == "" {
		t.Error("RDS detail view should not be empty")
	}
}

func TestRDSConfirmViewNotEmpty(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSConfirm
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1"}
	m.rds.action = "stop"

	v := m.View()
	if v == "" {
		t.Error("RDS confirm view should not be empty")
	}
}

func TestRDSConfirmStopRequiresTypedInput(t *testing.T) {
	// Test with standalone instance (confirm target = instance ID)
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSConfirm
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1", ClusterID: ""}
	m.rds.action = "stop"
	m.rds.confirmInput = ""

	// Enter without typing anything — should stay on confirm screen
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Error("enter without input should stay on confirm screen")
	}
	if cmd != nil {
		t.Error("should not execute action without correct input")
	}

	// Type wrong text + enter — should stay on confirm screen
	model.rds.confirmInput = "wrong-name"
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Error("enter with wrong input should stay on confirm screen")
	}
	if cmd != nil {
		t.Error("should not execute action with wrong input")
	}

	// Type correct instance ID + enter — should execute
	model.rds.confirmInput = "db-1"
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected detail screen after correct input, got %d", model.screen)
	}
	if cmd == nil {
		t.Error("expected action command after correct input")
	}
}

func TestRDSConfirmStopClusterRequiresClusterID(t *testing.T) {
	// Test with Aurora cluster member (confirm target = cluster ID)
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSConfirm
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "inst-1", ClusterID: "my-cluster", Status: "available"}
	m.rds.action = "stop"
	m.rds.confirmInput = ""

	// Type instance ID (wrong target) — should stay
	m.rds.confirmInput = "inst-1"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Error("typing instance ID for cluster member should not confirm")
	}
	if cmd != nil {
		t.Error("should not execute action with instance ID for cluster action")
	}

	// Type cluster ID (correct target) — should execute
	model.rds.confirmInput = "my-cluster"
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected detail screen after typing cluster ID, got %d", model.screen)
	}
	if cmd == nil {
		t.Error("expected action command after correct cluster ID input")
	}
}

func TestRDSConfirmFailoverRequiresTypedInput(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSConfirm
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "prod-db", MultiAZ: true}
	m.rds.action = "failover"
	m.rds.confirmInput = ""

	// Enter without typing — should stay
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Error("enter without input should stay on confirm screen")
	}
	if cmd != nil {
		t.Error("should not execute action without correct input")
	}

	// Type correct instance ID + enter — should execute
	model.rds.confirmInput = "prod-db"
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected detail screen after correct input, got %d", model.screen)
	}
	if cmd == nil {
		t.Error("expected action command after correct input")
	}
}

func TestRDSConfirmStartUsesSimpleYN(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSConfirm
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "stopped"}
	m.rds.action = "start"

	// Pressing 'y' should execute immediately (no typing required)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model := updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected detail screen after 'y' on start, got %d", model.screen)
	}
	if cmd == nil {
		t.Error("expected action command after 'y' on start")
	}
}

func TestRDSConfirmInputBackspace(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSConfirm
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1"}
	m.rds.action = "stop"
	m.rds.confirmInput = ""

	// Type "abc"
	for _, ch := range "abc" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}
	if m.rds.confirmInput != "abc" {
		t.Errorf("expected 'abc', got %q", m.rds.confirmInput)
	}

	// Backspace
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	if m.rds.confirmInput != "ab" {
		t.Errorf("expected 'ab' after backspace, got %q", m.rds.confirmInput)
	}
}

func TestRDSConfirmInputResetOnEntry(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.rds.selected = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "available", ClusterID: ""}
	m.rds.confirmInput = "leftover"

	// Press 'x' to go to confirm screen
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Errorf("expected confirm screen, got %d", model.screen)
	}
	if model.rds.confirmInput != "" {
		t.Errorf("expected empty confirm input on entry, got %q", model.rds.confirmInput)
	}
}

func TestBedrockKeyListCreateLoadsIdentityWhenMissing(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenBedrockKeyList

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model := updated.(Model)
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Fatal("expected identity loading command")
	}
}

func TestBedrockKeyCreateDefaultsToCurrentIAMUser(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenBedrockKeyList
	m.callerIdentity = &awsservice.CallerIdentity{Arn: "arn:aws:iam::123456789012:user/team/bedrock-user"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model := updated.(Model)
	if model.screen != screenBedrockKeyCreate {
		t.Fatalf("expected create screen, got %d", model.screen)
	}
	if model.bedrock.createField != bedrockCreateFieldMode {
		t.Fatalf("expected mode picker first, got %d", model.bedrock.createField)
	}
	if model.bedrock.createMode != 0 {
		t.Fatalf("expected current-user mode by default, got %d", model.bedrock.createMode)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.bedrock.createField != bedrockCreateFieldExpiration {
		t.Fatalf("expected expiration field, got %d", model.bedrock.createField)
	}
	if model.bedrock.createValues["user"] != "bedrock-user" {
		t.Fatalf("expected inferred user bedrock-user, got %q", model.bedrock.createValues["user"])
	}
	if model.bedrock.createValues["user_source"] != "current" {
		t.Fatalf("expected current user source, got %q", model.bedrock.createValues["user_source"])
	}
}

func TestBedrockCreateIdentityMessageDefaultsToCurrentIAMUser(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading

	updated, _ := m.Update(bedrockCreateIdentityMsg{
		identity: &awsservice.CallerIdentity{Arn: "arn:aws:iam::123456789012:user/bedrock-user"},
	})
	model := updated.(Model)
	if model.screen != screenBedrockKeyCreate {
		t.Fatalf("expected create screen, got %d", model.screen)
	}
	if model.bedrock.createField != bedrockCreateFieldMode {
		t.Fatalf("expected mode picker, got field %d", model.bedrock.createField)
	}
	if model.bedrock.createMode != 0 {
		t.Fatalf("expected current-user mode, got %d", model.bedrock.createMode)
	}
}

func TestBedrockCreateIdentityMessageFallsBackForNonIAMUser(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading

	updated, _ := m.Update(bedrockCreateIdentityMsg{
		identity: &awsservice.CallerIdentity{Arn: "arn:aws:sts::123456789012:assumed-role/Admin/session"},
	})
	model := updated.(Model)
	if model.screen != screenBedrockKeyCreate {
		t.Fatalf("expected create screen, got %d", model.screen)
	}
	if model.bedrock.createField != bedrockCreateFieldUser {
		t.Fatalf("expected explicit user input, got field %d", model.bedrock.createField)
	}
	if model.bedrock.createMode != 1 {
		t.Fatalf("expected another-user mode, got %d", model.bedrock.createMode)
	}
	view := model.bedrock.viewCreate(model)
	if !strings.Contains(view, "Current AWS identity is not an IAM user") {
		t.Fatalf("expected non-IAM identity explanation, got %q", view)
	}
	if strings.Contains(view, "Expiration Days") {
		t.Fatalf("expiration field should be hidden until a target IAM user is entered, got %q", view)
	}
}

func TestBedrockKeyCreateAnotherUserIsExplicitOption(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenBedrockKeyCreate
	m.callerIdentity = &awsservice.CallerIdentity{Arn: "arn:aws:iam::123456789012:user/current-user"}
	m.bedrock.createField = bedrockCreateFieldMode
	m.bedrock.createMode = 0
	m.bedrock.createValues = map[string]string{}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := updated.(Model)
	if model.bedrock.createMode != 1 {
		t.Fatalf("expected another-user mode after down, got %d", model.bedrock.createMode)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.bedrock.createField != bedrockCreateFieldUser {
		t.Fatalf("expected explicit user input, got field %d", model.bedrock.createField)
	}
}

func TestBedrockKeyCreateRequiresUser(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenBedrockKeyCreate
	m.bedrock.createField = bedrockCreateFieldUser
	m.bedrock.createInput = ""

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenBedrockKeyCreate {
		t.Fatalf("expected create screen, got %d", model.screen)
	}
	if !strings.Contains(model.bedrock.status, "IAM user name") {
		t.Fatalf("expected user validation message, got %q", model.bedrock.status)
	}
}

func TestBedrockKeyCreateAdvancesToTypedConfirm(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenBedrockKeyCreate
	m.bedrock.createField = bedrockCreateFieldUser
	m.bedrock.createInput = "bedrock-user"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenBedrockKeyCreate || model.bedrock.createField != bedrockCreateFieldExpiration {
		t.Fatalf("expected expiration field, got screen=%d field=%d", model.screen, model.bedrock.createField)
	}
	if model.bedrock.createInput != "30" {
		t.Fatalf("expected default 30 day expiration, got %q", model.bedrock.createInput)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenBedrockKeyConfirm {
		t.Fatalf("expected confirm screen, got %d", model.screen)
	}
	if target := model.bedrock.confirmTarget(); target != "bedrock-user" {
		t.Fatalf("expected confirm target bedrock-user, got %q", target)
	}

	model.bedrock.confirm = "wrong"
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenBedrockKeyConfirm {
		t.Fatalf("wrong confirmation should stay on confirm screen, got %d", model.screen)
	}
	if cmd != nil {
		t.Fatal("wrong confirmation should not run a command")
	}
}

func TestParseBedrockAgeDaysAllowsNoExpiration(t *testing.T) {
	for _, input := range []string{"", "0", " 0 "} {
		got, err := parseBedrockAgeDays(input)
		if err != nil {
			t.Fatalf("expected %q to parse as no expiration, got error %v", input, err)
		}
		if got != 0 {
			t.Fatalf("expected %q to parse as 0, got %d", input, got)
		}
	}
}

func TestParseBedrockAgeDaysRejectsOutOfRangeValues(t *testing.T) {
	for _, input := range []string{"-1", "36601"} {
		if _, err := parseBedrockAgeDays(input); err == nil {
			t.Fatalf("expected %q to be rejected", input)
		}
	}
}

func TestBedrockKeyDetailRotateAndDeleteGoToConfirm(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenBedrockKeyDetail
	m.bedrock.selectedKey = &awsservice.BedrockAPIKey{CredentialID: "ACCA123", UserName: "bedrock-user", Status: "Active"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model := updated.(Model)
	if model.screen != screenBedrockKeyConfirm {
		t.Fatalf("expected rotate confirm screen, got %d", model.screen)
	}
	if model.bedrock.action != "rotate" {
		t.Fatalf("expected rotate action, got %q", model.bedrock.action)
	}

	model.screen = screenBedrockKeyDetail
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	model = updated.(Model)
	if model.screen != screenBedrockKeyConfirm {
		t.Fatalf("expected delete confirm screen, got %d", model.screen)
	}
	if model.bedrock.action != "delete" {
		t.Fatalf("expected delete action, got %q", model.bedrock.action)
	}
}

func TestBedrockKeyResultEscReloadsList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenBedrockKeyResult
	m.bedrock.generatedKey = &awsservice.GeneratedBedrockAPIKey{
		BedrockAPIKey: awsservice.BedrockAPIKey{CredentialID: "ACCA123", UserName: "bedrock-user"},
		Secret:        "secret-token",
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %d", model.screen)
	}
	if model.bedrock.generatedKey != nil {
		t.Fatal("expected generated key to be cleared")
	}
	if cmd == nil {
		t.Fatal("expected reload command")
	}
}

func TestBedrockKeyResultDoesNotRenderSecret(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenBedrockKeyResult
	m.bedrock.generatedKey = &awsservice.GeneratedBedrockAPIKey{
		BedrockAPIKey: awsservice.BedrockAPIKey{CredentialID: "ACCA123", UserName: "bedrock-user"},
		Secret:        "secret-token",
	}

	view := m.bedrock.viewResult(m)
	if strings.Contains(view, "secret-token") {
		t.Fatalf("result view should not render raw secret, got %q", view)
	}
	if strings.Contains(view, "AWS_BEARER_TOKEN_BEDROCK=secret-token") {
		t.Fatalf("result view should not render raw env export, got %q", view)
	}
	for _, want := range []string{"copy-only", "[hidden] press c to copy", "[hidden] press e to copy export"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in result view, got %q", want, view)
		}
	}
}

func TestFitToHeight(t *testing.T) {
	m := New(testConfig(), "", "dev")

	// height=0 → no change
	m.height = 0
	input := "line1\nline2\nline3"
	if got := m.fitToHeight(input); got != input {
		t.Errorf("height=0 should not change output, got %q", got)
	}

	// Content fits → padded to exact height
	m.height = 5
	input = "line1\nline2\nline3"
	got := m.fitToHeight(input)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 lines (padded), got %d", len(lines))
	}

	// Content exceeds → trimmed to height with footer preserved
	m.height = 4
	input = "line1\nline2\nline3\nline4\nline5\nfooter"
	got = m.fitToHeight(input)
	lines = strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Errorf("expected 4 lines, got %d", len(lines))
	}
}

func TestViewFitsTerminalHeight(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.height = 10
	m.rds.selected = &awsservice.RDSInstance{
		DBInstanceID: "db-1", Engine: "mysql", EngineVersion: "8.0",
		Status: "available", InstanceClass: "db.t3.micro", MultiAZ: true, StorageGB: 20,
		Endpoint: "db-1.abc.us-east-1.rds.amazonaws.com:3306",
	}

	v := m.View()
	lines := strings.Split(v, "\n")
	if len(lines) > m.height {
		t.Errorf("view output has %d lines, exceeds terminal height %d", len(lines), m.height)
	}
}

// --- Security Group tests ---

func TestSecurityGroupListNavigation(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenSecurityGroupList
	m.securityGroups = []awsservice.SecurityGroup{
		{GroupID: "sg-1", Name: "web", VPCID: "vpc-1"},
		{GroupID: "sg-2", Name: "db", VPCID: "vpc-1"},
	}
	m.filteredSecurityGroups = m.securityGroups

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.sgIdx != 1 {
		t.Errorf("expected sgIdx 1, got %d", model.sgIdx)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(Model)
	if model.sgIdx != 0 {
		t.Errorf("expected sgIdx 0, got %d", model.sgIdx)
	}
}

func TestSecurityGroupListEnterGoesToDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenSecurityGroupList
	m.securityGroups = []awsservice.SecurityGroup{
		{GroupID: "sg-1", Name: "web", VPCID: "vpc-1"},
	}
	m.filteredSecurityGroups = m.securityGroups

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenSecurityGroupDetail {
		t.Errorf("expected detail screen, got %d", model.screen)
	}
	if model.selectedSecurityGroup == nil {
		t.Error("selectedSecurityGroup should not be nil")
	}
}

func TestSecurityGroupDetailEscGoesBack(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenSecurityGroupDetail
	m.selectedSecurityGroup = &awsservice.SecurityGroup{GroupID: "sg-1"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenSecurityGroupList {
		t.Errorf("expected list screen, got %d", model.screen)
	}
}

func TestSecurityGroupFilter(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenSecurityGroupList
	m.securityGroups = []awsservice.SecurityGroup{
		{GroupID: "sg-1", Name: "web-sg", VPCID: "vpc-1"},
		{GroupID: "sg-2", Name: "db-sg", VPCID: "vpc-1"},
	}
	m.filteredSecurityGroups = m.securityGroups

	// Activate filter
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model := updated.(Model)
	if !model.isFiltering(filterSecurityGroups) {
		t.Error("filter should be active")
	}

	// Type "web"
	for _, ch := range "web" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(Model)
	}
	if len(model.filteredSecurityGroups) != 1 {
		t.Errorf("expected 1 filtered SG, got %d", len(model.filteredSecurityGroups))
	}
}

func TestSecurityGroupDetailView(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenSecurityGroupDetail
	m.height = 30
	m.selectedSecurityGroup = &awsservice.SecurityGroup{
		GroupID:     "sg-aaa",
		Name:        "web-sg",
		Description: "Web servers",
		VPCID:       "vpc-111",
		IngressRules: []awsservice.SecurityGroupRule{
			{Protocol: "tcp", FromPort: 443, ToPort: 443, CIDRV4: "0.0.0.0/0", Description: "HTTPS"},
		},
		EgressRules: []awsservice.SecurityGroupRule{
			{Protocol: "-1", CIDRV4: "0.0.0.0/0"},
		},
	}

	v := m.View()
	if !strings.Contains(v, "sg-aaa") {
		t.Error("detail view should contain group ID")
	}
	if !strings.Contains(v, "Inbound Rules") {
		t.Error("detail view should show inbound rules section")
	}
	if !strings.Contains(v, "Outbound Rules") {
		t.Error("detail view should show outbound rules section")
	}
	if !strings.Contains(v, "443") {
		t.Error("detail view should show port 443")
	}
}

func TestSecurityGroupBrowserInCatalog(t *testing.T) {
	catalog := domain.Catalog()
	for _, svc := range catalog {
		if svc.Name == domain.ServiceEC2 {
			for _, feat := range svc.Features {
				if feat.Kind == domain.FeatureSecurityGroupBrowser {
					return
				}
			}
			t.Error("EC2 should have Security Group Browser feature")
			return
		}
	}
	t.Error("EC2 service not found")
}

// --- IAM tests ---

func TestIAMFeatureListContainsSeparateActions(t *testing.T) {
	m := New(testConfig(), "", "dev")

	for _, svc := range m.services {
		if svc.Name == domain.ServiceIAM {
			m.features = svc.Features
			break
		}
	}

	if len(m.features) != 3 {
		t.Fatalf("expected 3 IAM features, got %d", len(m.features))
	}
	if m.features[0].Kind != domain.FeatureIAMUsersBrowser {
		t.Fatalf("expected first IAM feature IAMUsersBrowser, got %s", m.features[0].Kind)
	}
	if m.features[1].Kind != domain.FeatureListAccessKeys {
		t.Fatalf("expected second IAM feature ListAccessKeys, got %s", m.features[1].Kind)
	}
	if m.features[2].Kind != domain.FeatureRotateAccessKey {
		t.Fatalf("expected third IAM feature RotateAccessKey, got %s", m.features[2].Kind)
	}
}

func TestIAMUserFeatureUsesUserBrowserFlow(t *testing.T) {
	m := New(testConfig(), "", "dev")

	for _, svc := range m.services {
		if svc.Name == domain.ServiceIAM {
			m.features = svc.Features
			break
		}
	}
	m.screen = screenFeatureList
	m.featIdx = 0

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Fatal("expected load IAM users command")
	}
}

func TestIAMUserListEnterGoesToDetailLoad(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenIAMUserList
	m.iam.users = []awsservice.IAMUser{
		{UserName: "alice"},
	}
	m.iam.filteredUsers = m.iam.users

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Fatal("expected load IAM user detail command")
	}
}

func TestIAMUserListNextPageStartsIncrementalLoad(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenIAMUserList
	m.iam.users = []awsservice.IAMUser{{UserName: "alice"}}
	m.iam.filteredUsers = m.iam.users
	m.iam.userHasMore = true
	m.iam.userNextMarker = "page-2"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := updated.(Model)
	if !model.iam.userLoadingMore {
		t.Fatal("expected IAM user incremental loading to start")
	}
	if model.screen != screenIAMUserList {
		t.Fatalf("expected IAM user list screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Fatal("expected load next IAM user page command")
	}
}

func TestIAMUserListFilterStartsBackgroundSummaryLoad(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenIAMUserList
	m.iam.users = []awsservice.IAMUser{{UserName: "alice"}}
	m.iam.filteredUsers = m.iam.users
	m.iam.userHasMore = true
	m.iam.userNextMarker = "page-2"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model := updated.(Model)
	if !model.isFiltering(filterIAMUsers) {
		t.Fatal("expected IAM user filter to activate")
	}
	if !model.iam.userLoadingMore {
		t.Fatal("expected background username loading for filter")
	}
	if cmd == nil {
		t.Fatal("expected background summary load command")
	}
}

func TestHandleIAMUsersLoadedMsgAppendsPage(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenIAMUserList
	m.iam.users = []awsservice.IAMUser{{UserName: "alice"}}
	m.iam.filteredUsers = m.iam.users
	m.iam.userLoadingMore = true

	updated, _, handled := m.iam.HandleMessage(&m, iamUsersLoadedMsg{
		users:      []awsservice.IAMUser{{UserName: "bob"}},
		append:     true,
		hasMore:    true,
		nextMarker: "page-3",
	})
	if !handled {
		t.Fatal("expected IAM users message to be handled")
	}

	model := updated.(Model)
	if len(model.iam.users) != 2 {
		t.Fatalf("expected 2 IAM users after append, got %d", len(model.iam.users))
	}
	if model.iam.userLoadingMore {
		t.Fatal("expected loading-more flag to be cleared")
	}
	if !model.iam.userHasMore {
		t.Fatal("expected hasMore to remain true")
	}
	if model.iam.userNextMarker != "page-3" {
		t.Fatalf("expected next marker page-3, got %q", model.iam.userNextMarker)
	}
}

func TestIAMUserDetailShowsGroupsPoliciesAndKeys(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenIAMUserDetail
	m.iam.selectedUser = &awsservice.IAMUserDetail{
		IAMUser: awsservice.IAMUser{
			UserName:         "alice",
			UserID:           "AIDA1234",
			ARN:              "arn:aws:iam::123456789012:user/alice",
			Path:             "/engineering/",
			CreateDate:       time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			LastActivity:     time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			PasswordLastUsed: time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC),
			MFAEnabled:       true,
		},
		Groups:           []string{"admins"},
		AttachedPolicies: []string{"ReadOnlyAccess"},
		AccessKeys: []awsservice.AccessKey{
			{
				AccessKeyID: "AKIATEST",
				Status:      "Active",
				CreateDate:  time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
				LastUsed:    time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	view := m.iam.viewUserDetail(m)
	if !strings.Contains(view, "admins") {
		t.Fatalf("expected groups in detail view, got %q", view)
	}
	if !strings.Contains(view, "ReadOnlyAccess") {
		t.Fatalf("expected attached policies in detail view, got %q", view)
	}
	if !strings.Contains(view, "AKIATEST") {
		t.Fatalf("expected access key list in detail view, got %q", view)
	}
}

func TestIAMUserListShowsLoadMoreHint(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenIAMUserList
	m.iam.users = []awsservice.IAMUser{{UserName: "alice"}}
	m.iam.filteredUsers = m.iam.users
	m.iam.userHasMore = true

	view := m.iam.viewUserList(m)
	if !strings.Contains(view, "Press n to load the next page") {
		t.Fatalf("expected load-more hint in IAM user list view, got %q", view)
	}
}

func TestIAMUserListShowsFilterBackgroundLoadHint(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenIAMUserList
	m.iam.users = []awsservice.IAMUser{{UserName: "alice"}}
	m.iam.filteredUsers = m.iam.users
	m.storeFilterValue(filterIAMUsers, "ali")
	m.iam.userLoadingMore = true

	view := m.iam.viewUserList(m)
	if !strings.Contains(view, "Loading remaining IAM usernames for filter") {
		t.Fatalf("expected filter background load hint, got %q", view)
	}
}

func TestIAMKeyDetailHidesRotateActionInListMode(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.iam.rotationEnabled = false
	m.iam.selectedKey = &awsservice.AccessKey{
		AccessKeyID: "AKIATEST",
		Status:      "Active",
	}

	view := m.iam.viewKeyDetail(m)
	if !strings.Contains(view, "RotateAccessKey feature") {
		t.Fatalf("expected list mode detail view to hide direct rotate action, got %q", view)
	}
}

func TestIAMKeyDetailShowsRotateActionInRotateMode(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.iam.rotationEnabled = true
	m.iam.selectedKey = &awsservice.AccessKey{
		AccessKeyID: "AKIATEST",
		Status:      "Active",
	}

	view := m.iam.viewKeyDetail(m)
	if !strings.Contains(view, "[r] Rotate key") {
		t.Fatalf("expected rotate mode detail view to show rotate action, got %q", view)
	}
}

func TestIAMRotationResultRequiresApplyBeforeDeactivateForCredentialCurrentIdentity(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg.AuthType = config.AuthTypeCredential
	m.iam.newKey = &awsservice.NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	}
	m.iam.rotationOldKeyID = "AKIAOLDKEY"

	if m.iam.canDeactivateOldKey(m) {
		t.Fatal("expected deactivate to be blocked before apply/verify")
	}

	view := m.iam.viewKeyRotateResult(m)
	if !strings.Contains(view, "Apply to ~/.aws/credentials and verify") {
		t.Fatalf("expected apply action in result view, got %q", view)
	}
	if !strings.Contains(view, "available after apply + verify") {
		t.Fatalf("expected deactivate gating message, got %q", view)
	}
}

func TestIAMRotationResultAllowsDeactivateAfterVerify(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg.AuthType = config.AuthTypeCredential
	m.iam.newKey = &awsservice.NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	}
	m.iam.rotationOldKeyID = "AKIAOLDKEY"
	m.iam.newKeyVerified = true

	if !m.iam.canDeactivateOldKey(m) {
		t.Fatal("expected deactivate to be allowed after verification")
	}
}

func TestIAMRotationResultRequiresNoApplyForSSOContext(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg.AuthType = config.AuthTypeSSO
	m.iam.newKey = &awsservice.NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	}
	m.iam.rotationOldKeyID = "AKIAOLDKEY"

	if !m.iam.canDeactivateOldKey(m) {
		t.Fatal("expected non-credential flow to allow immediate deactivate")
	}
}

func TestIAMRotationResultShowsApplyForLegacyCredentialContext(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg.AuthType = config.AuthTypeDefault
	m.cfg.Profile = "default"
	m.cfg.RoleArn = ""
	m.cfg.SSOStartURL = ""
	m.cfg.SSOAccountID = ""
	m.cfg.SSORoleName = ""
	m.iam.newKey = &awsservice.NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	}
	m.iam.rotationOldKeyID = "AKIAOLDKEY"

	if !m.iam.requiresCredentialApplyBeforeDeactivate(m) {
		t.Fatal("expected legacy profile-based context to require apply/verify")
	}

	view := m.iam.viewKeyRotateResult(m)
	if !strings.Contains(view, "[a] Apply to ~/.aws/credentials and verify") {
		t.Fatalf("expected apply action for legacy credential context, got %q", view)
	}
}

func TestIAMRotationResultShowsApplyForImplicitDefaultProfile(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg.AuthType = config.AuthTypeDefault
	m.cfg.Profile = ""
	m.cfg.RoleArn = ""
	m.cfg.SSOStartURL = ""
	m.cfg.SSOAccountID = ""
	m.cfg.SSORoleName = ""
	m.iam.newKey = &awsservice.NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	}
	m.iam.rotationOldKeyID = "AKIAOLDKEY"

	if !m.iam.requiresCredentialApplyBeforeDeactivate(m) {
		t.Fatal("expected implicit default profile to require apply/verify")
	}

	view := m.iam.viewKeyRotateResult(m)
	if !strings.Contains(view, "[a] Apply to ~/.aws/credentials and verify") {
		t.Fatalf("expected apply action for implicit default profile, got %q", view)
	}
}

func TestIAMRotationResultShowsDisabledApplyReasonForSSO(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg.AuthType = config.AuthTypeSSO
	m.iam.newKey = &awsservice.NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	}
	m.iam.rotationOldKeyID = "AKIAOLDKEY"

	view := m.iam.viewKeyRotateResult(m)
	if !strings.Contains(view, "disabled for auth:sso") {
		t.Fatalf("expected disabled reason for sso flow, got %q", view)
	}
}

func TestRotateAccessKeyFeatureUsesCurrentIdentityFlow(t *testing.T) {
	m := New(testConfig(), "", "dev")

	for _, svc := range m.services {
		if svc.Name == domain.ServiceIAM {
			m.features = svc.Features
			break
		}
	}
	m.screen = screenFeatureList
	m.featIdx = 2

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if !model.iam.rotationEnabled {
		t.Fatal("expected IAM rotation mode to be enabled")
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Fatal("expected load IAM keys command")
	}
}

func TestS3BrowserFeatureExistsInCatalog(t *testing.T) {
	m := New(testConfig(), "", "dev")

	for _, svc := range m.services {
		if svc.Name != domain.ServiceS3 {
			continue
		}
		if len(svc.Features) != 1 {
			t.Fatalf("expected 1 S3 feature, got %d", len(svc.Features))
		}
		if svc.Features[0].Kind != domain.FeatureS3Browser {
			t.Fatalf("expected S3 browser feature, got %s", svc.Features[0].Kind)
		}
		return
	}

	t.Fatal("expected S3 service in catalog")
}

func TestS3FeatureStartsBucketLoading(t *testing.T) {
	m := New(testConfig(), "", "dev")
	for _, svc := range m.services {
		if svc.Name == domain.ServiceS3 {
			m.features = svc.Features
			break
		}
	}
	m.screen = screenFeatureList

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Fatal("expected load S3 buckets command")
	}
}

func TestServiceListInspectorKeyEntersHomeScreen(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenServiceList

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	model := updated.(Model)
	if model.screen != screenInspectorHome {
		t.Fatalf("expected inspector home screen, got %d", model.screen)
	}
	if cmd != nil {
		t.Fatal("expected no loading command when entering inspector home")
	}
}

func TestInspectorHomeStartsDedicatedScanFlow(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenInspectorHome

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenInspectorScanning {
		t.Fatalf("expected inspector scanning screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Fatal("expected security scan command")
	}
}

func TestHandleInspectorScanLoadedMsgShowsResults(t *testing.T) {
	m := New(testConfig(), "", "dev")
	report := &inspector.SecurityScanReport{
		ScannerCount: 1,
		Findings: []inspector.SecurityFinding{
			{
				RuleID:         "sg-public-ssh",
				RuleName:       "SSH exposed to the internet",
				Severity:       inspector.RuleSeverityHigh,
				ResourceType:   "SecurityGroup",
				ResourceID:     "sg-123456",
				Summary:        "Ingress rule allows TCP/22 from 0.0.0.0/0.",
				Recommendation: "Restrict SSH to trusted sources.",
			},
		},
	}

	updated, _, handled := m.handleInspectorMsg(inspectorScanLoadedMsg{report: report})
	if !handled {
		t.Fatal("expected inspector scan message to be handled")
	}

	model := updated.(Model)
	if model.screen != screenInspectorResults {
		t.Fatalf("expected inspector results screen, got %d", model.screen)
	}
	if len(model.inspectorFindings) != 1 {
		t.Fatalf("expected 1 filtered finding, got %d", len(model.inspectorFindings))
	}
}

func TestNewModelMarksChecklistWorkflowReadyWhenConfigured(t *testing.T) {
	m := New(testConfig(), "", "dev", "/tmp/checklist.yaml")

	if len(m.inspectorWorkflows) < 2 {
		t.Fatalf("expected checklist workflow to be present, got %d workflows", len(m.inspectorWorkflows))
	}
	if !m.inspectorWorkflows[1].Available {
		t.Fatalf("expected checklist workflow to be available, got %+v", m.inspectorWorkflows[1])
	}
	if got := m.inspectorWorkflows[1].StatusLabel(); got != "READY" {
		t.Fatalf("expected READY status label, got %q", got)
	}
}

func TestInspectorResultsSeverityFilterNarrowsFindings(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenInspectorResults
	m.inspectorReport = &inspector.SecurityScanReport{
		Findings: []inspector.SecurityFinding{
			{RuleName: "Critical finding", Severity: inspector.RuleSeverityCritical, ResourceID: "sg-1"},
			{RuleName: "Medium finding", Severity: inspector.RuleSeverityMedium, ResourceID: "db-1"},
		},
	}
	m.applyInspectorSeverityFilter()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	model := updated.(Model)
	if model.inspectorSeverityFilter != inspector.RuleSeverityCritical {
		t.Fatalf("expected critical severity filter, got %q", model.inspectorSeverityFilter)
	}
	if len(model.inspectorFindings) != 1 || model.inspectorFindings[0].Severity != inspector.RuleSeverityCritical {
		t.Fatalf("expected only critical findings, got %+v", model.inspectorFindings)
	}
}

func TestInspectorResultsEnterShowsDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenInspectorResults
	m.inspectorFindings = []inspector.SecurityFinding{
		{
			RuleName:       "SSH exposed to the internet",
			Severity:       inspector.RuleSeverityHigh,
			ResourceID:     "sg-123456",
			Summary:        "Ingress rule allows SSH from anywhere.",
			Recommendation: "Restrict SSH ingress to trusted sources.",
		},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenInspectorFindingDetail {
		t.Fatalf("expected inspector detail screen, got %d", model.screen)
	}
	if model.selectedInspectorFinding == nil {
		t.Fatal("expected selected inspector finding")
	}
}

func TestHandleInspectorChecklistLoadedMsgShowsResults(t *testing.T) {
	m := New(testConfig(), "", "dev", "/tmp/checklist.yaml")
	report := &inspector.ChecklistReport{
		ChecklistName: "Readiness",
		Results: []inspector.ChecklistResult{
			{
				CheckID:  "secret-ready",
				Title:    "Secret readiness",
				Type:     inspector.ChecklistCheckSecret,
				Resource: "prod/app",
				Passed:   false,
				Summary:  "1 expectation(s) did not match.",
				Details:  []string{"missing value key \"password\""},
			},
		},
	}

	updated, _, handled := m.handleInspectorMsg(inspectorChecklistLoadedMsg{report: report})
	if !handled {
		t.Fatal("expected checklist scan message to be handled")
	}

	model := updated.(Model)
	if model.screen != screenInspectorChecklistResults {
		t.Fatalf("expected checklist results screen, got %d", model.screen)
	}
	if model.inspectorChecklistReport == nil || len(model.inspectorChecklistReport.Results) != 1 {
		t.Fatalf("expected stored checklist report, got %+v", model.inspectorChecklistReport)
	}
}

func TestInspectorHomeChecklistStartsDedicatedScanWhenConfigured(t *testing.T) {
	m := New(testConfig(), "", "dev", "/tmp/checklist.yaml")
	m.screen = screenInspectorHome
	m.inspectorWorkflowIdx = 1

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenInspectorScanning {
		t.Fatalf("expected inspector scanning screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Fatal("expected checklist scan command")
	}
}

func TestInspectorHomeChecklistOpensPickerWhenUnconfigured(t *testing.T) {
	dir := t.TempDir()
	checklistPath := writeChecklistFile(t, dir, "readiness.yaml", `
name: Readiness
checks:
  - type: secret
    resource: prod/app
    expect:
      rotation_enabled: true
`)

	m := New(testConfig(), "", "dev")
	m.screen = screenInspectorHome
	m.inspectorWorkflowIdx = 1
	m.inspectorChecklistDir = dir

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenInspectorChecklistPicker {
		t.Fatalf("expected checklist picker screen, got %d", model.screen)
	}
	if cmd != nil {
		t.Fatal("expected no command when opening the checklist picker")
	}
	if len(model.filteredChecklistFiles) == 0 {
		t.Fatal("expected checklist picker entries to be loaded")
	}

	found := false
	for _, entry := range model.filteredChecklistFiles {
		if entry.Path == checklistPath {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected picker to include %s, got %+v", checklistPath, model.filteredChecklistFiles)
	}
}

func TestInspectorChecklistPickerEnterFileStartsScan(t *testing.T) {
	dir := t.TempDir()
	checklistPath := writeChecklistFile(t, dir, "readiness.yaml", `
name: Readiness
checks:
  - type: secret
    resource: prod/app
    expect:
      rotation_enabled: true
`)

	m := New(testConfig(), "", "dev")
	m.screen = screenInspectorHome
	m.inspectorWorkflowIdx = 1
	m.inspectorChecklistDir = dir

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	for i, entry := range model.filteredChecklistFiles {
		if entry.Path == checklistPath {
			model.inspectorChecklistFileIdx = i
			break
		}
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenInspectorScanning {
		t.Fatalf("expected checklist scan screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Fatal("expected checklist scan command")
	}
	if model.inspectorChecklistPath != checklistPath {
		t.Fatalf("expected checklist path %q, got %q", checklistPath, model.inspectorChecklistPath)
	}
	if !model.inspectorWorkflows[1].Available {
		t.Fatalf("expected checklist workflow to be available after load, got %+v", model.inspectorWorkflows[1])
	}
}

func TestInspectorChecklistPickerInvalidFileStaysOnPicker(t *testing.T) {
	dir := t.TempDir()
	checklistPath := writeChecklistFile(t, dir, "broken.yaml", "not: [valid")

	m := New(testConfig(), "", "dev")
	m.screen = screenInspectorHome
	m.inspectorWorkflowIdx = 1
	m.inspectorChecklistDir = dir

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	for i, entry := range model.filteredChecklistFiles {
		if entry.Path == checklistPath {
			model.inspectorChecklistFileIdx = i
			break
		}
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenInspectorChecklistPicker {
		t.Fatalf("expected checklist picker screen, got %d", model.screen)
	}
	if cmd != nil {
		t.Fatal("expected no checklist scan command for an invalid file")
	}
	if model.inspectorChecklistError == "" {
		t.Fatal("expected checklist picker error for invalid YAML")
	}
}

func TestOpenChecklistPickerErrorReturnsUpdatedModel(t *testing.T) {
	m := New(testConfig(), "", "dev")
	lockedDir := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(lockedDir, 0o755); err != nil {
		t.Fatalf("failed to create locked checklist dir: %v", err)
	}
	if err := os.Chmod(lockedDir, 0o000); err != nil {
		t.Fatalf("failed to lock checklist dir: %v", err)
	}
	defer os.Chmod(lockedDir, 0o755)

	m.inspectorChecklistDir = lockedDir

	updated, cmd := m.openChecklistPicker()
	if cmd != nil {
		t.Fatal("expected no command when checklist picker loading fails")
	}
	if updated.screen != screenError {
		t.Fatalf("expected error screen, got %d", updated.screen)
	}
	if updated.errMsg == "" {
		t.Fatal("expected error message when checklist picker directory cannot be loaded")
	}
}

func TestInspectorChecklistResultsEnterShowsDetail(t *testing.T) {
	m := New(testConfig(), "", "dev", "/tmp/checklist.yaml")
	m.screen = screenInspectorChecklistResults
	m.inspectorChecklistReport = &inspector.ChecklistReport{
		Results: []inspector.ChecklistResult{
			{
				CheckID:  "db-ready",
				Title:    "Database baseline",
				Type:     inspector.ChecklistCheckRDS,
				Resource: "prod-db",
				Passed:   true,
				Summary:  "All expectations matched.",
			},
		},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenInspectorChecklistDetail {
		t.Fatalf("expected checklist detail screen, got %d", model.screen)
	}
	if model.selectedChecklistResult == nil {
		t.Fatal("expected selected checklist result")
	}
}

func TestS3ObjectListEscAtRootReturnsToBucketList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenS3ObjectList
	m.s3.selectedBucket = &awsservice.S3Bucket{Name: "my-bucket"}
	m.s3.currentPrefix = ""

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenS3BucketList {
		t.Fatalf("expected bucket list screen, got %d", model.screen)
	}
}

func TestS3ObjectListShowsBreadcrumb(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenS3ObjectList
	m.s3.selectedBucket = &awsservice.S3Bucket{Name: "my-bucket"}
	m.s3.currentPrefix = "logs/app/"

	view := m.s3.viewObjectList(m)
	if !strings.Contains(view, "Path: /logs/app") {
		t.Fatalf("expected breadcrumb in S3 object list, got %q", view)
	}
}

func TestCWLogViewerDownDoesNotOverflowShortEventList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenCWLogViewer
	m.height = 20
	m.cwLogs.events = []awsservice.LogEvent{
		{Timestamp: time.Unix(0, 0), Message: "one"},
		{Timestamp: time.Unix(1, 0), Message: "two"},
		{Timestamp: time.Unix(2, 0), Message: "three"},
	}
	m.cwLogs.scrollOffset = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.cwLogs.scrollOffset != 0 {
		t.Fatalf("expected scroll offset to remain 0, got %d", model.cwLogs.scrollOffset)
	}
}

func TestCWLogTailAppendClampsScrollOffsetForShortEventList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 20
	m.screen = screenCWLogViewer
	m.cwLogs.tailing = true
	m.cwLogs.scrollOffset = 7
	m.cwLogs.events = []awsservice.LogEvent{
		{Timestamp: time.Unix(0, 0), Message: "one"},
		{Timestamp: time.Unix(1, 0), Message: "two"},
	}

	updated, _, handled := m.cwLogs.HandleMessage(&m, cwLogEventsLoadedMsg{
		append: true,
		events: []awsservice.LogEvent{
			{Timestamp: time.Unix(2, 0), Message: "three"},
		},
	})
	if !handled {
		t.Fatal("expected CloudWatch logs message to be handled")
	}

	model := updated.(Model)
	if model.cwLogs.scrollOffset != 0 {
		t.Fatalf("expected clamped scroll offset 0, got %d", model.cwLogs.scrollOffset)
	}
}

func TestCWLogTailTickSchedulesPollAndNextTick(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cwLogs.tailing = true
	m.cwLogs.selectedGroup = &awsservice.LogGroup{Name: "/aws/lambda/test"}

	updated, cmd, handled := m.cwLogs.HandleMessage(&m, cwLogTailTickMsg{})
	if !handled {
		t.Fatal("expected CloudWatch logs tail tick to be handled")
	}
	if cmd == nil {
		t.Fatal("expected batched tail commands")
	}

	model := updated.(Model)
	if !model.cwLogs.tailing {
		t.Fatal("expected tailing to remain enabled")
	}

	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", msg)
	}
	if len(batch) != 2 {
		t.Fatalf("expected 2 batched commands, got %d", len(batch))
	}
}

func TestCWLogTailAppendDeduplicatesExistingEventIDs(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 20
	m.screen = screenCWLogViewer
	m.cwLogs.tailing = true
	m.cwLogs.events = []awsservice.LogEvent{
		{EventID: "evt-1", Timestamp: time.Unix(0, 0), Message: "one"},
		{EventID: "evt-2", Timestamp: time.Unix(1, 0), Message: "two"},
	}

	updated, _, handled := m.cwLogs.HandleMessage(&m, cwLogEventsLoadedMsg{
		append: true,
		events: []awsservice.LogEvent{
			{EventID: "evt-2", Timestamp: time.Unix(1, 0), Message: "two"},
			{EventID: "evt-3", Timestamp: time.Unix(2, 0), Message: "three"},
		},
	})
	if !handled {
		t.Fatal("expected CloudWatch logs message to be handled")
	}

	model := updated.(Model)
	if len(model.cwLogs.events) != 3 {
		t.Fatalf("expected 3 deduplicated events, got %d", len(model.cwLogs.events))
	}
	if model.cwLogs.events[2].EventID != "evt-3" {
		t.Fatalf("expected final event to be evt-3, got %q", model.cwLogs.events[2].EventID)
	}
}

func TestCWLogTailAppendDeduplicatesEventsWithoutEventIDs(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 20
	m.screen = screenCWLogViewer
	m.cwLogs.tailing = true
	m.cwLogs.events = []awsservice.LogEvent{
		{Timestamp: time.Unix(1, 0), Message: "duplicate"},
	}

	updated, _, handled := m.cwLogs.HandleMessage(&m, cwLogEventsLoadedMsg{
		append: true,
		events: []awsservice.LogEvent{
			{Timestamp: time.Unix(1, 0), Message: "duplicate"},
			{Timestamp: time.Unix(2, 0), Message: "new event"},
		},
	})
	if !handled {
		t.Fatal("expected CloudWatch logs message to be handled")
	}

	model := updated.(Model)
	if len(model.cwLogs.events) != 2 {
		t.Fatalf("expected 2 deduplicated events, got %d", len(model.cwLogs.events))
	}
	if got := strings.TrimSpace(model.cwLogs.events[1].Message); got != "new event" {
		t.Fatalf("expected final event to be new event, got %q", got)
	}
}

func TestCWLogLoadMoreDoesNotOverwriteTailToken(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cwLogs.tailToken = stringPtr("tail-token")
	m.cwLogs.nextToken = stringPtr("page-token")

	updated, _, handled := m.cwLogs.HandleMessage(&m, cwLogEventsLoadedMsg{
		append:                true,
		nextToken:             stringPtr("older-page-token"),
		updatePaginationToken: true,
		events:                []awsservice.LogEvent{{EventID: "evt-1", Timestamp: time.Unix(0, 0), Message: "one"}},
	})
	if !handled {
		t.Fatal("expected CloudWatch logs message to be handled")
	}

	model := updated.(Model)
	if got := derefString(model.cwLogs.tailToken); got != "tail-token" {
		t.Fatalf("expected tail token to remain unchanged, got %q", got)
	}
	if got := derefString(model.cwLogs.nextToken); got != "older-page-token" {
		t.Fatalf("expected pagination token to update, got %q", got)
	}
}

func TestCWLogTailAppendDoesNotOverwritePaginationToken(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cwLogs.nextToken = stringPtr("page-token")
	m.cwLogs.tailToken = stringPtr("tail-token")

	updated, _, handled := m.cwLogs.HandleMessage(&m, cwLogEventsLoadedMsg{
		append:          true,
		nextToken:       stringPtr("new-tail-token"),
		updateTailToken: true,
		events:          []awsservice.LogEvent{{EventID: "evt-2", Timestamp: time.Unix(1, 0), Message: "two"}},
	})
	if !handled {
		t.Fatal("expected CloudWatch logs message to be handled")
	}

	model := updated.(Model)
	if got := derefString(model.cwLogs.nextToken); got != "page-token" {
		t.Fatalf("expected pagination token to remain unchanged, got %q", got)
	}
	if got := derefString(model.cwLogs.tailToken); got != "new-tail-token" {
		t.Fatalf("expected tail token to update, got %q", got)
	}
}

func TestCWLogGroupsLoadedReplacesInitialPageAndStoresNextToken(t *testing.T) {
	m := New(testConfig(), "", "dev")

	updated, _, handled := m.cwLogs.HandleMessage(&m, cwLogGroupsLoadedMsg{
		groups: []awsservice.LogGroup{
			{Name: "/aws/lambda/a"},
			{Name: "/aws/lambda/b"},
		},
		nextToken: stringPtr("page-2"),
	})
	if !handled {
		t.Fatal("expected CloudWatch log groups message to be handled")
	}

	model := updated.(Model)
	if len(model.cwLogs.groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(model.cwLogs.groups))
	}
	if got := derefString(model.cwLogs.groupNextToken); got != "page-2" {
		t.Fatalf("expected next token page-2, got %q", got)
	}
}

func TestCWLogGroupsLoadedAppendExtendsExistingList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cwLogs.groups = []awsservice.LogGroup{{Name: "/aws/lambda/a"}}

	updated, _, handled := m.cwLogs.HandleMessage(&m, cwLogGroupsLoadedMsg{
		append: true,
		groups: []awsservice.LogGroup{
			{Name: "/aws/lambda/b"},
			{Name: "/aws/lambda/c"},
		},
	})
	if !handled {
		t.Fatal("expected CloudWatch log groups message to be handled")
	}

	model := updated.(Model)
	if len(model.cwLogs.groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(model.cwLogs.groups))
	}
	if model.cwLogs.groups[2].Name != "/aws/lambda/c" {
		t.Fatalf("expected appended group /aws/lambda/c, got %q", model.cwLogs.groups[2].Name)
	}
}

func TestCWLogStreamsLoadedAppendExtendsExistingList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cwLogs.streams = []awsservice.LogStream{{Name: "stream-a"}}

	updated, _, handled := m.cwLogs.HandleMessage(&m, cwLogStreamsLoadedMsg{
		append: true,
		streams: []awsservice.LogStream{
			{Name: "stream-b"},
			{Name: "stream-c"},
		},
		nextToken: stringPtr("stream-page-3"),
	})
	if !handled {
		t.Fatal("expected CloudWatch log streams message to be handled")
	}

	model := updated.(Model)
	if len(model.cwLogs.streams) != 3 {
		t.Fatalf("expected 3 streams, got %d", len(model.cwLogs.streams))
	}
	if got := derefString(model.cwLogs.streamNextToken); got != "stream-page-3" {
		t.Fatalf("expected next token stream-page-3, got %q", got)
	}
}

func TestReachabilityResultLinesUseReadableSections(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.reachabilityResult = &awsservice.ReachabilityAnalysisResult{
		Status:           "failed",
		NetworkPathFound: false,
		Source:           awsservice.ReachabilityTarget{Name: "src", ID: "eni-1"},
		Destination:      awsservice.ReachabilityTarget{Name: "dst", ID: "eni-2"},
		Protocol:         "TCP",
		DestinationPort:  443,
		ForwardPath: []awsservice.ReachabilityPathComponent{
			{Sequence: 1, Title: "eni-1", Details: []string{"subnet subnet-1"}, Explanations: []string{"security group blocked"}},
		},
		Explanations: []awsservice.ReachabilityExplanation{
			{Summary: "ENI_SG_RULES_MISMATCH at eni-1", Details: []string{"security group: sg-1"}},
		},
	}

	lines := m.reachabilityResultLines()
	rendered := strings.Join(lines, "\n")
	if !strings.Contains(rendered, "Summary") {
		t.Fatalf("expected Summary section, got %q", rendered)
	}
	if !strings.Contains(rendered, "● eni-1") {
		t.Fatalf("expected visual path node, got %q", rendered)
	}
	if !strings.Contains(rendered, "Findings") {
		t.Fatalf("expected Findings section, got %q", rendered)
	}
}

func TestReachabilityLoadingDetailsShowSourceAndDestination(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.reachabilityRegion = "ap-northeast-2"
	m.reachabilitySource = &awsservice.ReachabilityTarget{Name: "source-eni", ID: "eni-1"}
	m.reachabilityDestination = &awsservice.ReachabilityTarget{Name: "dest-eni", ID: "eni-2"}
	m.reachabilityProtocolIdx = 0
	m.reachabilityPortInput = "443"

	details := m.reachabilityLoadingDetails()
	if len(details) < 4 {
		t.Fatalf("expected vertical loading details, got %#v", details)
	}
	if !strings.Contains(details[1], "source-eni") {
		t.Fatalf("expected source line, got %#v", details)
	}
	if !strings.Contains(details[2], "│") {
		t.Fatalf("expected connector line, got %#v", details)
	}
	if !strings.Contains(details[3], "↓") {
		t.Fatalf("expected arrow line, got %#v", details)
	}
	if !strings.Contains(details[4], "dest-eni") {
		t.Fatalf("expected destination line, got %#v", details)
	}
	rendered := strings.Join(details, "\n")
	for _, want := range []string{"Region: ap-northeast-2", "source-eni", "dest-eni", "Intent: TCP/443"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected loading details to contain %q, got %q", want, rendered)
		}
	}
}

func TestReachabilityLoadingDetailsTruncateLongLabelsForNarrowWidth(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 30
	m.reachabilitySource = &awsservice.ReachabilityTarget{Name: strings.Repeat("source-", 8), ID: "eni-1"}
	m.reachabilityDestination = &awsservice.ReachabilityTarget{Name: strings.Repeat("dest-", 8), ID: "eni-2"}

	details := m.reachabilityLoadingDetails()
	if !strings.Contains(details[1], "…") {
		t.Fatalf("expected truncated source label, got %#v", details)
	}
	if !strings.Contains(details[4], "…") {
		t.Fatalf("expected truncated destination label, got %#v", details)
	}
}

func stringPtr(s string) *string {
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
