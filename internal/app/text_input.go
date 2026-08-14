package app

import tea "github.com/charmbracelet/bubbletea"

// Shared helpers for the manually managed text buffers used by form fields,
// type-to-confirm flows, filters, and lookup inputs. Both are rune-safe so
// multibyte input (Korean, emoji, ...) survives editing.

// trimLastRune removes the final rune from a buffer.
func trimLastRune(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	return string(runes[:len(runes)-1])
}

// appendKeyRunes appends the typed runes from a key message to a buffer.
func appendKeyRunes(s string, msg tea.KeyMsg) string {
	if runes := msg.Runes; len(runes) > 0 {
		return s + string(runes)
	}
	return s
}
