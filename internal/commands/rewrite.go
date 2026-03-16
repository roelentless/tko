package commands

import (
	"path/filepath"
	"strings"
)

// Rewrite returns the tko-prefixed version of cmd if we have a handler for
// the command. Returns ("", false) if no rewrite is needed.
//
// Only simple commands are rewritten — compound expressions (&&, ||, ;, |)
// are passed through unchanged to avoid breaking shell logic.
func Rewrite(cmd string) (string, bool) {
	name, args, trimmed, ok := parseSimple(cmd)
	if !ok {
		return "", false
	}
	if _, ok := Match(name, args); !ok {
		return "", false
	}
	return "tko -- " + trimmed, true
}

// ParseSimple parses cmd into (binary, args, full) if it is a simple,
// non-compound, non-tko command. Returns ok=false for empty, double-wrapped,
// or compound commands. Use this to decide whether a no-handler result is a
// recordable miss.
func ParseSimple(cmd string) (name string, args []string, full string, ok bool) {
	return parseSimple(cmd)
}

func parseSimple(cmd string) (name string, args []string, full string, ok bool) {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return "", nil, "", false
	}

	// Never double-wrap
	if strings.HasPrefix(trimmed, "tko ") || trimmed == "tko" {
		return "", nil, "", false
	}

	// Skip compound/piped/redirected commands — too risky to rewrite
	for _, tok := range []string{"&&", "||", ";", "|", "\n", "`", "$(", ">"} {
		if strings.Contains(trimmed, tok) {
			return "", nil, "", false
		}
	}

	parts := strings.Fields(trimmed)
	return filepath.Base(parts[0]), parts[1:], trimmed, true
}
