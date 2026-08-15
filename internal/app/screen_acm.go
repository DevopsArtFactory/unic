package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

type acmModel struct {
	items        []awsservice.ACMCertificate
	filtered     []awsservice.ACMCertificate
	idx          int
	selected     *awsservice.ACMCertificate
	detailScroll int
}

func newACMModel() acmModel { return acmModel{} }

func (am *acmModel) Start(m *Model) (tea.Model, tea.Cmd) { return m.startLoading(am.load(*m)) }

func (am *acmModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	loaded, ok := msg.(acmCertificatesLoadedMsg)
	if !ok {
		return *m, nil, false
	}
	am.items = loaded.certificates
	am.filtered = applyFilter(am.items, m.filterValue(filterACMCertificates))
	am.idx, am.selected, m.screen = 0, nil, screenACMCertificateList
	return *m, nil, true
}

func (am *acmModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenACMCertificateList:
		if cmd, handled := m.updateSharedFilter(msg, filterACMCertificates); handled {
			return *m, cmd, true
		}
		switch msg.String() {
		case "q", "esc":
			m.screen = screenFeatureList
			m.resetFilter(filterACMCertificates)
		case "up", "k":
			am.idx = previousListIndex(am.idx, len(am.filtered))
		case "down", "j":
			am.idx = nextListIndex(am.idx, len(am.filtered))
		case "/":
			return *m, m.activateFilter(filterACMCertificates), true
		case "r":
			m.resetFilter(filterACMCertificates)
			newM, cmd := m.startLoading(am.load(*m))
			return newM, cmd, true
		case "enter":
			if am.idx < len(am.filtered) {
				selected := am.filtered[am.idx]
				am.selected = &selected
				am.detailScroll = 0
				m.screen = screenACMCertificateDetail
			}
		}
		return *m, nil, true
	case screenACMCertificateDetail:
		lines := am.detailLines(*m)
		visibleLines := max(m.height-8, 5)
		maxOffset := max(len(lines)-visibleLines, 0)
		switch msg.String() {
		case "q", "esc":
			am.detailScroll = 0
			m.screen = screenACMCertificateList
		case "up", "k":
			am.detailScroll = max(am.detailScroll-1, 0)
		case "down", "j":
			am.detailScroll = min(am.detailScroll+1, maxOffset)
		case "pgup":
			am.detailScroll = max(am.detailScroll-visibleLines, 0)
		case "pgdown":
			am.detailScroll = min(am.detailScroll+visibleLines, maxOffset)
		}
		return *m, nil, true
	}
	return *m, nil, false
}

func (am acmModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenACMCertificateList:
		return am.viewList(m), true
	case screenACMCertificateDetail:
		return am.viewDetail(m), true
	}
	return "", false
}

func (am *acmModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterACMCertificates {
		return false
	}
	am.filtered, am.idx = applyFilter(am.items, m.filterValue(target)), 0
	return true
}

func (am acmModel) load(m Model) tea.Cmd {
	return func() tea.Msg {
		repo, err := awsservice.NewAwsRepository(m.commandContext(), m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		certificates, err := repo.ListCertificates(m.commandContext())
		if err != nil {
			return errMsg{err: err}
		}
		return acmCertificatesLoadedMsg{certificates: certificates}
	}
}

func (am acmModel) viewList(m Model) string {
	var b, panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("ACM Certificates — soonest expiry first"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterACMCertificates))
	b.WriteString("\n\n")
	if len(am.filtered) == 0 {
		panel.WriteString(dimStyle.Render("  No certificates found"))
		panel.WriteString("\n")
	} else {
		panel.WriteString(dimStyle.Render("  DOMAIN                                     STATUS         EXPIRES IN USE  RENEWAL"))
		panel.WriteString("\n")
		visibleLines := max(m.height-11, 5)
		start := max(am.idx-visibleLines+1, 0)
		for i := start; i < min(start+visibleLines, len(am.filtered)); i++ {
			cursor, style := "  ", normalStyle
			if i == am.idx {
				cursor, style = "> ", selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterACMCertificates, am.filtered[i].DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d certificates", len(am.filtered), len(am.items))))
	}
	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (am acmModel) viewDetail(m Model) string {
	if am.selected == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Certificate Detail"))
	b.WriteString("\n\n")
	lines := am.detailLines(m)
	visibleLines := max(m.height-8, 5)
	start := min(am.detailScroll, max(len(lines)-visibleLines, 0))
	for _, line := range lines[start:min(start+visibleLines, len(lines))] {
		b.WriteString(line)
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("↑/↓: scroll • pgup/pgdn: page • esc: back • H: home"))
	return b.String()
}

func (am acmModel) detailLines(m Model) []string {
	c := am.selected
	if c == nil {
		return nil
	}
	lines := []string{
		m.renderEC2DetailLine("Domain", c.DomainName),
		m.renderEC2DetailLine("Status", c.Status),
	}
	expires, daysLeft := "-", "-"
	if !c.NotAfter.IsZero() {
		expires = c.NotAfter.Format("2006-01-02 15:04:05")
		daysLeft = fmt.Sprintf("%d", c.DaysToExpiry(time.Now()))
	}
	lines = append(lines,
		m.renderEC2DetailLine("Expires", expires),
		m.renderEC2DetailLine("Days Left", daysLeft),
		m.renderEC2DetailLine("Renewal", ec2ValueOrDash(c.RenewalEligibility)),
		m.renderEC2DetailLine("ARN", c.ARN),
		m.renderEC2DetailLine("Region", c.Region),
	)
	for _, san := range c.SubjectAlternatives {
		lines = append(lines, m.renderEC2DetailLine("SAN", san))
	}
	for _, validation := range c.Validation {
		lines = append(lines, m.renderEC2DetailLine("Validation", fmt.Sprintf("%s — %s / %s", validation.Domain, validation.Method, validation.Status)))
	}
	for _, arn := range c.InUseBy {
		lines = append(lines, m.renderEC2DetailLine("In Use By", arn))
	}
	return lines
}
