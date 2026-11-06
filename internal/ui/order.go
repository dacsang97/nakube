package ui

import (
	"strings"
	"time"
)

// CompareText compares a and b, ignoring case. Descending flips the order.
func CompareText(a, b string, desc bool) int {
	c := strings.Compare(strings.ToLower(a), strings.ToLower(b))
	if desc {
		return -c
	}
	return c
}

// CompareFloat compares a and b. Descending flips the order.
func CompareFloat(a, b float64, desc bool) int {
	if a == b {
		return 0
	}
	less := a < b
	if desc {
		less = a > b
	}
	if less {
		return -1
	}
	return 1
}

// CompareTime compares a and b. A zero time sorts after a real one, in
// either direction, so a missing clock does not look newest.
func CompareTime(a, b time.Time, desc bool) int {
	switch {
	case a.IsZero() && b.IsZero():
		return 0
	case a.IsZero():
		return 1
	case b.IsZero():
		return -1
	case a.Equal(b):
		return 0
	}
	less := a.Before(b)
	if desc {
		less = a.After(b)
	}
	if less {
		return -1
	}
	return 1
}

// CompareKnown compares measurements. An unknown value sorts after a known
// one, in either direction.
func CompareKnown(aOK, bOK bool, a, b float64, desc bool) int {
	if aOK != bOK {
		if aOK {
			return -1
		}
		return 1
	}
	if !aOK {
		return 0
	}
	return CompareFloat(a, b, desc)
}

// Place keeps a selection on key when rows are filtered. An unknown key
// selects nothing, so a hidden row cannot be acted on. An empty key keeps
// selected only while it still points at a row.
func Place(selected int, key string, keys []string) int {
	if key != "" {
		for i, item := range keys {
			if item == key {
				return i
			}
		}
		return -1
	}
	if selected >= 0 && selected < len(keys) {
		return selected
	}
	return -1
}
