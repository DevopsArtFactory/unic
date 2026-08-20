package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func TestWatchToggleAndIntervalPreset(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenCWAlarmList

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'W'}})
	m = updated.(Model)
	if !m.watch.enabled || m.watch.target != screenCWAlarmList || cmd == nil {
		t.Fatalf("expected watch to start on the alarm list, watch=%+v cmd=%v", m.watch, cmd)
	}
	if got := m.watch.interval().String(); got != "5s" {
		t.Fatalf("expected default 5s interval, got %s", got)
	}
	if badge := m.watchBadge(); !strings.Contains(badge, "watch 5s") {
		t.Fatalf("expected active watch badge, got %q", badge)
	}

	oldToken := m.watch.token
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}})
	m = updated.(Model)
	if got := m.watch.interval().String(); got != "15s" {
		t.Fatalf("expected interval cycle to 15s, got %s", got)
	}
	if m.watch.token == oldToken || cmd == nil {
		t.Fatal("expected interval change to invalidate the old timer and schedule a new one")
	}
}

func TestWatchTickUsesCommandGenerationAndDropsStaleTimer(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenCWAlarmList
	updated, _ := m.toggleWatch()
	m = updated.(Model)
	tick := watchTickMsg{target: m.watch.target, token: m.watch.token}
	before := m.commands.CurrentGen()

	updated, cmd := m.Update(tick)
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected a watch tick to schedule refresh and next timer")
	}
	if got := m.commands.CurrentGen(); got != before+1 {
		t.Fatalf("expected refresh to renew command generation, got %d want %d", got, before+1)
	}

	updated, cmd = m.Update(watchTickMsg{target: tick.target, token: tick.token - 1})
	if cmd != nil || updated.(Model).commands.CurrentGen() != before+1 {
		t.Fatal("expected a stale timer to be ignored")
	}
}

func TestWatchNavigationCancelsInFlightRefresh(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenCWAlarmList
	updated, _ := m.toggleWatch()
	m = updated.(Model)
	inFlight := m.commands.Current()

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.watch.enabled || m.screen != screenFeatureList {
		t.Fatalf("expected leaving the alarm list to stop watch, screen=%v watch=%+v", m.screen, m.watch)
	}
	if inFlight.Err() == nil {
		t.Fatal("expected leaving the watched screen to cancel in-flight work")
	}
}

func TestAlarmWatchRefreshPreservesSelection(t *testing.T) {
	m := alarmsTestModel()
	m.cwAlarms.HandleMessage(&m, cwAlarmsLoadedMsg{alarms: testAlarms()})
	m.cwAlarms.idx = 1
	updated, _ := m.toggleWatch()
	m = updated.(Model)

	refreshed := []awsservice.CloudWatchAlarm{
		{Name: "new-alarm", State: "ALARM"},
		testAlarms()[1],
		testAlarms()[0],
	}
	updated, _ = m.Update(watchRefreshMsg{target: screenCWAlarmList, msg: cwAlarmsLoadedMsg{alarms: refreshed}})
	m = updated.(Model)
	if m.screen != screenCWAlarmList || m.cwAlarms.filtered[m.cwAlarms.idx].Name != "healthy" {
		t.Fatalf("expected alarm watch refresh to preserve the selected alarm, idx=%d alarms=%+v", m.cwAlarms.idx, m.cwAlarms.filtered)
	}
}

func TestECSWatchRefreshPreservesDetailScroll(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.ecs.services = []awsservice.ECSService{{Name: "api", ARN: "arn:svc/api"}}
	m.ecs.selectedService = &m.ecs.services[0]
	m.ecs.HandleMessage(&m, ecsServiceDetailLoadedMsg{detail: &awsservice.ECSServiceDetail{Name: "api", ARN: "arn:svc/api", RunningCount: 1}})
	m.ecs.detailScroll = 4
	updated, _ := m.toggleWatch()
	m = updated.(Model)

	updated, _ = m.Update(watchRefreshMsg{
		target: screenECSServiceDetail,
		msg:    ecsServiceDetailLoadedMsg{detail: &awsservice.ECSServiceDetail{Name: "api", ARN: "arn:svc/api", RunningCount: 2}},
	})
	m = updated.(Model)
	if m.screen != screenECSServiceDetail || m.ecs.detailScroll != 4 || m.ecs.selectedDetail.RunningCount != 2 {
		t.Fatalf("expected rollout refresh in place, screen=%v scroll=%d detail=%+v", m.screen, m.ecs.detailScroll, m.ecs.selectedDetail)
	}
}

func TestECSWatchRefreshClampsDetailScrollAfterShrink(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 14
	m.ecs.services = []awsservice.ECSService{{Name: "api", ARN: "arn:svc/api"}}
	m.ecs.selectedService = &m.ecs.services[0]
	events := make([]awsservice.ECSServiceEvent, 20)
	for i := range events {
		events[i].Message = "deployment event"
	}
	m.ecs.HandleMessage(&m, ecsServiceDetailLoadedMsg{detail: &awsservice.ECSServiceDetail{
		Name: "api", ARN: "arn:svc/api", Events: events,
	}})
	visibleLines := max(m.height-9, 5)
	m.ecs.detailScroll = max(len(m.ecs.serviceDetailLines())-visibleLines, 0)
	previousScroll := m.ecs.detailScroll
	updated, _ := m.toggleWatch()
	m = updated.(Model)

	updated, _ = m.Update(watchRefreshMsg{
		target: screenECSServiceDetail,
		msg:    ecsServiceDetailLoadedMsg{detail: &awsservice.ECSServiceDetail{Name: "api", ARN: "arn:svc/api"}},
	})
	m = updated.(Model)
	want := max(len(m.ecs.serviceDetailLines())-visibleLines, 0)
	if previousScroll <= want {
		t.Fatalf("expected test detail to shrink below the previous scroll, before=%d afterMax=%d", previousScroll, want)
	}
	if m.ecs.detailScroll != want {
		t.Fatalf("expected rollout scroll to clamp after refresh, got %d want %d", m.ecs.detailScroll, want)
	}
}

func TestSQSWatchRefreshKeepsSelectedQueueDetail(t *testing.T) {
	m := sqsTestModel()
	m.sqs.HandleMessage(&m, sqsQueuesLoadedMsg{queues: testQueues()})
	selected := testQueues()[1]
	m.sqs.selected = &selected
	m.sqs.idx = 1
	m.screen = screenSQSQueueDetail
	updated, _ := m.toggleWatch()
	m = updated.(Model)

	queues := testQueues()
	queues[1].Depth = 42
	updated, _ = m.Update(watchRefreshMsg{target: screenSQSQueueDetail, msg: sqsQueuesLoadedMsg{queues: queues}})
	m = updated.(Model)
	if m.screen != screenSQSQueueDetail || m.sqs.selected == nil || m.sqs.selected.ARN != selected.ARN || m.sqs.selected.Depth != 42 {
		t.Fatalf("expected queue detail to refresh in place, screen=%v selected=%+v", m.screen, m.sqs.selected)
	}
}

func TestELBWatchRefreshKeepsSelectedTarget(t *testing.T) {
	m := elbTestModel()
	m.elb.HandleMessage(&m, elbLoadBalancersLoadedMsg{balancers: testBalancers()})
	selectedLB := testBalancers()[0]
	m.elb.selectedLB = &selectedLB
	m.elb.HandleMessage(&m, elbTargetGroupsLoadedMsg{loadBalancerARN: selectedLB.ARN, groups: testTargetGroups()})
	m.elb.updateGroupList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m.elb.targetIdx = 1
	updated, _ := m.toggleWatch()
	m = updated.(Model)

	groups := testTargetGroups()
	groups[0].HealthyCount = 2
	groups[0].Targets[0], groups[0].Targets[1] = groups[0].Targets[1], groups[0].Targets[0]
	updated, _ = m.Update(watchRefreshMsg{target: screenELBTargetList, msg: elbTargetGroupsLoadedMsg{
		loadBalancerARN: selectedLB.ARN,
		groups:          groups,
	}})
	m = updated.(Model)
	if m.screen != screenELBTargetList || m.elb.selectedGroup == nil || m.elb.selectedGroup.HealthyCount != 2 {
		t.Fatalf("expected target health detail to refresh in place, screen=%v group=%+v", m.screen, m.elb.selectedGroup)
	}
	if got := m.elb.selectedGroup.Targets[m.elb.targetIdx].ID; got != "i-ok" {
		t.Fatalf("expected selected target identity to be preserved, got %q", got)
	}
}
