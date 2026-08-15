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

func ssmParamsTestModel() Model {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	m.screen = screenLoading
	return m
}

func testParameters() []awsservice.SSMParameter {
	return []awsservice.SSMParameter{
		{Name: "/app/dev/api-url", Type: "String", Tier: "Standard", Version: 1, Region: "us-east-1"},
		{Name: "/app/prod/db-password", Type: "SecureString", Tier: "Standard", Version: 3, KMSKeyID: "alias/app-key", Region: "us-east-1"},
	}
}

func TestSSMParametersLoadedOpensList(t *testing.T) {
	m := ssmParamsTestModel()
	parameters := testParameters()
	parameters[0].LastModified = time.Date(2026, 8, 1, 12, 34, 0, 0, time.UTC)

	_, _, handled := m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: parameters})
	if !handled || m.screen != screenSSMParamList {
		t.Fatalf("expected parameter list screen, got %v", m.screen)
	}
	view, ok := m.ssmParams.View(m)
	for _, want := range []string{"PATH", "TYPE", "TIER", "LAST MODIFIED", "/app/dev/api-url", "SecureString", "2026-08-01 12:34"} {
		if !ok || !strings.Contains(view, want) {
			t.Fatalf("expected aligned parameter table containing %q, got:\n%s", want, view)
		}
	}
}

func TestSSMParametersLoadedRendersEmptyList(t *testing.T) {
	m := ssmParamsTestModel()

	_, _, handled := m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{})
	view, ok := m.ssmParams.View(m)
	if !handled || !ok || m.screen != screenSSMParamList || !strings.Contains(view, "No parameters found") {
		t.Fatalf("expected empty parameter list, screen=%v view=%q", m.screen, view)
	}
}

func TestSSMParamDetailHidesValueUntilRevealed(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
	m.ssmParams.idx = 1

	m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenSSMParamDetail {
		t.Fatalf("expected detail screen, got %v", m.screen)
	}
	view, _ := m.ssmParams.View(m)
	if strings.Contains(view, "s3cret") {
		t.Fatal("value must not appear before reveal")
	}
	if !strings.Contains(view, "hidden") || !strings.Contains(view, "press v") {
		t.Fatalf("expected hidden-value hint, got:\n%s", view)
	}
}

func TestSSMParamRevealFetchesAndShowsValue(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
	m.ssmParams.idx = 1
	m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})

	// v starts the fetch...
	newM, cmd := m.ssmParams.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if cmd == nil {
		t.Fatal("expected v to start fetching the value")
	}
	m = newM.(Model)

	// ...and the loaded message reveals it.
	m.ssmParams.HandleMessage(&m, ssmParamValueLoadedMsg{name: "/app/prod/db-password", value: "s3cret", request: m.ssmParams.request})
	view, _ := m.ssmParams.View(m)
	if !strings.Contains(view, "s3cret") {
		t.Fatalf("expected revealed value, got:\n%s", view)
	}
}

func TestSSMParamRevealEscapesTerminalControlSequences(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
	m.ssmParams.idx = 1
	m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})

	value := "safe\x1b[31mred\x1b]52;c;payload\a"
	m.ssmParams.HandleMessage(&m, ssmParamValueLoadedMsg{name: "/app/prod/db-password", value: value})
	view, _ := m.ssmParams.View(m)
	if strings.Contains(view, "\x1b") || strings.Contains(view, "\a") {
		t.Fatalf("revealed value must not contain active terminal controls: %q", view)
	}
	if !strings.Contains(view, `\x1b[31m`) || !strings.Contains(view, `\x1b]52;c;payload\a`) {
		t.Fatalf("expected terminal controls to be visibly escaped: %q", view)
	}
}

func TestSSMParamDetailEscapesTerminalControlsInDescription(t *testing.T) {
	m := ssmParamsTestModel()
	parameters := testParameters()
	parameters[0].Description = "safe\x1b[31mred\x1b]52;c;payload\a"
	m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: parameters})
	m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})

	view, _ := m.ssmParams.View(m)
	if strings.Contains(view, "\x1b") || strings.Contains(view, "\a") {
		t.Fatalf("description must not contain active terminal controls: %q", view)
	}
	if !strings.Contains(view, `safe\x1b[31mred\x1b]52;c;payload\a`) {
		t.Fatalf("expected terminal controls to be visibly escaped: %q", view)
	}
}

func TestSSMParamCopyNeverRendersValue(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
	m.ssmParams.idx = 1
	m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})

	var copied string
	origCopy := ssmParamsCopyFn
	ssmParamsCopyFn = func(text string) error {
		copied = text
		return nil
	}
	defer func() { ssmParamsCopyFn = origCopy }()

	_, cmd := m.ssmParams.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("expected y to start the copy fetch")
	}
	m.ssmParams.HandleMessage(&m, ssmParamValueLoadedMsg{name: "/app/prod/db-password", value: "s3cret", copyOnly: true, request: m.ssmParams.request})
	if copied != "s3cret" {
		t.Fatalf("expected value copied to clipboard, got %q", copied)
	}
	view, _ := m.ssmParams.View(m)
	if strings.Contains(view, "s3cret") {
		t.Fatal("copy must not render the value")
	}
	if !strings.Contains(view, "Copied value to clipboard") {
		t.Fatalf("expected copy notice, got:\n%s", view)
	}
}

func TestSSMParamCopyClearsPreviouslyRevealedValue(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
	m.ssmParams.idx = 1
	m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m.ssmParams.HandleMessage(&m, ssmParamValueLoadedMsg{name: "/app/prod/db-password", value: "old", request: m.ssmParams.request})

	origCopy := ssmParamsCopyFn
	ssmParamsCopyFn = func(string) error { return nil }
	defer func() { ssmParamsCopyFn = origCopy }()

	m.ssmParams.request++
	m.ssmParams.HandleMessage(&m, ssmParamValueLoadedMsg{name: "/app/prod/db-password", value: "new", copyOnly: true, request: m.ssmParams.request})
	view, _ := m.ssmParams.View(m)
	if m.ssmParams.revealed || m.ssmParams.value != "" || strings.Contains(view, "old") {
		t.Fatalf("expected copy to clear the stale revealed value, got %+v view=%q", m.ssmParams, view)
	}
}

func TestSSMParamValueLoadIgnoresStaleSelection(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
	m.ssmParams.idx = 0
	m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})

	m.ssmParams.HandleMessage(&m, ssmParamValueLoadedMsg{name: "/app/prod/db-password", value: "s3cret"})
	if m.ssmParams.revealed {
		t.Fatal("expected a stale value load to be dropped")
	}
}

func TestSSMParamBackClearsRevealedValue(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
	m.ssmParams.idx = 1
	m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m.ssmParams.HandleMessage(&m, ssmParamValueLoadedMsg{name: "/app/prod/db-password", value: "s3cret"})

	m.ssmParams.updateDetail(&m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.ssmParams.revealed || m.ssmParams.value != "" || m.ssmParams.selected != nil {
		t.Fatalf("expected esc to clear the revealed value, got %+v", m.ssmParams)
	}
}

func TestSSMParamGlobalNavigationClearsRevealedValue(t *testing.T) {
	for _, key := range []string{"H", "C", "P"} {
		t.Run(key, func(t *testing.T) {
			m := ssmParamsTestModel()
			m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
			m.ssmParams.idx = 1
			m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
			m.ssmParams.HandleMessage(&m, ssmParamValueLoadedMsg{name: "/app/prod/db-password", value: "s3cret"})

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			m = updated.(Model)
			if m.ssmParams.revealed || m.ssmParams.value != "" {
				t.Fatalf("expected %s navigation to clear revealed value, got %+v", key, m.ssmParams)
			}
		})
	}
}

func TestSSMParamPaletteCancelRestoresUsableDetail(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
	m.ssmParams.idx = 1
	m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m.ssmParams.HandleMessage(&m, ssmParamValueLoadedMsg{name: "/app/prod/db-password", value: "s3cret"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	view, ok := m.ssmParams.View(m)
	if m.screen != screenSSMParamDetail || !ok || !strings.Contains(view, "/app/prod/db-password") {
		t.Fatalf("expected palette cancel to restore parameter detail, screen=%v view=%q", m.screen, view)
	}
	if m.ssmParams.revealed || m.ssmParams.value != "" || strings.Contains(view, "s3cret") {
		t.Fatalf("expected restored detail to keep the value cleared, got %+v", m.ssmParams)
	}
}

func TestSSMParamLateRevealDoesNotEscapePalette(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
	m.ssmParams.idx = 1
	m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})

	updated, _ := m.ssmParams.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = updated.(Model)
	request := m.ssmParams.request
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	m = updated.(Model)

	updated, _ = m.Update(ssmParamValueLoadedMsg{name: "/app/prod/db-password", value: "s3cret", request: request})
	m = updated.(Model)
	if m.screen != screenCommandPalette || m.ssmParams.revealed || m.ssmParams.value != "" {
		t.Fatalf("expected late reveal to be ignored in palette, screen=%v state=%+v", m.screen, m.ssmParams)
	}
}

func TestSSMParamLateFailureDoesNotEscapePalette(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
	m.ssmParams.idx = 1
	m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})

	updated, _ := m.ssmParams.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = updated.(Model)
	request := m.ssmParams.request
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	m = updated.(Model)

	updated, _ = m.Update(ssmParamValueLoadedMsg{name: "/app/prod/db-password", request: request, err: errors.New("fetch failed")})
	m = updated.(Model)
	if m.screen != screenCommandPalette || m.errMsg != "" {
		t.Fatalf("expected late failure to be ignored in palette, screen=%v error=%q", m.screen, m.errMsg)
	}
}

func TestSSMParamLateListDoesNotEscapePalette(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.request++
	m.ssmParams.loading = true
	request := m.ssmParams.request
	m.screen = screenLoading

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	m = updated.(Model)
	updated, _ = m.Update(ssmParametersLoadedMsg{parameters: testParameters(), request: request})
	m = updated.(Model)

	if m.screen != screenCommandPalette || len(m.ssmParams.items) != 0 {
		t.Fatalf("expected late list response to be ignored in palette, screen=%v items=%d", m.screen, len(m.ssmParams.items))
	}
}

func TestSSMParamPaletteCancelRestartsInitialLoad(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.request++
	m.ssmParams.loading = true
	m.screen = screenLoading

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.screen != screenLoading || cmd == nil || !m.ssmParams.loading {
		t.Fatalf("expected palette cancel to restart the parameter load, screen=%v loading=%v", m.screen, m.ssmParams.loading)
	}
}

func TestSSMParamPaletteCancelDuringValueLoadRestoresDetail(t *testing.T) {
	for _, key := range []rune{'v', 'y'} {
		t.Run(string(key), func(t *testing.T) {
			m := ssmParamsTestModel()
			m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
			m.ssmParams.idx = 1
			m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})

			updated, _ := m.ssmParams.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
			m = updated.(Model)
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
			m = updated.(Model)
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = updated.(Model)

			view, ok := m.ssmParams.View(m)
			if m.screen != screenSSMParamDetail || cmd != nil || !ok || !strings.Contains(view, "/app/prod/db-password") {
				t.Fatalf("expected palette cancel to restore hidden parameter detail, screen=%v view=%q", m.screen, view)
			}
			if m.ssmParams.revealed || m.ssmParams.value != "" {
				t.Fatalf("expected restored detail to keep the value hidden, got %+v", m.ssmParams)
			}
		})
	}
}

func TestSSMParamGlobalOverlayCancelRestoresInterruptedLoad(t *testing.T) {
	for _, overlay := range []rune{'S', 'V'} {
		t.Run(string(overlay)+"/list", func(t *testing.T) {
			m := ssmParamsTestModel()
			m.ssmParams.request++
			m.ssmParams.loading = true
			m.screen = screenLoading

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{overlay}})
			m = updated.(Model)
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = updated.(Model)

			if m.screen != screenLoading || cmd == nil || !m.ssmParams.loading {
				t.Fatalf("expected %c cancel to restart the parameter load, screen=%v loading=%v", overlay, m.screen, m.ssmParams.loading)
			}
		})

		for _, action := range []rune{'v', 'y'} {
			t.Run(string(overlay)+"/"+string(action), func(t *testing.T) {
				m := ssmParamsTestModel()
				m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
				m.ssmParams.idx = 1
				m.ssmParams.updateList(&m, tea.KeyMsg{Type: tea.KeyEnter})

				updated, _ := m.ssmParams.updateDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{action}})
				m = updated.(Model)
				updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{overlay}})
				m = updated.(Model)
				updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
				m = updated.(Model)

				if m.screen != screenSSMParamDetail || cmd != nil || m.ssmParams.selected == nil {
					t.Fatalf("expected %c cancel during %c to restore parameter detail, screen=%v", overlay, action, m.screen)
				}
				if m.ssmParams.revealed || m.ssmParams.value != "" {
					t.Fatalf("expected restored detail to keep the value hidden, got %+v", m.ssmParams)
				}
			})
		}
	}
}

func TestSSMParamLateListFailureDoesNotEscapePalette(t *testing.T) {
	m := ssmParamsTestModel()
	m.ssmParams.request++
	m.ssmParams.loading = true
	request := m.ssmParams.request
	m.screen = screenLoading

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	m = updated.(Model)
	updated, _ = m.Update(ssmParametersLoadedMsg{request: request, err: errors.New("list failed")})
	m = updated.(Model)

	if m.screen != screenCommandPalette || m.errMsg != "" {
		t.Fatalf("expected late list failure to be ignored in palette, screen=%v error=%q", m.screen, m.errMsg)
	}
}
