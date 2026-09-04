# Development Guide

## Local Commands

```bash
go run ./cmd/unic
go test ./...
make build
```

Read-only automation commands live under `internal/cli/`; keep their `--json` output versioned and deterministic, write only JSON to stdout, and cover human and JSON output paths with CLI tests.

## Branch Naming

Use [`branch-naming-harness.md`](branch-naming-harness.md) for branch names.

Preferred format:

```text
<work-type>/<issue-number>-<short-description>
```

## Work Tracking

Use GitHub issues and pull requests as the source of truth for planned work,
implementation status, and delivery decisions.

Before creating an issue, search both open and closed issues and check open pull
requests for existing coverage. Create new issues only from explicit maintainer
direction or a concrete gap supported by repository files or observable
behavior; do not invent roadmap items or mandatory milestone references.

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
