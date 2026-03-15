# tko

**Knock out useless tokens.**

Most CLI tools are built for humans: verbose, instructional output full of formatting that wastes agent context. Until tools ship native `AGENT=true` output modes, `tko` fills the gap — intercepting popular commands and rewriting their output into compact, meaningful forms that give agents exactly what they need to reason and act.

```
git status  →  471 tokens     tko -- git status  →  201 tokens   (-58%)
git diff    →  38k tokens     tko -- git diff    →  17k tokens   (-56%)
```

**Strategy-based, not magic rewrite.** Each handler is a purpose-built compressor for a specific command and argument pattern. You can read what it does, predict its output, and trust it. No LLM calls, no heuristics, no surprises.

Two output guarantees:

- **Lossless when possible** — the compressed output contains all the information of the original, just denser.
- **Lossless via summary when not** — when full fidelity isn't possible (e.g. a 3000-line lock file diff), the handler emits a structured summary and saves the raw output to a greppable temp file. The agent always receives a pointer and can follow it.

---

**What we optimise — and what we don't.** The target is bloat: commands that dump large, context-heavy output an agent has to wade through on every call. `git status`, `git diff`, build output, test results. These are the token sinks worth attacking.

Piped commands where an agent is actively searching — `grep`, `rg`, `jq`, `awk` pipelines — are a different story. The agent chose that pipeline deliberately; the output is already specific. Compressing or intercepting it risks breaking the workflow it was built around. Those we leave alone.

This is a deliberate trade-off between token efficiency and workflow efficiency. The goal is to keep agents fast and unblocked, not to wall them into compressed outputs they can't navigate out of. Getting this balance right is still a work in progress.

🌱 **Every token saved is compute not spent.** A 58% reduction on `git status`, multiplied across thousands of tool calls per session, adds up. Fewer tokens means less inference work, less energy, and faster responses. tko is a small tool with a measurable environmental return.

---

**Early implementation — contributions welcome.** The handler format and argument parsing are still evolving. Only a handful of commands are covered. If a command dominates your `tko misses` output, a new handler is a small, self-contained file. See [Adding a handler](#adding-a-handler) and [RFC-002](rfc/RFC-002-handlers.md) for what's planned and how to contribute.

---

## How it works

A Claude Code `PreToolUse` hook intercepts every shell command. If `tko` has a handler for it, the command is silently rewritten: `git status` → `tko -- git status`. The agent sees compressed output with identical semantics.

No prompt changes. No agent awareness. Just fewer tokens.

---

## Install

```sh
git clone https://github.com/you/tko
cd tko
make install          # builds + copies to ~/.local/bin/tko
tko hook install      # patches ~/.claude/settings.json
```

Restart Claude Code. Done.

**Requirements:** Go 1.22+

---

## What gets compressed

| Command | Strategy | Lossless |
|---------|----------|----------|
| `git status` | Strips instructional text, groups files with brace expansion | lossless |
| `git diff [args]` | Strips redundant headers, truncates lock/generated files | lossless / lossy |

**Lossless** — all information preserved, no temp file needed.
**Lossy** — large files (e.g. `package-lock.json`) are summarised; full output saved to `/tmp/tko-*.txt` with a pointer the agent can follow.

---

## Compressed output format

**git status**
```
branch:main=origin/main ↑2
staged(3):
  modified: pkg/{foo,bar}.go
  new: pkg/baz.go
unstaged(1):
  modified: main.go
untracked(2): tmp/{debug.log,notes.txt}
```

**git diff**
```
diff: 4 files +87 -23
--- pkg/server.go +45 -12
@@ -102,7 +102,9 @@ func (s *Server) Start() {
 existing context
-old line
+new line
--- go.sum +42 -11 [387 lines — truncated, see raw]
```

---

## Commands

```sh
tko [--sample] <command> [args]   # run and compress
tko stats                         # token savings summary
tko misses                        # top unhandled commands by potential savings
tko misses 'git log'              # zoom into a specific prefix
tko rewrite '<cmd>'               # test hook rewriting
tko hook install                  # set up Claude Code hook
tko hook uninstall                # remove hook
tko hook status                   # check hook state
```

`--sample` prints compression stats to stderr without affecting stdout — useful for benchmarking a handler against a real repo.

---

## Discovering what to implement next

```sh
tko misses
```
```
prefix        seen  avg tokens   potential
------        ----  ----------   ---------
git diff         2       29.0k       58.0k
git log          4        4.2k       16.8k
npm test         1        8.1k        8.1k
```

`potential = count × avg_tokens` — the highest rows are the best next handlers to write.

---

## Adding a handler

Three steps:

**1. Create** `internal/commands/<name>/<subcmd>.go`:
```go
package mycommand

import "tko/internal/commands"

func init() { commands.Register(&myHandler{}) }

type myHandler struct{}

func (h *myHandler) Route() string    { return "mycmd" }

func (h *myHandler) Supports(args []string) bool {
    // Strip global flags, match exact subcommand + arg pattern you own.
    return len(args) > 0 && args[0] == "subcommand"
}

func (h *myHandler) Handle(args []string, rawStdout, rawStderr string) (*commands.Result, error) {
    return &commands.Result{
        Stdout:   compress(rawStdout),
        Lossless: true,
    }, nil
}
```

**2. Import** in `cmd/tko/main.go`:
```go
_ "tko/internal/commands/mycommand"
```

**3. Test** in `<name>/<subcmd>_test.go` — shell out to the real binary, create a temp environment, assert both output correctness and that `Lossless` is declared accurately. See `internal/commands/git/diff_test.go` for the pattern.

---

## Design principles

- **Never fail the agent** — if a handler errors, `tko` falls back to raw passthrough and logs to `~/.local/share/tko/errors.log`
- **Lossless by default** — lossy handlers must declare it; the agent always gets a pointer to the full raw output
- **No compound commands** — `&&`, `||`, `;`, `|` are never rewritten (piped output format assumptions would break)
- **Transparent** — exit codes, stdin, and stderr are forwarded exactly

---

## State

| Path | Contents |
|------|----------|
| `~/.local/share/tko/tracking.db` | SQLite: compressions + misses |
| `~/.local/share/tko/errors.log` | Handler failures |
| `~/.claude/settings.json` | Patched with PreToolUse hook entry |
| `/tmp/tko-<ts>-<cmd>.txt` | Raw output for lossy compressions |
