Update project documentation to reflect current state of the codebase.

## Input
$ARGUMENTS — optional description of what changed (e.g., "RDS is now complete")

## Workflow

1. Read `README.md`, `PLAN.md`, and check recent `git log --oneline -10` to understand what shipped.
2. Update `README.md`:
   - Feature matrix: change status emojis (🚧 → ✅) for completed features
   - Key bindings table: add any new bindings
   - Configuration: update if new config options were added
3. Update `PLAN.md`:
   - Mark completed milestones with ✅
   - Update implementation order notes at the bottom
   - Add new sub-items to milestones if features were expanded
4. Commit with `docs:` prefix. Do NOT add Co-Authored-By lines.
5. Push to the current branch.
