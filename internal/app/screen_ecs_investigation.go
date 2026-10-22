package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	awsservice "unic/internal/services/aws"
)

const ecsInvestigationHelp = "↑/↓: scroll • pgup/pgdn: page • r: refresh • esc/q: rollout • H: home"

type ecsInvestigationLoadedMsg struct{ result awsservice.ECSInvestigation }

func (em *ecsModel) startInvestigation(m *Model) (tea.Model, tea.Cmd) {
	if em.selectedCluster == nil || em.selectedDetail == nil || m.cfg == nil {
		return *m, nil
	}
	m.stopWatch()
	if m.screen != screenECSInvestigation {
		em.rolloutScroll = em.detailScroll
		em.detailScroll = 0
	}
	em.investigationLoading = true
	em.investigation = awsservice.ECSInvestigation{}
	m.screen = screenECSInvestigation
	cfg, cluster, service := *m.cfg, em.selectedCluster.ARN, em.selectedDetail.ARN
	if m.commands == nil {
		m.commands = newCommandLifecycle()
	}
	generation := m.commands.Renew()
	baseCtx := m.commandContext()
	cmd := func() tea.Msg {
		ctx, cancel := context.WithTimeout(baseCtx, ecsAPITimeout)
		defer cancel()
		repo, err := newECSTargetHealthRepository(ctx, &cfg)
		if err != nil {
			return ecsInvestigationLoadedMsg{awsservice.ECSInvestigation{ObservedAt: time.Now().UTC(), Warnings: []string{"Investigation unavailable: " + err.Error()}}}
		}
		return ecsInvestigationLoadedMsg{repo.InvestigateECSService(ctx, cluster, service)}
	}
	return *m, m.commands.BindCmd(generation, cmd)
}

func (em *ecsModel) updateInvestigation(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		if m.commands != nil {
			m.commands.CancelAll()
		}
		em.investigationLoading = false
		em.detailScroll = em.rolloutScroll
		m.screen = screenECSServiceDetail
	case "r":
		return em.startInvestigation(m)
	case "up", "k", "down", "j", "pgup", "pgdown":
		if !em.investigationLoading {
			em.scrollServiceDetail(*m, msg.String(), ecsInvestigationHelp)
		}
	}
	return *m, nil
}

func (em ecsModel) investigationLines() []string {
	if em.investigationLoading {
		return []string{dimStyle.Render("Collecting related ECS evidence (30s limit)... Esc cancels and returns to rollout.")}
	}
	result := em.investigation
	lines := []string{renderDetailLine("Collection started (UTC)", result.ObservedAt.Format(time.RFC3339)),
		dimStyle.Render("Sequential observations, not an atomic snapshot. Correlation does not establish cause."),
		dimStyle.Render("Logs and CloudTrail: preceding 6h. Target health/log groups may include other services."), ""}
	for _, warning := range result.Warnings {
		lines = append(lines, warningStyle.Render("Unavailable/incomplete: "+warning))
	}
	lines = append(lines, "", titleStyle.Render("ECS rollout and service events"))
	if result.Rollout != nil {
		snapshot := em
		snapshot.selectedDetail = result.Rollout
		lines = append(lines, snapshot.serviceDetailLines()...)
	} else {
		lines = append(lines, "Fresh rollout unavailable; return to the previous rollout for earlier evidence.")
	}
	lines = append(lines, "", titleStyle.Render("Associated target health"))
	health := em
	health.healthResults = result.Health
	health.healthLoading = false
	health.healthObserved = result.ObservedAt
	health.healthError = nil
	if len(result.Health) == 0 {
		lines = append(lines, "No target evidence in this collection; check warnings before inferring no associations.")
	} else {
		lines = append(lines, health.targetHealthLines()...)
	}
	lines = append(lines, "", titleStyle.Render("ECS stopped tasks (one page, up to 20)"))
	if len(result.StoppedTasks) == 0 {
		lines = append(lines, "No stopped-task evidence returned. Check warnings; ECS retention is limited.")
	}
	for _, task := range result.StoppedTasks {
		stoppedAt := ecsEvidenceTime(task.StoppedAt)
		lines = append(lines, task.ARN, fmt.Sprintf("  %s %s: %s", stoppedAt, task.StopCode, task.Reason))
		lines = append(lines, task.Containers...)
	}
	lines = append(lines, "", titleStyle.Render("Associated awslogs groups (group-wide sample)"))
	if len(result.Logs) == 0 {
		lines = append(lines, "No log evidence collected; inspect association/permission warnings.")
	}
	for _, log := range result.Logs {
		lines = append(lines, log.Group)
		if log.Warning != "" {
			lines = append(lines, warningStyle.Render(log.Warning))
		}
		if len(log.Events) == 0 {
			lines = append(lines, "  No events in returned page; this does not prove no logs exist.")
		}
		for _, event := range log.Events {
			lines = append(lines, ecsEvidenceTime(event.Timestamp)+" "+event.Message)
		}
	}
	lines = append(lines, "", titleStyle.Render("CloudTrail writes (exact service ARN lookup, preceding 6h)"),
		"Resource indexing may omit relevant changes. Empty results do not prove no changes occurred.")
	if !result.ChangesCollected {
		lines = append(lines, "CloudTrail unavailable or not collected completely; any fetched events remain below. See warnings.")
	} else if len(result.Changes) == 0 {
		lines = append(lines, "No indexed writes returned for this service ARN and time window.")
	}
	for _, change := range result.Changes {
		lines = append(lines, change.Time.UTC().Format(time.RFC3339)+" "+change.Name+" "+change.Source)
	}
	lines = append(lines, "", "Next checks: compare deployment/task timestamps, verify image availability and target-health reasons.",
		"Inspect logs and change details in their owning browsers; the evidence above does not establish root cause.")
	// Expand multiline AWS text before the shared viewport wraps and counts physical lines.
	return strings.Split(strings.Join(lines, "\n"), "\n")
}

func ecsEvidenceTime(value time.Time) string {
	if value.IsZero() {
		return "not reported"
	}
	return value.UTC().Format(time.RFC3339)
}
