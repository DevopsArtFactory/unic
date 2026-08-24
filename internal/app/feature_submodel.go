package app

import tea "github.com/charmbracelet/bubbletea"

type featureSubmodel interface {
	HandleMessage(*Model, tea.Msg) (tea.Model, tea.Cmd, bool)
	HandleKey(*Model, tea.KeyMsg) (tea.Model, tea.Cmd, bool)
	View(Model) (string, bool)
	ApplyFilter(*Model, filterTarget) bool
}

// featureSubmodels are normal AWS feature browsers/workflows. App-shell flows
// such as service selection, context selection, SSM session launch, loading, and
// errors remain root-owned unless a separate shell abstraction is introduced.
func (m *Model) featureSubmodels() []featureSubmodel {
	return []featureSubmodel{&m.ec2Browser, &m.autoScaling, &m.ecs, &m.eks, &m.ecr, &m.fis, &m.vpc, &m.reachability, &m.cwMetrics, &m.cwAlarms, &m.cloudTrail, &m.cwLogs, &m.rds, &m.cloudFormation, &m.route53, &m.iam, &m.bedrock, &m.secrets, &m.security, &m.s3, &m.sqs, &m.elb, &m.ssmParams, &m.elasticache, &m.kms, &m.acm, &m.stepFunctions, &m.eventBridge, &m.lambda, &m.inspector}
}
