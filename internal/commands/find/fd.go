package find

import (
	"strings"

	"tko/internal/commands"
)

func init() {
	commands.Register(&fdHandler{})
}

type fdHandler struct{}

func (h *fdHandler) Id() string    { return "fd" }
func (h *fdHandler) Route() string { return "fd" }

// fdOutputModifiers are flags that cause fd to produce non-path output.
var fdOutputModifiers = map[string]bool{
	"--exec":         true,
	"-x":             true,
	"--exec-batch":   true,
	"-X":             true,
	"--list-details": true,
	"-l":             true,
	"--print0":       true,
	"-0":             true,
}

func (h *fdHandler) Supports(args []string) bool {
	for _, arg := range args {
		if fdOutputModifiers[arg] {
			return false
		}
		if strings.HasPrefix(arg, "--format=") || strings.HasPrefix(arg, "--format") && len(arg) == len("--format") {
			return false
		}
	}
	return true
}

func (h *fdHandler) Handle(args []string, rawStdout, _ string) (*commands.Result, error) {
	return handlePaths("fd", fdTypeLabel(args), rawStdout)
}

// fdTypeLabel returns "files", "dirs", or "items" based on -t / --type flags.
func fdTypeLabel(args []string) string {
	for i, a := range args {
		if (a == "-t" || a == "--type") && i+1 < len(args) {
			switch args[i+1] {
			case "f", "file":
				return "files"
			case "d", "dir", "directory":
				return "dirs"
			}
		}
		for _, pfx := range []string{"--type=", "-t="} {
			if strings.HasPrefix(a, pfx) {
				switch a[len(pfx):] {
				case "f", "file":
					return "files"
				case "d", "dir", "directory":
					return "dirs"
				}
			}
		}
	}
	return "items"
}
