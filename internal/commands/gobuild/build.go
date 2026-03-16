// Package gobuild registers a handler for go build.
package gobuild

import (
	"strings"

	"tko/internal/commands"
)

func init() {
	commands.Register(&goBuildHandler{})
}

type goBuildHandler struct{}

func (h *goBuildHandler) Id() string    { return "go-build" }
func (h *goBuildHandler) Route() string { return "go" }

// Supports accepts `go build` with package patterns and safe flags.
// Flags that change output format (-v, -n, -x, -work) are rejected.
func (h *goBuildHandler) Supports(args []string) bool {
	if len(args) == 0 || args[0] != "build" {
		return false
	}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		if !strings.HasPrefix(a, "-") {
			// Package pattern — accepted.
			continue
		}
		// Flags that change output format → reject.
		switch a {
		case "-v", "-n", "-x", "-work":
			return false
		}
		// Flags that consume the next argument.
		switch a {
		case "-o", "-tags", "-mod", "-ldflags", "-gcflags", "-asmflags", "-buildmode", "-compiler", "-gccgoflags", "-installsuffix", "-pkgdir", "-toolexec":
			if i+1 >= len(rest) {
				return false
			}
			i++
		case "-race", "-a", "-trimpath", "-msan", "-asan":
			// Single-word flags — accepted.
		default:
			// Unknown flag → reject (allowlist approach).
			return false
		}
	}
	return true
}

func (h *goBuildHandler) Handle(_ []string, rawStdout, rawStderr string) (*commands.Result, error) {
	return &commands.Result{
		Stdout:   rawStdout,
		Stderr:   compressBuildErrors(rawStderr),
		Lossless: true,
	}, nil
}

// compressBuildErrors strips the common module prefix from `# package/path`
// header lines. The error lines themselves (file:line:col: msg) are unchanged.
//
// Example input:
//
//	# github.com/example/myapp/internal/handler
//	internal/handler/foo.go:10:5: undefined: Bar
//
// Example output:
//
//	# ./internal/handler
//	internal/handler/foo.go:10:5: undefined: Bar
func compressBuildErrors(raw string) string {
	if raw == "" {
		return ""
	}

	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")

	// Collect all package paths from "# <path>" header lines.
	var pkgPaths []string
	for _, l := range lines {
		if strings.HasPrefix(l, "# ") {
			pkgPaths = append(pkgPaths, strings.TrimPrefix(l, "# "))
		}
	}

	if len(pkgPaths) == 0 {
		return raw
	}

	prefix := commonPathPrefix(pkgPaths)
	if prefix == "" {
		return raw
	}

	var sb strings.Builder
	for i, l := range lines {
		if strings.HasPrefix(l, "# ") {
			sb.WriteString("# ./")
			sb.WriteString(strings.TrimPrefix(strings.TrimPrefix(l, "# "), prefix))
		} else {
			sb.WriteString(l)
		}
		if i < len(lines)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// commonPathPrefix returns the longest common slash-delimited prefix shared by
// all paths, not including the final segment. Returns "" if no meaningful prefix
// exists (single-segment paths or no common prefix).
func commonPathPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	// Split each path into slash-separated segments.
	split := make([][]string, len(paths))
	for i, p := range paths {
		split[i] = strings.Split(p, "/")
	}

	// Find the common prefix segments.
	ref := split[0]
	common := 0
	for i := range ref {
		for _, segs := range split[1:] {
			if i >= len(segs) || segs[i] != ref[i] {
				goto done
			}
		}
		common = i + 1
	}
done:

	// A common prefix of the entire path (all segments shared) means all
	// packages are the same — not a useful prefix to strip.
	if common == 0 || common == len(ref) {
		return ""
	}

	return strings.Join(ref[:common], "/") + "/"
}
