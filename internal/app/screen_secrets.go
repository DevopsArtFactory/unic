package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

type secretsModel struct {
	items    []awsservice.Secret
	filtered []awsservice.Secret
	idx      int
	selected *awsservice.SecretDetail
}

func newSecretsModel() secretsModel {
	return secretsModel{}
}

func (sm *secretsModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(sm.loadSecrets(*m))
}

func (sm *secretsModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case secretsLoadedMsg:
		sm.items = msg.secrets
		sm.filtered = applyFilter(sm.items, m.filterValue(filterSecrets))
		sm.idx = 0
		m.screen = screenSecretList
		return *m, nil, true
	case secretDetailLoadedMsg:
		sm.selected = msg.detail
		m.screen = screenSecretDetail
		return *m, nil, true
	}
	return *m, nil, false
}

func (sm *secretsModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenSecretList:
		newM, cmd := sm.updateList(m, msg)
		return newM, cmd, true
	case screenSecretDetail:
		newM, cmd := sm.updateDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (sm secretsModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenSecretList:
		return sm.viewList(m), true
	case screenSecretDetail:
		return sm.viewDetail(m), true
	default:
		return "", false
	}
}

func (sm *secretsModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterSecrets {
		return false
	}
	sm.filtered = applyFilter(sm.items, m.filterValue(target))
	sm.idx = 0
	return true
}

func (sm secretsModel) loadSecrets(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		secrets, err := repo.ListSecrets(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(secrets) == 0 {
			return errMsg{err: fmt.Errorf("no secrets found")}
		}
		return secretsLoadedMsg{secrets: secrets}
	}
}

func (sm secretsModel) loadSecretDetail(m Model, name string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}
		detail, err := repo.GetSecretDetail(ctx, name)
		if err != nil {
			return errMsg{err: err}
		}
		return secretDetailLoadedMsg{detail: detail}
	}
}

func (sm *secretsModel) updateList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterSecrets); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterSecrets)
	case "up", "k":
		sm.idx = previousListIndex(sm.idx, len(sm.filtered))
	case "down", "j":
		sm.idx = nextListIndex(sm.idx, len(sm.filtered))
	case "/":
		return *m, m.activateFilter(filterSecrets)
	case "enter":
		if len(sm.filtered) > 0 && sm.idx < len(sm.filtered) {
			selected := sm.filtered[sm.idx]
			return m.startLoading(sm.loadSecretDetail(*m, selected.Name))
		}
	}
	return *m, nil
}

func (sm *secretsModel) updateDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		sm.selected = nil
		m.screen = screenSecretList
	}
	return *m, nil
}

func (sm secretsModel) viewList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Secrets Manager"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterSecrets))
	b.WriteString("\n\n")

	if len(sm.filtered) == 0 {
		panel.WriteString(dimStyle.Render("  No matching secrets"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if sm.idx >= visibleLines {
			start = sm.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(sm.filtered))

		for i := start; i < end; i++ {
			s := sm.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == sm.idx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterSecrets, s.DisplayTitle())))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d secrets", len(sm.filtered), len(sm.items))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: detail • esc: back • H: home"))
	return b.String()
}

func (sm secretsModel) viewDetail(m Model) string {
	if sm.selected == nil {
		return ""
	}
	d := sm.selected
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Secret Detail"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Name", normalStyle.Render(d.Name)))
	b.WriteString("\n")

	kmsKey := d.KMSKeyID
	if kmsKey == "" {
		kmsKey = dimStyle.Render("(aws/secretsmanager)")
	}
	b.WriteString(renderDetailLine("Encryption Key", kmsKey))
	b.WriteString("\n\n")

	if len(d.Values) > 0 {
		b.WriteString(titleStyle.Render("Key / Value"))
		b.WriteString("\n\n")

		keys := make([]string, 0, len(d.Values))
		for k := range d.Values {
			keys = append(keys, k)
		}
		for i := 1; i < len(keys); i++ {
			for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
				keys[j], keys[j-1] = keys[j-1], keys[j]
			}
		}

		for _, k := range keys {
			b.WriteString(fmt.Sprintf("  %s  %s\n", dimStyle.Render(k), normalStyle.Render(d.Values[k])))
		}
	} else if d.Raw != "" {
		b.WriteString(titleStyle.Render("Value"))
		b.WriteString("\n\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  %s", d.Raw)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("esc: back • H: home"))
	return b.String()
}
