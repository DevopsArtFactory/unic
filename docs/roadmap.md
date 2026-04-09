# Roadmap Notes

This file is intentionally lightweight.
Current implementation truth lives in code, tests, README, and GitHub issues/PRs.

## Current Product Areas

- context setup and shell export workflows
- EC2 / SSM operations
- VPC exploration
- RDS operations
- Route53 record management
- Secrets Manager browsing
- IAM user and access key workflows
- CloudWatch Logs browsing and live tail
- ECS exec workflows
- S3 bucket and object browsing

## Near-Term Maintenance Themes

- keep docs synchronized with real behavior
- continue modularizing large app flows without breaking established UX
- preserve test coverage for repository logic and screen transitions
- keep auth flows explicit and predictable across credential / assume-role / SSO contexts
