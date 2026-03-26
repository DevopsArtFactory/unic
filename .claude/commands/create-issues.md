Create GitHub issues for planned but unimplemented features.

## Input
$ARGUMENTS — optional scope (e.g., "M3 services" or "all remaining")

## Workflow

1. Read `PLAN.md` to identify planned milestones and features.
2. Run `gh issue list --state all --limit 50` to see existing issues.
3. Identify features in PLAN.md that don't have corresponding issues yet.
4. For each missing feature, create an issue with `gh issue create`:
   - Title: `feat: <description> (M<X>.<Y>)` with the milestone reference
   - Label: `enhancement`
   - Body format:
     ```
     ## Summary
     <1-2 sentence description>

     ## Details
     <bullet points of what needs to be built>

     ## Checklist
     <implementation steps referencing specific files/directories>
     ```
5. Skip features that already have open or closed issues.
6. Report the list of created issues with URLs.
