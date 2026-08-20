package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
