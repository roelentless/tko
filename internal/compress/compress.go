package compress

import "strings"

// TokenCount returns an approximate token count for the given string.
// Uses a chars/4 heuristic — good enough for measuring relative compression.
func TokenCount(s string) int {
	if len(s) == 0 {
		return 0
	}
	return (len(s) + 3) / 4
}

// LineCount returns the number of lines in s.
func LineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
