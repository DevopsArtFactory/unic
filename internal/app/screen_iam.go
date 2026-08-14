package app

import (
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

type iamModel struct {
	users            []awsservice.IAMUser
	filteredUsers    []awsservice.IAMUser
	userIdx          int
	userLoadingMore  bool
	userHasMore      bool
	userNextMarker   string
	selectedUser     *awsservice.IAMUserDetail
	keys             []awsservice.AccessKey
	keyIdx           int
	selectedKey      *awsservice.AccessKey
	rotationEnabled  bool
	rotateConfirm    string // typed input for rotate confirmation
	rotationOldKeyID string
	newKey           *awsservice.NewAccessKey
	copyMsg          string // feedback message for clipboard copy
	rotationStatus   string
	newKeyVerified   bool
	oldKeyDeleted    bool
	oldKeyInactive   bool
}

func newIAMModel() iamModel {
	return iamModel{}
}

func (im *iamModel) StartUsers(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(im.loadUsers(*m))
}

func (im *iamModel) StartKeys(m *Model, rotationEnabled bool) (tea.Model, tea.Cmd) {
	im.rotationEnabled = rotationEnabled
	return m.startLoading(im.loadKeys(*m))
}

func (im *iamModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case iamUsersLoadedMsg:
		if msg.append {
			im.users = append(im.users, msg.users...)
		} else {
			im.users = msg.users
			im.userIdx = 0
		}
		sort.Slice(im.users, func(i, j int) bool {
			return strings.ToLower(im.users[i].UserName) < strings.ToLower(im.users[j].UserName)
		})
		im.userLoadingMore = false
		im.userHasMore = msg.hasMore
		im.userNextMarker = msg.nextMarker
		im.refreshUserFilter(m)
		m.screen = screenIAMUserList
		return *m, nil, true

	case iamUserDetailLoadedMsg:
		im.selectedUser = msg.user
		m.screen = screenIAMUserDetail
		return *m, nil, true

	case iamKeysLoadedMsg:
		im.keys = msg.keys
		im.keyIdx = 0
		m.screen = screenIAMKeyList
		return *m, nil, true

	case iamKeyCreatedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return *m, nil, true
		}
		im.newKey = msg.newKey
		im.copyMsg = ""
		im.rotationStatus = ""
		im.newKeyVerified = false
		im.oldKeyInactive = false
		im.oldKeyDeleted = false
		m.screen = screenIAMKeyRotateResult
		return *m, nil, true

	case iamKeyVerifiedMsg:
		if msg.err != nil {
			im.rotationStatus = fmt.Sprintf("Verification failed: %s", msg.err)
			return *m, nil, true
		}
		im.newKeyVerified = true
		if msg.identity != nil {
			im.rotationStatus = fmt.Sprintf("Verified new key as %s", msg.identity.Arn)
		} else {
			im.rotationStatus = "Verified new key"
		}
		return *m, nil, true

	case iamKeyDeactivatedMsg:
		if msg.err != nil {
			im.rotationStatus = msg.err.Error()
			return *m, nil, true
		}
		im.oldKeyInactive = true
		im.rotationStatus = fmt.Sprintf("Old key %s marked Inactive", msg.keyID)
		return *m, nil, true

	case iamKeyDeletedMsg:
		if msg.err != nil {
			im.rotationStatus = msg.err.Error()
			return *m, nil, true
		}
		im.oldKeyDeleted = true
		im.rotationStatus = fmt.Sprintf("Old key %s deleted", msg.keyID)
		return *m, nil, true
	}
	return *m, nil, false
}

func (im *iamModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenIAMUserList:
		newM, cmd := im.updateUserList(m, msg)
		return newM, cmd, true
	case screenIAMUserDetail:
		newM, cmd := im.updateUserDetail(m, msg)
		return newM, cmd, true
	case screenIAMKeyList:
		newM, cmd := im.updateKeyList(m, msg)
		return newM, cmd, true
	case screenIAMKeyDetail:
		newM, cmd := im.updateKeyDetail(m, msg)
		return newM, cmd, true
	case screenIAMKeyRotateConfirm:
		newM, cmd := im.updateKeyRotateConfirm(m, msg)
		return newM, cmd, true
	case screenIAMKeyRotateResult:
		newM, cmd := im.updateKeyRotateResult(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (im iamModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenIAMUserList:
		return im.viewUserList(m), true
	case screenIAMUserDetail:
		return im.viewUserDetail(m), true
	case screenIAMKeyList:
		return im.viewKeyList(m), true
	case screenIAMKeyDetail:
		return im.viewKeyDetail(m), true
	case screenIAMKeyRotateConfirm:
		return im.viewKeyRotateConfirm(m), true
	case screenIAMKeyRotateResult:
		return im.viewKeyRotateResult(m), true
	default:
		return "", false
	}
}

func (im *iamModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterIAMUsers {
		return false
	}
	im.refreshUserFilter(m)
	return true
}

func (im iamModel) loadUsers(m Model) tea.Cmd {
	return im.loadUsersPage(m, "", false)
}

func (im iamModel) loadUsersPage(m Model, marker string, appendPage bool) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
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

func (im iamModel) loadAllUserSummaries(m Model, marker string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
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

func (im iamModel) loadUserDetail(m Model, userName string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
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

func (im iamModel) loadKeys(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
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

func (im iamModel) createKey(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
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

func (im iamModel) verifyKey(m Model) tea.Cmd {
	return func() tea.Msg {
		if im.newKey == nil {
			return iamKeyVerifiedMsg{err: fmt.Errorf("no new key available to verify")}
		}

		if err := auth.UpdateSharedCredentialsProfile(m.cfg.Profile, im.newKey.AccessKeyID, im.newKey.SecretAccessKey); err != nil {
			return iamKeyVerifiedMsg{err: err}
		}

		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return iamKeyVerifiedMsg{err: err}
			}
		}

		identity, err := repo.VerifyAccessKey(ctx, im.newKey)
		return iamKeyVerifiedMsg{identity: identity, err: err}
	}
}

func (im iamModel) deactivateKey(m Model, oldKeyID string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
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

func (im iamModel) deleteKey(m Model, keyID string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
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

func (im iamModel) requiresCredentialApplyBeforeDeactivate(m Model) bool {
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

func (im iamModel) canDeactivateOldKey(m Model) bool {
	if im.newKey == nil || im.rotationOldKeyID == "" || im.oldKeyInactive {
		return false
	}
	if !im.requiresCredentialApplyBeforeDeactivate(m) {
		return true
	}
	return im.newKeyVerified
}

func (im iamModel) applyActionLine(m Model) string {
	if im.requiresCredentialApplyBeforeDeactivate(m) {
		if im.newKeyVerified {
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

func (im *iamModel) updateUserList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterIAMUsers); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		im.selectedUser = nil
		m.resetFilter(filterIAMUsers)
		im.userLoadingMore = false
		im.userHasMore = false
		im.userNextMarker = ""
		im.filteredUsers = nil
		im.userIdx = 0
	case "up", "k":
		im.userIdx = previousListIndex(im.userIdx, len(im.filteredUsers))
	case "down", "j":
		im.userIdx = nextListIndex(im.userIdx, len(im.filteredUsers))
	case "/":
		filterCmd := m.activateFilter(filterIAMUsers)
		if im.userHasMore && !im.userLoadingMore {
			im.userLoadingMore = true
			return *m, tea.Batch(filterCmd, im.loadAllUserSummaries(*m, im.userNextMarker))
		}
		return *m, filterCmd
	case "enter":
		if len(im.filteredUsers) > 0 && im.userIdx < len(im.filteredUsers) {
			selected := im.filteredUsers[im.userIdx]
			return m.startLoading(im.loadUserDetail(*m, selected.UserName))
		}
	case "n":
		if im.userHasMore && !im.userLoadingMore {
			im.userLoadingMore = true
			return *m, im.loadUsersPage(*m, im.userNextMarker, true)
		}
	}
	return *m, nil
}

func (im *iamModel) refreshUserFilter(m *Model) {
	query := m.filterValue(filterIAMUsers)
	if query == "" {
		im.filteredUsers = im.users
	} else {
		im.filteredUsers = applyFilter(im.users, query)
	}

	if len(im.filteredUsers) == 0 {
		im.userIdx = 0
		return
	}
	if im.userIdx >= len(im.filteredUsers) {
		im.userIdx = len(im.filteredUsers) - 1
	}
}

func (im *iamModel) updateUserDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		im.selectedUser = nil
		m.screen = screenIAMUserList
	}
	return *m, nil
}

func (im *iamModel) updateKeyList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
		im.keyIdx = 0
	case "up", "k":
		im.keyIdx = previousListIndex(im.keyIdx, len(im.keys))
	case "down", "j":
		im.keyIdx = nextListIndex(im.keyIdx, len(im.keys))
	case "enter":
		if len(im.keys) > 0 && im.keyIdx < len(im.keys) {
			selected := im.keys[im.keyIdx]
			im.selectedKey = &selected
			m.screen = screenIAMKeyDetail
		}
	}
	return *m, nil
}

func (im *iamModel) updateKeyDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenIAMKeyList
	case "r":
		if im.rotationEnabled && im.selectedKey != nil && im.selectedKey.Status == "Active" {
			im.rotateConfirm = ""
			im.rotationOldKeyID = im.selectedKey.AccessKeyID
			im.newKey = nil
			im.copyMsg = ""
			im.rotationStatus = ""
			im.newKeyVerified = false
			im.oldKeyInactive = false
			im.oldKeyDeleted = false
			m.screen = screenIAMKeyRotateConfirm
		}
	}
	return *m, nil
}

func (im *iamModel) updateKeyRotateConfirm(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	confirmTarget := ""
	if im.selectedKey != nil {
		confirmTarget = im.selectedKey.AccessKeyID
	}

	switch msg.String() {
	case "esc":
		m.screen = screenIAMKeyDetail
	case "enter":
		if im.selectedKey != nil && im.rotateConfirm == confirmTarget {
			return m.startLoading(im.createKey(*m))
		}
	case "backspace":
		if len(im.rotateConfirm) > 0 {
			im.rotateConfirm = im.rotateConfirm[:len(im.rotateConfirm)-1]
		}
	default:
		if runes := msg.Runes; len(runes) > 0 {
			im.rotateConfirm += string(runes)
		}
	}
	return *m, nil
}

func (im *iamModel) updateKeyRotateResult(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "c":
		if im.newKey != nil {
			exportStr := fmt.Sprintf(
				"export AWS_ACCESS_KEY_ID=%s\nexport AWS_SECRET_ACCESS_KEY=%s",
				im.newKey.AccessKeyID, im.newKey.SecretAccessKey,
			)
			if err := clipboard.Copy(exportStr); err != nil {
				im.copyMsg = fmt.Sprintf("Clipboard error: %s", err)
			} else {
				im.copyMsg = "Copied to clipboard!"
			}
		}
	case "a":
		if im.requiresCredentialApplyBeforeDeactivate(*m) && im.newKey != nil && !im.newKeyVerified {
			im.rotationStatus = "Applying new key to ~/.aws/credentials and verifying..."
			return *m, im.verifyKey(*m)
		}
	case "d":
		if im.canDeactivateOldKey(*m) {
			return *m, im.deactivateKey(*m, im.rotationOldKeyID)
		}
	case "x":
		if im.oldKeyInactive && !im.oldKeyDeleted && im.rotationOldKeyID != "" {
			return *m, im.deleteKey(*m, im.rotationOldKeyID)
		}
	case "q", "esc":
		im.rotationOldKeyID = ""
		im.newKey = nil
		im.copyMsg = ""
		im.rotationStatus = ""
		im.newKeyVerified = false
		im.oldKeyInactive = false
		im.oldKeyDeleted = false
		return m.startLoading(im.loadKeys(*m))
	}
	return *m, nil
}

// --- View functions ---

func (im iamModel) viewUserList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("IAM Users"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterIAMUsers))
	b.WriteString("\n\n")

	if len(im.filteredUsers) == 0 {
		panel.WriteString(dimStyle.Render("  No matching IAM users"))
		panel.WriteString("\n")
	} else {
		maxName := len("USERNAME")
		for _, user := range im.filteredUsers {
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
		if im.userIdx >= visibleLines {
			start = im.userIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(im.filteredUsers))

		for i := start; i < end; i++ {
			user := im.filteredUsers[i]
			cursor := "  "
			style := normalStyle
			if i == im.userIdx {
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
		status := fmt.Sprintf("  %d/%d loaded IAM users", len(im.filteredUsers), len(im.users))
		if im.userHasMore {
			status += " • more available"
		}
		panel.WriteString(dimStyle.Render(status))
	}

	if im.userLoadingMore {
		panel.WriteString("\n")
		if m.isFiltering(filterIAMUsers) || m.filterValue(filterIAMUsers) != "" {
			panel.WriteString(filterStyle.Render("  Loading remaining IAM usernames for filter..."))
		} else {
			panel.WriteString(filterStyle.Render("  Loading more IAM users..."))
		}
	} else if im.userHasMore {
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

func (im iamModel) viewUserDetail(m Model) string {
	if im.selectedUser == nil {
		return ""
	}

	u := im.selectedUser
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

func (im iamModel) viewKeyList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	title := "IAM Access Keys"
	if im.rotationEnabled {
		title = "Rotate IAM Access Key"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	if im.rotationEnabled {
		b.WriteString(dimStyle.Render("  User: Current identity"))
		b.WriteString("\n\n")
	}

	if len(im.keys) == 0 {
		panel.WriteString(dimStyle.Render("  No access keys found"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if im.keyIdx >= visibleLines {
			start = im.keyIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(im.keys))

		for i := start; i < end; i++ {
			key := im.keys[i]
			cursor := "  "
			style := normalStyle
			if i == im.keyIdx {
				cursor = "> "
				style = selectedStyle
			}
			title := key.DisplayTitle()
			if key.IsAged() {
				title = errorStyle.Render(title)
				cursor = errorStyle.Render(cursor)
			}
			if i == im.keyIdx && !key.IsAged() {
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
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d keys", len(im.keys))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • enter: detail • esc: back • H: home"))
	return b.String()
}

func (im iamModel) viewKeyDetail(m Model) string {
	if im.selectedKey == nil {
		return ""
	}
	k := im.selectedKey
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
	if !im.rotationEnabled {
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

func (im iamModel) viewKeyRotateConfirm(m Model) string {
	if im.selectedKey == nil {
		return ""
	}
	k := im.selectedKey

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
	b.WriteString(filterStyle.Render(fmt.Sprintf("  %s▏", im.rotateConfirm)))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("enter: confirm • esc: cancel"))
	return b.String()
}

func (im iamModel) viewKeyRotateResult(m Model) string {
	if im.newKey == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(selectedStyle.Render("New Access Key Created"))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render("  New credentials (shown once only):"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Access Key ID", normalStyle.Render(im.newKey.AccessKeyID)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Secret Access Key", normalStyle.Render(im.newKey.SecretAccessKey)))
	b.WriteString("\n\n")

	if im.rotationOldKeyID != "" {
		oldKeyStatus := "Pending"
		if im.oldKeyInactive {
			oldKeyStatus = "Inactive"
		}
		if im.oldKeyDeleted {
			oldKeyStatus = "Deleted"
		}
		b.WriteString(renderDetailLine("Old Key", normalStyle.Render(im.rotationOldKeyID)))
		b.WriteString("\n")
		b.WriteString(renderDetailLine("Old Key Status", normalStyle.Render(oldKeyStatus)))
		b.WriteString("\n\n")
	}

	if im.copyMsg != "" {
		b.WriteString(selectedStyle.Render(fmt.Sprintf("  %s", im.copyMsg)))
		b.WriteString("\n\n")
	}

	if im.rotationStatus != "" {
		b.WriteString(selectedStyle.Render(fmt.Sprintf("  %s", im.rotationStatus)))
		b.WriteString("\n\n")
	}

	b.WriteString(titleStyle.Render("Actions"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("  [c] Copy as export commands"))
	b.WriteString("\n")
	b.WriteString(im.applyActionLine(m))
	b.WriteString("\n")
	if im.canDeactivateOldKey(m) {
		b.WriteString(normalStyle.Render("  [d] Deactivate old key"))
	} else if im.oldKeyInactive {
		b.WriteString(dimStyle.Render("  [d] Old key already inactive"))
	} else if im.requiresCredentialApplyBeforeDeactivate(m) {
		b.WriteString(dimStyle.Render("  [d] Deactivate old key (available after apply + verify)"))
	} else {
		b.WriteString(dimStyle.Render("  [d] Deactivate old key"))
	}
	b.WriteString("\n")
	if im.oldKeyInactive && !im.oldKeyDeleted {
		b.WriteString(normalStyle.Render("  [x] Delete old inactive key"))
	} else if im.oldKeyDeleted {
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
