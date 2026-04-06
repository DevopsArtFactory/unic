package app

import "strings"

// Filterable is implemented by any type that supports text-based filtering.
type Filterable interface {
	FilterText() string
}

// applyFilter returns items matching the query via case-insensitive substring on FilterText.
// Returns the full slice unchanged if query is empty.
func applyFilter[T Filterable](items []T, query string) []T {
	if query == "" {
		return items
	}
	q := strings.ToLower(query)
	var result []T
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.FilterText()), q) {
			result = append(result, item)
		}
	}
	return result
}

// handleFilterKey processes a keystroke while filter input is active.
// Returns the updated filter string, whether to deactivate filter mode, and whether the filter changed.
func handleFilterKey(key, filter string) (newFilter string, deactivate bool, changed bool) {
	switch key {
	case "esc", "enter":
		return filter, true, false
	case "backspace":
		if len(filter) > 0 {
			return filter[:len(filter)-1], false, true
		}
		return filter, false, false
	default:
		if len(key) == 1 {
			return filter + key, false, true
		}
		return filter, false, false
	}
}
