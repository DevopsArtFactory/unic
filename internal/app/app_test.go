package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

func testConfig() *config.Config {
	return &config.Config{Profile: "default", Region: "us-east-1"}
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

func TestViewEmptyWhenQuitting(t *testing.T) {
	m := Model{quitting: true}
	v := m.View()
	if v != "" {
		t.Error("view should be empty when quitting")
	}
}

func TestServiceListNavigation(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenServiceList
	// Press down — should move to index 1 (now 2 services: EC2, VPC)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.svcIdx != 1 {
		t.Errorf("expected svcIdx 1 after pressing j, got %d", model.svcIdx)
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

// --- RDS screen tests ---

func TestRDSListNavigation(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rdsInstances = []awsservice.RDSInstance{
		{DBInstanceID: "db-1", Engine: "mysql", Status: "available", InstanceClass: "db.t3.micro"},
		{DBInstanceID: "db-2", Engine: "postgres", Status: "stopped", InstanceClass: "db.t3.small"},
	}
	m.filteredRDS = m.rdsInstances

	// Press down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.rdsIdx != 1 {
		t.Errorf("expected rdsIdx 1 after pressing j, got %d", model.rdsIdx)
	}

	// Press up
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(Model)
	if model.rdsIdx != 0 {
		t.Errorf("expected rdsIdx 0 after pressing k, got %d", model.rdsIdx)
	}
}

func TestRDSListEnterGoesToDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rdsInstances = []awsservice.RDSInstance{
		{DBInstanceID: "db-1", Engine: "mysql", Status: "available", InstanceClass: "db.t3.micro"},
	}
	m.filteredRDS = m.rdsInstances

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected RDS detail screen, got %d", model.screen)
	}
	if model.selectedRDS == nil {
		t.Error("selectedRDS should not be nil")
	}
}

func TestRDSListEscGoesBack(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rdsInstances = []awsservice.RDSInstance{}
	m.filteredRDS = m.rdsInstances

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenFeatureList {
		t.Errorf("expected feature list screen, got %d", model.screen)
	}
}

func TestRDSDetailEscGoesBack(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "available"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenRDSList {
		t.Errorf("expected RDS list screen, got %d", model.screen)
	}
}

func TestRDSDetailStopGoesToConfirm(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "available", ClusterID: ""}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Errorf("expected confirm screen, got %d", model.screen)
	}
	if model.rdsAction != "stop" {
		t.Errorf("expected action 'stop', got %q", model.rdsAction)
	}
}

func TestRDSDetailStartGoesToConfirm(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "stopped"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model := updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Errorf("expected confirm screen, got %d", model.screen)
	}
	if model.rdsAction != "start" {
		t.Errorf("expected action 'start', got %q", model.rdsAction)
	}
}

func TestRDSDetailFailoverGoesToConfirm(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "available", MultiAZ: true}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model := updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Errorf("expected confirm screen, got %d", model.screen)
	}
	if model.rdsAction != "failover" {
		t.Errorf("expected action 'failover', got %q", model.rdsAction)
	}
}

func TestRDSDetailStopClusterMember(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "available", ClusterID: "my-cluster"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := updated.(Model)
	// Aurora cluster members can be stopped (cluster-level stop)
	if model.screen != screenRDSConfirm {
		t.Errorf("expected confirm screen for cluster stop, got %d", model.screen)
	}
	if model.rdsAction != "stop" {
		t.Errorf("expected action 'stop', got %q", model.rdsAction)
	}
}

func TestRDSConfirmNoGoesBack(t *testing.T) {
	// For start action, 'n' cancels back to detail
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSConfirm
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1"}
	m.rdsAction = "start"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected detail screen after cancel, got %d", model.screen)
	}
}

func TestRDSConfirmEscGoesBack(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSConfirm
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1"}
	m.rdsAction = "stop"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected detail screen after esc, got %d", model.screen)
	}
}

func TestRDSListFilter(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rdsInstances = []awsservice.RDSInstance{
		{DBInstanceID: "prod-db", Engine: "mysql", Status: "available", InstanceClass: "db.t3.micro"},
		{DBInstanceID: "dev-db", Engine: "postgres", Status: "stopped", InstanceClass: "db.t3.small"},
	}
	m.filteredRDS = m.rdsInstances

	// Activate filter
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model := updated.(Model)
	if !model.rdsFilterActive {
		t.Error("filter should be active")
	}

	// Type 'p', 'r', 'o', 'd'
	for _, ch := range "prod" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(Model)
	}

	if len(model.filteredRDS) != 1 {
		t.Errorf("expected 1 filtered instance, got %d", len(model.filteredRDS))
	}
	if model.filteredRDS[0].DBInstanceID != "prod-db" {
		t.Errorf("expected 'prod-db', got %q", model.filteredRDS[0].DBInstanceID)
	}
}

func TestRDSActionDoneMsg_Success(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1"}

	updated, cmd := m.Update(rdsActionDoneMsg{action: "stop", instanceID: "db-1", err: nil})
	model := updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected detail screen after action done, got %d", model.screen)
	}
	if !model.rdsPolling {
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
	if len(model.rdsInstances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(model.rdsInstances))
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

func TestRDSViewNotEmpty(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSList
	m.rdsInstances = []awsservice.RDSInstance{
		{DBInstanceID: "db-1", Engine: "mysql", Status: "available", InstanceClass: "db.t3.micro", EngineVersion: "8.0"},
	}
	m.filteredRDS = m.rdsInstances
	m.height = 30

	v := m.View()
	if v == "" {
		t.Error("RDS list view should not be empty")
	}
}

func TestRDSDetailViewNotEmpty(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.selectedRDS = &awsservice.RDSInstance{
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
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1"}
	m.rdsAction = "stop"

	v := m.View()
	if v == "" {
		t.Error("RDS confirm view should not be empty")
	}
}

func TestRDSConfirmStopRequiresTypedInput(t *testing.T) {
	// Test with standalone instance (confirm target = instance ID)
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSConfirm
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1", ClusterID: ""}
	m.rdsAction = "stop"
	m.rdsConfirmInput = ""

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
	model.rdsConfirmInput = "wrong-name"
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Error("enter with wrong input should stay on confirm screen")
	}
	if cmd != nil {
		t.Error("should not execute action with wrong input")
	}

	// Type correct instance ID + enter — should execute
	model.rdsConfirmInput = "db-1"
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
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "inst-1", ClusterID: "my-cluster", Status: "available"}
	m.rdsAction = "stop"
	m.rdsConfirmInput = ""

	// Type instance ID (wrong target) — should stay
	m.rdsConfirmInput = "inst-1"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Error("typing instance ID for cluster member should not confirm")
	}
	if cmd != nil {
		t.Error("should not execute action with instance ID for cluster action")
	}

	// Type cluster ID (correct target) — should execute
	model.rdsConfirmInput = "my-cluster"
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
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "prod-db", MultiAZ: true}
	m.rdsAction = "failover"
	m.rdsConfirmInput = ""

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
	model.rdsConfirmInput = "prod-db"
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
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "stopped"}
	m.rdsAction = "start"

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
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1"}
	m.rdsAction = "stop"
	m.rdsConfirmInput = ""

	// Type "abc"
	for _, ch := range "abc" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}
	if m.rdsConfirmInput != "abc" {
		t.Errorf("expected 'abc', got %q", m.rdsConfirmInput)
	}

	// Backspace
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	if m.rdsConfirmInput != "ab" {
		t.Errorf("expected 'ab' after backspace, got %q", m.rdsConfirmInput)
	}
}

func TestRDSConfirmInputResetOnEntry(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenRDSDetail
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "available", ClusterID: ""}
	m.rdsConfirmInput = "leftover"

	// Press 'x' to go to confirm screen
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := updated.(Model)
	if model.screen != screenRDSConfirm {
		t.Errorf("expected confirm screen, got %d", model.screen)
	}
	if model.rdsConfirmInput != "" {
		t.Errorf("expected empty confirm input on entry, got %q", model.rdsConfirmInput)
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
	m.selectedRDS = &awsservice.RDSInstance{
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
	if !model.sgFilterActive {
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
	m.iamUsers = []awsservice.IAMUser{
		{UserName: "alice"},
	}
	m.filteredIAMUsers = m.iamUsers

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
	m.iamUsers = []awsservice.IAMUser{{UserName: "alice"}}
	m.filteredIAMUsers = m.iamUsers
	m.iamUserHasMore = true
	m.iamUserNextMarker = "page-2"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := updated.(Model)
	if !model.iamUserLoadingMore {
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
	m.iamUsers = []awsservice.IAMUser{{UserName: "alice"}}
	m.filteredIAMUsers = m.iamUsers
	m.iamUserHasMore = true
	m.iamUserNextMarker = "page-2"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model := updated.(Model)
	if !model.iamUserFilterActive {
		t.Fatal("expected IAM user filter to activate")
	}
	if !model.iamUserLoadingMore {
		t.Fatal("expected background username loading for filter")
	}
	if cmd == nil {
		t.Fatal("expected background summary load command")
	}
}

func TestHandleIAMUsersLoadedMsgAppendsPage(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenIAMUserList
	m.iamUsers = []awsservice.IAMUser{{UserName: "alice"}}
	m.filteredIAMUsers = m.iamUsers
	m.iamUserLoadingMore = true

	updated, _, handled := m.handleIAMMsg(iamUsersLoadedMsg{
		users:      []awsservice.IAMUser{{UserName: "bob"}},
		append:     true,
		hasMore:    true,
		nextMarker: "page-3",
	})
	if !handled {
		t.Fatal("expected IAM users message to be handled")
	}

	model := updated.(Model)
	if len(model.iamUsers) != 2 {
		t.Fatalf("expected 2 IAM users after append, got %d", len(model.iamUsers))
	}
	if model.iamUserLoadingMore {
		t.Fatal("expected loading-more flag to be cleared")
	}
	if !model.iamUserHasMore {
		t.Fatal("expected hasMore to remain true")
	}
	if model.iamUserNextMarker != "page-3" {
		t.Fatalf("expected next marker page-3, got %q", model.iamUserNextMarker)
	}
}

func TestIAMUserDetailShowsGroupsPoliciesAndKeys(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenIAMUserDetail
	m.selectedIAMUser = &awsservice.IAMUserDetail{
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

	view := m.viewIAMUserDetail()
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
	m.iamUsers = []awsservice.IAMUser{{UserName: "alice"}}
	m.filteredIAMUsers = m.iamUsers
	m.iamUserHasMore = true

	view := m.viewIAMUserList()
	if !strings.Contains(view, "Press n to load the next page") {
		t.Fatalf("expected load-more hint in IAM user list view, got %q", view)
	}
}

func TestIAMUserListShowsFilterBackgroundLoadHint(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenIAMUserList
	m.iamUsers = []awsservice.IAMUser{{UserName: "alice"}}
	m.filteredIAMUsers = m.iamUsers
	m.iamUserFilterActive = true
	m.iamUserLoadingMore = true

	view := m.viewIAMUserList()
	if !strings.Contains(view, "Loading remaining IAM usernames for filter") {
		t.Fatalf("expected filter background load hint, got %q", view)
	}
}

func TestIAMKeyDetailHidesRotateActionInListMode(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.iamRotationEnabled = false
	m.selectedIAMKey = &awsservice.AccessKey{
		AccessKeyID: "AKIATEST",
		Status:      "Active",
	}

	view := m.viewIAMKeyDetail()
	if !strings.Contains(view, "RotateAccessKey feature") {
		t.Fatalf("expected list mode detail view to hide direct rotate action, got %q", view)
	}
}

func TestIAMKeyDetailShowsRotateActionInRotateMode(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.iamRotationEnabled = true
	m.selectedIAMKey = &awsservice.AccessKey{
		AccessKeyID: "AKIATEST",
		Status:      "Active",
	}

	view := m.viewIAMKeyDetail()
	if !strings.Contains(view, "[r] Rotate key") {
		t.Fatalf("expected rotate mode detail view to show rotate action, got %q", view)
	}
}

func TestIAMRotationResultRequiresApplyBeforeDeactivateForCredentialCurrentIdentity(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg.AuthType = config.AuthTypeCredential
	m.iamNewKey = &awsservice.NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	}
	m.iamRotationOldKeyID = "AKIAOLDKEY"

	if m.canDeactivateIAMOldKey() {
		t.Fatal("expected deactivate to be blocked before apply/verify")
	}

	view := m.viewIAMKeyRotateResult()
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
	m.iamNewKey = &awsservice.NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	}
	m.iamRotationOldKeyID = "AKIAOLDKEY"
	m.iamNewKeyVerified = true

	if !m.canDeactivateIAMOldKey() {
		t.Fatal("expected deactivate to be allowed after verification")
	}
}

func TestIAMRotationResultRequiresNoApplyForSSOContext(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg.AuthType = config.AuthTypeSSO
	m.iamNewKey = &awsservice.NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	}
	m.iamRotationOldKeyID = "AKIAOLDKEY"

	if !m.canDeactivateIAMOldKey() {
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
	m.iamNewKey = &awsservice.NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	}
	m.iamRotationOldKeyID = "AKIAOLDKEY"

	if !m.requiresIAMCredentialApplyBeforeDeactivate() {
		t.Fatal("expected legacy profile-based context to require apply/verify")
	}

	view := m.viewIAMKeyRotateResult()
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
	m.iamNewKey = &awsservice.NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	}
	m.iamRotationOldKeyID = "AKIAOLDKEY"

	if !m.requiresIAMCredentialApplyBeforeDeactivate() {
		t.Fatal("expected implicit default profile to require apply/verify")
	}

	view := m.viewIAMKeyRotateResult()
	if !strings.Contains(view, "[a] Apply to ~/.aws/credentials and verify") {
		t.Fatalf("expected apply action for implicit default profile, got %q", view)
	}
}

func TestIAMRotationResultShowsDisabledApplyReasonForSSO(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg.AuthType = config.AuthTypeSSO
	m.iamNewKey = &awsservice.NewAccessKey{
		AccessKeyID:     "AKIANEWKEY",
		SecretAccessKey: "secret",
	}
	m.iamRotationOldKeyID = "AKIAOLDKEY"

	view := m.viewIAMKeyRotateResult()
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
	if !model.iamRotationEnabled {
		t.Fatal("expected IAM rotation mode to be enabled")
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %d", model.screen)
	}
	if cmd == nil {
		t.Fatal("expected load IAM keys command")
	}
}

func TestCWLogViewerDownDoesNotOverflowShortEventList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenCWLogViewer
	m.height = 20
	m.cwLogEvents = []awsservice.LogEvent{
		{Timestamp: time.Unix(0, 0), Message: "one"},
		{Timestamp: time.Unix(1, 0), Message: "two"},
		{Timestamp: time.Unix(2, 0), Message: "three"},
	}
	m.cwLogScrollOffset = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.cwLogScrollOffset != 0 {
		t.Fatalf("expected scroll offset to remain 0, got %d", model.cwLogScrollOffset)
	}
}

func TestCWLogTailAppendClampsScrollOffsetForShortEventList(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 20
	m.screen = screenCWLogViewer
	m.cwLogTailing = true
	m.cwLogScrollOffset = 7
	m.cwLogEvents = []awsservice.LogEvent{
		{Timestamp: time.Unix(0, 0), Message: "one"},
		{Timestamp: time.Unix(1, 0), Message: "two"},
	}

	updated, _, handled := m.handleCloudWatchLogsMsg(cwLogEventsLoadedMsg{
		append: true,
		events: []awsservice.LogEvent{
			{Timestamp: time.Unix(2, 0), Message: "three"},
		},
	})
	if !handled {
		t.Fatal("expected CloudWatch logs message to be handled")
	}

	model := updated.(Model)
	if model.cwLogScrollOffset != 0 {
		t.Fatalf("expected clamped scroll offset 0, got %d", model.cwLogScrollOffset)
	}
}

func TestCWLogTailTickSchedulesPollAndNextTick(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cwLogTailing = true
	m.selectedCWLogGroup = &awsservice.LogGroup{Name: "/aws/lambda/test"}

	updated, cmd, handled := m.handleCloudWatchLogsMsg(cwLogTailTickMsg{})
	if !handled {
		t.Fatal("expected CloudWatch logs tail tick to be handled")
	}
	if cmd == nil {
		t.Fatal("expected batched tail commands")
	}

	model := updated.(Model)
	if !model.cwLogTailing {
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
	m.cwLogTailing = true
	m.cwLogEvents = []awsservice.LogEvent{
		{EventID: "evt-1", Timestamp: time.Unix(0, 0), Message: "one"},
		{EventID: "evt-2", Timestamp: time.Unix(1, 0), Message: "two"},
	}

	updated, _, handled := m.handleCloudWatchLogsMsg(cwLogEventsLoadedMsg{
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
	if len(model.cwLogEvents) != 3 {
		t.Fatalf("expected 3 deduplicated events, got %d", len(model.cwLogEvents))
	}
	if model.cwLogEvents[2].EventID != "evt-3" {
		t.Fatalf("expected final event to be evt-3, got %q", model.cwLogEvents[2].EventID)
	}
}

func TestCWLogTailAppendDeduplicatesEventsWithoutEventIDs(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 20
	m.screen = screenCWLogViewer
	m.cwLogTailing = true
	m.cwLogEvents = []awsservice.LogEvent{
		{Timestamp: time.Unix(1, 0), Message: "duplicate"},
	}

	updated, _, handled := m.handleCloudWatchLogsMsg(cwLogEventsLoadedMsg{
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
	if len(model.cwLogEvents) != 2 {
		t.Fatalf("expected 2 deduplicated events, got %d", len(model.cwLogEvents))
	}
	if got := strings.TrimSpace(model.cwLogEvents[1].Message); got != "new event" {
		t.Fatalf("expected final event to be new event, got %q", got)
	}
}

func TestCWLogLoadMoreDoesNotOverwriteTailToken(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cwLogTailToken = stringPtr("tail-token")
	m.cwLogNextToken = stringPtr("page-token")

	updated, _, handled := m.handleCloudWatchLogsMsg(cwLogEventsLoadedMsg{
		append:                true,
		nextToken:             stringPtr("older-page-token"),
		updatePaginationToken: true,
		events:                []awsservice.LogEvent{{EventID: "evt-1", Timestamp: time.Unix(0, 0), Message: "one"}},
	})
	if !handled {
		t.Fatal("expected CloudWatch logs message to be handled")
	}

	model := updated.(Model)
	if got := derefString(model.cwLogTailToken); got != "tail-token" {
		t.Fatalf("expected tail token to remain unchanged, got %q", got)
	}
	if got := derefString(model.cwLogNextToken); got != "older-page-token" {
		t.Fatalf("expected pagination token to update, got %q", got)
	}
}

func TestCWLogTailAppendDoesNotOverwritePaginationToken(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cwLogNextToken = stringPtr("page-token")
	m.cwLogTailToken = stringPtr("tail-token")

	updated, _, handled := m.handleCloudWatchLogsMsg(cwLogEventsLoadedMsg{
		append:          true,
		nextToken:       stringPtr("new-tail-token"),
		updateTailToken: true,
		events:          []awsservice.LogEvent{{EventID: "evt-2", Timestamp: time.Unix(1, 0), Message: "two"}},
	})
	if !handled {
		t.Fatal("expected CloudWatch logs message to be handled")
	}

	model := updated.(Model)
	if got := derefString(model.cwLogNextToken); got != "page-token" {
		t.Fatalf("expected pagination token to remain unchanged, got %q", got)
	}
	if got := derefString(model.cwLogTailToken); got != "new-tail-token" {
		t.Fatalf("expected tail token to update, got %q", got)
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
