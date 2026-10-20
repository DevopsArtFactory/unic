package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func requireDemoModel(t *testing.T, model tea.Model) demoModel {
	t.Helper()
	d, ok := model.(demoModel)
	if !ok {
		t.Fatalf("unexpected demo model type %T", model)
	}
	return d
}

func TestDemoFailureEvidenceFitsViewport(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{{Width: 100, Height: 30}, {Width: 80, Height: 24}, {Width: 60, Height: 20}} {
		t.Run(fmt.Sprintf("%dx%d", size.Width, size.Height), func(t *testing.T) {
			updated, _ := NewDemo().Update(size)
			d := requireDemoModel(t, updated)
			d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyEnter})
			for range 30 {
				d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyPgDown})
			}
			view := ansi.Strip(d.View())
			compact := strings.NewReplacer(" ", "", "\n", "", "│", "").Replace(view)
			evidence := "CannotPullContainerError:imagemanifestnotfoundfordemo-42."
			if !strings.Contains(compact, evidence) {
				t.Fatalf("complete failure evidence is unreachable:\n%s", view)
			}
			if !strings.Contains(view, "╰") || strings.Contains(view, "  ...") {
				t.Fatalf("viewport was cropped instead of scrolled:\n%s", view)
			}
			if !strings.Contains(compact, "?:guide•q:quit") {
				t.Fatalf("guide or quit shortcut is hidden:\n%s", view)
			}
			if len(strings.Split(view, "\n")) != size.Height {
				t.Fatal("detail did not fit terminal height")
			}
			for _, line := range strings.Split(view, "\n") {
				if ansi.StringWidth(line) > size.Width {
					t.Fatalf("detail exceeds terminal width: %q", line)
				}
			}
		})
	}
}

func TestDemoScrollClampsAfterResize(t *testing.T) {
	updated, _ := NewDemo().Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	d := demoKey(t, requireDemoModel(t, updated), tea.KeyMsg{Type: tea.KeyEnter})
	for range 10 {
		d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyPgDown})
	}
	updated, _ = d.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	d = requireDemoModel(t, updated)
	lines, visible := d.shell.ecs.serviceDetailViewport(d.shell, demoRolloutHelp)
	want := max(len(lines)-visible-1, 0)
	d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyUp})
	if d.shell.ecs.detailScroll != want {
		t.Fatalf("first up after resize should move from visible position: got %d, want %d", d.shell.ecs.detailScroll, want)
	}
	d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyPgUp})
	if d.shell.ecs.detailScroll != 0 {
		t.Fatal("page up did not clamp at the first line")
	}
	d = demoKey(t, d, tea.KeyMsg{Type: tea.KeyUp})
	if d.shell.ecs.detailScroll != 0 {
		t.Fatal("up scrolled before the first line")
	}
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
