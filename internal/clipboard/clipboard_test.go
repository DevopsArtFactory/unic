package clipboard

import (
	"errors"
	"os/exec"
	"testing"
)

func TestClipboardCommandLinuxUsesXselWhenAvailable(t *testing.T) {
	originalLookPath := lookPath
	originalCommand := command
	defer func() {
		lookPath = originalLookPath
		command = originalCommand
	}()

	lookPath = func(file string) (string, error) {
		switch file {
		case "wl-copy", "xclip":
			return "", errors.New("missing")
		case "xsel":
			return "/usr/bin/xsel", nil
		default:
			return "", errors.New("unexpected")
		}
	}
	command = func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command("echo")
		cmd.Args = append([]string{name}, args...)
		return cmd
	}

	cmd, err := clipboardCommand("linux")
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Args) != 3 || cmd.Args[0] != "xsel" || cmd.Args[1] != "--clipboard" || cmd.Args[2] != "--input" {
		t.Fatalf("unexpected command args: %#v", cmd.Args)
	}
}

func TestClipboardCommandLinuxErrorsWhenNoUtilityExists(t *testing.T) {
	originalLookPath := lookPath
	defer func() { lookPath = originalLookPath }()

	lookPath = func(file string) (string, error) {
		return "", errors.New("missing")
	}

	_, err := clipboardCommand("linux")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "no clipboard utility found (tried: wl-copy, xclip, xsel)" {
		t.Fatalf("unexpected error: %v", err)
	}
}
