package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"unic/internal/auth"
	"unic/internal/clipboard"
	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

const iamUserPageSize = 25
const iamUserFilterPageSize = 100

func (m Model) handleIAMMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case iamUsersLoadedMsg:
		if msg.append {
			m.iamUsers = append(m.iamUsers, msg.users...)
		} else {
			m.iamUsers = msg.users
			m.iamUserIdx = 0
		}
		sort.Slice(m.iamUsers, func(i, j int) bool {
			return strings.ToLower(m.iamUsers[i].UserName) < strings.ToLower(m.iamUsers[j].UserName)
		})
		m.iamUserLoadingMore = false
		m.iamUserHasMore = msg.hasMore
		m.iamUserNextMarker = msg.nextMarker
		m.refreshIAMUserFilter()
		m.screen = screenIAMUserList
		return m, nil, true

	case iamUserDetailLoadedMsg:
		m.selectedIAMUser = msg.user
		m.screen = screenIAMUserDetail
		return m, nil, true

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

func (m Model) loadIAMUsers() tea.Cmd {
	return m.loadIAMUsersPage("", false)
}

func (m Model) loadIAMUsersPage(marker string, appendPage bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
			m.awsRepo = repo
		}

		page, err := repo.ListIAMUserSummariesPage(ctx, marker, iamUserPageSize)
		if err != nil {
			return errMsg{err: err}
		}
		if len(page.Users) == 0 && !appendPage {
			return errMsg{err: fmt.Errorf("no IAM users found")}
		}
		return iamUsersLoadedMsg{
			users:      page.Users,
			append:     appendPage,
			hasMore:    page.HasMore,
			nextMarker: page.NextMarker,
		}
	}
}

func (m Model) loadAllIAMUserSummaries(marker string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
			m.awsRepo = repo
		}

		var users []awsservice.IAMUser
		nextMarker := marker
		for nextMarker != "" {
			page, err := repo.ListIAMUserSummariesPage(ctx, nextMarker, iamUserFilterPageSize)
			if err != nil {
				return errMsg{err: err}
			}
			users = append(users, page.Users...)
			if !page.HasMore || page.NextMarker == "" {
				nextMarker = ""
				break
			}
			nextMarker = page.NextMarker
		}

		return iamUsersLoadedMsg{
			users:      users,
			append:     true,
			hasMore:    false,
			nextMarker: "",
		}
	}
}

func (m Model) loadIAMUserDetail(userName string) tea.Cmd {
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

		user, err := repo.GetIAMUserDetail(ctx, userName)
		if err != nil {
			return errMsg{err: err}
		}
		return iamUserDetailLoadedMsg{user: user}
	}
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

func (m Model) updateIAMUserList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterIAMUsers); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.selectedIAMUser = nil
		m.resetFilter(filterIAMUsers)
		m.iamUserLoadingMore = false
		m.iamUserHasMore = false
		m.iamUserNextMarker = ""
		m.filteredIAMUsers = nil
		m.iamUserIdx = 0
	case "up", "k":
		if m.iamUserIdx > 0 {
			m.iamUserIdx--
		}
	case "down", "j":
		if m.iamUserIdx < len(m.filteredIAMUsers)-1 {
			m.iamUserIdx++
		}
	case "/":
		filterCmd := m.activateFilter(filterIAMUsers)
		if m.iamUserHasMore && !m.iamUserLoadingMore {
			m.iamUserLoadingMore = true
			return m, tea.Batch(filterCmd, m.loadAllIAMUserSummaries(m.iamUserNextMarker))
		}
		return m, filterCmd
	case "enter":
		if len(m.filteredIAMUsers) > 0 && m.iamUserIdx < len(m.filteredIAMUsers) {
			selected := m.filteredIAMUsers[m.iamUserIdx]
			return m.startLoading(m.loadIAMUserDetail(selected.UserName))
		}
	case "n":
		if m.iamUserHasMore && !m.iamUserLoadingMore {
			m.iamUserLoadingMore = true
			return m, m.loadIAMUsersPage(m.iamUserNextMarker, true)
		}
	}
	return m, nil
}

func (m *Model) refreshIAMUserFilter() {
	query := m.filterValue(filterIAMUsers)
	if query == "" {
		m.filteredIAMUsers = m.iamUsers
	} else {
		m.filteredIAMUsers = applyFilter(m.iamUsers, query)
	}

	if len(m.filteredIAMUsers) == 0 {
		m.iamUserIdx = 0
		return
	}
	if m.iamUserIdx >= len(m.filteredIAMUsers) {
		m.iamUserIdx = len(m.filteredIAMUsers) - 1
	}
}

func (m Model) updateIAMUserDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.selectedIAMUser = nil
		m.screen = screenIAMUserList
	}
	return m, nil
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
			return m.startLoading(m.createIAMKey())
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
		return m.startLoading(m.loadIAMKeys())
	}
	return m, nil
}

// --- View functions ---

func (m Model) viewIAMUserList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("IAM Users"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterIAMUsers))
	b.WriteString("\n\n")

	if len(m.filteredIAMUsers) == 0 {
		panel.WriteString(dimStyle.Render("  No matching IAM users"))
		panel.WriteString("\n")
	} else {
		maxName := len("USERNAME")
		for _, user := range m.filteredIAMUsers {
			if len(user.UserName) > maxName {
				maxName = len(user.UserName)
			}
		}

		nameCol := lipgloss.NewStyle().Width(maxName + 2)
		createdCol := lipgloss.NewStyle().Width(12)
		pathCol := lipgloss.NewStyle().Width(24)

		panel.WriteString(dimStyle.Render(
			"  " +
				nameCol.Render("USERNAME") +
				createdCol.Render("CREATED") +
				"PATH",
		))
		panel.WriteString("\n")

		visibleLines := max(m.height-11, 5)
		start := 0
		if m.iamUserIdx >= visibleLines {
			start = m.iamUserIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredIAMUsers))

		for i := start; i < end; i++ {
			user := m.filteredIAMUsers[i]
			cursor := "  "
			style := normalStyle
			if i == m.iamUserIdx {
				cursor = "> "
				style = selectedStyle
			}

			row := cursor +
				nameCol.Inherit(style).Render(m.renderHighlightedValue(filterIAMUsers, user.UserName)) +
				createdCol.Inherit(dimStyle).Render(user.CreateDate.Format(time.DateOnly)) +
				pathCol.Inherit(dimStyle).Render(m.renderHighlightedValue(filterIAMUsers, truncateIAMPath(user.Path)))
			panel.WriteString(row)
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		status := fmt.Sprintf("  %d/%d loaded IAM users", len(m.filteredIAMUsers), len(m.iamUsers))
		if m.iamUserHasMore {
			status += " • more available"
		}
		panel.WriteString(dimStyle.Render(status))
	}

	if m.iamUserLoadingMore {
		panel.WriteString("\n")
		if m.isFiltering(filterIAMUsers) || m.filterValue(filterIAMUsers) != "" {
			panel.WriteString(filterStyle.Render("  Loading remaining IAM usernames for filter..."))
		} else {
			panel.WriteString(filterStyle.Render("  Loading more IAM users..."))
		}
	} else if m.iamUserHasMore {
		panel.WriteString("\n")
		if m.isFiltering(filterIAMUsers) || m.filterValue(filterIAMUsers) != "" {
			panel.WriteString(dimStyle.Render("  Continue typing to filter loaded usernames"))
		} else {
			panel.WriteString(dimStyle.Render("  Press n to load the next page"))
		}
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • n: next page • enter: detail • esc: back • H: home"))
	return b.String()
}

func (m Model) viewIAMUserDetail() string {
	if m.selectedIAMUser == nil {
		return ""
	}

	u := m.selectedIAMUser
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("IAM User Detail"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("User Name", normalStyle.Render(u.UserName)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("User ID", normalStyle.Render(u.UserID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("ARN", normalStyle.Render(u.ARN)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Path", normalStyle.Render(u.Path)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Created", normalStyle.Render(u.CreateDate.Format(time.DateOnly))))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Console Last Used", normalStyle.Render(u.PasswordLastUsedDisplay())))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Last Activity", normalStyle.Render(u.LastActivityDisplay())))
	b.WriteString("\n")

	mfaText := dimStyle.Render("Disabled")
	if u.MFAEnabled {
		mfaText = selectedStyle.Render("Enabled")
	}
	b.WriteString(renderDetailLine("MFA", mfaText))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Access Keys", normalStyle.Render(fmt.Sprintf("%d", len(u.AccessKeys)))))
	b.WriteString("\n\n")

	b.WriteString(titleStyle.Render("Groups"))
	b.WriteString("\n")
	b.WriteString(renderIAMTextList(u.Groups))
	b.WriteString("\n\n")

	b.WriteString(titleStyle.Render("Attached Policies"))
	b.WriteString("\n")
	b.WriteString(renderIAMTextList(u.AttachedPolicies))
	b.WriteString("\n\n")

	b.WriteString(titleStyle.Render("Access Keys"))
	b.WriteString("\n")
	b.WriteString(renderIAMAccessKeyList(u.AccessKeys))
	b.WriteString("\n\n")

	b.WriteString(m.renderHelpBar("esc: back • H: home"))
	return b.String()
}

func (m Model) viewIAMKeyList() string {
	var b strings.Builder
	var panel strings.Builder
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
		panel.WriteString(dimStyle.Render("  No access keys found"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
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
			panel.WriteString(title)
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d keys", len(m.iamKeys))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • enter: detail • esc: back • H: home"))
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

	b.WriteString(renderDetailLine("Access Key ID", normalStyle.Render(k.AccessKeyID)))
	b.WriteString("\n")

	statusStr := k.Status
	if k.Status == "Active" {
		statusStr = selectedStyle.Render(k.Status)
	} else {
		statusStr = dimStyle.Render(k.Status)
	}
	b.WriteString(renderDetailLine("Status", statusStr))
	b.WriteString("\n")

	b.WriteString(renderDetailLine("Created", normalStyle.Render(k.CreateDate.Format(time.DateOnly))))
	b.WriteString("\n")

	ageStr := fmt.Sprintf("%d days", k.Age())
	if k.IsAged() {
		ageStr = errorStyle.Render(fmt.Sprintf("%d days ⚠ (>90 days)", k.Age()))
	}
	b.WriteString(renderDetailLine("Age", ageStr))
	b.WriteString("\n")

	lastUsed := dimStyle.Render("Never")
	if !k.LastUsed.IsZero() {
		lastUsed = k.LastUsed.Format(time.DateOnly)
	}
	b.WriteString(renderDetailLine("Last Used", lastUsed))
	b.WriteString("\n")

	if k.ServiceName != "" && k.ServiceName != "N/A" {
		b.WriteString(renderDetailLine("Last Service", normalStyle.Render(k.ServiceName)))
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
	b.WriteString(m.renderHelpBar("esc: back • H: home"))
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
	b.WriteString(m.renderHelpBar("enter: confirm • esc: cancel"))
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

	b.WriteString(renderDetailLine("Access Key ID", normalStyle.Render(m.iamNewKey.AccessKeyID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Secret Access Key", normalStyle.Render(m.iamNewKey.SecretAccessKey)))
	b.WriteString("\n\n")

	if m.iamRotationOldKeyID != "" {
		oldKeyStatus := "Pending"
		if m.iamOldKeyInactive {
			oldKeyStatus = "Inactive"
		}
		if m.iamOldKeyDeleted {
			oldKeyStatus = "Deleted"
		}
		b.WriteString(renderDetailLine("Old Key", normalStyle.Render(m.iamRotationOldKeyID)))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Old Key Status", normalStyle.Render(oldKeyStatus)))
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
	b.WriteString(m.renderHelpBar("esc: back to key list"))
	return b.String()
}

func renderIAMTextList(items []string) string {
	if len(items) == 0 {
		return dimStyle.Render("  None")
	}

	var b strings.Builder
	for _, item := range items {
		b.WriteString(normalStyle.Render(fmt.Sprintf("  - %s", item)))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderIAMAccessKeyList(keys []awsservice.AccessKey) string {
	if len(keys) == 0 {
		return dimStyle.Render("  None")
	}

	var b strings.Builder
	for _, key := range keys {
		lastUsed := "never"
		if !key.LastUsed.IsZero() {
			lastUsed = key.LastUsed.Format(time.DateOnly)
		}
		line := fmt.Sprintf("  %s [%s] created:%s last:%s",
			key.AccessKeyID, key.Status, key.CreateDate.Format(time.DateOnly), lastUsed)
		if key.IsAged() {
			b.WriteString(errorStyle.Render(line))
		} else {
			b.WriteString(normalStyle.Render(line))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func truncateIAMPath(path string) string {
	if len(path) <= 22 {
		return path
	}
	return path[:19] + "..."
}
