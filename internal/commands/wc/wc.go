// Package wc registers a handler for wc -l.
package wc

import (
	"fmt"
	"strings"

	"tko/internal/commands"
)

func init() {
	commands.Register(&wcHandler{})
}

type wcHandler struct{}

func (h *wcHandler) Id() string    { return "wc-l" }
func (h *wcHandler) Route() string { return "wc" }

// Supports accepts wc -l <paths...>. Rejects combined counting flags (-lc, -lw
// etc.) because they change the output format.
func (h *wcHandler) Supports(args []string) bool {
	hasL := false
	hasPaths := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			return false
		}
		if strings.HasPrefix(arg, "-") {
			for _, c := range arg[1:] {
				if c == 'l' {
					hasL = true
				} else {
					return false
				}
			}
		} else {
			hasPaths = true
		}
	}
	return hasL && hasPaths
}

func (h *wcHandler) Handle(args []string, rawStdout, _ string) (*commands.Result, error) {
	return handleWC(rawStdout)
}

type wcEntry struct {
	count string
	path  string
}

func handleWC(raw string) (*commands.Result, error) {
	if strings.TrimSpace(raw) == "" {
		return &commands.Result{Stdout: raw, Lossless: true}, nil
	}

	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	var entries []wcEntry
	var total string

	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("wc: unexpected line format: %q", line)
		}
		if fields[1] == "total" {
			total = fields[0]
		} else {
			entries = append(entries, wcEntry{count: fields[0], path: fields[1]})
		}
	}

	// Single file: no total line — nothing to compress.
	if total == "" || len(entries) <= 1 {
		return &commands.Result{Stdout: raw, Lossless: true}, nil
	}

	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = e.path
	}
	prefix := commonDirPrefix(paths)

	var sb strings.Builder
	if prefix != "" {
		fmt.Fprintf(&sb, "wc -l %s (%d files, %s total):\n", prefix, len(entries), total)
		for _, e := range entries {
			rel := strings.TrimPrefix(e.path, prefix)
			if rel == "" {
				rel = "."
			}
			fmt.Fprintf(&sb, "  %s  %s\n", e.count, rel)
		}
	} else {
		fmt.Fprintf(&sb, "wc -l (%d files, %s total):\n", len(entries), total)
		for _, e := range entries {
			fmt.Fprintf(&sb, "  %s  %s\n", e.count, e.path)
		}
	}

	return &commands.Result{Stdout: sb.String(), Lossless: true}, nil
}

func commonDirPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	prefix := parentDir(paths[0])
	for _, p := range paths[1:] {
		parent := parentDir(p)
		prefix = sharedPrefix(prefix, parent)
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

func parentDir(path string) string {
	path = strings.TrimRight(path, "/")
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx+1]
	}
	return ""
}

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
