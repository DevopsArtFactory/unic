---
name: unic-aws
description: Use unic to discover supported AWS operations, inspect AWS Backup vaults, or preview SSO context synchronization through MCP.
---

# unic AWS

Use the `unic` MCP server for supported AWS inspection and context planning.

1. Call `get_capabilities` when the requested service or operation is unclear.
2. Call `get_command_schema` before composing an automation command contract.
3. Call `list_backup_vaults` with optional `profile` and `region` arguments to inspect AWS Backup.
4. Call `plan_context_sync` to preview SSO context changes. It never applies or writes configuration.

The server inherits local unic and AWS configuration from its process environment. Never request, store, or place AWS credentials in plugin configuration. Report structured permission errors and partial-result warnings to the user.
