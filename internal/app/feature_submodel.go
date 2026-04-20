package app

import tea "github.com/charmbracelet/bubbletea"

type featureSubmodel interface {
	HandleMessage(*Model, tea.Msg) (tea.Model, tea.Cmd, bool)
	HandleKey(*Model, tea.KeyMsg) (tea.Model, tea.Cmd, bool)
	View(Model) (string, bool)
	ApplyFilter(*Model, filterTarget) bool
}

func (m *Model) featureSubmodels() []featureSubmodel {
	return []featureSubmodel{&m.cwMetrics, &m.cwLogs, &m.bedrock}
}
