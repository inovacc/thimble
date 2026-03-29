# Roadmap

## Current Status
**Overall Progress:** Phase 26 complete — 120+ MCP tools (46 native + ~80 GitHub API + dynamic plugins), official Claude Code plugin, single-binary architecture, OpenTelemetry observability, 8 language parsers (Go/Python/Rust/TypeScript/Protobuf/Shell/C/Java), 25 packages, v4.6.0

## Phases

### Phase 1: Foundation [COMPLETE]
- [x] Project scaffold (structure, tooling, CI config)
- [x] gRPC proto definition (6 services: ContentStore, Session, Executor, Hooks, Security, CodeAnalysis)
- [x] Server: TCP listener, idle tracker, graceful shutdown, singleton guard (file lock)
- [x] Server info registration (server.json with PID/port/addr)
- [x] Client: 4-step discovery chain, health check, auto-start

### Phase 2: Thin Client [COMPLETE]
- [x] CLI commands forwarded via gRPC
- [x] Platform-specific process detachment (Windows/Unix)
- [x] Singleton client with lazy init
- [x] Auto-lifecycle daemon (spawned on demand, idle shutdown, no manual start/stop)

### Phase 3: MCP Bridge [COMPLETE]
- [x] MCP server bridge (stdio → gRPC) with 10 tools
- [x] Hook dispatcher (PreToolUse, PostToolUse, SessionStart, PreCompact)
- [x] IDE platform adapters (Claude Code, Gemini CLI, VS Code Copilot, Cursor, OpenCode, Codex)
- [x] Hook setup command with deep merge into settings files
- [x] Routing instructions generator

### Phase 4: Knowledge Base [COMPLETE]
- [x] SQLite FTS5 content store with BM25 ranking
- [x] 4-layer search fallback (Porter AND/OR → Trigram AND/OR → Fuzzy)
- [x] Session persistence with event dedup and FIFO eviction
- [x] Resume snapshot builder with priority budget tiers
- [x] Polyglot executor (11 languages, code classification, timeout handling)
- [x] Security/permission enforcement (Bash policies, file path deny, shell-escape detection)

### Phase 5: Code Analysis [COMPLETE]
- [x] 5 language parsers (Go, Python, Rust, TypeScript, Protobuf)
- [x] Symbol extraction (functions, types, interfaces, methods, constants)
- [x] Go reference/call graph extraction
- [x] File summaries, dependency graphs, Mermaid architecture maps
- [x] CodeAnalysis gRPC service (Analyze, Symbols, References)

### Phase 6: Polish & Hardening [COMPLETE]
- [x] ADR-0003 import guard (server-exclusive DB access)
- [x] Zombie process prevention (GracefulStop timeout, TestMain guards)
- [x] Legacy code port to new architecture
- [x] End-to-end integration tests (9 E2E tests: ListTools, Execute, IndexAndSearch, etc.)
- [x] Build-time version embedding (ldflags in Taskfile + GoReleaser)
- [x] Setup command integration tests (5 tests: config build, merge, format)
- [x] Self-update command (thimble upgrade) with SHA256 verification
- [x] golangci-lint clean pass (0 issues)

### Phase 7: Release [COMPLETE]
- [x] GoReleaser configuration (cross-platform builds, ldflags, checksums)
- [x] Architecture documentation (Mermaid diagrams)
- [x] README.md (install, usage, tools, architecture)
- [x] CLAUDE.md (updated for all 17 packages)
- [x] Taskfile dev tasks (dev, server:*, setup, doctor)
- [x] v0.2.0 tagged
- [x] GitHub Actions CI/CD pipeline (ci.yml, release.yaml, docker.yml)
- [ ] Push to github.com/inovacc/thimble

### Phase 8: Reliability & Streaming [COMPLETE]
- [x] Monitor/supervisor pattern (crash-resilient restart loop, exponential backoff 1s-2m, crash window reset 60s, max 5 consecutive restarts, JSON-lines crash log)
- [x] Client discovery chain test coverage (20 tests, injectable deps via discoveryDeps, all 3 discovery steps + singleton + probeHealth)
- [x] Streaming gRPC executor (server-streaming RPCs ExecuteStream/ExecuteFileStream, pipe-based stdout/stderr capture, OutputChunk callback, MCP bridge chunk accumulation)

### Phase 9: Migration Readiness [COMPLETE]
- [x] Per-project content stores (project isolation, registry pattern)
- [x] ctx_delegate background task delegation (ADR-0007, 4 MCP tools)
- [x] P0 migration gaps (UserPromptSubmit hook, tool name aliasing, XML routing block)
- [x] Migration plan document (docs/MIGRATION.md)

### Phase 10: Data Lifecycle [COMPLETE]
- [x] ContentStore cleanup/GC (CleanupByAge 30-day default, CleanupStale, Vacuum, MostRecentIndexedAt)
- [x] Periodic background cleanup goroutine (24h interval, 7-day stale threshold)
- [x] Cleanup RPC in server services (30-day age cleanup + vacuum)
- [x] CleanupStaleStores removes dead project stores

### Phase 11: Reports & Distribution [COMPLETE]
- [x] Auto-report system (doctor, crash, stats — AI-consumable markdown with embedded JSON)
- [x] CLI report management (list, show, delete)
- [x] MCP report tools (ctx_report_list, ctx_report_show, ctx_report_delete)
- [x] `--report` flag on `doctor` and `crashes` commands
- [x] Restructured binary entry point (`cmd/thimble/`)
- [x] Install scripts (`scripts/install.sh`, `scripts/install.ps1`)
- [x] npm package: `npm i -g @inovacc/thimble` (wraps binary, suggests plugin-dir or legacy setup)
- [x] GitHub Actions CI/CD pipeline (ci.yml, release.yaml, docker.yml)
- [ ] Push to github.com/inovacc/thimble

### Phase 12: GitHub & Git Integration [COMPLETE]
- [x] GitHub MCP Server import — 80 GitHub tools registered via glue code (`internal/mcp/github.go`)
- [x] Inventory-based registration (`RegisterAll` on thimble's MCP server)
- [x] Auth via `GITHUB_PERSONAL_ACCESS_TOKEN` env var (optional, graceful skip)
- [x] GitHub Enterprise support via `GITHUB_HOST` env var
- [x] Deps middleware injection (REST, GraphQL, Raw clients)
- [x] Git operation tracking in session events (16 git command patterns detected via PostToolUse)
- [x] Incremental code analysis via `git diff --name-only HEAD`
- [x] Git gRPC service — 6 RPCs (Status, Diff, Log, Blame, BranchList, Stash) in `internal/server/services_git.go`
- [x] `ctx_git_status` MCP tool — repo status, branch, staged/unstaged changes, untracked files
- [x] `ctx_git_diff` MCP tool — diff with context control, file filtering, stat summary
- [x] `ctx_git_log` MCP tool — commit history with author, date, message, range filtering
- [x] `ctx_git_blame` MCP tool — per-line attribution with commit info
- [x] `ctx_git_branches` MCP tool — list branches with upstream tracking, ahead/behind counts
- [x] `ctx_git_stash` MCP tool — list, show, save, pop, drop stashes
- [x] Git-aware security policies — dangerous git subcommands denied by default (force push, reset --hard, clean -f, etc.)
- [x] Auto-index git diff/log/blame output into knowledge base for context-aware code review
- [x] `ctx_git_commit` MCP tool — stage files, create commits with message validation
- [x] `ctx_git_changelog` MCP tool — conventional commits parsing, grouped markdown changelog
- [x] GitHub CLI (gh) subprocess execution (replaced in-process `cli/cli/v2` dependency)
- [x] GhCli gRPC service — subprocess gh execution with graceful error handling
- [x] 7 gh MCP tools: ctx_gh, ctx_gh_pr_status, ctx_gh_run_status, ctx_gh_issue_list, ctx_gh_search, ctx_gh_api, ctx_gh_repo_view
- [x] gh security policies — 14 dangerous gh subcommands denied by default
- [x] `ctx_git_merge` MCP tool — merge branches with conflict detection and resolution hints
- [x] `ctx_git_rebase` MCP tool — rebase with abort/continue/skip, conflict detection

### Phase 13: Lint & Code Quality Integration [COMPLETE]
- [x] Import `github.com/golangci/golangci-lint/v2` (v2.11.3) as Go dependency (in-process, no binary required)
- [x] Lint gRPC service — `commands.Execute()` with captured stdout/stderr
- [x] `ctx_lint` MCP tool — run golangci-lint on project/files, auto-index results into knowledge base
- [x] `ctx_lint_fix` MCP tool — run with `--fix` flag for auto-fixable issues
- [x] Lint result parsing — structured issue extraction (file, line, column, linter, message, severity)
- [x] `thimble lint` CLI command — run lint via thin client with --fix, --fast, --enable, --timeout flags
- [x] Integration with code analysis — cross-reference lint issues with symbol/call graph data

### Phase 14: Plugin System [COMPLETE]
- [x] `internal/plugin/` — JSON-based plugin definition loader (`PluginDef`, `ToolDef`, `InputFieldDef`)
- [x] Plugin directory: `{DataDir}/plugins/` — scans for `*.json` tool definitions at startup
- [x] Plugin validation: `ctx_` prefix enforcement, required fields, invalid JSON skipped with warning
- [x] `internal/mcp/tools_plugin.go` — dynamic MCP tool registration from plugin definitions
- [x] `text/template` command substitution with `{{.fieldname}}` placeholders, missing fields handled gracefully
- [x] Security: plugin commands run through executor gRPC service with deny-policy enforcement
- [x] Built-in tool override protection — plugins cannot shadow built-in MCP tool names
- [x] Auto-index plugin tool output into knowledge base
- [x] `thimble plugin list` CLI command — list installed plugins and their tools
- [x] `thimble plugin dir` CLI command — show the plugins directory path
- [x] Plugin marketplace — `thimble plugin install/remove/search` from [registry](https://github.com/inovacc/thimble-plugins)
- [x] Plugin hot-reload — fsnotify file watcher triggers dynamic re-registration without restart
- [x] `thimble release-notes` CLI command — generate release notes from git changelog
- [x] Plugin E2E integration tests
- [x] ADR-0008: imported CLI tools as Go dependencies

### Phase 15: Publish, Build Constraints & Darwin Paths [COMPLETE]
- [x] `thimble publish` CLI command — commit, tag, push, and monitor CI pipeline flow
- [x] `thimble publish-status` CLI command — check publish/release pipeline status
- [x] Auto version bump in publish command
- [x] 75 plugin definitions across 12 categories in thimble-plugins registry
- [x] Kubernetes manifests + golangci-lint auto-fix across 125 files
- [x] Platform-specific code split into build-constrained files (13 files: 6 _windows.go, 6 _unix.go, 1 _darwin.go)
- [x] Both gh CLI and golangci-lint now use subprocess execution (no build tags needed)
- [x] `thimble plugin update` CLI command — update plugins from registry with `--check` dry-run
- [x] Darwin-specific paths (`paths_darwin.go`)
- [x] Zero lint issues — all resolved
- [x] Test coverage push

### Phase 16: Dependency Internalization [COMPLETE]
- [x] Drop `golangci-lint/v2` Go import — now subprocess execution (requires `golangci-lint` on PATH)
- [x] Drop `cli/cli/v2` Go import — `gh` already subprocess since Phase 12
- [x] Remove `nolint`/`nogh` build tags (no longer needed)
- [x] Binary size reduced: 101 MB → 28 MB stripped
- [x] Module count reduced: 709 → 132 transitive modules
- [x] `internal/ghauth/` package for standalone GitHub token resolution
- [x] Docker multi-platform build fix (QEMU + cross-compile via TARGETARCH)
- [x] v3.0.0 released (PR #1), v3.0.1 Docker fix

### Phase 17: Single-Binary Architecture [COMPLETE]
- [x] Remove gRPC daemon — every instance is standalone (ADR-0009)
- [x] Remove `internal/client/` (gRPC client, discovery chain, auto-start)
- [x] Remove `internal/server/` (gRPC daemon, supervisor, lockfile, idle tracker, 11 service wrappers)
- [x] Remove `proto/v1/` (Protobuf definitions) and `pkg/api/v1/` (generated stubs)
- [x] Remove `service` command (hidden daemon) and `crashes` command (supervisor crash log)
- [x] Remove all `google.golang.org/grpc` and `google.golang.org/protobuf` dependencies
- [x] Extract `internal/hooks/` — hook dispatcher (PreToolUse/PostToolUse/SessionStart/PreCompact)
- [x] Extract `internal/gitops/` — 13 git subprocess operations
- [x] Extract `internal/ghcli/` — GitHub CLI wrapper
- [x] Extract `internal/linter/` — golangci-lint wrapper with symbol enrichment
- [x] Extract `internal/delegate/` — background task manager
- [x] Rewire MCP bridge — direct service references instead of gRPC stubs
- [x] Rewire hook CLI — in-process hook dispatch instead of gRPC dial
- [x] Rewire lint CLI — direct linter package call
- [x] Rewire hooklog CLI — direct hooklog package access
- [x] Rename `{DataDir}/projects/` → `{DataDir}/sessions/` with one-time migration
- [x] Update import guard (ADR-0003) — allow mcp/hooks/delegate to import DB packages

### Phase 18: Observability [COMPLETE]
- [x] `spanTool` helper — OTel spans with attributes, error recording, duration, status codes
- [x] `spanHook` helper — OTel spans for hook dispatch events
- [x] Tracing init in MCP bridge — opt-in via `THIMBLE_TRACING=1`, stdout exporter to stderr
- [x] All 41 native MCP tool handlers instrumented with `spanTool`
- [x] Upgraded 5 bare spans (execute, search, index, fetch, batch) to rich `spanTool` spans
- [x] Tracing shutdown wired into `Bridge.Close()`

### Phase 19: Claude Code Plugin Conversion [COMPLETE]
- [x] Official Claude Code plugin structure (`.claude-plugin/plugin.json`, `hooks/hooks.json`, `.mcp.json`, `skills/`)
- [x] `${CLAUDE_PLUGIN_ROOT}` variable paths in hooks and MCP server config (self-contained plugin)
- [x] `${CLAUDE_PLUGIN_DATA}` persistent data support via `THIMBLE_PLUGIN_DATA` env var in `internal/paths/`
- [x] Plugin manifest enriched with `homepage`, `hooks`, and `mcpServers` component paths
- [x] GoReleaser plugin archive — per-platform archives bundling binary + all plugin assets
- [x] `scripts/plugin-check.sh` — SessionStart binary verification
- [x] Deprecation notice on `thimble setup --client claude --plugin` (prefer `claude --plugin-dir` or marketplace)
- [x] Embedded `assets/plugin/` preserved for non-Claude-Code platforms (Gemini, Cursor, VS Code Copilot)

### Phase 20: Polish & Documentation [COMPLETE]
- [x] Shell parser for code analysis (functions, constants, exports, aliases, source imports)
- [x] Branch protection on main (require "build" status check, no force pushes)
- [x] CI/CD pipeline fixes (15+ lint issues, Windows test timeout, Docker build, release workflow)
- [x] Test coverage improvements (linter 29→97%, MCP 20 integration tests, gitops 22 tests, cmd 75 tests)
- [x] Documentation site (`docs/site/` — 7 pages + MkDocs Material config)
- [x] Windows CRLF fix (parseConflictMarkers, git helper stdout capture)
- [x] golangci-lint v2.11.3, zero issues

### Phase 21: Features & Extensibility [COMPLETE]
- [x] OTLP gRPC exporter for Jaeger/Grafana Tempo (auto-detects `OTEL_EXPORTER_OTLP_ENDPOINT`)
- [x] `docker-compose.observability.yml` with Jaeger all-in-one
- [x] Plugin dependency resolution (`SatisfiesConstraint`, `ResolveDependenciesDeep`, cycle detection)
- [x] Configurable security policies (`DangerousGitOverrides`/`DangerousGhOverrides` in settings.json)
- [x] Streaming progress reporter (`ProgressReporter` interface, MCP `notifications/progress`)
- [x] Session improvements (dedup window 5→15, configurable budget via `THIMBLE_SNAPSHOT_BUDGET`, goal-based grouping)
- [x] GitHub Pages deployment workflow (MkDocs Material)
- [x] ADR-0010: Claude Code plugin conversion

### Phase 22: Integration & Wiring [COMPLETE]
- [x] Wire plugin dependency resolution into `thimble plugin install` (auto-installs transitive deps)
- [x] Wire configurable security overrides into PreToolUse hooks and gh tools
- [x] Goal tagging MCP tools (`ctx_set_goal`/`ctx_clear_goal`) with companion event recording
- [x] Grafana dashboard (5 panels: latency, call count, error rate, hook timing, timeline) + provisioning
- [x] 14 MCP git integration tests (status, diff, log, branches)
- [x] 4 E2E plugin dependency resolution tests

### Phase 23: Ecosystem & Hardening [COMPLETE]
- [x] C/C++ parser (functions, structs, enums, typedefs, defines, includes — `.c/.h/.cpp/.hpp/.cc/.cxx`)
- [x] MCP rate limiting (token bucket, `THIMBLE_RATE_LIMIT`/`THIMBLE_RATE_BURST` env vars)
- [x] Multi-project workspace detection (go.work, pnpm-workspace, VS Code) + `ctx_workspace_info`
- [x] Session analytics (`ctx_session_insights` — events by type, top tools, error rate, duration)
- [x] Semver-aware plugin updates (`ClassifyUpdate`, `--major` flag, breaking change warnings)
- [x] 10 plugin hook E2E tests (all events, failure handling, multi-plugin order)
- [x] 38 broad coverage tests across 7 packages (84%+ overall)
- [x] CONTRIBUTING.md (dev setup, PR process, plugin dev, code style)
- [x] Cross-platform migration test fix (HOME on Unix, LOCALAPPDATA on Windows)

### Phase 24: Ecosystem & Developer Experience [COMPLETE]
- [x] Java/Kotlin parser (.java/.kt/.kts) with classes, interfaces, methods, enums, annotations
- [x] Plugin init scaffolding (`thimble plugin init <name>`)
- [x] Plugin conflict detection (`DetectConflicts`, `ctx_plugin_conflicts`, annotated `plugin list`)
- [x] Stats CLI command (`thimble stats --json --report`)
- [x] Session export/import (`thimble session export/import/list`)
- [x] Benchmark suite (store, session, analysis, executor — 10 benchmarks)
- [x] i18n README translations (Portuguese, Spanish)

### Phase 25: Architecture & Quality [COMPLETE]
- [x] Plugin sandboxing (command allowlists, path deny, timeout cap, DefaultSandbox)
- [x] LSP client integration (JSON-RPC 2.0 stdio, opt-in via `THIMBLE_LSP_GOPLS=1`, regex fallback)
- [x] Web UI dashboard (`thimble web`, embedded HTML, 4 API routes, dark theme, auto-refresh)
- [x] Plugin testing framework (`thimble plugin test`, 5 check categories, dry-run execution)
- [x] Content chunking strategies (paragraph/section/sliding, `THIMBLE_CHUNK_STRATEGY` env var)
- [x] Skill authoring guide (`docs/site/skill-authoring.md`)
- [x] Diff-aware incremental analysis (file cache, mod-time comparison, git diff pruning)
- [x] Cross-session knowledge sharing (`ctx_shared_index/search/list`, lazy shared store)

### Phase 26: Hardening & Protocol [COMPLETE]
- [x] Fuzz testing for all 8 language parsers
- [x] Plugin per-tool permission scopes
- [x] MCP resource providers (protocol-compliant resources)
- [x] Plugin marketplace web UI
- [x] Embedding model auto-detection
- [x] Event-driven plugin hooks (pub/sub event bus)
- [x] Session diff viewer in web UI (API: `/api/session/diff`)
