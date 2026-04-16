package inspector

import "strings"

func normalizedSortKey(values ...string) string {
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key != "" {
			return key
		}
	}
	return ""
}
