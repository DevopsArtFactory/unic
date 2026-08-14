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

var authTypes = []string{"sso", "credential", "console_login", "assume_role", "okta_saml"}

var fieldsByAuthType = map[string][]fieldDef{
	"sso": {
		{key: "name", label: "Name", required: true},
		{key: "order", label: "Display Order (optional, lower first)", required: false},
		{key: "region", label: "Default Resource Region", required: true},
		{key: "regions", label: "Other Resource Regions (optional, comma-separated)", required: false},
		{key: "sso_region", label: "SSO Login Region (optional, defaults to Region)", required: false},
		{key: "sso_start_url", label: "SSO Start URL", required: true},
		{key: "sso_account_id", label: "SSO Account ID", required: true},
		{key: "sso_role_name", label: "SSO Role Name", required: true},
	},
	"credential": {
		{key: "name", label: "Name", required: true},
		{key: "order", label: "Display Order (optional, lower first)", required: false},
		{key: "region", label: "Region", required: true},
		{key: "regions", label: "Other Resource Regions (optional, comma-separated)", required: false},
		{key: "profile", label: "Profile", required: true},
		{key: "role_arn", label: "Role ARN to chain into (optional)", required: false},
		{key: "external_id", label: "External ID (optional)", required: false},
		{key: "mfa_serial", label: "MFA Device ARN (optional)", required: false},
	},
	"console_login": {
		{key: "name", label: "Name", required: true},
		{key: "order", label: "Display Order (optional, lower first)", required: false},
		{key: "region", label: "Region", required: true},
		{key: "regions", label: "Other Resource Regions (optional, comma-separated)", required: false},
		{key: "profile", label: "Profile", required: true},
		{key: "role_arn", label: "Role ARN to chain into (optional)", required: false},
		{key: "external_id", label: "External ID (optional)", required: false},
		{key: "mfa_serial", label: "MFA Device ARN (optional)", required: false},
	},
	"assume_role": {
		{key: "name", label: "Name", required: true},
		{key: "order", label: "Display Order (optional, lower first)", required: false},
		{key: "region", label: "Region", required: true},
		{key: "regions", label: "Other Resource Regions (optional, comma-separated)", required: false},
		{key: "profile", label: "Profile", required: true},
		{key: "role_arn", label: "Role ARN", required: true},
		{key: "external_id", label: "External ID (optional)", required: false},
		{key: "mfa_serial", label: "MFA Device ARN (optional)", required: false},
	},
	// okta_saml stores only non-secret metadata; passwords and MFA secrets
	// never go into config.yaml. Runtime credential exchange is tracked in #85.
	"okta_saml": {
		{key: "name", label: "Name", required: true},
		{key: "order", label: "Display Order (optional, lower first)", required: false},
		{key: "region", label: "Region", required: true},
		{key: "regions", label: "Other Resource Regions (optional, comma-separated)", required: false},
		{key: "okta_org_url", label: "Okta Org URL (https://acme.okta.com)", required: true},
		{key: "okta_app_id", label: "Okta AWS App ID (from the app embed link)", required: true},
		{key: "role_arn", label: "Preferred Role ARN (optional)", required: false},
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
			m.addAuthIdx = previousListIndex(m.addAuthIdx, len(authTypes))
		case "down", "j":
			m.addAuthIdx = nextListIndex(m.addAuthIdx, len(authTypes))
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
		order, err := parseOptionalContextOrder(m.addValues["order"])
		if err != nil {
			return errMsg{err: err}
		}
		entry := config.ContextEntry{
			Name:  m.addValues["name"],
			Order: order,
			Auth: &config.ContextAuth{
				Type:         m.addValues["auth_type"],
				Profile:      m.addValues["profile"],
				RoleArn:      m.addValues["role_arn"],
				ExternalID:   m.addValues["external_id"],
				MFASerial:    m.addValues["mfa_serial"],
				SSOStartURL:  m.addValues["sso_start_url"],
				SSORegion:    m.addValues["sso_region"],
				SSOAccountID: m.addValues["sso_account_id"],
				SSORoleName:  m.addValues["sso_role_name"],
				OktaOrgURL:   m.addValues["okta_org_url"],
				OktaAppID:    m.addValues["okta_app_id"],
			},
			Resources: &config.ContextResources{
				DefaultRegion: m.addValues["region"],
				Regions:       parseRegionList(m.addValues["regions"]),
			},
		}
		if err := config.AddContext(m.configPath, entry); err != nil {
			return errMsg{err: err}
		}
		// Reload context list
		contexts, _ := config.Contexts(m.configPath)
		return contextsLoadedMsg{contexts: contexts}
	}
}

func parseRegionList(raw string) []string {
	parts := strings.Split(raw, ",")
	regions := make([]string, 0, len(parts))
	for _, part := range parts {
		if region := strings.TrimSpace(part); region != "" {
			regions = append(regions, region)
		}
	}
	return regions
}

func parseOptionalContextOrder(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	var order int
	if _, err := fmt.Sscanf(value, "%d", &order); err != nil || order < 0 {
		return 0, fmt.Errorf("display order must be a non-negative integer")
	}
	return order, nil
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
		b.WriteString(m.renderHelpBar("↑/↓: navigate • enter: select • esc: cancel"))
		return b.String()
	}

	// Show completed fields
	b.WriteString(dimStyle.Render(fmt.Sprintf("  auth_type: %s", m.addValues["auth_type"])))
	b.WriteString("\n")
	visibleLines := len(m.addFields)
	if m.height > 0 {
		visibleLines = max(m.height-8, 5)
	}
	end := len(m.addFields)
	if m.addStep > 0 {
		end = min(m.addFieldIdx+1, len(m.addFields))
	}
	start := max(end-visibleLines, 0)
	if start > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ... %d earlier fields", start)))
		b.WriteString("\n")
	}
	for i := start; i < end; i++ {
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
		b.WriteString(m.renderHelpBar("enter: save • esc: cancel"))
	} else if m.addStep > 0 {
		b.WriteString("\n")
		b.WriteString(m.renderHelpBar("enter: next • esc: back"))
	}

	return b.String()
}
