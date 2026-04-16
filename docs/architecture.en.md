# UNIC Architecture

## Overview

UNIC is a terminal-first AWS operations console built in Go.
The codebase is organized around four layers:

1. CLI entry and bootstrapping
2. Authentication and config resolution
3. Bubble Tea application state and screen transitions
4. AWS repository methods and service-specific models

## Runtime Flow

```text
cmd/unic/main.go
  -> internal/cli (flags, subcommands)
  -> internal/config.Load(...)
  -> internal/services/aws.NewAwsRepository(...)
  -> internal/app.New(...)
  -> Bubble Tea event loop
```

If a CLI subcommand is selected, the TUI is skipped.
If no subcommand is selected, `unic` starts the full-screen TUI.

## Main Modules

### `cmd/unic/`

- process entrypoint
- wires Cobra commands to runtime startup

### `internal/cli/`

Owns non-TUI commands:

- `unic init`
- `unic update`
- `unic env [context]`
- `unic context setup`
  - live incremental filtering for large context/account/role lists
  - interactive context ordering via `unic context order`
- `unic context unset`

### `internal/config/`

Owns `~/.config/unic/config.yaml` handling:

- legacy flat config support
- context-based config support
- current-context switching
- context upsert / unset helpers
- auth-type normalization

### `internal/auth/`

Owns shell export and interactive setup workflows:

- build shell exports for `credential`, `assume_role`, and concrete `sso` contexts
- include a `UNIC_CONTEXT` marker in shell exports and cleanup commands
- run interactive setup for SSO base contexts
- share SSO account / role resolution helpers across CLI and TUI flows
- discover available SSO accounts and roles
- return clipboard-friendly export strings

### `internal/domain/`

Pure catalog and enum-like definitions:

- AWS service names
- feature kinds
- service-to-feature registration

This layer should stay free of AWS SDK and UI logic.

### `internal/services/aws/`

Repository and per-service AWS integrations.
Current repository clients include:

- EC2
- SSM
- RDS
- Route53
- Secrets Manager
- IAM
- STS
- CloudWatch Logs
- ECS
- S3

Pattern:

- `repository.go` initializes SDK clients
- `inspector*.go` owns Security Inspector scan orchestration and finding models
- `*_model.go` defines UI-facing data models
- `*.go` implements service operations
- tests use mock client interfaces per AWS service

### `internal/app/`

Bubble Tea application state, navigation, and rendering.
The app remains centered on a root model, but screen-specific rendering is now split across dedicated files such as:

- `screen_ec2.go`
- `screen_vpc.go`
- `screen_rds.go`
- `screen_route53.go`
- `screen_securitygroup.go`
- `screen_iam.go`
- `screen_cloudwatchlogs.go`
- `screen_ecs.go`
- `screen_s3.go`
- `screen_inspector.go`
- `screen_context.go`

Supporting files include `styles.go`, `filter.go`, and `messages.go`.
`filter.go` and `filter_match.go` now centralize shared list filtering, fuzzy match ordering, and inline match highlighting across common list screens, including the VPC and subnet lists.

## Authentication Model

UNIC supports three main auth modes.

### `credential`

- uses shared AWS profile credentials
- `unic env` exports `AWS_PROFILE` and region variables
- TUI uses AWS SDK shared config loading

### `assume_role`

- starts from a base profile
- calls STS `AssumeRole`
- `unic env` exports temporary session credentials
- SDK clients are initialized with assumed-role credentials

### `sso`

Two shapes exist:

1. SSO base context
   - profile + start URL only
   - used by `unic context setup`
2. Concrete SSO context
   - includes `sso_account_id` and `sso_role_name`
   - can produce direct environment exports and SDK credentials

## TUI Screen Families

Current screen families include:

- service list
- feature list
- EC2 / SSM
- VPC / subnet / available IP detail
- RDS list, detail, and confirm flows
- Route53 zone, record, and mutation flows
- Secrets Manager list/detail
- Security Group list/detail/edit flows
- IAM user, key, and key-rotation flows
- CloudWatch Logs group/stream/viewer flows
- ECS cluster/service/task/container flows
- S3 bucket/object/detail flows
- Inspector home/scanning/results/detail flows
- context picker, context add, and TUI-native context setup/export/unset flows
- SSO account / role selection and exit notice flows
- loading and error screens

## Extension Pattern

When adding a feature:

1. add service or feature constants in `internal/domain/model.go`
2. register them in `internal/domain/catalog.go`
3. add repository methods and models under `internal/services/aws/`
4. wire the screen flow in `internal/app/`
5. add tests for repository logic and app transitions
6. update README and `docs/` if behavior is user-visible
