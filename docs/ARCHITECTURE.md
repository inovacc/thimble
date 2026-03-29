# Architecture

## Overview

Thimble is a single-binary MCP plugin. Every instance is standalone — no daemon, no gRPC, no discovery chain. The MCP bridge opens SQLite databases directly and calls service packages as in-process function calls.

```mermaid
graph TB
    subgraph "IDE Process"
        IDE[AI Coding Assistant]
        MCP[MCP Bridge<br/>stdio transport]
        HOOK[Hook Dispatcher<br/>in-process, ~10ms]
    end

    subgraph "thimble (single binary)"
        CS[ContentStore<br/>FTS5 per-project stores]
        SS[SessionDB<br/>event tracking]
        EX[PolyglotExecutor<br/>11 languages + streaming]
        SEC[Security Engine<br/>policy enforcement]
        CA[CodeAnalysis<br/>8 language parsers]
        HK[Hooks<br/>pre/post tool use]
        TD[TaskDelegate<br/>background tasks]
        GIT[GitOps<br/>13 operations]
        GH[GhCli<br/>subprocess gh]
        LINT[Linter<br/>subprocess golangci-lint]
        PLUG[Plugin System<br/>dynamic tools]
    end

    IDE -->|stdio| MCP
    IDE -->|stdin/stdout| HOOK
    MCP --> CS
    MCP --> SS
    MCP --> EX
    MCP --> CA
    MCP --> TD
    MCP --> GIT
    MCP --> GH
    MCP --> LINT
    HOOK --> HK
    HK --> SS
    HK --> SEC
    HK --> CS
    PLUG --> EX
```

## Component Diagram

```mermaid
graph LR
    subgraph cmd
        ROOT[root.go<br/>MCP bridge default]
        HOOKC[hook.go<br/>event dispatcher]
        SETUP[setup.go<br/>IDE configuration]
        DOC[doctor.go<br/>diagnostics]
        UPG[upgrade.go<br/>self-update]
        LINTC[lint.go<br/>golangci-lint CLI]
        HLOG[hooklog.go<br/>log viewer]
    end

    subgraph "internal (services)"
        HOOKS[hooks/<br/>HookDispatcher]
        GITOPS[gitops/<br/>13 git operations]
        GHCLI[ghcli/<br/>gh subprocess]
        LINTER[linter/<br/>golangci-lint]
        DELEGATE[delegate/<br/>TaskManager]
    end

    subgraph "internal (engines)"
        STORE[store/<br/>FTS5 ContentStore]
        SESSION[session/<br/>SessionDB]
        EXEC[executor/<br/>PolyglotExecutor]
        SECPKG[security/<br/>policy engine]
        ANALYSIS[analysis/<br/>8 parsers]
        FETCH[fetch/<br/>URL→Markdown]
    end

    subgraph "internal (support)"
        MODEL[model/<br/>domain types]
        MCPB[mcp/<br/>MCP bridge + 46 tools]
        PLAT[platform/<br/>9 IDE adapters]
        ROUTE[routing/<br/>instructions]
        PLUGPKG[plugin/<br/>JSON tool defs]
        GHAUTH[ghauth/<br/>token resolution]
        DB[db/<br/>SQLite WAL]
        PATHS[paths/<br/>platform dirs]
        HOOKLOG[hooklog/<br/>JSONL log]
        REPORT[report/<br/>auto-reports]
        TRACING[tracing/<br/>OTel setup]
        WEB[web/<br/>HTTP dashboard]
    end

    ROOT --> MCPB
    MCPB --> STORE
    MCPB --> SESSION
    MCPB --> EXEC
    MCPB --> DELEGATE
    MCPB --> GITOPS
    MCPB --> GHCLI
    MCPB --> LINTER
    MCPB --> ANALYSIS
    HOOKC --> HOOKS
    HOOKS --> SESSION
    HOOKS --> STORE
    HOOKS --> SECPKG
    LINTC --> LINTER
    HLOG --> HOOKLOG
    STORE --> DB
    SESSION --> DB
```

## Data Flow

### MCP Tool Call

```mermaid
sequenceDiagram
    participant IDE as AI Assistant
    participant MCP as MCP Bridge
    participant EX as Executor
    participant CS as ContentStore

    IDE->>MCP: CallTool(ctx_execute)
    MCP->>EX: ExecuteStream(lang, code, timeout, callback)
    EX-->>MCP: stdout/stderr chunks via callback
    EX-->>MCP: ExecResult{exitCode, timedOut}
    MCP->>CS: IndexPlainText(output, label)
    CS-->>MCP: IndexResult
    MCP-->>IDE: TextContent(filtered output)
```

### Hook Event

```mermaid
sequenceDiagram
    participant IDE as AI Assistant
    participant H as Hook CLI
    participant P as Platform Adapter
    participant HK as HookDispatcher
    participant SEC as Security
    participant SS as SessionDB

    IDE->>H: stdin(hook payload)
    H->>P: ParsePreToolUse(payload)
    P-->>H: NormalizedEvent
    H->>HK: PreToolUse(platform, payload)
    HK->>SEC: EvaluateCommand(cmd)
    SEC-->>HK: allow/deny
    HK->>SS: InsertEvent(session, event)
    HK-->>H: Response{result, blocked}
    H->>P: FormatPreToolUse(response)
    P-->>H: platform JSON
    H-->>IDE: stdout(response)
```

### Streaming Executor

```mermaid
sequenceDiagram
    participant IDE as AI Assistant
    participant MCP as MCP Bridge
    participant EX as Executor
    participant P as Subprocess

    IDE->>MCP: CallTool(ctx_execute)
    MCP->>EX: ExecuteStream(lang, code, timeout, callback)
    EX->>P: Start subprocess
    EX->>EX: Create stdout/stderr pipes

    par Scanner goroutines
        P->>EX: stdout bytes
        EX->>MCP: callback(OutputChunk{stdout})
    and
        P->>EX: stderr bytes
        EX->>MCP: callback(OutputChunk{stderr})
    end

    P->>EX: Process exits
    EX-->>MCP: ExecResult{exitCode}
    MCP->>MCP: Accumulate all chunks
    MCP-->>IDE: TextContent(combined output)
```

## Session Data Layout

```
{DataDir}/
├── sessions/
│   └── {16-char-sha256-digest}/    # per-project data
│       ├── content.db              # FTS5 knowledge base
│       ├── content.db-wal
│       ├── session.db              # session events + metadata
│       └── session.db-wal
├── plugins/                         # installed plugin definitions
├── hooklog.jsonl                    # hook interaction log
└── debug/                           # hook debug payloads

Platform-specific base:
├── Windows: %LOCALAPPDATA%\Thimble
├── macOS:   ~/Library/Application Support/thimble
└── Linux:   ~/.thimble
```

## Package Dependency Rules

Per ADR-0009, database access is allowed for service packages:

```mermaid
graph TD
    subgraph "May import DB packages"
        MCP[internal/mcp]
        HOOKS[internal/hooks]
        DELEGATE[internal/delegate]
    end

    subgraph "DB packages"
        DB[internal/db]
        STORE[internal/store]
        SESSION[internal/session]
        HOOKLOG[internal/hooklog]
    end

    subgraph "Must NOT import DB packages"
        SECURITY[internal/security]
        PLATFORM[internal/platform]
        ROUTING[internal/routing]
        FETCH[internal/fetch]
        MODEL[internal/model]
    end

    MCP --> STORE
    MCP --> SESSION
    MCP --> HOOKLOG
    HOOKS --> STORE
    HOOKS --> SESSION
    HOOKS --> HOOKLOG
    DELEGATE -.->|executor only| DB
    SECURITY -.->|blocked| DB
    PLATFORM -.->|blocked| STORE
```

This rule is enforced by `TestDBAccessGuard` in `internal/importguard_test.go`.

## Platform-Specific Files

Build-constrained files across 3 packages:

| Package | Windows | Unix | Darwin |
|---------|---------|------|--------|
| `internal/paths` | `paths_windows.go` | `paths_unix.go` | `paths_darwin.go` |
| `internal/executor` | `proc_windows.go` | `proc_unix.go` | — |
| `cmd/thimble` | `upgrade_windows.go` | `upgrade_unix.go` | — |

## External CLI Dependencies

The `internal/ghcli` package invokes `gh` as an external subprocess. The `internal/linter` package invokes `golangci-lint` as an external subprocess. No build tags are needed for either. Token resolution for direct GitHub API access is handled by `internal/ghauth/` (reads `GH_TOKEN`, `GITHUB_TOKEN`, or `~/.config/gh/hosts.yml`).

## Platform Support

| Platform | Paradigm | Hook Events | Adapter |
|----------|----------|-------------|---------|
| Claude Code | JSON-stdio hooks | PreToolUse, PostToolUse, PreCompact, SessionStart | `claude_code.go` |
| Gemini CLI | JSON-stdio hooks | BeforeTool, AfterTool, PreCompress, SessionStart | `gemini_cli.go` |
| VS Code Copilot | JSON-stdio hooks | PreToolUse, PostToolUse, PreCompact, SessionStart | `vscode_copilot.go` |
| Cursor | JSON-stdio hooks | preToolUse, postToolUse | `cursor.go` |
| OpenCode | TS plugin | N/A (MCP only) | `opencode.go` |
| Codex | MCP only | N/A | `codex.go` |
| Kiro | JSON-stdio hooks | PreToolUse, PostToolUse | `kiro.go` |
| OpenClaw | JSON-stdio hooks | PreToolUse, PostToolUse, SessionStart | `openclaw.go` |
| Antigravity | JSON-stdio hooks | PreToolUse, PostToolUse | `antigravity.go` |
