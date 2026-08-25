package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func backupTestVaults() []awsservice.BackupVault {
	return []awsservice.BackupVault{
		{Name: "prod", ARN: "arn:vault:prod", Region: "ap-northeast-2", State: "AVAILABLE", Type: "BACKUP_VAULT", RecoveryPointCount: 3, Locked: true, EncryptionKeyARN: "arn:kms:prod"},
		{Name: "dev\x1b]52;c;spoof\a", ARN: "arn:vault:dev", Region: "ap-northeast-2", State: "FAILED", Type: "BACKUP_VAULT"},
	}
}

func TestBackupHelpScreenTitles(t *testing.T) {
	m := New(testConfig(), "", "dev")
	for _, tc := range []struct {
		screen screen
		want   string
	}{
		{screenBackupVaultList, "AWS Backup Vaults"},
		{screenBackupVaultDetail, "AWS Backup Recovery Detail"},
	} {
		m.screen = tc.screen
		if got := m.helpScreenTitle(); got != tc.want {
			t.Errorf("helpScreenTitle() = %q, want %q", got, tc.want)
		}
	}
}

func TestBackupVaultListRendersFiltersWarningsAndEscapesControls(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20
	started, _ := m.backup.Start(&m)
	m = started.(Model)
	_, _, handled := m.backup.HandleMessage(&m, backupVaultsLoadedMsg{
		vaults: backupTestVaults(), warnings: []error{errors.New("second page denied")},
	})
	if !handled || m.screen != screenBackupVaultList {
		t.Fatalf("expected backup vault list, screen=%v handled=%v", m.screen, handled)
	}
	view := m.backup.viewList(m)
	plain := stripANSI(view)
	for _, want := range []string{"AWS Backup Vaults", "prod", "AVAILABLE", "3", "vault listing failures"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected %q in vault list, got:\n%s", want, plain)
		}
	}
	if strings.Contains(view, "\x1b]52;c;spoof\a") || !strings.Contains(plain, `\x1b]52;c;spoof\a`) {
		t.Fatalf("expected terminal controls to be escaped, got %q", view)
	}

	m.storeFilterValue(filterBackupVaults, "failed")
	m.applyFilterTarget(filterBackupVaults)
	if len(m.backup.filtered) != 1 || !strings.Contains(m.backup.filtered[0].Name, "dev") {
		t.Fatalf("expected one failed vault match, got %+v", m.backup.filtered)
	}
}

func TestBackupDrillDownRendersPartialDetailAndScrolls(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 100
	m.height = 10
	m.screen = screenBackupVaultList
	m.backup.vaults = backupTestVaults()[:1]
	m.backup.filtered = append([]awsservice.BackupVault(nil), m.backup.vaults...)

	updated, cmd := m.backup.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || m.screen != screenLoading || m.backup.selected == nil || m.backup.selected.Name != "prod" {
		t.Fatalf("expected detail load, screen=%v selected=%+v command=%v", m.screen, m.backup.selected, cmd)
	}
	now := time.Date(2026, 8, 26, 3, 0, 0, 0, time.UTC)
	detail := &awsservice.BackupVaultDetail{
		Vault: *m.backup.selected,
		RecoveryPoints: []awsservice.BackupRecoveryPoint{{
			ARN: "arn:point", ResourceName: "database\x1b[31m", ResourceType: "RDS", Status: "PARTIAL", StatusMessage: "access denied", CreatedAt: now, DeleteAt: now.Add(24 * time.Hour), SizeBytes: 2048,
		}},
		ProtectedResources: []awsservice.BackupProtectedResource{{Name: "database", Type: "RDS", ARN: "arn:db", LastBackupAt: now, LastRecoveryPointARN: "arn:point"}},
		FailedJobs:         []awsservice.BackupJob{{ID: "job-1", ResourceName: "database", ResourceType: "RDS", State: "FAILED", StatusMessage: "timeout", CreatedAt: now}},
	}
	m.backup.HandleMessage(&m, backupVaultDetailLoadedMsg{vaultName: "prod", detail: detail, warnings: []error{errors.New("protected resources denied")}})
	if m.screen != screenBackupVaultDetail {
		t.Fatalf("expected backup detail, got %v", m.screen)
	}
	initial := m.backup.viewDetail(m)
	plain := stripANSI(initial)
	for _, want := range []string{"AWS Backup Recovery", "detail lookup failures", "Recovery Points"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected %q in initial detail, got:\n%s", want, plain)
		}
	}
	if strings.Contains(initial, "\x1b[31m") {
		t.Fatalf("expected recovery point controls to be escaped, got %q", initial)
	}

	scrolled := ""
	for range 10 {
		m.backup.HandleKey(&m, tea.KeyMsg{Type: tea.KeyPgDown})
		scrolled = stripANSI(m.backup.viewDetail(m))
		if strings.Contains(scrolled, "Protected Resources") || strings.Contains(scrolled, "Failed / Expired Jobs") {
			break
		}
	}
	if m.backup.detailScroll == 0 || (!strings.Contains(scrolled, "Protected Resources") && !strings.Contains(scrolled, "Failed / Expired Jobs")) {
		t.Fatalf("expected page-down to reveal later recovery sections, scroll=%d view:\n%s", m.backup.detailScroll, scrolled)
	}
}

func TestBackupIgnoresStaleDetailLoads(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading
	m.loadingReturnScreen = screenBackupVaultDetail
	m.backup.selected = &awsservice.BackupVault{Name: "current"}
	_, _, handled := m.backup.HandleMessage(&m, backupVaultDetailLoadedMsg{
		vaultName: "stale", detail: &awsservice.BackupVaultDetail{Vault: awsservice.BackupVault{Name: "stale"}},
	})
	if !handled || m.screen != screenLoading || m.backup.detail != nil {
		t.Fatalf("expected stale detail to be ignored, screen=%v detail=%+v handled=%v", m.screen, m.backup.detail, handled)
	}
}

func TestBackupLoadCompletionStaysBehindGlobalOverlays(t *testing.T) {
	for _, tc := range []struct {
		name    string
		screen  screen
		prepare func(*Model)
		result  func(Model) screen
	}{
		{name: "settings", screen: screenSettings, prepare: func(m *Model) { m.settingsPrevScreen = screenLoading }, result: func(m Model) screen { return m.settingsPrevScreen }},
		{name: "palette", screen: screenCommandPalette, prepare: func(m *Model) { m.palette.prevScreen = screenLoading }, result: func(m Model) screen { return m.palette.prevScreen }},
		{name: "views", screen: screenViewList, prepare: func(m *Model) { m.views.prevScreen = screenLoading }, result: func(m Model) screen { return m.views.prevScreen }},
		{name: "region", screen: screenRegionPicker, prepare: func(m *Model) { m.regionPrevScreen = screenLoading }, result: func(m Model) screen { return m.regionPrevScreen }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(testConfig(), "", "dev")
			m.screen = tc.screen
			m.loadingReturnScreen = screenBackupVaultList
			tc.prepare(&m)
			m.backup.HandleMessage(&m, backupVaultsLoadedMsg{vaults: backupTestVaults()})
			if m.screen != tc.screen || tc.result(m) != screenBackupVaultList {
				t.Fatalf("expected load completion behind %s, screen=%v return=%v", tc.name, m.screen, tc.result(m))
			}
		})
	}
}

func TestBackupErrorBehindContextPickerIsClearedByContextSwitch(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenContextPicker
	m.ctxPrevScreen = screenLoading
	m.loadingReturnScreen = screenBackupVaultList
	m.backup.HandleMessage(&m, backupVaultsLoadedMsg{err: errors.New("backup denied")})
	if m.screen != screenContextPicker || m.ctxPrevScreen != screenError || !m.backup.errorActive {
		t.Fatalf("expected error behind picker, screen=%v return=%v state=%+v", m.screen, m.ctxPrevScreen, m.backup)
	}

	next := testConfig()
	next.ContextName = "next"
	updated, _ := m.Update(contextSwitchedMsg{cfg: next})
	m = updated.(Model)
	if m.screen != screenServiceList || len(m.backup.vaults) != 0 || m.backup.errorActive {
		t.Fatalf("expected context switch to clear backup state and return, screen=%v state=%+v", m.screen, m.backup)
	}
}

func TestBackupContextSwitchClearsStateAndFilter(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenContextPicker
	m.ctxPrevScreen = screenBackupVaultDetail
	m.backup.vaults = backupTestVaults()
	m.backup.selected = &m.backup.vaults[0]
	m.storeFilterValue(filterBackupVaults, "prod")

	next := &config.Config{ContextName: "next", Region: "us-east-1"}
	updated, _ := m.Update(contextSwitchedMsg{cfg: next})
	m = updated.(Model)
	if m.screen != screenServiceList || len(m.backup.vaults) != 0 || m.backup.selected != nil || m.filterValue(filterBackupVaults) != "" {
		t.Fatalf("expected clean backup state after context switch, screen=%v state=%+v filter=%q", m.screen, m.backup, m.filterValue(filterBackupVaults))
	}
}
