// Package ls registers a handler for the ls command.
package ls

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"tko/internal/commands"
)

func init() {
	commands.Register(&lsHandler{})
}

type lsHandler struct{}

func (h *lsHandler) Id() string    { return "ls" }
func (h *lsHandler) Route() string { return "ls" }

func (h *lsHandler) Supports(args []string) bool {
	_, ok := parseLSFlags(args)
	return ok
}

func (h *lsHandler) Handle(args []string, rawStdout, _ string) (*commands.Result, error) {
	longFormat, _ := parseLSFlags(args)
	return handleLS(rawStdout, longFormat)
}

// parseLSFlags parses ls args. Only -l, -a, -A (and combinations like -la, -al)
// are accepted. Returns (longFormat, ok). Rejects --color, -R, and anything else.
func parseLSFlags(args []string) (longFormat bool, ok bool) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			return false, false
		}
		if strings.HasPrefix(arg, "-") {
			flags := arg[1:]
			for _, c := range flags {
				switch c {
				case 'l':
					longFormat = true
				case 'a', 'A':
					// Accepted.
				default:
					return false, false
				}
			}
		}
		// Non-flag: path argument, accepted.
	}
	return longFormat, true
}

// handleLS compresses ls output.
func handleLS(raw string, longFormat bool) (*commands.Result, error) {
	if strings.TrimSpace(raw) == "" {
		return &commands.Result{Stdout: "(empty)\n", Lossless: true}, nil
	}

	if longFormat {
		return handleLSLong(raw)
	}
	return handleLSPlain(raw)
}

// handleLSPlain compresses plain ls output (one filename per line).
func handleLSPlain(raw string) (*commands.Result, error) {
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	var names []string
	for _, l := range lines {
		if n := strings.TrimSpace(l); n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return &commands.Result{Stdout: "(empty)\n", Lossless: true}, nil
	}
	out := strings.Join(names, " ") + fmt.Sprintf("  (%d items)\n", len(names))
	return &commands.Result{Stdout: out, Lossless: true}, nil
}

// handleLSLong compresses ls -l / ls -la output.
func handleLSLong(raw string) (*commands.Result, error) {
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")

	var dirs, files, hidden []string
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		perm := fields[0]
		name := strings.Join(fields[8:], " ")

		// Skip . and ..
		if name == "." || name == ".." {
			continue
		}

		isDir := len(perm) > 0 && perm[0] == 'd'
		isHidden := strings.HasPrefix(name, ".")

		switch {
		case isDir && isHidden:
			hidden = append(hidden, name+"/")
		case isDir:
			dirs = append(dirs, name+"/")
		case isHidden:
			hidden = append(hidden, name)
		default:
			files = append(files, name)
		}
	}

	total := len(dirs) + len(files) + len(hidden)
	if total == 0 {
		return &commands.Result{Stdout: "total: 0 items\n", Lossless: true}, nil
	}

	var sb strings.Builder
	if len(dirs) > 0 {
		fmt.Fprintf(&sb, "dirs(%d): %s\n", len(dirs), strings.Join(dirs, " "))
	}
	if len(files) > 0 {
		fmt.Fprintf(&sb, "files(%d): %s\n", len(files), groupFileNames(files))
	}
	if len(hidden) > 0 {
		fmt.Fprintf(&sb, "hidden(%d): %s\n", len(hidden), strings.Join(hidden, " "))
	}
	fmt.Fprintf(&sb, "total: %d items\n", total)

	return &commands.Result{Stdout: sb.String(), Lossless: true}, nil
}

// groupFileNames groups filenames by extension.
// Files sharing the same extension are shown as *.ext(N).
// Files with unique extensions or no extension are listed as-is.
func groupFileNames(names []string) string {
	type group struct {
		ext   string
		count int
		first string // for single-file groups
	}

	extCount := map[string]int{}
	extOrder := []string{}
	noExt := []string{}

	for _, name := range names {
		ext := filepath.Ext(name)
		if ext == "" || ext == name {
			noExt = append(noExt, name)
			continue
		}
		if extCount[ext] == 0 {
			extOrder = append(extOrder, ext)
		}
		extCount[ext]++
	}

	// Sort extensions for deterministic output.
	sort.Strings(extOrder)

	var parts []string
	for _, ext := range extOrder {
		n := extCount[ext]
		if n == 1 {
			// Find the single file with this extension.
			for _, name := range names {
				if filepath.Ext(name) == ext {
					parts = append(parts, name)
					break
				}
			}
		} else {
			parts = append(parts, fmt.Sprintf("*%s(%d)", ext, n))
		}
	}
	parts = append(parts, noExt...)
	return strings.Join(parts, " ")
}
