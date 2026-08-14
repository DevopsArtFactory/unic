package app

import (
	"strings"
	"testing"

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

	_, _, handled := m.ssmParams.HandleMessage(&m, ssmParametersLoadedMsg{parameters: testParameters()})
	if !handled || m.screen != screenSSMParamList {
		t.Fatalf("expected parameter list screen, got %v", m.screen)
	}
	view, ok := m.ssmParams.View(m)
	if !ok || !strings.Contains(view, "/app/dev/api-url") || !strings.Contains(view, "SecureString") {
		t.Fatalf("expected parameter rows with type column, got:\n%s", view)
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
	m.ssmParams.HandleMessage(&m, ssmParamValueLoadedMsg{name: "/app/prod/db-password", value: "s3cret"})
	view, _ := m.ssmParams.View(m)
	if !strings.Contains(view, "s3cret") {
		t.Fatalf("expected revealed value, got:\n%s", view)
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
	m.ssmParams.HandleMessage(&m, ssmParamValueLoadedMsg{name: "/app/prod/db-password", value: "s3cret", copyOnly: true})
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
