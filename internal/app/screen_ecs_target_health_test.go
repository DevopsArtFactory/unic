package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

type ecsHealthClient struct {
	awsservice.ECSClientAPI
	t *testing.T
}

func (c ecsHealthClient) DescribeServices(ctx context.Context, in *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	if _, ok := ctx.Deadline(); !ok {
		c.t.Fatal("lookup must have a deadline")
	}
	if awssdk.ToString(in.Cluster) != "cluster" || len(in.Services) != 1 || in.Services[0] != "service" {
		c.t.Fatalf("wrong scope: %+v", in)
	}
	return &ecs.DescribeServicesOutput{Services: []ecstypes.Service{{ServiceArn: awssdk.String("service")}}}, nil
}

func TestECSTargetHealthNavigationAndConfig(t *testing.T) {
	original := newECSTargetHealthRepository
	t.Cleanup(func() { newECSTargetHealthRepository = original })
	calls := 0
	newECSTargetHealthRepository = func(ctx context.Context, cfg *config.Config) (*awsservice.AwsRepository, error) {
		calls++
		if cfg.ContextName != "dev" || cfg.Region != "us-west-2" || cfg.Profile != "pilot" {
			t.Fatalf("lost context: %+v", cfg)
		}
		return &awsservice.AwsRepository{ECSClient: ecsHealthClient{t: t}, Region: cfg.Region}, nil
	}
	m := New(testConfig(), "", "dev")
	m.cfg.ContextName = "dev"
	m.cfg.Region = "us-west-2"
	m.cfg.Profile = "pilot"
	m.screen = screenECSServiceDetail
	m.width, m.height = 80, 24
	m.ecs.selectedCluster = &awsservice.ECSCluster{ARN: "cluster"}
	m.ecs.selectedDetail = &awsservice.ECSServiceDetail{ARN: "service", Name: "checkout"}
	m.ecs.detailScroll = 3
	m.watch.enabled = true
	m.watch.target = screenECSServiceDetail
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	m = updated.(Model)
	if cmd == nil || m.screen != screenECSTargetHealth || !m.ecs.healthLoading || m.watch.enabled {
		t.Fatalf("bad loading transition: %+v", m.ecs)
	}
	if !strings.Contains(stripANSI(m.View()), "Esc cancels") {
		t.Fatal("loading must explain cancellation")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if calls != 1 || m.ecs.healthLoading || m.ecs.healthError != nil {
		t.Fatalf("lookup not applied: %+v", m.ecs)
	}
	if !strings.Contains(stripANSI(m.View()), "No ELB target-group associations") {
		t.Fatal("missing association not explained")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenECSServiceDetail || m.ecs.detailScroll != 3 || m.cfg.Region != "us-west-2" || m.ecs.selectedDetail.ARN != "service" {
		t.Fatal("return lost rollout or context")
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if msg := cmd(); msg != nil || calls != 1 {
		t.Fatal("cancelled queued command still queried AWS")
	}
}

func TestECSTargetHealthEvidenceAndWrapping(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenECSTargetHealth
	m.width, m.height = 60, 20
	m.ecs.selectedDetail = &awsservice.ECSServiceDetail{Name: "checkout"}
	m.ecs.healthLoading = true
	updated, _ := m.Update(ecsTargetHealthLoadedMsg{
		observed: time.Date(2026, 10, 20, 1, 2, 3, 0, time.UTC),
		err:      errors.New("deadline exceeded after partial evidence"),
		results: []awsservice.ECSServiceTargetHealth{
			{Group: awsservice.ELBTargetGroupHealth{ARN: "good", UnhealthyCount: 1, Targets: []awsservice.ELBTargetHealth{{ID: "task-ip", Port: 8080, State: "unhealthy", Reason: "Target.Timeout", Description: strings.Repeat("long evidence ", 12) + "FINAL"}}}},
			{Group: awsservice.ELBTargetGroupHealth{ARN: "empty"}},
			{Group: awsservice.ELBTargetGroupHealth{ARN: "denied"}, Warning: "AccessDenied"},
		},
	})
	m = updated.(Model)
	all := stripANSI(strings.Join(m.ecs.targetHealthLines(), "\n"))
	for _, want := range []string{"2026-10-20T01:02:03Z", "deadline exceeded", "Target.Timeout", "AccessDenied", "No registered targets", "other services", "do not establish"} {
		if !strings.Contains(all, want) {
			t.Fatalf("missing %q in %s", want, all)
		}
	}
	views := ""
	for range 20 {
		views += stripANSI(m.View())
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = updated.(Model)
	}
	if !strings.Contains(views, "FINAL") || !strings.Contains(strings.Join(strings.Fields(stripANSI(m.View())), " "), "esc: rollout") {
		t.Fatalf("wrapped evidence/footer unreachable: final=%v view=%q", strings.Contains(views, "FINAL"), stripANSI(m.View()))
	}
}

func TestECSTargetHealthClearedAcrossContextAndRegion(t *testing.T) {
	for _, regionSwitch := range []bool{false, true} {
		t.Run(map[bool]string{false: "context", true: "region"}[regionSwitch], func(t *testing.T) {
			m := New(testConfig(), "", "dev")
			m.cfg.ContextName = "dev"
			m.screen = screenContextPicker
			m.ctxPrevScreen = screenECSTargetHealth
			m.ecs.selectedDetail = &awsservice.ECSServiceDetail{ARN: "old-service"}
			m.ecs.healthLoading = true
			var msg tea.Msg
			if regionSwitch {
				msg = regionSwitchedMsg{region: "eu-west-1"}
			} else {
				cfg := *m.cfg
				cfg.ContextName = "other"
				msg = contextSwitchedMsg{cfg: &cfg}
			}
			updated, _ := m.Update(msg)
			m = updated.(Model)
			if m.ecs.selectedDetail != nil || m.ecs.healthLoading || m.screen == screenECSTargetHealth {
				t.Fatal("stale ECS identity retained")
			}
			updated, _ = m.Update(ecsTargetHealthLoadedMsg{err: errors.New("stale")})
			m = updated.(Model)
			if m.ecs.healthError != nil {
				t.Fatal("late old-context evidence applied")
			}
		})
	}
}

func TestECSTargetHealthCancelInFlight(t *testing.T) {
	original := newECSTargetHealthRepository
	t.Cleanup(func() { newECSTargetHealthRepository = original })
	started := make(chan context.Context, 1)
	newECSTargetHealthRepository = func(ctx context.Context, _ *config.Config) (*awsservice.AwsRepository, error) {
		started <- ctx
		<-ctx.Done()
		return nil, ctx.Err()
	}
	m := New(testConfig(), "", "dev")
	m.screen = screenECSServiceDetail
	m.ecs.selectedCluster = &awsservice.ECSCluster{ARN: "cluster"}
	m.ecs.selectedDetail = &awsservice.ECSServiceDetail{ARN: "service"}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	m = updated.(Model)
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var ctx context.Context
	select {
	case ctx = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("lookup did not start")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("escape did not cancel active request")
	}
	select {
	case msg := <-done:
		updated, _ = m.Update(msg)
		m = updated.(Model)
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled lookup did not finish")
	}
	if m.screen != screenECSServiceDetail || m.ecs.healthError != nil {
		t.Fatal("late cancellation result changed the rollout")
	}
}
