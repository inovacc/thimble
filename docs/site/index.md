# Thimble

Thimble is an official Claude Code plugin -- a single-binary MCP server, hook dispatcher, and CLI toolset. No daemon, no gRPC, no discovery chain. Every instance is standalone.

## Key Features

- **120+ MCP Tools** -- 41 native tools + ~80 GitHub API tools via github-mcp-server v0.33.0 + dynamic plugin tools
- **FTS5 Knowledge Base** -- BM25-ranked search with 5-layer fallback (Porter stemming, trigram, fuzzy, embedding, TF-IDF cosine similarity)
- **Polyglot Executor** -- Run code in 11 languages (shell, Python, JS/TS, Go, Rust, Ruby, PHP, Perl, R, Elixir) with streaming output
- **Code Analysis** -- 6 language parsers (Go, Python, Rust, TypeScript, Protobuf, Shell), symbol extraction, cross-language call graphs
- **9 IDE Adapters** -- Claude Code, Gemini CLI, VS Code Copilot, Cursor, OpenCode, Codex, Kiro, OpenClaw, Antigravity
- **Hook System** -- PreToolUse, PostToolUse, SessionStart, PreCompact, UserPromptSubmit with security enforcement
- **Plugin Marketplace** -- Install community plugins from the registry, or from any URL/GitHub path
- **Session Persistence** -- Per-project event tracking, resume snapshots, priority-budgeted context
- **Security Engine** -- Bash policy enforcement, file path deny globs, shell-escape detection, git/gh dangerous command blocking
- **Git/GitHub Integration** -- 13 git operations + 8 gh CLI tools + ~80 GitHub API tools
- **Background Delegation** -- Up to 5 concurrent background tasks with 1MB output cap
- **OpenTelemetry Tracing** -- Distributed traces across MCP tool calls and hooks

## Quick Install

**Claude Code plugin (recommended):**

```bash
claude plugin install thimble@thimble
```

**Development mode:**

```bash
claude --plugin-dir /path/to/thimble
```

**Binary install (Linux/macOS):**

```bash
curl -fsSL https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.sh | bash
```

**Binary install (Windows PowerShell):**

```powershell
irm https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.ps1 | iex
```

**Go install:**

```bash
go install github.com/inovacc/thimble/cmd/thimble@latest
```

## Architecture at a Glance

```
thimble (single binary)
  |
  |-- MCP Bridge (stdio) ---- ContentStore (FTS5/SQLite)
  |   (41 native + ~80 GH)    SessionDB (events, snapshots)
  |                            PolyglotExecutor (11 langs)
  |-- Hook Dispatcher -------- Security Engine (policies)
  |   (PreToolUse/PostToolUse) CodeAnalysis (6 parsers)
  |                            TaskDelegate (background)
  |-- CLI Commands ----------- GitOps (13 operations)
  |   (lint, hooklog, doctor)  GhCli (subprocess)
  |                            Linter (subprocess)
  |-- Plugin System ---------- Report Engine
      (hot-reload, registry)   OTel Tracing
```

## Documentation

| Page | Description |
|------|-------------|
| [Getting Started](getting-started.md) | Installation methods and first configuration |
| [Tools Reference](tools-reference.md) | All 120+ MCP tools, categorized with descriptions |
| [Hooks](hooks.md) | Hook system, security policies, custom hooks |
| [Plugins](plugins.md) | Plugin system, JSON format, marketplace |
| [Platforms](platforms.md) | Supported IDE platforms and capabilities |
| [Configuration](configuration.md) | Environment variables, data directories, settings |

## License

BSD 3-Clause
