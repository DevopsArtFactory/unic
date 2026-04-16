# unic

`unic` is a Go-based TUI for browsing and operating AWS resources from the terminal.
It combines a Bubble Tea application, Cobra-based CLI commands, and AWS SDK v2 clients behind a context-aware authentication layer.

## What It Does

- Browse AWS services from a single terminal UI
- Switch between credential, assume-role, and SSO contexts
- Export shell environment variables for the active context
- Drill down into resources with filters, detail views, and action screens
- Open a context-aware keyboard shortcut help screen with `?`
- Show animated loading indicators while async AWS data is being fetched
- Perform operational workflows such as SSM sessions, RDS control, Route53 record changes, ECS exec, and IAM access key rotation
- Open the Security Inspector workflow and review built-in findings for Security Groups, RDS, IAM keys, Secrets Manager, and S3 buckets

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
- [Roadmap / Planning Notes](docs/roadmap.md)
- [Ticket Tracking Note](TICKET.md)

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
unic --verbose
```

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

# Set a display order for a context
unic context order prod-admin 10

# Or open reorder mode, then move the selected context with arrow keys and save
unic context order

# Clear current context and copy cleanup commands to clipboard
unic context unset
```

`unic context setup` writes its prompts to `stderr` and copies the generated shell commands to the clipboard.
`unic env` prints shell commands to `stdout` so it can be used with `eval`.
Both flows now include a `UNIC_CONTEXT` marker in the generated exports so the TUI can show which shell context is currently active.
Contexts can be prioritized in the setup picker with an `order` field in config.
In the CLI `unic context setup` flow, the picker now filters contexts, SSO accounts, and SSO roles as you type, with arrow-key navigation and Enter to confirm.
Use `unic context order` to open reorder mode, choose a context with `↑/↓` or `j/k`, press `Enter` to start moving it, then press `Enter` again to save. `unic context order <name> <number>` still works for direct updates.

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

  - name: prod-admin
    order: 20
    profile: base-profile
    region: us-east-1
    auth_type: assume_role
    role_arn: arn:aws:iam::123456789012:role/Admin
    external_id: optional-external-id

  - name: staging
    profile: staging
    region: eu-west-1
    auth_type: credential
```

### Auth Types

| Auth Type | Meaning | Required Fields |
|---|---|---|
| `credential` | Use shared AWS profile credentials | `profile` |
| `assume_role` | Assume a role from a base profile | `profile`, `role_arn` |
| `sso` | Use AWS IAM Identity Center / SSO | `profile`, `sso_start_url`, and for concrete contexts `sso_account_id`, `sso_role_name` |

Optional context fields:

| Field | Meaning |
|---|---|
| `order` | Lower values appear first in the context setup picker. Contexts without `order` fall back after ordered entries in their existing file order. |

Resolution priority:

```text
CLI flags > selected context > config defaults > hardcoded default (us-east-1)
```

Context ordering:

- Lower `order` values appear first
- Contexts without `order` appear after ordered contexts
- Contexts with the same `order` keep their file order

## Current Features

| Service | Feature |
|---|---|
| EC2 | SSM Session Manager |
| EC2 | Security Group Browser |
| VPC | VPC Browser |
| VPC | Reachability Analyzer |
| RDS | RDS Browser |
| Route53 | Route53 Browser |
| Secrets Manager | Secrets Browser |
| CloudWatch Logs | Logs Browser |
| ECS | ECS Exec Sessions |
| S3 | S3 Browser |
| IAM | IAM User Browser |
| IAM | ListAccessKeys |
| IAM | RotateAccessKey |
| Inspector | Security Scan |

The Inspector feature ships built-in rule packs for Security Group exposure, RDS encryption/public access/backups, IAM access key age, Secrets Manager rotation age, and S3 public access/versioning checks.

## TUI Navigation

### Global

| Key | Action |
|---|---|
| `j` / `k`, `↑` / `↓` | Move selection |
| `Enter` | Select / drill down |
| `Esc` | Go back |
| `q` | Quit from top-level screens |
| `H` | Jump to service list |
| `C` | Open context picker |
| `/` | Toggle filter mode on supported screens |
| `?` | Toggle context-aware shortcut help |
| `Ctrl+C` | Force quit |

### Service-specific highlights

| Area | Keys |
|---|---|
| EC2 SSM | `r` refresh, `Enter` connect |
| Security Groups | `a` add rule, `d` delete rule, `Tab` switch ingress/egress |
| Reachability Analyzer | Region select first, `←`/`→` or `Tab` change type, `/` filter, `Enter` advance, `Tab`/`↑`/`↓` move config fields, `←`/`→` protocol, `r` rerun |
| RDS | `s` start, `x` stop, `f` failover, `r` refresh |
| Route53 | `c` create, `e` edit, `d` delete |
| IAM Key Rotation | `r` rotate, `c` copy exports, `a` apply and verify, `d` deactivate old key, `x` delete old key |
| CloudWatch Logs | log groups/streams load 10 at a time, `n` load more, `1`-`6` time presets, `t` live tail, `f` filter pattern, `w` wrap toggle, `h/l` horizontal scroll |
| ECS Exec | `r` refresh, `Enter` drill down / exec |
| Inspector | `r` run/rescan, `1`-`5` severity filter, `Enter` finding detail |
| Context Picker | `a` add context, type or `/` filter, `s` setup selected context and quit, `y` copy selected exports and quit, `u` clear shell context and quit with a final confirmation message |

Filtering is currently available on EC2 instances, IAM users, VPCs/subnets, RDS instances, Route53 zones/records, CloudWatch log groups/streams, Secrets Manager resources, ECS clusters/services, S3 buckets/objects, and the context picker.

Reachability Analyzer starts with a region selection step, defaults to the current context region, and now surfaces the AWS-documented source and destination resource types that unic supports: EC2 instances, Internet gateways, Network interfaces, Transit gateways, Transit gateway attachments, Virtual private gateways, VPC endpoint services, VPC endpoints, VPC peering connections, plus IP addresses as destinations. The source and destination pickers support type tabs, keyword filtering, IPv4 destination validation, and automatic cleanup of temporary Network Insights resources after each analysis. During analysis, the loading screen shows a vertical source-to-destination flow and intent summary, and the result view renders path hops and findings in a more readable layout.

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
