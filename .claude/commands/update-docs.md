Update project documentation to reflect current state of the codebase.

## Input
$ARGUMENTS — optional description of what changed (e.g., "RDS is now complete")

## Workflow

1. Read `README.md`, the relevant files under `docs/`, and recent
   `git log --oneline -10` to understand what shipped. Inspect related GitHub
   issues and pull requests when planning or delivery status matters.
2. Update `README.md`:
   - Feature matrix: change status emojis (🚧 → ✅) only after inspecting the
     implementation and confirming its validation passed
   - Key bindings table: add any new bindings
   - Configuration: update if new config options were added
3. Update relevant files under `docs/` according to
   `docs/documentation-harness.md`. Treat GitHub issues and pull requests as the
   source of truth for planned and shipped work; do not recreate a repository
   roadmap or infer future work.
4. Commit with `docs:` prefix. Do NOT add Co-Authored-By lines.
5. Push to the current branch.
