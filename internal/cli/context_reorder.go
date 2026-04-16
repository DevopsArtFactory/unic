package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/term"

	"unic/internal/config"
)

func reorderContexts(configPath string, in io.Reader, errOut io.Writer) ([]string, error) {
	contexts, err := config.Contexts(configPath)
	if err != nil {
		return nil, err
	}
	if len(contexts) == 0 {
		return nil, fmt.Errorf("no contexts found in %s", configPath)
	}

	inFile, okIn := in.(*os.File)
	outFile, okOut := errOut.(*os.File)
	if okIn && okOut && term.IsTerminal(inFile.Fd()) && term.IsTerminal(outFile.Fd()) {
		return reorderContextsInteractive(inFile, outFile, contexts)
	}
	return reorderContextsLineMode(bufio.NewReader(in), errOut, contexts)
}

func reorderContextsLineMode(in *bufio.Reader, errOut io.Writer, contexts []config.ContextInfo) ([]string, error) {
	for i, ctx := range contexts {
		fmt.Fprintf(errOut, "%2d) %s\n", i+1, ctx.Name)
	}

	selected := 0
	for {
		fmt.Fprintf(errOut, "Select context to move [1-%d]: ", len(contexts))
		raw, err := in.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if _, err := fmt.Sscanf(raw, "%d", &selected); err == nil && selected >= 1 && selected <= len(contexts) {
			break
		}
		fmt.Fprintf(errOut, "Invalid selection %q\n", raw)
	}

	target := 0
	for {
		fmt.Fprintf(errOut, "Enter new position for %s [1-%d]: ", contexts[selected-1].Name, len(contexts))
		raw, err := in.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if _, err := fmt.Sscanf(raw, "%d", &target); err == nil && target >= 1 && target <= len(contexts) {
			break
		}
		fmt.Fprintf(errOut, "Invalid position %q\n", raw)
	}

	ordered := reorderContextInfos(contexts, selected-1, target-1)
	return contextNames(ordered), nil
}

func reorderContextsInteractive(in *os.File, errOut *os.File, contexts []config.ContextInfo) ([]string, error) {
	oldState, err := term.MakeRaw(in.Fd())
	if err != nil {
		return nil, err
	}
	defer term.Restore(in.Fd(), oldState)
	defer fmt.Fprint(errOut, "\x1b[2J\x1b[H\x1b[?25h")

	reader := bufio.NewReader(in)
	cursor := 0
	moving := false
	renderContextReorderPicker(errOut, contexts, cursor, moving)
	fmt.Fprint(errOut, "\x1b[?25l")

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			return nil, err
		}

		switch r {
		case '\r', '\n':
			if moving {
				return contextNames(contexts), nil
			}
			moving = true
		case 3:
			return nil, io.EOF
		case 'k':
			if moving {
				contexts, cursor = moveContext(contexts, cursor, -1)
			} else {
				cursor = moveReorderCursor(len(contexts), cursor, -1)
			}
		case 'j':
			if moving {
				contexts, cursor = moveContext(contexts, cursor, 1)
			} else {
				cursor = moveReorderCursor(len(contexts), cursor, 1)
			}
		case 27:
			next, _, err := reader.ReadRune()
			if err != nil {
				if moving {
					moving = false
					renderContextReorderPicker(errOut, contexts, cursor, moving)
					continue
				}
				return nil, err
			}
			if next != '[' {
				if moving {
					moving = false
				}
				continue
			}
			arrow, _, err := reader.ReadRune()
			if err != nil {
				return nil, err
			}
			switch arrow {
			case 'A':
				if moving {
					contexts, cursor = moveContext(contexts, cursor, -1)
				} else {
					cursor = moveReorderCursor(len(contexts), cursor, -1)
				}
			case 'B':
				if moving {
					contexts, cursor = moveContext(contexts, cursor, 1)
				} else {
					cursor = moveReorderCursor(len(contexts), cursor, 1)
				}
			}
		}

		renderContextReorderPicker(errOut, contexts, cursor, moving)
	}
}

func renderContextReorderPicker(out io.Writer, contexts []config.ContextInfo, cursor int, moving bool) {
	fmt.Fprint(out, "\x1b[2J\x1b[H")
	height := 24
	if file, ok := out.(*os.File); ok {
		if _, h, err := term.GetSize(file.Fd()); err == nil && h > 0 {
			height = h
		}
	}

	fmt.Fprint(out, "Reorder contexts\r\n")
	if moving {
		fmt.Fprint(out, "Move mode: ↑/↓ or j/k move this context, Enter save, Esc stop moving, Ctrl+C cancel\r\n\r\n")
	} else {
		fmt.Fprint(out, "Select mode: ↑/↓ or j/k choose a context, Enter start moving, Ctrl+C cancel\r\n\r\n")
	}

	indexWidth := len(strconv.Itoa(len(contexts)))
	visibleRows := height - 6
	if visibleRows < 5 {
		visibleRows = 5
	}
	start, end := reorderWindow(len(contexts), cursor, visibleRows)

	for i := start; i < end; i++ {
		marker := " "
		if i == cursor {
			if moving {
				marker = "↕"
			} else {
				marker = ">"
			}
		}
		fmt.Fprintf(out, "%s %*d) %s\r\n", marker, indexWidth, i+1, renderReorderContextChoice(contexts[i]))
	}

	if len(contexts) > visibleRows {
		fmt.Fprint(out, "\r\n")
		fmt.Fprintf(out, "Showing %d-%d of %d\r\n", start+1, end, len(contexts))
	}
}

func moveReorderCursor(total, cursor, delta int) int {
	next := cursor + delta
	if next < 0 {
		return 0
	}
	if next >= total {
		return total - 1
	}
	return next
}

func reorderWindow(total, cursor, visibleRows int) (int, int) {
	if total <= visibleRows {
		return 0, total
	}
	start := cursor - visibleRows/2
	if start < 0 {
		start = 0
	}
	end := start + visibleRows
	if end > total {
		end = total
		start = end - visibleRows
	}
	return start, end
}

func renderReorderContextChoice(ctx config.ContextInfo) string {
	current := ""
	if ctx.Current {
		current = " (current)"
	}
	authType := ctx.AuthType
	if authType == "" {
		authType = "default"
	}
	return fmt.Sprintf("%s [%s]%s", ctx.Name, strings.ToLower(authType), current)
}

func moveContext(contexts []config.ContextInfo, cursor, delta int) ([]config.ContextInfo, int) {
	next := cursor + delta
	if next < 0 || next >= len(contexts) {
		return contexts, cursor
	}
	contexts[cursor], contexts[next] = contexts[next], contexts[cursor]
	return contexts, next
}

func reorderContextInfos(contexts []config.ContextInfo, from, to int) []config.ContextInfo {
	if from < 0 || from >= len(contexts) || to < 0 || to >= len(contexts) || from == to {
		return append([]config.ContextInfo(nil), contexts...)
	}

	item := contexts[from]
	updated := append([]config.ContextInfo(nil), contexts[:from]...)
	updated = append(updated, contexts[from+1:]...)

	// Adjust 'to' if it comes after 'from' since we removed an element
	adjustedTo := to
	if to > from {
		adjustedTo = to - 1
	}

	result := make([]config.ContextInfo, 0, len(contexts))
	result = append(result, updated[:adjustedTo]...)
	result = append(result, item)
	result = append(result, updated[adjustedTo:]...)
	return result
}
}

func contextNames(contexts []config.ContextInfo) []string {
	names := make([]string, 0, len(contexts))
	for _, ctx := range contexts {
		names = append(names, ctx.Name)
	}
	return names
}
