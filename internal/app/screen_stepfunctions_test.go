package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func stepFunctionsTestStateMachines() []awsservice.StepFunctionStateMachine {
	return []awsservice.StepFunctionStateMachine{
		{ARN: "arn:standard", Name: "orders", Type: "STANDARD", Region: "us-east-1", CreationDate: time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC)},
		{ARN: "arn:express", Name: "events", Type: "EXPRESS", Region: "us-east-1"},
	}
}

func TestStepFunctionsStateMachineListRendersFiltersAndRejectsExpressHistory(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading
	_, _, handled := m.stepFunctions.HandleMessage(&m, stepFunctionStateMachinesLoadedMsg{stateMachines: stepFunctionsTestStateMachines()})
	if !handled || m.screen != screenStepFunctionStateMachineList {
		t.Fatalf("expected state machine list, got screen=%v handled=%v", m.screen, handled)
	}
	view, ok := m.stepFunctions.View(m)
	if !ok || !strings.Contains(view, "orders") || !strings.Contains(view, "events") || !strings.Contains(view, "CREATED") {
		t.Fatalf("expected state machine table, got:\n%s", view)
	}

	m.storeFilterValue(filterStepFunctionStateMachines, "express")
	m.applyFilterTarget(filterStepFunctionStateMachines)
	if len(m.stepFunctions.filteredStateMachines) != 1 || m.stepFunctions.filteredStateMachines[0].Name != "events" {
		t.Fatalf("expected EXPRESS filter result, got %+v", m.stepFunctions.filteredStateMachines)
	}
	m.stepFunctions.updateStateMachineList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenStepFunctionStateMachineList || !strings.Contains(m.stepFunctions.notice, "unavailable for EXPRESS") {
		t.Fatalf("expected EXPRESS history notice, screen=%v notice=%q", m.screen, m.stepFunctions.notice)
	}
}

func TestStepFunctionsDrillDownAndDetailPayload(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 20
	m.width = 120
	m.screen = screenLoading
	m.stepFunctions.HandleMessage(&m, stepFunctionStateMachinesLoadedMsg{stateMachines: stepFunctionsTestStateMachines()[:1]})

	model, cmd := m.stepFunctions.updateStateMachineList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if cmd == nil || m.screen != screenLoading || m.stepFunctions.selectedStateMachine == nil {
		t.Fatalf("expected execution load, screen=%v selected=%+v cmd=%v", m.screen, m.stepFunctions.selectedStateMachine, cmd)
	}

	executions := []awsservice.StepFunctionExecution{
		{ARN: "arn:failed", Name: "failed-run", StateMachineARN: "arn:standard", Status: "FAILED", StartDate: time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC)},
		{ARN: "arn:ok", Name: "ok-run", StateMachineARN: "arn:standard", Status: "SUCCEEDED"},
	}
	m.stepFunctions.HandleMessage(&m, stepFunctionExecutionsLoadedMsg{stateMachineARN: "arn:standard", executions: executions})
	if m.screen != screenStepFunctionExecutionList {
		t.Fatalf("expected execution list, got %v", m.screen)
	}
	view, _ := m.stepFunctions.View(m)
	if !strings.Contains(view, "failed-run") || !strings.Contains(view, "latest 200, failures first") {
		t.Fatalf("expected execution triage table, got:\n%s", view)
	}

	model, cmd = m.stepFunctions.updateExecutionList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if cmd == nil || m.screen != screenLoading {
		t.Fatalf("expected execution detail load, screen=%v cmd=%v", m.screen, cmd)
	}
	detail := &awsservice.StepFunctionExecutionDetail{
		StepFunctionExecution: executions[0],
		FailedStep:            "ChargeCard",
		Error:                 "States.TaskFailed",
		Cause:                 "payment rejected\x1b[31m",
		Input:                 "{ \n \"order\": 42 }",
	}
	m.stepFunctions.HandleMessage(&m, stepFunctionExecutionDetailLoadedMsg{detail: detail})
	if m.screen != screenStepFunctionExecutionDetail {
		t.Fatalf("expected execution detail, got %v", m.screen)
	}
	detailView, _ := m.stepFunctions.View(m)
	plain := stripANSI(detailView)
	for _, want := range []string{"ChargeCard", "States.TaskFailed", `{"order":42}`, `payment rejected\x1b[31m`} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected %q in execution detail, got:\n%s", want, plain)
		}
	}
	if strings.Contains(detailView, "\x1b[31m") {
		t.Fatalf("expected terminal control bytes to be escaped, got %q", detailView)
	}
}

func TestStepFunctionsIgnoresStaleExecutionLoads(t *testing.T) {
	for _, tc := range []struct {
		name     string
		selected *awsservice.StepFunctionStateMachine
	}{
		{name: "no selected state machine"},
		{name: "different state machine", selected: &awsservice.StepFunctionStateMachine{ARN: "arn:other"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(testConfig(), "", "dev")
			m.screen = screenLoading
			m.stepFunctions.selectedStateMachine = tc.selected
			_, _, handled := m.stepFunctions.HandleMessage(&m, stepFunctionExecutionsLoadedMsg{
				stateMachineARN: "arn:stale",
				executions:      []awsservice.StepFunctionExecution{{ARN: "arn:execution"}},
			})
			if !handled || m.screen != screenLoading || len(m.stepFunctions.executions) != 0 {
				t.Fatalf("expected stale load to be ignored, screen=%v executions=%+v handled=%v", m.screen, m.stepFunctions.executions, handled)
			}
		})
	}
}

func TestStepFunctionsBackNavigation(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenStepFunctionExecutionList
	m.stepFunctions.HandleKey(&m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenStepFunctionStateMachineList {
		t.Fatalf("expected execution list to return to state machines, got %v", m.screen)
	}
	m.screen = screenStepFunctionExecutionDetail
	m.stepFunctions.HandleKey(&m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenStepFunctionExecutionList {
		t.Fatalf("expected execution detail to return to executions, got %v", m.screen)
	}
}

func TestStepFunctionsDetailPlaceholderAndPayloadLimit(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenStepFunctionExecutionDetail
	view := stripANSI(m.stepFunctions.viewExecutionDetail(m))
	for _, want := range []string{"Step Functions Execution Detail", "No execution detail loaded", "esc: executions"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in placeholder view, got:\n%s", want, view)
		}
	}

	for name, payload := range map[string]string{
		"json": `{"value":"` + strings.Repeat("x", stepFunctionsPayloadPreviewLimit) + `"}`,
		"text": strings.Repeat("word ", stepFunctionsPayloadPreviewLimit),
	} {
		t.Run(name, func(t *testing.T) {
			preview := stepFunctionsPayloadPreview(payload)
			if len(preview) > stepFunctionsPayloadPreviewLimit || !strings.HasSuffix(preview, "...") {
				t.Fatalf("expected bounded truncated preview, length=%d suffix=%q", len(preview), preview[max(len(preview)-20, 0):])
			}
		})
	}
}

func TestStepFunctionsDetailScrolls(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 10
	m.width = 100
	m.screen = screenStepFunctionExecutionDetail
	m.stepFunctions.selectedExecution = &awsservice.StepFunctionExecutionDetail{
		StepFunctionExecution: awsservice.StepFunctionExecution{ARN: "arn:execution", Name: "run", StateMachineARN: "arn:machine", Status: "FAILED"},
		FailedStep:            "Work",
		Error:                 "Error",
		Cause:                 "Cause",
		Input:                 `{"input":true}`,
		Output:                `{"output":false}`,
	}
	initial := stripANSI(m.stepFunctions.viewExecutionDetail(m))
	if strings.Contains(initial, `{"output":false}`) {
		t.Fatalf("expected output to begin below the window, got:\n%s", initial)
	}
	_, _, handled := m.stepFunctions.HandleKey(&m, tea.KeyMsg{Type: tea.KeyPgDown})
	if !handled {
		t.Fatal("expected page-down to be handled")
	}
	scrolled := stripANSI(m.stepFunctions.viewExecutionDetail(m))
	if !strings.Contains(scrolled, `{"output":false}`) {
		t.Fatalf("expected page-down to reveal output, got:\n%s", scrolled)
	}
}

func TestStepFunctionsLoadCompletionStaysBehindSettings(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenSettings
	m.settingsPrevScreen = screenLoading
	m.stepFunctions.HandleMessage(&m, stepFunctionStateMachinesLoadedMsg{stateMachines: stepFunctionsTestStateMachines()})
	if m.screen != screenSettings || m.settingsPrevScreen != screenStepFunctionStateMachineList {
		t.Fatalf("expected completed load behind Settings, screen=%v previous=%v", m.screen, m.settingsPrevScreen)
	}
}

func TestStepFunctionsLoadCompletionsUpdatePendingContextPickerReturn(t *testing.T) {
	cfg := testConfig()
	cfg.ContextName = "account-a"
	m := New(cfg, "", "dev")
	updated, _ := m.startLoadingFor(screenStepFunctionStateMachineList, "Loading state machines...", nil, nil)
	m = updated.(Model)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = updated.(Model)
	if cmd == nil || m.ctxPrevScreen != screenLoading {
		t.Fatalf("expected pending context picker over the load, previous=%v command=%v", m.ctxPrevScreen, cmd)
	}

	updated, _ = m.Update(stepFunctionStateMachinesLoadedMsg{stateMachines: stepFunctionsTestStateMachines()})
	m = updated.(Model)
	if m.screen != screenStepFunctionStateMachineList || m.ctxPrevScreen != screenStepFunctionStateMachineList {
		t.Fatalf("expected load completion to update the picker return, screen=%v previous=%v", m.screen, m.ctxPrevScreen)
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || m.screen != screenLoading {
		t.Fatalf("expected execution load before the picker opens, screen=%v command=%v", m.screen, cmd)
	}
	updated, _ = m.Update(stepFunctionExecutionsLoadedMsg{stateMachineARN: "arn:standard"})
	m = updated.(Model)
	if m.screen != screenStepFunctionExecutionList || m.ctxPrevScreen != screenStepFunctionExecutionList {
		t.Fatalf("expected the later drill-down to update the picker return, screen=%v previous=%v", m.screen, m.ctxPrevScreen)
	}

	updated, _ = m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{{Name: "account-a", Current: true}}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenStepFunctionExecutionList {
		t.Fatalf("expected picker cancel to return to the completed load, got %v", m.screen)
	}
}

func TestStepFunctionsContextSwitchNormalizesNestedSettingsReturn(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenLoading
	m.loadingReturnScreen = screenStepFunctionStateMachineList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	m = updated.(Model)
	if m.screen != screenSettings || m.settingsPrevScreen != screenLoading {
		t.Fatalf("expected Settings over the Step Functions load, screen=%v previous=%v", m.screen, m.settingsPrevScreen)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = updated.(Model)
	updated, _ = m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{{Name: "account-b", Current: true}}})
	m = updated.(Model)
	if m.screen != screenContextPicker || m.ctxPrevScreen != screenSettings {
		t.Fatalf("expected context picker over Settings, screen=%v previous=%v", m.screen, m.ctxPrevScreen)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || m.screen != screenLoading || m.settingsPrevScreen != screenServiceList {
		t.Fatalf("expected the context switch to replace the abandoned load return, screen=%v settings previous=%v command=%v", m.screen, m.settingsPrevScreen, cmd)
	}

	nextCfg := testConfig()
	nextCfg.ContextName = "account-b"
	updated, _ = m.Update(contextSwitchedMsg{cfg: nextCfg})
	m = updated.(Model)
	if m.screen != screenSettings {
		t.Fatalf("expected context switch to preserve Settings, got %v", m.screen)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenServiceList {
		t.Fatalf("expected Settings to return to the service list, got %v", m.screen)
	}
}

func TestStepFunctionsContextSwitchClearsAccountState(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		startScreen          screen
		loadingReturnScreen  screen
		completeBeforeSwitch bool
	}{
		{name: "pending load completes before switch", startScreen: screenLoading, loadingReturnScreen: screenStepFunctionStateMachineList, completeBeforeSwitch: true},
		{name: "pending load completes after switch", startScreen: screenLoading, loadingReturnScreen: screenStepFunctionStateMachineList},
		{name: "execution detail", startScreen: screenStepFunctionExecutionDetail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(testConfig(), "", "dev")
			m.screen = tc.startScreen
			m.loadingReturnScreen = tc.loadingReturnScreen
			m.stepFunctions.stateMachines = stepFunctionsTestStateMachines()
			m.stepFunctions.selectedStateMachine = &m.stepFunctions.stateMachines[0]
			m.stepFunctions.selectedExecution = &awsservice.StepFunctionExecutionDetail{
				StepFunctionExecution: awsservice.StepFunctionExecution{ARN: "arn:account-a", Name: "account-a-run"},
			}
			m.storeFilterValue(filterStepFunctionStateMachines, "account-a")
			m.storeFilterValue(filterStepFunctionExecutions, "failed")

			oldGeneration := m.commands.CurrentGen()
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
			m = updated.(Model)
			updated, _ = m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{{Name: "account-b", Current: true}}})
			m = updated.(Model)
			if m.screen != screenContextPicker {
				t.Fatalf("expected context picker, got %v", m.screen)
			}
			if tc.completeBeforeSwitch {
				updated, _ = m.Update(stepFunctionStateMachinesLoadedMsg{stateMachines: stepFunctionsTestStateMachines()})
				m = updated.(Model)
				if m.screen != screenContextPicker || m.ctxPrevScreen != screenStepFunctionStateMachineList {
					t.Fatalf("expected completed load to stay behind the picker, screen=%v previous=%v", m.screen, m.ctxPrevScreen)
				}
			}

			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(Model)
			if cmd == nil || m.screen != screenLoading || m.commands.CurrentGen() <= oldGeneration {
				t.Fatalf("expected context switch to supersede the old load, screen=%v generation=%d command=%v", m.screen, m.commands.CurrentGen(), cmd)
			}
			if !tc.completeBeforeSwitch {
				updated, _ = m.Update(genBoundMsg{gen: oldGeneration, msg: stepFunctionStateMachinesLoadedMsg{
					stateMachines: []awsservice.StepFunctionStateMachine{{ARN: "arn:stale-result", Name: "stale-result"}},
				}})
				m = updated.(Model)
				if len(m.stepFunctions.stateMachines) != 2 || m.stepFunctions.stateMachines[0].ARN != "arn:standard" {
					t.Fatalf("expected the superseded account-A result to be dropped, got %+v", m.stepFunctions.stateMachines)
				}
			}

			nextCfg := testConfig()
			nextCfg.ContextName = "account-b"
			updated, _ = m.Update(contextSwitchedMsg{cfg: nextCfg})
			m = updated.(Model)
			if m.screen != screenServiceList {
				t.Fatalf("expected service list after context switch, got %v", m.screen)
			}
			if len(m.stepFunctions.stateMachines) != 0 || m.stepFunctions.selectedStateMachine != nil || m.stepFunctions.selectedExecution != nil {
				t.Fatalf("expected Step Functions state to be cleared, got %+v", m.stepFunctions)
			}
			if m.filterValue(filterStepFunctionStateMachines) != "" || m.filterValue(filterStepFunctionExecutions) != "" {
				t.Fatalf("expected Step Functions filters to be cleared")
			}
		})
	}
}

func TestStepFunctionsSameContextRefreshPreservesState(t *testing.T) {
	cfg := testConfig()
	cfg.ContextName = "account-a"
	m := New(cfg, "", "dev")
	m.screen = screenStepFunctionExecutionDetail
	m.stepFunctions.stateMachines = stepFunctionsTestStateMachines()
	m.stepFunctions.selectedStateMachine = &m.stepFunctions.stateMachines[0]
	m.stepFunctions.selectedExecution = &awsservice.StepFunctionExecutionDetail{
		StepFunctionExecution: awsservice.StepFunctionExecution{ARN: "arn:account-a", Name: "account-a-run"},
	}
	m.storeFilterValue(filterStepFunctionStateMachines, "orders")
	m.storeFilterValue(filterStepFunctionExecutions, "failed")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = updated.(Model)
	updated, _ = m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{{Name: "account-a", Current: true}}})
	m = updated.(Model)
	if m.screen != screenContextPicker || m.ctxPrevScreen != screenStepFunctionExecutionDetail {
		t.Fatalf("expected picker over the execution detail, screen=%v previous=%v", m.screen, m.ctxPrevScreen)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || m.screen != screenLoading || m.ctxPrevScreen != screenStepFunctionExecutionDetail {
		t.Fatalf("expected same-context refresh to preserve the return screen, screen=%v previous=%v command=%v", m.screen, m.ctxPrevScreen, cmd)
	}

	nextCfg := testConfig()
	nextCfg.ContextName = "account-a"
	updated, _ = m.Update(contextSwitchedMsg{cfg: nextCfg})
	m = updated.(Model)
	if m.screen != screenStepFunctionExecutionDetail || m.stepFunctions.selectedExecution == nil || m.stepFunctions.selectedExecution.ARN != "arn:account-a" {
		t.Fatalf("expected same-context refresh to preserve Step Functions detail, screen=%v state=%+v", m.screen, m.stepFunctions)
	}
	if m.filterValue(filterStepFunctionStateMachines) != "orders" || m.filterValue(filterStepFunctionExecutions) != "failed" {
		t.Fatalf("expected same-context refresh to preserve Step Functions filters")
	}

	changedRegionCfg := *nextCfg
	changedRegionCfg.Region = "us-west-2"
	updated, _ = m.Update(contextSwitchedMsg{cfg: &changedRegionCfg})
	m = updated.(Model)
	if m.screen != screenServiceList {
		t.Fatalf("expected service list after same-context region change, got %v", m.screen)
	}
	if len(m.stepFunctions.stateMachines) != 0 || m.stepFunctions.selectedStateMachine != nil || m.stepFunctions.selectedExecution != nil {
		t.Fatalf("expected Step Functions state to be cleared after region change, got %+v", m.stepFunctions)
	}
	if m.filterValue(filterStepFunctionStateMachines) != "" || m.filterValue(filterStepFunctionExecutions) != "" {
		t.Fatalf("expected Step Functions filters to be cleared after region change")
	}
}

func TestStepFunctionsSameContextRefreshPreservesPendingLoadReturn(t *testing.T) {
	cfg := testConfig()
	cfg.ContextName = "account-a"
	m := New(cfg, "", "dev")
	updated, _ := m.startLoadingFor(screenStepFunctionExecutionList, "Loading executions...", nil, func() tea.Msg { return nil })
	m = updated.(Model)
	oldGeneration := m.commands.CurrentGen()

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = updated.(Model)
	updated, _ = m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{{Name: "account-a", Current: true}}})
	m = updated.(Model)
	if m.screen != screenContextPicker || m.ctxPrevScreen != screenLoading {
		t.Fatalf("expected picker over the pending load, screen=%v previous=%v", m.screen, m.ctxPrevScreen)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || m.screen != screenLoading || m.commands.CurrentGen() <= oldGeneration {
		t.Fatalf("expected context refresh to supersede the pending load, screen=%v generation=%d command=%v", m.screen, m.commands.CurrentGen(), cmd)
	}

	nextCfg := testConfig()
	nextCfg.ContextName = "account-a"
	updated, _ = m.Update(contextSwitchedMsg{cfg: nextCfg})
	m = updated.(Model)
	if m.screen != screenStepFunctionExecutionList {
		t.Fatalf("expected same-context refresh to restore the pending load target, got %v", m.screen)
	}
}

func TestStepFunctionsSameContextRefreshDoesNotRestoreStaleDrillDownData(t *testing.T) {
	refreshContext := func(t *testing.T, m Model) Model {
		t.Helper()
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
		m = updated.(Model)
		updated, _ = m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{{Name: "account-a", Current: true}}})
		m = updated.(Model)
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(Model)
		if cmd == nil || m.screen != screenLoading {
			t.Fatalf("expected same-context refresh load, screen=%v command=%v", m.screen, cmd)
		}
		updated, _ = m.Update(contextSwitchedMsg{cfg: m.cfg})
		return updated.(Model)
	}

	t.Run("execution list", func(t *testing.T) {
		cfg := testConfig()
		cfg.ContextName = "account-a"
		m := New(cfg, "", "dev")
		m.screen = screenStepFunctionStateMachineList
		m.stepFunctions.stateMachines = []awsservice.StepFunctionStateMachine{
			{ARN: "arn:machine-a", Name: "machine-a", Type: "STANDARD"},
			{ARN: "arn:machine-b", Name: "machine-b", Type: "STANDARD"},
		}
		m.stepFunctions.filteredStateMachines = append([]awsservice.StepFunctionStateMachine(nil), m.stepFunctions.stateMachines...)
		m.stepFunctions.stateMachineIdx = 1
		m.stepFunctions.selectedStateMachine = &m.stepFunctions.stateMachines[0]
		m.stepFunctions.executions = []awsservice.StepFunctionExecution{{ARN: "arn:execution-a", Name: "account-a-run"}}
		m.stepFunctions.filteredExecutions = append([]awsservice.StepFunctionExecution(nil), m.stepFunctions.executions...)

		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(Model)
		if cmd == nil || m.screen != screenLoading || m.stepFunctions.selectedStateMachine == nil || m.stepFunctions.selectedStateMachine.Name != "machine-b" {
			t.Fatalf("expected machine-b execution load, screen=%v selected=%+v command=%v", m.screen, m.stepFunctions.selectedStateMachine, cmd)
		}

		m = refreshContext(t, m)
		view := stripANSI(m.stepFunctions.viewExecutionList(m))
		if m.screen != screenStepFunctionExecutionList || strings.Contains(view, "account-a-run") {
			t.Fatalf("expected an empty machine-b execution list after refresh, screen=%v view:\n%s", m.screen, view)
		}
	})

	t.Run("execution detail", func(t *testing.T) {
		cfg := testConfig()
		cfg.ContextName = "account-a"
		m := New(cfg, "", "dev")
		m.screen = screenStepFunctionExecutionList
		m.stepFunctions.selectedStateMachine = &awsservice.StepFunctionStateMachine{ARN: "arn:machine-b", Name: "machine-b", Type: "STANDARD"}
		m.stepFunctions.executions = []awsservice.StepFunctionExecution{{ARN: "arn:execution-b", Name: "account-b-run"}}
		m.stepFunctions.filteredExecutions = append([]awsservice.StepFunctionExecution(nil), m.stepFunctions.executions...)
		m.stepFunctions.selectedExecution = &awsservice.StepFunctionExecutionDetail{
			StepFunctionExecution: awsservice.StepFunctionExecution{ARN: "arn:execution-a", Name: "account-a-run"},
		}

		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(Model)
		if cmd == nil || m.screen != screenLoading {
			t.Fatalf("expected execution-b detail load, screen=%v command=%v", m.screen, cmd)
		}

		m = refreshContext(t, m)
		view := stripANSI(m.stepFunctions.viewExecutionDetail(m))
		if m.screen != screenStepFunctionExecutionDetail || strings.Contains(view, "account-a-run") || !strings.Contains(view, "No execution detail loaded") {
			t.Fatalf("expected an empty execution-b detail after refresh, screen=%v view:\n%s", m.screen, view)
		}
	})
}

func TestStepFunctionsRegionSwitchClearsState(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.stepFunctions.stateMachines = stepFunctionsTestStateMachines()
	m.stepFunctions.selectedStateMachine = &m.stepFunctions.stateMachines[0]
	m.stepFunctions.selectedExecution = &awsservice.StepFunctionExecutionDetail{
		StepFunctionExecution: awsservice.StepFunctionExecution{ARN: "arn:old-region", Name: "old-region-run"},
	}
	m.storeFilterValue(filterStepFunctionStateMachines, "orders")
	m.storeFilterValue(filterStepFunctionExecutions, "failed")

	updated, _ := m.Update(regionSwitchedMsg{region: "us-west-2"})
	m = updated.(Model)
	if m.screen != screenServiceList || m.cfg.Region != "us-west-2" {
		t.Fatalf("expected service list in the new region, screen=%v region=%q", m.screen, m.cfg.Region)
	}
	if len(m.stepFunctions.stateMachines) != 0 || m.stepFunctions.selectedStateMachine != nil || m.stepFunctions.selectedExecution != nil {
		t.Fatalf("expected Step Functions state to be cleared after region switch, got %+v", m.stepFunctions)
	}
	if m.filterValue(filterStepFunctionStateMachines) != "" || m.filterValue(filterStepFunctionExecutions) != "" {
		t.Fatalf("expected Step Functions filters to be cleared after region switch")
	}
}

func TestStepFunctionsSavedViewAndHelpTitles(t *testing.T) {
	if target, ok := featurePrimaryFilter["Step Functions Execution Browser"]; !ok || target != filterStepFunctionStateMachines {
		t.Fatalf("expected Step Functions saved-view filter, got %v %v", target, ok)
	}
	m := New(testConfig(), "", "dev")
	for screen, want := range map[screen]string{
		screenStepFunctionStateMachineList: "Step Functions State Machines",
		screenStepFunctionExecutionList:    "Step Functions Executions",
		screenStepFunctionExecutionDetail:  "Step Functions Execution Detail",
	} {
		m.screen = screen
		if got := m.helpScreenTitle(); got != want {
			t.Fatalf("screen %v: expected %q, got %q", screen, want, got)
		}
		if len(m.currentScreenShortcuts()) == 0 {
			t.Fatalf("screen %v: expected keymap shortcuts", screen)
		}
	}
}
