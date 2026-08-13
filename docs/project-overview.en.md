# UNIC Project Overview

UNIC is a Go-based AWS terminal console that combines:

- a Bubble Tea TUI for browsing and operating AWS resources
- Cobra CLI helpers for context setup and environment export
- context-aware authentication across credential, assume-role, SSO, and Okta SAML workflows

## Current Scope

Implemented service areas currently include:

- EC2 / SSM
- VPC
- RDS
- Route53
- Secrets Manager
- IAM
- CloudWatch Metrics
- CloudWatch Logs
- ECS
- ECR
- FIS
- S3
- Lambda
- Inspector mode

The application already includes interactive mutation flows, polling-based status flows, context helpers, and per-service drill-down screens. CloudWatch Metrics now includes resource-centric preset groups plus time-range, period, and statistic controls for faster terminal triage. EKS includes managed add-on status review, current-version upgrade readiness checks that compare control plane, managed node group, managed add-on version alignment, and EKS upgrade insights before a target upgrade is planned, plus a kubeconfig access helper that prepares copyable `aws eks update-kubeconfig` and `kubectl` handoff commands. ECR includes repository and image/tag browsing with cleanup-oriented untagged and stale image signals. FIS includes experiment template browsing with safe-run blast-radius preview, targets, actions, role ARN, stop condition summaries, and recent experiment history with status, timing, and failure/stop reasons.
Inspector mode now includes built-in security scans plus checklist-driven readiness checks for RDS, security groups, secrets, Route53, VPCs/subnets, CloudWatch Logs, and baseline posture wrappers.

## Primary User Flows

1. Run `unic` to enter the TUI
2. Select an AWS service and feature from the catalog, or press `i` to enter Inspector mode
3. Optionally start `unic --checklist <path>` to pre-load Checklist Inspector, or load a YAML readiness file from the in-TUI checklist picker
4. Drill into resource lists, inspector workflows, and detail views
5. Execute supported actions when available
6. Use `unic env` or `unic context setup` when shell exports are needed

## Configuration and Auth

Configuration lives in `~/.config/unic/config.yaml`.
The app supports:

- legacy flat config
- context-based config with `current`
- `credential` auth
- `console_login` auth for AWS CLI `aws login`-backed local development profiles
- `assume_role` auth
- `sso` auth, including base contexts resolved by `unic context setup`
- `okta_saml` auth using the Okta app embed link and `sts:AssumeRoleWithSAML` (v1 MFA: TOTP and Okta Verify push)

## Repository Layout

```text
cmd/unic/                 entrypoint
internal/cli/             Cobra commands
internal/config/          config load/save and context helpers
internal/auth/            env export and interactive setup logic
internal/domain/          AWS service catalog and feature enums
internal/services/aws/    AWS repository, models, and service operations
internal/inspector/       cross-service inspector workflows, findings, and rule packs
internal/app/             Bubble Tea model, screens, styles, messages
```

## Maintenance Principles

- keep README aligned with real behavior
- treat `docs/` as the canonical documentation location
- update architecture docs when screen or module boundaries change materially
- prefer tests for repository logic and TUI transitions
