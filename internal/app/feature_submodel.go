package app

import tea "github.com/charmbracelet/bubbletea"

type featureSubmodel interface {
	HandleMessage(*Model, tea.Msg) (tea.Model, tea.Cmd, bool)
	HandleKey(*Model, tea.KeyMsg) (tea.Model, tea.Cmd, bool)
	View(Model) (string, bool)
	ApplyFilter(*Model, filterTarget) bool
}

func (m *Model) featureSubmodels() []featureSubmodel {
	return []featureSubmodel{&m.ec2Browser, &m.ecs, &m.eks, &m.ecr, &m.vpc, &m.reachability, &m.cwMetrics, &m.cwLogs, &m.rds, &m.route53, &m.iam, &m.bedrock, &m.secrets, &m.security, &m.s3, &m.lambda, &m.inspector}
}
