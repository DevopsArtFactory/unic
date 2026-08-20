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
  - can trigger `aws login` for `console_login` contexts
- `unic context sync [base-context]`
  - generates a sync-managed context for every account/role pair visible to an SSO base context
  - generated contexts carry a `sync_source` marker to stay distinguishable from manual ones
  - sync-managed contexts whose account/role disappeared are reported as orphans and removed only with `--prune`
  - `--dry-run` prints the plan without writing config
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
- build profile-based exports for `console_login` contexts after `aws login`
- include a `UNIC_CONTEXT` marker in shell exports and cleanup commands
- run interactive setup for SSO base contexts
- run `aws login` for standalone `console_login` contexts
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
- CloudWatch Metrics
- STS
- CloudWatch Logs
- ECS
- ECR
- FIS
- ElastiCache
- ACM
- S3
- KMS

Pattern:

- `repository.go` initializes SDK clients
- `*_model.go` defines UI-facing data models
- `*.go` implements service operations
- tests use mock client interfaces per AWS service

### `internal/inspector/`

Cross-service inspector workflows and rule packs.

Pattern:

- workflow-local finding/report models live here
- Security Inspector scan orchestration and rule registration live here
- ACM expiry scanning uses the repository certificate list and the configured positive `inspector.acm_expiry_window_days` threshold (30 days by default)
- Checklist Inspector YAML schema loading, checklist result models, and readiness runners live here
- rule packs still depend on `internal/services/aws` repository methods and client interfaces rather than raw SDK setup
- the cost/waste rule pack reuses the EC2 and ELBv2 client interfaces for resource-state, tag, snapshot-age, and target-registration checks
- this package remains the growth path for future inspector workflows beyond Security and Checklist Inspector

### `internal/app/`

Bubble Tea application state, navigation, and rendering.
The app remains centered on a root model, but AWS feature browsers now sit behind submodel contracts so the root model can stay focused on boot, global navigation, shared chrome, and screen switching.

Current direction:

- `app.go` keeps the root model and global event loop
- feature submodels handle feature-local state, message handling, key handling, and view rendering
- app-shell flows such as service selection, feature selection, context selection, SSM session picking, loading, and error handling remain root-owned until a separate shell-flow abstraction is chosen

Ownership boundary:

- App-shell flows are root-owned when they select global application context, choose a feature, coordinate shared chrome, launch a subprocess/session, or represent cross-feature transition state.
- Feature submodels own AWS resource browser flows that start from a feature-list choice and can keep their state, messages, filters, key handling, commands, and views feature-local.
- Context picker/add/SSO flows remain app-shell because they mutate or choose the AWS context used by all features.
- The EC2 SSM session picker remains app-shell because it launches an external session workflow rather than browsing a feature-local resource graph.
- Loading, error, exit, service list, feature list, global help, home navigation, and context switching remain root-owned shared infrastructure.

Screen-specific rendering still lives in dedicated files such as:

- `screen_ec2.go`
- `screen_ec2_browser.go`
- `screen_vpc.go`
- `screen_rds.go`
- `screen_route53.go`
- `screen_securitygroup.go`
- `screen_iam.go`
- `screen_cloudwatchmetrics.go`
- `screen_cloudwatchlogs.go`
- `screen_ecs.go`
- `screen_eks.go`
- `screen_ecr.go`
- `screen_elasticache.go`
- `screen_acm.go`
- `screen_s3.go`
- `screen_lambda.go`
- `screen_bedrock.go`
- `screen_secrets.go`
- `screen_kms.go`
- `screen_inspector.go`
- `screen_context.go`

Supporting files include `styles.go`, `filter.go`, and `messages.go`.
`filter.go` and `filter_match.go` now centralize shared list filtering, fuzzy match ordering, and inline match highlighting across common list screens, including the VPC and subnet lists. When a shared filter is active, arrow-key navigation still flows through to the current list selection so users can move through filtered results without closing filter mode first.

## Authentication Model

UNIC supports five main auth modes.

### `credential`

- uses shared AWS profile credentials
- `unic env` exports `AWS_PROFILE` and region variables
- TUI uses AWS SDK shared config loading

### `console_login`

- uses a shared AWS profile plus AWS CLI `aws login`
- `unic context setup` runs `aws login --profile <profile> --region <region>`
- remains profile-backed after login, so `unic env` exports `AWS_PROFILE` and region variables
- with `role_arn` set, the login profile credentials chain into STS AssumeRole through the same path as assume_role (including the MFA session cache)

### `assume_role`

- starts from a base profile
- calls STS `AssumeRole`
- `unic env` exports temporary session credentials
- SDK clients are initialized with assumed-role credentials
- with `mfa_serial` set, CLI flows (`unic env`, `unic context setup`) prompt for a token code on stderr and cache the session credentials under `~/.config/unic/cache/assume-role/` until expiry
- the TUI passively reuses a valid cached session and otherwise fails with a pointer to `unic env <context>`

### `okta_saml`

- SAML federation through an Okta AWS app
- `unic env` runs Okta primary authentication and, when required, an MFA challenge (v1: TOTP and Okta Verify push)
- the SAML assertion is fetched from the app embed link and exchanged via `sts:AssumeRoleWithSAML`
- role selection is deterministic: `role_arn` wins, a single role is auto-selected, multiple roles error explicitly
- only the exchanged AWS session credentials are cached under `~/.config/unic/cache/okta-saml/`; passwords, one-time codes, and Okta session tokens are never persisted
- the TUI passively reuses a valid cached session and otherwise points at `unic env <context>`

### `sso`

Two shapes exist:

1. SSO base context
   - profile + start URL only
   - used by `unic context setup`
2. Concrete SSO context
   - includes `sso_account_id` and `sso_role_name`
   - can produce direct environment exports and SDK credentials

Contexts can use the structured `auth` and `resources` sections to keep identity independent from resource location. `auth.sso_region` is used for SSO login and `GetRoleCredentials`; `resources.default_region` is the initial resource region, and `resources.regions` defines the regions available to the runtime picker. Switching regions reuses the credentials provider and only recreates regional SDK clients. The EC2 Instance Browser can additionally toggle an all-regions scope with `A`, merging instances from every configured region into one list while reporting per-region query failures inline without hiding other regions' results.

`unic context setup` also treats the active resource region as session state. After any required SSO account and role selection, multi-region contexts prompt for a resource region and export it through `AWS_REGION` and `AWS_DEFAULT_REGION`. The persisted default region is unchanged, and single-region contexts skip the picker.

Legacy flat fields remain supported. `region` maps to `resources.default_region`, `regions` maps to the selectable resource regions, and `sso_region` maps to `auth.sso_region`. A missing region list produces the previous single-region behavior.

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
- CloudWatch metric list/detail flows
- CloudWatch Logs group/stream/viewer flows
- ECS cluster/service/rollout detail/task/container flows
- EKS cluster/node group/add-on status, upgrade readiness, and access helper flows
- ECR repository/image/detail flows
- FIS experiment template list/detail, safe-run preview, and experiment history/detail flows
- ElastiCache replication-group/cluster list, node list, and node detail flows
- ACM certificate list/detail flows
- S3 bucket/object/detail flows
- KMS key list/detail and rotation-posture flows
- Inspector mode home, checklist setup, security findings/detail (including KMS rotation findings), and checklist results/detail flows
- context picker, context add, and TUI-native context setup/export/unset flows
- SSO account / role selection and exit notice flows
- loading and error screens

## Extension Pattern

When adding a feature:

1. add service or feature constants in `internal/domain/model.go`
2. register them in `internal/domain/catalog.go`
3. add repository methods and models under `internal/services/aws/`
4. for cross-service inspector work, add workflow/rule logic under `internal/inspector/`
5. wire normal AWS feature browsers as feature submodels in `internal/app/`
6. keep app-shell flows root-owned unless a separate shell abstraction is introduced
7. add tests for repository logic and app transitions
8. update README and `docs/` if behavior is user-visible
