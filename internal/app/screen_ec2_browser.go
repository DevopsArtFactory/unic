package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsservice "unic/internal/services/aws"
)

type ec2InstanceBrowserModel struct {
	instances       []awsservice.EC2Instance
	filtered        []awsservice.EC2Instance
	idx             int
	allRegions      bool
	regionErrors    []awsservice.RegionError
	selected        *awsservice.EC2Instance
	relationships   *awsservice.EC2InstanceRelationships
	relatedKind     ec2RelatedKind
	relatedItems    []ec2RelatedItem
	filteredRelated []ec2RelatedItem
	relatedIdx      int
	selectedRelated *ec2RelatedItem
}

func newEC2InstanceBrowserModel() ec2InstanceBrowserModel {
	return ec2InstanceBrowserModel{}
}

type ec2RelatedKind string

const (
	ec2RelatedSecurityGroups ec2RelatedKind = "security groups"
	ec2RelatedAutoScaling    ec2RelatedKind = "auto scaling"
	ec2RelatedTargetGroups   ec2RelatedKind = "target groups"
	ec2RelatedLoadBalancers  ec2RelatedKind = "load balancers"
	ec2RelatedListeners      ec2RelatedKind = "listeners"
)

type ec2RelatedItem struct {
	title   string
	filter  string
	details []detailRow
}

type detailRow struct {
	label string
	value string
}

func (i ec2RelatedItem) DisplayTitle() string {
	return i.title
}

func (i ec2RelatedItem) FilterText() string {
	return strings.ToLower(i.filter)
}

func (em *ec2InstanceBrowserModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(em.loadInstances(*m))
}

func (em *ec2InstanceBrowserModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case ec2BrowserInstancesLoadedMsg:
		em.instances = msg.instances
		em.regionErrors = msg.regionErrors
		em.filtered = applyFilter(em.instances, m.filterValue(filterEC2BrowserInstances))
		em.idx = 0
		em.selected = nil
		em.relationships = nil
		m.screen = screenEC2InstanceBrowserList
		return *m, nil, true
	case ec2RelationshipsLoadedMsg:
		em.relationships = msg.relationships
		em.relatedKind = msg.kind
		em.relatedItems = em.buildRelatedItems(msg.kind)
		em.filteredRelated = applyFilter(em.relatedItems, m.filterValue(filterEC2BrowserRelated))
		em.relatedIdx = 0
		em.selectedRelated = nil
		m.screen = screenEC2InstanceBrowserRelatedList
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
	case screenEC2InstanceBrowserRelatedList:
		newM, cmd := em.updateRelatedList(m, msg)
		return newM, cmd, true
	case screenEC2InstanceBrowserRelatedDetail:
		newM, cmd := em.updateRelatedDetail(m, msg)
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
	case screenEC2InstanceBrowserRelatedList:
		return em.viewRelatedList(m), true
	case screenEC2InstanceBrowserRelatedDetail:
		return em.viewRelatedDetail(m), true
	default:
		return "", false
	}
}

func (em *ec2InstanceBrowserModel) ApplyFilter(m *Model, target filterTarget) bool {
	switch target {
	case filterEC2BrowserInstances:
		em.filtered = applyFilter(em.instances, m.filterValue(target))
		em.idx = 0
		return true
	case filterEC2BrowserRelated:
		em.filteredRelated = applyFilter(em.relatedItems, m.filterValue(target))
		em.relatedIdx = 0
		return true
	}
	return false
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
	case "A":
		if m.hasMultipleRegions() {
			em.allRegions = !em.allRegions
			m.resetFilter(filterEC2BrowserInstances)
			return m.startLoading(em.loadInstances(*m))
		}
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
	case "g":
		return em.openRelated(m, ec2RelatedSecurityGroups)
	case "a":
		return em.openRelated(m, ec2RelatedAutoScaling)
	case "t":
		return em.openRelated(m, ec2RelatedTargetGroups)
	case "b":
		return em.openRelated(m, ec2RelatedLoadBalancers)
	case "n":
		return em.openRelated(m, ec2RelatedListeners)
	}
	return *m, nil
}

func (em *ec2InstanceBrowserModel) updateRelatedList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterEC2BrowserRelated); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q", "esc":
		m.screen = screenEC2InstanceBrowserDetail
		m.resetFilter(filterEC2BrowserRelated)
	case "up", "k":
		em.relatedIdx = previousListIndex(em.relatedIdx, len(em.filteredRelated))
	case "down", "j":
		em.relatedIdx = nextListIndex(em.relatedIdx, len(em.filteredRelated))
	case "/":
		return *m, m.activateFilter(filterEC2BrowserRelated)
	case "r":
		m.resetFilter(filterEC2BrowserRelated)
		return em.openRelated(m, em.relatedKind)
	case "enter":
		if len(em.filteredRelated) > 0 && em.relatedIdx < len(em.filteredRelated) {
			selected := em.filteredRelated[em.relatedIdx]
			em.selectedRelated = &selected
			m.screen = screenEC2InstanceBrowserRelatedDetail
		}
	}
	return *m, nil
}

func (em *ec2InstanceBrowserModel) updateRelatedDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenEC2InstanceBrowserRelatedList
	}
	return *m, nil
}

func (em ec2InstanceBrowserModel) loadInstances(m Model) tea.Cmd {
	allRegions := em.allRegions && m.hasMultipleRegions()
	var regions []string
	if m.cfg != nil {
		regions = append(regions, m.cfg.Regions...)
	}
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}

		if allRegions {
			instances, regionErrors := repo.ListEC2InstancesAcrossRegions(ctx, regions)
			return ec2BrowserInstancesLoadedMsg{instances: instances, regionErrors: regionErrors}
		}

		instances, err := repo.ListEC2Instances(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return ec2BrowserInstancesLoadedMsg{instances: instances}
	}
}

func (em *ec2InstanceBrowserModel) openRelated(m *Model, kind ec2RelatedKind) (tea.Model, tea.Cmd) {
	if em.selected == nil {
		return *m, nil
	}
	return m.startLoadingWithMessage("Loading EC2 relationships...", []string{em.selected.InstanceID, string(kind)}, em.loadRelationships(*m, *em.selected, kind))
}

func (em ec2InstanceBrowserModel) loadRelationships(m Model, inst awsservice.EC2Instance, kind ec2RelatedKind) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		if inst.Region != "" && inst.Region != repo.Region {
			repo = repo.ForRegion(inst.Region)
		}
		relationships, err := repo.DescribeEC2InstanceRelationships(ctx, inst)
		if err != nil {
			return errMsg{err: err}
		}
		return ec2RelationshipsLoadedMsg{relationships: relationships, kind: kind}
	}
}

func (em ec2InstanceBrowserModel) viewList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	// allRegions can outlive a switch to a single-region context; render from
	// the effective scope that loadInstances actually uses.
	allRegions := em.allRegions && m.hasMultipleRegions()
	title := "EC2 Instance Browser"
	if allRegions {
		title += " (all regions)"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterEC2BrowserInstances))
	b.WriteString("\n\n")

	for _, regionErr := range em.regionErrors {
		panel.WriteString(errorStyle.Render(fmt.Sprintf("  %s: %v", regionErr.Region, regionErr.Err)))
		panel.WriteString("\n")
	}
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
			row := inst.DisplayTitle()
			if allRegions {
				row = fmt.Sprintf("[%s] %s", inst.Region, row)
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterEC2BrowserInstances, row)))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d instances", len(em.filtered), len(em.instances))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	helpText := "↑/↓: navigate • /: filter • r: refresh • enter: details • esc: back • H: home"
	if m.hasMultipleRegions() {
		helpText = "↑/↓: navigate • /: filter • r: refresh • A: all regions • enter: details • esc: back • H: home"
	}
	b.WriteString(m.renderHelpBar(helpText))
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

	b.WriteString(m.renderEC2DetailLine("Instance ID", inst.InstanceID))
	b.WriteString(m.renderEC2DetailLine("Name", ec2ValueOrDash(inst.Name)))
	b.WriteString(m.renderEC2StyledDetailLine("State", renderEC2InstanceState(inst.State)))
	b.WriteString(m.renderEC2DetailLine("Type", ec2ValueOrDash(inst.InstanceType)))
	b.WriteString(m.renderEC2DetailLine("Region", ec2ValueOrDash(inst.Region)))
	b.WriteString(m.renderEC2DetailLine("AZ", ec2ValueOrDash(inst.AvailabilityZone)))
	b.WriteString(m.renderEC2DetailLine("VPC", ec2ValueOrDash(inst.VPCID)))
	b.WriteString(m.renderEC2DetailLine("Subnet", ec2ValueOrDash(inst.SubnetID)))
	b.WriteString(m.renderEC2DetailLine("Private IP", ec2ValueOrDash(inst.PrivateIP)))
	b.WriteString(m.renderEC2DetailLine("Public IP", ec2ValueOrDash(inst.PublicIP)))
	b.WriteString(m.renderEC2DetailLine("Security Groups", formatEC2InstanceSecurityGroups(inst.SecurityGroups)))
	b.WriteString(m.renderEC2DetailLine("Launch Time", formatEC2LaunchTime(inst.LaunchTime)))
	b.WriteString(m.renderEC2DetailLine("Platform", ec2ValueOrDash(inst.PlatformDetails)))
	b.WriteString(m.renderEC2DetailLine("IAM Profile", ec2ValueOrDash(inst.IAMProfile)))

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Tags"))
	b.WriteString("\n")
	if len(inst.Tags) == 0 {
		b.WriteString(dimStyle.Render("  (none)"))
		b.WriteString("\n")
	} else {
		for _, key := range sortedEC2TagKeys(inst.Tags) {
			b.WriteString(m.renderEC2TagLine(key, inst.Tags[key]))
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("g: SGs • a: ASG • t: TGs • b: LBs • n: listeners • esc: back • H: home"))
	return b.String()
}

func (em ec2InstanceBrowserModel) viewRelatedList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	title := fmt.Sprintf("EC2 Related %s", ec2RelatedTitle(em.relatedKind))
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	if em.selected != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Instance: %s (%s)", ec2ValueOrDash(em.selected.Name), em.selected.InstanceID)))
		b.WriteString("\n")
	}
	b.WriteString(m.renderFilterValue(filterEC2BrowserRelated))
	b.WriteString("\n\n")

	if errText := em.relationshipErrorText(em.relatedKind); errText != "" {
		panel.WriteString(errorStyle.Render("  " + errText))
		panel.WriteString("\n")
	}
	if len(em.filteredRelated) == 0 {
		emptyText := fmt.Sprintf("  No related %s", em.relatedKind)
		if len(em.relatedItems) > 0 {
			emptyText = fmt.Sprintf("  No matching %s", em.relatedKind)
		}
		panel.WriteString(dimStyle.Render(emptyText))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-11, 5)
		start := 0
		if em.relatedIdx >= visibleLines {
			start = em.relatedIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(em.filteredRelated))
		for i := start; i < end; i++ {
			item := em.filteredRelated[i]
			cursor := "  "
			style := normalStyle
			if i == em.relatedIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterEC2BrowserRelated, item.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d %s", len(em.filteredRelated), len(em.relatedItems), em.relatedKind)))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: details • esc: back • H: home"))
	return b.String()
}

func (em ec2InstanceBrowserModel) viewRelatedDetail(m Model) string {
	if em.selectedRelated == nil {
		return ""
	}
	item := em.selectedRelated
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render(fmt.Sprintf("EC2 Related %s Detail", ec2RelatedTitle(em.relatedKind))))
	b.WriteString("\n\n")
	for _, row := range item.details {
		b.WriteString(m.renderEC2DetailLine(row.label, ec2ValueOrDash(row.value)))
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("esc: back • H: home"))
	return b.String()
}

func (em ec2InstanceBrowserModel) buildRelatedItems(kind ec2RelatedKind) []ec2RelatedItem {
	if em.relationships == nil {
		return nil
	}
	switch kind {
	case ec2RelatedSecurityGroups:
		items := make([]ec2RelatedItem, 0, len(em.relationships.SecurityGroups))
		for _, sg := range em.relationships.SecurityGroups {
			items = append(items, ec2RelatedItem{
				title:  sg.DisplayTitle(),
				filter: sg.FilterText(),
				details: []detailRow{
					{"Group ID", sg.GroupID},
					{"Name", sg.Name},
					{"VPC", sg.VPCID},
					{"Description", sg.Description},
					{"Ingress Rules", fmt.Sprint(len(sg.IngressRules))},
					{"Egress Rules", fmt.Sprint(len(sg.EgressRules))},
				},
			})
		}
		return items
	case ec2RelatedAutoScaling:
		if em.relationships.AutoScaling == nil {
			return nil
		}
		asg := *em.relationships.AutoScaling
		return []ec2RelatedItem{{
			title:  asg.DisplayTitle(),
			filter: asg.FilterText(),
			details: []detailRow{
				{"Name", asg.Name},
				{"ARN", asg.ARN},
				{"Lifecycle", asg.LifecycleState},
				{"Health", asg.HealthStatus},
				{"Desired", fmt.Sprint(asg.DesiredCapacity)},
				{"Min", fmt.Sprint(asg.MinSize)},
				{"Max", fmt.Sprint(asg.MaxSize)},
				{"Target Groups", strings.Join(asg.TargetGroupARNs, ", ")},
				{"Classic LBs", strings.Join(asg.LoadBalancerNames, ", ")},
			},
		}}
	case ec2RelatedTargetGroups:
		items := make([]ec2RelatedItem, 0, len(em.relationships.TargetGroups))
		for _, tg := range em.relationships.TargetGroups {
			items = append(items, ec2RelatedItem{
				title:  tg.DisplayTitle(),
				filter: tg.FilterText(),
				details: []detailRow{
					{"Name", tg.Name},
					{"ARN", tg.ARN},
					{"Protocol", tg.Protocol},
					{"Port", fmt.Sprint(tg.Port)},
					{"VPC", tg.VPCID},
					{"Target Type", tg.TargetType},
					{"Health", tg.HealthState},
					{"Health Reason", tg.HealthReason},
					{"Health Detail", tg.HealthDescription},
					{"Load Balancers", strings.Join(tg.LoadBalancerARNs, ", ")},
				},
			})
		}
		return items
	case ec2RelatedLoadBalancers:
		items := make([]ec2RelatedItem, 0, len(em.relationships.LoadBalancers))
		for _, lb := range em.relationships.LoadBalancers {
			items = append(items, ec2RelatedItem{
				title:  lb.DisplayTitle(),
				filter: lb.FilterText(),
				details: []detailRow{
					{"Name", lb.Name},
					{"ARN", lb.ARN},
					{"DNS", lb.DNSName},
					{"Type", lb.Type},
					{"Scheme", lb.Scheme},
					{"State", lb.State},
					{"VPC", lb.VPCID},
				},
			})
		}
		return items
	case ec2RelatedListeners:
		items := make([]ec2RelatedItem, 0, len(em.relationships.Listeners))
		for _, listener := range em.relationships.Listeners {
			items = append(items, ec2RelatedItem{
				title:  listener.DisplayTitle(),
				filter: listener.FilterText(),
				details: []detailRow{
					{"Listener ARN", listener.ARN},
					{"Load Balancer", listener.LoadBalancerName},
					{"Load Balancer ARN", listener.LoadBalancerARN},
					{"Protocol", listener.Protocol},
					{"Port", fmt.Sprint(listener.Port)},
					{"Rules", fmt.Sprint(listener.RuleCount)},
					{"Default Action", listener.DefaultAction},
				},
			})
		}
		return items
	}
	return nil
}

func (m Model) renderEC2DetailLine(label, value string) string {
	return m.renderEC2DetailLineWithLabelWidth(label, value, ec2DetailLabelWidth)
}

func (m Model) renderEC2StyledDetailLine(label, renderedValue string) string {
	return m.renderEC2StyledDetailLineWithLabelWidth(label, renderedValue, ec2DetailLabelWidth)
}

func (m Model) renderEC2TagLine(label, value string) string {
	return m.renderEC2DetailLineWithLabelWidth(label, value, ec2TagLabelWidth)
}

func (m Model) renderEC2DetailLineWithLabelWidth(label, value string, labelWidth int) string {
	width := m.ec2DetailValueWidth(labelWidth)
	return m.renderEC2StyledDetailLineWithLabelWidth(label, normalStyle.Render(truncateEC2DetailValue(escapeTerminalControls(value), width)), labelWidth)
}

func escapeTerminalControls(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) {
			quoted := strconv.QuoteRuneToGraphic(r)
			b.WriteString(quoted[1 : len(quoted)-1])
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (m Model) renderEC2StyledDetailLineWithLabelWidth(label, renderedValue string, labelWidth int) string {
	label = truncateEC2DetailValue(escapeTerminalControls(label), labelWidth)
	padding := labelWidth - lipgloss.Width(label)
	if padding < 0 {
		padding = 0
	}
	return "  " + dimStyle.Render(label+strings.Repeat(" ", padding)) + renderedValue + "\n"
}

func (m Model) ec2DetailValueWidth(labelWidth int) int {
	if m.width <= 0 {
		return 88
	}
	width := m.width - 2 - labelWidth
	if width < 24 {
		return 24
	}
	return width
}

const ec2DetailLabelWidth = 18
const ec2TagLabelWidth = 30

func truncateEC2DetailValue(value string, width int) string {
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	suffix := "..."
	target := width - lipgloss.Width(suffix)
	if target <= 0 {
		return suffix
	}
	var b strings.Builder
	current := 0
	for _, r := range value {
		rw := lipgloss.Width(string(r))
		if current+rw > target {
			break
		}
		b.WriteRune(r)
		current += rw
	}
	return b.String() + suffix
}

func (em ec2InstanceBrowserModel) relationshipErrorText(kind ec2RelatedKind) string {
	if em.relationships == nil {
		return ""
	}
	section := string(kind)
	if kind == ec2RelatedLoadBalancers || kind == ec2RelatedListeners {
		section = "load balancers/listeners"
	}
	for _, relErr := range em.relationships.Errors {
		if relErr.Section == section && relErr.Err != nil {
			return relErr.Err.Error()
		}
	}
	return ""
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

func formatEC2InstanceSecurityGroups(groups []awsservice.EC2InstanceSecurityGroup) string {
	if len(groups) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		label := group.GroupID
		if group.Name != "" {
			label = fmt.Sprintf("%s (%s)", group.Name, group.GroupID)
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, ", ")
}

func ec2RelatedTitle(kind ec2RelatedKind) string {
	switch kind {
	case ec2RelatedSecurityGroups:
		return "Security Groups"
	case ec2RelatedAutoScaling:
		return "Auto Scaling"
	case ec2RelatedTargetGroups:
		return "Target Groups"
	case ec2RelatedLoadBalancers:
		return "Load Balancers"
	case ec2RelatedListeners:
		return "Listeners"
	default:
		return "Resources"
	}
}

func ec2ValueOrDash(value string) string {
	if value == "" || value == "Unknown" {
		return "-"
	}
	return value
}
