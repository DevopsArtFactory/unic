package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

func alarmsTestModel() Model {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	m.screen = screenLoading
	return m
}

func testAlarms() []awsservice.CloudWatchAlarm {
	return []awsservice.CloudWatchAlarm{
		{Name: "db-cpu-high", State: "ALARM", Namespace: "AWS/RDS", MetricName: "CPUUtilization",
			Dimensions: []awsservice.CloudWatchAlarmDimension{{Name: "DBInstanceIdentifier", Value: "prod-db"}}},
		{Name: "healthy", State: "OK", Namespace: "AWS/EC2", MetricName: "CPUUtilization",
			Dimensions: []awsservice.CloudWatchAlarmDimension{{Name: "InstanceId", Value: "i-123"}}},
	}
}

func TestAlarmsLoadedOpensListWithStateTabs(t *testing.T) {
	m := alarmsTestModel()

	_, _, handled := m.cwAlarms.HandleMessage(&m, cwAlarmsLoadedMsg{alarms: testAlarms()})
	if !handled || m.screen != screenCWAlarmList {
		t.Fatalf("expected alarm list screen, got %v handled=%v", m.screen, handled)
	}
	if len(m.cwAlarms.filtered) != 2 {
		t.Fatalf("expected all alarms under ALL tab, got %d", len(m.cwAlarms.filtered))
	}

	view, ok := m.cwAlarms.View(m)
	if !ok || !strings.Contains(view, "db-cpu-high") || !strings.Contains(view, "[ALL]") {
		t.Fatalf("expected alarm list view with state tabs, got:\n%s", view)
	}
}

func TestAlarmsStateTabFilters(t *testing.T) {
	m := alarmsTestModel()
	m.cwAlarms.HandleMessage(&m, cwAlarmsLoadedMsg{alarms: testAlarms()})

	// ALL -> ALARM
	m.cwAlarms.updateList(&m, tea.KeyMsg{Type: tea.KeyTab})
	if len(m.cwAlarms.filtered) != 1 || m.cwAlarms.filtered[0].State != "ALARM" {
		t.Fatalf("expected only firing alarms on the ALARM tab, got %+v", m.cwAlarms.filtered)
	}
}

func TestAlarmDetailRelatedResourceJump(t *testing.T) {
	m := alarmsTestModel()
	m.cwAlarms.HandleMessage(&m, cwAlarmsLoadedMsg{alarms: testAlarms()})
	alarm := testAlarms()[0]
	m.cwAlarms.selected = &alarm
	m.screen = screenCWAlarmDetail

	newM, cmd, handled := m.cwAlarms.HandleKey(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if !handled || cmd == nil {
		t.Fatal("expected related-resource jump to start")
	}
	model := newM.(Model)
	if model.filterValue(filterRDS) != "prod-db" {
		t.Fatalf("expected RDS filter prefilled from the alarm dimension, got %q", model.filterValue(filterRDS))
	}
	if model.activeService != domain.ServiceRDS {
		t.Fatalf("expected active service RDS, got %v", model.activeService)
	}
}

func TestAlarmRelatedLogGroupForLambda(t *testing.T) {
	alarm := awsservice.CloudWatchAlarm{
		Dimensions: []awsservice.CloudWatchAlarmDimension{{Name: "FunctionName", Value: "checkout"}},
	}
	logGroup, ok := alarmRelatedLogGroup(alarm)
	if !ok || logGroup != "/aws/lambda/checkout" {
		t.Fatalf("expected derived lambda log group, got %q ok=%v", logGroup, ok)
	}

	if _, ok := alarmRelatedLogGroup(awsservice.CloudWatchAlarm{}); ok {
		t.Fatal("expected no log group without a mappable dimension")
	}
}

func TestAlarmHistoryLoadedOpensDetail(t *testing.T) {
	m := alarmsTestModel()
	alarm := testAlarms()[0]
	m.cwAlarms.selected = &alarm

	_, _, handled := m.cwAlarms.HandleMessage(&m, cwAlarmHistoryLoadedMsg{
		alarmName: "db-cpu-high",
		items:     []awsservice.CloudWatchAlarmHistoryItem{{Summary: "OK to ALARM"}},
	})
	if !handled || m.screen != screenCWAlarmDetail {
		t.Fatalf("expected detail screen after history load, got %v", m.screen)
	}
	view, ok := m.cwAlarms.View(m)
	if !ok || !strings.Contains(view, "OK to ALARM") || !strings.Contains(view, "prod-db") {
		t.Fatalf("expected detail view with history and dimensions, got:\n%s", view)
	}
}
