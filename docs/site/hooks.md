# Hook System

Thimble's hook dispatcher intercepts IDE tool calls and lifecycle events, providing security enforcement, session tracking, and context injection. Hooks run in-process with ~10ms latency.

## Hook Events

| Event | When It Fires | What It Does |
|-------|---------------|-------------|
| `PreToolUse` | Before a tool executes | Security check, routing instructions, can block or modify |
| `PostToolUse` | After a tool executes | Session event recording, auto-indexing, output modification |
| `SessionStart` | When a new session begins | Initialize session context, load directives |
| `PreCompact` | Before context compaction | Inject priority context to survive compaction |
| `UserPromptSubmit` | When user submits a prompt | Pre-process user input |

## Hook Configuration

Hooks are defined in `hooks/hooks.json` (deployed by `thimble setup --plugin`):

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash|Read|Edit|Write|WebFetch|Grep|Agent|Task|mcp__thimble__ctx_execute|mcp__thimble__ctx_execute_file|mcp__thimble__ctx_batch_execute",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/thimble hook claude-code pretooluse"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/thimble hook claude-code posttooluse"
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/thimble hook claude-code sessionstart"
          }
        ]
      }
    ],
    "PreCompact": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/thimble hook claude-code precompact"
          }
        ]
      }
    ]
  }
}
```

### Matchers

- Empty string (`""`) matches all tool calls
- Pipe-separated patterns match specific tools: `"Bash|Read|Edit|Write"`
- Matchers are regex patterns applied to tool names

## Hook Dispatch Flow

```
IDE -> stdin(JSON payload) -> thimble hook <platform> <event>
  -> Platform adapter parses to NormalizedEvent
  -> HookDispatcher evaluates security policies
  -> SessionDB records event
  -> Platform adapter formats response
  -> stdout(JSON response) -> IDE
```

### Hook Responses

| Decision | Effect |
|----------|--------|
| `allow` | Tool execution proceeds normally |
| `deny` | Tool execution is blocked; reason shown to user |
| `modify` | Tool input is modified before execution |
| `context` | Additional context is injected (PreCompact/SessionStart) |

## Security Policies

The security engine evaluates commands against three pattern lists:

### Permission Patterns

```
Allow: ["Bash(git status*)", "Bash(ls *)", "Read(**)"]
Deny:  ["Bash(rm -rf *)"]
Ask:   ["Bash(git push*)"]
```

Pattern syntax: `ToolName(glob)` where the glob is matched against the tool's arguments.

### Security Checks

| Check | What It Catches |
|-------|----------------|
| Bash policy enforcement | Commands matching deny patterns |
| File path deny globs | Access to sensitive paths (`.env`, credentials) |
| Shell-escape detection | Attempts to escape sandboxed execution |
| curl/wget/inline-HTTP detection | Unauthorized network access in shell commands |
| Git dangerous command blocking | Force pushes, hard resets on protected branches |

### Git-Specific Policies

The `internal/security/gitpolicy.go` module enforces git-specific rules:

- Blocks `git push --force` to protected branches
- Blocks `git reset --hard` without explicit confirmation
- Blocks `git clean -f` operations
- Warns on `git checkout .` and `git restore .`

## Hook Logging

All hook interactions are logged to an append-only JSONL file at `{DataDir}/hooklog.jsonl`.

```bash
thimble hooklog             # View all interactions
thimble hooklog --blocked   # Only blocked/denied
thimble hooklog --debug     # Full payloads
thimble hooklog --clear     # Clear the log
```

## OpenTelemetry Integration

When tracing is enabled (`THIMBLE_TRACING=1`), hooks emit OTel spans via `observe.go` helpers (`spanTool`, `spanHook`). This provides distributed tracing across MCP tool calls and hook evaluations.

## Plugin Hooks

Plugins can define their own hooks that run alongside built-in hooks. See [Plugins](plugins.md) for the hook definition format.

Valid plugin hook events: `PreToolUse`, `PostToolUse`, `SessionStart`, `PreCompact`.

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "ctx_docker_.*",
        "command": "docker info --format '{{.ServerVersion}}'"
      }
    ]
  }
}
```
