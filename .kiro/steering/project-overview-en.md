---
inclusion: auto
---

# UNIC Project Overview

UNIC is a Rust-based TUI (Terminal User Interface) tool for browsing and managing AWS resources via a CLI/TUI application.

## Tech Stack

- Language: Rust (Edition 2024)
- TUI Framework: ratatui 0.30 + crossterm 0.29
- CLI Parser: clap 4.5 (derive mode)
- AWS SDK: aws-sdk-ec2, aws-sdk-sts, aws-config, aws-credential-types
- Config: serde + serde_yaml (YAML-based)
- Async Runtime: tokio (macros, rt-multi-thread)
- Error Handling: anyhow
- Testing: tempfile (dev-dependency)

## Config File Location

`~/.config/unic/config.yaml`

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
    external_id: optional-external-id
```

## Authentication Flow

1. Load context from config.yaml
2. If `role_arn` is present, authenticate via STS AssumeRole
3. If `role_arn` is absent and the profile is an SSO profile, run `aws sso login`
4. Otherwise, use profile-based authentication

## Execution Flow

1. Parse CLI arguments → if a subcommand exists, handle it and exit
2. If no subcommand, enter TUI mode
3. In TUI, drill down: service catalog → feature selection → resource browsing

## Project Structure

```
src/
├── main.rs          # Entry point, CLI parsing + TUI loop
├── lib.rs           # config module re-export (for external crate use)
├── config/          # Load/save ~/.config/unic/config.yaml
├── cli/             # clap-based CLI definitions
├── auth/            # SSO/STS authentication logic
│   ├── mod.rs       # apply_context_side_effects (auth branching)
│   ├── sso.rs       # Run aws sso login
│   ├── sts.rs       # STS AssumeRole
│   ├── aws_files.rs # ~/.aws/config & credentials file management
│   └── session_env.rs # Env var setup + session.env file generation
├── domain/          # Business domain models
│   ├── catalog.rs   # Service/feature catalog definitions
│   └── model.rs     # AwsService, FeatureKind, ResourceItem, etc.
├── app/             # TUI application state management
│   ├── mod.rs       # App struct, initialization, key handling
│   ├── types.rs     # Screen, MenuState, ViewMode types
│   ├── actions.rs   # enter/refresh screen transition logic
│   └── navigation.rs # Cursor movement, scrolling
├── tui/             # ratatui rendering
│   └── tui.rs       # render function
└── services/        # AWS API call implementations
    └── aws/
        ├── repository.rs # AwsRepository (EC2 Client initialization)
        ├── vpc.rs        # VPC/Subnet/IP queries
        ├── rds.rs        # RDS (not yet implemented)
        ├── iam.rs        # IAM (not yet implemented)
        ├── ssm.rs        # SSM (not yet implemented)
        ├── env.rs        # Env var reading + debug lines
        ├── ipcalc.rs     # CIDR-based available IP calculation
        └── model.rs      # SubnetIpAvailability
```
