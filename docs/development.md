# Development Guide

## Local Commands

```bash
go run ./cmd/unic
go test ./...
make build
```

`make build` derives the CLI version from `git describe`; use an explicit override such as `make build VERSION=0.3.1` for reproducible packaging outside a tagged checkout.

Local builds place both executables in the repository root. Follow the [source installation steps](../README.md#build-from-source) to put them on the MCP client's `PATH`, or configure the client with an absolute path to the built `unic-mcp` executable.

`make build` and `make release` build both `unic` and `unic-mcp`. The platform targets and `make build-all` build both binaries for macOS and Linux (amd64 and arm64) and Windows (amd64). `make archive` bundles each pair in a platform archive, using the executable names expected by `install.sh`. The installer validates both extracted binaries before replacing either installed executable; `make test` includes an offline regression check for incomplete archives.

## Machine-readable command discovery

Use the registered Cobra command tree and domain catalog as the source of truth for automation contracts:

```bash
unic capabilities --json
unic schema context sync --json
```

Discovery output is deterministic, versioned JSON. New executable commands should set the `unic.dev/read-only`, `unic.dev/destructive`, and `unic.dev/output-version` annotations when their defaults do not describe the command accurately.

Read-only automation commands live under `internal/cli/`; keep their `--json` output versioned and deterministic, write only JSON to stdout, and cover human and JSON output paths with CLI tests.

The stdio MCP entry point lives at `cmd/unic-mcp` and delegates tool calls to those same CLI commands through `internal/cli.ExecuteAutomation`. Keep the MCP layer limited to protocol handling and argument mapping; AWS and config behavior belongs in the existing CLI, auth, and service packages. MCP mutation tools remain preview-only until their trust boundary is reviewed.

The repository root is also the portable agent-plugin package. Keep shared MCP guidance in `skills/unic-aws`, Kiro metadata in `plugin.json` and `mcp.json`, and client-specific manifests in `.codex-plugin`, `.claude-plugin`, and `.mcp.json`. All clients must launch the released `unic-mcp` binary from `PATH`; do not add client-specific MCP implementations.

Codex filters the environment inherited by stdio MCP servers. Keep the `.mcp.json` `env_vars` list and the README's direct Codex registration example aligned: forward AWS credential/profile/region variables, credential-provider and configuration paths, and `XDG_CONFIG_HOME` by name, never by copying credential values into manifests. When changing this list, run a Codex client-launch smoke check with dummy environment values and verify that they reach the MCP child process; MCP initialization alone does not prove that credential forwarding works. Use an isolated client configuration and a local probe server so this check needs neither real credentials nor AWS calls.

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

## Plugin Scanner CI

`.github/workflows/hol-plugin-scanner.yml` runs the SHA-pinned [HOL Plugin Scanner action](https://github.com/hashgraph-online/ai-plugin-scanner-action/tree/caba2e96aa8ad2feb6cf6fca52442b52e22e779f) on pull requests to `main` and pushes to `main`. It uses static `scan` mode with the default profile and ecosystem detection, a minimum score of 80, and failure on high or critical findings. A failed threshold keeps the job failed; the report upload does not override the result.

The job needs only `contents: read`, does not persist checkout credentials, and requires no repository or AWS secrets. Online probing, runtime `verify`, PR comments, and SARIF upload are disabled. After a report is produced, a separate step renders its score, analyzer status, and finding rules/locations into the job summary, including when the scanner fails its gate. Finding descriptions remain in the full `hol-plugin-scanner-report` JSON artifact, retained for 14 days. Setup errors before report generation may leave no summary or artifact. The workflow checks the renderer with `python3 -m unittest discover -s .github/scripts -p 'test_*.py'` before scanning; run the same command locally when changing it.

For a local reproduction, use an isolated Python environment with `plugin-scanner==3.0.123`, the version bundled by the pinned action. The action verifies the scanner wheel's committed SHA-256 and PyPI provenance and installs hash-locked dependencies; use its pinned installation files when reproducing the CI environment. Run from a clean checkout and write the report outside the repository:

```bash
env -u MCP_SCANNER_API_KEY -u MCP_SCANNER_LLM_API_KEY \
  plugin-scanner scan /path/to/clean/unic \
  --profile default --ecosystem auto \
  --cisco-skill-scan auto --cisco-mcp-scan auto --cisco-policy balanced \
  --min-score 80 --fail-on-severity high \
  --format json --output /path/outside/unic/report.json
```

Inspect the integration statuses before interpreting a score: unavailable Cisco analyzers mean those deeper checks did not run. Static JSON output's `verify_pass` field is not evidence of runtime verification. Review each finding's rule and location against the source; the existing fixture and false-positive dispositions are recorded in [issue #344](https://github.com/DevopsArtFactory/unic/issues/344#issuecomment-5595530140). Document dispositions or fix confirmed defects without lowering the gate to hide them. Keep the action SHA, bundled scanner version, and this reproduction guidance aligned when updating the scanner.

## Docs Ownership Model

- `README.md`: concise user-facing entrypoint
- `docs/`: canonical detailed documentation
- `.kiro/`: internal compatibility pointers and steering helpers
