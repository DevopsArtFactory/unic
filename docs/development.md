# Development Guide

## Local Commands

```bash
go run ./cmd/unic
go test ./...
make build
```

## Branch Naming

Use [`branch-naming-harness.md`](branch-naming-harness.md) for branch names.

Preferred format:

```text
<work-type>/<issue-number>-<short-description>
```

## Worktree Isolation

Always start repository work from `main` in a fresh git worktree.

1. Fetch or verify the intended `main` base.
2. Create a new worktree and task branch from `main` or `origin/main`.
3. Make all edits for that task inside the new worktree.
4. Keep one worktree per issue, feature, refactor, or PR-sized change.

Do not implement new work directly in the primary checkout or on an existing
feature branch. If a task appears to depend on another unmerged branch, still
start from `main` first and document the dependency before applying any stacked
changes.

## Adding a New AWS Feature

1. Add service or feature constants in `internal/domain/model.go`
2. Register the feature in `internal/domain/catalog.go`
3. Add AWS repository methods and UI-facing models in `internal/services/aws/`
4. Wire new state and screen transitions in `internal/app/`
5. Add tests for repository behavior and screen transitions
6. Update documentation when behavior is user-visible

## Documentation Update Rule

If you change:

- a user-facing command
- auth/config behavior
- supported AWS services/features
- screen structure or navigation

then update at least:

1. `README.md`
2. the relevant file in `docs/`

Use [`documentation-harness.md`](documentation-harness.md) as the required checklist for deciding which docs must move with the implementation.

## Testing Expectations

Prefer tests for:

- repository methods with mocked AWS clients
- config and auth helpers
- TUI transition logic when a feature adds or changes a flow
- context add/setup flows when auth types or auth-specific branches change

## Docs Ownership Model

- `README.md`: concise user-facing entrypoint
- `docs/`: canonical detailed documentation
- `.kiro/`: internal compatibility pointers and steering helpers
