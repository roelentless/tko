# Handler Writing Guide

Guidelines for writing compression handlers in `internal/commands/`.
See [RFC-002](../../rfc/RFC-002-handlers.md) for the planned handler roadmap.

---

## Structure

Each handler lives in its own file under `internal/commands/<binary>/`:

```
internal/commands/
  git/
    status.go       ← gitStatusHandler
    diff.go         ← gitDiffHandler
    global.go       ← shared flag-stripping helpers
    status_test.go
    diff_test.go
    routing_test.go
```

One handler per file. One `init()` per file. No shared dispatch structs.

---

## The interface

```go
type Handler interface {
    Route()    string                                              // binary name: "git", "npm"
    Supports(args []string) bool                                   // owns this exact invocation?
    Handle(args []string, rawStdout, rawStderr string) (*Result, error)
}
```

Register in `init()`:

```go
func init() { commands.Register(&myHandler{}) }
```

Then import in `cmd/tko/main.go`:

```go
_ "tko/internal/commands/mypackage"
```

---

## Route()

Return the binary name only — no subcommand, no path.

```go
func (h *myHandler) Route() string { return "git" }  // not "git status"
```

Multiple handlers share the same `Route()`. The registry holds a list per key;
`Match()` walks it in registration order and returns the first `Supports()` match.

---

## Supports()

**This is the routing contract.** Get it right — everything else depends on it.

Rules:
- Strip known global flags (e.g. `-C <path>`) before checking the subcommand. See `git/global.go` for the pattern.
- Match exactly what you handle. If you handle `git status` with no extra args, reject `git status --short`.
- Use an allowlist for flags. Never accept unknown flags on the assumption they're safe — different flags produce different output formats.
- Return `false` for anything unfamiliar. Passthrough is always the safe fallback.

```go
func (h *gitStatusHandler) Supports(args []string) bool {
    sub, rest, ok := gitSubcommand(args) // strips -C etc.
    return ok && sub == "status" && len(rest) == 0
}
```

---

## Handle()

**Never modify the input.** `args` are forwarded to the runner unchanged. The command
the agent requested must be the command that runs. Only transform `rawStdout`/`rawStderr`.

```go
func (h *myHandler) Handle(args []string, rawStdout, rawStderr string) (*Result, error) {
    compressed, lossless := compress(rawStdout)
    return &Result{
        Stdout:   compressed,
        Lossless: lossless,
    }, nil
}
```

On error, return `nil, err`. tko falls back to raw passthrough and logs the error.
Never swallow errors and return garbled output.

---

## Lossless only

All handlers must be lossless. `Lossless: true` is the only valid declaration.

If you cannot guarantee that the compressed output contains every piece of information
from the original, return `nil, err` instead. tko will fall back to raw passthrough.
There is no pager, no temp file, no truncation path.

---

## Output format principles

- **Dense but readable.** The agent reads the output; make it scannable.
- **Preserve structure.** File paths, counts, status codes, error locations — keep them.
- **Strip ceremony.** Instructional text ("use `git add` to stage"), progress lines ("Compiling foo v1.0"), column padding — remove these.
- **No invented structure.** Don't introduce YAML/JSON wrapping that wasn't there. Use the natural shape of the data.
- **Counts help.** `staged(3):` tells the agent how many items follow without it having to count.

---

## Tests

Every opt-in pattern in `Supports()` must have a test. Tests must:

1. **Use the real binary** — no mocking. Create a temp environment with `t.TempDir()`.
2. **Test `Supports()` directly** for each pattern (and rejection cases).
3. **Test the compressed output** — assert structure, not just that it ran.
4. **Assert `Lossless`** — if your handler declares lossless, the test must verify nothing was dropped.
5. **No absolute paths in string literals** — unit test data (strings passed to `Handle`, `Supports`, or helper functions directly) must use relative paths like `foo/bar/baz.txt`. Never use paths starting with `/`. Integration tests that shell out to a real binary via `t.TempDir()` will produce absolute paths from the OS — that is the only acceptable source of absolute paths in tests.

See `git/routing_test.go` for `Supports()` pattern coverage and `git/status_test.go`
for integration test structure.

### Rejection cases to always cover

```go
// Unknown flag → passthrough
handler.Supports([]string{"subcommand", "--unknown-flag"}) // want: false

// Wrong subcommand → not our handler
handler.Supports([]string{"other-subcommand"}) // want: false

// Global flag that changes output format → passthrough
handler.Supports([]string{"--format=json", "subcommand"}) // want: false
```

---

## Checklist before opening a PR

- [ ] `Route()` returns the binary name only
- [ ] `Supports()` uses an allowlist for flags, not a blocklist (exception: commands whose default output is plain paths and where only a small set of flags change that format — e.g. `find`, `fd` — may use a blocklist of output-modifying flags)
- [ ] `Supports()` strips global flags via a shared helper
- [ ] `Handle()` does not modify `args`
- [ ] `Lossless: true` — if lossless cannot be guaranteed, return `nil, err` instead
- [ ] Every opt-in pattern has a test
- [ ] Every rejection case has a test
- [ ] `go test ./...` passes
