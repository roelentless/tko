package commands

import "path/filepath"

// ignoredBinaries are command names that should never be tracked as misses.
//
// These fall into two categories:
//
//  1. Content-display commands — the raw output IS the value; compression is
//     lossy by definition (cat, head, tail, grep, sed, awk).
//
//  2. Side-effect-only commands — produce little or no stdout; nothing to
//     compress (rm, mkdir, chmod, mv, cp, touch, ln, kill, echo, ...).
//
// Script runners (bash, python, node, ...) are also excluded: their output is
// arbitrary and cannot be predicted or compressed safely.
var ignoredBinaries = map[string]bool{
	// content display
	"cat":  true,
	"head": true,
	"tail": true,
	// stream processing
	"grep": true,
	"rg":   true,
	"awk":  true,
	"sed":  true,
	"xargs": true,
	// file/dir operations (no stdout)
	"rm":    true,
	"rmdir": true,
	"mkdir": true,
	"chmod": true,
	"chown": true,
	"mv":    true,
	"cp":    true,
	"touch": true,
	"ln":    true,
	// output primitives
	"echo":   true,
	"printf": true,
	// process management
	"kill":    true,
	"pkill":   true,
	"killall": true,
	// network — raw response content
	"curl": true,
	"wget": true,
	// script / program runners — arbitrary output
	"bash":    true,
	"sh":      true,
	"zsh":     true,
	"fish":    true,
	"python":  true,
	"python3": true,
	"node":    true,
	"ruby":    true,
	"perl":    true,
}

// IsIgnored reports whether cmd should be excluded from miss tracking.
func IsIgnored(cmd string) bool {
	return ignoredBinaries[filepath.Base(cmd)]
}
