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
| AWS | aws-sdk-go-v2 (ec2, ssm, sts) | latest |
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
    │  │      ├─ InstanceList     │     │
    │  │      └─ VpcList          │     │
    │  │          └─ SubnetList   │     │
    │  │              └─ Detail   │     │
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
    │  ├─ repository.go (client init)   │
    │  ├─ ec2.go   (EC2 instances)      │
    │  ├─ ec2_model.go (EC2Instance)    │
    │  ├─ vpc.go   (VPC/Subnet/IP)      │
    │  ├─ vpc_model.go (VPC, Subnet)    │
    │  ├─ ssm.go   (session mgmt)       │
    │  └─ ssm_exec.go (plugin exec)     │
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
| ServiceList | AWS service list | `domain.Catalog()` |
| FeatureList | Features for the selected service | `domain.Catalog()` |
| InstanceList | SSM-eligible EC2 instances (with filter) | `AwsRepository.ListRunningInstances()` |
| VPCList | VPC list | `AwsRepository.ListVPCs()` |
| SubnetList | Subnets for selected VPC | `AwsRepository.ListSubnets()` |
| SubnetDetail | Available IPs in selected subnet | `AwsRepository.ListAvailableIPs()` |
| Loading | Loading indicator | — |
| Error | Error display | — |

### Key Bindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Enter` | Select (next screen) |
| `Esc` / `q` | Previous screen |
| `H` | Go to home (service list) |
| `/` | Toggle filter (instance list, IP list) |
| `q` (on service list) | Quit |

## CLI Subcommands

```
unic                          # Enter TUI mode
unic --profile my-profile     # Use a specific profile
unic --region ap-northeast-2  # Use a specific region

unic init                     # Create default config file
unic init --force             # Overwrite existing config file
```

## Currently Implemented Features

| Service | Feature | Status |
|---------|---------|--------|
| EC2 | SSM Session Manager (connect to EC2 instances) | ✅ Implemented |
| VPC | VPC Browser (VPCs → subnets → available IPs) | ✅ Implemented |
| RDS | RDS Browser (list, detail, start/stop/failover) | ✅ Implemented |
| Route53 | ListHostedZones | 🚧 Coming Soon |
| IAM | ListUsers | 🚧 Coming Soon |

## Planned Improvements

### M5 — UI Beautification (Charmbracelet Ecosystem)

- **File extraction**: Split `internal/app/app.go` (~1700 lines) into `styles.go`, `views.go`, `commands.go`, `filter.go`
- **bubbles components**: Add `bubbles/textinput` (filter input), `bubbles/spinner` (loading), `bubbles/table` (context picker)
- **Enhanced styles**: Bordered list views, full-width status bar, consistent label alignment, styled help bar
- Dependency: `github.com/charmbracelet/bubbles`

### M6 — Search/Filter for Long Lists

- **Fuzzy matching**: Replace `strings.Contains` with `sahilm/fuzzy` for scored fuzzy search
- **Match highlighting**: Bold + orange on matched characters in list items
- **Universal filter**: Add "/" filter to all list views (VPC list and subnet list currently missing)
- **Unified architecture**: `Filterable` interface + generic `applyFuzzyFilter[T]()` to eliminate per-screen duplication
- Dependency: `github.com/sahilm/fuzzy`

See `PLAN.md` for full milestone details and implementation order.

## New Feature Addition Checklist

1. `internal/domain/model.go` → Add `AwsService` and `FeatureKind` string constants
2. `internal/domain/catalog.go` → Add `Service` entry in `Catalog()`
3. `internal/services/aws/` → Create new file with `AwsRepository` method + model file
4. `internal/services/aws/repository.go` → Add client interface and field to `AwsRepository`
5. `internal/app/app.go` → Add new screen constant and feature handling in `Update()`
6. If needed: `go.mod` → Add new AWS SDK module via `go get`
7. Write tests with mock client

## Build & Run

```bash
go build -o unic ./cmd/unic   # Build
go run ./cmd/unic              # Development run
go test ./...                  # Run tests
```

Docker builds are also supported (see `Dockerfile.build`).
