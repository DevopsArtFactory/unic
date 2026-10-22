package app

import (
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"unic/internal/clipboard"
)

const ecsSummaryHelp = "↑/↓: scroll • pgup/pgdn: page • c: copy • s: save new file • d: toggle identifiers • esc: investigation"

var ecsSummaryCopy = clipboard.Copy
var ecsSummarySave = saveECSSummary

type ecsSummaryActionMsg struct {
	notice string
	err    error
}

func (em ecsModel) summaryMarkdown(m Model) string {
	cluster, service, region := "", "", ""
	if em.selectedCluster != nil {
		cluster = em.selectedCluster.ARN
	}
	if em.selectedDetail != nil {
		service = em.selectedDetail.ARN
	}
	if m.cfg != nil {
		region = m.cfg.Region
	}
	return ecsSummary(em.investigation, cluster, service, region, em.summaryIdentifiers)
}

func (em *ecsModel) openSummary(m *Model) (tea.Model, tea.Cmd) {
	if em.investigationLoading || em.investigation.ObservedAt.IsZero() {
		return *m, nil
	}
	em.investigationScroll = em.detailScroll
	em.detailScroll = 0
	em.summaryIdentifiers = false
	em.summaryNotice = ""
	em.summaryPath = ecsSummaryFilename(time.Now())
	if absolute, err := filepath.Abs(em.summaryPath); err == nil {
		em.summaryPath = absolute
	}
	m.screen = screenECSSummary
	return *m, nil
}

func (em *ecsModel) updateSummary(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		em.detailScroll = em.investigationScroll
		m.screen = screenECSInvestigation
	case "d":
		em.summaryIdentifiers = !em.summaryIdentifiers
		em.detailScroll = 0
		em.summaryNotice = ""
	case "c":
		text := em.summaryMarkdown(*m)
		return *m, func() tea.Msg {
			return ecsSummaryActionMsg{notice: "Copied the previewed Markdown.", err: ecsSummaryCopy(text)}
		}
	case "s":
		text, path := em.summaryMarkdown(*m), em.summaryPath
		return *m, func() tea.Msg { return ecsSummaryActionMsg{notice: "Saved " + path, err: ecsSummarySave(path, text)} }
	case "up", "k", "down", "j", "pgup", "pgdown":
		em.scrollServiceDetail(*m, msg.String(), ecsSummaryHelp)
	}
	return *m, nil
}

func (em ecsModel) summaryLines(m Model) []string {
	state := "OFF (redacted)"
	if em.summaryIdentifiers {
		state = "ON: cluster/service ARNs and region will be disclosed"
	}
	lines := []string{"Identifiers: " + state, "Save path: " + em.summaryPath, "Existing files are never overwritten; nothing is uploaded.", em.summaryNotice, ""}
	return append(lines, strings.Split(em.summaryMarkdown(m), "\n")...)
}
