package auth

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/term"
)

type pickerState struct {
	query    string
	filtered []int
	cursor   int
}

func interactiveChoiceIO(in io.Reader, errOut io.Writer) (*os.File, *os.File, bool) {
	file, ok := in.(*os.File)
	if !ok {
		return nil, nil, false
	}

	outFile, ok := errOut.(*os.File)
	if !ok {
		return nil, nil, false
	}

	if !term.IsTerminal(file.Fd()) || !term.IsTerminal(outFile.Fd()) {
		return nil, nil, false
	}

	return file, outFile, true
}

func chooseInteractiveIndex[T any](
	in *os.File,
	errOut *os.File,
	label string,
	items []T,
	render func(T) string,
	matches func(T, string) bool,
) (int, error) {
	oldState, err := term.MakeRaw(in.Fd())
	if err != nil {
		return 0, err
	}
	defer term.Restore(in.Fd(), oldState)
	defer fmt.Fprint(errOut, "\x1b[2J\x1b[H\x1b[?25h")

	reader := bufio.NewReader(in)
	state := newPickerState(len(items))
	renderInteractivePicker(errOut, label, items, render, state)
	fmt.Fprint(errOut, "\x1b[?25l")

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			return 0, err
		}

		switch r {
		case '\r', '\n':
			if len(state.filtered) == 0 {
				continue
			}
			return state.filtered[state.cursor], nil
		case 3:
			return 0, io.EOF
		case 8, 127:
			deleteLastPickerRune(&state, items, matches)
		case 21:
			clearPickerQuery(&state, items, matches)
		case 27:
			next, _, err := reader.ReadRune()
			if err != nil {
				return 0, err
			}
			if next != '[' {
				continue
			}
			arrow, _, err := reader.ReadRune()
			if err != nil {
				return 0, err
			}
			switch arrow {
			case 'A':
				state.moveUp()
			case 'B':
				state.moveDown()
			}
		default:
			if unicode.IsControl(r) {
				continue
			}
			appendPickerRune(&state, items, matches, r)
		}

		renderInteractivePicker(errOut, label, items, render, state)
	}
}

func newPickerState(count int) pickerState {
	filtered := make([]int, count)
	for i := 0; i < count; i++ {
		filtered[i] = i
	}
	return pickerState{filtered: filtered}
}

func (s *pickerState) moveUp() {
	if len(s.filtered) == 0 || s.cursor == 0 {
		return
	}
	s.cursor--
}

func (s *pickerState) moveDown() {
	if len(s.filtered) == 0 || s.cursor >= len(s.filtered)-1 {
		return
	}
	s.cursor++
}

func clearPickerQuery[T any](s *pickerState, items []T, matches func(T, string) bool) {
	s.query = ""
	applyPickerFilter(s, items, matches)
}

func appendPickerRune[T any](s *pickerState, items []T, matches func(T, string) bool, r rune) {
	s.query += string(r)
	applyPickerFilter(s, items, matches)
}

func deleteLastPickerRune[T any](s *pickerState, items []T, matches func(T, string) bool) {
	if s.query == "" {
		return
	}
	_, size := utf8.DecodeLastRuneInString(s.query)
	s.query = s.query[:len(s.query)-size]
	applyPickerFilter(s, items, matches)
}

func applyPickerFilter[T any](s *pickerState, items []T, matches func(T, string) bool) {
	s.filtered = s.filtered[:0]
	query := strings.TrimSpace(s.query)
	for i, item := range items {
		if query == "" || matches(item, query) {
			s.filtered = append(s.filtered, i)
		}
	}

	if len(s.filtered) == 0 {
		s.cursor = 0
		return
	}
	if s.cursor >= len(s.filtered) {
		s.cursor = len(s.filtered) - 1
	}
}

func renderInteractivePicker[T any](out io.Writer, label string, items []T, render func(T) string, state pickerState) {
	fmt.Fprint(out, "\x1b[2J\x1b[H")
	height := 24
	if file, ok := out.(*os.File); ok {
		if _, h, err := term.GetSize(file.Fd()); err == nil && h > 0 {
			height = h
		}
	}
	if height <= 0 {
		height = 24
	}

	fmt.Fprintf(out, "Select %s\r\n", strings.TrimSuffix(label, "s"))
	if state.query == "" {
		fmt.Fprint(out, "Filter: type to filter\r\n")
	} else {
		fmt.Fprintf(out, "Filter: %s\r\n", state.query)
	}
	fmt.Fprint(out, "Keys: type to filter, ↑/↓ move, Enter select, Backspace delete, Ctrl+U clear\r\n")
	fmt.Fprint(out, "\r\n")

	if len(state.filtered) == 0 {
		fmt.Fprintf(out, "  No %s match %q\r\n", label, state.query)
		return
	}

	indexWidth := len(strconv.Itoa(len(state.filtered)))
	headerLines := 4
	footerLines := 2
	visibleRows := height - headerLines - footerLines
	if visibleRows < 5 {
		visibleRows = 5
	}
	start, end := pickerWindow(len(state.filtered), state.cursor, visibleRows)

	for i := start; i < end; i++ {
		originalIdx := state.filtered[i]
		marker := " "
		if i == state.cursor {
			marker = ">"
		}
		fmt.Fprintf(out, "%s %*d) %s\r\n", marker, indexWidth, i+1, render(items[originalIdx]))
	}

	if len(state.filtered) > visibleRows {
		fmt.Fprint(out, "\r\n")
		fmt.Fprintf(out, "Showing %d-%d of %d\r\n", start+1, end, len(state.filtered))
	}
}

func pickerWindow(total, cursor, visibleRows int) (int, int) {
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
