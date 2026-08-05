package app

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func multiRegionTestModel() Model {
	m := New(&config.Config{
		ContextName: "production",
		Region:      "ap-northeast-2",
		Regions:     []string{"ap-northeast-2", "us-east-1"},
	}, "", "dev")
	return m
}

func TestGlobalRegionShortcutOpensPicker(t *testing.T) {
	m := multiRegionTestModel()
	m.screen = screenServiceList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	model := updated.(Model)
	if model.screen != screenRegionPicker {
		t.Fatalf("expected region picker, got %v", model.screen)
	}
	if model.regionIdx != 0 {
		t.Fatalf("expected active region at index 0, got %d", model.regionIdx)
	}
}

func TestRegionShortcutBlockedDuringActiveRequests(t *testing.T) {
	for _, activeScreen := range []screen{screenLoading, screenInspectorScanning} {
		m := multiRegionTestModel()
		m.screen = activeScreen
		if m.canSwitchResourceRegion() {
			t.Fatalf("expected region switching to be blocked on screen %v", activeScreen)
		}
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
		if model := updated.(Model); model.screen != activeScreen {
			t.Fatalf("expected screen %v to remain active, got %v", activeScreen, model.screen)
		}
	}
}

func TestRegionPickerNavigationAndCancel(t *testing.T) {
	keys := []struct {
		name string
		msg  tea.KeyMsg
	}{
		{name: "q", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}},
		{name: "esc", msg: tea.KeyMsg{Type: tea.KeyEsc}},
	}
	for _, key := range keys {
		m := multiRegionTestModel()
		m.screen = screenRegionPicker
		m.regionPrevScreen = screenFeatureList
		updated, _ := m.updateRegionPicker(key.msg)
		if model := updated.(Model); model.screen != screenFeatureList {
			t.Fatalf("%s should restore previous screen, got %v", key.name, model.screen)
		}
	}

	m := multiRegionTestModel()
	m.screen = screenRegionPicker
	updated, _ := m.updateRegionPicker(tea.KeyMsg{Type: tea.KeyDown})
	if model := updated.(Model); model.regionIdx != 1 {
		t.Fatalf("expected cursor at region 1, got %d", model.regionIdx)
	}
}

func TestRegionPickerSelectingActiveRegionReturnsWithoutSwitch(t *testing.T) {
	m := multiRegionTestModel()
	m.screen = screenRegionPicker
	m.regionPrevScreen = screenRDSList
	m.regionIdx = 0

	updated, cmd := m.updateRegionPicker(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if cmd != nil {
		t.Fatal("expected no command when selecting the active region")
	}
	if model.screen != screenRDSList {
		t.Fatalf("expected previous screen, got %v", model.screen)
	}
}

func TestRegionPickerRepositoryCreationFailure(t *testing.T) {
	original := newAwsRepositoryForRegionFn
	t.Cleanup(func() { newAwsRepositoryForRegionFn = original })
	expectedErr := errors.New("credentials unavailable")
	newAwsRepositoryForRegionFn = func(context.Context, *config.Config) (*awsservice.AwsRepository, error) {
		return nil, expectedErr
	}

	m := multiRegionTestModel()
	m.screen = screenRegionPicker
	m.regionIdx = 1
	updated, cmd := m.updateRegionPicker(tea.KeyMsg{Type: tea.KeyEnter})
	if model := updated.(Model); model.screen != screenLoading {
		t.Fatalf("expected loading screen, got %v", model.screen)
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatal("expected loading command batch")
	}
	var errResult errMsg
	found := false
	for _, batchedCmd := range batch {
		if msg, ok := batchedCmd().(errMsg); ok {
			errResult = msg
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected repository failure errMsg in command batch")
	}
	if !errors.Is(errResult.err, expectedErr) {
		t.Fatalf("unexpected error: %v", errResult.err)
	}
}

func TestRegionSwitchedMessageUpdatesRuntimeRegion(t *testing.T) {
	m := multiRegionTestModel()
	repo := &awsservice.AwsRepository{Region: "us-east-1"}

	updated, _ := m.Update(regionSwitchedMsg{region: "us-east-1", repo: repo})
	model := updated.(Model)
	if model.cfg.Region != "us-east-1" {
		t.Fatalf("expected active region us-east-1, got %q", model.cfg.Region)
	}
	if model.awsRepo != repo {
		t.Fatal("expected switched repository to be installed")
	}
	if model.screen != screenServiceList {
		t.Fatalf("expected service list after switch, got %v", model.screen)
	}
}

func TestSingleRegionContextDoesNotOpenPicker(t *testing.T) {
	m := New(&config.Config{
		ContextName: "production",
		Region:      "ap-northeast-2",
		Regions:     []string{"ap-northeast-2"},
	}, "", "dev")
	m.screen = screenServiceList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if model := updated.(Model); model.screen != screenServiceList {
		t.Fatalf("single-region context should stay on service list, got %v", model.screen)
	}
}
