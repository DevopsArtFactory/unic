Act as a Senior Software Engineer conducting a thorough code quality review of this codebase. Focus on Clean Code principles, Go idioms, and actionable refactoring advice.

## Input
$ARGUMENTS — optional: specific focus area or concern (e.g., "screen_route53.go is getting big"). If empty, review the entire codebase.

## Phase 1: LoC Inventory

1. Run `find internal/ cmd/ -name '*.go' | xargs wc -l | sort -rn` to get line counts for all Go source files.
2. Run `find internal/ cmd/ -name '*_test.go' | xargs wc -l | sort -rn` separately for test files.
3. Produce a **LoC Breakdown Table** sorted by line count descending:
   - Columns: File, Lines, Status
   - Status: flag files > 300 lines as `[LONG]`, > 500 as `[CRITICAL]`
   - Include a total line count and average

## Phase 2: Deep Review

Use Explore agents to read and analyze source files. For each file, evaluate against these categories:

### Category 1: File Size & Function Length
- Files exceeding 300 LoC — suggest extraction/splitting
- Functions exceeding 50 LoC — suggest decomposition
- Deeply nested blocks (3+ levels) — suggest early returns or extraction

### Category 2: Naming & Readability
- Go idioms: short names in small scopes, descriptive names in exported APIs
- Receiver names: single-letter lowercase matching the type (Go convention)
- Misleading or ambiguous names
- Unnecessary or stale comments (code should be self-documenting)
- Magic numbers or string literals that should be constants

### Category 3: SOLID / Abstraction / DRY
- Single Responsibility: does each file/struct have one clear purpose?
- Duplicate code: repeated logic across files that should be extracted
- Interface segregation: are interfaces minimal or bloated?
- Premature abstraction: abstractions that serve only one call site
- Missing abstraction: repeated patterns that warrant a shared helper

### Category 4: Error Handling & Edge Cases
- Swallowed errors (err ignored or logged but not returned)
- Inconsistent error wrapping (`fmt.Errorf` with `%w` vs without)
- Missing nil checks on pointers before dereference
- Boundary validation at system edges (user input, AWS API responses)

### Category 5: Go-Specific Idioms
- Proper use of `context.Context` (passed through, not stored in structs)
- Correct error type assertions vs string matching
- Goroutine leaks or missing cancellation
- Proper use of `defer` for cleanup
- Struct field ordering (exported before unexported, logical grouping)

## Phase 3: Report

Produce a structured report with the following sections:

### 1. Executive Summary
2-3 sentences: overall code health, biggest concern, biggest strength.

### 2. LoC Breakdown Table
From Phase 1.

### 3. Findings

For each finding, use this format:

```
[SEVERITY] Category — Short title
  File: path/to/file.go:LINE
  Issue: What is wrong and why it matters.
  Suggestion: Concrete refactoring advice (what to extract, rename, restructure).
```

Severity levels:
- **[CRITICAL]** — Likely bug, maintainability blocker, or violated Go best practice that will cause issues at scale
- **[WARNING]** — Code smell that hurts readability or maintainability but works correctly today
- **[NIT]** — Style preference or minor improvement, low priority

### 4. Refactoring Priorities
A ranked list of the top 5 most impactful refactorings, each with:
- What to change
- Why it matters
- Estimated scope (which files, rough effort)

### 5. Positive Patterns
Call out 2-3 things the codebase does well — patterns worth preserving or extending.

## Guidelines

- Be direct and opinionated. A senior engineer gives clear recommendations, not vague suggestions.
- Every finding must include a concrete `Suggestion:` — don't just point out problems.
- Respect existing patterns. If the codebase consistently does X, don't flag individual instances of X — flag the pattern itself if it's problematic.
- Don't flag test files for naming or length unless they have actual quality issues (copy-paste test logic, missing edge cases).
- Don't suggest changes that would break the existing architecture without a strong justification.
- If $ARGUMENTS specifies a focus area, still produce the full LoC table but prioritize findings in that area.
