package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func requireDemoModel(t *testing.T, model tea.Model) demoModel {
	t.Helper()
	d, ok := model.(demoModel)
	if !ok {
		t.Fatalf("unexpected demo model type %T", model)
	}
	return d
}

func demoKey(t *testing.T, d demoModel, key tea.KeyMsg) demoModel {
	t.Helper()
	updated, cmd := d.Update(key)
	if cmd != nil {
		t.Fatal("demo navigation returned an asynchronous command")
	}
	return requireDemoModel(t, updated)
}

func TestDemoOfflineNavigationAndIsolation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(home, "missing-config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(home, "missing-credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	// A configured installation must not be read or rewritten either.
	configDir := filepath.Join(home, "unic")
	if err := os.Mkdir(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	const original = "deliberately invalid: [\n"
	if err := os.WriteFile(configPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", home)

	d := requireDemoModel(t, NewDemo())
	if d.Init() != nil || d.shell.awsRepo != nil || d.shell.commands != nil || d.shell.configPath != "" {
		t.Fatal("demo initialized live application dependencies")
	}
	if !strings.Contains(d.View(), "synthetic/offline") {
		t.Fatal("missing persistent demo label")
	}
	d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyEnter})
	if d.shell.screen != screenECSServiceDetail {
		t.Fatal("Enter did not open rollout")
	}
	for _, evidence := range []string{"FAILED", "running:0 desired:2", "CannotPullContainerError"} {
		if !strings.Contains(strings.Join(d.shell.ecs.serviceDetailLines(), "\n"), evidence) {
			t.Fatalf("missing evidence %q", evidence)
		}
	}
	for range 30 {
		d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyDown})
	}
	if d.shell.ecs.detailScroll == 0 || !strings.Contains(d.View(), "CannotPullContainerError") {
		t.Fatal("scrolling did not expose service events")
	}
	for _, key := range []string{"C", "R", "S", "P", "V", "W", "I", "f", "i", "d", "enter"} {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		if key == "enter" {
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		}
		d = demoKey(t, d, msg)
		if d.shell.screen != screenECSServiceDetail {
			t.Fatalf("shortcut %q escaped demo", key)
		}
	}
	d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !strings.Contains(d.View(), "verify the image tag") {
		t.Fatal("missing investigation guide")
	}
	d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyEsc})
	d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if d.shell.ecs.detailScroll != 0 {
		t.Fatal("restart did not reset scroll")
	}
	d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyDown})
	scroll := d.shell.ecs.detailScroll
	d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyEsc})
	d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if d.shell.ecs.detailScroll != scroll {
		t.Fatal("restart key changed rollout state from service picker")
	}
	if d.shell.screen != screenECSServiceList {
		t.Fatal("Esc did not return to service picker")
	}
	data, err := os.ReadFile(configPath)
	if err != nil || string(data) != original {
		t.Fatal("demo changed existing configuration")
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 1 {
		t.Fatal("demo created files in user home")
	}
}

func TestDemoResizeAndQuit(t *testing.T) {
	for _, key := range []tea.KeyMsg{{Type: tea.KeyCtrlC}, {Type: tea.KeyRunes, Runes: []rune("q")}} {
		d := requireDemoModel(t, NewDemo())
		updated, cmd := d.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
		if cmd != nil {
			t.Fatal("resize returned a command")
		}
		d = requireDemoModel(t, updated)
		if len(strings.Split(d.View(), "\n")) != 20 {
			t.Fatal("demo did not fit terminal height")
		}
		updated, cmd = d.Update(key)
		if cmd == nil {
			t.Fatal("quit did not return a command")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok || updated.View() != "" {
			t.Fatal("demo did not exit cleanly")
		}
	}
}
