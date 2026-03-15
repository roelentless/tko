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
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return "", false
	}

	// Never double-wrap
	if strings.HasPrefix(trimmed, "tko ") || trimmed == "tko" {
		return "", false
	}

	// Skip compound/piped/redirected commands — too risky to rewrite
	for _, tok := range []string{"&&", "||", ";", "|", "\n", "`", "$(", ">"} {
		if strings.Contains(trimmed, tok) {
			return "", false
		}
	}

	// Extract command name from first word, stripping any directory prefix
	parts := strings.Fields(trimmed)
	name := filepath.Base(parts[0]) // /usr/bin/git → git

	if _, ok := Match(name, parts[1:]); !ok {
		return "", false
	}

	return "tko -- " + trimmed, true
}
