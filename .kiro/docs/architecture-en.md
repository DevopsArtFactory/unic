# UNIC Architecture Document

## Overview

UNIC (Unified Infrastructure Console) is a Rust TUI tool for browsing AWS resources in the terminal.
It manages authentication contexts via `~/.config/unic/config.yaml` and provides drill-down exploration of AWS services registered in the catalog.

## Tech Stack

| Area | Technology | Version |
|------|-----------|---------|
| Language | Rust | Edition 2024 |
| TUI | ratatui + crossterm | 0.30 / 0.29 |
| CLI | clap (derive) | 4.5 |
| AWS | aws-sdk-ec2, aws-sdk-sts | 1.183 / 1.99 |
| Config | serde_yaml | 0.9 |
| Async | tokio | 1.49 |
| Error | anyhow | 1.0 |

## Architecture Diagram

```
┌─────────────────────────────────────────────────────┐
│                     main.rs                         │
│  CLI parsing (clap) → subcommand branch or TUI      │
└──────────┬──────────────────────┬───────────────────┘
           │                      │
    ┌──────▼──────┐        ┌──────▼──────┐
    │  CLI Mode   │        │  TUI Mode   │
    │  (context)  │        │  (ratatui)  │
    └──────┬──────┘        └──────┬──────┘
           │                      │
    ┌──────▼──────────────────────▼──────┐
    │              auth/                 │
    │  config.yaml → SSO or STS branch   │
    │  ┌─────────┐  ┌─────────┐         │
    │  │ sso.rs  │  │ sts.rs  │         │
    │  └─────────┘  └─────────┘         │
    └──────────────────┬────────────────┘
                       │
    ┌──────────────────▼────────────────┐
    │        app/ (state machine)       │
    │  Stack-based screen navigation    │
    │  ┌──────────────────────────┐     │
    │  │ ServiceList              │     │
    │  │  └─ FeatureList          │     │
    │  │      └─ VpcList          │     │
    │  │          └─ SubnetList   │     │
    │  │              └─ Result   │     │
    │  └──────────────────────────┘     │
    └──────────────────┬────────────────┘
                       │
    ┌──────────────────▼────────────────┐
    │       domain/ (pure models)       │
    │  catalog.rs: service/feature reg  │
    │  model.rs: AwsService, FeatureKind│
    └──────────────────┬────────────────┘
                       │
    ┌──────────────────▼────────────────┐
    │     services/aws/ (API calls)     │
    │  AwsRepository                    │
    │  ├─ vpc.rs   (VPC/Subnet/IP)      │
    │  ├─ rds.rs   (not implemented)    │
    │  ├─ iam.rs   (not implemented)    │
    │  ├─ ssm.rs   (not implemented)    │
    │  ├─ ipcalc.rs (CIDR calculation)  │
    │  └─ env.rs   (env var handling)   │
    └───────────────────────────────────┘
```

## Authentication Flow Details

### Config File Structure (`~/.config/unic/config.yaml`)

```yaml
version: 1
current: dev-sso        # Currently active context

defaults:
  region: us-east-1     # Default when context has no region

contexts:
  - name: dev-sso       # SSO-based
    profile: dev-sso

  - name: prod-admin    # STS AssumeRole-based
    profile: base-user
    role_arn: arn:aws:iam::111111111111:role/AdministratorAccess
    external_id: optional-id
```

### Authentication Branching Logic (`auth/mod.rs`)

```
Load config.yaml
    │
    ├─ role_arn present?
    │   ├─ YES → Prepare ~/.aws/credentials
    │   │        → If SSO profile, run aws sso login
    │   │        → Call STS AssumeRole
    │   │        → Generate session.env file
    │   │
    │   └─ NO → Is it an SSO profile?
    │       ├─ YES → Run aws sso login
    │       │        → Set profile env vars
    │       │
    │       └─ NO → Set profile env vars only
```

### SSO Profile Detection

A profile is identified as SSO if its section in `~/.aws/config` or `~/.aws/config.origin` contains a `sso_session` or `sso_start_url` key.

### Credentials File Management

- For STS: if credentials file is missing, restore from backup
- For SSO: move credentials to backup before SSO login
- On SSO failure: restore credentials from backup

## TUI Screen Structure

Screens are managed as a `Vec<Screen>` stack:

| Screen | Description | Data Source |
|--------|-------------|-------------|
| ServiceList | AWS service list | `catalog::list_services()` |
| FeatureList | Features for the selected service | `catalog::list_features()` |
| VpcList | VPC list | `AwsRepository::list_vpcs()` |
| SubnetList | Subnet list | `AwsRepository::list_subnets()` |
| ResultView | Scrollable result display | Per-feature API results |

### Key Bindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `g` / `Home` | Scroll to top |
| `G` / `End` | Scroll to bottom |
| `Enter` | Select (next screen) |
| `Backspace` / `Esc` / `←` | Previous screen |
| `r` | Refresh current screen |
| `q` | Quit |

## CLI Subcommands

```
unic                          # Enter TUI mode
unic --context dev-sso        # Enter TUI with a specific context
unic --profile my-profile     # Use a specific profile
unic --region ap-northeast-2  # Use a specific region

unic context list             # List configured contexts
unic context current          # Print current context
unic context use [name]       # Switch context (interactive selection if name omitted)
```

## Currently Implemented Features

| Service | Feature | Status |
|---------|---------|--------|
| VPC | RemainPrivateIP (subnet available IP query) | ✅ Implemented |
| RDS | ListDBInstances | 🚧 Coming Soon |
| Route53 | ListHostedZones | 🚧 Coming Soon |
| IAM | ListUsers | 🚧 Coming Soon |

## New Feature Addition Checklist

1. `src/domain/model.rs` → Add variant to `AwsService` / `FeatureKind` enums
2. `src/domain/catalog.rs` → Add mapping in `list_services()` / `list_features()`
3. `src/services/aws/` → Create new file, add `AwsRepository` impl
4. `src/services/aws/mod.rs` → Register the module
5. `src/app/actions.rs` → Add new `FeatureKind` branch in `enter()`
6. If needed: `src/app/types.rs` → Add new `Screen` enum variant
7. If needed: `Cargo.toml` → Add new AWS SDK crate
8. Write tests

## Build & Run

```bash
cargo build --release     # Release build
cargo run                 # Development run
cargo test                # Run tests
```

Docker builds are also supported (see `Dockerfile.build`).
