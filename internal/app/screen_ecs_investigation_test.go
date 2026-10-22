package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func TestECSInvestigationLifecycleAndOverlay(t *testing.T) {
	original := newECSTargetHealthRepository
	t.Cleanup(func() { newECSTargetHealthRepository = original })
	calls := 0
	newECSTargetHealthRepository = func(ctx context.Context, cfg *config.Config) (*awsservice.AwsRepository, error) {
		calls++
		if _, ok := ctx.Deadline(); !ok || cfg.Profile != "pilot" || cfg.Region != "us-west-2" {
			t.Fatal("lost deadline or scope")
		}
		return nil, errors.New("AccessDenied")
	}
	m := New(testConfig(), "", "dev")
	m.cfg.ContextName = "dev"
	m.cfg.Profile = "pilot"
	m.cfg.Region = "us-west-2"
	m.screen = screenECSServiceDetail
	m.width = 80
	m.height = 24
	m.ecs.selectedCluster = &awsservice.ECSCluster{ARN: "cluster"}
	m.ecs.selectedDetail = &awsservice.ECSServiceDetail{ARN: "service", Name: "checkout"}
	m.ecs.detailScroll = 3
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m = updated.(Model)
	if cmd == nil || m.screen != screenECSInvestigation || !m.ecs.investigationLoading || !strings.Contains(stripANSI(m.View()), "Esc cancels") {
		t.Fatal("missing investigation entry/cancel guidance")
	}
	generation := m.commands.CurrentGen()
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	m = updated.(Model)
	updated, _ = m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{{Name: "dev", Current: true}}})
	m = updated.(Model)
	updated, other := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if other != nil || m.commands.CurrentGen() != generation || m.screen != screenSettings {
		t.Fatal("overlay cancelled pending investigation")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if calls != 1 || m.ecs.investigationLoading {
		t.Fatal("collection did not finish under overlay")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if !strings.Contains(strings.Join(m.ecs.investigationLines(), " "), "AccessDenied") {
		t.Fatal("failure hidden")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenECSServiceDetail || m.ecs.detailScroll != 3 {
		t.Fatal("return lost rollout scroll")
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if cmd() != nil || calls != 1 {
		t.Fatal("cancelled queued lookup ran")
	}
	updated, _ = m.Update(ecsInvestigationLoadedMsg{awsservice.ECSInvestigation{Warnings: []string{"stale"}}})
	m = updated.(Model)
	if strings.Contains(strings.Join(m.ecs.investigation.Warnings, " "), "stale") {
		t.Fatal("stale result applied")
	}
	resetECSContextState(&m)
	if m.ecs.selectedDetail != nil {
		t.Fatal("identity switch retained resources")
	}
}

func TestECSInvestigationEvidenceAndWrapping(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 60
	m.height = 20
	m.screen = screenECSInvestigation
	m.ecs.selectedDetail = &awsservice.ECSServiceDetail{Name: "checkout"}
	m.ecs.investigationLoading = true
	result := awsservice.ECSInvestigation{ObservedAt: time.Now(), Warnings: []string{"partial AccessDenied"}, Logs: []awsservice.ECSInvestigationLog{{Group: "group", Events: []awsservice.LogEvent{{Message: strings.Repeat("evidence ", 50) + "FINAL"}}}}}
	updated, _ := m.Update(ecsInvestigationLoadedMsg{result})
	m = updated.(Model)
	views := ""
	for range 40 {
		views += stripANSI(m.View())
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = updated.(Model)
	}
	views = strings.Join(strings.Fields(views), " ")
	for _, want := range []string{"AccessDenied", "FINAL"} {
		if !strings.Contains(views, want) {
			t.Fatalf("unreachable %q", want)
		}
	}
	for _, want := range []string{"do not prove", "root cause"} {
		if !strings.Contains(strings.Join(m.ecs.investigationLines(), " "), want) {
			t.Fatalf("missing evidence limitation %q", want)
		}
	}
}
