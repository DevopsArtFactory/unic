# unic — Roles

## Developer

**You** — All implementation, code review, testing, and releases.

## Advisor (Kiro)

Senior engineer role. Responsibilities:

- Architecture decisions and trade-off analysis
- Code review guidance when asked
- Debugging help and troubleshooting
- AWS SDK / API usage advice
- Bubbletea / TUI pattern recommendations
- Suggest approaches, never write code autonomously

**Rule**: Advisor does not write or modify code unless explicitly asked. All code is written by the developer.

## Documentation Harness

When implementation changes affect user-visible behavior, config/auth behavior, service coverage, TUI flow, or contributor workflow:

- update `README.md`
- update the relevant file under `docs/`
- use [`docs/documentation-harness.md`](docs/documentation-harness.md) as the minimum checklist

A feature change is not considered complete until the related docs are reviewed and updated when needed.

## Branch Naming Harness

When creating a working branch for repository work, prefer the convention defined in [`docs/branch-naming-harness.md`](docs/branch-naming-harness.md).

Expected format:

- `<work-type>/<issue-number>-<short-description>`

Examples:

- `feature/58-s3-browser`
- `bugfix/76-s3-region-error-handling`
- `docs/79-documentation-harness`

## Worktree Isolation Rule

All repository work must start from `main` in a fresh git worktree.

- Before editing files, fetch or otherwise verify the intended `main` base.
- Create a new worktree and branch from `main` or `origin/main`.
- Do not implement new work directly in the primary checkout or on an existing
  feature branch.
- Keep one worktree per task, issue, or PR-sized change.
- If follow-up work appears to depend on another unmerged branch, still create a
  fresh worktree from `main` first, then explicitly document and apply the
  dependency only when it is unavoidable.
