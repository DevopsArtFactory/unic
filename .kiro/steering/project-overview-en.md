---
inclusion: auto
---

# UNIC Project Overview

UNIC is a Go-based TUI (Terminal User Interface) tool for browsing and managing AWS resources via a CLI/TUI application.

## Tech Stack

- Language: Go (1.22+)
- TUI Framework: Bubbletea + Lipgloss + Bubbles
- CLI Parser: Cobra
- AWS SDK: aws-sdk-go-v2 (ec2, sts, config, credentials)
- Config: gopkg.in/yaml.v3 (YAML-based)
- Concurrency: goroutines + errgroup
- Error Handling: fmt.Errorf wrapping / standard errors package
- Testing: testing + t.TempDir()

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
cmd/
└── unic/
    └── main.go              # Entry point

internal/
├── cli/                     # Cobra-based CLI definitions
│   ├── root.go              # Root command, global flags
│   └── context.go           # Context subcommands
├── config/                  # Load/save ~/.config/unic/config.yaml
│   └── config.go
├── auth/                    # SSO/STS authentication logic
│   ├── auth.go              # ApplyContextSideEffects (auth branching)
│   ├── sso.go               # Run aws sso login
│   ├── sts.go               # STS AssumeRole
│   ├── aws_files.go         # ~/.aws/config & credentials file management
│   └── session_env.go       # Env var setup + session.env file generation
├── domain/                  # Business domain models
│   ├── catalog.go           # Service/feature catalog definitions
│   └── model.go             # AwsService, FeatureKind, ResourceItem, etc.
├── app/                     # Bubbletea TUI application
│   ├── app.go               # Root model, initialization, key handling
│   ├── screens.go           # Screen types and navigation stack
│   ├── actions.go           # Screen transition logic
│   └── navigation.go        # Cursor movement, scrolling
├── tui/                     # Reusable Bubbletea components
│   ├── components.go        # Filterable list, dialog, spinner, etc.
│   └── styles.go            # Lipgloss style definitions
└── services/                # AWS API call implementations
    └── aws/
        ├── repository.go    # AwsRepository (client initialization)
        ├── vpc.go           # VPC/Subnet/IP queries
        ├── rds.go           # RDS (not yet implemented)
        ├── iam.go           # IAM (not yet implemented)
        ├── ssm.go           # SSM (not yet implemented)
        ├── env.go           # Env var reading + debug lines
        ├── ipcalc.go        # CIDR-based available IP calculation
        └── model.go         # SubnetIpAvailability

go.mod
go.sum
Makefile
```
