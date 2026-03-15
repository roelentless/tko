package git

import (
	"fmt"
	"strings"

	"tko/internal/commands"
)

func init() {
	commands.Register(&gitShowHandler{})
}

type gitShowHandler struct{}

func (h *gitShowHandler) Id() string    { return "git-show" }
func (h *gitShowHandler) Route() string { return "git" }

func (h *gitShowHandler) Supports(args []string) bool {
	sub, rest, ok := gitSubcommand(args)
	if !ok || sub != "show" {
		return false
	}
	return supportsShow(rest)
}

func (h *gitShowHandler) Handle(args []string, rawStdout, _ string) (*commands.Result, error) {
	return handleShow(rawStdout)
}

// supportsShow returns true for: git show, git show <hash>, git show <hash> -- <path>.
// Rejects any flags (--stat, --name-only, etc.) and commit:path notation.
func supportsShow(args []string) bool {
	sawRef := false
	sawDashDash := false
	for _, arg := range args {
		if sawDashDash {
			// Anything after -- is a path: fine.
			continue
		}
		if arg == "--" {
			sawDashDash = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return false
		}
		// Positional: commit ref. Reject commit:path (shows raw file content).
		if strings.Contains(arg, ":") {
			return false
		}
		if sawRef {
			// Second positional before -- is unusual; reject.
			return false
		}
		sawRef = true
	}
	return true
}

// handleShow compresses git show output: commit header + diff.
func handleShow(raw string) (*commands.Result, error) {
	if strings.TrimSpace(raw) == "" {
		return &commands.Result{Stdout: "", Lossless: true}, nil
	}

	// Split at the first diff --git line.
	diffIdx := strings.Index(raw, "\ndiff --git ")
	var header, diffPart string
	if diffIdx >= 0 {
		header = raw[:diffIdx]
		diffPart = raw[diffIdx+1:]
	} else {
		header = raw
	}

	// Parse commit header fields.
	var hash, author, date, subject string
	inMsg := false
	for _, line := range strings.Split(header, "\n") {
		switch {
		case strings.HasPrefix(line, "commit "):
			h := strings.TrimPrefix(line, "commit ")
			if idx := strings.IndexByte(h, ' '); idx >= 0 {
				h = h[:idx]
			}
			if len(h) > 7 {
				h = h[:7]
			}
			hash = h
		case strings.HasPrefix(line, "Author:"):
			author = strings.TrimSpace(strings.TrimPrefix(line, "Author:"))
		case strings.HasPrefix(line, "Date:"):
			dateStr := strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
			date = parseGitDate(dateStr)
		case strings.HasPrefix(line, "Merge:"),
			strings.HasPrefix(line, "AuthorDate:"),
			strings.HasPrefix(line, "Commit:"),
			strings.HasPrefix(line, "CommitDate:"):
			// Skip.
		case line == "":
			inMsg = true
		case inMsg && subject == "":
			if s := strings.TrimSpace(line); s != "" {
				subject = s
			}
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "commit %s\n", hash)
	if author != "" || date != "" {
		fmt.Fprintf(&sb, "author: %s  date: %s\n", author, date)
	}
	if subject != "" {
		fmt.Fprintf(&sb, "    %s\n", subject)
	}

	if diffPart == "" {
		return &commands.Result{Stdout: sb.String(), Lossless: true}, nil
	}

	diffResult, err := handleDiff(diffPart)
	if err != nil {
		return nil, err
	}
	if !diffResult.Lossless {
		// Diff is too large to compress losslessly; fall back to raw passthrough.
		return nil, fmt.Errorf("git show: diff exceeds lossless threshold")
	}

	sb.WriteByte('\n')
	sb.WriteString(diffResult.Stdout)

	return &commands.Result{
		Stdout:   sb.String(),
		Lossless: true,
	}, nil
}
