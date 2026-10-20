package app

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

var newECSTargetHealthRepository = awsservice.NewAwsRepository

const ecsTargetHealthHelp = "↑/↓: scroll • pgup/pgdn: page • r: refresh • esc: rollout • H: home"

type ecsTargetHealthLoadedMsg struct {
	results  []awsservice.ECSServiceTargetHealth
	err      error
	observed time.Time
}

func (em *ecsModel) startTargetHealth(m *Model) (tea.Model, tea.Cmd) {
	m.stopWatch()
	em.healthLoading = true
	em.healthResults, em.healthError = nil, nil
	if m.screen != screenECSTargetHealth {
		em.detailScroll = 0
	}
	m.screen = screenECSTargetHealth
	cmd := em.loadTargetHealth(*m)
	if m.commands != nil {
		cmd = m.commands.BindCmd(m.commands.Renew(), cmd)
	}
	return *m, cmd
}

func (em *ecsModel) updateTargetHealth(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		if m.commands != nil {
			m.commands.CancelAll()
		}
		em.healthLoading = false
		em.detailScroll = em.rolloutScroll
		m.screen = screenECSServiceDetail
	case "r":
		return em.startTargetHealth(m)
	case "up", "k", "down", "j", "pgup", "pgdown":
		if !em.healthLoading {
			em.scrollServiceDetail(*m, msg.String(), ecsTargetHealthHelp)
		}
	}
	return *m, nil
}

func (em ecsModel) loadTargetHealth(m Model) tea.Cmd {
	if em.selectedCluster == nil || em.selectedDetail == nil || m.cfg == nil {
		return func() tea.Msg {
			return ecsTargetHealthLoadedMsg{err: fmt.Errorf("no cluster, service, or AWS configuration selected"), observed: time.Now().UTC()}
		}
	}
	cluster, service := em.selectedCluster.ARN, em.selectedDetail.ARN
	cfg := *m.cfg
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.commandContext(), ecsAPITimeout)
		defer cancel()
		repo, err := newECSTargetHealthRepository(ctx, &cfg)
		if err != nil {
			return ecsTargetHealthLoadedMsg{err: err, observed: time.Now().UTC()}
		}
		results, err := repo.ListECSServiceTargetHealth(ctx, cluster, service)
		return ecsTargetHealthLoadedMsg{results: results, err: err, observed: time.Now().UTC()}
	}
}

func (em ecsModel) targetHealthLines() []string {
	if em.healthLoading {
		return []string{dimStyle.Render("Loading associated target health... Esc cancels and returns to rollout.")}
	}
	observed := "-"
	if !em.healthObserved.IsZero() {
		observed = em.healthObserved.Format(time.RFC3339)
	}
	lines := []string{
		renderDetailLine("Observed", observed),
		dimStyle.Render("Source: ECS service associations + ELBv2 target health."),
		dimStyle.Render("Group-wide observations; shared groups can include other services."),
		dimStyle.Render("Unhealthy targets alone do not establish the cause of a rollout failure."),
		"",
	}
	if em.healthError != nil {
		lines = append(lines, errorStyle.Render("Unavailable: "+em.healthError.Error()))
	}
	if len(em.healthResults) == 0 && em.healthError == nil {
		lines = append(lines, dimStyle.Render("No ELB target-group associations on this ECS service."))
	}
	for _, result := range em.healthResults {
		group := result.Group
		lines = append(lines, titleStyle.Render(zeroFallback(group.ARN, "Unsupported association")))
		if result.Warning != "" {
			lines = append(lines, warningStyle.Render("Unavailable: "+result.Warning), "")
			continue
		}
		lines = append(lines, fmt.Sprintf("  healthy:%d unhealthy:%d other:%d", group.HealthyCount, group.UnhealthyCount, group.OtherCount))
		if len(group.Targets) == 0 {
			lines = append(lines, dimStyle.Render("  No registered targets"))
		}
		for _, target := range group.Targets {
			lines = append(lines, "  "+target.DisplayTitle())
			if target.Description != "" {
				lines = append(lines, dimStyle.Render("    "+target.Description))
			}
		}
		lines = append(lines, "")
	}
	return lines
}

// A new AWS identity must not retain a service ARN or a return target from the old one.
func resetECSContextState(m *Model) {
	m.ecs = newECSModel()
	for _, previous := range []*screen{&m.ctxPrevScreen, &m.settingsPrevScreen, &m.views.prevScreen, &m.palette.prevScreen, &m.regionPrevScreen} {
		switch *previous {
		case screenECSClusterList, screenECSServiceList, screenECSServiceDetail, screenECSTargetHealth, screenECSTaskList, screenECSContainerList:
			*previous = screenFeatureList
		}
	}
}
