package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"tko/internal/commands"
	_ "tko/internal/commands/du"      // register du handler
	_ "tko/internal/commands/find"    // register find and fd handlers
	_ "tko/internal/commands/git"     // register git handlers
	_ "tko/internal/commands/gobuild" // register go build handler
	_ "tko/internal/commands/ls"      // register ls handler
	_ "tko/internal/commands/wc"      // register wc handler
	"tko/internal/compress"
	"tko/internal/hook"
	"tko/internal/runner"
	"tko/internal/tracking"
	"tko/internal/upgrade"
	"tko/internal/version"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	if args[0] == "-h" || args[0] == "--help" {
		printUsage()
		return
	}

	if args[0] == "--version" || args[0] == "version" {
		fmt.Println(version.Version)
		return
	}

	// Scan for -- separator, collecting root-level flags before it.
	sample := false
	ddIdx := -1
	for i, a := range args {
		switch a {
		case "--sample":
			sample = true
		case "--":
			ddIdx = i
		default:
			// First non-flag arg: stop scanning (it's a subcommand name).
			if ddIdx < 0 {
				goto dispatch
			}
		}
		if ddIdx >= 0 {
			break
		}
	}

dispatch:
	if ddIdx >= 0 {
		// Wrapped command: tko [--sample] -- <cmd> [args...]
		wrapped := args[ddIdx+1:]
		if len(wrapped) == 0 {
			fmt.Fprintln(os.Stderr, "tko: missing command after --")
			os.Exit(1)
		}
		runWrapped(wrapped, sample)
		return
	}

	// Built-in subcommand dispatch. Strip --sample if it snuck in.
	var sub []string
	for _, a := range args {
		if a != "--sample" {
			sub = append(sub, a)
		}
	}

	switch sub[0] {
	case "rewrite":
		runRewrite(sub[1:])
	case "hook":
		runHook(sub[1:])
	case "stats":
		if err := tracking.PrintStats(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "misses":
		prefix := ""
		if len(sub) > 1 {
			prefix = strings.Join(sub[1:], " ")
		}
		if err := tracking.PrintMisses(prefix); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "reset":
		if err := tracking.Reset(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "upgrade":
		if err := upgrade.Run(version.Version, version.Repo); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "tko: unknown command %q\n\nDid you mean:  tko -- %s\n", sub[0], strings.Join(sub, " "))
		os.Exit(1)
	}
}

func runWrapped(args []string, sample bool) {
	cmd := args[0]
	cmdArgs := args[1:]

	handler, ok := commands.Match(cmd, cmdArgs)
	if !ok {
		prefix := commands.CommandPrefix(cmd, cmdArgs)
		fullCmd := cmd + " " + strings.Join(cmdArgs, " ")
		exitCode, outputBytes := runner.PassthroughCounted(cmd, cmdArgs)
		tracking.RecordMiss(prefix, strings.TrimSpace(fullCmd), outputBytes, exitCode)
		os.Exit(exitCode)
		return
	}

	result, err := runner.Run(cmd, cmdArgs)
	if err != nil {
		runner.LogError(cmd, cmdArgs, fmt.Errorf("run: %w", err))
		os.Exit(runner.Passthrough(cmd, cmdArgs))
		return
	}

	compressed, herr := handler.Handle(cmdArgs, result.Stdout, result.Stderr)
	if herr != nil {
		runner.LogError(cmd, cmdArgs, fmt.Errorf("handler: %w", herr))
		fmt.Print(result.Stdout)
		fmt.Fprint(os.Stderr, result.Stderr)
		os.Exit(result.ExitCode)
		return
	}

	output := compressed.Stdout

	if compressed.Stderr != "" {
		fmt.Fprint(os.Stderr, compressed.Stderr)
	} else {
		fmt.Fprint(os.Stderr, result.Stderr)
	}

	tracking.Record(
		handler.Id(),
		compress.TokenCount(result.Stdout),
		compress.TokenCount(output),
		compressed.Lossless,
	)

	if sample {
		printSample(cmd, cmdArgs, result.Stdout, output, compressed.Lossless)
	}

	fmt.Print(output)
	if output != "" && !strings.HasSuffix(output, "\n") {
		fmt.Println()
	}

	os.Exit(result.ExitCode)
}

// runRewrite handles `tko rewrite <cmd>`.
// Used by the Claude Code hook script. Exits 0 with rewritten command on
// stdout if a rewrite applies, exits 1 otherwise (hook passes through).
func runRewrite(args []string) {
	if len(args) == 0 {
		os.Exit(1)
	}
	cmd := strings.Join(args, " ")
	rewritten, ok := commands.Rewrite(cmd)
	if !ok {
		os.Exit(1)
	}
	fmt.Println(rewritten)
}

// runHook handles `tko hook <claude|install|uninstall|status>`.
func runHook(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: tko hook <install|uninstall|status|claude>")
		os.Exit(1)
	}
	var err error
	switch args[0] {
	case "claude":
		runHookExec()
	case "install":
		err = hook.Install()
	case "uninstall":
		err = hook.Uninstall()
	case "status":
		hook.Status()
	default:
		fmt.Fprintf(os.Stderr, "unknown hook command: %s\n", args[0])
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runHookExec is the Claude Code PreToolUse hook handler (tko hook claude).
// It reads the hook JSON payload from stdin, rewrites the command if a handler
// exists, and writes the updated payload to stdout. Any error silently exits 0
// so Claude Code always falls through to normal execution.
func runHookExec() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(data) == 0 {
		os.Exit(0)
	}

	var input map[string]json.RawMessage
	if err := json.Unmarshal(data, &input); err != nil {
		os.Exit(0)
	}

	var toolInput map[string]json.RawMessage
	if err := json.Unmarshal(input["tool_input"], &toolInput); err != nil {
		os.Exit(0)
	}

	var cmd string
	if err := json.Unmarshal(toolInput["command"], &cmd); err != nil || cmd == "" {
		os.Exit(0)
	}

	rewritten, ok := commands.Rewrite(cmd)
	if !ok {
		if name, args, full, simple := commands.ParseSimple(cmd); simple {
			prefix := commands.CommandPrefix(name, args)
			tracking.RecordMiss(prefix, full, 0, 0)
		}
		os.Exit(0)
	}

	rewrittenRaw, _ := json.Marshal(rewritten)
	updatedInput := make(map[string]json.RawMessage, len(toolInput))
	for k, v := range toolInput {
		updatedInput[k] = v
	}
	updatedInput["command"] = rewrittenRaw

	updatedInputBytes, _ := json.Marshal(updatedInput)
	fmt.Printf(`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","permissionDecisionReason":"tko auto-rewrite","updatedInput":%s}}`, updatedInputBytes)
	fmt.Println()
}

func printSample(cmd string, args []string, input, output string, lossless bool) {
	inTok := compress.TokenCount(input)
	outTok := compress.TokenCount(output)
	inLines := compress.LineCount(input)
	outLines := compress.LineCount(output)
	saved := 0
	if inTok > 0 {
		saved = 100 - (outTok*100)/inTok
	}
	fmt.Fprintf(os.Stderr, "[tko sample] %s %s\n", cmd, strings.Join(args, " "))
	fmt.Fprintf(os.Stderr, "  input:  %d lines / %d chars / ~%d tokens\n", inLines, len(input), inTok)
	fmt.Fprintf(os.Stderr, "  output: %d lines / %d chars / ~%d tokens\n", outLines, len(output), outTok)
	fmt.Fprintf(os.Stderr, "  saved:  %d%% tokens\n", saved)
	fmt.Fprintf(os.Stderr, "  lossy:  %v\n", !lossless)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `tko %s — knock out useless tokens

usage:
  tko [--sample] -- <command> [args...]   compress command output
  tko stats                               show token savings summary
  tko misses [<prefix>]                   show missed commands by potential
  tko hook <install|uninstall|status>     manage Claude Code hook
  tko upgrade                             upgrade tko to the latest release
  tko rewrite <cmd>                       rewrite cmd to use tko (hook use)

flags:
  --sample         show before/after compression stats on stderr
  --version        print version and exit
  -h, --help       show this help
`, version.Version)
}
