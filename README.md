<div align="center">
  <img src="https://raw.githubusercontent.com/roelentless/tko/main/assets/tko.png" width="600"/>
</div>

# tko

**An experiment in reducing agent token waste.**

Most CLI tools are built for humans: verbose, instructional output full of formatting that burns agent context. Until tools ship native `AGENT=true` output modes, `tko` is an attempt to fill that gap — intercepting popular commands and rewriting their output into compact, lossless forms.

```
git status  →  471 tokens     tko -- git status  →  201 tokens   (-58%)
```

**Strategy-based, not magic rewrite.** Each handler is a purpose-built compressor for a specific command and argument pattern. You can read what it does, predict its output, and trust it. No LLM calls, no heuristics, no surprises.

**Lossless only.** tko never drops information. If a command can't be compressed losslessly, it is passed through raw. The agent always gets the full picture.

---

**What we optimise — and what we don't.** The target is bloat: commands that dump large, context-heavy output an agent has to wade through on every call. `git status`, `git log --oneline`, `ls`. These are the token sinks worth attacking.

Targeted commands where an agent is actively searching — `git diff path/to/file`, `grep`, `rg`, `jq` — are left alone. The agent asked for something specific; the output is already intentional. Intercepting it risks breaking the workflow it was built around.

---

## How it works

A Claude Code `PreToolUse` hook intercepts every shell command. If `tko` has a handler for it, the command is silently rewritten: `git status` → `tko -- git status`. The agent sees compressed output with identical semantics.

No prompt changes. No agent awareness. Just fewer tokens.

---

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/roelentless/tko/main/install.sh | sh
```

Installs to `~/.local/bin/tko`, adds it to your PATH, and registers the Claude Code hook. Restart Claude Code. Done.

**Build from source** (requires Go 1.22+):

```sh
git clone https://github.com/roelentless/tko
cd tko
make install          # builds + copies to ~/.local/bin/tko
tko hook install      # patches ~/.claude/settings.json
```

---

## What gets compressed

All handlers are lossless. Commands that can't be compressed losslessly are passed through raw.

| Command | What's stripped | Always lossless |
|---------|-----------------|-----------------|
| `git status` | Instructional text; files brace-grouped by dir/extension | yes |
| `git log --oneline` | Trailing whitespace only | yes |
| `git log -n N` (N ≤ 20) | Author/date boilerplate; shows hash + date + subject | yes |
| `git show` | Commit header boilerplate; diff headers stripped | yes (falls back to raw if diff is large) |
| `ls` / `ls -la` | Collapses to single-line count; strips permission/owner/date columns | yes |

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

**git log -n 5**
```
log: 5 commits
a1b2c3d 2026-03-15 feat: add git diff handler
b2c3d4e 2026-03-14 fix: status parser edge case
...
```

**git show**
```
commit a1b2c3d
author: Jane <jane@example.com>  date: 2026-03-15
    feat: add git diff handler

diff: 2 files +45 -12
--- pkg/diff.go +45 -12
@@ ...
```

---

## Commands

```sh
tko [--sample] -- <command> [args]   # run and compress
tko stats                            # token savings summary
tko misses                           # top unhandled commands by potential savings
tko misses 'git log'                 # zoom into a specific prefix
tko rewrite '<cmd>'                  # test hook rewriting
tko hook install                     # set up Claude Code hook
tko hook uninstall                   # remove hook
tko hook status                      # check hook state
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
npm test         1        8.1k        8.1k
```

`potential = count × avg_tokens` — the highest rows are the best next handlers to write.

---

## Adding a handler

Handlers must be lossless. If you can't guarantee that, don't add the handler.

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

**3. Test** in `<name>/<subcmd>_test.go` — shell out to the real binary, create a temp environment, assert output correctness. See `internal/commands/git/status_test.go` for the pattern.

---

## Design principles

- **Lossless only** — if a handler can't preserve all information, it passes through raw
- **Never fail the agent** — if a handler errors, `tko` falls back to raw passthrough and logs to `~/.local/share/tko/errors.log`
- **No compound commands** — `&&`, `||`, `;`, `|` are never rewritten
- **Transparent** — exit codes, stdin, and stderr are forwarded exactly

---

## State

| Path | Contents |
|------|----------|
| `~/.local/share/tko/tracking.db` | SQLite: compressions + misses |
| `~/.local/share/tko/errors.log` | Handler failures |
| `~/.claude/settings.json` | Patched with PreToolUse hook entry |
