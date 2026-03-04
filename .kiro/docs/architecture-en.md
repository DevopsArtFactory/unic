# UNIC Architecture Document

## Overview

UNIC (Unified Infrastructure Console) is a Go TUI tool for browsing AWS resources in the terminal.
It manages authentication contexts via `~/.config/unic/config.yaml` and provides drill-down exploration of AWS services registered in the catalog.

## Tech Stack

| Area | Technology | Version |
|------|-----------|---------|
| Language | Go | 1.22+ |
| TUI | Bubbletea + Lipgloss + Bubbles | latest |
| CLI | Cobra | latest |
| AWS | aws-sdk-go-v2 (ec2, sts) | latest |
| Config | gopkg.in/yaml.v3 | 0.9 |
| Concurrency | goroutines + errgroup | stdlib |
| Error | fmt.Errorf / errors | stdlib |

## Architecture Diagram

```
┌─────────────────────────────────────────────────────┐
│                   cmd/unic/main.go                  │
│  CLI parsing (Cobra) → subcommand branch or TUI     │
└──────────┬──────────────────────┬───────────────────┘
           │                      │
    ┌──────▼──────┐        ┌──────▼──────┐
    │  CLI Mode   │        │  TUI Mode   │
    │  (context)  │        │ (bubbletea) │
    └──────┬──────┘        └──────┬──────┘
           │                      │
    ┌──────▼──────────────────────▼──────┐
    │           internal/auth/           │
    │  config.yaml → SSO or STS branch   │
    │  ┌─────────┐  ┌─────────┐         │
    │  │ sso.go  │  │ sts.go  │         │
    │  └─────────┘  └─────────┘         │
    └──────────────────┬────────────────┘
                       │
    ┌──────────────────▼────────────────┐
    │    internal/app/ (Bubbletea)      │
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
    │   internal/domain/ (pure models)  │
    │  catalog.go: service/feature reg  │
    │  model.go: AwsService, FeatureKind│
    └──────────────────┬────────────────┘
                       │
    ┌──────────────────▼────────────────┐
    │  internal/services/aws/ (API)     │
    │  AwsRepository                    │
    │  ├─ vpc.go   (VPC/Subnet/IP)      │
    │  ├─ rds.go   (not implemented)    │
    │  ├─ iam.go   (not implemented)    │
    │  ├─ ssm.go   (not implemented)    │
    │  ├─ ipcalc.go (CIDR calculation)  │
    │  └─ env.go   (env var handling)   │
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

### Authentication Branching Logic (`internal/auth/auth.go`)

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

Screens are managed as a stack of Bubbletea `Model` implementations:

| Screen | Description | Data Source |
|--------|-------------|-------------|
| ServiceList | AWS service list | `catalog.ListServices()` |
| FeatureList | Features for the selected service | `catalog.ListFeatures()` |
| VpcList | VPC list | `AwsRepository.ListVpcs()` |
| SubnetList | Subnet list | `AwsRepository.ListSubnets()` |
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

1. `internal/domain/model.go` → Add constant to `AwsService` / `FeatureKind` types
2. `internal/domain/catalog.go` → Add mapping in `ListServices()` / `ListFeatures()`
3. `internal/services/aws/` → Create new file, add `AwsRepository` method
4. `internal/app/actions.go` → Add new `FeatureKind` branch in screen transition
5. If needed: `internal/app/screens.go` → Add new screen model
6. If needed: `go.mod` → Add new AWS SDK module via `go get`
7. Write tests

## Build & Run

```bash
go build -o unic ./cmd/unic   # Build
go run ./cmd/unic              # Development run
go test ./...                  # Run tests
```

Docker builds are also supported (see `Dockerfile.build`).
