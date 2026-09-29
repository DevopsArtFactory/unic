---
name: unic-aws
description: Use unic to discover supported AWS operations, inspect AWS resources, or preview SSO context synchronization through MCP.
---

# unic AWS

Use the `unic` MCP server for supported AWS inspection and context planning.

1. Call `get_mcp_capabilities` first to discover operations this MCP server can actually execute.
2. Call `get_capabilities` only when broader unic TUI or CLI feature discovery is useful.
3. Call `get_command_schema` before composing an automation command contract.
4. Call a discovered read-only resource tool with optional `profile` and `region` arguments; for example, `list_backup_vaults` or `list_sns_topics`.
5. Call `plan_context_sync` to preview SSO context changes. It never applies or writes configuration.

The server inherits local unic and AWS configuration from its process environment. Never request, store, or place AWS credentials in plugin configuration. Report structured permission errors and partial-result warnings to the user.
