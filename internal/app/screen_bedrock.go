package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"unic/internal/clipboard"
	awsservice "unic/internal/services/aws"
)

const (
	bedrockMaxCredentialAgeDays = 36600

	bedrockCreateFieldMode = iota
	bedrockCreateFieldUser
	bedrockCreateFieldExpiration
)

func (m Model) handleBedrockMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case bedrockKeysLoadedMsg:
		m.bedrockKeys = msg.keys
		m.filteredBedrockKeys = applyFilter(m.bedrockKeys, m.filterValue(filterBedrockKeys))
		m.bedrockKeyIdx = 0
		m.screen = screenBedrockKeyList
		return m, nil, true

	case bedrockCreateIdentityMsg:
		if msg.err == nil && msg.identity != nil {
			m.callerIdentity = msg.identity
		}
		m.enterBedrockCreateFlow()
		return m, nil, true

	case bedrockKeyGeneratedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil, true
		}
		m.bedrockGeneratedKey = msg.key
		m.bedrockCopyMsg = ""
		m.bedrockStatus = fmt.Sprintf("Bedrock API key %s complete", msg.action)
		m.screen = screenBedrockKeyResult
		return m, nil, true

	case bedrockKeyDeletedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil, true
		}
		m.selectedBedrockKey = nil
		m.bedrockStatus = fmt.Sprintf("Deleted Bedrock API key %s", msg.credentialID)
		newM, cmd := m.startLoading(m.loadBedrockAPIKeys())
		return newM, cmd, true
	}
	return m, nil, false
}

func (m Model) loadBedrockAPIKeys() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		keys, err := repo.ListBedrockAPIKeys(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return bedrockKeysLoadedMsg{keys: keys}
	}
}

func (m Model) loadBedrockCreateIdentity() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return bedrockCreateIdentityMsg{err: err}
			}
		}

		identity, err := repo.GetCallerIdentity(ctx)
		return bedrockCreateIdentityMsg{identity: identity, err: err}
	}
}

func (m Model) createBedrockAPIKey(userName string, ageDays int32) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return bedrockKeyGeneratedMsg{action: "generation", err: err}
			}
		}

		key, err := repo.CreateBedrockAPIKey(ctx, userName, ageDays)
		return bedrockKeyGeneratedMsg{key: key, action: "generation", err: err}
	}
}

func (m Model) rotateBedrockAPIKey(key awsservice.BedrockAPIKey) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return bedrockKeyGeneratedMsg{action: "rotation", err: err}
			}
		}

		generated, err := repo.RotateBedrockAPIKey(ctx, key.UserName, key.CredentialID)
		return bedrockKeyGeneratedMsg{key: generated, action: "rotation", err: err}
	}
}

func (m Model) deleteBedrockAPIKey(key awsservice.BedrockAPIKey) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return bedrockKeyDeletedMsg{credentialID: key.CredentialID, err: err}
			}
		}

		err := repo.DeleteBedrockAPIKey(ctx, key.UserName, key.CredentialID)
		return bedrockKeyDeletedMsg{credentialID: key.CredentialID, err: err}
	}
}

func (m Model) updateBedrockKeyList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterBedrockKeys); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.selectedBedrockKey = nil
		m.resetFilter(filterBedrockKeys)
	case "up", "k":
		if m.bedrockKeyIdx > 0 {
			m.bedrockKeyIdx--
		}
	case "down", "j":
		if m.bedrockKeyIdx < len(m.filteredBedrockKeys)-1 {
			m.bedrockKeyIdx++
		}
	case "/":
		return m, m.activateFilter(filterBedrockKeys)
	case "c":
		if m.callerIdentity == nil {
			return m.startLoadingWithMessage("Resolving caller identity...", nil, m.loadBedrockCreateIdentity())
		}
		m.enterBedrockCreateFlow()
		m.screen = screenBedrockKeyCreate
	case "enter":
		if len(m.filteredBedrockKeys) > 0 && m.bedrockKeyIdx < len(m.filteredBedrockKeys) {
			selected := m.filteredBedrockKeys[m.bedrockKeyIdx]
			m.selectedBedrockKey = &selected
			m.screen = screenBedrockKeyDetail
		}
	}
	return m, nil
}

func (m *Model) enterBedrockCreateFlow() {
	m.bedrockAction = "create"
	m.bedrockConfirm = ""
	m.bedrockCreateField = bedrockCreateFieldMode
	m.bedrockCreateInput = ""
	m.bedrockCreateMode = 0
	m.bedrockCreateValues = map[string]string{}
	m.bedrockStatus = ""
	if _, ok := m.currentIAMUserName(); !ok {
		m.bedrockCreateField = bedrockCreateFieldUser
		m.bedrockCreateMode = 1
	}
	m.screen = screenBedrockKeyCreate
}

func (m Model) updateBedrockKeyDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenBedrockKeyList
	case "r":
		if m.selectedBedrockKey != nil && m.selectedBedrockKey.Status == "Active" {
			m.bedrockAction = "rotate"
			m.bedrockConfirm = ""
			m.bedrockStatus = ""
			m.screen = screenBedrockKeyConfirm
		}
	case "d":
		if m.selectedBedrockKey != nil {
			m.bedrockAction = "delete"
			m.bedrockConfirm = ""
			m.bedrockStatus = ""
			m.screen = screenBedrockKeyConfirm
		}
	}
	return m, nil
}

func (m Model) updateBedrockKeyCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenBedrockKeyList
		m.bedrockStatus = ""
	case "up", "k":
		if m.bedrockCreateField == bedrockCreateFieldMode && m.bedrockCreateMode > 0 {
			m.bedrockCreateMode--
		}
	case "down", "j":
		if m.bedrockCreateField == bedrockCreateFieldMode && m.bedrockCreateMode < 1 {
			m.bedrockCreateMode++
		}
	case "enter":
		if m.bedrockCreateValues == nil {
			m.bedrockCreateValues = map[string]string{}
		}
		if m.bedrockCreateField == bedrockCreateFieldMode {
			if m.bedrockCreateMode == 0 {
				userName, ok := m.currentIAMUserName()
				if !ok {
					m.bedrockStatus = "Current IAM user could not be inferred for this context"
					m.bedrockCreateMode = 1
					m.bedrockCreateField = bedrockCreateFieldUser
					return m, nil
				}
				m.bedrockCreateValues["user"] = userName
				m.bedrockCreateValues["user_source"] = "current"
				m.bedrockCreateField = bedrockCreateFieldExpiration
				m.bedrockCreateInput = "30"
				m.bedrockStatus = ""
				return m, nil
			}
			m.bedrockCreateField = bedrockCreateFieldUser
			m.bedrockCreateInput = ""
			m.bedrockStatus = ""
			return m, nil
		}
		if m.bedrockCreateField == bedrockCreateFieldUser {
			userName := strings.TrimSpace(m.bedrockCreateInput)
			if userName == "" {
				m.bedrockStatus = "IAM user name is required"
				return m, nil
			}
			m.bedrockCreateValues["user"] = userName
			m.bedrockCreateValues["user_source"] = "custom"
			m.bedrockCreateField = bedrockCreateFieldExpiration
			m.bedrockCreateInput = "30"
			m.bedrockStatus = ""
			return m, nil
		}

		days := strings.TrimSpace(m.bedrockCreateInput)
		if _, err := parseBedrockAgeDays(days); err != nil {
			m.bedrockStatus = err.Error()
			return m, nil
		}
		m.bedrockCreateValues["days"] = days
		m.bedrockAction = "create"
		m.bedrockConfirm = ""
		m.bedrockStatus = ""
		m.screen = screenBedrockKeyConfirm
	case "backspace":
		if m.bedrockCreateField != bedrockCreateFieldMode && len(m.bedrockCreateInput) > 0 {
			m.bedrockCreateInput = m.bedrockCreateInput[:len(m.bedrockCreateInput)-1]
		}
	default:
		if runes := msg.Runes; m.bedrockCreateField != bedrockCreateFieldMode && len(runes) > 0 {
			m.bedrockCreateInput += string(runes)
		}
	}
	return m, nil
}

func (m Model) updateBedrockKeyConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	target := m.bedrockConfirmTarget()
	switch msg.String() {
	case "esc":
		if m.bedrockAction == "create" {
			m.screen = screenBedrockKeyCreate
		} else {
			m.screen = screenBedrockKeyDetail
		}
	case "enter":
		if target == "" || m.bedrockConfirm != target {
			return m, nil
		}
		switch m.bedrockAction {
		case "create":
			userName := m.bedrockCreateValues["user"]
			ageDays, err := parseBedrockAgeDays(m.bedrockCreateValues["days"])
			if err != nil {
				m.bedrockStatus = err.Error()
				m.screen = screenBedrockKeyCreate
				return m, nil
			}
			return m.startLoadingWithMessage("Generating Bedrock API key...", []string{"The secret will be shown once."}, m.createBedrockAPIKey(userName, ageDays))
		case "rotate":
			if m.selectedBedrockKey != nil {
				return m.startLoadingWithMessage("Rotating Bedrock API key...", []string{"The old secret will be invalidated immediately."}, m.rotateBedrockAPIKey(*m.selectedBedrockKey))
			}
		case "delete":
			if m.selectedBedrockKey != nil {
				return m.startLoadingWithMessage("Deleting Bedrock API key...", nil, m.deleteBedrockAPIKey(*m.selectedBedrockKey))
			}
		}
	case "backspace":
		if len(m.bedrockConfirm) > 0 {
			m.bedrockConfirm = m.bedrockConfirm[:len(m.bedrockConfirm)-1]
		}
	default:
		if runes := msg.Runes; len(runes) > 0 {
			m.bedrockConfirm += string(runes)
		}
	}
	return m, nil
}

func (m Model) updateBedrockKeyResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.bedrockGeneratedKey == nil {
		return m.startLoading(m.loadBedrockAPIKeys())
	}

	switch msg.String() {
	case "c":
		if err := clipboard.Copy(m.bedrockGeneratedKey.Secret); err != nil {
			m.bedrockCopyMsg = fmt.Sprintf("Clipboard error: %s", err)
		} else {
			m.bedrockCopyMsg = "Copied API key to clipboard"
		}
	case "e":
		if err := clipboard.Copy(m.bedrockGeneratedKey.EnvExport()); err != nil {
			m.bedrockCopyMsg = fmt.Sprintf("Clipboard error: %s", err)
		} else {
			m.bedrockCopyMsg = "Copied shell export to clipboard"
		}
	case "q", "esc":
		m.bedrockGeneratedKey = nil
		m.bedrockCopyMsg = ""
		return m.startLoading(m.loadBedrockAPIKeys())
	}
	return m, nil
}

func (m Model) bedrockConfirmTarget() string {
	if m.bedrockAction == "create" && m.bedrockCreateValues != nil {
		return m.bedrockCreateValues["user"]
	}
	if m.selectedBedrockKey != nil {
		return m.selectedBedrockKey.CredentialID
	}
	return ""
}

func (m Model) currentIAMUserName() (string, bool) {
	if m.callerIdentity == nil {
		return "", false
	}
	const marker = ":user/"
	idx := strings.Index(m.callerIdentity.Arn, marker)
	if idx < 0 {
		return "", false
	}
	path := strings.TrimSpace(m.callerIdentity.Arn[idx+len(marker):])
	if path == "" {
		return "", false
	}
	parts := strings.Split(path, "/")
	userName := strings.TrimSpace(parts[len(parts)-1])
	return userName, userName != ""
}

func parseBedrockAgeDays(value string) (int32, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	days, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("expiration days must be a number")
	}
	if days < 1 || days > bedrockMaxCredentialAgeDays {
		return 0, fmt.Errorf("expiration days must be 1-%d, or blank for no expiration", bedrockMaxCredentialAgeDays)
	}
	return int32(days), nil
}

func (m Model) viewBedrockKeyList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Bedrock API Keys"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterBedrockKeys))
	b.WriteString("\n\n")

	if len(m.filteredBedrockKeys) == 0 {
		panel.WriteString(dimStyle.Render("  No Bedrock API keys found"))
		panel.WriteString("\n")
	} else {
		aliasWidth := len("ALIAS")
		userWidth := len("USER")
		for _, key := range m.filteredBedrockKeys {
			alias := key.Alias
			if alias == "" {
				alias = "-"
			}
			if len(alias) > aliasWidth {
				aliasWidth = len(alias)
			}
			if len(key.UserName) > userWidth {
				userWidth = len(key.UserName)
			}
		}
		if aliasWidth > 38 {
			aliasWidth = 38
		}
		if userWidth > 24 {
			userWidth = 24
		}

		aliasCol := lipgloss.NewStyle().Width(aliasWidth + 2)
		userCol := lipgloss.NewStyle().Width(userWidth + 2)
		statusCol := lipgloss.NewStyle().Width(10)
		createdCol := lipgloss.NewStyle().Width(13)
		expiresCol := lipgloss.NewStyle().Width(13)

		panel.WriteString(dimStyle.Render(
			"  " +
				aliasCol.Render("ALIAS") +
				userCol.Render("USER") +
				statusCol.Render("STATUS") +
				createdCol.Render("CREATED") +
				expiresCol.Render("EXPIRES"),
		))
		panel.WriteString("\n")

		visibleLines := max(m.height-12, 5)
		start := 0
		if m.bedrockKeyIdx >= visibleLines {
			start = m.bedrockKeyIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredBedrockKeys))

		for i := start; i < end; i++ {
			key := m.filteredBedrockKeys[i]
			alias := key.Alias
			if alias == "" {
				alias = "-"
			}
			cursor := "  "
			style := normalStyle
			if i == m.bedrockKeyIdx {
				cursor = "> "
				style = selectedStyle
			}
			row := cursor +
				aliasCol.Render(m.renderHighlightedValue(filterBedrockKeys, alias)) +
				userCol.Render(m.renderHighlightedValue(filterBedrockKeys, key.UserName)) +
				statusCol.Render(key.Status) +
				createdCol.Render(key.CreatedDisplay()) +
				expiresCol.Render(key.ExpiresDisplay())
			panel.WriteString(style.Render(row))
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d keys", len(m.filteredBedrockKeys), len(m.bedrockKeys))))
	}

	if m.bedrockStatus != "" {
		panel.WriteString("\n")
		panel.WriteString(selectedStyle.Render("  " + m.bedrockStatus))
		panel.WriteString("\n")
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • c: create • enter: detail • esc: back • H: home"))
	return b.String()
}

func (m Model) viewBedrockKeyDetail() string {
	if m.selectedBedrockKey == nil {
		return ""
	}
	key := m.selectedBedrockKey
	alias := key.Alias
	if alias == "" {
		alias = "(unavailable)"
	}

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Bedrock API Key Detail"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Alias", normalStyle.Render(alias)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("ID", normalStyle.Render(key.CredentialID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("IAM User", normalStyle.Render(key.UserName)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Status", normalStyle.Render(key.Status)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Created", normalStyle.Render(key.CreatedDisplay())))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Expires", normalStyle.Render(key.ExpiresDisplay())))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Service", normalStyle.Render(key.ServiceName)))
	b.WriteString("\n\n")

	b.WriteString(titleStyle.Render("Actions"))
	b.WriteString("\n")
	if key.Status == "Active" {
		b.WriteString(normalStyle.Render("  [r] Rotate secret"))
	} else {
		b.WriteString(dimStyle.Render("  [r] Rotate secret"))
	}
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  [d] Delete key"))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("r: rotate • d: delete • esc: back • H: home"))
	return b.String()
}

func (m Model) viewBedrockKeyCreate() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Generate Bedrock API Key"))
	b.WriteString("\n\n")

	if m.bedrockCreateField == bedrockCreateFieldMode {
		currentUser, _ := m.currentIAMUserName()
		options := []string{
			fmt.Sprintf("Current IAM user (%s)", currentUser),
			"Another IAM user",
		}
		b.WriteString(normalStyle.Render("  Generate for:"))
		b.WriteString("\n\n")
		for i, opt := range options {
			cursor := "  "
			style := normalStyle
			if i == m.bedrockCreateMode {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("  %s%s", cursor, opt)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	} else {
		userValue := ""
		userSource := ""
		if m.bedrockCreateValues != nil {
			userValue = m.bedrockCreateValues["user"]
			userSource = m.bedrockCreateValues["user_source"]
		}

		userLine := userValue
		if m.bedrockCreateField == bedrockCreateFieldUser {
			userLine = m.bedrockCreateInput + "▏"
		}
		if userSource == "current" && userLine != "" {
			userLine += " (current)"
		}

		if m.bedrockCreateField == bedrockCreateFieldUser && userSource != "current" {
			b.WriteString(dimStyle.Render("  Current AWS identity is not an IAM user."))
			b.WriteString("\n")
			b.WriteString(dimStyle.Render("  Bedrock API keys must be generated for an IAM user."))
			b.WriteString("\n\n")
		}

		daysLine := "(next)"
		if m.bedrockCreateField == bedrockCreateFieldExpiration {
			daysLine = m.bedrockCreateInput + "▏"
		} else if m.bedrockCreateValues != nil {
			daysLine = m.bedrockCreateValues["days"]
		}
		if strings.TrimSpace(daysLine) == "" && m.bedrockCreateField != bedrockCreateFieldExpiration && userValue != "" {
			daysLine = "(never)"
		}

		b.WriteString(renderDetailLine("IAM User", normalStyle.Render(userLine)))
		b.WriteString("\n")
		if m.bedrockCreateField != bedrockCreateFieldUser {
			b.WriteString(renderDetailLine("Expiration Days", normalStyle.Render(daysLine)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if m.bedrockCreateField != bedrockCreateFieldUser {
		b.WriteString(dimStyle.Render("  Blank expiration creates a key with no expiration."))
		b.WriteString("\n")
	}
	b.WriteString(dimStyle.Render("  The generated secret is shown only once."))
	b.WriteString("\n")
	if m.bedrockStatus != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  " + m.bedrockStatus))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if m.bedrockCreateField == bedrockCreateFieldMode {
		b.WriteString(m.renderHelpBar("↑/↓: select target • enter: continue • esc: cancel"))
	} else {
		b.WriteString(m.renderHelpBar("type: edit • enter: continue • esc: cancel"))
	}
	return b.String()
}

func (m Model) viewBedrockKeyConfirm() string {
	target := m.bedrockConfirmTarget()
	targetLabel := "API key ID"
	action := m.bedrockAction
	switch m.bedrockAction {
	case "create":
		targetLabel = "IAM user name"
		action = "generate a Bedrock API key for"
	case "rotate":
		action = "rotate Bedrock API key"
	case "delete":
		action = "delete Bedrock API key"
	}

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(errorStyle.Render("Confirm Bedrock API Key Action"))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render(fmt.Sprintf("  You are about to %s:", action)))
	b.WriteString("\n")
	b.WriteString(selectedStyle.Render(fmt.Sprintf("  %s", target)))
	b.WriteString("\n\n")
	if m.bedrockAction == "rotate" {
		b.WriteString(dimStyle.Render("  Rotation immediately invalidates the previous secret."))
		b.WriteString("\n\n")
	}
	if m.bedrockAction == "create" {
		b.WriteString(dimStyle.Render("  The generated secret will be shown only once."))
		b.WriteString("\n\n")
		if m.bedrockCreateValues != nil && m.bedrockCreateValues["user_source"] == "custom" {
			b.WriteString(errorStyle.Render("  Cross-user generation is explicit; confirm the target IAM user carefully."))
			b.WriteString("\n\n")
		}
	}
	b.WriteString(normalStyle.Render(fmt.Sprintf("  Type the %s to confirm:", targetLabel)))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", m.bedrockConfirm)))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("enter: confirm • esc: cancel"))
	return b.String()
}

func (m Model) viewBedrockKeyResult() string {
	if m.bedrockGeneratedKey == nil {
		return ""
	}
	key := m.bedrockGeneratedKey
	alias := key.Alias
	if alias == "" {
		alias = "(unavailable)"
	}

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Bedrock API Key Generated"))
	b.WriteString("\n\n")

	b.WriteString(errorStyle.Render("  Store this secret now. It cannot be retrieved after leaving this screen."))
	b.WriteString("\n")
	b.WriteString(warningStyle.Render("  The secret is copy-only and is not printed to avoid terminal logs/history."))
	b.WriteString("\n\n")
	b.WriteString(renderDetailLine("Alias", normalStyle.Render(alias)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("ID", normalStyle.Render(key.CredentialID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("IAM User", normalStyle.Render(key.UserName)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Expires", normalStyle.Render(key.ExpiresDisplay())))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Secret", normalStyle.Render("[hidden] press c to copy")))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Env", normalStyle.Render("[hidden] press e to copy export")))
	b.WriteString("\n")
	if m.bedrockCopyMsg != "" {
		b.WriteString("\n")
		b.WriteString(selectedStyle.Render("  " + m.bedrockCopyMsg))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("c: copy key • e: copy env • esc: list"))
	return b.String()
}
