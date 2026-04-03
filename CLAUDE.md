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

## README Maintenance

- **피쳐를 추가, 수정, 삭제할 때 반드시 `README.md`를 함께 업데이트한다.**
  - `Currently Implemented Features` 테이블: 새 서비스/기능 추가, 상태 변경(🚧→✅), 삭제된 항목 제거
  - `TUI Key Bindings` 테이블: 키 바인딩이 추가/변경/삭제된 경우 반영
  - `Usage` 섹션: 새로운 CLI 명령이나 플래그가 추가된 경우 반영
  - `Configuration` 섹션: 설정 형식이 변경된 경우 반영
- PR 생성 전 README가 현재 구현 상태와 일치하는지 확인한다.
