package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func rdsModifyTestModel() Model {
	m := Model{screen: screenRDSDetail, cfg: &config.Config{Region: "us-east-1"}}
	m.rds = newRDSModel()
	m.rds.selected = &awsservice.RDSInstance{
		DBInstanceID:  "prod-db",
		Engine:        "postgres",
		EngineVersion: "16.3",
		InstanceClass: "db.t3.medium",
	}
	return m
}

func TestRDSClassesLoadedOpensPicker(t *testing.T) {
	m := rdsModifyTestModel()
	m.screen = screenLoading

	msg := rdsClassesLoadedMsg{
		instanceID: "prod-db",
		classes:    []string{"db.r6g.large", "db.t3.medium", "db.t3.large"},
	}
	_, _, handled := m.rds.HandleMessage(&m, msg)
	if !handled || m.screen != screenRDSClassPicker {
		t.Fatalf("expected class picker screen, got %v handled=%v", m.screen, handled)
	}

	view, ok := m.rds.View(m)
	if !ok || !strings.Contains(view, "db.t3.medium") || !strings.Contains(view, "(current)") {
		t.Fatalf("expected picker view with current marker, got:\n%s", view)
	}
}

func TestRDSClassPickerFilterAndSelect(t *testing.T) {
	m := rdsModifyTestModel()
	m.screen = screenRDSClassPicker
	m.rds.classes = []string{"db.r6g.large", "db.t3.medium", "db.t3.large"}
	m.rds.filteredClasses = m.rds.classes

	m.rds.classFiltering = true
	m.rds.updateClassPicker(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m.rds.updateClassPicker(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}})
	if len(m.rds.filteredClasses) != 1 || m.rds.filteredClasses[0] != "db.r6g.large" {
		t.Fatalf("expected filter to narrow classes, got %+v", m.rds.filteredClasses)
	}

	m.rds.classFiltering = false
	m.rds.updateClassPicker(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.rds.pendingClass != "db.r6g.large" || m.rds.action != "modify" {
		t.Fatalf("expected pending class selection, got %q action=%q", m.rds.pendingClass, m.rds.action)
	}
	if m.rds.applyImmediately {
		t.Fatal("expected apply-immediately to default to false")
	}
	if m.screen != screenRDSConfirm {
		t.Fatalf("expected confirm screen, got %v", m.screen)
	}
}

func TestRDSModifyConfirmRequiresInstanceIDAndTogglesApply(t *testing.T) {
	m := rdsModifyTestModel()
	m.screen = screenRDSConfirm
	m.rds.action = "modify"
	m.rds.pendingClass = "db.r6g.large"

	m.rds.updateConfirm(&m, tea.KeyMsg{Type: tea.KeyTab})
	if !m.rds.applyImmediately {
		t.Fatal("expected tab to toggle apply-immediately on")
	}

	// Wrong confirmation input must not execute
	m.rds.confirmInput = "wrong"
	_, cmd := m.rds.updateConfirm(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || m.screen != screenRDSConfirm {
		t.Fatal("expected wrong identifier to be rejected")
	}

	m.rds.confirmInput = "prod-db"
	_, cmd = m.rds.updateConfirm(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || m.screen != screenRDSDetail {
		t.Fatalf("expected confirmed modify to execute, screen=%v", m.screen)
	}

	view := m.rds.viewConfirm(m)
	for _, want := range []string{"db.t3.medium", "db.r6g.large", "yes (may cause downtime now)"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected confirm view to contain %q, got:\n%s", want, view)
		}
	}
}

func TestRDSModifyConfirmEscReturnsToPicker(t *testing.T) {
	m := rdsModifyTestModel()
	m.screen = screenRDSConfirm
	m.rds.action = "modify"

	m.rds.updateConfirm(&m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenRDSClassPicker {
		t.Fatalf("expected esc to return to the class picker, got %v", m.screen)
	}
}

func TestRDSClassPickerRejectsCurrentClass(t *testing.T) {
	m := rdsModifyTestModel()
	m.screen = screenRDSClassPicker
	m.rds.classes = []string{"db.t3.medium", "db.t3.large"}
	m.rds.filteredClasses = m.rds.classes
	m.rds.classIdx = 0 // db.t3.medium == current

	m.rds.updateClassPicker(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenRDSClassPicker || m.rds.action == "modify" {
		t.Fatalf("expected current-class selection to be a no-op, screen=%v action=%q", m.screen, m.rds.action)
	}
}

func TestRDSImmediateModifyKeepsPollingWhilePending(t *testing.T) {
	m := rdsModifyTestModel()
	m.screen = screenRDSDetail
	m.rds.action = "modify"
	m.rds.applyImmediately = true
	m.rds.polling = true
	m.rds.instances = []awsservice.RDSInstance{*m.rds.selected}

	// available but the class change is still pending -> keep polling
	pending := *m.rds.selected
	pending.Status = "available"
	pending.PendingInstanceClass = "db.r6g.large"
	_, cmd, _ := m.rds.HandleMessage(&m, rdsStatusRefreshedMsg{instance: &pending})
	if cmd == nil || !m.rds.polling {
		t.Fatal("expected polling to continue while an immediate modify is pending")
	}

	// pending cleared -> polling stops
	settled := pending
	settled.PendingInstanceClass = ""
	settled.InstanceClass = "db.r6g.large"
	_, _, _ = m.rds.HandleMessage(&m, rdsStatusRefreshedMsg{instance: &settled})
	if m.rds.polling {
		t.Fatal("expected polling to stop once the pending class clears")
	}
}

func TestRDSDeferredModifyStopsPollingAndShowsPendingClass(t *testing.T) {
	m := rdsModifyTestModel()
	m.screen = screenRDSDetail
	m.rds.action = "modify"
	m.rds.applyImmediately = false
	m.rds.polling = true
	m.rds.instances = []awsservice.RDSInstance{*m.rds.selected}

	deferred := *m.rds.selected
	deferred.Status = "available"
	deferred.PendingInstanceClass = "db.r6g.large"
	_, _, _ = m.rds.HandleMessage(&m, rdsStatusRefreshedMsg{instance: &deferred})
	if m.rds.polling {
		t.Fatal("expected deferred modify to stop polling once status is stable")
	}

	view := m.rds.viewDetail(m)
	if !strings.Contains(view, "db.r6g.large") || !strings.Contains(view, "next maintenance window") {
		t.Fatalf("expected pending class line in detail view, got:\n%s", view)
	}
}
