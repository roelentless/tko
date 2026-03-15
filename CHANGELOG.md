# Changelog

## 2026-03-15

- feature: `tko -- <cmd>` CLI — clean `--` separator between tko subcommands and wrapped commands; unknown commands suggest the correct form
- feature: git status compression — lossless ~58% token reduction; brace-groups files sharing a directory, compact branch/upstream/ahead-behind line
- feature: git diff compression — per-file +/- stats, strips boilerplate headers; large files truncated with pager fallback so raw is always recoverable
- feature: token tracking — SQLite-backed `tko stats` and `tko misses` to measure savings and discover high-value handlers to add next
- feature: Claude Code hook — `tko hook claude` registers the tko binary directly as a `PreToolUse` hook; no shell script or `jq` dependency; rewrites handled commands transparently before the agent sees them
- feature: pager — lossy compressions save full raw output to `/tmp/tko-<ts>-<cmd>.txt` with a hint line so the agent can always reach the original
