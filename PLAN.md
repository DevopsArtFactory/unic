# unic — Implementation Plan (Phase 1)

## Overview

AWS DevOps CLI with TUI (Bubbletea) that supports multiple login methods and provides interactive access to commonly used AWS services. Built with Go + Cobra + Bubbletea.

Open-source project — region/account selectable by user.

---

## Architecture

```
unic/
├── cmd/unic/
│   └── main.go               # Entry point
├── internal/
│   ├── cli/                   # Cobra command definitions
│   │   ├── root.go            # Root command, global flags
│   │   └── init.go            # unic init subcommand
│   ├── config/                # Profile / account / region management
│   │   └── config.go
│   ├── domain/                # Pure business models
│   │   ├── model.go           # AwsService, FeatureKind, Service, Feature
│   │   └── catalog.go         # Service/feature catalog
│   ├── app/                   # Bubbletea TUI application
│   │   └── app.go             # Root model, screens, navigation, rendering
│   └── services/              # AWS service modules
│       └── aws/
│           ├── repository.go  # AwsRepository (client initialization)
│           ├── ec2.go         # EC2 instance listing (SSM-managed)
│           ├── ec2_model.go   # EC2Instance model
│           ├── vpc.go         # VPC/Subnet/IP queries
│           ├── vpc_model.go   # VPC, Subnet models
│           ├── ssm.go         # SSM session start/terminate
│           └── ssm_exec.go    # session-manager-plugin subprocess
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yaml
└── PLAN.md
```

> **Note**: `internal/auth/` and `internal/tui/` are planned but not yet implemented.
> The TUI screens, navigation, and styles are currently consolidated in `internal/app/app.go`.

---

## Decisions

| Topic | Decision | Rationale |
|-------|----------|-----------|
| Credentials | `~/.aws/credentials` + `~/.aws/config` | Reuse AWS CLI standard paths, no custom credential store needed |
| App config | XDG-compliant (`~/.config/unic/config.yaml`) | unic-specific settings (default profile, UI preferences) separate from AWS creds |
| SSM sessions | Depend on `session-manager-plugin` | Pure SDK WebSocket shell is impractical; plugin is AWS-recommended |
| Okta MFA | Support all Okta-supported factors (Push, TOTP, SMS, etc.) | Maximum compatibility |
| Concurrency | goroutines + errgroup | Native Go concurrency; no external runtime needed |

---

## Milestones

### M1 — Core Foundation ✅

**M1.1 — Config & Profile Management** ✅
- Read/write `~/.aws/config` and `~/.aws/credentials` (shared with AWS CLI)
- unic app config at `~/.config/unic/config.yaml` (UI prefs, default profile)
- Multi-account profile support
- CLI flags: `--profile <name>`, `--region <region>`
- `unic init` command for config file creation

**M1.2 — TUI Shell** ✅
- Main menu: service list navigation
- Screen router with back navigation stack (Bubbletea model stack)
- Shared layout: header (profile / region / account), body, footer (keybindings)
- Reusable components: filterable list, loading spinner, error display
- 8 screens: ServiceList, FeatureList, InstanceList, VPCList, SubnetList, SubnetDetail, Loading, Error

---

### M2 — Authentication

**M2.1 — IAM User (Access Key / Secret Key)**
- Read from `~/.aws/credentials` or prompt interactively
- Validate via `sts:GetCallerIdentity`
- Credential expiry detection + re-auth prompt

**M2.2 — AWS IAM Identity Center (SSO)**
- `sso-oidc` device authorization flow
- Browser open for login, poll for token
- Cache SSO token, auto-refresh
- List available accounts/roles post-login

**M2.3 — Okta SAML Federation**
- Okta API authentication (username/password)
- MFA challenge handling (Push, TOTP, SMS, and other Okta-supported factors)
- SAML assertion retrieval → `sts:AssumeRoleWithSAML`
- Okta session token caching

**M2.4 — Assume Role & Role Chaining**
- `sts:AssumeRole` with optional external ID
- Role chaining (A → B → C, up to AWS limit)
- Configurable session name
- TUI: role selector when multiple roles available

---

### M3 — AWS Services

**M3.1 — VPC** ✅
- List VPCs → subnets → show available IP count per subnet
- Reachability Analysis: create/run `NetworkInsights` path, display results

**M3.2 — RDS** ✅
- List DB instances/clusters with status
- Start / Stop (with confirmation)
- Failover for Multi-AZ (with confirmation)
- Real-time status polling after action
- Aurora cluster-level stop/start/failover
- Type-to-confirm for destructive actions (stop, failover)

**M3.3 — IAM Credentials** ✅
- List access keys, show key age/status ✅
- Rotate: create new key → display once → deactivate old (with confirmation) ✅

**M3.4 — Systems Manager (Sessions Manager)** ✅
- List SSM-eligible EC2 instances (agent status check)
- Select instance → open interactive session
- Implementation: suspend Bubbletea program → spawn `session-manager-plugin` process → resume on exit
- Prerequisite check: verify `session-manager-plugin` is installed

**M3.5 — EC2 Security Groups** ✅
- List/filter security groups (by VPC, name, ID) ✅
- View inbound + outbound rules ✅
- Add / Delete rules (protocol, port range, source/dest CIDR or SG reference) ✅
- Type-to-confirm for rule deletion ✅

**M3.6 — Route 53**
- List hosted zones → records
- Modify A / CNAME records (value, TTL)
- Create / Delete records
- Show change status (PENDING → INSYNC)

**M3.6.1 — Route 53 (Phase 1)** ✅
- List hosted zones → drill into DNS records → record detail view
- Filter support on zone list and record list

**M3.9 — IAM Users** ✅
- List IAM users with metadata (creation date, last activity, MFA status) ✅
- View user details (attached policies, groups, access keys) ✅

**M3.7 — CloudWatch Logs**
- List log groups → log streams
- Live tail (polling via `FilterLogEvents`)
- Time range search + filter pattern
- Log line syntax highlighting

**M3.8 — CloudWatch Metrics**
- List namespaces → metrics → dimensions
- Select metric + time range + period
- Graph rendering: Bubbletea viewport with braille/block characters
- Multiple metrics overlay on single chart

**M3.9 — Secrets Manager** ✅
- List secrets ✅
- Drill into secret detail: name, key/value pairs, encryption key (KMS key ID) ✅

---

### M4 — Polish & Release

**M4.1 — UX**
- Keyboard shortcut help screen (`?` key)
- Color theme (respect terminal colors via Lipgloss)
- ~~Fuzzy search/filter on all list views~~ → moved to M6
- ~~Loading spinners for async operations~~ → moved to M5.3

**M4.2 — Error Handling & Logging** ✅
- Structured error messages with actionable hints
- Debug log file (`~/.config/unic/logs/`) ✅
- `--verbose` flag ✅

**M4.3 — Distribution** 🟡
- GitHub Actions CI/CD ✅
- Prebuilt binaries (Linux, macOS, Windows) ✅ (GoReleaser configured)
- Homebrew formula
- Install script

---

### M5 — UI Beautification (Charmbracelet Ecosystem)

Prerequisite: `go get github.com/charmbracelet/bubbles`

**M5.1 — File Extraction (no behavior change)** ✅
- Split `internal/app/app.go` (~1700 lines) into focused files:
  - `styles.go` — all lipgloss style vars + new styles
  - `views.go` — all `view*()` methods + `renderStatusBar()`
  - `commands.go` — all `tea.Cmd` functions (`load*`, `execute*`, `poll*`, etc.)
  - `filter.go` — filter logic (`applyFilter`, `applyRDSFilter`, `applyIPFilter`)
- `app.go` retains: Model struct, `New()`, `Init()`, `Update()`, and all `update*()` methods

**M5.2 — bubbles/textinput for Filter**
- Replace manual character-by-character filter input with a single shared `textinput.Model`
- On "/" key → `filterTI.Focus()`, on esc/enter → `filterTI.Blur()`
- Check `filterTI.Focused()` instead of `filterActive` booleans
- Remove: `filterInput`, `filterActive`, `rdsFilter`, `rdsFilterActive`, `ipFilter`, `ipFilterActive`

**M5.3 — bubbles/spinner for Loading**
- Replace static `"Loading..."` with animated spinner (`spinner.MiniDot`)
- Start spinner tick on entering `screenLoading`, ignore ticks elsewhere

**M5.4 — bubbles/table for Context Picker**
- Convert manual column-aligned context picker to `bubbles/table`
- Columns: Name, Region, Auth, Current (*)
- Keep "a" key shortcut for adding contexts

**M5.5 — Enhanced Lipgloss Styles**
- Status bar: full-width via `lipgloss.Width(m.width)`, left/right split with `JoinHorizontal`
- List views: wrap content in `lipgloss.RoundedBorder()` box
- Detail views: consistent label alignment with `lipgloss.NewStyle().Width(14)`
- Help bar: consistent `helpStyle` across all screens

---

### M6 — Search/Filter for Long Lists

Prerequisite: `go get github.com/sahilm/fuzzy`

**M6.1 — Fuzzy Matching**
- Replace `strings.Contains` filter logic with `sahilm/fuzzy` scored fuzzy matching
- Match against `DisplayTitle()` for both filtering and highlighting
- Sort results by score descending

**M6.2 — Match Highlighting**
- Highlight matched characters with bold + orange (color 214) style
- Use match indices from fuzzy library to render per-character styling

**M6.3 — Filter on All List Views**
- Add "/" filter to VPC list and subnet list (currently missing)
- All filterable items already implement `FilterText()` and `DisplayTitle()`

**M6.4 — Unified Filter Architecture**
- Define `Filterable` interface: `FilterText() string`, `DisplayTitle() string`
- Implement generic `applyFuzzyFilter[T Filterable](items []T, query string) []filterMatch[T]`
- Store `matchIndices [][]int` parallel to filtered slices for highlighting
- Eliminate per-screen filter state duplication

---

## Implementation Order

```
M1.1 → M1.2 → M2.1 → M2.2 → M2.3 → M2.4
                                        ↓
                        M3.1 ~ M3.9 (independent, any order)
                                        ↓
                                  M4.1 → M4.2 → M4.3
                                        ↓
                              M5.1 → M5.2 ~ M5.5
                                        ↓
                                  M6.1 ~ M6.4
```

Note: M5.2 (textinput) provides the foundation for M6 (enhanced search) — they converge naturally.

- M1 is complete; M2 is deferred (relying on AWS SDK default credential chain)
- M2.1 (Context-based auth with SSO, credential, assume-role) is complete
- M3 services are independent of each other, build in any order
- M3.1 (VPC), M3.2 (RDS), M3.3 (IAM Credentials), M3.4 (SSM Sessions), M3.5 (Security Groups), M3.6.1 (Route53 phase 1), IAM Users, and Secrets Manager are complete
- M4.3 (Distribution) is partially done (GoReleaser + GitHub Actions)
- M5.1 (File Extraction) is complete — app.go split into per-screen files
- M5 and M6 can begin independently of remaining M3/M4 work

---

## Tech Stack

| Component | Choice |
|-----------|--------|
| Language | Go (1.22+) |
| CLI Parser | Cobra |
| TUI | Bubbletea + Lipgloss + Bubbles |
| AWS SDK | aws-sdk-go-v2 (ec2, rds, ssm, iam, route53, cloudwatchlogs, cloudwatch, sts, sso, ssooidc) |
| Config | gopkg.in/yaml.v3 |
| Error handling | fmt.Errorf / errors |
| Concurrency | goroutines + errgroup |
| HTTP (Okta API) | net/http (stdlib) |
