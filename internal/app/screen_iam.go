package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"unic/internal/auth"
	"unic/internal/clipboard"
	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func (m Model) handleIAMMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case iamKeysLoadedMsg:
		m.iamKeys = msg.keys
		m.iamKeyIdx = 0
		m.screen = screenIAMKeyList
		return m, nil, true

	case iamKeyCreatedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil, true
		}
		m.iamNewKey = msg.newKey
		m.iamCopyMsg = ""
		m.iamRotationStatus = ""
		m.iamNewKeyVerified = false
		m.iamOldKeyInactive = false
		m.iamOldKeyDeleted = false
		m.screen = screenIAMKeyRotateResult
		return m, nil, true

	case iamKeyVerifiedMsg:
		if msg.err != nil {
			m.iamRotationStatus = fmt.Sprintf("Verification failed: %s", msg.err)
			return m, nil, true
		}
		m.iamNewKeyVerified = true
		if msg.identity != nil {
			m.iamRotationStatus = fmt.Sprintf("Verified new key as %s", msg.identity.Arn)
		} else {
			m.iamRotationStatus = "Verified new key"
		}
		return m, nil, true

	case iamKeyDeactivatedMsg:
		if msg.err != nil {
			m.iamRotationStatus = msg.err.Error()
			return m, nil, true
		}
		m.iamOldKeyInactive = true
		m.iamRotationStatus = fmt.Sprintf("Old key %s marked Inactive", msg.keyID)
		return m, nil, true

	case iamKeyDeletedMsg:
		if msg.err != nil {
			m.iamRotationStatus = msg.err.Error()
			return m, nil, true
		}
		m.iamOldKeyDeleted = true
		m.iamRotationStatus = fmt.Sprintf("Old key %s deleted", msg.keyID)
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) loadIAMKeys() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		m.awsRepo = repo

		keys, err := repo.ListAccessKeys(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(keys) == 0 {
			return errMsg{err: fmt.Errorf("no access keys found")}
		}
		return iamKeysLoadedMsg{keys: keys}
	}
}

func (m Model) createIAMKey() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return iamKeyCreatedMsg{err: err}
			}
		}

		newKey, err := repo.CreateAccessKey(ctx)
		return iamKeyCreatedMsg{newKey: newKey, err: err}
	}
}

func (m Model) verifyIAMKey() tea.Cmd {
	return func() tea.Msg {
		if m.iamNewKey == nil {
			return iamKeyVerifiedMsg{err: fmt.Errorf("no new key available to verify")}
		}

		if err := auth.UpdateSharedCredentialsProfile(m.cfg.Profile, m.iamNewKey.AccessKeyID, m.iamNewKey.SecretAccessKey); err != nil {
			return iamKeyVerifiedMsg{err: err}
		}

		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return iamKeyVerifiedMsg{err: err}
			}
		}

		identity, err := repo.VerifyAccessKey(ctx, m.iamNewKey)
		return iamKeyVerifiedMsg{identity: identity, err: err}
	}
}

func (m Model) deactivateIAMKey(oldKeyID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return iamKeyDeactivatedMsg{keyID: oldKeyID, err: err}
			}
		}

		err := repo.DeactivateAccessKey(ctx, oldKeyID)
		return iamKeyDeactivatedMsg{keyID: oldKeyID, err: err}
	}
}

func (m Model) deleteIAMKey(keyID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return iamKeyDeletedMsg{keyID: keyID, err: err}
			}
		}

		err := repo.DeleteAccessKey(ctx, keyID)
		return iamKeyDeletedMsg{keyID: keyID, err: err}
	}
}

func (m Model) requiresIAMCredentialApplyBeforeDeactivate() bool {
	if m.cfg == nil {
		return false
	}

	if m.cfg.AuthType == config.AuthTypeCredential {
		return true
	}

	// Legacy contexts may omit auth_type even though they are profile-based
	// shared credentials sessions. Keep the safer handoff path available there too.
	return m.cfg.AuthType == config.AuthTypeDefault &&
		m.cfg.RoleArn == "" &&
		m.cfg.SSOStartURL == "" &&
		m.cfg.SSOAccountID == "" &&
		m.cfg.SSORoleName == ""
}

func (m Model) canDeactivateIAMOldKey() bool {
	if m.iamNewKey == nil || m.iamRotationOldKeyID == "" || m.iamOldKeyInactive {
		return false
	}
	if !m.requiresIAMCredentialApplyBeforeDeactivate() {
		return true
	}
	return m.iamNewKeyVerified
}

func (m Model) iamApplyActionLine() string {
	if m.requiresIAMCredentialApplyBeforeDeactivate() {
		if m.iamNewKeyVerified {
			return selectedStyle.Render("  [a] Applied to ~/.aws/credentials and verified")
		}
		return normalStyle.Render("  [a] Apply to ~/.aws/credentials and verify")
	}
	authType := "default"
	if m.cfg != nil && m.cfg.AuthType != "" {
		authType = string(m.cfg.AuthType)
	}
	return dimStyle.Render(fmt.Sprintf("  [a] Apply to ~/.aws/credentials and verify (disabled for auth:%s)", authType))
}

func (m Model) updateIAMKeyList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		m.iamKeyIdx = 0
	case "up", "k":
		if m.iamKeyIdx > 0 {
			m.iamKeyIdx--
		}
	case "down", "j":
		if m.iamKeyIdx < len(m.iamKeys)-1 {
			m.iamKeyIdx++
		}
	case "enter":
		if len(m.iamKeys) > 0 && m.iamKeyIdx < len(m.iamKeys) {
			selected := m.iamKeys[m.iamKeyIdx]
			m.selectedIAMKey = &selected
			m.screen = screenIAMKeyDetail
		}
	}
	return m, nil
}

func (m Model) updateIAMKeyDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenIAMKeyList
	case "r":
		if m.iamRotationEnabled && m.selectedIAMKey != nil && m.selectedIAMKey.Status == "Active" {
			m.iamRotateConfirm = ""
			m.iamRotationOldKeyID = m.selectedIAMKey.AccessKeyID
			m.iamNewKey = nil
			m.iamCopyMsg = ""
			m.iamRotationStatus = ""
			m.iamNewKeyVerified = false
			m.iamOldKeyInactive = false
			m.iamOldKeyDeleted = false
			m.screen = screenIAMKeyRotateConfirm
		}
	}
	return m, nil
}

func (m Model) updateIAMKeyRotateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	confirmTarget := ""
	if m.selectedIAMKey != nil {
		confirmTarget = m.selectedIAMKey.AccessKeyID
	}

	switch msg.String() {
	case "esc":
		m.screen = screenIAMKeyDetail
	case "enter":
		if m.selectedIAMKey != nil && m.iamRotateConfirm == confirmTarget {
			m.screen = screenLoading
			return m, m.createIAMKey()
		}
	case "backspace":
		if len(m.iamRotateConfirm) > 0 {
			m.iamRotateConfirm = m.iamRotateConfirm[:len(m.iamRotateConfirm)-1]
		}
	default:
		if runes := msg.Runes; len(runes) > 0 {
			m.iamRotateConfirm += string(runes)
		}
	}
	return m, nil
}

func (m Model) updateIAMKeyRotateResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "c":
		if m.iamNewKey != nil {
			exportStr := fmt.Sprintf(
				"export AWS_ACCESS_KEY_ID=%s\nexport AWS_SECRET_ACCESS_KEY=%s",
				m.iamNewKey.AccessKeyID, m.iamNewKey.SecretAccessKey,
			)
			if err := clipboard.Copy(exportStr); err != nil {
				m.iamCopyMsg = fmt.Sprintf("Clipboard error: %s", err)
			} else {
				m.iamCopyMsg = "Copied to clipboard!"
			}
		}
	case "a":
		if m.requiresIAMCredentialApplyBeforeDeactivate() && m.iamNewKey != nil && !m.iamNewKeyVerified {
			m.iamRotationStatus = "Applying new key to ~/.aws/credentials and verifying..."
			return m, m.verifyIAMKey()
		}
	case "d":
		if m.canDeactivateIAMOldKey() {
			return m, m.deactivateIAMKey(m.iamRotationOldKeyID)
		}
	case "x":
		if m.iamOldKeyInactive && !m.iamOldKeyDeleted && m.iamRotationOldKeyID != "" {
			return m, m.deleteIAMKey(m.iamRotationOldKeyID)
		}
	case "q", "esc":
		m.iamRotationOldKeyID = ""
		m.iamNewKey = nil
		m.iamCopyMsg = ""
		m.iamRotationStatus = ""
		m.iamNewKeyVerified = false
		m.iamOldKeyInactive = false
		m.iamOldKeyDeleted = false
		m.screen = screenLoading
		return m, m.loadIAMKeys()
	}
	return m, nil
}

// --- View functions ---

func (m Model) viewIAMKeyList() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	title := "IAM Access Keys"
	if m.iamRotationEnabled {
		title = "Rotate IAM Access Key"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	if m.iamRotationEnabled {
		b.WriteString(dimStyle.Render("  User: Current identity"))
		b.WriteString("\n\n")
	}

	if len(m.iamKeys) == 0 {
		b.WriteString(dimStyle.Render("  No access keys found"))
		b.WriteString("\n")
	} else {
		visibleLines := max(m.height-8, 5)
		start := 0
		if m.iamKeyIdx >= visibleLines {
			start = m.iamKeyIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.iamKeys))

		for i := start; i < end; i++ {
			key := m.iamKeys[i]
			cursor := "  "
			style := normalStyle
			if i == m.iamKeyIdx {
				cursor = "> "
				style = selectedStyle
			}
			title := key.DisplayTitle()
			if key.IsAged() {
				title = errorStyle.Render(title)
				cursor = errorStyle.Render(cursor)
			}
			if i == m.iamKeyIdx && !key.IsAged() {
				title = style.Render(fmt.Sprintf("%s%s", cursor, key.DisplayTitle()))
			} else if key.IsAged() {
				title = fmt.Sprintf("%s%s", cursor, title)
			} else {
				title = fmt.Sprintf("%s%s", cursor, key.DisplayTitle())
			}
			b.WriteString(title)
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d keys", len(m.iamKeys))))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓: navigate • enter: detail • esc: back • H: home"))
	return b.String()
}

func (m Model) viewIAMKeyDetail() string {
	if m.selectedIAMKey == nil {
		return ""
	}
	k := m.selectedIAMKey
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Access Key Detail"))
	b.WriteString("\n\n")

	labelStyle := lipgloss.NewStyle().Width(16)
	b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("Access Key ID"), k.AccessKeyID)))
	b.WriteString("\n")

	statusStr := k.Status
	if k.Status == "Active" {
		statusStr = selectedStyle.Render(k.Status)
	} else {
		statusStr = dimStyle.Render(k.Status)
	}
	b.WriteString(fmt.Sprintf("  %s%s", labelStyle.Render("Status"), statusStr))
	b.WriteString("\n")

	b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("Created"), k.CreateDate.Format(time.DateOnly))))
	b.WriteString("\n")

	ageStr := fmt.Sprintf("%d days", k.Age())
	if k.IsAged() {
		ageStr = errorStyle.Render(fmt.Sprintf("%d days ⚠ (>90 days)", k.Age()))
	}
	b.WriteString(fmt.Sprintf("  %s%s", labelStyle.Render("Age"), ageStr))
	b.WriteString("\n")

	lastUsed := dimStyle.Render("Never")
	if !k.LastUsed.IsZero() {
		lastUsed = k.LastUsed.Format(time.DateOnly)
	}
	b.WriteString(fmt.Sprintf("  %s%s", labelStyle.Render("Last Used"), lastUsed))
	b.WriteString("\n")

	if k.ServiceName != "" && k.ServiceName != "N/A" {
		b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("Last Service"), k.ServiceName)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Actions"))
	b.WriteString("\n")
	if !m.iamRotationEnabled {
		b.WriteString(dimStyle.Render("  Rotation is available from the RotateAccessKey feature"))
		b.WriteString("\n")
	} else if k.Status == "Active" {
		b.WriteString(normalStyle.Render("  [r] Rotate key (create new → verify/apply → deactivate)"))
		b.WriteString("\n")
	} else {
		b.WriteString(dimStyle.Render("  [r] Rotate key (inactive — cannot rotate)"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("esc: back • H: home"))
	return b.String()
}

func (m Model) viewIAMKeyRotateConfirm() string {
	if m.selectedIAMKey == nil {
		return ""
	}
	k := m.selectedIAMKey

	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(errorStyle.Render("Confirm Key Rotation"))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render("  You are about to rotate access key:"))
	b.WriteString("\n")
	b.WriteString(selectedStyle.Render(fmt.Sprintf("  %s", k.AccessKeyID)))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("  This will:"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  1. Create a new access key"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  2. Let you verify and apply the new key"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  3. Deactivate the old key only when you confirm"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("  Type the access key ID to confirm:"))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", m.iamRotateConfirm)))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  enter: confirm • esc: cancel"))
	return b.String()
}

func (m Model) viewIAMKeyRotateResult() string {
	if m.iamNewKey == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(selectedStyle.Render("New Access Key Created"))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render("  New credentials (shown once only):"))
	b.WriteString("\n\n")

	labelStyle := lipgloss.NewStyle().Width(22)
	b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("Access Key ID"), m.iamNewKey.AccessKeyID)))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("Secret Access Key"), m.iamNewKey.SecretAccessKey)))
	b.WriteString("\n\n")

	if m.iamRotationOldKeyID != "" {
		oldKeyStatus := "Pending"
		if m.iamOldKeyInactive {
			oldKeyStatus = "Inactive"
		}
		if m.iamOldKeyDeleted {
			oldKeyStatus = "Deleted"
		}
		b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("Old Key"), m.iamRotationOldKeyID)))
		b.WriteString("\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  %s%s", labelStyle.Render("Old Key Status"), oldKeyStatus)))
		b.WriteString("\n\n")
	}

	if m.iamCopyMsg != "" {
		b.WriteString(selectedStyle.Render(fmt.Sprintf("  %s", m.iamCopyMsg)))
		b.WriteString("\n\n")
	}

	if m.iamRotationStatus != "" {
		b.WriteString(selectedStyle.Render(fmt.Sprintf("  %s", m.iamRotationStatus)))
		b.WriteString("\n\n")
	}

	b.WriteString(titleStyle.Render("Actions"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  [c] Copy as export commands"))
	b.WriteString("\n")
	b.WriteString(m.iamApplyActionLine())
	b.WriteString("\n")
	if m.canDeactivateIAMOldKey() {
		b.WriteString(normalStyle.Render("  [d] Deactivate old key"))
	} else if m.iamOldKeyInactive {
		b.WriteString(dimStyle.Render("  [d] Old key already inactive"))
	} else if m.requiresIAMCredentialApplyBeforeDeactivate() {
		b.WriteString(dimStyle.Render("  [d] Deactivate old key (available after apply + verify)"))
	} else {
		b.WriteString(dimStyle.Render("  [d] Deactivate old key"))
	}
	b.WriteString("\n")
	if m.iamOldKeyInactive && !m.iamOldKeyDeleted {
		b.WriteString(normalStyle.Render("  [x] Delete old inactive key"))
	} else if m.iamOldKeyDeleted {
		b.WriteString(dimStyle.Render("  [x] Old key already deleted"))
	} else {
		b.WriteString(dimStyle.Render("  [x] Delete old key (available after deactivation)"))
	}
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  esc: back to key list"))
	return b.String()
}
