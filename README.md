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

## Installation & Build

```bash
git clone <repository-url>
go build -o unic ./cmd/unic
```

## Usage

```bash
# Enter TUI mode
unic

# Specify profile/region
unic --profile my-profile
unic --region ap-northeast-2

# Initialize config file
unic init                      # Create default config
unic init --force              # Overwrite existing config
```

## Configuration

`~/.config/unic/config.yaml` (created via `unic init` or auto-generated on first run)

```yaml
# Simple format
default_profile: my-profile
default_region: ap-northeast-2
```

```yaml
# Context-based format
current: dev-sso

contexts:
  - name: dev-sso
    profile: dev-sso
    region: ap-northeast-2

  - name: prod-admin
    profile: prod-admin
    region: us-east-1
```

**Priority**: CLI flags (`--profile`, `--region`) > context settings > config defaults > hardcoded defaults (`us-east-1`)

## Currently Implemented Features

| Service | Feature | Status |
|---------|---------|--------|
| EC2 | SSM Session Manager (connect to EC2 instances) | ✅ Implemented |
| VPC | VPC Browser (VPCs → subnets → available IPs) | ✅ Implemented |
| RDS | RDS Browser (list, start/stop, failover, Aurora cluster support) | ✅ Implemented |
| Route53 | ListHostedZones | 🚧 Coming Soon |
| IAM | ListUsers | 🚧 Coming Soon |

## TUI Key Bindings

### Global

| Key | Action |
|-----|--------|
| `j`/`k` or `↑`/`↓` | Navigate |
| `Enter` | Select |
| `Esc`/`q` | Go back |
| `H` | Go to home (service list) |
| `C` | Context switcher |
| `q` (on service list) | Quit |

### EC2 (SSM Session)

| Key | Action |
|-----|--------|
| `/` | Filter instances |
| `r` | Refresh instance list |
| `Enter` | Connect to instance |

### RDS

| Key | Action |
|-----|--------|
| `s` | Start instance |
| `x` | Stop instance |
| `f` | Failover (Aurora) |

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
