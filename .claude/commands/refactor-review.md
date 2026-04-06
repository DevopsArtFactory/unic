Refactor the codebase based on a prior `/senior-review` report.

## Input
$ARGUMENTS — optional: specific finding or priority number to tackle (e.g., "priority 1", "#3 DRY filter logic", "all warnings"). If empty, work through findings by priority order.

## Phase 1: Load Review

1. Find the most recent review report in `.claude/reports/senior-review-*.md` (sort by filename descending to get the latest).
2. If no report exists, tell the user to run `/senior-review` first and stop.
3. Read the full report. Parse out:
   - The **Refactoring Priorities** section (ranked list)
   - All **Findings** with their severity, file paths, and suggestions

## Phase 2: Select Scope

If `$ARGUMENTS` is provided:
- Match it against priority numbers ("priority 1", "priority 2") or finding titles/categories
- Scope the work to just those items

If `$ARGUMENTS` is empty:
- Present the Refactoring Priorities list to the user via AskUserQuestion
- Ask which priority (or priorities) to tackle in this session
- Recommend starting with Priority 1

## Phase 3: Plan

Enter plan mode. For each selected refactoring:

1. Read every file mentioned in the finding's `File:` references
2. Understand the current pattern and all call sites
3. Design the refactoring following the finding's `Suggestion:` as a starting point, but adapt based on what you see in the actual code
4. Ensure the refactoring does NOT change behavior — this is a pure refactor
5. Identify all files that need to change and the order of changes

Key constraints:
- Preserve all existing tests — they must still pass after refactoring
- Do not change public API signatures unless the finding explicitly calls for it
- If a refactoring touches more than 10 files, break it into smaller commits
- Follow existing code patterns (lipgloss styling, Bubbletea conventions, etc.)

## Phase 4: Implement

After plan approval, implement the refactoring:

1. Make changes file by file, following the plan order
2. After each logical group of changes, run `make test` to verify nothing broke
3. If tests fail, fix immediately before continuing

## Phase 5: Verify & Update Report

1. `make test` — all tests pass
2. `make build` — binary compiles
3. Re-run the LoC count (`find internal/ cmd/ -name '*.go' ! -name '*_test.go' | xargs wc -l | sort -rn`) and compare against the report's original LoC table
4. Update the review report file: mark completed findings as `[DONE]` and note what changed
5. Report a summary of what was refactored and suggest running `/ship-pr` to create the PR

## Guidelines

- This is a **refactor-only** skill. No new features, no behavior changes.
- Every change must be justified by a specific finding in the review report.
- If a suggestion from the report turns out to be impractical after reading the code, skip it and explain why.
- Prefer small, focused changes over large sweeping rewrites.
- If the refactoring reveals new issues not in the original report, note them but don't fix them — they belong in the next `/senior-review` cycle.
