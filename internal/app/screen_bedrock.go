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

type bedrockModel struct {
	keys         []awsservice.BedrockAPIKey
	filteredKeys []awsservice.BedrockAPIKey
	keyIdx       int
	selectedKey  *awsservice.BedrockAPIKey

	action       string // "create", "rotate", "delete"
	confirm      string
	createField  int
	createMode   int // 0=current IAM user, 1=another IAM user
	createInput  string
	createValues map[string]string
	generatedKey *awsservice.GeneratedBedrockAPIKey
	copyMsg      string
	status       string
}

func newBedrockModel() bedrockModel {
	return bedrockModel{}
}

func (bm *bedrockModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(bm.loadAPIKeys(*m))
}

func (bm *bedrockModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case bedrockKeysLoadedMsg:
		bm.keys = msg.keys
		bm.filteredKeys = applyFilter(bm.keys, m.filterValue(filterBedrockKeys))
		bm.keyIdx = 0
		m.screen = screenBedrockKeyList
		return *m, nil, true

	case bedrockCreateIdentityMsg:
		if msg.err == nil && msg.identity != nil {
			m.callerIdentity = msg.identity
		}
		bm.enterCreateFlow(m)
		return *m, nil, true

	case bedrockKeyGeneratedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return *m, nil, true
		}
		bm.generatedKey = msg.key
		bm.copyMsg = ""
		bm.status = fmt.Sprintf("Bedrock API key %s complete", msg.action)
		m.screen = screenBedrockKeyResult
		return *m, nil, true

	case bedrockKeyDeletedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return *m, nil, true
		}
		bm.selectedKey = nil
		bm.status = fmt.Sprintf("Deleted Bedrock API key %s", msg.credentialID)
		newM, cmd := m.startLoading(bm.loadAPIKeys(*m))
		return newM, cmd, true
	}
	return *m, nil, false
}

func (bm *bedrockModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenBedrockKeyList:
		newM, cmd := bm.updateList(m, msg)
		return newM, cmd, true
	case screenBedrockKeyDetail:
		newM, cmd := bm.updateDetail(m, msg)
		return newM, cmd, true
	case screenBedrockKeyCreate:
		newM, cmd := bm.updateCreate(m, msg)
		return newM, cmd, true
	case screenBedrockKeyConfirm:
		newM, cmd := bm.updateConfirm(m, msg)
		return newM, cmd, true
	case screenBedrockKeyResult:
		newM, cmd := bm.updateResult(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (bm bedrockModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenBedrockKeyList:
		return bm.viewList(m), true
	case screenBedrockKeyDetail:
		return bm.viewDetail(m), true
	case screenBedrockKeyCreate:
		return bm.viewCreate(m), true
	case screenBedrockKeyConfirm:
		return bm.viewConfirm(m), true
	case screenBedrockKeyResult:
		return bm.viewResult(m), true
	default:
		return "", false
	}
}

func (bm *bedrockModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterBedrockKeys {
		return false
	}
	bm.filteredKeys = applyFilter(bm.keys, m.filterValue(target))
	bm.keyIdx = 0
	return true
}

func (bm bedrockModel) loadAPIKeys(m Model) tea.Cmd {
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

func (bm bedrockModel) loadCreateIdentity(m Model) tea.Cmd {
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

func (bm bedrockModel) createAPIKey(m Model, userName string, ageDays int32) tea.Cmd {
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

func (bm bedrockModel) rotateAPIKey(m Model, key awsservice.BedrockAPIKey) tea.Cmd {
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

func (bm bedrockModel) deleteAPIKey(m Model, key awsservice.BedrockAPIKey) tea.Cmd {
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

func (bm *bedrockModel) updateList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterBedrockKeys); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		bm.selectedKey = nil
		m.resetFilter(filterBedrockKeys)
	case "up", "k":
		if bm.keyIdx > 0 {
			bm.keyIdx--
		}
	case "down", "j":
		if bm.keyIdx < len(bm.filteredKeys)-1 {
			bm.keyIdx++
		}
	case "/":
		return *m, m.activateFilter(filterBedrockKeys)
	case "c":
		if m.callerIdentity == nil {
			return m.startLoadingWithMessage("Resolving caller identity...", nil, bm.loadCreateIdentity(*m))
		}
		bm.enterCreateFlow(m)
		m.screen = screenBedrockKeyCreate
	case "enter":
		if len(bm.filteredKeys) > 0 && bm.keyIdx < len(bm.filteredKeys) {
			selected := bm.filteredKeys[bm.keyIdx]
			bm.selectedKey = &selected
			m.screen = screenBedrockKeyDetail
		}
	}
	return *m, nil
}

func (bm *bedrockModel) enterCreateFlow(m *Model) {
	bm.action = "create"
	bm.confirm = ""
	bm.createField = bedrockCreateFieldMode
	bm.createInput = ""
	bm.createMode = 0
	bm.createValues = map[string]string{}
	bm.status = ""
	if _, ok := currentIAMUserName(*m); !ok {
		bm.createField = bedrockCreateFieldUser
		bm.createMode = 1
	}
	m.screen = screenBedrockKeyCreate
}

func (bm *bedrockModel) updateDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenBedrockKeyList
	case "r":
		if bm.selectedKey != nil && bm.selectedKey.Status == "Active" {
			bm.action = "rotate"
			bm.confirm = ""
			bm.status = ""
			m.screen = screenBedrockKeyConfirm
		}
	case "d":
		if bm.selectedKey != nil {
			bm.action = "delete"
			bm.confirm = ""
			bm.status = ""
			m.screen = screenBedrockKeyConfirm
		}
	}
	return *m, nil
}

func (bm *bedrockModel) updateCreate(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenBedrockKeyList
		bm.status = ""
	case "up", "k":
		if bm.createField == bedrockCreateFieldMode && bm.createMode > 0 {
			bm.createMode--
		}
	case "down", "j":
		if bm.createField == bedrockCreateFieldMode && bm.createMode < 1 {
			bm.createMode++
		}
	case "enter":
		if bm.createValues == nil {
			bm.createValues = map[string]string{}
		}
		if bm.createField == bedrockCreateFieldMode {
			if bm.createMode == 0 {
				userName, ok := currentIAMUserName(*m)
				if !ok {
					bm.status = "Current IAM user could not be inferred for this context"
					bm.createMode = 1
					bm.createField = bedrockCreateFieldUser
					return *m, nil
				}
				bm.createValues["user"] = userName
				bm.createValues["user_source"] = "current"
				bm.createField = bedrockCreateFieldExpiration
				bm.createInput = "30"
				bm.status = ""
				return *m, nil
			}
			bm.createField = bedrockCreateFieldUser
			bm.createInput = ""
			bm.status = ""
			return *m, nil
		}
		if bm.createField == bedrockCreateFieldUser {
			userName := strings.TrimSpace(bm.createInput)
			if userName == "" {
				bm.status = "IAM user name is required"
				return *m, nil
			}
			bm.createValues["user"] = userName
			bm.createValues["user_source"] = "custom"
			bm.createField = bedrockCreateFieldExpiration
			bm.createInput = "30"
			bm.status = ""
			return *m, nil
		}

		days := strings.TrimSpace(bm.createInput)
		if _, err := parseBedrockAgeDays(days); err != nil {
			bm.status = err.Error()
			return *m, nil
		}
		bm.createValues["days"] = days
		bm.action = "create"
		bm.confirm = ""
		bm.status = ""
		m.screen = screenBedrockKeyConfirm
	case "backspace":
		if bm.createField != bedrockCreateFieldMode && len(bm.createInput) > 0 {
			bm.createInput = bm.createInput[:len(bm.createInput)-1]
		}
	default:
		if runes := msg.Runes; bm.createField != bedrockCreateFieldMode && len(runes) > 0 {
			bm.createInput += string(runes)
		}
	}
	return *m, nil
}

func (bm *bedrockModel) updateConfirm(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	target := bm.confirmTarget()
	switch msg.String() {
	case "esc":
		if bm.action == "create" {
			m.screen = screenBedrockKeyCreate
		} else {
			m.screen = screenBedrockKeyDetail
		}
	case "enter":
		if target == "" || bm.confirm != target {
			return *m, nil
		}
		switch bm.action {
		case "create":
			userName := bm.createValues["user"]
			ageDays, err := parseBedrockAgeDays(bm.createValues["days"])
			if err != nil {
				bm.status = err.Error()
				m.screen = screenBedrockKeyCreate
				return *m, nil
			}
			return m.startLoadingWithMessage("Generating Bedrock API key...", []string{"The secret will be shown once."}, bm.createAPIKey(*m, userName, ageDays))
		case "rotate":
			if bm.selectedKey != nil {
				return m.startLoadingWithMessage("Rotating Bedrock API key...", []string{"The old secret will be invalidated immediately."}, bm.rotateAPIKey(*m, *bm.selectedKey))
			}
		case "delete":
			if bm.selectedKey != nil {
				return m.startLoadingWithMessage("Deleting Bedrock API key...", nil, bm.deleteAPIKey(*m, *bm.selectedKey))
			}
		}
	case "backspace":
		if len(bm.confirm) > 0 {
			bm.confirm = bm.confirm[:len(bm.confirm)-1]
		}
	default:
		if runes := msg.Runes; len(runes) > 0 {
			bm.confirm += string(runes)
		}
	}
	return *m, nil
}

func (bm *bedrockModel) updateResult(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if bm.generatedKey == nil {
		return m.startLoading(bm.loadAPIKeys(*m))
	}

	switch msg.String() {
	case "c":
		if err := clipboard.Copy(bm.generatedKey.Secret); err != nil {
			bm.copyMsg = fmt.Sprintf("Clipboard error: %s", err)
		} else {
			bm.copyMsg = "Copied API key to clipboard"
		}
	case "e":
		if err := clipboard.Copy(bm.generatedKey.EnvExport()); err != nil {
			bm.copyMsg = fmt.Sprintf("Clipboard error: %s", err)
		} else {
			bm.copyMsg = "Copied shell export to clipboard"
		}
	case "q", "esc":
		bm.generatedKey = nil
		bm.copyMsg = ""
		return m.startLoading(bm.loadAPIKeys(*m))
	}
	return *m, nil
}

func (bm bedrockModel) confirmTarget() string {
	if bm.action == "create" && bm.createValues != nil {
		return bm.createValues["user"]
	}
	if bm.selectedKey != nil {
		return bm.selectedKey.CredentialID
	}
	return ""
}

func currentIAMUserName(m Model) (string, bool) {
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
	if days < 0 || days > bedrockMaxCredentialAgeDays {
		return 0, fmt.Errorf("expiration days must be 1-%d, or 0/blank for no expiration", bedrockMaxCredentialAgeDays)
	}
	return int32(days), nil
}

func (bm bedrockModel) viewList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Bedrock API Keys"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterBedrockKeys))
	b.WriteString("\n\n")

	if len(bm.filteredKeys) == 0 {
		panel.WriteString(dimStyle.Render("  No Bedrock API keys found"))
		panel.WriteString("\n")
	} else {
		aliasWidth := len("ALIAS")
		userWidth := len("USER")
		for _, key := range bm.filteredKeys {
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
		if bm.keyIdx >= visibleLines {
			start = bm.keyIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(bm.filteredKeys))

		for i := start; i < end; i++ {
			key := bm.filteredKeys[i]
			alias := key.Alias
			if alias == "" {
				alias = "-"
			}
			cursor := "  "
			style := normalStyle
			if i == bm.keyIdx {
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
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d keys", len(bm.filteredKeys), len(bm.keys))))
	}

	if bm.status != "" {
		panel.WriteString("\n")
		panel.WriteString(selectedStyle.Render("  " + bm.status))
		panel.WriteString("\n")
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • c: create • enter: detail • esc: back • H: home"))
	return b.String()
}

func (bm bedrockModel) viewDetail(m Model) string {
	if bm.selectedKey == nil {
		return ""
	}
	key := bm.selectedKey
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

func (bm bedrockModel) viewCreate(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Generate Bedrock API Key"))
	b.WriteString("\n\n")

	if bm.createField == bedrockCreateFieldMode {
		currentUser, _ := currentIAMUserName(m)
		options := []string{
			fmt.Sprintf("Current IAM user (%s)", currentUser),
			"Another IAM user",
		}
		b.WriteString(normalStyle.Render("  Generate for:"))
		b.WriteString("\n\n")
		for i, opt := range options {
			cursor := "  "
			style := normalStyle
			if i == bm.createMode {
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
		if bm.createValues != nil {
			userValue = bm.createValues["user"]
			userSource = bm.createValues["user_source"]
		}

		userLine := userValue
		if bm.createField == bedrockCreateFieldUser {
			userLine = bm.createInput + "▏"
		}
		if userSource == "current" && userLine != "" {
			userLine += " (current)"
		}

		if bm.createField == bedrockCreateFieldUser && userSource != "current" {
			b.WriteString(dimStyle.Render("  Current AWS identity is not an IAM user."))
			b.WriteString("\n")
			b.WriteString(dimStyle.Render("  Bedrock API keys must be generated for an IAM user."))
			b.WriteString("\n\n")
		}

		daysLine := "(next)"
		if bm.createField == bedrockCreateFieldExpiration {
			daysLine = bm.createInput + "▏"
		} else if bm.createValues != nil {
			daysLine = bm.createValues["days"]
		}
		if strings.TrimSpace(daysLine) == "" && bm.createField != bedrockCreateFieldExpiration && userValue != "" {
			daysLine = "(never)"
		}

		b.WriteString(renderDetailLine("IAM User", normalStyle.Render(userLine)))
		b.WriteString("\n")
		if bm.createField != bedrockCreateFieldUser {
			b.WriteString(renderDetailLine("Expiration Days", normalStyle.Render(daysLine)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if bm.createField != bedrockCreateFieldUser {
		b.WriteString(dimStyle.Render("  Blank or 0 expiration creates a key with no expiration."))
		b.WriteString("\n")
	}
	b.WriteString(dimStyle.Render("  The generated secret is shown only once."))
	b.WriteString("\n")
	if bm.status != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  " + bm.status))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if bm.createField == bedrockCreateFieldMode {
		b.WriteString(m.renderHelpBar("↑/↓: select target • enter: continue • esc: cancel"))
	} else {
		b.WriteString(m.renderHelpBar("type: edit • enter: continue • esc: cancel"))
	}
	return b.String()
}

func (bm bedrockModel) viewConfirm(m Model) string {
	target := bm.confirmTarget()
	targetLabel := "API key ID"
	action := bm.action
	switch bm.action {
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
	if bm.action == "rotate" {
		b.WriteString(dimStyle.Render("  Rotation immediately invalidates the previous secret."))
		b.WriteString("\n\n")
	}
	if bm.action == "create" {
		b.WriteString(dimStyle.Render("  The generated secret will be shown only once."))
		b.WriteString("\n\n")
		if bm.createValues != nil && bm.createValues["user_source"] == "custom" {
			b.WriteString(errorStyle.Render("  Cross-user generation is explicit; confirm the target IAM user carefully."))
			b.WriteString("\n\n")
		}
	}
	b.WriteString(normalStyle.Render(fmt.Sprintf("  Type the %s to confirm:", targetLabel)))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", bm.confirm)))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("enter: confirm • esc: cancel"))
	return b.String()
}

func (bm bedrockModel) viewResult(m Model) string {
	if bm.generatedKey == nil {
		return ""
	}
	key := bm.generatedKey
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
	if bm.copyMsg != "" {
		b.WriteString("\n")
		b.WriteString(selectedStyle.Render("  " + bm.copyMsg))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("c: copy key • e: copy env • esc: list"))
	return b.String()
}
