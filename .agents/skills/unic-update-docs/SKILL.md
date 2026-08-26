---
name: unic-update-docs
description: Use when the user asks to refresh documentation for the unic repository after code changes, shipped features, delivery status changes, or README drift. Do not use for feature implementation or PR shipping.
---

Update `unic` documentation so the checked-in docs match the current repo
state.

## Workflow

1. Gather current state.
- Read `README.md` and the relevant files under `docs/`.
- Check recent history with `git log --oneline -10`.
- If the docs update is tied to shipped or in-flight GitHub work, inspect the
  source issue or PR with `gh issue view <number>` or `gh pr view <number>`.
- When feature naming or status is ambiguous, use
  `gh issue list --state open --limit 50` to align the docs with the actual
  backlog.
- Read the touched code paths or diff when the docs change is tied to a
  specific implementation.

2. Update `README.md` to match shipped behavior.
- Keep `Currently Implemented Features` accurate.
- Update `TUI Key Bindings` when shortcuts changed.
- Update `Usage` when commands or flags changed.
- Update `Configuration` when config shape or precedence changed.

1. Use GitHub tracking for planning status.
- Treat related issues and pull requests as the source of truth for planned and
  shipped work.
- Do not recreate a repository roadmap or infer future work from implementation
  details.
- Report issue or pull request status drift that is outside the requested docs
  update.

4. Keep the docs factual.
- Do not mark work complete unless the code and validation support it.
- Do not invent roadmap items or future scope that is not grounded in the repo.

## Output

Summarize what documentation changed and call out any remaining mismatches you
could not resolve from the current repo state.
