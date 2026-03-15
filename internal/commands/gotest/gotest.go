// Package gotest registers a handler for `go test` output.
package gotest

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"tko/internal/commands"
)

func init() {
	commands.Register(&goTestHandler{})
}

type goTestHandler struct{}

func (h *goTestHandler) Route() string { return "go" }

func (h *goTestHandler) Supports(args []string) bool {
	return supportsGoTest(args)
}

func (h *goTestHandler) Handle(args []string, rawStdout, rawStderr string) (*commands.Result, error) {
	return handleGoTest(rawStdout, rawStderr)
}

// supportsGoTest uses an allowlist: only flags that don't change the output
// format are accepted. go test -bench, -cpu, -memprofile etc. are not supported.
func supportsGoTest(args []string) bool {
	if len(args) == 0 || args[0] != "test" {
		return false
	}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			// Package path (./..., ./pkg, etc.): fine.
			continue
		}
		switch {
		case arg == "-v":
		case arg == "-run":
			if i+1 >= len(args) {
				return false
			}
			i++ // skip the pattern value
		case strings.HasPrefix(arg, "-run="):
		case arg == "-count":
			if i+1 >= len(args) {
				return false
			}
			i++
		case strings.HasPrefix(arg, "-count="):
		default:
			return false
		}
	}
	return true
}

var (
	reOkPkg   = regexp.MustCompile(`^ok  \t`)
	reFailPkg = regexp.MustCompile(`^FAIL\t`)
	rePassTest = regexp.MustCompile(`^--- PASS:`)
	reRunTest  = regexp.MustCompile(`^=== RUN\s`)
	rePkgTime  = regexp.MustCompile(`(\d+\.\d+)s$`)
)

func handleGoTest(rawStdout, rawStderr string) (*commands.Result, error) {
	combined := rawStdout
	if rawStderr != "" {
		if combined != "" && !strings.HasSuffix(combined, "\n") {
			combined += "\n"
		}
		combined += rawStderr
	}

	lines := strings.Split(strings.TrimRight(combined, "\n"), "\n")

	var passed, failed int
	var totalMs float64
	var failureLines []string

	for _, line := range lines {
		switch {
		case reOkPkg.MatchString(line):
			passed++
			if m := rePkgTime.FindStringSubmatch(line); m != nil {
				d, _ := strconv.ParseFloat(m[1], 64)
				totalMs += d
			}
		case reFailPkg.MatchString(line):
			failed++
			if m := rePkgTime.FindStringSubmatch(line); m != nil {
				d, _ := strconv.ParseFloat(m[1], 64)
				totalMs += d
			}
			failureLines = append(failureLines, line)
		case rePassTest.MatchString(line), reRunTest.MatchString(line):
			// Discard: passing test noise.
		default:
			failureLines = append(failureLines, line)
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "go test: %d passed  %d failed  (%.1fs)\n", passed, failed, totalMs)

	// Append failure details, stripping leading/trailing blank lines.
	failOutput := strings.TrimSpace(strings.Join(failureLines, "\n"))
	if failOutput != "" {
		sb.WriteByte('\n')
		sb.WriteString(failOutput)
		sb.WriteByte('\n')
	}

	return &commands.Result{
		Stdout:   sb.String(),
		Lossless: false, // passing test output is discarded
	}, nil
}
