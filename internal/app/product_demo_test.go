//go:build productdemo

package app

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

// TestProductDemo captures production views and key handling with synthetic API
// messages. Only saveContext is executed, against t.TempDir; Init, authentication,
// and AWS commands are never executed.
func TestProductDemo(t *testing.T) {
	out := os.Getenv("UNIC_DEMO_FRAMES")
	if out == "" {
		t.Fatal("set UNIC_DEMO_FRAMES to an output JSON path")
	}
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
	for _, name := range []string{"UNIC_CONTEXT", "AWS_PROFILE", "AWS_DEFAULT_PROFILE"} {
		t.Setenv(name, "")
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	m := New(&config.Config{Profile: "demo-profile", Region: "us-east-1", AuthType: config.AuthTypeCredential}, configPath, "demo")
	m.width, m.height = 120, 36
	// Verify the documented first-use route even without saved unic contexts.
	entry := New(&config.Config{Profile: "demo-profile", Region: "us-east-1"}, "", "demo")
	updated, _ := entry.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	entry = updated.(Model)
	for _, r := range "ECS Browser" {
		updated, _ = entry.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		entry = updated.(Model)
	}
	updated, _ = entry.Update(tea.KeyMsg{Type: tea.KeyEnter})
	entry = updated.(Model)
	if entry.screen != screenLoading || string(entry.activeService) != "ECS" || entry.cfg.Profile != "demo-profile" {
		t.Fatal("documented palette entry did not retain the CLI profile")
	}
	entry.palette.stopIndex()
	type frame struct {
		Seconds float64 `json:"seconds"`
		Chapter string  `json:"chapter"`
		Caption string  `json:"caption"`
		Keys    string  `json:"keys"`
		ANSI    string  `json:"ansi"`
	}
	var frames []frame
	capture := func(seconds float64, chapter, caption, keys, expected string) {
		t.Helper()
		view := m.View()
		plain := ansiEscapePattern.ReplaceAllString(view, "")
		if !strings.Contains(plain, expected) {
			t.Fatalf("%s: missing %q in view:\n%s", chapter, expected, plain)
		}
		if strings.Contains(plain, "\n  ...\n") {
			t.Fatalf("%s: capture clips the production view", chapter)
		}
		if lipgloss.Width(view) > m.width || lipgloss.Height(view) > m.height {
			t.Fatalf("%s: view exceeds %dx%d: %dx%d", chapter, m.width, m.height, lipgloss.Width(view), lipgloss.Height(view))
		}
		frames = append(frames, frame{seconds, chapter, caption, keys, view})
	}
	key := func(msg tea.KeyMsg) {
		t.Helper()
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	capture(2, "00 / ADD YOUR CONTEXT", "Start with an existing AWS profile. Save a reusable unic context.", "a: add your first context", "No contexts defined")
	key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.screen != screenContextAdd {
		t.Fatal("a did not open Add Context")
	}
	key(tea.KeyMsg{Type: tea.KeyDown})
	if authTypes[m.addAuthIdx] != "credential" {
		t.Fatal("expected profile-backed credential auth type")
	}
	capture(2, "00 / ADD YOUR CONTEXT", "Choose credential to reuse a named AWS profile; no access keys are entered here.", "Down > credential > Enter", "credential")
	key(tea.KeyMsg{Type: tea.KeyEnter})
	values := map[string]string{"name": "demo-readonly", "region": "us-east-1", "profile": "demo-profile"}
	for _, field := range fieldsByAuthType["credential"] {
		value := values[field.key]
		if value != "" {
			key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)})
			capture(2, "00 / ADD YOUR CONTEXT", "Name the context, choose its region, and reference your existing AWS profile.", field.label+": "+value+" > Enter (skip optional fields)", value)
		}
		key(tea.KeyMsg{Type: tea.KeyEnter})
	}
	if m.addStep != -1 {
		t.Fatal("context wizard did not reach the save confirmation")
	}
	capture(2, "00 / ADD YOUR CONTEXT", "Review the profile and region before saving. Optional role chaining is left blank.", "Enter: save context", "Save this context?")
	// This is the one executed command: the inspected local-file save, never auth.
	updated, save := m.updateContextAdd(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if save == nil {
		t.Fatal("Enter did not request the local context save")
	}
	saved, ok := save().(contextsLoadedMsg)
	if !ok || len(saved.contexts) != 1 || saved.contexts[0].Name != "demo-readonly" || saved.contexts[0].Profile != "demo-profile" || saved.contexts[0].Region != "us-east-1" {
		t.Fatalf("context was not persisted correctly: %+v", saved)
	}
	updated, _ = m.Update(saved)
	m = updated.(Model)
	capture(2, "00 / ADD YOUR CONTEXT", "Saved. Select the context, then open ECS Browser & Exec and choose a cluster.", "Enter: select context (AWS authentication is not run in this demo)", "demo-readonly")
	key(tea.KeyMsg{Type: tea.KeyEnter})
	if m.pendingContextName != "demo-readonly" {
		t.Fatal("saved context could not be selected")
	}
	// Represent the authenticated state with a synthetic message; discard AWS commands.
	updated, _ = m.Update(contextSwitchedMsg{cfg: &config.Config{ContextName: "demo-readonly", Profile: "demo-profile", Region: "us-east-1", AuthType: config.AuthTypeCredential}})
	m = updated.(Model)
	m.ecs.selectedCluster = &awsservice.ECSCluster{Name: "demo-platform", ARN: "arn:aws:ecs:us-east-1:000000000000:cluster/demo-platform"}
	services := []awsservice.ECSService{
		{Name: "catalog-api", Status: "ACTIVE", LaunchType: "FARGATE", RunningCount: 3, DesiredCount: 3},
		{Name: "checkout-api", Status: "ACTIVE", LaunchType: "FARGATE", RunningCount: 1, DesiredCount: 3, PendingCount: 2},
		{Name: "payments-worker", Status: "ACTIVE", LaunchType: "FARGATE", RunningCount: 2, DesiredCount: 2},
		{Name: "search-api", Status: "ACTIVE", LaunchType: "FARGATE", RunningCount: 3, DesiredCount: 3},
	}
	m.ecs.HandleMessage(&m, ecsServicesLoadedMsg{services: services})
	capture(3, "01 / FIND THE SERVICE", "With access configured: investigate a deployment in your ECS cluster.", "ECS > ECS Browser & Exec > cluster > services", "checkout-api")
	key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	query := "checkout"
	for i, r := range query {
		key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		capture(0.15, "01 / FIND THE SERVICE", "Filter the cluster to the service you are investigating.", "/  "+query[:i+1], "checkout-api")
	}
	key(tea.KeyMsg{Type: tea.KeyEnter}) // finish editing the filter
	if len(m.ecs.filteredServices) != 1 || m.ecs.filteredServices[0].Name != "checkout-api" {
		t.Fatal("filter did not isolate checkout-api")
	}
	capture(1.8, "01 / FIND THE SERVICE", "One running task. Three desired. Open the rollout to inspect the evidence.", "Enter", "1/4 services")
	key(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenLoading || m.ecs.selectedService.Name != "checkout-api" {
		t.Fatal("Enter did not request the selected service detail")
	}
	at := time.Date(2026, 10, 20, 9, 41, 0, 0, time.UTC)
	detail := &awsservice.ECSServiceDetail{
		ARN:  "arn:aws:ecs:us-east-1:000000000000:service/demo-platform/checkout-api",
		Name: "checkout-api", Status: "ACTIVE", LaunchType: "FARGATE",
		RunningCount: 1, DesiredCount: 3, PendingCount: 2,
		DeploymentControllerType: "ECS", SchedulingStrategy: "REPLICA",
		TaskDefinitionFamily: "checkout-api", TaskDefinitionRevision: 42,
		PlatformVersion: "1.4.0", NetworkMode: "awsvpc", RequiresCompatibilities: []string{"FARGATE"},
		Deployments: []awsservice.ECSDeployment{{
			Status: "PRIMARY", RolloutState: "FAILED", TaskDefinition: "checkout-api:42",
			RunningCount: 1, DesiredCount: 3, PendingCount: 2, FailedTasks: 4,
			RolloutStateReason: "ECS deployment circuit breaker: tasks failed to start.", UpdatedAt: at,
		}},
		ContainerImages: []awsservice.ECSContainerImage{{Name: "app", Image: "example.invalid/checkout-api:release-42"}},
		Events: []awsservice.ECSServiceEvent{
			{CreatedAt: at, Message: "(service checkout-api) deployment failed: tasks failed to start."},
			{CreatedAt: at.Add(-time.Minute), Message: "(service checkout-api) was unable to consistently start tasks successfully."},
			{CreatedAt: at.Add(-2 * time.Minute), Message: "(service checkout-api) has started 2 tasks."},
		},
	}
	m.ecs.HandleMessage(&m, ecsServiceDetailLoadedMsg{detail: detail})
	capture(3, "02 / INSPECT THE ROLLOUT", "ACTIVE is the service status. FAILED is the deployment state that matters here.", "Read rollout state + task counts", "FAILED")
	key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")}) // Discard the live collection command.
	if m.screen != screenECSInvestigation || !m.ecs.investigationLoading {
		t.Fatal("i did not open the investigation")
	}
	updated, _ = m.Update(ecsInvestigationLoadedMsg{result: awsservice.ECSInvestigation{
		ObservedAt: at, Rollout: detail, ChangesCollected: true,
		Health:       []awsservice.ECSServiceTargetHealth{{Group: awsservice.ELBTargetGroupHealth{ARN: "synthetic-target-group", UnhealthyCount: 1, Targets: []awsservice.ELBTargetHealth{{ID: "192.0.2.10", Port: 8080, State: "unhealthy", Reason: "Target.Timeout"}}}}},
		StoppedTasks: []awsservice.ECSStoppedTask{{ARN: "synthetic-stopped-task", StoppedAt: at, StopCode: "TaskFailedToStart", Reason: "CannotPullContainerError: image manifest not found for release-42."}},
		Logs:         []awsservice.ECSInvestigationLog{{Group: "/synthetic/checkout", Warning: "No application log sample; tasks failed before application startup."}},
		Changes:      []awsservice.CloudTrailEvent{{Time: at.Add(-3 * time.Minute), Name: "UpdateService", Source: "ecs.amazonaws.com"}},
		Warnings:     []string{"Synthetic scenario: group-wide target health does not establish the image-pull failure cause."},
	}})
	m = updated.(Model)
	scrollTo := func(expected string) {
		t.Helper()
		for range 12 {
			if strings.Contains(ansiEscapePattern.ReplaceAllString(m.View(), ""), expected) {
				return
			}
			key(tea.KeyMsg{Type: tea.KeyPgDown})
		}
		t.Fatalf("evidence %q not reachable", expected)
	}
	scrollTo("healthy:0 unhealthy:1 other:0")
	capture(3, "03 / CONNECT THE EVIDENCE", "One key follows target health and stopped tasks. Shared-group observations are not a root cause.", "i: investigate > PgDn: related evidence", "healthy:0 unhealthy:1 other:0")
	scrollTo("UpdateService")
	capture(2, "04 / RETAIN THE UNKNOWNS", "Read the task failure, unavailable logs, and nearby change. Correlation is not causation.", "PgDn: logs, changes, limits, and next checks", "UpdateService")
	key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	if m.screen != screenECSSummary || m.ecs.summaryIdentifiers {
		t.Fatal("summary did not default to redacted preview")
	}
	m.ecs.summaryPath = "/synthetic/unic-investigation-preview.md"
	if strings.Contains(m.ecs.summaryMarkdown(m), "checkout-api") || strings.Contains(m.ecs.summaryMarkdown(m), "000000000000") {
		t.Fatal("default export leaked fixture identifiers")
	}
	capture(2, "05 / HAND OFF THE FINDINGS", "Preview a redacted Markdown handoff. Copy or save locally only when you choose.", "m: preview > c: copy / s: new file (not executed in this recording)", "Identifiers: OFF")
	var duration float64
	for _, f := range frames {
		duration += f.Seconds
	}
	if math.Abs(duration-30) >= 0.01 {
		t.Fatalf("expected 30 seconds, got %f", duration)
	}
	data, err := json.MarshalIndent(frames, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, data, 0600); err != nil {
		t.Fatal(err)
	}
}
