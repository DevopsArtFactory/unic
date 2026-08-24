# unic

`unic` is a Go-based TUI for browsing and operating AWS resources from the terminal.
It combines a Bubble Tea application, Cobra-based CLI commands, and AWS SDK v2 clients behind a context-aware authentication layer.

## What It Does

- Browse AWS services from a single terminal UI
- Switch between credential, assume-role, SSO, and Okta SAML contexts
- Export shell environment variables for the active context
- Drill down into resources with filters, detail views, and action screens
- Open a context-aware keyboard shortcut help screen with `?`
- Show animated loading indicators while async AWS data is being fetched
- Perform operational workflows such as EC2 inventory inspection, SSM sessions, RDS control, Route53 record changes, ECS rollout inspection/exec, EKS cluster and node group review, IAM access key rotation, and Bedrock API key management
- Press `i` from the service picker to enter Inspector mode, then run either the Security Inspector workflow for built-in security and cost/waste findings or the Checklist Inspector workflow for YAML-driven readiness checks across databases, network resources, DNS, logging, secrets, and baseline posture. Checklist files can be loaded from the in-TUI picker or preloaded with `--checklist <path>`

## Documentation Map

This is the closest structure to a "harness-like" doc hub in this repository: one entry document with focused reference docs behind it.

- [Docs Hub](docs/README.md)
- [Documentation Harness](docs/documentation-harness.md)
- [Architecture (EN)](docs/architecture.en.md)
- [Architecture (KO)](docs/architecture.ko.md)
- [Project Overview (EN)](docs/project-overview.en.md)
- [Project Overview (KO)](docs/project-overview.ko.md)
- [Development Guide](docs/development.md)
- [Branch Naming Harness](docs/branch-naming-harness.md)

## Tech Stack

- Go 1.22+
- TUI: Bubble Tea, Bubbles, Lip Gloss
- CLI: Cobra
- AWS: aws-sdk-go-v2
- Config: gopkg.in/yaml.v3
- Logging: structured file logging under `~/.config/unic/logs/`
- Testing: Go `testing`, table-driven tests, mocked AWS clients

## Installation

### Homebrew

```bash
brew tap DevopsArtFactory/unic
brew install unic
```

### Install Script

```bash
curl -sSL https://raw.githubusercontent.com/DevopsArtFactory/unic/main/install.sh | sh
```

Set `INSTALL_DIR` to override the default install path.

### Build From Source

```bash
git clone https://github.com/DevopsArtFactory/unic.git
cd unic
make build
```

## CLI Usage

### Run the TUI

```bash
unic
unic --profile my-profile
unic --region ap-northeast-2
unic --checklist ./checklists/readiness.yaml   # optional: pre-load a checklist at startup
unic --verbose
```

The TUI shows a short retro bootup splash after a new install or version update, then records the version so later launches go straight to the context picker. Press `Enter`, `Esc`, or `Space` to skip it. Press `S` to open Settings, where the splash can be toggled on or off for every launch.

### Config/bootstrap

```bash
unic init
unic init --force
unic update
```

### Shell environment helpers

```bash
# Print exports for current context
unic env

# Print exports for a named context
eval "$(unic env prod-admin)"

# Interactively choose/setup a context and copy exports to clipboard
unic context setup

# Print an ECR login command for the current context (scripting helper;
# the TUI's ECR Login Helper is the primary workflow)
unic ecr login

# Copy a Podman ECR login command to the clipboard
unic ecr login --runtime podman --copy

# Set a display order for a context
unic context order prod-admin 10

# Or open reorder mode, then move the selected context with arrow keys and save
unic context order

# Clear current context and copy cleanup commands to clipboard
unic context unset

# Generate contexts from the accounts/roles visible to an SSO base context
unic context sync
unic context sync dev-sso --dry-run
unic context sync dev-sso --prune
```

`unic context setup` writes its prompts to `stderr` and copies the generated shell commands to the clipboard.
`unic env` prints shell commands to `stdout` so it can be used with `eval`.
`unic ecr login` prints a machine-usable container registry login command to `stdout` and can optionally copy it to the clipboard with `--copy`.
Both flows now include a `UNIC_CONTEXT` marker in the generated exports so the TUI can show which shell context is currently active.
Contexts can be prioritized in the setup picker with an `order` field in config.
In the CLI `unic context setup` flow, the picker filters contexts, SSO accounts, SSO roles, and configured resource regions as you type, with arrow-key navigation and Enter to confirm. Multi-region contexts prompt for the shell session region after account/role selection; single-region contexts skip that step. The selection changes `AWS_REGION` and `AWS_DEFAULT_REGION` in the generated exports without modifying the context's persisted default region.
Use `unic context order` to open reorder mode, choose a context with `↑/↓` or `j/k`, press `Enter` to start moving it, then press `Enter` again to save. `unic context order <name> <number>` still works for direct updates.
`unic context sync [base-context]` lists the AWS accounts and roles visible to an SSO base context and adds a sync-managed concrete context for each new account/role pair, inheriting the base context's regions. When only one SSO base context exists the argument can be omitted. Existing contexts are never rewritten: pairs that already have a context (manual or synced) are kept as-is. Synced contexts carry a `sync_source: <base-context>` marker in `config.yaml`; when their account/role disappears from SSO they are reported as orphans and removed only with `--prune`. Use `--dry-run` to preview the plan without writing config.

## Configuration

Primary config path:

```text
~/.config/unic/config.yaml
```

### Legacy Flat Format

```yaml
default_profile: my-profile
default_region: ap-northeast-2
```

### Context-Based Format

```yaml
current: dev-sso

defaults:
  region: ap-northeast-2

favorites:
  services:
    - Bedrock
    - ECS
  contexts:
    - prod-admin

views:
  - name: incident-rds
    context: prod-admin
    service: RDS
    feature: RDS Browser
    filter: prod-db

ui:
  boot_splash: false
  last_boot_splash_version: 0.1.3

contexts:
  - name: dev-sso
    order: 10
    profile: my-sso-profile
    region: ap-northeast-2
    auth_type: sso
    sso_start_url: https://example.awsapps.com/start

  - name: dev-sso-123456789012-developerrole
    profile: my-sso-profile
    region: ap-northeast-2
    auth_type: sso
    sso_start_url: https://example.awsapps.com/start
    sso_account_id: "123456789012"
    sso_role_name: DeveloperRole

  # Preferred structured format: one identity, several resource regions
  - name: production
    auth:
      type: sso
      sso_region: us-east-1
      sso_start_url: https://example.awsapps.com/start
      sso_account_id: "123456789012"
      sso_role_name: Admin
    resources:
      default_region: ap-northeast-2
      regions:
        - us-east-1
        - eu-west-1

  - name: prod-admin
    order: 20
    profile: base-profile
    region: us-east-1
    auth_type: assume_role
    role_arn: arn:aws:iam::123456789012:role/Admin
    external_id: optional-external-id
    mfa_serial: arn:aws:iam::123456789012:mfa/my-user   # optional: MFA-protected roles

  - name: local-dev
    profile: local-dev
    region: ap-northeast-2
    auth_type: console_login

  - name: staging
    profile: staging
    region: eu-west-1
    auth_type: credential

  - name: okta-prod
    region: ap-northeast-2
    auth_type: okta_saml
    okta_org_url: https://acme.okta.com
    okta_app_id: amazon_aws/0oa1b2c3d4e5f6g7h8i9/272
    role_arn: arn:aws:iam::123456789012:role/OktaAdmin   # required when the assertion carries multiple roles
```

### Auth Types

| Auth Type | Meaning | Required Fields |
|---|---|---|
| `credential` | Use shared AWS profile credentials, optionally chaining into a role | `profile`; optional `role_arn` (+ `external_id`, `mfa_serial`) |
| `console_login` | Run `aws login` during `unic context setup`, then use the resulting profile-backed console credentials, optionally chaining into a role | `profile`; optional `role_arn` (+ `external_id`, `mfa_serial`) |
| `assume_role` | Assume a role from a base profile, optionally with MFA | `profile`, `role_arn`; optional `mfa_serial` |
| `sso` | Use AWS IAM Identity Center / SSO, reusing a valid AWS CLI SSO cache and prompting for login only when needed | `sso_start_url`, and for concrete contexts `sso_account_id`, `sso_role_name`; `profile` is optional |
| `okta_saml` | Okta SAML federation: `unic env` signs in to Okta (prompt on stderr, password without echo; `UNIC_OKTA_USERNAME`/`UNIC_OKTA_PASSWORD` for automation), exchanges the SAML assertion via `sts:AssumeRoleWithSAML`, and caches the session under `~/.config/unic/cache/okta-saml/`. The TUI reuses a valid cached session passively. Okta MFA challenges support TOTP codes and Okta Verify push in v1 | `okta_org_url`, `okta_app_id`; `role_arn` required when the assertion carries multiple roles. Passwords and MFA secrets are never stored |

The preferred context format separates `auth` from `resources`. `auth.sso_region` controls IAM Identity Center login and role-credential retrieval. `resources.default_region` is selected at startup, and `resources.regions` lists additional regions available from the global `R` region picker. Switching regions reuses the current credentials and recreates only the regional AWS clients. The EC2 Instance Browser, RDS Browser, and Lambda Browser can additionally aggregate all configured regions into a single list with `A`; rows gain a region tag, per-region failures render inline, and actions (RDS start/stop/modify, Lambda invoke) run against the row's own region.

Legacy flat fields (`auth_type`, `profile`, `region`, `regions`, `sso_region`, and related auth fields) remain supported. A context without a region list behaves as a single-region context, and an omitted SSO region still falls back to its default resource region.

TUI startup is passive for SSO contexts: it loads the context picker without launching `aws sso login`. SSO login is prompted when you explicitly select or set up an SSO context, or when an AWS-backed workflow needs credentials.

### Okta SAML Contexts

`okta_saml` contexts federate into AWS through an Okta AWS app:

```bash
# Sign in to Okta and export session credentials
eval "$(unic env okta-prod)"

# Non-interactive (CI or scripts)
UNIC_OKTA_USERNAME=user@acme.com UNIC_OKTA_PASSWORD=... unic env okta-prod
```

Runtime flow: Okta primary authentication → optional MFA challenge → SAML assertion from the app embed link → `sts:AssumeRoleWithSAML` → session credentials cached under `~/.config/unic/cache/okta-saml/` until expiry. The TUI reuses a valid cached session passively and otherwise asks you to run `unic env <context>` first — it never prompts for Okta credentials itself.

`okta_app_id` is the app-specific part of the Okta app embed link: for `https://acme.okta.com/home/amazon_aws/0oa1b2c3d4e5f6g7h8i9/272`, use `amazon_aws/0oa1b2c3d4e5f6g7h8i9/272`.

v1 limitations:

- MFA factors: TOTP (`token:software:totp`) and Okta Verify push only. TOTP is preferred when both are enrolled; push polls for approval with a 60s deadline. Other factors fail with a list of what was found.
- Role selection is deterministic: `role_arn` wins, a single assertion role is auto-selected, and multiple roles without `role_arn` produce an explicit error listing the ARNs.
- Passwords, one-time codes, and Okta session tokens are never persisted; only the exchanged AWS session credentials are cached (0600 files in a 0700 directory).

Optional context fields:

| Field | Meaning |
|---|---|
| `order` | Lower values appear first in the context setup picker. Contexts without `order` fall back after ordered entries in their existing file order. |
| `sync_source` | Name of the SSO base context that generated this context via `unic context sync`. Marks the context as sync-managed: re-syncs may prune it (with `--prune`) when its account/role disappears from SSO. Contexts without this field are never touched by sync. |
| `sso_region` | (SSO only) Region of the IAM Identity Center portal, used for SSO login and role-credential retrieval. Defaults to `region` when unset. Use it when the SSO portal and your resources live in different regions. |
| `mfa_serial` | (assume_role only) ARN of the MFA device required by the role's trust policy. CLI flows (`unic env`, `unic context setup`) prompt for a token code on stderr and cache the resulting session under `~/.config/unic/cache/assume-role/` until it expires. The TUI reuses a valid cached session passively and otherwise asks you to run `unic env <context>` first. |
| `resources.regions` / `regions` | Additional resource regions available through the global `R` picker. The default resource region is always included automatically. |

Resolution priority:

```text
CLI flags > selected context > config defaults > hardcoded default (us-east-1)
```

Context ordering:

- Lower `order` values appear first
- Contexts without `order` appear after ordered contexts
- Contexts with the same `order` keep their file order

## Current Features

### AWS Service Catalog

| Service | Feature |
|---|---|
| EC2 | SSM Session Manager |
| EC2 | Instance Browser |
| EC2 | Security Group Browser |
| EC2 | Auto Scaling Group Browser |
| VPC | VPC Browser |
| VPC | Reachability Analyzer |
| RDS | RDS Browser |
| RDS | Instance Class Modification |
| CloudFormation | Stack Browser |
| Route53 | Route53 Browser |
| Secrets Manager | Secrets Browser |
| CloudTrail | Event Lookup |
| CloudWatch | Alarm Browser |
| CloudWatch | Metrics Viewer |
| CloudWatch Logs | Logs Browser |
| ECR | ECR Login Helper |
| ECR | Repository Browser |
| ECS | ECS Browser & Exec |
| EKS | Cluster & Node Group Browser |
| FIS | Experiment Template Browser |
| ElastiCache | Cluster & Replication Group Browser |
| SQS | Queue Browser |
| ELB | Load Balancer Browser |
| SSM Parameter Store | Parameter Browser |
| KMS | Key Browser |
| ACM | Certificate Browser |
| Step Functions | Execution Browser |
| S3 | S3 Browser |
| Lambda | Lambda Browser |
| Bedrock | API Key Manager |
| IAM | IAM User Browser |
| IAM | ListAccessKeys |
| IAM | RotateAccessKey |

### Inspector Mode

| Workflow | Status | Notes |
|---|---|---|
| Security Inspector | Ready | Runs built-in security and cost/waste rule packs and opens severity-filtered findings |
| Checklist Inspector | Ready | Runs a YAML checklist and reports pass/fail per check with resource context and mismatch details |

Security Inspector ships built-in rule packs for Security Group exposure, RDS encryption/public access/backups and public snapshot sharing, IAM access key age/root-account hardening/wildcard policies, Secrets Manager rotation age, KMS customer-key rotation, S3 public access/Block Public Access/versioning, CloudTrail baseline coverage, GuardDuty and AWS Config baseline controls, ElastiCache for Valkey encryption/backup/access-control checks, and cost/waste checks for unattached EIPs and EBS volumes, stopped EC2 instances, empty target groups, untagged EC2-family resources, and EBS snapshots aged 90 days or more.

Checklist Inspector can load a YAML file either from the Inspector-mode file picker or from `--checklist` at startup. Press `a` on the checklist results screen to add a check through type-specific prompts instead of editing YAML: pick one of the twelve rule types, fill the prompted fields (empty skips optional expectations), and the check is appended to the loaded checklist file — or a new `unic-checklist.yaml` when none is loaded — validated through the same `LoadChecklist` rules before anything is written, then the checklist reruns so the new result shows immediately. Currently supported types:

- `rds` for expected DB instance state such as status, engine, class, Multi-AZ, encryption, public access, and backup retention
- `security_group` for required or forbidden ingress/egress rule matchers
- `secret` for rotation state, KMS key ID, and required JSON value keys
- `hosted_zone` for hosted zone existence and private/public scope checks
- `route53_record` for DNS record existence, type, TTL, values, and alias target checks within `expect.zone`
- `vpc` for VPC existence, CIDR, default-VPC posture, and subnet-count checks
- `subnet` for subnet existence, optional `expect.vpc` scoping, CIDR, availability zone, and minimum available-IP checks
- `cloudwatch_log_group` for log-group existence and retention-days checks
- `cloudtrail_baseline`, `guardduty_baseline`, `config_baseline`, and `elasticache_valkey_baseline` for checklist-driven pass/fail wrappers around the built-in baseline security scanners

Minimal checklist example:

```yaml
name: Production Readiness
checks:
  - type: rds
    resource: prod-db
    expect:
      publicly_accessible: false
      storage_encrypted: true
      backup_retention_days: 7

  - type: security_group
    resource: sg-web
    expect:
      ingress_absent:
        - protocol: tcp
          from_port: 22
          to_port: 22
          cidr: 0.0.0.0/0

  - type: secret
    resource: prod/app
    expect:
      rotation_enabled: true
      value_keys:
        - username
        - password

  - type: route53_record
    resource: api.example.internal
    expect:
      zone: example.internal
      record_type: A
      alias_target: internal-alb-123.ap-northeast-2.elb.amazonaws.com

  - type: vpc
    resource: main-vpc
    expect:
      cidr: 10.0.0.0/16
      subnet_count: 2

  - type: cloudwatch_log_group
    resource: /aws/ecs/app
    expect:
      retention_days: 30

  - type: cloudtrail_baseline
    resource: cloudtrail
```

## TUI Navigation

### Global

| Key | Action |
|---|---|
| `j` / `k`, `↑` / `↓` | Move selection, wrapping at list boundaries |
| `Enter` | Select / drill down |
| `Esc` | Go back |
| `q` | Quit from top-level screens |
| `H` | Jump to service list |
| `i` | Enter Inspector mode from the service list |
| `C` | Open context picker |
| `R` | Switch between the active context's configured resource regions |
| `S` | Open settings |
| `P` | Open the command palette (fuzzy search across features, contexts, and indexed resources) |
| `Tab` | Toggle current-context or synced-context resource scope in the command palette |
| `V` | Open saved views (save/apply/delete repeatable feature + filter + context jumps) |
| `/` | Toggle filter mode on supported screens |
| `f` | Favorite/unfavorite the selected service or context on supported lists |
| `?` | Toggle context-aware shortcut help |
| `Ctrl+C` | Force quit |

### Service-specific highlights

| Area | Keys |
|---|---|
| EC2 SSM | `r` refresh, `Enter` connect |
| EC2 Instance Browser | `r` refresh, `/` filter, `A` toggle all-regions scope (multi-region contexts), `Enter` detail, detail `g/a/t/b/n` opens related security groups/ASG/target groups/load balancers/listeners |
| Auto Scaling Groups | `/` filter, `r` refresh, `Enter` capacity/instance/activity detail, detail `↑`/`↓` scroll, `PgUp`/`PgDn` page, `c` change desired capacity (range validation and type-to-confirm) |
| Security Groups | `a` add rule, `d` delete rule, `Tab` switch ingress/egress |
| Reachability Analyzer | Region select first, `←`/`→` or `Tab` change type, `/` filter, `Enter` advance, `Tab`/`↑`/`↓` move config fields, `←`/`→` protocol, `r` rerun |
| RDS | `A` toggle all-regions scope (multi-region contexts), `s` start, `x` stop, `f` failover, `m` modify instance class (filterable class picker, `Tab` apply-immediately toggle, type-to-confirm), `r` refresh |
| CloudFormation | `/` filter, `r` refresh, `Enter` stack detail with parameters/outputs/recent events, detail `d` detect drift, detail `r` refresh when drift detection is idle, `↑`/`↓` scroll, `PgUp`/`PgDn` page |
| Route53 | `c` create, `e` edit, `d` delete |
| IAM Key Rotation | `r` rotate, `c` copy exports, `a` apply and verify, `d` deactivate old key, `x` delete old key |
| Bedrock API Keys | `c` create, choose current IAM user or another user, `r` rotate secret, `d` delete, type the IAM user/key ID to confirm, `c` copy one-time key without printing it, `e` copy `AWS_BEARER_TOKEN_BEDROCK` export |
| CloudTrail Events | `1`-`5` time window (1h/6h/24h/3d/7d), `m` mutations-only toggle, `n` server-side resource-name lookup, `/` filter, `r` refresh, `Enter` detail with scrollable raw event |
| CloudWatch Alarms | `tab` cycle state filter (ALL/ALARM/INSUFFICIENT_DATA/OK), `/` filter, `W` watch, `I` watch interval, `r` refresh, `Enter` detail with recent transitions, detail `g` jump to related resource (RDS/EC2/ECS/Lambda dimensions), `l` jump to logs when derivable |
| CloudWatch Metrics | preset-driven metric list/detail flow, `/` filter, `space` select related series, `g` preset cycle, `t/p/s` range-period-stat controls, `r` refresh, in-terminal single-series and comparison charts |
| CloudWatch Logs | log groups/streams load 10 at a time, `n` load more, `1`-`6` time presets, `t` live tail, `f` filter pattern, `w` wrap toggle, `h/l` horizontal scroll |
| ECR Login Helper | `c` copy Docker login command, `p` copy Podman login command, `r` refresh; CLI helper for scripting: `unic ecr login [--runtime docker\|podman] [--copy]` |
| ECS Exec | `r` refresh, `Enter` drill down / exec |
| ECS Rollout / Exec | cluster/service lists support refresh and drill-down, service detail shows deployments/task definition images/events, `W` watches rollout state, `I` changes the watch interval, `Enter` continues into tasks and exec |
| EKS Browser | cluster/node group/add-on lists support `/` filter and `r` refresh, cluster view shows version/status/endpoint visibility/ARN summary, `a` opens managed add-ons, `U` opens current-version upgrade readiness, `u` opens kubeconfig access helper, node group detail shows desired/min/max scaling plus health issues |
| FIS | `Enter` template detail/run detail, `/` filter, `h` selected-template history, `H` all experiment history, `r` refresh, template detail includes safe-run preview and detail scrolls through targets/actions/stop conditions |
| Inspector Mode | `i` open mode from the service list, `Enter` open the selected workflow, `l` open the checklist file picker |
| Security Inspector | `r` run/rescan, `1`-`5` severity filter, `Enter` finding detail |
| Checklist Inspector | `l` load or switch checklist files, `a` add a check through type-specific prompts, `r` run/rerun the loaded checklist, `Enter` result detail |
| Settings | `Enter`/`Space` toggle selected setting, `Esc`/`q` back |
| Context Picker | `a` add context, `f` favorite/unfavorite selected context, type or `/` filter, `s` setup selected context and quit, `y` copy selected exports and quit, filter-mode `Ctrl+S` setup selected filtered context, filter-mode `Ctrl+Y` copy selected filtered exports, `u` clear shell context and quit with a final confirmation message |
| ECR | `Enter` images, `d` repository detail, `/` filter, `r` refresh, image detail `c` copy digest, `t` copy tag |
| SQS | `A` toggle all-regions scope, `/` filter, `W` watch queue depth, `I` watch interval, `r` refresh, `Enter` detail, detail `d` jump to DLQ, `m` redrive DLQ (type-to-confirm), `x` purge (type-to-confirm) |
| ELB | `A` toggle all-regions scope, `/` filter, `r` refresh, `Enter` target groups, target health screens support `W` watch and `I` watch interval, target group list `Enter` per-target health |
| Parameter Store | `/` filter, `r` refresh, `Enter` detail, detail `v` reveal value (decrypts SecureString), `y` copy value without revealing |
| ElastiCache | `/` filter, `r` refresh, `Enter` nodes, node `Enter` detail, detail `c` copy endpoint |
| KMS | `/` filter, `r` refresh, `Enter` key detail with aliases and rotation status |
| ACM Certificates | `/` filter, `r` refresh, `Enter` certificate detail, detail `↑`/`↓` scroll, `PgUp`/`PgDn` page |
| Step Functions | `/` filter state machines by name/ARN/type/region or executions by status/name/ARN, `r` refresh, `Enter` executions/detail, detail `↑`/`↓` scroll, `PgUp`/`PgDn` page |
| Lambda | `A` toggle all-regions scope (multi-region contexts), `Enter` invoke, `d` detail, `l` view CloudWatch Logs, `/` filter, `r` refresh |

The command palette (`P`) fuzzy-searches three kinds of items from anywhere outside text-entry screens: service features (jump straight into a browser), contexts (switch without opening the picker), and resources indexed across services. Opening the palette starts an async index of EC2 instances, RDS instances, Lambda functions, S3 buckets, ECS clusters, and Route53 zones in the current context. Press `Tab` to opt into searching the active context plus sync-managed contexts; context fan-out is bounded, rows show context and region tags, and per-context/service failures are shown inline. Matching covers names, IDs, ARNs, contexts, and regions where available. Selecting a resource in another context switches context and then jumps to the owning browser with the shared filter prefilled to that resource.

Saved views (`V`) capture repeatable operational workflows: pressing `s` on the views screen snapshots the last opened service feature, its active shared filter, and the current context under a name you type; `enter` reapplies a view in one step — switching to the view's context first when it differs — and `d` deletes one. Views persist under `views:` in `config.yaml` (fields: `name`, `context`, `service`, `feature`, `filter`); the format is additive so future fields extend it without breaking existing files.

The service list defaults to favorites first, then alphabetical order. Press `f` to favorite or unfavorite the selected service; favorites are saved under `favorites.services` in `config.yaml` and rendered with a distinct marker/style. The context picker also supports `f`; context favorites are saved under `favorites.contexts`, displayed first in the picker, and rendered with a distinct color style while preserving the configured context order within favorite and non-favorite groups. The service list supports `/` filtering across service names, feature names, and feature descriptions. Shared list filters use fuzzy matching with inline match highlighting. While filter mode stays active, `↑`/`↓` continue to move through the filtered results without requiring an extra Enter first. Filtering is currently available on the service list, EC2 SSM instances, EC2 inventory instances, IAM users, VPCs, subnets, RDS instances, CloudFormation stacks, Route53 zones/records, CloudWatch metrics, CloudWatch log groups/streams, Secrets Manager resources, ECS clusters/services, EKS clusters/node groups/add-ons, ECR repositories/images, FIS experiment templates/history, ElastiCache replication groups/clusters, S3 buckets/objects, SQS queues, load balancers/target groups, SSM parameters, ACM certificates, Step Functions state machines/executions, Lambda functions, Bedrock API keys, and the context picker.
The service list defaults to favorites first, then alphabetical order. Press `f` to favorite or unfavorite the selected service; favorites are saved under `favorites.services` in `config.yaml` and rendered with a distinct marker/style. The context picker also supports `f`; context favorites are saved under `favorites.contexts`, displayed first in the picker, and rendered with a distinct color style while preserving the configured context order within favorite and non-favorite groups. The service list supports `/` filtering across service names, feature names, and feature descriptions. Shared list filters use fuzzy matching with inline match highlighting. While filter mode stays active, `↑`/`↓` continue to move through the filtered results without requiring an extra Enter first. Filtering is currently available on the service list, EC2 SSM instances, EC2 inventory instances, Auto Scaling groups, IAM users, VPCs, subnets, RDS instances, Route53 zones/records, CloudWatch metrics, CloudWatch log groups/streams, Secrets Manager resources, ECS clusters/services, EKS clusters/node groups/add-ons, ECR repositories/images, FIS experiment templates/history, ElastiCache replication groups/clusters, S3 buckets/objects, SQS queues, load balancers/target groups, SSM parameters, ACM certificates, Step Functions state machines/executions, Lambda functions, Bedrock API keys, and the context picker.

Watch mode is available on the CloudWatch alarm list, ECS rollout detail, SQS queue list/detail, and ELB target-group/target-health screens. Press `W` to toggle opt-in background refresh and `I` to cycle the 5s, 15s, and 30s presets. The active screen keeps its selection or scroll position while data changes; leaving the screen or starting an explicit refresh stops watch mode and cancels any in-flight watch request.

The EKS Browser includes a managed add-on status view for each cluster. Add-on rows show the installed version, status, and health summary, with degraded or unhealthy add-ons highlighted so core components such as CoreDNS, kube-proxy, VPC CNI, and CSI drivers are easy to spot.

The ECR Login Helper resolves the private registry URI for the active context and shows copyable Docker and Podman login commands without leaving the TUI. The copied commands are prefixed with `eval "$(unic env <context>)"` so they authenticate with the active unic context rather than whatever ambient AWS credentials the shell happens to have; `unic ecr login` remains as a secondary CLI helper for scripting. The ECR Repository Browser opens image/tag lists from each repository. Image rows include tags, digest, pushed time, and size, and mark untagged images or images older than 90 days as cleanup candidates. Image detail exposes digest and tag values for clipboard copy.

The RDS detail screen can resize an instance with `m`: unic loads the orderable DB instance classes for the instance's engine/version in the active region into a filterable picker (current class marked), then a confirmation screen shows the current and new class with a `Tab`-toggleable apply-immediately choice (default: next maintenance window) and requires typing the instance identifier before calling ModifyDBInstance. After submitting, the detail screen polls the instance status until the change settles.

The CloudFormation Stack Browser lists failed and rollback stacks first, followed by in-progress and healthy stacks. Stack detail shows status reasons, parameters, outputs, and up to 30 of the newest resource events with failure reasons. Press `d` to start drift detection; the detail view polls for up to five minutes for CloudFormation to report the resulting stack drift status and drifted-resource count.

The SQS Queue Browser is a backlog-first triage view: queues list deepest-backlog first with `!` marking dead-letter queues, showing depth, in-flight, and delayed counts per row. Queue detail shows the redrive relationship — `d` jumps from a source queue into its DLQ, and on a DLQ `m` starts a redrive (StartMessageMoveTask) moving messages back to their source queues. `x` purges a queue. Both actions require typing the queue name to confirm, and rows loaded through the all-regions scope act against their own region.

The Load Balancer Browser is a target-health-first triage view: the load balancer list (name, type, scheme, state, with `A` toggling an all-regions scope) drills into the balancer's target groups sorted unhealthiest first with per-group healthy/unhealthy/other counts, and one more `Enter` opens per-target health with unhealthy targets first, showing reason codes (e.g. `Target.Timeout`) and descriptions. Rows loaded through the all-regions scope drill into their own region.

The Parameter Store Browser lists parameter metadata (hierarchical path, type, tier, last-modified) with the shared filter matching path segments. Values are never fetched or rendered implicitly: parameter detail shows metadata with the value hidden, `v` explicitly fetches and reveals it (decrypting SecureString), and `y` fetches and copies the value straight to the clipboard without ever printing it to the terminal.

The ElastiCache Browser lists Valkey/Redis replication groups and standalone cache clusters with engine, status, node count, and node type. Opening a resource shows its cache nodes with shard, role, status, and Availability Zone; node detail exposes the connection endpoint and `c` copies it to the clipboard.
The KMS Key Browser lists key state, manager, aliases, and automatic-rotation posture. Key detail shows the ARN, origin, description, aliases, and rotation status; Security Inspector reports customer-managed keys that do not have automatic rotation enabled.
The ACM Certificate Browser sorts certificates by soonest expiry and shows status, days remaining, in-use count, renewal eligibility, domains/SANs, validation state, and attached resource ARNs. Security Inspector reports certificates expiring within 30 days by default, with certificates at seven days or less raised as high severity. Set `inspector.acm_expiry_window_days` in `config.yaml` to use a different positive-day warning window.
The Step Functions Execution Browser lists state machines in the active region, then shows up to the 200 most recent STANDARD workflow executions with failed, timed-out, aborted, and pending-redrive runs ahead of running and successful runs. Execution detail shows timing, the failed state resolved from execution history, error/cause, and compact input/output previews. If execution history is unavailable or not permitted, the remaining execution detail still loads with the failed state shown as unavailable. AWS does not expose execution listing or history for EXPRESS state machines through these APIs, so unic lists them but explains that their execution history is unavailable.

The CloudTrail Event Lookup answers "who changed what, and when": recent API events list newest-first with mutations marked `*`, actor, call, and source service per row. Keys `1`-`5` switch the time window (1h/6h/24h/3d/7d), `m` restricts to mutations (server-side via the `ReadOnly=false` lookup attribute), and `n` runs a server-side resource-name lookup — CloudTrail accepts one lookup attribute per call, so combining both applies the mutations restriction client-side. Results are capped at 100 events per query, so narrow the window or use the resource lookup when a busy account truncates. Event detail shows actor, source, region, source IP, touched resources, and the full raw event JSON with scrolling.

The CloudWatch Alarm Browser is an alarm-first incident entry point: alarms list firing-first (ALARM, then INSUFFICIENT_DATA, then OK) with a `tab`-cycled state filter and text filtering across names, states, metrics, and dimensions. Alarm detail shows the state reason, condition, dimensions, and the most recent state transitions. When an alarm's dimensions map to a supported browser (`DBInstanceIdentifier`, `InstanceId`, `ClusterName`, `FunctionName`, `LoadBalancer`, `TargetGroup`), `g` jumps into that resource browser with the filter prefilled to the unhealthy resource (for ELB alarms the target group filter is prefilled too, so the drill-down lands on the alarmed target group), and `l` opens CloudWatch Logs prefilled with the derived log group (e.g. `/aws/lambda/<function>`).

The FIS Experiment Template Browser lists experiment templates in the active region and opens a detail screen with role ARN, targets, actions, target mappings, parameters, filters, and stop condition summaries without leaving the TUI. Template detail includes a Safe Run Preview that summarizes blast radius, target selection modes, action count, active stop conditions, IAM role, and warnings for missing stop conditions, missing role ARN, broad selection, or unbounded selectors. The preview also states the template ID that any future execution path must type to confirm before a run can start. Press `h` on a selected template or template detail to inspect recent runs for that template, or `H` from the template list to inspect recent experiment history across the active account/region. History rows include run status, timing, and stop/failure summaries, with failed, stopped, stopping, and cancelled runs visually highlighted; `Enter` opens run detail with start/end times, duration, action states, targets, stop conditions, and failure metadata.

The EKS Browser includes a current-version upgrade readiness view for each selected cluster. It compares the control plane version with managed node group versions, checks installed managed add-on versions against EKS compatibility metadata for the current cluster version, includes EKS `UPGRADE_READINESS` insights, and highlights blockers or warnings before planning a target-version upgrade.

The EKS Browser includes an access helper for each selected cluster. Press `u` from the cluster list to review the cluster endpoint, ARN, current region/profile, and copy an `aws eks update-kubeconfig` command or a `kubectl get nodes` smoke-check command for handoff into Kubernetes workflows.

The EC2 Instance Browser lists EC2 instances across available states for the active context and region, separate from the SSM session picker that only lists connectable running instances. For multi-region contexts, press `A` to toggle an all-regions scope that lists instances from every configured resource region in one view: rows gain a region tag, per-region API failures are shown inline without hiding other regions' results, and detail and related-resource drill-downs query the selected instance's own region. The detail screen shows core metadata including instance ID, name tag, state, instance type, AZ, VPC, subnet, security groups, private and public IPs, launch time, platform details, IAM profile, and tags. From instance detail, related-resource drill-down screens open attached security groups (`g`), Auto Scaling membership (`a`), registered target groups (`t`), associated load balancers (`b`), and listeners (`n`). Related lists support filtering, refresh, wrap navigation, empty states for missing associations, and inline errors when a relationship cannot be loaded because of API or permission failures.

The Auto Scaling Group Browser lists desired, minimum, and maximum capacity alongside instance and healthy-instance counts. Group detail shows each instance's lifecycle and health state plus the 20 most recent scaling activities, including AWS failure messages when an activity fails. Press `c` to enter a new desired capacity within the group's current min/max bounds; unic warns that this can launch or terminate instances and requires the exact group name before calling `SetDesiredCapacity`.

Bedrock API key management uses the active unic AWS context and IAM service-specific credential APIs for `bedrock.amazonaws.com`. The TUI lists long-term Bedrock API key metadata, opens a detail screen for inspection, defaults new key generation to the current IAM user when that user can be inferred from caller identity, and keeps another-user generation as an explicit option. Creation supports an optional expiration period, where blank or `0` means no expiration, rotates secrets with a one-time result screen, and deletes keys only after typed confirmation. Generated and rotated key values are intentionally copy-only and are not printed to the terminal; on the result screen, `c` copies the key and `e` copies `export AWS_BEARER_TOKEN_BEDROCK=...`.

Reachability Analyzer starts with a region selection step, defaults to the current context region, and now surfaces the AWS-documented source and destination resource types that unic supports: EC2 instances, Internet gateways, Network interfaces, Transit gateways, Transit gateway attachments, Virtual private gateways, VPC endpoint services, VPC endpoints, VPC peering connections, plus IP addresses as destinations. The source and destination pickers support type tabs, keyword filtering, IPv4 destination validation, and automatic cleanup of temporary Network Insights resources after each analysis. During analysis, the loading screen shows a vertical source-to-destination flow and intent summary, and the result view renders path hops and findings in a more readable layout.

## Product Principles

- **TUI-first.** New user-facing AWS service capabilities ship with a TUI entry point first. CLI commands are secondary surfaces for scripting, automation, or copy/paste handoff, and should be documented as such.

## Development

```bash
go run ./cmd/unic
go test ./...
make build
```

Release artifacts are produced through GoReleaser and the `dist/` outputs.

## Community Standards

- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Contributing Guide](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)
- [License](LICENSE)

## Issue Bot

Comment on an issue with:

| Command | Action |
|---|---|
| `@unic-bot: assign me` | Assign the issue to yourself |

## Contributors

<p align="left">
  <a href="https://github.com/nathanhuh">
    <img src="https://avatars.githubusercontent.com/u/23186038?v=4" width="72" alt="nathanhuh" />
  </a>
  <a href="https://github.com/YoungJinJung">
    <img src="https://avatars.githubusercontent.com/u/18644538?v=4" width="72" alt="YoungJinJung" />
  </a>
  <a href="https://github.com/jjjjjjeonda86">
    <img src="https://avatars.githubusercontent.com/u/37580012?v=4" width="72" alt="jjjjjjeonda86" />
  </a>
</p>

- [nathanhuh](https://github.com/nathanhuh)
- [YoungJinJung](https://github.com/YoungJinJung)
- [jjjjjjeonda86](https://github.com/jjjjjjeonda86)
