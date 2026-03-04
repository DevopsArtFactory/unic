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
│   ├── auth/                  # Login / credential management
│   │   ├── auth.go            # Unified credential provider logic
│   │   ├── saml.go            # Okta SAML federation
│   │   ├── sso.go             # IAM Identity Center
│   │   ├── iam.go             # Access Key / Secret Key
│   │   └── sts.go             # Assume Role, role chaining
│   ├── services/              # AWS service modules
│   │   └── aws/
│   │       ├── vpc.go
│   │       ├── rds.go
│   │       ├── iam.go
│   │       ├── ssm.go
│   │       ├── ec2.go
│   │       ├── r53.go
│   │       ├── cloudwatch_logs.go
│   │       └── cloudwatch_metrics.go
│   ├── app/                   # Bubbletea TUI application
│   │   ├── app.go             # Root model
│   │   ├── screens.go         # Screen navigation stack
│   │   ├── actions.go         # Screen transitions
│   │   └── navigation.go      # Cursor/scroll
│   ├── tui/                   # Reusable Bubbletea components
│   │   ├── components.go      # Lists, dialogs, spinners
│   │   └── styles.go          # Lipgloss styles
│   ├── domain/                # Pure business models
│   │   ├── model.go
│   │   └── catalog.go
│   └── config/                # Profile / account / region management
│       └── config.go
├── go.mod
├── go.sum
├── Makefile
└── PLAN.md
```

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

### M1 — Core Foundation

**M1.1 — Config & Profile Management**
- Read/write `~/.aws/config` and `~/.aws/credentials` (shared with AWS CLI)
- unic app config at `~/.config/unic/config.yaml` (UI prefs, default profile)
- Multi-account profile support
- CLI flags: `--profile <name>`, `--region <region>`

**M1.2 — TUI Shell**
- Main menu: service list navigation
- Screen router with back navigation stack (Bubbletea model stack)
- Shared layout: header (profile / region / account), body, footer (keybindings)
- Reusable components: filterable list, confirmation dialog, notification bar, loading spinner

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

**M3.1 — VPC**
- List VPCs → subnets → show available IP count per subnet
- Reachability Analysis: create/run `NetworkInsights` path, display results

**M3.2 — RDS**
- List DB instances/clusters with status
- Start / Stop (with confirmation)
- Failover for Multi-AZ (with confirmation)
- Real-time status polling after action

**M3.3 — IAM Credentials**
- List access keys, show key age/status
- Rotate: create new key → display once → deactivate old (with confirmation)

**M3.4 — Systems Manager (Sessions Manager)**
- List SSM-eligible EC2 instances (agent status check)
- Select instance → open interactive session
- Implementation: suspend Bubbletea program → spawn `session-manager-plugin` process → resume on exit
- Prerequisite check: verify `session-manager-plugin` is installed

**M3.5 — EC2 Security Groups**
- List/filter security groups (by VPC, name, ID)
- View inbound + outbound rules
- Add / Delete / Modify rules (protocol, port range, source/dest CIDR or SG)
- Confirmation before apply

**M3.6 — Route 53**
- List hosted zones → records
- Modify A / CNAME records (value, TTL)
- Create / Delete records
- Show change status (PENDING → INSYNC)

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

---

### M4 — Polish & Release

**M4.1 — UX**
- Keyboard shortcut help screen (`?` key)
- Fuzzy search/filter on all list views
- Loading spinners for async operations
- Color theme (respect terminal colors via Lipgloss)

**M4.2 — Error Handling & Logging**
- Structured error messages with actionable hints
- Debug log file (`~/.config/unic/logs/`)
- `--verbose` flag

**M4.3 — Distribution**
- GitHub Actions CI/CD
- Prebuilt binaries (Linux, macOS, Windows)
- Homebrew formula
- Install script

---

## Implementation Order

```
M1.1 → M1.2 → M2.1 → M2.2 → M2.3 → M2.4
                                        ↓
                        M3.1 ~ M3.8 (independent, any order)
                                        ↓
                                  M4.1 → M4.2 → M4.3
```

- M1 + M2 are sequential — prerequisites for everything else
- M3 services are independent of each other, build in any order
- M3.4 (SSM Sessions) has highest complexity (TUI ↔ child process handoff) — consider doing last in M3
- M4 after all features are functional

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
