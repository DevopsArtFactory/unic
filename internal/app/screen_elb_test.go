package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

func elbTestModel() Model {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	m.screen = screenLoading
	return m
}

func testBalancers() []awsservice.ELBLoadBalancer {
	return []awsservice.ELBLoadBalancer{
		{Name: "api-alb", ARN: "arn:lb/api", DNSName: "api.elb.amazonaws.com", Type: "application", Scheme: "internet-facing", State: "active", Region: "us-east-1"},
		{Name: "internal-nlb", ARN: "arn:lb/internal", Type: "network", Scheme: "internal", State: "active", Region: "us-east-1"},
	}
}

func testTargetGroups() []awsservice.ELBTargetGroupHealth {
	return []awsservice.ELBTargetGroupHealth{
		{
			Name: "broken-tg", ARN: "arn:tg/broken", Protocol: "HTTPS", Port: 443,
			HealthyCount: 1, UnhealthyCount: 1,
			Targets: []awsservice.ELBTargetHealth{
				{ID: "i-bad", Port: 443, State: "unhealthy", Reason: "Target.Timeout", Description: "Request timed out"},
				{ID: "i-ok", Port: 443, State: "healthy"},
			},
		},
		{Name: "healthy-tg", ARN: "arn:tg/healthy", Protocol: "HTTP", Port: 80, HealthyCount: 2},
	}
}

func TestELBLoadBalancersLoadedOpensList(t *testing.T) {
	m := elbTestModel()

	_, _, handled := m.elb.HandleMessage(&m, elbLoadBalancersLoadedMsg{balancers: testBalancers()})
	if !handled || m.screen != screenELBList {
		t.Fatalf("expected LB list screen, got %v", m.screen)
	}
	view, ok := m.elb.View(m)
	if !ok || !strings.Contains(view, "api-alb") || !strings.Contains(view, "internal-nlb") {
		t.Fatalf("expected both load balancers listed, got:\n%s", view)
	}
}

func TestELBEnterLoadsTargetGroupsForSelection(t *testing.T) {
	m := elbTestModel()
	m.elb.HandleMessage(&m, elbLoadBalancersLoadedMsg{balancers: testBalancers()})

	_, cmd := m.elb.updateLBList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter to start loading target groups")
	}
	if m.elb.selectedLB == nil || m.elb.selectedLB.Name != "api-alb" {
		t.Fatalf("expected first row selected, got %+v", m.elb.selectedLB)
	}
}

func TestELBTargetGroupsLoadedShowsHealthAndDrillDown(t *testing.T) {
	m := elbTestModel()
	m.elb.HandleMessage(&m, elbLoadBalancersLoadedMsg{balancers: testBalancers()})
	selected := testBalancers()[0]
	m.elb.selectedLB = &selected

	_, _, handled := m.elb.HandleMessage(&m, elbTargetGroupsLoadedMsg{loadBalancerARN: "arn:lb/api", groups: testTargetGroups()})
	if !handled || m.screen != screenELBTargetGroupList {
		t.Fatalf("expected target group list screen, got %v", m.screen)
	}
	view, _ := m.elb.View(m)
	if !strings.Contains(view, "broken-tg") || !strings.Contains(view, "unhealthy:1") {
		t.Fatalf("expected health counts in the group list, got:\n%s", view)
	}

	// Second keystroke: enter opens per-target health, unhealthy target first.
	m.elb.updateGroupList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenELBTargetList {
		t.Fatalf("expected target list screen, got %v", m.screen)
	}
	view, _ = m.elb.View(m)
	if !strings.Contains(view, "i-bad") || !strings.Contains(view, "Target.Timeout") || !strings.Contains(view, "Request timed out") {
		t.Fatalf("expected unhealthy target with reason code and description, got:\n%s", view)
	}
}

func TestELBTargetGroupsLoadedIgnoresStaleLB(t *testing.T) {
	m := elbTestModel()
	m.elb.HandleMessage(&m, elbLoadBalancersLoadedMsg{balancers: testBalancers()})
	selected := testBalancers()[0]
	m.elb.selectedLB = &selected
	m.screen = screenELBList

	m.elb.HandleMessage(&m, elbTargetGroupsLoadedMsg{loadBalancerARN: "arn:lb/other", groups: testTargetGroups()})
	if m.screen != screenELBList || len(m.elb.groups) != 0 {
		t.Fatalf("expected stale target group load to be dropped, screen=%v groups=%d", m.screen, len(m.elb.groups))
	}
}

func TestAlarmLoadBalancerDimensionMapsToELBBrowser(t *testing.T) {
	alarm := awsservice.CloudWatchAlarm{
		Dimensions: []awsservice.CloudWatchAlarmDimension{
			{Name: "LoadBalancer", Value: "app/api-alb/50dc6c495c0c9188"},
			{Name: "TargetGroup", Value: "targetgroup/broken-tg/73e2d6bc24d8a067"},
		},
	}
	feature, target, value, ok := alarmRelatedResource(alarm)
	if !ok || feature != domain.FeatureELBBrowser || target != filterELBs || value != "api-alb" {
		t.Fatalf("expected LoadBalancer dimension to map to the ELB browser, got %v %v %q %v", feature, target, value, ok)
	}
}

func TestAlarmTargetGroupOnlyDimensionStillMapsToELBBrowser(t *testing.T) {
	alarm := awsservice.CloudWatchAlarm{
		Dimensions: []awsservice.CloudWatchAlarmDimension{
			{Name: "TargetGroup", Value: "targetgroup/broken-tg/73e2d6bc24d8a067"},
		},
	}
	feature, target, value, ok := alarmRelatedResource(alarm)
	if !ok || feature != domain.FeatureELBBrowser || target != filterELBs || value != "" {
		t.Fatalf("expected TargetGroup-only alarm to land in the ELB browser unfiltered, got %v %v %q %v", feature, target, value, ok)
	}
}
