package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsservice "unic/internal/services/aws"
)

type snsModel struct {
	topics         []awsservice.SNSTopic
	filteredTopics []awsservice.SNSTopic
	topicIdx       int
	selectedTopic  *awsservice.SNSTopic
	detailScroll   int
	topicWarnings  []error
	subscriptions  []awsservice.SNSSubscription
	filteredSubs   []awsservice.SNSSubscription
	subIdx         int
	subWarnings    []error
}

func newSNSModel() snsModel { return snsModel{} }

func isSNSScreen(s screen) bool {
	switch s {
	case screenSNSTopicList, screenSNSTopicDetail, screenSNSSubscriptionList:
		return true
	default:
		return false
	}
}

func resetSNSContextState(m *Model) {
	m.sns = newSNSModel()
	m.resetFilter(filterSNSTopics)
	m.resetFilter(filterSNSSubscriptions)
}

func normalizeSNSContextReturn(m *Model) {
	if previous := snsContextReturn(m); previous != nil {
		*previous = screenFeatureList
	}
}

func preservePendingSNSContextReturn(m *Model) {
	if previous := snsContextReturn(m); previous != nil && *previous == screenLoading {
		*previous = m.loadingReturnScreen
	}
}

func snsContextReturn(m *Model) *screen {
	previous := &m.ctxPrevScreen
	seen := make(map[screen]struct{})
	for range 8 {
		current := *previous
		if _, ok := seen[current]; ok {
			return nil
		}
		seen[current] = struct{}{}
		if isSNSScreen(current) || current == screenLoading && isSNSScreen(m.loadingReturnScreen) {
			return previous
		}
		previous = overlayPreviousScreen(m, current)
		if previous == nil {
			return nil
		}
	}
	return nil
}

func (sm *snsModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoadingFor(screenSNSTopicList, "Loading SNS topics...", nil, sm.loadTopics(*m))
}

func (sm *snsModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case snsTopicsLoadedMsg:
		if msg.err != nil {
			finishSNSError(m, msg.err)
			return *m, nil, true
		}
		sm.topics = msg.topics
		sm.topicWarnings = msg.warnings
		sm.filteredTopics = applyFilter(sm.topics, m.filterValue(filterSNSTopics))
		sm.topicIdx = 0
		sm.selectedTopic = nil
		sm.detailScroll = 0
		finishSNSLoad(m, screenSNSTopicList)
		return *m, nil, true
	case snsSubscriptionsLoadedMsg:
		// A slow subscription load for a topic the operator already left must
		// not steal the screen or overwrite the newer topic's list.
		if sm.selectedTopic == nil || sm.selectedTopic.ARN != msg.topicARN {
			return *m, nil, true
		}
		if msg.err != nil {
			finishSNSError(m, msg.err)
			return *m, nil, true
		}
		sm.subscriptions = msg.subscriptions
		sm.subWarnings = msg.warnings
		sm.filteredSubs = applyFilter(sm.subscriptions, m.filterValue(filterSNSSubscriptions))
		sm.subIdx = 0
		finishSNSLoad(m, screenSNSSubscriptionList)
		return *m, nil, true
	}
	return *m, nil, false
}

func finishSNSError(m *Model, err error) {
	m.errMsg = err.Error()
	m.loadingTitle = ""
	m.loadingDetails = nil
	finishSNSLoad(m, screenError)
}

// finishSNSLoad keeps a completed load behind a global overlay and rewrites
// that overlay's return target instead of stealing the screen.
func finishSNSLoad(m *Model, target screen) {
	if isSNSScreen(m.loadingReturnScreen) {
		m.loadingReturnScreen = target
	}
	if m.ctxPrevScreen == screenLoading || isSNSScreen(m.ctxPrevScreen) {
		m.ctxPrevScreen = target
	}
	finishBrowserLoad(m, target)
}

func (sm *snsModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenSNSTopicList:
		newM, cmd := sm.updateTopicList(m, msg)
		return newM, cmd, true
	case screenSNSTopicDetail:
		newM, cmd := sm.updateTopicDetail(m, msg)
		return newM, cmd, true
	case screenSNSSubscriptionList:
		newM, cmd := sm.updateSubscriptionList(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (sm snsModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenSNSTopicList:
		return sm.viewTopicList(m), true
	case screenSNSTopicDetail:
		return sm.viewTopicDetail(m), true
	case screenSNSSubscriptionList:
		return sm.viewSubscriptionList(m), true
	default:
		return "", false
	}
}

func (sm *snsModel) ApplyFilter(m *Model, target filterTarget) bool {
	switch target {
	case filterSNSTopics:
		sm.filteredTopics = applyFilter(sm.topics, m.filterValue(target))
		sm.topicIdx = 0
		return true
	case filterSNSSubscriptions:
		sm.filteredSubs = applyFilter(sm.subscriptions, m.filterValue(target))
		sm.subIdx = 0
		return true
	default:
		return false
	}
}

func (sm *snsModel) updateTopicList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterSNSTopics); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterSNSTopics)
	case "up", "k":
		sm.topicIdx = previousListIndex(sm.topicIdx, len(sm.filteredTopics))
	case "down", "j":
		sm.topicIdx = nextListIndex(sm.topicIdx, len(sm.filteredTopics))
	case "/":
		return *m, m.activateFilter(filterSNSTopics)
	case "r":
		return m.startLoadingFor(screenSNSTopicList, "Loading SNS topics...", nil, sm.loadTopics(*m))
	case "enter":
		if sm.topicIdx < len(sm.filteredTopics) {
			selected := sm.filteredTopics[sm.topicIdx]
			sm.selectedTopic = &selected
			sm.detailScroll = 0
			// Subscriptions belong to the previously selected topic until the
			// new load lands.
			sm.subscriptions = nil
			sm.filteredSubs = nil
			sm.subWarnings = nil
			m.screen = screenSNSTopicDetail
		}
	}
	return *m, nil
}

func (sm *snsModel) updateTopicDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := sm.topicDetailLines(*m)
	visibleLines := max(m.height-9, 5)
	maxOffset := max(len(lines)-visibleLines, 0)
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
		m.resetFilter(filterSNSTopics)
	case "esc":
		sm.detailScroll = 0
		m.screen = screenSNSTopicList
	case "up", "k":
		sm.detailScroll = max(sm.detailScroll-1, 0)
	case "down", "j":
		sm.detailScroll = min(sm.detailScroll+1, maxOffset)
	case "pgup":
		sm.detailScroll = max(sm.detailScroll-visibleLines, 0)
	case "pgdown":
		sm.detailScroll = min(sm.detailScroll+visibleLines, maxOffset)
	case "r":
		return m.startLoadingFor(screenSNSTopicList, "Loading SNS topics...", nil, sm.loadTopics(*m))
	case "enter":
		if sm.selectedTopic == nil {
			return *m, nil
		}
		m.resetFilter(filterSNSSubscriptions)
		return m.startLoadingFor(screenSNSSubscriptionList, "Loading subscriptions...",
			[]string{sm.selectedTopic.Name}, sm.loadSubscriptions(*m, sm.selectedTopic.ARN))
	}
	return *m, nil
}

func (sm *snsModel) updateSubscriptionList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterSNSSubscriptions); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
		m.resetFilter(filterSNSTopics)
		m.resetFilter(filterSNSSubscriptions)
	case "esc":
		m.resetFilter(filterSNSSubscriptions)
		m.screen = screenSNSTopicDetail
	case "up", "k":
		sm.subIdx = previousListIndex(sm.subIdx, len(sm.filteredSubs))
	case "down", "j":
		sm.subIdx = nextListIndex(sm.subIdx, len(sm.filteredSubs))
	case "/":
		return *m, m.activateFilter(filterSNSSubscriptions)
	case "r":
		if sm.selectedTopic == nil {
			return *m, nil
		}
		return m.startLoadingFor(screenSNSSubscriptionList, "Loading subscriptions...",
			[]string{sm.selectedTopic.Name}, sm.loadSubscriptions(*m, sm.selectedTopic.ARN))
	}
	return *m, nil
}

func (sm snsModel) loadTopics(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return snsTopicsLoadedMsg{err: err}
		}
		topics, warnings, err := repo.ListSNSTopics(ctx)
		return snsTopicsLoadedMsg{topics: topics, warnings: warnings, err: err}
	}
}

func (sm snsModel) loadSubscriptions(m Model, topicARN string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return snsSubscriptionsLoadedMsg{topicARN: topicARN, err: err}
		}
		subscriptions, warnings, err := repo.ListSNSSubscriptionsByTopic(ctx, topicARN)
		return snsSubscriptionsLoadedMsg{topicARN: topicARN, subscriptions: subscriptions, warnings: warnings, err: err}
	}
}

func (sm snsModel) viewTopicList(m Model) string {
	var b, panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("SNS Topics"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterSNSTopics))
	b.WriteString("\n\n")

	warningLines := 0
	if len(sm.topicWarnings) > 0 {
		b.WriteString(m.renderWarningSummary(len(sm.topicWarnings), "topic attribute lookup failures", sm.topicWarnings[0].Error()))
		warningLines = 2
	}

	if len(sm.filteredTopics) == 0 {
		empty := "  No SNS topics found"
		if len(sm.topics) > 0 {
			empty = "  No matching SNS topics"
		}
		panel.WriteString(dimStyle.Render(empty))
		panel.WriteString("\n")
	} else {
		panel.WriteString(dimStyle.Render("  " + snsTopicRow(m, "NAME", "TYPE", "SUBSCRIPTIONS", "ENCRYPTION")))
		panel.WriteString("\n")
		visibleLines := max(m.height-12-warningLines, 5)
		start := max(sm.topicIdx-visibleLines+1, 0)
		end := min(start+visibleLines, len(sm.filteredTopics))
		for i := start; i < end; i++ {
			topic := sm.filteredTopics[i]
			cursor, style := "  ", normalStyle
			if i == sm.topicIdx {
				cursor, style = "> ", selectedStyle
			}
			row := snsTopicRow(m, topic.Name, topic.KindLabel(), topic.SubscriptionSummary(), snsEncryptionLabel(topic))
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterSNSTopics, row)))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d topics", len(sm.filteredTopics), len(sm.topics))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func snsTopicRow(m Model, name, kind, subscriptions, encryption string) string {
	widths := snsColumnWidths(m, []int{18, 8, 16, 20}, []int{44, 9, 18, 40})
	return padInspectorText(inspectorShorten(escapeTerminalControls(name), widths[0]), widths[0]) + "  " +
		padInspectorText(inspectorShorten(escapeTerminalControls(kind), widths[1]), widths[1]) + "  " +
		padInspectorText(inspectorShorten(escapeTerminalControls(subscriptions), widths[2]), widths[2]) + "  " +
		snsShortenTail(escapeTerminalControls(encryption), widths[3])
}

func snsEncryptionLabel(topic awsservice.SNSTopic) string {
	if !topic.AttributesKnown {
		return "-"
	}
	if topic.KMSMasterKeyID == "" {
		return "none"
	}
	return topic.KMSMasterKeyID
}

func (sm snsModel) viewTopicDetail(m Model) string {
	if sm.selectedTopic == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("SNS Topic Detail"))
	b.WriteString("\n\n")
	lines := sm.topicDetailLines(m)
	visibleLines := max(m.height-9, 5)
	start := min(sm.detailScroll, max(len(lines)-visibleLines, 0))
	for _, line := range lines[start:min(start+visibleLines, len(lines))] {
		b.WriteString(line)
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (sm snsModel) topicDetailLines(m Model) []string {
	topic := sm.selectedTopic
	if topic == nil {
		return nil
	}
	lines := []string{
		m.renderEC2DetailLine("Name", topic.Name),
		m.renderEC2DetailLine("ARN", topic.ARN),
		m.renderEC2DetailLine("Region", ec2ValueOrDash(topic.Region)),
		m.renderEC2DetailLine("Type", topic.KindLabel()),
	}
	if !topic.AttributesKnown {
		return append(lines, m.renderEC2DetailLine("Attributes", "unavailable (topic attribute lookup failed)"))
	}
	lines = append(lines,
		m.renderEC2DetailLine("Display Name", ec2ValueOrDash(topic.DisplayName)),
		m.renderEC2DetailLine("Encryption", snsEncryptionLabel(*topic)),
		m.renderEC2DetailLine("Confirmed", fmt.Sprintf("%d", topic.SubscriptionsConfirmed)),
		m.renderEC2DetailLine("Pending", fmt.Sprintf("%d", topic.SubscriptionsPending)),
		m.renderEC2DetailLine("Deleted", fmt.Sprintf("%d", topic.SubscriptionsDeleted)),
	)
	if topic.IsFIFO() {
		lines = append(lines, m.renderEC2DetailLine("Content Dedup", fmt.Sprintf("%t", topic.ContentBasedDeduplication)))
	}
	if topic.DeliveryPolicy != "" {
		lines = append(lines, jsonDetailLines(m, "Delivery Policy", topic.DeliveryPolicy)...)
	}
	if topic.EffectiveDeliveryPolicy != "" {
		lines = append(lines, jsonDetailLines(m, "Effective Policy", topic.EffectiveDeliveryPolicy)...)
	}
	return lines
}

func (sm snsModel) viewSubscriptionList(m Model) string {
	if sm.selectedTopic == nil {
		return ""
	}
	var b, panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("SNS Subscriptions — pending first"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Topic: " + escapeTerminalControls(sm.selectedTopic.Name)))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterSNSSubscriptions))
	b.WriteString("\n\n")

	warningLines := 0
	if len(sm.subWarnings) > 0 {
		b.WriteString(m.renderWarningSummary(len(sm.subWarnings), "subscription attribute lookup failures", sm.subWarnings[0].Error()))
		warningLines = 2
	}

	if len(sm.filteredSubs) == 0 {
		empty := "  No subscriptions on this topic"
		if len(sm.subscriptions) > 0 {
			empty = "  No matching subscriptions"
		}
		panel.WriteString(dimStyle.Render(empty))
		panel.WriteString("\n")
	} else {
		panel.WriteString(dimStyle.Render("  " + snsSubscriptionRow(m, "PROTOCOL", "ENDPOINT", "OWNER", "STATUS", "DLQ")))
		panel.WriteString("\n")
		visibleLines := max(m.height-13-warningLines, 5)
		start := max(sm.subIdx-visibleLines+1, 0)
		end := min(start+visibleLines, len(sm.filteredSubs))
		for i := start; i < end; i++ {
			subscription := sm.filteredSubs[i]
			cursor, style := "  ", normalStyle
			if i == sm.subIdx {
				cursor, style = "> ", selectedStyle
			}
			dlq := snsSubscriptionDLQLabel(subscription)
			row := snsSubscriptionRow(m, subscription.Protocol, subscription.Endpoint, subscription.Owner, subscription.Status(), dlq)
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterSNSSubscriptions, row)))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d subscriptions", len(sm.filteredSubs), len(sm.subscriptions))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func snsSubscriptionDLQLabel(subscription awsservice.SNSSubscription) string {
	if !subscription.Confirmed() {
		return "n/a"
	}
	if !subscription.AttributesKnown {
		return "?"
	}
	if targetARN := subscription.DeadLetterTargetARN(); targetARN != "" {
		return targetARN
	}
	if subscription.HasRedrive() {
		return "?"
	}
	return "-"
}

func snsSubscriptionRow(m Model, protocol, endpoint, owner, status, dlq string) string {
	widths := snsColumnWidths(m, []int{8, 16, 12, 9, 21}, []int{11, 42, 14, 10, 42})
	return padInspectorText(inspectorShorten(escapeTerminalControls(protocol), widths[0]), widths[0]) + "  " +
		padInspectorText(snsShortenEndpoint(escapeTerminalControls(endpoint), widths[1]), widths[1]) + "  " +
		padInspectorText(inspectorShorten(escapeTerminalControls(owner), widths[2]), widths[2]) + "  " +
		padInspectorText(inspectorShorten(escapeTerminalControls(status), widths[3]), widths[3]) + "  " +
		snsShortenTail(escapeTerminalControls(dlq), widths[4])
}

func snsShortenEndpoint(value string, width int) string {
	if strings.HasPrefix(value, "arn:") {
		return snsShortenTail(value, width)
	}
	return inspectorShorten(value, width)
}

func snsColumnWidths(m Model, minimum, desired []int) []int {
	widths := append([]int(nil), minimum...)
	if m.width <= 0 {
		return append([]int(nil), desired...)
	}

	available := max(m.width-m.currentListPanelStyle().GetHorizontalFrameSize()-2-2*(len(widths)-1), len(widths))
	total := 0
	for _, width := range widths {
		total += width
	}
	for total > available {
		for i := range widths {
			if total <= available {
				break
			}
			if widths[i] > 1 {
				widths[i]--
				total--
			}
		}
	}
	for total < available {
		grew := false
		for i := range widths {
			if total >= available {
				break
			}
			if widths[i] < desired[i] {
				widths[i]++
				total++
				grew = true
			}
		}
		if !grew {
			break
		}
	}
	return widths
}

func snsShortenTail(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 3 {
		return inspectorShorten(value, width)
	}

	target := width - 3
	runes := []rune(value)
	start, suffixWidth := len(runes), 0
	for start > 0 {
		runeWidth := lipgloss.Width(string(runes[start-1]))
		if suffixWidth+runeWidth > target {
			break
		}
		start--
		suffixWidth += runeWidth
	}
	return "..." + string(runes[start:])
}
