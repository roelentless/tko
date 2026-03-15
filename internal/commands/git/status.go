// Package git registers handlers for git subcommands.
package git

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"tko/internal/commands"
)

func init() {
	commands.Register(&gitStatusHandler{})
}

// gitStatusHandler handles plain `git status` (no extra args or flags).
type gitStatusHandler struct{}

func (h *gitStatusHandler) Id() string    { return "git-status" }
func (h *gitStatusHandler) Route() string { return "git" }

func (h *gitStatusHandler) Supports(args []string) bool {
	sub, rest, ok := gitSubcommand(args)
	return ok && sub == "status" && len(rest) == 0
}

func (h *gitStatusHandler) Handle(args []string, rawStdout, _ string) (*commands.Result, error) {
	return handleStatus(rawStdout)
}

// --- git status ---------------------------------------------------------------

func handleStatus(raw string) (*commands.Result, error) {
	s, err := parseStatus(raw)
	if err != nil {
		return nil, err
	}
	return &commands.Result{
		Stdout:   formatStatus(s),
		Lossless: true,
	}, nil
}

// statusResult holds all parsed data from `git status` output.
type statusResult struct {
	branch    string
	upstream  string
	ahead     int
	behind    int
	detached  bool
	detachRef string
	staged    []fileEntry
	unstaged  []fileEntry
	untracked []string
}

// fileEntry represents a single file in staged/unstaged sections.
type fileEntry struct {
	symbol   string // ~, +, -, R, C
	path     string // for R/C: "old→new"
	groupable bool   // false for renamed/copied
}

type section int

const (
	secNone section = iota
	secStaged
	secUnstaged
	secUntracked
)

var (
	reBranch   = regexp.MustCompile(`^On branch (.+)$`)
	reDetached = regexp.MustCompile(`^HEAD detached at (.+)$`)
	reUpstream = regexp.MustCompile(`'([^']+)'`)
	reAhead    = regexp.MustCompile(`ahead of .+? by (\d+)`)
	reBehind   = regexp.MustCompile(`behind .+? by (\d+)`)
	reDiverged = regexp.MustCompile(`have (\d+) and (\d+) different commits`)

	reModified = regexp.MustCompile(`^\t(?:modified|both modified):\s+(.+)$`)
	reNewFile  = regexp.MustCompile(`^\tnew file:\s+(.+)$`)
	reDeleted  = regexp.MustCompile(`^\tdeleted:\s+(.+)$`)
	reRenamed  = regexp.MustCompile(`^\trenamed:\s+(.+) -> (.+)$`)
	reCopied   = regexp.MustCompile(`^\tcopied:\s+(.+) -> (.+)$`)
)

func parseStatus(output string) (*statusResult, error) {
	r := &statusResult{}
	sec := secNone

	for _, line := range strings.Split(output, "\n") {
		switch {
		case reBranch.MatchString(line):
			r.branch = reBranch.FindStringSubmatch(line)[1]

		case reDetached.MatchString(line):
			r.detached = true
			r.detachRef = reDetached.FindStringSubmatch(line)[1]

		case strings.Contains(line, "Your branch") || strings.Contains(line, "have diverged"):
			if m := reUpstream.FindStringSubmatch(line); m != nil {
				r.upstream = m[1]
			}
			if m := reAhead.FindStringSubmatch(line); m != nil {
				r.ahead, _ = strconv.Atoi(m[1])
			}
			if m := reBehind.FindStringSubmatch(line); m != nil {
				r.behind, _ = strconv.Atoi(m[1])
			}

		case strings.Contains(line, "different commits each"):
			// "and have 3 and 1 different commits each, respectively."
			if m := reDiverged.FindStringSubmatch(line); m != nil {
				r.ahead, _ = strconv.Atoi(m[1])
				r.behind, _ = strconv.Atoi(m[2])
			}

		case line == "Changes to be committed:":
			sec = secStaged
		case line == "Changes not staged for commit:":
			sec = secUnstaged
		case line == "Untracked files:":
			sec = secUntracked

		// Instruction lines — skip
		case strings.HasPrefix(line, "  ("):
			continue

		// File entries
		case strings.HasPrefix(line, "\t"):
			switch sec {
			case secStaged, secUnstaged:
				if e := parseFileEntry(line); e != nil {
					if sec == secStaged {
						r.staged = append(r.staged, *e)
					} else {
						r.unstaged = append(r.unstaged, *e)
					}
				}
			case secUntracked:
				// Skip instruction lines inside untracked section
				if !strings.HasPrefix(line, "\t(") {
					r.untracked = append(r.untracked, strings.TrimPrefix(line, "\t"))
				}
			}
		}
	}

	return r, nil
}

func parseFileEntry(line string) *fileEntry {
	if m := reModified.FindStringSubmatch(line); m != nil {
		return &fileEntry{symbol: "~", path: m[1], groupable: true}
	}
	if m := reNewFile.FindStringSubmatch(line); m != nil {
		return &fileEntry{symbol: "+", path: m[1], groupable: true}
	}
	if m := reDeleted.FindStringSubmatch(line); m != nil {
		return &fileEntry{symbol: "-", path: m[1], groupable: true}
	}
	if m := reRenamed.FindStringSubmatch(line); m != nil {
		return &fileEntry{symbol: "R", path: m[1] + "→" + m[2], groupable: false}
	}
	if m := reCopied.FindStringSubmatch(line); m != nil {
		return &fileEntry{symbol: "C", path: m[1] + "→" + m[2], groupable: false}
	}
	return nil
}

// --- formatting ---------------------------------------------------------------

func formatStatus(s *statusResult) string {
	var sb strings.Builder

	// Branch line
	if s.detached {
		fmt.Fprintf(&sb, "branch:HEAD@%s", s.detachRef)
	} else {
		fmt.Fprintf(&sb, "branch:%s", s.branch)
		if s.upstream != "" {
			fmt.Fprintf(&sb, "=%s", s.upstream)
		}
	}
	switch {
	case s.ahead > 0 && s.behind > 0:
		fmt.Fprintf(&sb, " ↑%d↓%d", s.ahead, s.behind)
	case s.ahead > 0:
		fmt.Fprintf(&sb, " ↑%d", s.ahead)
	case s.behind > 0:
		fmt.Fprintf(&sb, " ↓%d", s.behind)
	}

	if len(s.staged) == 0 && len(s.unstaged) == 0 && len(s.untracked) == 0 {
		sb.WriteString(" clean")
		return sb.String()
	}

	sb.WriteString("\n")

	if len(s.staged) > 0 {
		fmt.Fprintf(&sb, "staged(%d):\n%s\n", len(s.staged), formatEntriesYAML(s.staged))
	}
	if len(s.unstaged) > 0 {
		fmt.Fprintf(&sb, "unstaged(%d):\n%s\n", len(s.unstaged), formatEntriesYAML(s.unstaged))
	}
	if len(s.untracked) > 0 {
		fmt.Fprintf(&sb, "untracked(%d): %s\n", len(s.untracked), groupPaths(s.untracked))
	}

	return strings.TrimRight(sb.String(), "\n")
}

// symbolVerb maps a file entry symbol to a human-readable verb.
func symbolVerb(sym string) string {
	switch sym {
	case "~":
		return "modified"
	case "+":
		return "new"
	case "-":
		return "deleted"
	case "R":
		return "renamed"
	case "C":
		return "copied"
	default:
		return sym
	}
}

// formatEntriesYAML renders file entries grouped by verb with YAML-like indentation:
//
//	  modified: pkg/{foo,bar}.go main.go
//	  new: pkg/baz.go
//	  renamed: old.go→new.go
func formatEntriesYAML(entries []fileEntry) string {
	var verbOrder []string
	verbPaths := map[string][]string{}  // verb → groupable paths
	verbDirect := map[string][]string{} // verb → non-groupable (renamed/copied)
	seen := map[string]bool{}

	for _, e := range entries {
		verb := symbolVerb(e.symbol)
		if !seen[verb] {
			seen[verb] = true
			verbOrder = append(verbOrder, verb)
		}
		if e.groupable {
			verbPaths[verb] = append(verbPaths[verb], e.path)
		} else {
			verbDirect[verb] = append(verbDirect[verb], e.path)
		}
	}

	var lines []string
	for _, verb := range verbOrder {
		var parts []string
		if paths := verbPaths[verb]; len(paths) > 0 {
			parts = append(parts, groupFilePaths(paths))
		}
		parts = append(parts, verbDirect[verb]...)
		lines = append(lines, "  "+verb+": "+strings.Join(parts, " "))
	}
	return strings.Join(lines, "\n")
}

// groupFilePaths groups file paths by (parent_dir, extension) using brace notation.
func groupFilePaths(paths []string) string {
	type groupKey struct {
		dir string
		ext string
	}
	type groupVal struct {
		stems []string
		order int
	}

	var order []groupKey
	groups := map[groupKey]*groupVal{}

	for _, p := range paths {
		dir := filepath.Dir(p)
		base := filepath.Base(p)
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)

		key := groupKey{dir: dir, ext: ext}
		if g, ok := groups[key]; ok {
			g.stems = append(g.stems, stem)
		} else {
			groups[key] = &groupVal{stems: []string{stem}, order: len(order)}
			order = append(order, key)
		}
	}

	parts := make([]string, len(order))
	for _, key := range order {
		g := groups[key]
		parts[g.order] = joinGroup(key.dir, g.stems, key.ext, "")
	}
	return strings.Join(parts, " ")
}

// groupPaths groups plain paths (untracked files/dirs) by parent directory.
func groupPaths(paths []string) string {
	type groupKey struct {
		dir      string
		trailing bool // dir entries end with /
	}
	type groupVal struct {
		names []string
		order int
	}

	var order []groupKey
	groups := map[groupKey]*groupVal{}

	for _, p := range paths {
		trailing := strings.HasSuffix(p, "/")
		clean := strings.TrimSuffix(p, "/")
		dir := filepath.Dir(clean)
		name := filepath.Base(clean)

		key := groupKey{dir: dir, trailing: trailing}
		if g, ok := groups[key]; ok {
			g.names = append(g.names, name)
		} else {
			groups[key] = &groupVal{names: []string{name}, order: len(order)}
			order = append(order, key)
		}
	}

	parts := make([]string, len(order))
	for _, key := range order {
		g := groups[key]
		suffix := ""
		if key.trailing {
			suffix = "/"
		}
		parts[g.order] = joinGroup(key.dir, g.names, "", suffix)
	}
	return strings.Join(parts, " ")
}

// joinGroup builds "dir/{a,b}ext" or "dir/aext" depending on group size.
func joinGroup(dir string, names []string, ext, suffix string) string {
	var base string
	if len(names) == 1 {
		base = names[0] + ext + suffix
	} else {
		base = "{" + strings.Join(names, ",") + "}" + ext + suffix
	}
	if dir == "." {
		return base
	}
	return dir + "/" + base
}
