package ui

import "strings"

// Match reports whether query is empty or contained in any field. The
// comparison is case-insensitive and ignores surrounding space.
func Match(query string, fields ...string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

// Count is the list count, or "shown of total" when a search or filter
// hides rows.
func Count(shown, total int) string {
	if shown == total {
		return itoa(total)
	}
	return itoa(shown) + " of " + itoa(total)
}
