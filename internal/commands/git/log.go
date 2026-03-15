package git

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"tko/internal/commands"
)

func init() {
	commands.Register(&gitLogHandler{})
}

// logDisplayThreshold is the max commits shown before declaring lossy.
const logDisplayThreshold = 20

type gitLogHandler struct{}

func (h *gitLogHandler) Id() string    { return "git-log" }
func (h *gitLogHandler) Route() string { return "git" }

func (h *gitLogHandler) Supports(args []string) bool {
	sub, rest, ok := gitSubcommand(args)
	if !ok || sub != "log" {
		return false
	}
	oneline, maxCount, ok2 := parseLogFlags(rest)
	if !ok2 {
		return false
	}
	// Only handle cases we can guarantee lossless:
	//   --oneline (already compact, pass-through)
	//   -n N where N ≤ threshold (bounded output)
	// Plain `git log` with no limit passes through raw.
	return oneline || (maxCount > 0 && maxCount <= logDisplayThreshold)
}

func (h *gitLogHandler) Handle(args []string, rawStdout, _ string) (*commands.Result, error) {
	_, rest, _ := gitSubcommand(args)
	oneline, maxCount, _ := parseLogFlags(rest)
	return handleLog(rawStdout, oneline, maxCount)
}

// parseLogFlags parses args after "log". Returns (oneline, maxCount, ok).
// maxCount=-1 means no explicit limit. ok=false for unsupported flags/args.
func parseLogFlags(args []string) (oneline bool, maxCount int, ok bool) {
	maxCount = -1
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--oneline":
			oneline = true
		case arg == "-n" || arg == "--max-count":
			if i+1 >= len(args) {
				return false, 0, false
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n <= 0 {
				return false, 0, false
			}
			maxCount = n
			i++
		case strings.HasPrefix(arg, "--max-count="):
			n, err := strconv.Atoi(strings.TrimPrefix(arg, "--max-count="))
			if err != nil || n <= 0 {
				return false, 0, false
			}
			maxCount = n
		case len(arg) > 2 && arg[0] == '-' && arg[1] == 'n':
			// -n5 combined form
			n, err := strconv.Atoi(arg[2:])
			if err != nil || n <= 0 {
				return false, 0, false
			}
			maxCount = n
		case strings.HasPrefix(arg, "-"):
			return false, 0, false
		default:
			// Positional arg (commit range, path) — not supported.
			return false, 0, false
		}
	}
	return oneline, maxCount, true
}

type logEntry struct {
	hash    string
	date    string
	subject string
}

func handleLog(raw string, oneline bool, maxCount int) (*commands.Result, error) {
	if strings.TrimSpace(raw) == "" {
		return &commands.Result{Stdout: "log: 0 commits\n", Lossless: true}, nil
	}

	if oneline {
		// --oneline is already compact; strip trailing whitespace per line.
		lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
		var out strings.Builder
		for _, l := range lines {
			out.WriteString(strings.TrimRight(l, " \t"))
			out.WriteByte('\n')
		}
		return &commands.Result{Stdout: out.String(), Lossless: true}, nil
	}

	entries := parseLogEntries(raw)
	if len(entries) == 0 {
		return nil, fmt.Errorf("git log: could not parse output")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "log: %d commits\n", len(entries))
	for _, e := range entries {
		fmt.Fprintf(&sb, "%s %s %s\n", e.hash, e.date, e.subject)
	}

	return &commands.Result{
		Stdout:   sb.String(),
		Lossless: true,
	}, nil
}

// parseLogEntries parses the default git log output into entries.
// Format per commit:
//
//	commit <hash>
//	Author: ...
//	Date:   <date>
//	(blank)
//	    <subject>
func parseLogEntries(raw string) []logEntry {
	var entries []logEntry
	var current *logEntry
	inMessage := false

	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "commit ") {
			if current != nil && current.hash != "" {
				entries = append(entries, *current)
			}
			hash := strings.TrimPrefix(line, "commit ")
			// Strip decoration suffix like " (HEAD -> main)"
			if idx := strings.IndexByte(hash, ' '); idx >= 0 {
				hash = hash[:idx]
			}
			if len(hash) > 7 {
				hash = hash[:7]
			}
			current = &logEntry{hash: hash}
			inMessage = false
		} else if current == nil {
			continue
		} else if strings.HasPrefix(line, "Author:") ||
			strings.HasPrefix(line, "Merge:") ||
			strings.HasPrefix(line, "AuthorDate:") ||
			strings.HasPrefix(line, "Commit:") ||
			strings.HasPrefix(line, "CommitDate:") {
			// Skip header fields we don't need.
		} else if strings.HasPrefix(line, "Date:") {
			dateStr := strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
			current.date = parseGitDate(dateStr)
		} else if line == "" {
			inMessage = true
		} else if inMessage && current.subject == "" {
			if s := strings.TrimSpace(line); s != "" {
				current.subject = s
			}
		}
	}
	if current != nil && current.hash != "" {
		entries = append(entries, *current)
	}
	return entries
}

// parseGitDate parses git's author date format and returns "YYYY-MM-DD".
// Input example: "Thu Mar 15 10:23:01 2026 +0000"
func parseGitDate(s string) string {
	t, err := time.Parse("Mon Jan 2 15:04:05 2006 -0700", s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02")
}
