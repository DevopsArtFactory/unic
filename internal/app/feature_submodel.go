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
	return []featureSubmodel{&m.ec2Browser, &m.autoScaling, &m.ecs, &m.eks, &m.ecr, &m.fis, &m.vpc, &m.reachability, &m.cwMetrics, &m.cwAlarms, &m.cloudTrail, &m.cwLogs, &m.rds, &m.cloudFormation, &m.route53, &m.iam, &m.bedrock, &m.secrets, &m.security, &m.s3, &m.sns, &m.sqs, &m.elb, &m.ssmParams, &m.elasticache, &m.kms, &m.acm, &m.stepFunctions, &m.eventBridge, &m.lambda, &m.dynamodb, &m.inspector}
}

// overlayPreviousScreen returns the pointer holding the screen a global
// overlay will return to, or nil when the screen is not an overlay.
func overlayPreviousScreen(m *Model, current screen) *screen {
	switch current {
	case screenSettings:
		return &m.settingsPrevScreen
	case screenCommandPalette:
		return &m.palette.prevScreen
	case screenViewList:
		return &m.views.prevScreen
	case screenContextPicker:
		return &m.ctxPrevScreen
	case screenRegionPicker:
		return &m.regionPrevScreen
	default:
		return nil
	}
}

// finishBrowserLoad reveals a completed load, or — when a global overlay was
// opened while it was in flight — rewrites that overlay's return target so
// dismissing it lands on the loaded screen instead of a stale spinner.
// Overlays can stack, so the chain is walked; the bound and the seen set guard
// against a cycle built by an unusual navigation sequence.
func finishBrowserLoad(m *Model, target screen) {
	if m.screen == screenLoading {
		m.screen = target
		return
	}
	current := m.screen
	seen := make(map[screen]struct{})
	for range 8 {
		if _, ok := seen[current]; ok {
			return
		}
		seen[current] = struct{}{}
		previous := overlayPreviousScreen(m, current)
		if previous == nil {
			return
		}
		if *previous == screenLoading {
			*previous = target
			return
		}
		current = *previous
	}
}
