package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

func testConfig() *config.Config {
	return &config.Config{Profile: "default", Region: "us-east-1"}
}

func TestNewModelNotQuitting(t *testing.T) {
	m := New(testConfig(), "")
	if m.quitting {
		t.Error("new model should not be quitting")
	}
}

func TestNewModelStartsOnContextPicker(t *testing.T) {
	m := New(testConfig(), "")
	if m.screen != screenContextPicker {
		t.Error("new model should start on context picker screen")
	}
}

func TestQuitOnCtrlC(t *testing.T) {
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
	m.screen = screenServiceList
	// Press down — should move to index 1 (now 2 services: EC2, VPC)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.svcIdx != 1 {
		t.Errorf("expected svcIdx 1 after pressing j, got %d", model.svcIdx)
	}
}

func TestServiceListEnterGoesToFeatures(t *testing.T) {
	m := New(testConfig(), "")
	m.screen = screenServiceList
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenFeatureList {
		t.Errorf("expected feature list screen, got %d", model.screen)
	}
}

func TestFeatureListEscGoesBack(t *testing.T) {
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
	m.screen = screenRDSDetail
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "available"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenRDSList {
		t.Errorf("expected RDS list screen, got %d", model.screen)
	}
}

func TestRDSDetailStopGoesToConfirm(t *testing.T) {
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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

func TestRDSDetailNoStopForClusterMember(t *testing.T) {
	m := New(testConfig(), "")
	m.screen = screenRDSDetail
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1", Status: "available", ClusterID: "my-cluster"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := updated.(Model)
	// Should stay on detail screen since CanStop() is false
	if model.screen != screenRDSDetail {
		t.Errorf("expected to stay on detail screen, got %d", model.screen)
	}
}

func TestRDSConfirmNoGoesBack(t *testing.T) {
	m := New(testConfig(), "")
	m.screen = screenRDSConfirm
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1"}
	m.rdsAction = "stop"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := updated.(Model)
	if model.screen != screenRDSDetail {
		t.Errorf("expected detail screen after cancel, got %d", model.screen)
	}
}

func TestRDSConfirmEscGoesBack(t *testing.T) {
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
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
	m := New(testConfig(), "")
	m.screen = screenRDSConfirm
	m.selectedRDS = &awsservice.RDSInstance{DBInstanceID: "db-1"}
	m.rdsAction = "stop"

	v := m.View()
	if v == "" {
		t.Error("RDS confirm view should not be empty")
	}
}
