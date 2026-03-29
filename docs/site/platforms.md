# Supported Platforms

Thimble supports 9 IDE platforms through platform-specific adapters. The MCP server layer is 100% portable -- only the hook layer requires platform-specific adapters.

## Platform Comparison

| Platform | Paradigm | Hook Events | Adapter File |
|----------|----------|-------------|-------------|
| Claude Code | JSON-stdio hooks | PreToolUse, PostToolUse, PreCompact, SessionStart | `claude_code.go` |
| Gemini CLI | JSON-stdio hooks | BeforeTool, AfterTool, PreCompress, SessionStart | `gemini_cli.go` |
| VS Code Copilot | JSON-stdio hooks | PreToolUse, PostToolUse, PreCompact, SessionStart | `vscode_copilot.go` |
| Cursor | JSON-stdio hooks | preToolUse, postToolUse | `cursor.go` |
| Kiro | JSON-stdio hooks | PreToolUse, PostToolUse | `kiro.go` |
| OpenClaw | JSON-stdio hooks | PreToolUse, PostToolUse, SessionStart | `openclaw.go` |
| Antigravity | JSON-stdio hooks | PreToolUse, PostToolUse | `antigravity.go` |
| OpenCode | TS plugin | N/A (MCP only) | `opencode.go` |
| Codex | MCP only | N/A | `codex.go` |

## Paradigms

### JSON-stdio Hooks

Most platforms use JSON-stdio hooks: the IDE sends a JSON payload to thimble's stdin, and thimble responds with a JSON payload on stdout. This enables:

- **PreToolUse** -- Intercept and block/modify tool calls before execution
- **PostToolUse** -- Record tool results, modify output
- **PreCompact** -- Inject priority context before compaction
- **SessionStart** -- Initialize session state

### TS Plugin

OpenCode uses a TypeScript plugin paradigm where thimble acts as an imported module rather than a subprocess.

### MCP Only

Codex uses thimble purely as an MCP server with no hook integration. All 120+ tools are available but security enforcement and session tracking do not apply.

## Capabilities by Platform

| Platform | Pre-Tool | Post-Tool | Compact | Session | Modify Args | Modify Output | Inject Context |
|----------|----------|-----------|---------|---------|-------------|---------------|----------------|
| Claude Code | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Gemini CLI | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| VS Code Copilot | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Cursor | Yes | Yes | No | No | Yes | Yes | No |
| Kiro | Yes | Yes | No | No | Yes | Yes | No |
| OpenClaw | Yes | Yes | No | Yes | Yes | Yes | Yes |
| Antigravity | Yes | Yes | No | No | Yes | Yes | No |
| OpenCode | No | No | No | No | No | No | No |
| Codex | No | No | No | No | No | No | No |

## Setup

Configure hooks for your platform:

```bash
# Auto-detect platform
thimble setup

# Specify explicitly
thimble setup --client claude
thimble setup --client gemini
thimble setup --client cursor
thimble setup --client vscode-copilot
thimble setup --client opencode
thimble setup --client codex
thimble setup --client kiro
thimble setup --client openclaw
thimble setup --client antigravity

# Deploy as full plugin (Claude Code only)
thimble setup --client claude --plugin
```

## Hook Validation

Each adapter implements `ValidateHooks()` which checks:

1. Whether the platform's settings file exists
2. Whether expected hook entries are configured

Run `thimble doctor` to see validation results for all detected platforms.

## Platform Detection

Thimble can auto-detect the current platform via `internal/platform/detect.go`. Detection checks for platform-specific indicators (environment variables, process names, settings files) and returns the matching `PlatformID`.

## Routing Instructions

The `internal/routing/` package provides platform-specific PreToolUse routing instructions. These are tailored guidance messages injected before tool execution to help the AI assistant use tools correctly on each platform.

## Adding a New Platform

To add support for a new IDE:

1. Add a new `PlatformID` constant in `internal/platform/platform.go`
2. Create an adapter file (e.g., `myplatform.go`) implementing the `Adapter` interface
3. Register it in the `Get()` switch statement and `AllPlatformIDs()` slice
4. Add routing instructions in `internal/routing/`
5. Add setup support in `cmd/thimble/setup.go`
