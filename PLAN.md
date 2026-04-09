# unic Roadmap Notes

This file is no longer the source of truth for implementation status.
Current planning and delivery are tracked in GitHub issues and pull requests.

## What This File Is For Now

Use this file as a lightweight roadmap note, not as a milestone-by-milestone checklist.
For actual status, prefer:

- GitHub Issues
- GitHub Pull Requests
- `README.md`
- `.kiro/docs/architecture-en.md`
- `.kiro/docs/architecture.md`

## Current Product Areas

- Context setup and shell export workflows
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
- continue modularizing large app flows without changing UX unexpectedly
- preserve test coverage for repository logic and screen transitions
- keep auth flows explicit and predictable across credential / assume-role / SSO contexts

## Historical Note

Earlier versions of this file tracked milestone plans for features that are now already implemented or have since changed shape. That content was intentionally retired to avoid stale guidance.
