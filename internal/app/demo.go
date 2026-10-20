package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

// demoModel uses the live rollout renderer, but never the live application's
// initialization or event dispatcher: even global shortcuts cannot reach AWS.
type demoModel struct {
	shell Model
	help  bool
}

// NewDemo starts a fixed, synthetic incident without reading config or credentials.
func NewDemo() tea.Model {
	observed := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	detail := &awsservice.ECSServiceDetail{
		Name: "checkout-demo", Status: "ACTIVE", LaunchType: "FARGATE",
		DesiredCount: 2, RunningCount: 0, DeploymentControllerType: "ECS",
		SchedulingStrategy: "REPLICA", NetworkMode: "awsvpc",
		TaskDefinitionFamily: "checkout-demo", TaskDefinitionRevision: 42,
		RequiresCompatibilities: []string{"FARGATE"},
		ContainerImages:         []awsservice.ECSContainerImage{{Name: "checkout", Image: "example.invalid/checkout:demo-42"}},
		Deployments: []awsservice.ECSDeployment{{
			ID: "demo-deployment", Status: "PRIMARY", RolloutState: "FAILED",
			DesiredCount: 2, FailedTasks: 3, UpdatedAt: observed,
			RolloutStateReason: "Deployment circuit breaker: tasks failed to start.",
		}},
		Events: []awsservice.ECSServiceEvent{
			{CreatedAt: observed, Message: "Deployment failed: deployment circuit breaker was triggered."},
			{CreatedAt: observed.Add(-time.Minute), Message: "Unable to start tasks: CannotPullContainerError: image manifest not found for demo-42."},
		},
	}
	return demoModel{shell: Model{
		cfg:    &config.Config{ContextName: "DEMO - synthetic/offline", Region: "us-east-1"},
		screen: screenECSServiceList, width: 100, height: 30,
		ecs: ecsModel{selectedDetail: detail},
	}}
}

func (d demoModel) Init() tea.Cmd { return nil }

func (d demoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.shell.width, d.shell.height = msg.Width, msg.Height
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" || key == "q" {
			d.shell.quitting = true
			return d, tea.Quit
		}
		if key == "?" {
			d.help = !d.help
			return d, nil
		}
		if d.help {
			if key == "esc" || key == "enter" {
				d.help = false
			}
			return d, nil
		}
		switch key {
		case "H", "esc":
			d.shell.screen = screenECSServiceList
		case "enter":
			if d.shell.screen == screenECSServiceList {
				d.shell.screen = screenECSServiceDetail
				d.shell.ecs.detailScroll = 0
			}
		case "r":
			d.shell.ecs.detailScroll = 0
		case "up", "k", "down", "j", "pgup", "pgdown":
			if d.shell.screen == screenECSServiceDetail {
				// Only scrolling keys reach this handler; loading and exec keys
				// are deliberately excluded from the offline event loop.
				d.shell.ecs.updateServiceDetail(&d.shell, msg)
			}
		}
	}
	return d, nil
}

func (d demoModel) View() string {
	if d.shell.quitting {
		return ""
	}
	m := d.shell
	if d.help {
		return m.fitToHeight(m.renderStatusBar() + titleStyle.Render("Offline ECS incident demo") + "\n\n" +
			"All resources and timestamps are synthetic. No AWS calls or configuration changes.\n\n" +
			"1. Press Enter on checkout-demo to open its rollout.\n" +
			"2. Scroll to Deployments: compare desired/running tasks and FAILED state.\n" +
			"3. Read Recent Service Events for the image-pull failure evidence.\n" +
			"4. Next check: verify the image tag in your registry before deploying again.\n\n" +
			"This sample does not query logs, target health, or recent changes.\n" +
			"Live AWS browsing requires leaving demo and running unic separately.\n\n" +
			m.renderHelpBar("?: close help • esc/enter: back • q/ctrl+c: quit"))
	}
	if m.screen == screenECSServiceDetail {
		return m.fitToHeight(m.ecs.viewServiceDetailWithHelp(m,
			"↑/↓/j/k: scroll • pgup/pgdn: page • r: restart • esc/H: service • ?: guide • q: quit"))
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Offline ECS incident demo"))
	b.WriteString("\n\nInspect a failed deployment: why are there no running tasks?\n\n")
	b.WriteString(m.renderListPanel(selectedStyle.Render("> checkout-demo   FARGATE   running:0 desired:2") +
		"\n\n" + dimStyle.Render("Synthetic fixture • no AWS access • no config changes")))
	b.WriteString("\n\nEnter: inspect rollout → scroll to deployments and recent events.\n\n")
	b.WriteString(m.renderHelpBar("enter: rollout • ?: guide • q/ctrl+c: quit"))
	return m.fitToHeight(b.String())
}
