package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	awsservice "unic/internal/services/aws"
)

type kmsModel struct {
	items, filtered []awsservice.KMSKey
	idx             int
	selected        *awsservice.KMSKey
	warnings        []error
}

func newKMSModel() kmsModel                              { return kmsModel{} }
func (km *kmsModel) Start(m *Model) (tea.Model, tea.Cmd) { return m.startLoading(km.load(*m)) }
func (km kmsModel) load(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return kmsKeysLoadedMsg{err: err}
		}
		keys, warnings, err := repo.ListKMSKeys(ctx)
		return kmsKeysLoadedMsg{keys: keys, warnings: warnings, err: err}
	}
}
func (km *kmsModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	loaded, ok := msg.(kmsKeysLoadedMsg)
	if !ok {
		return *m, nil, false
	}
	if loaded.err != nil {
		nm, cmd := m.Update(errMsg{err: loaded.err})
		return nm, cmd, true
	}
	km.items = loaded.keys
	km.warnings = loaded.warnings
	km.filtered = applyFilter(km.items, m.filterValue(filterKMSKeys))
	km.idx = 0
	km.selected = nil
	m.screen = screenKMSKeyList
	return *m, nil, true
}
func (km *kmsModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenKMSKeyList:
		if cmd, ok := m.updateSharedFilter(msg, filterKMSKeys); ok {
			return *m, cmd, true
		}
		switch msg.String() {
		case "q", "esc":
			m.screen = screenFeatureList
			m.resetFilter(filterKMSKeys)
		case "up", "k":
			km.idx = previousListIndex(km.idx, len(km.filtered))
		case "down", "j":
			km.idx = nextListIndex(km.idx, len(km.filtered))
		case "/":
			return *m, m.activateFilter(filterKMSKeys), true
		case "r":
			model, cmd := m.startLoading(km.load(*m))
			return model, cmd, true
		case "enter":
			if len(km.filtered) > 0 && km.idx < len(km.filtered) {
				v := km.filtered[km.idx]
				km.selected = &v
				m.screen = screenKMSKeyDetail
			}
		}
		return *m, nil, true
	case screenKMSKeyDetail:
		if msg.String() == "q" || msg.String() == "esc" {
			km.selected = nil
			m.screen = screenKMSKeyList
		}
		return *m, nil, true
	}
	return *m, nil, false
}
func (km kmsModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenKMSKeyList:
		return km.viewList(m), true
	case screenKMSKeyDetail:
		return km.viewDetail(m), true
	}
	return "", false
}
func (km *kmsModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterKMSKeys {
		return false
	}
	km.filtered = applyFilter(km.items, m.filterValue(target))
	km.idx = 0
	return true
}
func (km kmsModel) viewList(m Model) string {
	var b, p strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("KMS Keys"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterKMSKeys))
	b.WriteString("\n")
	warningLines := 0
	if len(km.warnings) > 0 {
		b.WriteString(m.renderWarningSummary(len(km.warnings), "resource lookup failures", km.warnings[0].Error()))
		warningLines = 2
	}
	b.WriteString("\n")
	if len(km.filtered) == 0 {
		p.WriteString(dimStyle.Render("  No KMS keys found\n"))
	} else {
		idCol := lipgloss.NewStyle().Width(38).MaxWidth(38)
		stateCol := lipgloss.NewStyle().Width(16).MaxWidth(16)
		p.WriteString(dimStyle.Render("  " + idCol.Render("KEY ID / ALIAS") + " " + stateCol.Render("STATE") + " MANAGER  ROTATION\n"))
		visibleLines := max(m.height-11-warningLines, 5)
		start := 0
		if km.idx >= visibleLines {
			start = km.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(km.filtered))
		for i := start; i < end; i++ {
			key := km.filtered[i]
			name := key.ID
			if len(key.Aliases) > 0 {
				name = key.Aliases[0]
			}
			rotation := "n/a"
			if key.RotationEligible {
				rotation = "unknown"
				if key.RotationKnown {
					rotation = fmt.Sprintf("%t", key.RotationEnabled)
				}
			}
			row := idCol.Render(name) + " " + stateCol.Render(key.State) + " " + lipgloss.NewStyle().Width(8).Render(key.Manager) + " " + rotation
			cursor := "  "
			style := normalStyle
			if i == km.idx {
				cursor = "> "
				style = selectedStyle
			}
			p.WriteString(style.Render(cursor + m.renderHighlightedValue(filterKMSKeys, row)))
			p.WriteString("\n")
		}
	}
	b.WriteString(m.renderListPanel(p.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}
func (km kmsModel) viewDetail(m Model) string {
	if km.selected == nil {
		return ""
	}
	k := km.selected
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("KMS Key Detail"))
	b.WriteString("\n\n")
	b.WriteString(m.renderEC2DetailLine("Key ID", k.ID))
	b.WriteString(m.renderEC2DetailLine("ARN", k.ARN))
	b.WriteString(m.renderEC2DetailLine("State", k.State))
	b.WriteString(m.renderEC2DetailLine("Manager", k.Manager))
	b.WriteString(m.renderEC2DetailLine("Origin", k.Origin))
	rotation := "Not eligible"
	if k.RotationEligible {
		rotation = "Unknown"
		if k.RotationKnown {
			rotation = fmt.Sprintf("%t", k.RotationEnabled)
		}
	}
	b.WriteString(m.renderEC2DetailLine("Rotation Enabled", rotation))
	b.WriteString(m.renderEC2DetailLine("Aliases", ec2ValueOrDash(strings.Join(k.Aliases, ", "))))
	if k.Description != "" {
		b.WriteString(m.renderEC2DetailLine("Description", k.Description))
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}
