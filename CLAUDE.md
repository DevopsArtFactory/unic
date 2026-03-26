# unic — Claude Code Guidelines

## Workflow

- **Never commit directly.** Always follow this order:
  1. Create a GitHub issue first
  2. Work on the implementation
  3. Push to a feature branch and create a PR referencing the issue
- This applies to all changes — features, fixes, improvements, docs.

## Code Patterns

- Follow existing patterns in the codebase (see RDS implementation as reference)
- Use lipgloss for styled TUI output — column-aligned tables with dimmed labels
- Tests use mock client interfaces (see `rds_test.go` pattern)
- Scroll windowing: `visibleLines := max(m.height-N, 5)`
