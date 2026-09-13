package unic

import _ "embed"

// CodexMCPManifest is the Codex plugin MCP registration shipped by unic.
//
//go:embed .mcp.json
var CodexMCPManifest []byte

// AgentMCPManifest is the Agent Plugins MCP registration shipped by unic.
//
//go:embed mcp.json
var AgentMCPManifest []byte
