# Changelog

## 2026-03-16

- feature: `go build` compression — strips the common module prefix from `# package/path` error header lines (e.g. `# github.com/org/repo/internal/foo` → `# ./internal/foo`); error lines preserved verbatim; passes through unchanged on success or when there is no common prefix

## 2026-03-15

- feature: `git log` compression — shows up to 20 commits with hash, date, subject; lossy for large histories (pager saves full output), lossless for small repos or explicit `-n ≤ 20`; `--oneline` passed through unchanged
- feature: `git show` compression — commit header (hash, author, date, subject) followed by compressed diff using the existing diff handler; lossy when any file exceeds the diff truncation threshold
- feature: `go test` compression — strips passing package lines (`ok  \t...`), `=== RUN`, and `--- PASS:` noise; failure details and `FAIL\t...` package summaries shown in full
- feature: `ls` compression — plain `ls` collapses to a single count line; `ls -la` groups dirs, files (by extension), and hidden entries with a total count; permissions, owner, group, and date columns stripped

- feature: `tko -- <cmd>` CLI — clean `--` separator between tko subcommands and wrapped commands; unknown commands suggest the correct form
- feature: git status compression — lossless ~58% token reduction; brace-groups files sharing a directory, compact branch/upstream/ahead-behind line
- feature: git diff compression — per-file +/- stats, strips boilerplate headers; large files truncated with pager fallback so raw is always recoverable
- feature: token tracking — SQLite-backed `tko stats` and `tko misses` to measure savings and discover high-value handlers to add next
- feature: Claude Code hook — `tko hook claude` registers the tko binary directly as a `PreToolUse` hook; no shell script or `jq` dependency; rewrites handled commands transparently before the agent sees them
- feature: pager — lossy compressions save full raw output to `/tmp/tko-<ts>-<cmd>.txt` with a hint line so the agent can always reach the original
