# tko — Claude Code Instructions

Read [`AGENTS.md`](AGENTS.md) first. It is the primary reference for this codebase.

---

## Tools

- `fd` is installed for fast file finding (use instead of `find` when possible)
- `rg` (ripgrep) is installed for fast content searching (use instead of `grep` when possible)

Prefer these tools for better performance.

---

## Style

- No emojis in code or docs unless explicitly requested.
- Concise responses — lead with the action or answer, not the reasoning.
- Do not add comments, docstrings, or type annotations to code you did not change.
- Do not introduce abstractions or helpers for one-off operations.
- Do not add error handling for scenarios that cannot happen.
- Avoid backwards-compatibility shims. If something is unused, delete it.
