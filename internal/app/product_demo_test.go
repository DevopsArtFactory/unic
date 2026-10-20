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
// messages. Never execute returned commands or Init: those can contact real AWS.
func TestProductDemo(t *testing.T) {
	out := os.Getenv("UNIC_DEMO_FRAMES")
	if out == "" {
		t.Fatal("set UNIC_DEMO_FRAMES to an output JSON path")
	}
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
	m := New(&config.Config{ContextName: "demo-readonly", Region: "us-east-1", AuthType: config.AuthTypeCredential}, filepath.Join(t.TempDir(), "config.yaml"), "demo")
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
	m.ecs.selectedCluster = &awsservice.ECSCluster{Name: "demo-platform"}
	services := []awsservice.ECSService{
		{Name: "catalog-api", Status: "ACTIVE", LaunchType: "FARGATE", RunningCount: 3, DesiredCount: 3},
		{Name: "checkout-api", Status: "ACTIVE", LaunchType: "FARGATE", RunningCount: 1, DesiredCount: 3, PendingCount: 2},
		{Name: "payments-worker", Status: "ACTIVE", LaunchType: "FARGATE", RunningCount: 2, DesiredCount: 2},
		{Name: "search-api", Status: "ACTIVE", LaunchType: "FARGATE", RunningCount: 3, DesiredCount: 3},
	}
	m.ecs.HandleMessage(&m, ecsServicesLoadedMsg{services: services})
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
	capture(4, "01 / FIND THE SERVICE", "A deployment is failing. Start with the service, not another console tab.", "ECS > cluster > services", "checkout-api")
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
	capture(2.8, "01 / FIND THE SERVICE", "One running task. Three desired. Open the rollout to inspect the evidence.", "Enter", "1/4 services")
	key(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenLoading || m.ecs.selectedService.Name != "checkout-api" {
		t.Fatal("Enter did not request the selected service detail")
	}
	at := time.Date(2026, 10, 20, 9, 41, 0, 0, time.UTC)
	detail := &awsservice.ECSServiceDetail{
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
	capture(9, "02 / INSPECT THE ROLLOUT", "ACTIVE is the service status. FAILED is the deployment state that matters here.", "Read rollout state + task counts", "FAILED")
	capture(9, "03 / READ THE EVIDENCE", "The circuit breaker reports tasks failing to start. Check task diagnostics next.", "Image + timestamped events", "was unable to consistently start")
	capture(4, "YOUR NEXT DEPLOYMENT", "Try it: unic --profile your-profile --region your-region", "ECS > ECS Browser & Exec > cluster > service", "Recent Service Events")
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
