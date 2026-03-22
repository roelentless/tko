// Package find registers handlers for find and fd.
package find

import (
	"fmt"
	"strings"

	"tko/internal/commands"
)

func init() {
	commands.Register(&findHandler{})
}

type findHandler struct{}

func (h *findHandler) Id() string    { return "find" }
func (h *findHandler) Route() string { return "find" }

// findOutputModifiers are flags that cause find to produce non-path output.
var findOutputModifiers = map[string]bool{
	"-printf":  true,
	"-exec":    true,
	"-execdir": true,
	"-ok":      true,
	"-okdir":   true,
	"-delete":  true,
	"-ls":      true,
	"-print0":  true,
	"-fprintf": true,
	"-fprint":  true,
	"-fprint0": true,
}

func (h *findHandler) Supports(args []string) bool {
	if len(args) == 0 {
		return false
	}
	for _, arg := range args {
		if findOutputModifiers[arg] {
			return false
		}
	}
	return true
}

func (h *findHandler) Handle(args []string, rawStdout, _ string) (*commands.Result, error) {
	return handlePaths("find", typeLabel(args, "-type"), rawStdout)
}

// typeLabel returns "files", "dirs", or "items" based on a -flag f|d arg.
func typeLabel(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			switch args[i+1] {
			case "f":
				return "files"
			case "d":
				return "dirs"
			}
		}
	}
	return "items"
}

// handlePaths compresses a newline-separated list of paths by stripping the
// longest common directory prefix and emitting a compact header.
func handlePaths(cmd, label, raw string) (*commands.Result, error) {
	if strings.TrimSpace(raw) == "" {
		return &commands.Result{Stdout: "(empty)\n", Lossless: true}, nil
	}

	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	var paths []string
	for _, l := range lines {
		if l != "" {
			paths = append(paths, l)
		}
	}

	if len(paths) == 0 {
		return &commands.Result{Stdout: "(empty)\n", Lossless: true}, nil
	}

	prefix := commonDirPrefix(paths)

	var sb strings.Builder
	n := len(paths)
	if prefix != "" {
		fmt.Fprintf(&sb, "%s %s (%d %s):\n", cmd, prefix, n, label)
		for _, p := range paths {
			rel := strings.TrimPrefix(p, prefix)
			if rel == "" {
				rel = "."
			}
			fmt.Fprintln(&sb, rel)
		}
	} else {
		fmt.Fprintf(&sb, "%s (%d %s):\n", cmd, n, label)
		for _, p := range paths {
			fmt.Fprintln(&sb, p)
		}
	}

	return &commands.Result{Stdout: sb.String(), Lossless: true}, nil
}

// commonDirPrefix returns the longest common directory prefix (with trailing
// slash) shared by all paths. Returns "" if the only shared prefix is "/" or
// if there are no paths.
func commonDirPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	prefix := parentDir(paths[0])
	for _, p := range paths[1:] {
		parent := parentDir(p)
		prefix = sharedPrefix(prefix, parent)
		// Trim to the last "/" to stay at a directory boundary.
		if idx := strings.LastIndex(prefix, "/"); idx >= 0 {
			prefix = prefix[:idx+1]
		} else {
			prefix = ""
		}
		if prefix == "" || prefix == "/" {
			return ""
		}
	}
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
