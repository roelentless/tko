package git

import (
	"fmt"
	"strings"

	"tko/internal/commands"
)

// diffFileTruncateLines is the max diff lines (hunks + context) shown per file.
// Files exceeding this are replaced with a stat line; the handler declares lossy.
const diffFileTruncateLines = 300

// handleDiff compresses git diff (unified patch) output.
// Lossless when every file fits within diffFileTruncateLines.
// Lossy when large files are summarised; raw output saved to pager.
func handleDiff(raw string) (*commands.Result, error) {
	if strings.TrimSpace(raw) == "" {
		return &commands.Result{Stdout: "", Lossless: true}, nil
	}

	files := parseDiffFiles(raw)
	if len(files) == 0 {
		// Unrecognised format — return raw
		return nil, fmt.Errorf("git diff: could not parse output")
	}

	lossless := true
	var sb strings.Builder

	// Summary header
	totalAdded, totalRemoved := 0, 0
	for _, f := range files {
		totalAdded += f.added
		totalRemoved += f.removed
	}
	n := len(files)
	fmt.Fprintf(&sb, "diff: %d %s +%d -%d\n", n, pluralFile(n), totalAdded, totalRemoved)

	for _, f := range files {
		// Build file label
		label := f.path
		if f.oldPath != "" && f.oldPath != f.path {
			label = f.oldPath + " → " + f.path
		}

		flags := ""
		if f.isNew {
			flags = " (new)"
		} else if f.isDeleted {
			flags = " (deleted)"
		}

		if f.isBinary {
			fmt.Fprintf(&sb, "--- %s [binary]%s\n", label, flags)
			continue
		}

		contentLines := strings.Count(f.content, "\n")
		if contentLines > diffFileTruncateLines {
			lossless = false
			fmt.Fprintf(&sb, "--- %s +%d -%d%s [%d lines — truncated, see raw]\n",
				label, f.added, f.removed, flags, contentLines)
			continue
		}

		fmt.Fprintf(&sb, "--- %s +%d -%d%s\n", label, f.added, f.removed, flags)
		sb.WriteString(f.content)
	}

	return &commands.Result{
		Stdout:   sb.String(),
		Lossless: lossless,
	}, nil
}

func pluralFile(n int) string {
	if n == 1 {
		return "file"
	}
	return "files"
}

// diffFile holds parsed info for one file in a unified diff.
type diffFile struct {
	path      string // destination path
	oldPath   string // source path (non-empty for renames)
	isNew     bool
	isDeleted bool
	isBinary  bool
	added     int
	removed   int
	content   string // hunk lines (@@, +, -, space, \)
}

// parseDiffFiles splits a unified diff into per-file records.
func parseDiffFiles(raw string) []diffFile {
	lines := strings.Split(raw, "\n")
	var files []diffFile
	i := 0
	for i < len(lines) {
		if !strings.HasPrefix(lines[i], "diff --git ") {
			i++
			continue
		}
		var f diffFile
		f, i = parseSingleDiffFile(lines, i)
		files = append(files, f)
	}
	return files
}

// parseSingleDiffFile parses from the "diff --git" line until the next one.
func parseSingleDiffFile(lines []string, start int) (diffFile, int) {
	var f diffFile
	i := start

	// Extract initial path from "diff --git a/<path> b/<path>"
	header := strings.TrimPrefix(lines[i], "diff --git ")
	i++
	if idx := strings.LastIndex(header, " b/"); idx >= 0 {
		f.path = strings.TrimPrefix(header[:idx], "a/")
	}

	var content strings.Builder

	for i < len(lines) {
		line := lines[i]

		if strings.HasPrefix(line, "diff --git ") {
			break
		}

		switch {
		case strings.HasPrefix(line, "index "):
			// Skip: object hashes, not useful to LLM
		case strings.HasPrefix(line, "new file mode"):
			f.isNew = true
		case strings.HasPrefix(line, "deleted file mode"):
			f.isDeleted = true
		case strings.HasPrefix(line, "old mode"), strings.HasPrefix(line, "new mode"):
			// Skip mode change metadata
		case strings.HasPrefix(line, "similarity index"), strings.HasPrefix(line, "dissimilarity index"):
			// Skip
		case strings.HasPrefix(line, "rename from "):
			f.oldPath = strings.TrimPrefix(line, "rename from ")
		case strings.HasPrefix(line, "rename to "):
			f.path = strings.TrimPrefix(line, "rename to ")
		case strings.HasPrefix(line, "copy from "), strings.HasPrefix(line, "copy to "):
			// Skip copy metadata
		case strings.HasPrefix(line, "Binary files "):
			f.isBinary = true
		case strings.HasPrefix(line, "--- "):
			// Redundant with our header — skip, but update path from /dev/null detection
			src := strings.TrimPrefix(line, "--- ")
			src = strings.TrimPrefix(src, "a/")
			if src == "/dev/null" {
				f.isNew = true
			}
		case strings.HasPrefix(line, "+++ "):
			// Get the canonical destination path
			dst := strings.TrimPrefix(line, "+++ ")
			dst = strings.TrimPrefix(dst, "b/")
			if dst == "/dev/null" {
				f.isDeleted = true
			} else if f.oldPath == "" {
				// Not a rename — use this as the canonical path (handles quoted names)
				f.path = dst
			}
		case strings.HasPrefix(line, "@@ "):
			content.WriteString(line)
			content.WriteByte('\n')
		case line == `\ No newline at end of file`:
			content.WriteString(line)
			content.WriteByte('\n')
		case len(line) > 0 && line[0] == '+':
			f.added++
			content.WriteString(line)
			content.WriteByte('\n')
		case len(line) > 0 && line[0] == '-':
			f.removed++
			content.WriteString(line)
			content.WriteByte('\n')
		case len(line) > 0 && line[0] == ' ':
			// Context line
			content.WriteString(line)
			content.WriteByte('\n')
		}

		i++
	}

	f.content = content.String()
	return f, i
}
