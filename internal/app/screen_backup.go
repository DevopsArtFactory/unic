package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

type backupModel struct {
	vaults       []awsservice.BackupVault
	filtered     []awsservice.BackupVault
	idx          int
	selected     *awsservice.BackupVault
	detail       *awsservice.BackupVaultDetail
	detailScroll int
	warnings     []error
	detailErrors []error
	errorActive  bool
}

func newBackupModel() backupModel { return backupModel{} }

func isBackupScreen(value screen) bool {
	return value == screenBackupVaultList || value == screenBackupVaultDetail
}

func resetBackupContextState(m *Model) {
	m.backup = newBackupModel()
	m.resetFilter(filterBackupVaults)
}

func normalizeBackupContextReturn(m *Model) {
	previous := &m.ctxPrevScreen
	seen := make(map[screen]struct{})
	for range 8 {
		current := *previous
		if _, ok := seen[current]; ok {
			return
		}
		seen[current] = struct{}{}
		if isBackupScreen(current) || current == screenLoading && isBackupScreen(m.loadingReturnScreen) || current == screenError && m.backup.errorActive {
			*previous = screenServiceList
			return
		}
		previous = backupOverlayPrevious(m, current)
		if previous == nil {
			return
		}
	}
}

func backupOverlayPrevious(m *Model, current screen) *screen {
	switch current {
	case screenSettings:
		return &m.settingsPrevScreen
	case screenCommandPalette:
		return &m.palette.prevScreen
	case screenViewList:
		return &m.views.prevScreen
	case screenContextPicker:
		return &m.ctxPrevScreen
	case screenRegionPicker:
		return &m.regionPrevScreen
	default:
		return nil
	}
}

func (bm *backupModel) Start(m *Model) (tea.Model, tea.Cmd) {
	bm.errorActive = false
	return m.startLoadingFor(screenBackupVaultList, "Loading AWS Backup vaults...", nil, bm.loadVaults(*m))
}

func (bm *backupModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case backupVaultsLoadedMsg:
		if msg.err != nil {
			bm.finishError(m, msg.err)
			return *m, nil, true
		}
		bm.vaults = msg.vaults
		bm.filtered = applyFilter(bm.vaults, m.filterValue(filterBackupVaults))
		bm.idx = 0
		bm.selected = nil
		bm.detail = nil
		bm.warnings = msg.warnings
		bm.detailErrors = nil
		bm.errorActive = false
		finishBackupLoad(m, screenBackupVaultList)
		return *m, nil, true
	case backupVaultDetailLoadedMsg:
		if bm.selected == nil || bm.selected.Name != msg.vaultName {
			return *m, nil, true
		}
		if msg.err != nil {
			bm.finishError(m, msg.err)
			return *m, nil, true
		}
		bm.detail = msg.detail
		bm.detailErrors = msg.warnings
		bm.detailScroll = 0
		bm.errorActive = false
		finishBackupLoad(m, screenBackupVaultDetail)
		return *m, nil, true
	}
	return *m, nil, false
}

func (bm *backupModel) finishError(m *Model, err error) {
	bm.errorActive = true
	m.errMsg = err.Error()
	m.loadingTitle = ""
	m.loadingDetails = nil
	finishBackupLoad(m, screenError)
}

func finishBackupLoad(m *Model, target screen) {
	if !isBackupScreen(m.loadingReturnScreen) {
		return
	}
	if m.ctxPickerPending && m.ctxPrevScreen == screenLoading {
		m.ctxPrevScreen = target
	}
	if m.screen == screenLoading {
		m.screen = target
		m.loadingReturnScreen = 0
		return
	}
	current := m.screen
	seen := make(map[screen]struct{})
	for range 8 {
		if _, ok := seen[current]; ok {
			return
		}
		seen[current] = struct{}{}
		previous := backupOverlayPrevious(m, current)
		if previous == nil {
			return
		}
		if *previous == screenLoading {
			*previous = target
			m.loadingReturnScreen = 0
			return
		}
		current = *previous
	}
}

func (bm *backupModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenBackupVaultList:
		updated, cmd := bm.updateList(m, msg)
		return updated, cmd, true
	case screenBackupVaultDetail:
		updated, cmd := bm.updateDetail(m, msg)
		return updated, cmd, true
	default:
		return *m, nil, false
	}
}

func (bm backupModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenBackupVaultList:
		return bm.viewList(m), true
	case screenBackupVaultDetail:
		return bm.viewDetail(m), true
	default:
		return "", false
	}
}

func (bm *backupModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterBackupVaults {
		return false
	}
	bm.filtered = applyFilter(bm.vaults, m.filterValue(target))
	bm.idx = 0
	return true
}

func (bm *backupModel) updateList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterBackupVaults); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q", "esc":
		m.resetFilter(filterBackupVaults)
		m.screen = screenFeatureList
	case "up", "k":
		bm.idx = previousListIndex(bm.idx, len(bm.filtered))
	case "down", "j":
		bm.idx = nextListIndex(bm.idx, len(bm.filtered))
	case "/":
		return *m, m.activateFilter(filterBackupVaults)
	case "r":
		return bm.Start(m)
	case "enter":
		if bm.idx < len(bm.filtered) {
			selected := bm.filtered[bm.idx]
			bm.selected = &selected
			bm.detail = nil
			bm.detailErrors = nil
			bm.errorActive = false
			return m.startLoadingFor(screenBackupVaultDetail, "Loading backup recovery posture...", []string{selected.Name}, bm.loadDetail(*m, selected))
		}
	}
	return *m, nil
}

func (bm *backupModel) updateDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := bm.detailLines(*m)
	visibleLines := max(m.height-8, 5)
	maxOffset := max(len(lines)-visibleLines, 0)
	switch msg.String() {
	case "q", "esc":
		bm.detailScroll = 0
		m.screen = screenBackupVaultList
	case "up", "k":
		bm.detailScroll = max(bm.detailScroll-1, 0)
	case "down", "j":
		bm.detailScroll = min(bm.detailScroll+1, maxOffset)
	case "pgup":
		bm.detailScroll = max(bm.detailScroll-visibleLines, 0)
	case "pgdown":
		bm.detailScroll = min(bm.detailScroll+visibleLines, maxOffset)
	case "r":
		if bm.selected != nil {
			bm.errorActive = false
			return m.startLoadingFor(screenBackupVaultDetail, "Refreshing backup recovery posture...", []string{bm.selected.Name}, bm.loadDetail(*m, *bm.selected))
		}
	}
	return *m, nil
}

func (bm backupModel) loadVaults(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return backupVaultsLoadedMsg{err: err}
			}
		}
		vaults, warnings, err := repo.ListBackupVaults(ctx)
		return backupVaultsLoadedMsg{vaults: vaults, warnings: warnings, err: err}
	}
}

func (bm backupModel) loadDetail(m Model, vault awsservice.BackupVault) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return backupVaultDetailLoadedMsg{vaultName: vault.Name, err: err}
			}
		}
		detail, warnings, err := repo.GetBackupVaultDetail(ctx, vault)
		return backupVaultDetailLoadedMsg{vaultName: vault.Name, detail: detail, warnings: warnings, err: err}
	}
}

func (bm backupModel) viewList(m Model) string {
	var b, panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("AWS Backup Vaults"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterBackupVaults))
	b.WriteString("\n")
	warningLines := 0
	if len(bm.warnings) > 0 {
		b.WriteString(m.renderWarningSummary(len(bm.warnings), "vault listing failures", bm.warnings[0].Error()))
		warningLines = 2
	}
	b.WriteString("\n")

	if len(bm.filtered) == 0 {
		message := "  No backup vaults found"
		if len(bm.vaults) > 0 {
			message = "  No matching backup vaults"
		}
		panel.WriteString(dimStyle.Render(message))
		panel.WriteString("\n")
	} else {
		panel.WriteString(dimStyle.Render("  VAULT                             STATE           POINTS  LOCK      TYPE"))
		panel.WriteString("\n")
		visibleLines := max(m.height-11-warningLines, 5)
		start := max(bm.idx-visibleLines+1, 0)
		width := max(m.width-m.currentListPanelStyle().GetHorizontalFrameSize()-2, 1)
		for i := start; i < min(start+visibleLines, len(bm.filtered)); i++ {
			cursor, style := "  ", normalStyle
			if i == bm.idx {
				cursor, style = "> ", selectedStyle
			}
			row := truncateEC2DetailValue(escapeTerminalControls(bm.filtered[i].DisplayTitle()), width)
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterBackupVaults, row)))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d vaults", len(bm.filtered), len(bm.vaults))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (bm backupModel) viewDetail(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	name := ""
	if bm.selected != nil {
		name = escapeTerminalControls(bm.selected.Name)
	}
	b.WriteString(titleStyle.Render("AWS Backup Recovery — " + name))
	b.WriteString("\n")
	warningLines := 0
	if len(bm.detailErrors) > 0 {
		b.WriteString(m.renderWarningSummary(len(bm.detailErrors), "detail lookup failures", bm.detailErrors[0].Error()))
		warningLines = 2
	}
	b.WriteString("\n")

	lines := bm.detailLines(m)
	visibleLines := max(m.height-8-warningLines, 5)
	start := min(bm.detailScroll, max(len(lines)-visibleLines, 0))
	end := min(start+visibleLines, len(lines))
	b.WriteString(m.renderListPanel(strings.Join(lines[start:end], "\n")))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (bm backupModel) detailLines(m Model) []string {
	if bm.detail == nil {
		return []string{dimStyle.Render("  No backup vault detail loaded")}
	}
	detail := bm.detail
	vault := detail.Vault
	lines := []string{
		backupDetailLine(m, "Vault", vault.Name),
		backupDetailLine(m, "State", vault.State),
		backupDetailLine(m, "Type", vault.Type),
		backupDetailLine(m, "Region", vault.Region),
		backupDetailLine(m, "Recovery Points", fmt.Sprintf("%d", vault.RecoveryPointCount)),
		backupDetailLine(m, "Encryption Key", valueOrDashApp(vault.EncryptionKeyARN)),
		backupDetailLine(m, "Key Type", valueOrDashApp(vault.EncryptionKeyType)),
		backupDetailLine(m, "Vault Lock", backupLockSummary(vault)),
		backupDetailLine(m, "ARN", vault.ARN),
		"",
		titleStyle.Render(fmt.Sprintf("  Recovery Points (%d)", len(detail.RecoveryPoints))),
	}
	if len(detail.RecoveryPoints) == 0 {
		lines = append(lines, dimStyle.Render("  None available"))
	}
	for _, point := range detail.RecoveryPoints {
		style := normalStyle
		if point.NeedsAttention() {
			style = warningStyle
		}
		lines = append(lines,
			style.Render("  "+escapeTerminalControls(valueOrDashApp(point.ResourceName))),
			backupDetailLine(m, "Type / Status", fmt.Sprintf("%s / %s", valueOrDashApp(point.ResourceType), valueOrDashApp(point.Status))),
			backupDetailLine(m, "Resource ARN", point.ResourceARN),
			backupDetailLine(m, "Created", backupTime(point.CreatedAt)),
			backupDetailLine(m, "Completed", backupTime(point.CompletedAt)),
			backupDetailLine(m, "Cold Storage", backupTime(point.MoveToColdAt)),
			backupDetailLine(m, "Expires", backupTime(point.DeleteAt)),
			backupDetailLine(m, "Size", backupRecoveryPointSize(point)),
			backupDetailLine(m, "Encrypted", fmt.Sprintf("%t", point.Encrypted)),
			backupDetailLine(m, "Source Vault", valueOrDashApp(point.SourceVaultARN)),
			backupDetailLine(m, "Recovery Point", point.ARN),
		)
		if point.StatusMessage != "" {
			lines = append(lines, backupDetailLine(m, "Reason", point.StatusMessage))
		}
	}

	lines = append(lines, "", titleStyle.Render(fmt.Sprintf("  Protected Resources (%d)", len(detail.ProtectedResources))))
	if len(detail.ProtectedResources) == 0 {
		lines = append(lines, dimStyle.Render("  None available"))
	}
	for _, resource := range detail.ProtectedResources {
		lines = append(lines,
			normalStyle.Render("  "+escapeTerminalControls(valueOrDashApp(resource.Name))),
			backupDetailLine(m, "Type", resource.Type),
			backupDetailLine(m, "Last Backup", backupTime(resource.LastBackupAt)),
			backupDetailLine(m, "Resource ARN", resource.ARN),
			backupDetailLine(m, "Recovery Point", resource.LastRecoveryPointARN),
		)
	}

	lines = append(lines, "", titleStyle.Render(fmt.Sprintf("  Recent Failed / Expired Jobs (%d)", len(detail.FailedJobs))))
	if len(detail.FailedJobs) == 0 {
		lines = append(lines, dimStyle.Render("  None in the AWS Backup 30-day job window"))
	}
	for _, job := range detail.FailedJobs {
		lines = append(lines,
			warningStyle.Render("  "+escapeTerminalControls(valueOrDashApp(job.ResourceName))),
			backupDetailLine(m, "Type / State", fmt.Sprintf("%s / %s", valueOrDashApp(job.ResourceType), valueOrDashApp(job.State))),
			backupDetailLine(m, "Created", backupTime(job.CreatedAt)),
			backupDetailLine(m, "Completed", backupTime(job.CompletedAt)),
			backupDetailLine(m, "Job ID", job.ID),
		)
		if job.StatusMessage != "" {
			lines = append(lines, backupDetailLine(m, "Reason", job.StatusMessage))
		}
	}
	return lines
}

func backupDetailLine(m Model, label, value string) string {
	return strings.TrimSuffix(m.renderEC2DetailLine(label, value), "\n")
}

func backupTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Local().Format("2006-01-02 15:04 MST")
}

func backupLockSummary(vault awsservice.BackupVault) string {
	if !vault.Locked {
		return "unlocked"
	}
	retention := "locked"
	switch {
	case vault.MinRetentionKnown && vault.MaxRetentionKnown:
		retention += fmt.Sprintf(" (%d-%d days)", vault.MinRetentionDays, vault.MaxRetentionDays)
	case vault.MinRetentionKnown:
		retention += fmt.Sprintf(" (minimum %d days)", vault.MinRetentionDays)
	case vault.MaxRetentionKnown:
		retention += fmt.Sprintf(" (maximum %d days)", vault.MaxRetentionDays)
	}
	if !vault.LockDate.IsZero() {
		retention += " lock date " + backupTime(vault.LockDate)
	}
	return retention
}

func backupRecoveryPointSize(point awsservice.BackupRecoveryPoint) string {
	if !point.SizeBytesKnown {
		return "-"
	}
	return formatBytes(point.SizeBytes)
}

func valueOrDashApp(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
