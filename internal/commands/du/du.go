// Package du registers a handler for du -sh.
package du

import (
	"fmt"
	"strings"

	"tko/internal/commands"
)

func init() {
	commands.Register(&duHandler{})
}

type duHandler struct{}

func (h *duHandler) Id() string    { return "du" }
func (h *duHandler) Route() string { return "du" }

func (h *duHandler) Supports(args []string) bool {
	return parseDUFlags(args)
}

func (h *duHandler) Handle(args []string, rawStdout, _ string) (*commands.Result, error) {
	return handleDU(rawStdout)
}

// parseDUFlags returns true if args contain only -s and -h flags (in any
// combination) plus optional path arguments. Both -s and -h must be present.
func parseDUFlags(args []string) bool {
	hasS, hasH := false, false
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			return false
		}
		if strings.HasPrefix(arg, "-") {
			for _, c := range arg[1:] {
				switch c {
				case 's':
					hasS = true
				case 'h':
					hasH = true
				default:
					return false
				}
			}
		}
		// Non-flag: path argument — accepted.
	}
	return hasS && hasH
}

type duEntry struct {
	size string
	path string
}

func handleDU(raw string) (*commands.Result, error) {
	if strings.TrimSpace(raw) == "" {
		return &commands.Result{Stdout: "(empty)\n", Lossless: true}, nil
	}

	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	var entries []duEntry
	for _, line := range lines {
		if line == "" {
			continue
		}
		idx := strings.IndexByte(line, '\t')
		if idx < 0 {
			return nil, fmt.Errorf("du: unexpected line format: %q", line)
		}
		entries = append(entries, duEntry{
			size: strings.TrimSpace(line[:idx]),
			path: line[idx+1:],
		})
	}

	if len(entries) == 0 {
		return &commands.Result{Stdout: "(empty)\n", Lossless: true}, nil
	}

	prefix := longestCommonDirPrefix(entries)

	var sb strings.Builder
	if prefix != "" {
		fmt.Fprintf(&sb, "du %s (%d items):\n", prefix, len(entries))
		for _, e := range entries {
			rel := strings.TrimPrefix(e.path, prefix)
			if rel == "" {
				rel = "."
			}
			fmt.Fprintf(&sb, "  %-6s  %s\n", e.size, rel)
		}
	} else {
		fmt.Fprintf(&sb, "du (%d items):\n", len(entries))
		for _, e := range entries {
			fmt.Fprintf(&sb, "  %-6s  %s\n", e.size, e.path)
		}
	}

	return &commands.Result{Stdout: sb.String(), Lossless: true}, nil
}

// longestCommonDirPrefix finds the longest directory prefix shared by all
// entry paths. Returns "" if there is no meaningful common prefix (i.e. only
// root "/" in common).
func longestCommonDirPrefix(entries []duEntry) string {
	if len(entries) == 0 {
		return ""
	}

	// Start with the parent directory of the first path.
	prefix := parentDir(entries[0].path)
	for _, e := range entries[1:] {
		parent := parentDir(e.path)
		prefix = sharedPrefix(prefix, parent)
		// Trim to directory boundary.
		if idx := strings.LastIndex(prefix, "/"); idx >= 0 {
			prefix = prefix[:idx+1]
		} else {
			prefix = ""
		}
	}

	// Don't bother stripping root "/".
	if prefix == "/" {
		return ""
	}
	return prefix
}

// parentDir returns the directory portion of path, always ending in "/".
func parentDir(path string) string {
	path = strings.TrimRight(path, "/")
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx+1]
	}
	return ""
}

// sharedPrefix returns the longest common byte prefix of a and b.
func sharedPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:n]
}
