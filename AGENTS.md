# tko — Agent Guide

**tko** ("knock out useless tokens") is a CLI output compression proxy for LLM coding agents.
It wraps shell commands, compresses their output token-efficiently, and passes unknown commands
through transparently. The LLM never knows tko exists.

---

## Build & Install

```sh
make build          # produces ./dist/tko binary
make install        # installs to ~/.local/bin/tko
make test           # runs all tests
tko hook install    # patches ~/.claude/settings.json (run after make install)
```

After `tko hook install`, restart Claude Code. From that point every `git status` the agent
runs is silently rewritten to `tko -- git status`.

---

## Project Layout

```
cmd/tko/main.go              entry point — dispatch, compression pipeline, sample flag
internal/
  runner/                    fork+exec child processes
    runner.go                Run() captures stdout+stderr; PassthroughCounted() for misses;
                             Passthrough() for pure transparent forwarding
  commands/
    registry.go              Handler interface + Register/Lookup + CommandPrefix()
    rewrite.go               Rewrite(cmd) — wraps with "tko " if handler registered
    git/
      status.go              git status handler (lossless, ~58% token reduction)
  hook/
    hook.go                  hook install/uninstall/status/claude; patches settings.json
  tracking/
    tracking.go              SQLite at ~/.local/share/tko/tracking.db
                             Record() — compression events; RecordMiss() — passthroughs
                             PrintStats() / PrintMisses(prefix)
  compress/compress.go       TokenCount() — chars/4 approximation
  pager/pager.go             Save raw output to /tmp/tko-<ts>-<cmd>.txt (lossy path)
rfc/RFC-001-tko-architecture.md  full design document
```

---

## Adding a New Handler

See [`internal/commands/AGENTS.md`](internal/commands/AGENTS.md) for the full handler
writing guide, output format principles, and PR checklist.

Quick summary — three steps:

**1. Create** `internal/commands/<binary>/<subcommand>.go` with `Route()`, `Supports()`, `Handle()`, and `init()`.

**2. Import** in `cmd/tko/main.go`:

```go
_ "tko/internal/commands/mycommand"
```

**3. Test** every opt-in pattern in `Supports()` plus rejection cases. Shell out to the real binary; no mocking. See `internal/commands/git/routing_test.go` and `status_test.go` for the pattern.

---

## Key Design Rules

**Lossless only** — every handler must declare `Lossless: true`. If a handler cannot guarantee that all information is preserved, it must return an error instead, which causes tko to fall back to raw passthrough. There is no pager, no temp file, no truncation. The agent always gets either compressed-and-complete or the original raw output.

**Never modify the input** — handlers receive the original command and args unchanged. The runner always executes exactly what the agent requested. Handlers only transform stdout/stderr. This is a hard contract: breaking it means the agent runs something different from what it asked for.

**Never fail silently** — if a handler errors, tko falls back to raw output and logs to
`~/.local/share/tko/errors.log`. The agent always gets a valid response.

**No compound commands** — `Rewrite()` skips any command containing `&&`, `||`, `;`, `|`,
backticks, or `$(`. Piped commands stay raw: the agent might pipe compressed output into
grep expecting the original format, which would break.

**Stderr** — handlers receive `rawStderr` and can set `Result.Stderr` to compress it.
If `Result.Stderr` is empty, rawStderr is forwarded verbatim. Most handlers leave it empty.

---

## Compression Formats

### git status

```
branch:main=origin/main
staged(3):
  modified: pkg/{foo,bar}.go
  new: pkg/baz.go
unstaged(1):
  modified: main.go
untracked(2): tmp/{debug.log,notes.txt}
```

- Branch line: `branch:<name>[=<upstream>][ ↑N↓N]`
- Sections: `staged`, `unstaged`, `untracked`
- Verbs: `modified`, `new`, `deleted`, `renamed`, `copied`
- Brace grouping: files sharing parent dir + extension → `dir/{a,b}.ext`

### git log (--oneline or -n N ≤ 20)

```
log: 5 commits
a1b2c3d 2026-03-15 feat: add git diff handler
b2c3d4e 2026-03-14 fix: status parser edge case
```

- One line per commit: `<7-char hash> <YYYY-MM-DD> <subject>`
- Plain `git log` (no limit) passes through raw — not handled

### git show

```
commit a1b2c3d
author: Jane <jane@example.com>  date: 2026-03-15
    feat: add git diff handler

diff: 2 files +45 -12
--- pkg/foo.go +12 -3
@@ -45,7 +45,9 @@ func Foo() {
 context
-old line
+new line
```

- Compact commit header: hash, author, date, subject
- Diff section reuses the unified diff parser
- If the diff exceeds the lossless threshold, falls back to raw passthrough

### ls / ls -la

```
dirs(2): cmd/ internal/
files(5): *.go(2) go.mod go.sum README.md
hidden(1): .gitignore
total: 8 items
```

- Plain `ls`: single line with all names and count
- `ls -la`: dirs / files (grouped by extension) / hidden, permissions stripped

---

## Discovering What to Implement Next

```sh
tko misses                # top missed commands by optimization potential (count × avg tokens)
tko misses 'git diff'     # zoom into specific invocations under that prefix
tko stats                 # compression savings + miss count
```

The miss tracker uses `PassthroughCounted()` — real-time stdout forwarding with a counting
writer, so token estimates are free with no buffering.

---

## Hook Mechanics

Claude Code `PreToolUse` hook calls `tko hook claude` on every Bash tool use:
1. Reads JSON from stdin, extracts `.tool_input.command`
2. Calls `commands.Rewrite(cmd)` — returns rewritten command or exits 0 to pass through
3. If rewritten: returns `updatedInput` JSON with `permissionDecisionReason: "tko auto-rewrite"`

`tko rewrite` only rewrites if the first word is a registered handler name. Skips compound
commands. Never double-wraps.

---

## Changelog

`CHANGELOG.md` at project root.

**Granularity: one entry per feature or meaningful improvement.** A new handler, a new subcommand, a bugfix, a behaviour change — those get entries. Internal refactors, renames, or moving code do not unless they change observable behaviour.

**Scope: user-visible impact only.** Describe what changed in the agent's or developer's experience, not how it was built. No internal jargon. Combine multiple internal changes toward one outcome into one line.

**Format:** `- <type>: <description>` under a `## YYYY-MM-DD` header. Types: `feature`, `improvement`, `bugfix`, `security`.

---

## State

| Path | Contents |
|------|----------|
| `~/.local/share/tko/tracking.db` | SQLite: compressions + misses tables |
| `~/.local/share/tko/errors.log` | Handler failures with timestamp + command |
| `~/.claude/settings.json` | Patched with PreToolUse Bash hook entry |
