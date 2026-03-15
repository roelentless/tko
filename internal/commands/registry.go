package commands

import (
	"path/filepath"
	"strings"
)

// Result is the output of a Handler.Handle call.
type Result struct {
	Stdout   string
	Stderr   string // if empty, raw stderr is forwarded verbatim
	Lossless bool
}

// Handler compresses the output of a specific command invocation.
//
// Each handler is responsible for its own routing:
//
//   - Route()    — the binary name used as the O(1) registry key ("git", "npm")
//   - Supports() — fine-grained match: does this handler own this exact invocation?
//   - Handle()   — compress rawStdout (and optionally rawStderr)
//
// Multiple handlers may share the same Route(). They are checked in registration
// order; the first one whose Supports() returns true wins.
//
// CONTRACT: handlers must never modify the input command or args.
// The command is always executed exactly as the agent requested it.
// Handlers only transform the output (stdout/stderr).
//
// Register in an init() function: commands.Register(&MyHandler{})
type Handler interface {
	// Route returns the binary name used as the registry lookup key.
	// e.g. "git", "npm", "cargo"
	Route() string

	// Supports returns true if this handler owns this specific invocation.
	// All matching logic lives here — args format, flags, subcommand, etc.
	Supports(args []string) bool

	// Handle compresses rawStdout and optionally rawStderr.
	// Return an empty Stderr to forward rawStderr verbatim.
	Handle(args []string, rawStdout, rawStderr string) (*Result, error)
}

// registry maps binary name → ordered list of handlers.
// Multiple handlers per key are checked in registration order; first match wins.
var registry = map[string][]Handler{}

// Register adds h to the global registry under h.Route().
// Call from an init() function.
func Register(h Handler) {
	key := h.Route()
	registry[key] = append(registry[key], h)
}

// Match finds the first registered handler for cmd that accepts args.
// filepath.Base is applied to cmd so both "git" and "/usr/bin/git" resolve correctly.
func Match(cmd string, args []string) (Handler, bool) {
	for _, h := range registry[filepath.Base(cmd)] {
		if h.Supports(args) {
			return h, true
		}
	}
	return nil, false
}

// CommandPrefix returns the canonical prefix used for miss tracking:
// the command name plus the first non-flag argument (the subcommand).
// Examples: "git log", "npm install", "cargo build", "ls".
func CommandPrefix(cmd string, args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return cmd + " " + a
		}
	}
	return cmd
}
