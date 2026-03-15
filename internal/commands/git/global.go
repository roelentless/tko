package git

import "strings"

// gitSubcommand strips known git global flags from args and returns the
// subcommand and its arguments. Used by handlers for routing only —
// the original args are always forwarded to the runner unchanged.
//
// Returns ok=false for unknown global flags so the handler falls through
// to passthrough rather than risking a parse of an unexpected format.
//
// Handled global flags:
//
//	-C <path>                    run as if in <path>
//	-c <key>=<value>             set config value
//	--git-dir=<path>             set repo path
//	--work-tree=<path>           set working tree
//	--namespace=<name>           set namespace
//	--no-pager / -P              suppress pager (doesn't affect output format)
func gitSubcommand(args []string) (sub string, rest []string, ok bool) {
	i := 0
	for i < len(args) {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			return arg, args[i+1:], true
		}
		switch {
		case arg == "-C", arg == "-c",
			arg == "--git-dir", arg == "--work-tree", arg == "--namespace":
			i += 2 // flag + value
		case strings.HasPrefix(arg, "--git-dir="),
			strings.HasPrefix(arg, "--work-tree="),
			strings.HasPrefix(arg, "--namespace="):
			i++ // combined flag=value
		case arg == "--no-pager", arg == "-P":
			i++
		default:
			// Unknown global flag — don't handle.
			return "", nil, false
		}
	}
	return "", nil, false
}
