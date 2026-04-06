# unic

`unic` is a Go-based TUI (Terminal User Interface) tool for browsing and managing AWS resources in the terminal.

It manages authentication contexts (SSO or STS AssumeRole) via `~/.config/unic/config.yaml` and provides drill-down exploration of AWS services registered in the catalog.

## Tech Stack

- Go (1.22+)
- TUI: Bubbletea + Lipgloss + Bubbles
- CLI: Cobra
- AWS SDK: aws-sdk-go-v2
- Config: gopkg.in/yaml.v3
- Concurrency: goroutines + errgroup
- Error handling: fmt.Errorf / errors

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap DevopsArtFactory/unic
brew install unic
```

### Install Script (macOS/Linux)

```bash
curl -sSL https://raw.githubusercontent.com/DevopsArtFactory/unic/main/install.sh | sh
```

Set `INSTALL_DIR` to change the install location (default: `/usr/local/bin`).

### Build from Source

```bash
git clone https://github.com/DevopsArtFactory/unic.git
cd unic
make build
```

## Usage

```bash
# Enter TUI mode
unic

# Specify profile/region
unic --profile my-profile
unic --region ap-northeast-2

# Enable verbose debug logging (writes to ~/.config/unic/logs/unic.log)
unic --verbose
unic -v

# Initialize config file
unic init                      # Create default config
unic init --force              # Overwrite existing config

# Update to latest version
unic update                    # Auto-detects install method (brew vs binary)
```

## Configuration

`~/.config/unic/config.yaml` (created via `unic init` or auto-generated on first run)

### Legacy Format (Flat)

```yaml
default_profile: my-profile
default_region: ap-northeast-2
```

### Context-Based Format

```yaml
current: dev-sso

defaults:
  region: us-east-1

contexts:
  # SSO authentication
  - name: dev-sso
    region: ap-northeast-2
    auth_type: sso
    sso_start_url: https://my-sso-portal.awsapps.com/start
    sso_account_id: "123456789012"
    sso_role_name: DeveloperRole

  # Assume Role (cross-account)
  - name: prod-assume
    region: us-east-1
    auth_type: assume_role
    profile: base-profile
    role_arn: arn:aws:iam::987654321098:role/CrossAccountRole
    external_id: optional-external-id

  # Credential profile
  - name: staging-creds
    region: eu-west-1
    auth_type: credential
    profile: staging
```

### Auth Types

| Auth Type | Required Fields | Description |
|-----------|----------------|-------------|
| `sso` | `sso_start_url`, `sso_account_id`, `sso_role_name` | AWS SSO portal login with token caching |
| `credential` | `profile` | Uses `~/.aws/credentials` profile directly |
| `assume_role` | `profile`, `role_arn` | Assumes a cross-account role from a base profile |

**Priority**: CLI flags (`--profile`, `--region`) > context settings > config defaults > hardcoded defaults (`us-east-1`)

## Currently Implemented Features

| Service | Feature | Status |
|---------|---------|--------|
| EC2 | SSM Session Manager (connect to running, SSM-managed instances) | ✅ Implemented |
| EC2 | Security Group Browser (list/filter SGs, view rules, add/delete rules with confirmation) | ✅ Implemented |
| VPC | VPC Browser (VPCs → Subnets → Available IPs with reserved-IP exclusion) | ✅ Implemented |
| RDS | RDS Browser (list, start/stop, failover, Aurora cluster support, auto-polling) | ✅ Implemented |
| Route53 | DNS Browser (Hosted Zones → Records → Record Detail, create/edit/delete A/CNAME, change status polling) | ✅ Implemented |
| CloudWatch Logs | Log Browser (Log Groups → Streams → Events, live tail with 2s poll, time range presets, filter patterns, log level highlighting) | ✅ Implemented |
| Secrets Manager | Secrets Browser (list secrets, view key-value pairs or raw values) | ✅ Implemented |
| IAM | Access Key Browser (list keys with status, age, last used) | ✅ Implemented |
| IAM | Access Key Rotation (create → verify/apply → deactivate → delete) | ✅ Implemented |

## TUI Key Bindings

### Global

| Key | Action |
|-----|--------|
| `j`/`k` or `↑`/`↓` | Navigate list |
| `Enter` | Select item |
| `Esc` | Go back one screen |
| `q` | Quit (on service list) |
| `H` | Jump to home (service list) |
| `C` | Open context switcher |
| `/` | Toggle filter mode |
| `Ctrl+C` | Force quit |

### EC2 SSM Session

| Key | Action |
|-----|--------|
| `r` | Refresh instance list |
| `Enter` | Connect to instance |

### Security Groups

| Key | Action | Screen |
|-----|--------|--------|
| `/` | Filter security groups | List |
| `r` | Refresh list | List |
| `Enter` | View inbound/outbound rules | List |
| `Tab` | Switch between Inbound/Outbound section | Detail |
| `j`/`k` or `↑`/`↓` | Navigate rules | Detail |
| `a` | Add rule (multi-step form) | Detail |
| `d` | Delete selected rule (type-to-confirm) | Detail |

### RDS Detail

| Key | Action | Condition |
|-----|--------|-----------|
| `s` | Start database | Instance/cluster is stopped |
| `x` | Stop database | Instance/cluster is available |
| `f` | Failover database | Multi-AZ standalone or Aurora cluster |
| `r` | Refresh status | Always |

### IAM Access Key Rotation

| Key | Action | Screen |
|-----|--------|--------|
| `r` | Rotate access key | Key detail (RotateAccessKey mode) |
| `c` | Copy new key as export commands | Rotation result |
| `a` | Apply new key to ~/.aws/credentials and verify | Rotation result |
| `d` | Deactivate old key | Rotation result |
| `x` | Delete old inactive key | Rotation result |

### CloudWatch Logs

| Key | Action | Screen |
|-----|--------|--------|
| `/` | Filter log groups/streams | List |
| `Enter` | Drill into streams/view logs | List |
| `1`-`6` | Time range preset (5m/15m/1h/6h/24h/7d) | Viewer |
| `t` | Toggle live tail (2s poll) | Viewer |
| `f` | Enter filter pattern | Viewer |
| `n` | Load more (older events) | Viewer |
| `PgUp`/`PgDn` | Page scroll | Viewer |

### Context Switcher

| Key | Action |
|-----|--------|
| `Enter` | Switch to selected context |
| `a` | Add new context (wizard) |
| `/` | Filter contexts |
| `Esc` | Back |

### Filtering

Available on: EC2 instances, VPC/Subnets, RDS instances, Route53 zones/records, CloudWatch Log Groups/Streams, Secrets Manager, Context Switcher. Press `/` to enter filter mode, type to search, `Esc` or `Enter` to exit filter mode.

### IAM Access Key Rotation

| Key | Action | Screen |
|-----|--------|--------|
| `r` | Rotate access key | Key detail |
| `c` | Copy new key as export commands | Rotation result |
| `a` | Apply new key to ~/.aws/credentials | Rotation result |
| `d` | Deactivate old key | Rotation result |
| `x` | Delete old inactive key | Rotation result |

## Documentation

- [Architecture](.kiro/docs/architecture-en.md)

## License

This project is licensed under the terms in [LICENSE](LICENSE).

## Issue Bot

Comment on any issue to interact with `@unic-bot`:

| Command | Action |
|---------|--------|
| `@unic-bot: assign me` | Assign the issue to yourself |

## Community Standards

- Code of Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Contributing Guide: [CONTRIBUTING.md](CONTRIBUTING.md)
- Security Policy: [SECURITY.md](SECURITY.md)

## Maintainers

- Add maintainers here.
