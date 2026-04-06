Implement a feature for the unic TUI tool.

## Input
$ARGUMENTS — feature description or GitHub issue number (e.g., "CloudWatch Logs browser" or "#29"). Can be empty.

## Phase 0: Auto-Suggest (when no argument is provided)

If `$ARGUMENTS` is empty or blank:

1. Read `PLAN.md` and identify milestones/features that are **not** marked ✅ (complete).
2. Run `gh issue list --state open --limit 30` to fetch open GitHub issues.
3. Cross-reference: find open issues whose title or body matches an incomplete PLAN.md item. Also note incomplete PLAN.md items that have no issue yet.
4. Rank candidates by priority:
   - Items in the earliest incomplete milestone come first (M3 before M4, M4 before M5, etc.)
   - Within a milestone, prefer items that already have an open GitHub issue
   - Within the same milestone, prefer items with fewer dependencies or prerequisites
5. Present the top 3–5 candidates to the user using AskUserQuestion, showing for each:
   - The PLAN.md item name and milestone (e.g., "M3.6 — Route 53 Phase 2")
   - The linked GitHub issue number if one exists, or "(no issue yet)" otherwise
   - A one-line summary of what's involved
6. After the user picks one, if no GitHub issue exists for it, remind the user that one should be created (per project workflow rules) before proceeding.
7. Continue to Phase 1 with the selected feature.

## Phase 1: Gather Context

1. If the argument is a GitHub issue number, fetch it with `gh issue view <number>`. Read the full body, checklist, and comments.
2. If the argument is a description, search for a matching issue with `gh issue list --search "<description>"` and read it if found.
3. Read `PLAN.md` (if not already read in Phase 0) to find the relevant milestone and any design decisions.
4. Read `.kiro/docs/architecture-en.md` if it exists for architectural context.
5. Explore the codebase to understand existing patterns:
   - `internal/domain/model.go` and `catalog.go` for service/feature registration
   - `internal/services/aws/repository.go` for client interfaces
   - Pick one completed service (e.g., `rds.go`, `rds_model.go`, `rds_test.go`) as the reference implementation
   - `internal/app/app.go` for screen enum, Update handlers, View functions

## Phase 2: Interview

Ask the user to clarify before implementing. Tailor questions based on what the issue and docs leave ambiguous:

- What AWS API operations are needed? (list, describe, create, delete, etc.)
- What data should be shown in the list view vs detail view?
- Are there any actions (mutating operations) needed? If so, which need confirmation?
- Any edge cases or constraints specific to their AWS environment?
- Should this go on a separate branch, or add to an existing one?

Skip questions that are already answered by the GitHub issue or PLAN.md.

## Phase 3: Plan

Enter plan mode. Design the implementation following existing patterns:
- Domain constants and catalog registration
- AWS client interface, implementation, and model
- TUI screens with scroll windowing (`visibleLines := max(m.height-N, 5)`)
- Tests using mock clients
- Destructive actions must use type-to-confirm (see `updateRDSConfirm` pattern)

## Phase 4: Implement

After plan approval, implement in this order:
1. Domain model (`internal/domain/model.go`, `catalog.go`)
2. AWS service layer (`internal/services/aws/<service>.go`, `<service>_model.go`)
3. Client interface update (`internal/services/aws/repository.go`)
4. TUI integration (`internal/app/app.go`)
5. Tests (`internal/services/aws/<service>_test.go`, `internal/app/app_test.go`)

## Phase 5: Verify

1. `make test` — all tests pass
2. `make build` — binary compiles
3. Report what was implemented and suggest running `/ship-pr` to create the PR

Do NOT add Co-Authored-By lines to commits.
