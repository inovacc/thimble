// Package plugin provides embedded plugin assets for deployment via thimble setup.
//
// The embedded filesystem contains the complete Claude Code plugin structure:
//
//	.claude-plugin/plugin.json       — Plugin manifest
//	.claude-plugin/marketplace.json  — Marketplace catalog
//	.mcp.json                        — MCP server configuration
//	hooks/hooks.json                 — Hook definitions
//	skills/*/SKILL.md                — Skill definitions
package plugin

import "embed"

// FS contains the embedded plugin assets rooted at this directory.
// All paths are relative (e.g., ".claude-plugin/plugin.json", "hooks/hooks.json").
//
//go:embed .claude-plugin .mcp.json hooks skills
var FS embed.FS
