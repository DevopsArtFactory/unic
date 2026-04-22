package clipboard

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

var lookPath = exec.LookPath
var command = exec.Command

// Copy copies text to the system clipboard.
func Copy(text string) error {
	cmd, err := clipboardCommand(runtime.GOOS)
	if err != nil {
		return err
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func clipboardCommand(goos string) (*exec.Cmd, error) {
	switch goos {
	case "darwin":
		return command("pbcopy"), nil
	case "linux":
		if _, err := lookPath("wl-copy"); err == nil {
			return command("wl-copy"), nil
		}
		if _, err := lookPath("xclip"); err == nil {
			return command("xclip", "-selection", "clipboard"), nil
		}
		if _, err := lookPath("xsel"); err == nil {
			return command("xsel", "--clipboard", "--input"), nil
		}
		return nil, fmt.Errorf("no clipboard utility found (tried: wl-copy, xclip, xsel)")
	default:
		return nil, fmt.Errorf("clipboard not supported on %s", goos)
	}
}
