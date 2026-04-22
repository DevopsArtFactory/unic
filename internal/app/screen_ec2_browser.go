package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

type ec2InstanceBrowserModel struct {
	instances []awsservice.EC2Instance
	filtered  []awsservice.EC2Instance
	idx       int
	selected  *awsservice.EC2Instance
}

func newEC2InstanceBrowserModel() ec2InstanceBrowserModel {
	return ec2InstanceBrowserModel{}
}

func (em *ec2InstanceBrowserModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(em.loadInstances(*m))
}

func (em *ec2InstanceBrowserModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case ec2BrowserInstancesLoadedMsg:
		em.instances = msg.instances
		em.filtered = applyFilter(em.instances, m.filterValue(filterEC2BrowserInstances))
		em.idx = 0
		em.selected = nil
		m.screen = screenEC2InstanceBrowserList
		return *m, nil, true
	}
	return *m, nil, false
}

func (em *ec2InstanceBrowserModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenEC2InstanceBrowserList:
		newM, cmd := em.updateList(m, msg)
		return newM, cmd, true
	case screenEC2InstanceBrowserDetail:
		newM, cmd := em.updateDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (em ec2InstanceBrowserModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenEC2InstanceBrowserList:
		return em.viewList(m), true
	case screenEC2InstanceBrowserDetail:
		return em.viewDetail(m), true
	default:
		return "", false
	}
}

func (em *ec2InstanceBrowserModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterEC2BrowserInstances {
		return false
	}
	em.filtered = applyFilter(em.instances, m.filterValue(target))
	em.idx = 0
	return true
}

func (em *ec2InstanceBrowserModel) updateList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterEC2BrowserInstances); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterEC2BrowserInstances)
	case "up", "k":
		em.idx = previousListIndex(em.idx, len(em.filtered))
	case "down", "j":
		em.idx = nextListIndex(em.idx, len(em.filtered))
	case "/":
		return *m, m.activateFilter(filterEC2BrowserInstances)
	case "r":
		m.resetFilter(filterEC2BrowserInstances)
		return m.startLoading(em.loadInstances(*m))
	case "enter":
		if len(em.filtered) > 0 && em.idx < len(em.filtered) {
			selected := em.filtered[em.idx]
			em.selected = &selected
			m.screen = screenEC2InstanceBrowserDetail
		}
	}
	return *m, nil
}

func (em *ec2InstanceBrowserModel) updateDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenEC2InstanceBrowserList
	}
	return *m, nil
}

func (em ec2InstanceBrowserModel) loadInstances(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}

		instances, err := repo.ListEC2Instances(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return ec2BrowserInstancesLoadedMsg{instances: instances}
	}
}

func (em ec2InstanceBrowserModel) viewList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("EC2 Instance Browser"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterEC2BrowserInstances))
	b.WriteString("\n\n")

	if len(em.filtered) == 0 {
		emptyText := "  No EC2 instances found"
		if len(em.instances) > 0 {
			emptyText = "  No matching instances"
		}
		panel.WriteString(dimStyle.Render(emptyText))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if em.idx >= visibleLines {
			start = em.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filtered))

		for i := start; i < end; i++ {
			inst := em.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == em.idx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterEC2BrowserInstances, inst.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d instances", len(em.filtered), len(em.instances))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: details • esc: back • H: home"))
	return b.String()
}

func (em ec2InstanceBrowserModel) viewDetail(m Model) string {
	if em.selected == nil {
		return ""
	}
	inst := em.selected
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("EC2 Instance Detail"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Instance ID", normalStyle.Render(inst.InstanceID)))
	b.WriteString(renderDetailLine("Name", normalStyle.Render(ec2ValueOrDash(inst.Name))))
	b.WriteString(renderDetailLine("State", renderEC2InstanceState(inst.State)))
	b.WriteString(renderDetailLine("Type", normalStyle.Render(ec2ValueOrDash(inst.InstanceType))))
	b.WriteString(renderDetailLine("AZ", normalStyle.Render(ec2ValueOrDash(inst.AvailabilityZone))))
	b.WriteString(renderDetailLine("VPC", normalStyle.Render(ec2ValueOrDash(inst.VPCID))))
	b.WriteString(renderDetailLine("Subnet", normalStyle.Render(ec2ValueOrDash(inst.SubnetID))))
	b.WriteString(renderDetailLine("Private IP", normalStyle.Render(ec2ValueOrDash(inst.PrivateIP))))
	b.WriteString(renderDetailLine("Public IP", normalStyle.Render(ec2ValueOrDash(inst.PublicIP))))
	b.WriteString(renderDetailLine("Launch Time", normalStyle.Render(formatEC2LaunchTime(inst.LaunchTime))))
	b.WriteString(renderDetailLine("Platform", normalStyle.Render(ec2ValueOrDash(inst.PlatformDetails))))
	b.WriteString(renderDetailLine("IAM Profile", normalStyle.Render(ec2ValueOrDash(inst.IAMProfile))))

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Tags"))
	b.WriteString("\n")
	if len(inst.Tags) == 0 {
		b.WriteString(dimStyle.Render("  (none)"))
		b.WriteString("\n")
	} else {
		for _, key := range sortedEC2TagKeys(inst.Tags) {
			b.WriteString(renderDetailLine(key, normalStyle.Render(inst.Tags[key])))
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("esc: back • H: home"))
	return b.String()
}

func renderEC2InstanceState(state string) string {
	switch state {
	case "running":
		return selectedStyle.Render(state)
	case "stopped", "terminated", "shutting-down":
		return errorStyle.Render(ec2ValueOrDash(state))
	case "":
		return dimStyle.Render("-")
	default:
		return normalStyle.Render(state)
	}
}

func formatEC2LaunchTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05 MST")
}

func sortedEC2TagKeys(tags map[string]string) []string {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func ec2ValueOrDash(value string) string {
	if value == "" || value == "Unknown" {
		return "-"
	}
	return value
}
