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

**Lossless vs Lossy** — every handler declares `Lossless: true/false` explicitly.
- Lossless: all information is preserved in compressed form. No temp file needed.
- Lossy: some detail is omitted. The pager saves raw stdout to `/tmp/tko-<ts>-<cmd>.txt`
  and appends `# [RAW] Full output: /path` so the agent can grep the original.

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

### git diff

```
diff: 3 files +45 -12
--- pkg/foo.go +12 -3
@@ -45,7 +45,9 @@ func Foo() {
 context
-old line
+new line
--- bar/baz.go +33 -9 (new)
@@ -0,0 +1,33 @@
+...
--- some/big.lock +0 -0 (new) [2606 lines — truncated, see raw]
```

- Summary header: `diff: N files +A -D`
- Per-file header: `--- path +added -removed [flags]`
- Flags: `(new)`, `(deleted)`, `[binary]`, `[N lines — truncated, see raw]`
- Renames shown as: `--- old.go → new.go +A -D`
- Stripped: `diff --git`, `index`, `--- a/`, `+++ b/` headers
- Lossless when no file exceeds `diffFileTruncateLines` (300) lines
- Lossy (raw saved to pager) when large files are truncated (e.g. lock files)

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
| `/tmp/tko-<ts>-<cmd>.txt` | Pager temp files for lossy compressions |
