package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
)

type fieldDef struct {
	key      string
	label    string
	required bool
}

var authTypes = []string{"sso", "credential", "assume_role"}

var fieldsByAuthType = map[string][]fieldDef{
	"sso": {
		{key: "name", label: "Name", required: true},
		{key: "region", label: "Region", required: true},
		{key: "sso_start_url", label: "SSO Start URL", required: true},
		{key: "sso_account_id", label: "SSO Account ID", required: true},
		{key: "sso_role_name", label: "SSO Role Name", required: true},
	},
	"credential": {
		{key: "name", label: "Name", required: true},
		{key: "region", label: "Region", required: true},
		{key: "profile", label: "Profile", required: true},
	},
	"assume_role": {
		{key: "name", label: "Name", required: true},
		{key: "region", label: "Region", required: true},
		{key: "profile", label: "Profile", required: true},
		{key: "role_arn", label: "Role ARN", required: true},
		{key: "external_id", label: "External ID (optional)", required: false},
	},
}

type contextAddedMsg struct{}

func (m Model) updateContextAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	runes := msg.Runes

	// Step 0: auth_type selection
	if m.addStep == 0 {
		switch key {
		case "esc":
			return m, m.loadContexts()
		case "up", "k":
			if m.addAuthIdx > 0 {
				m.addAuthIdx--
			}
		case "down", "j":
			if m.addAuthIdx < len(authTypes)-1 {
				m.addAuthIdx++
			}
		case "enter":
			selected := authTypes[m.addAuthIdx]
			m.addValues["auth_type"] = selected
			m.addFields = fieldsByAuthType[selected]
			m.addFieldIdx = 0
			m.addInput = ""
			m.addStep = 1
		}
		return m, nil
	}

	// Step -1: confirm
	if m.addStep == -1 {
		switch key {
		case "enter":
			return m, m.saveContext()
		case "esc":
			return m, m.loadContexts()
		}
		return m, nil
	}

	// Field input steps
	switch key {
	case "esc":
		// Go back one step
		if m.addFieldIdx > 0 {
			m.addFieldIdx--
			m.addInput = m.addValues[m.addFields[m.addFieldIdx].key]
		} else {
			m.addStep = 0
			m.addInput = ""
		}
	case "enter":
		field := m.addFields[m.addFieldIdx]
		val := strings.TrimSpace(m.addInput)
		if field.required && val == "" {
			return m, nil // don't advance on empty required field
		}
		m.addValues[field.key] = val
		m.addInput = ""
		m.addFieldIdx++
		if m.addFieldIdx >= len(m.addFields) {
			m.addStep = -1 // confirm step
		}
	case "backspace":
		if len(m.addInput) > 0 {
			m.addInput = m.addInput[:len(m.addInput)-1]
		}
	default:
		if len(runes) > 0 {
			m.addInput += string(runes)
		}
	}
	return m, nil
}

func (m Model) saveContext() tea.Cmd {
	return func() tea.Msg {
		entry := config.ContextEntry{
			Name:         m.addValues["name"],
			AuthType:     m.addValues["auth_type"],
			Region:       m.addValues["region"],
			Profile:      m.addValues["profile"],
			RoleArn:      m.addValues["role_arn"],
			ExternalID:   m.addValues["external_id"],
			SSOStartURL:  m.addValues["sso_start_url"],
			SSOAccountID: m.addValues["sso_account_id"],
			SSORoleName:  m.addValues["sso_role_name"],
		}
		if err := config.AddContext(m.configPath, entry); err != nil {
			return errMsg{err: err}
		}
		// Reload context list
		contexts, _ := config.Contexts(m.configPath)
		return contextsLoadedMsg{contexts: contexts}
	}
}

func (m Model) viewContextAdd() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Add Context"))
	b.WriteString("\n\n")

	// Step 0: auth_type selection
	if m.addStep == 0 {
		b.WriteString(normalStyle.Render("  Select auth type:"))
		b.WriteString("\n\n")
		for i, at := range authTypes {
			cursor := "  "
			style := normalStyle
			if i == m.addAuthIdx {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("  %s%s", cursor, at)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  ↑/↓: navigate • enter: select • esc: cancel"))
		return b.String()
	}

	// Show completed fields
	b.WriteString(dimStyle.Render(fmt.Sprintf("  auth_type: %s", m.addValues["auth_type"])))
	b.WriteString("\n")
	for i := 0; i < len(m.addFields); i++ {
		field := m.addFields[i]
		if m.addStep == -1 || i < m.addFieldIdx {
			// Completed field
			b.WriteString(dimStyle.Render(fmt.Sprintf("  %s: %s", field.label, m.addValues[field.key])))
			b.WriteString("\n")
		} else if i == m.addFieldIdx && m.addStep > 0 {
			// Current input field
			b.WriteString("\n")
			b.WriteString(normalStyle.Render(fmt.Sprintf("  %s: ", field.label)))
			b.WriteString(filterStyle.Render(fmt.Sprintf("%s▏", m.addInput)))
			b.WriteString("\n")
			if !field.required {
				b.WriteString(dimStyle.Render("  (press enter to skip)"))
				b.WriteString("\n")
			}
			break
		}
	}

	// Confirm step
	if m.addStep == -1 {
		b.WriteString("\n")
		b.WriteString(normalStyle.Render("  Save this context?"))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("  enter: save • esc: cancel"))
	} else if m.addStep > 0 {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  enter: next • esc: back"))
	}

	return b.String()
}
