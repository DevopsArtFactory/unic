# unic

`unic` is a Rust-based TUI (Terminal User Interface) tool for browsing and managing AWS resources in the terminal.

It manages authentication contexts (SSO or STS AssumeRole) via `~/.config/unic/config.yaml` and provides drill-down exploration of AWS services registered in the catalog.

## Tech Stack

- Rust (Edition 2024)
- TUI: ratatui 0.30 + crossterm 0.29
- CLI: clap 4.5 (derive)
- AWS SDK: aws-sdk-ec2, aws-sdk-sts
- Config: serde_yaml
- Async: tokio
- Error handling: anyhow

## Installation & Build

```bash
git clone <repository-url>
cargo build --release
```

## Usage

```bash
# Enter TUI mode
unic

# Specify context/profile/region
unic --context dev-sso
unic --profile my-profile
unic --region ap-northeast-2

# Context management
unic context list              # List contexts
unic context current           # Show current context
unic context use [name]        # Switch context
```

## Configuration

`~/.config/unic/config.yaml` (auto-generated on first run)

```yaml
version: 1
current: dev-sso

defaults:
  region: us-east-1

contexts:
  - name: dev-sso
    profile: dev-sso

  - name: prod-admin
    profile: base-user
    role_arn: arn:aws:iam::111111111111:role/AdministratorAccess
```

## Authentication Methods

| Method | Condition | Behavior |
|--------|-----------|----------|
| STS AssumeRole | `role_arn` is set | Issue temporary credentials via STS |
| SSO | Profile has `sso_session` | Run `aws sso login` |
| Profile | Otherwise | Set AWS profile env vars |

## Currently Implemented Features

| Service | Feature | Status |
|---------|---------|--------|
| VPC | RemainPrivateIP (subnet available IP query) | ✅ Implemented |
| RDS | ListDBInstances | 🚧 Coming Soon |
| Route53 | ListHostedZones | 🚧 Coming Soon |
| IAM | ListUsers | 🚧 Coming Soon |

## TUI Key Bindings

| Key | Action |
|-----|--------|
| `j`/`k` or `↑`/`↓` | Navigate |
| `Enter` | Select |
| `Backspace`/`Esc` | Go back |
| `r` | Refresh |
| `g`/`G` | Top/Bottom |
| `q` | Quit |

## Documentation

- [Architecture](.kiro/docs/architecture-en.md)

## License

This project is licensed under the terms in [LICENSE](LICENSE).

## Community Standards

- Code of Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Contributing Guide: [CONTRIBUTING.md](CONTRIBUTING.md)
- Security Policy: [SECURITY.md](SECURITY.md)

## Maintainers

- Add maintainers here.
