# thimble

Official Claude Code plugin. Single binary: MCP server, hook dispatcher, CLI tools. No daemon, no gRPC.

Installable via `claude --plugin-dir .` or marketplace. Uses `${CLAUDE_PLUGIN_ROOT}` for self-contained paths and `${CLAUDE_PLUGIN_DATA}` (via `THIMBLE_PLUGIN_DATA` env var) for persistent storage.

## Architecture

Single-binary pattern: every instance is standalone. MCP bridge opens SQLite databases directly. No daemon spawning, no gRPC, no discovery chain.

- `cmd/` — Cobra CLI (root=MCP bridge, hook, setup, doctor, upgrade, report, selfheal, plugin [list/dir/install/remove/search/update/validate], release-notes, hooklog, lint, publish, publish-status, session [export/import/list], stats, web, version)
- `internal/mcp/` — MCP server bridge (stdio, 46 native tools + ~80 GitHub tools via import + dynamic plugin tools, tools_git.go (13 git tools), tools_gh.go (8 gh CLI tools), tools_lint.go (2 lint tools), tools_plugin.go (runtime plugin tool registration), github.go (github-mcp-server v0.33.0), observe.go (spanTool/spanHook OTel helpers), ratelimit.go (token bucket), progress.go (streaming notifications), throttling, filtering, auto-injects projectDir, signal.go)
- `internal/hooks/` — Hook dispatcher (PreToolUse/PostToolUse/SessionStart/PreCompact, security checks, guidance advisories, auto-indexing, session event recording with goal tagging, OTel hook spans)
- `internal/gitops/` — Git subprocess operations (13 operations: status, diff, log, blame, branches, stash, commit, changelog, merge, rebase, validate_branch, lint_commit, conflicts)
- `internal/ghcli/` — GitHub CLI subprocess wrapper
- `internal/linter/` — golangci-lint subprocess wrapper with symbol enrichment
- `internal/delegate/` — Background task manager (max 5 concurrent, 1MB output cap)
- `internal/plugin/` — Plugin system (JSON tool definitions, ctx_ prefix enforcement, validation, marketplace, hot-reload, dependency resolution with semver constraints, author/license/keywords metadata, scopes [user/project/local], plugin hooks [PreToolUse/PostToolUse/SessionStart/PreCompact], THIMBLE_PLUGIN_ROOT/THIMBLE_PLUGIN_DATA env vars)
- `internal/model/` — Domain types (Language, SessionEvent, ExecResult, HookInput/Output)
- `internal/store/` — ContentStore (FTS5 knowledge base, 5-layer search fallback, BM25 ranking, TF-IDF cosine similarity, optional OpenAI-compatible vector embeddings, cleanup/GC)
- `internal/session/` — SessionDB (persistent per-project, event dedup window=15, resume snapshots with configurable budget, goal-based event grouping, session directives, multi-project workspace detection, analytics queries)
- `internal/executor/` — PolyglotExecutor (11 languages, code classification, timeout handling, streaming via pipe-based stdout/stderr capture, JS/TS network byte tracking)
- `internal/security/` — Permission enforcement (Bash policies, file path deny, shell-escape detection, curl/wget/inline-HTTP detection, configurable git/gh dangerous command overrides)
- `internal/analysis/` — Code analysis (Go/Python/Rust/TypeScript/Protobuf/Shell/C/Java parsers with symbol extraction, call graph, cross-language subprocess detection)
- `internal/routing/` — PreToolUse routing instructions per platform
- `internal/platform/` — 9 IDE adapters (Claude Code, Gemini CLI, VS Code Copilot, Cursor, OpenCode, Codex, Kiro, OpenClaw, Antigravity)
- `internal/report/` — Auto-report generation (doctor, stats — AI-consumable markdown with embedded JSON)
- `internal/hooklog/` — Append-only JSONL logging for hook interactions (write/read/filter/clear)
- `internal/tracing/` — OpenTelemetry tracing setup (stdout exporter to stderr + OTLP gRPC for Jaeger/Tempo, opt-in via `THIMBLE_TRACING=1` or `OTEL_EXPORTER_OTLP_ENDPOINT`)
- `internal/web/` — HTTP web server (dashboard UI, API routes, dark theme, auto-refresh)
- `internal/fetch/` — URL fetch + HTML→Markdown
- `internal/ghauth/` — GitHub token resolution (GH_TOKEN, GITHUB_TOKEN, ~/.config/gh/hosts.yml)
- `internal/db/` — SQLite WAL mode init (modernc.org/sqlite)
- `internal/paths/` — Platform-aware directory helpers (per-project data stored under `{DataDir}/sessions/{sha256-digest}`, not inside project dirs; darwin-specific paths via `paths_darwin.go`)
- `skills/` — 8 Claude Code skills (analyze, cloud-setup, cloud-status, doctor, remember, reports, stats, thimble)
- `hooks/` — Hook dispatcher config (hooks.json)
- `agents/` — 4 Claude Code subagents (code-reviewer, security-auditor, test-generator, pr-creator)
- `.claude-plugin/` — Plugin manifests (plugin.json, marketplace.json)
- `assets/plugin/` — Embedded plugin assets for `setup --plugin` deployment

## Commands

```bash
task build              # Build binary
task test               # Run tests
task lint               # golangci-lint
go run ./cmd/thimble --help                    # CLI help (default: MCP server on stdio)
go run ./cmd/thimble hook <platform> <event>   # Dispatch hook event (in-process)
go run ./cmd/thimble setup --client claude     # Configure hooks for IDE (legacy; prefer claude --plugin-dir)
claude --plugin-dir .                          # Load as Claude Code plugin (development)
claude plugin install thimble@thimble          # Install from marketplace
go run ./cmd/thimble doctor                    # Run diagnostic checks
go run ./cmd/thimble upgrade                   # Self-update from GitHub Releases
go run ./cmd/thimble hooklog                   # Show hook interaction logs
go run ./cmd/thimble hooklog --blocked         # Show only blocked hooks
go run ./cmd/thimble hooklog --debug           # Show full payloads
go run ./cmd/thimble hooklog --clear           # Clear the hook log
go run ./cmd/thimble lint                      # Run golangci-lint
go run ./cmd/thimble lint --fix                # Run golangci-lint with auto-fix
go run ./cmd/thimble plugin list               # List installed plugins and their tools
go run ./cmd/thimble plugin install <source>   # Install plugin from registry, URL, or GitHub path
go run ./cmd/thimble plugin remove <name>      # Remove an installed plugin
go run ./cmd/thimble plugin search             # Browse available plugins from the registry
go run ./cmd/thimble plugin dir                # Show the plugins directory path
go run ./cmd/thimble plugin update             # Update all installed plugins from registry
go run ./cmd/thimble plugin update docker      # Update specific plugin
go run ./cmd/thimble plugin validate path.json # Validate a plugin definition
go run ./cmd/thimble release-notes             # Generate release notes from git changelog
go run ./cmd/thimble publish                  # Commit, tag, push, and monitor CI pipeline
go run ./cmd/thimble publish-status            # Check publish/release pipeline status
```

### ctx_delegate (background task delegation)

The MCP bridge exposes 4 delegation tools (ADR-0007):
- `ctx_delegate` — Submit a task for background execution (max 5 concurrent, 1MB output cap)
- `ctx_delegate_status` — Check task progress/result by ID
- `ctx_delegate_cancel` — Cancel a running task by ID
- `ctx_delegate_list` — List all tasks with status summary

The MCP bridge exposes 2 goal tagging tools:
- `ctx_set_goal` — Set the active goal tag; all subsequent events are tagged for resume grouping
- `ctx_clear_goal` — Clear the active goal tag

## Conventions

- Module: `github.com/inovacc/thimble`
- Session data: `{DataDir}/sessions/{sha256-digest}/` (content.db + session.db)
- SQLite: `modernc.org/sqlite` (pure Go, no CGO), WAL mode, busy_timeout=5000
- Logging: `log/slog` with JSON to stderr
- Errors: `fmt.Errorf("context: %w", err)`
- Import guard: `internal/mcp/`, `internal/hooks/`, `internal/delegate/` may import DB packages; restricted packages may not
