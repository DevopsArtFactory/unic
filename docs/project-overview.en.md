# UNIC Project Overview

UNIC is a Go-based AWS terminal console that combines:

- a Bubble Tea TUI for browsing and operating AWS resources
- Cobra CLI helpers for context setup and environment export
- context-aware authentication across credential, assume-role, SSO, and Okta SAML workflows

## Current Scope

Implemented service areas currently include:

- EC2 (including SSM Session Manager)
- VPC
- RDS
- Route53
- Secrets Manager
- IAM
- CloudWatch Metrics
- CloudWatch Alarms
- CloudWatch Logs
- CloudTrail
- EventBridge
- ECS
- ECR
- EKS
- FIS
- ElastiCache
- S3
- SNS
- SQS
- ELB
- Parameter Store
- KMS
- ACM
- Step Functions
- Lambda
- Bedrock
- CloudFormation
- DynamoDB
- AWS Backup
- API Gateway v2
- WAF
- Inspector mode

The application already includes interactive mutation flows, polling-based status flows, context helpers, and per-service drill-down screens. EC2 includes a first-class Auto Scaling Group browser for capacity, instance health, recent activity failures, and type-confirmed desired-capacity changes. CloudFormation includes failure-prioritized stack browsing, parameters, outputs, recent events with failure reasons, and polling-based drift detection. CloudWatch Metrics now includes resource-centric preset groups plus time-range, period, and statistic controls for faster terminal triage. EKS includes managed add-on status review, current-version upgrade readiness checks that compare control plane, managed node group, managed add-on version alignment, and EKS upgrade insights before a target upgrade is planned, plus a kubeconfig access helper that prepares copyable `aws eks update-kubeconfig` and `kubectl` handoff commands. ECR includes repository and image/tag browsing with cleanup-oriented untagged and stale image signals. FIS includes experiment template browsing with safe-run blast-radius preview, targets, actions, role ARN, stop condition summaries, and recent experiment history with status, timing, and failure/stop reasons. ACM includes an expiry-sorted certificate browser with validation, renewal, domain, and in-use details. KMS includes key browsing with aliases, state, manager, and automatic-rotation posture. Both browsers keep successfully loaded resources visible when an individual detail lookup fails and surface the failure inline; denied KMS rotation lookups are shown as unknown. ElastiCache includes replication-group and standalone-cluster browsing with node metadata and copyable endpoints. Step Functions includes state machine browsing and failure-first STANDARD execution triage with failed-state, error/cause, and input/output previews. EventBridge includes cross-bus rule browsing with complete scrollable event patterns, targets, best-effort seven-day CloudWatch trigger activity, and type-to-confirm enable/disable actions for eligible customer-managed rules; all-management-events rules remain read-only so their exact matching mode is preserved. SNS includes name-sorted topic browsing with subscription counts, encryption, and delivery policies, plus a pending-first subscription list; topics whose attribute lookup is denied stay listed with their attributes marked unavailable. DynamoDB includes table capacity, size, key, GSI, TTL, and stream inspection plus a single `GetItem` lookup by complete primary key; it has no scan path.
WAF combines regional and CloudFront-scope Web ACL posture, priority-ordered rules, logging, and supported resource associations while keeping scope and per-resource authorization failures isolated.
API Gateway v2 includes HTTP/WebSocket API, stage, route, and integration browsing with partial-detail warnings, copyable targets, and filtered Lambda handoff.
AWS Backup includes read-only vault browsing with recovery points, protected resources, Vault Lock/encryption metadata, and recent failed or expired jobs; paginated partial results remain visible with inline warnings.
Inspector mode now includes built-in security and cost/waste scans, including customer-managed KMS key rotation checks and ACM certificate expiry findings, plus checklist-driven readiness checks for RDS, security groups, secrets, Route53, VPCs/subnets, CloudWatch Logs, and baseline posture wrappers. The cost/waste pack surfaces unattached EIPs and EBS volumes, stopped EC2 instances, empty target groups, untagged EC2-family resources, and EBS snapshots aged 90 days or more. Per-resource lookup failures appear as warnings while findings from successful lookups remain available. When `inspector.required_tags` is configured, it additionally reports missing required keys on Elastic IPs, EBS volumes and snapshots, and EC2 instances.

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
- a positive `inspector.acm_expiry_window_days` override for the default 30-day ACM warning window
- an optional `inspector.required_tags` list that enables policy-based missing-tag findings
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
