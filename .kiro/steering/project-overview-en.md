---
inclusion: auto
---

# UNIC Project Overview

UNIC is a Go-based TUI (Terminal User Interface) tool for browsing and managing AWS resources via a CLI/TUI application.

## Tech Stack

- Language: Go (1.22+)
- TUI Framework: Bubbletea + Lipgloss + Bubbles (planned: bubbles/textinput, bubbles/spinner, bubbles/table)
- Search: sahilm/fuzzy (planned — M6 fuzzy search/filter)
- CLI Parser: Cobra
- AWS SDK: aws-sdk-go-v2 (ec2, rds, ssm, sts, sso, ssooidc)
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
│   ├── root.go              # Root command, global flags (--profile, --region)
│   └── init.go              # unic init subcommand
├── config/                  # Load/save ~/.config/unic/config.yaml
│   └── config.go
├── domain/                  # Business domain models
│   ├── model.go             # AwsService, FeatureKind, Service, Feature
│   └── catalog.go           # Service/feature catalog (Catalog())
├── app/                     # Bubbletea TUI application
│   └── app.go               # Root model, screens, navigation, rendering
└── services/                # AWS API call implementations
    └── aws/
        ├── repository.go    # AwsRepository (EC2/SSM client initialization)
        ├── ec2.go           # EC2 instance listing (SSM-managed)
        ├── ec2_model.go     # EC2Instance model
        ├── vpc.go           # VPC/Subnet/IP queries
        ├── vpc_model.go     # VPC, Subnet models
        ├── ssm.go           # SSM session start/terminate
        └── ssm_exec.go      # session-manager-plugin subprocess execution

.goreleaser.yaml             # Release configuration
go.mod
go.sum
Makefile
```

> **Note**: `internal/auth/` and `internal/tui/` are planned but not yet implemented.
> TUI screens, navigation, and styles are currently consolidated in `internal/app/app.go`.
