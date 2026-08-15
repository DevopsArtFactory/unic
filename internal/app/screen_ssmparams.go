package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/clipboard"
	awsservice "unic/internal/services/aws"
)

// The Parameter Store browser mirrors the Secrets browser, with one hard
// rule: parameter values — SecureString or not — are never fetched or
// rendered implicitly. `v` reveals the value on screen, `y` copies it to the
// clipboard without ever printing it; everything else shows metadata only.

type ssmParamsModel struct {
	items    []awsservice.SSMParameter
	filtered []awsservice.SSMParameter
	idx      int
	selected *awsservice.SSMParameter
	revealed bool
	value    string
	notice   string
	request  int
}

// ssmParamsCopyFn is swapped out in tests to observe copies without a real
// clipboard.
var ssmParamsCopyFn = clipboard.Copy

func newSSMParamsModel() ssmParamsModel {
	return ssmParamsModel{}
}

func (pm *ssmParamsModel) clearValue() {
	pm.request++
	pm.revealed = false
	pm.value = ""
	pm.notice = ""
}

func (pm *ssmParamsModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(pm.loadParameters(*m))
}

func (pm *ssmParamsModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case ssmParametersLoadedMsg:
		pm.items = msg.parameters
		pm.filtered = applyFilter(pm.items, m.filterValue(filterSSMParameters))
		pm.idx = 0
		pm.selected = nil
		m.screen = screenSSMParamList
		return *m, nil, true
	case ssmParamValueLoadedMsg:
		if pm.selected == nil || pm.selected.Name != msg.name || pm.request != msg.request {
			return *m, nil, true
		}
		if msg.err != nil {
			pm.clearValue()
			newM, cmd := m.Update(errMsg{err: msg.err})
			return newM, cmd, true
		}
		if msg.copyOnly {
			// The value goes straight to the clipboard, never the screen.
			if err := ssmParamsCopyFn(msg.value); err != nil {
				pm.notice = fmt.Sprintf("Copy failed: %v", err)
			} else {
				pm.notice = "Copied value to clipboard"
			}
		} else {
			pm.revealed = true
			pm.value = msg.value
			pm.notice = ""
		}
		m.screen = screenSSMParamDetail
		return *m, nil, true
	}
	return *m, nil, false
}

func (pm *ssmParamsModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenSSMParamList:
		newM, cmd := pm.updateList(m, msg)
		return newM, cmd, true
	case screenSSMParamDetail:
		newM, cmd := pm.updateDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (pm ssmParamsModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenSSMParamList:
		return pm.viewList(m), true
	case screenSSMParamDetail:
		return pm.viewDetail(m), true
	default:
		return "", false
	}
}

func (pm *ssmParamsModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterSSMParameters {
		return false
	}
	pm.filtered = applyFilter(pm.items, m.filterValue(target))
	pm.idx = 0
	return true
}

func (pm *ssmParamsModel) updateList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterSSMParameters); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterSSMParameters)
	case "up", "k":
		pm.idx = previousListIndex(pm.idx, len(pm.filtered))
	case "down", "j":
		pm.idx = nextListIndex(pm.idx, len(pm.filtered))
	case "/":
		return *m, m.activateFilter(filterSSMParameters)
	case "r":
		m.resetFilter(filterSSMParameters)
		return m.startLoading(pm.loadParameters(*m))
	case "enter":
		if len(pm.filtered) > 0 && pm.idx < len(pm.filtered) {
			selected := pm.filtered[pm.idx]
			pm.selected = &selected
			pm.revealed = false
			pm.value = ""
			pm.notice = ""
			m.screen = screenSSMParamDetail
		}
	}
	return *m, nil
}

func (pm *ssmParamsModel) updateDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		pm.clearValue()
		pm.selected = nil
		m.screen = screenSSMParamList
	case "v":
		if pm.selected != nil && !pm.revealed {
			pm.request++
			return m.startLoadingWithMessage("Fetching value...", []string{pm.selected.Name}, pm.loadValue(*m, pm.selected.Name, false, pm.request))
		}
	case "y":
		if pm.selected != nil {
			pm.request++
			return m.startLoadingWithMessage("Copying value...", []string{pm.selected.Name}, pm.loadValue(*m, pm.selected.Name, true, pm.request))
		}
	}
	return *m, nil
}

func (pm ssmParamsModel) loadParameters(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		parameters, err := repo.ListParameters(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return ssmParametersLoadedMsg{parameters: parameters}
	}
}

func (pm ssmParamsModel) loadValue(m Model, name string, copyOnly bool, request int) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return ssmParamValueLoadedMsg{name: name, request: request, err: err}
		}
		value, err := repo.GetParameterValue(ctx, name)
		if err != nil {
			return ssmParamValueLoadedMsg{name: name, request: request, err: err}
		}
		return ssmParamValueLoadedMsg{name: name, value: value, copyOnly: copyOnly, request: request}
	}
}

func (pm ssmParamsModel) viewList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Parameter Store"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterSSMParameters))
	b.WriteString("\n\n")

	if len(pm.filtered) == 0 {
		emptyText := "  No parameters found"
		if len(pm.items) > 0 {
			emptyText = "  No matching parameters"
		}
		panel.WriteString(dimStyle.Render(emptyText))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if pm.idx >= visibleLines {
			start = pm.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(pm.filtered))
		for i := start; i < end; i++ {
			param := pm.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == pm.idx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterSSMParameters, param.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d parameters", len(pm.filtered), len(pm.items))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (pm ssmParamsModel) viewDetail(m Model) string {
	if pm.selected == nil {
		return ""
	}
	param := pm.selected
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Parameter Detail"))
	b.WriteString("\n\n")

	b.WriteString(m.renderEC2DetailLine("Name", param.Name))
	b.WriteString(m.renderEC2DetailLine("Type", param.Type))
	b.WriteString(m.renderEC2DetailLine("Tier", ec2ValueOrDash(param.Tier)))
	b.WriteString(m.renderEC2DetailLine("Version", fmt.Sprintf("%d", param.Version)))
	if param.IsSecure() {
		kmsKey := param.KMSKeyID
		if kmsKey == "" {
			kmsKey = "(alias/aws/ssm)"
		}
		b.WriteString(m.renderEC2DetailLine("KMS Key", kmsKey))
	}
	if param.Description != "" {
		b.WriteString(m.renderEC2DetailLine("Description", param.Description))
	}
	modified := "-"
	if !param.LastModified.IsZero() {
		modified = param.LastModified.Format("2006-01-02 15:04:05")
	}
	b.WriteString(m.renderEC2DetailLine("Last Modified", modified))
	b.WriteString(m.renderEC2DetailLine("Region", ec2ValueOrDash(param.Region)))

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Value"))
	b.WriteString("\n")
	if pm.revealed {
		b.WriteString(normalStyle.Render("  " + strconv.QuoteToGraphic(pm.value)))
		b.WriteString("\n")
	} else {
		hidden := "  (hidden — press v to reveal, y to copy without revealing)"
		if param.IsSecure() {
			hidden = "  (SecureString, hidden — press v to decrypt and reveal, y to copy without revealing)"
		}
		b.WriteString(dimStyle.Render(hidden))
		b.WriteString("\n")
	}

	if pm.notice != "" {
		b.WriteString("\n")
		b.WriteString(selectedStyle.Render("  " + pm.notice))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}
