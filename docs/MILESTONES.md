# Milestones

## v0.1.0 - Foundation + Thin Client
- **Status:** Complete
- **Goals:**
  - [x] Project scaffolding
  - [x] gRPC proto definition (6 services)
  - [x] Server with idle timeout, graceful shutdown, singleton guard
  - [x] Client with 4-step discovery chain and auto-start
  - [x] CLI forwarding to gRPC daemon

## v0.2.0 - MCP Bridge + Knowledge Base
- **Status:** Complete
- **Goals:**
  - [x] MCP server bridge (stdio → gRPC) with 10 tools
  - [x] Hook dispatcher (PreToolUse, PostToolUse, PreCompact, SessionStart)
  - [x] 6 IDE platform adapters
  - [x] SQLite FTS5 content store with BM25 ranking
  - [x] Session persistence with event dedup
  - [x] Polyglot executor (11 languages)
  - [x] Security/permission enforcement
  - [x] Code analysis (5 parsers)
  - [x] GoReleaser config, README, CLAUDE.md, architecture docs

## v0.3.0 - Reliability & Streaming
- **Status:** Complete
- **Goals:**
  - [x] Monitor/supervisor pattern (crash-resilient restart loop)
  - [x] Streaming gRPC executor (ExecuteStream/ExecuteFileStream)
  - [x] Client discovery chain tests (20 tests)
  - [x] Intent-based snippet extraction
  - [x] Error classification (12 categories)

## v0.4.0 - Migration Readiness
- **Status:** Complete
- **Goals:**
  - [x] Per-project content stores (project isolation)
  - [x] ctx_delegate background task delegation (ADR-0007)
  - [x] P0 migration gaps (UserPromptSubmit, tool aliasing, XML routing)
  - [x] Migration plan document

## v0.5.0 - Data Lifecycle + Branding
- **Status:** Complete
- **Goals:**
  - [x] ContentStore cleanup/GC (CleanupByAge, CleanupStale, Vacuum)
  - [x] Periodic background cleanup (24h interval)
  - [x] OpenTelemetry tracing across CLI, daemon, MCP bridge
  - [x] Brand assets and Windows icon embedding

## v0.6.0 - Plugin System + Reports
- **Status:** Complete
- **Goals:**
  - [x] Auto-report system (doctor, crash, stats)
  - [x] MCP report tools (ctx_report_list/show/delete)
  - [x] Claude Code plugin (manifests, 6 skills, hooks)
  - [x] Embedded plugin deployment (`setup --plugin`)
  - [x] Pushed to github.com/inovacc/thimble (private)

## v0.7.0 - context-mode Security & UX Parity
- **Status:** Complete
- **Goals:**
  - [x] Executor hardening (env deny list, 100MB cap, process group kill, CA certs, Elixir BEAM, .cmd shims)
  - [x] MCP bridge parity (search throttling, 40KB cap, batch timeout, bridge-side security)
  - [x] Session events file auto-indexing
  - [x] Distribution scripts (install.sh/install.ps1, npm package scaffold, marketplace launchers)

## v0.8.0 - Final context-mode Parity
- **Status:** Complete
- **Goals:**
  - [x] MCP clientInfo → platform detection (InitializedHandler)
  - [x] 3 new platform adapters (Kiro, OpenClaw, Antigravity — total 9)
  - [x] OpenClaw workspace-to-session mapping
  - [x] SessionStart directive injection for resume/compact
  - [x] Batch execute network byte tracking
  - [x] Self-heal, guidance advisories, signal handling, netdetect
  - [x] Agent/Task routing uses detected platform
  - [x] cmd/thimble coverage 55.6% → 77.0%

## v0.9.0 - context-mode Porting + Coverage
- **Status:** Complete
- **Goals:**
  - [x] Port 12 context-mode features (Wave 1-3: INPUTRC, THIMBLE_PLATFORM, keepalive, ctx_upgrade, batch coercion, content-type routing, UserPromptSubmit, Source/ListSources proto, intent filtering, rich stats)
  - [x] Port high-priority porting gaps (FTS5 highlight snippets, GetDistinctiveTerms/GetChunksBySource RPCs, fetch preview, cross-source fallback, intent title-only + vocab hints, net marker cleanup)
  - [x] Coverage push: store 88.3%, analysis 88.3%, executor 80.3%, platform 99.2%, mcp 80.0%
  - [x] Comprehensive porting doc (docs/porting/CONTEXT-MODE-DIFF.md)

## v1.0.0 - First Stable Release
- **Status:** Complete
- **Goals:**
  - [x] Test coverage 80%+ across most packages — 16/18 at 80%+; only `cmd/thimble` (77.0%) below ceiling
  - [x] GitHub Actions CI/CD pipeline (Build, CI, Test — green)
  - [x] Distribution (install scripts, npm package scaffold, marketplace launchers)
  - [x] Full context-mode feature parity (only anti-pattern reference docs deferred)
  - [x] 8 Claude Code skills (analyze, cloud-setup, cloud-status, doctor, remember, reports, stats, thimble)
  - [x] Comprehensive porting doc (docs/porting/CONTEXT-MODE-DIFF.md)
  - [x] Docker image (distroless base) — shipped in v1.1.0
  - [ ] Publish npm package to registry — deferred to v1.3.0

## v1.1.0 - Docker + TF-IDF Semantic Search
- **Status:** Complete
- **Goals:**
  - [x] Docker image (multi-stage, distroless runtime, .dockerignore)
  - [x] TF-IDF cosine similarity as search layer 5 (concept matching)
  - [x] Tokenization with camelCase/underscore expansion for identifiers

## v1.2.0 - Cross-Language Call Graphs + Embedding Search
- **Status:** Complete
- **Goals:**
  - [x] Cross-language call graph detection (Go, Python, TypeScript, Shell, Rust subprocess patterns)
  - [x] LangShell support in analyzer (.sh/.bash files)
  - [x] Optional embedding-based semantic search (THIMBLE_EMBEDDING_URL)
  - [x] SQLite chunk_embeddings table for vector storage
  - [x] Graceful TF-IDF fallback when embedding API not configured

## v1.2.1 - Hook Deep Tests + Benchmarks
- **Status:** Complete
- **Goals:**
  - [x] 18 hook E2E integration tests (PreCompact, UserPromptSubmit, tool name normalization, deny paths)
  - [x] 21 ContentStore benchmarks (index, search, semantic, vocabulary, stats)
  - [x] Server coverage 81.8% → 82.6%

## v1.3.1 - Hook Formatting + Security Refinements
- **Status:** Complete
- **Goals:**
  - [x] Fix `formatViaAdapter` dropping server responses (type-switch on `"result"` key)
  - [x] Convert hard-deny security blocks (WebFetch, curl/wget, build tools) to soft advisories
  - [x] Docker CI workflow + progressive batch search
  - [x] GitHub MCP Server integration (~80 tools via `github-mcp-server` v0.33.0)

## v2.0.0 - Git & GitHub Integration
- **Status:** Complete
- **Goals:**
  - [x] Git gRPC service — 8 RPCs (Status, Diff, Log, Blame, BranchList, Stash, Commit, Changelog)
  - [x] 8 git MCP tools with auto-indexing and git-aware security policies
  - [x] GitHub CLI — subprocess execution (replaced in-process `cli/cli/v2` dependency)
  - [x] GhCli gRPC service — 7 MCP tools (ctx_gh, pr_status, run_status, issue_list, search, api, repo_view)
  - [x] 14 dangerous gh subcommands denied by default
  - [x] 10 gRPC services total, 33 native MCP tools + ~80 GitHub API tools

## v2.2.0 - Git Workflow, Lint & Plugin Integration
- **Status:** Complete
- **Goals:**
  - [x] Phases 12-14 complete (Git/GitHub, Lint, Plugin)
  - [x] 5 new git tools: ctx_git_merge, ctx_git_rebase, ctx_git_conflicts, ctx_git_validate_branch, ctx_git_lint_commit
  - [x] 1 new gh tool: ctx_gh_pr_template
  - [x] Lint gRPC service — golangci-lint v2 imported as Go dependency (in-process)
  - [x] 2 lint MCP tools: ctx_lint, ctx_lint_fix
  - [x] `thimble lint` CLI command with --fix, --fast, --enable, --timeout flags
  - [x] Plugin system — JSON-based dynamic tool definitions from {DataDir}/plugins/
  - [x] `thimble plugin list` and `thimble plugin dir` CLI commands
  - [x] 41 native MCP tools + ~80 GitHub API tools + dynamic plugins, 11 gRPC services, 21 packages
  - [x] Test coverage: 72.7% overall

## v2.3.0 - Plugin Marketplace, Hot-Reload & Release Notes
- **Status:** Complete
- **Goals:**
  - [x] Plugin marketplace — install/remove/search from [registry](https://github.com/inovacc/thimble-plugins)
  - [x] Plugin hot-reload — fsnotify file watcher, dynamic re-registration without restart
  - [x] `thimble release-notes` CLI command — generate release notes from git changelog
  - [x] Plugin E2E integration tests
  - [x] ADR-0008: imported CLI tools as Go dependencies
  - [x] All lint errors fixed
  - [x] 22 packages, test coverage: 71.7% overall

## v2.5.0 - Publish CLI, Plugin Marketplace & Kubernetes
- **Status:** Complete
- **Goals:**
  - [x] `thimble publish` CLI command — commit, tag, push, and monitor CI pipeline flow
  - [x] `thimble publish-status` CLI command — check pipeline status
  - [x] Auto version bump in publish command
  - [x] 75 plugin definitions across 12 categories in thimble-plugins registry
  - [x] Kubernetes manifests + golangci-lint auto-fix across 125 files
  - [x] Binary size optimization

## v2.6.0 - Zero Lint, Darwin Paths & Build Constraints
- **Status:** Complete
- **Goals:**
  - [x] Zero lint issues — all resolved across codebase
  - [x] Platform-specific code split into build-constrained files (13 files: 6 _windows.go, 6 _unix.go, 1 _darwin.go)
  - [x] Both gh CLI and golangci-lint now use subprocess execution (no build tags needed)
  - [x] `thimble plugin update` CLI command — update plugins from registry with `--check` dry-run
  - [x] Darwin-specific paths (`paths_darwin.go`)
  - [x] Test coverage push

## v3.0.0 - Dependency Internalization
- **Status:** Complete
- **Goals:**
  - [x] Drop `golangci-lint/v2` and `cli/cli/v2` Go imports — both now subprocess
  - [x] Remove `nolint`/`nogh` build tags (no longer needed)
  - [x] Binary size: 101 MB → 28 MB stripped
  - [x] Module count: 709 → 132 transitive modules
  - [x] `internal/ghauth/` package for standalone GitHub token resolution
  - [x] PR #1 merged

## v3.0.1 - Docker Multi-Platform Fix
- **Status:** Complete
- **Goals:**
  - [x] Docker multi-platform build fix (QEMU + cross-compile via TARGETARCH)

## v4.0.0 - Single-Binary Architecture + Claude Code Plugin
- **Status:** Complete
- **Goals:**
  - [x] Single-binary architecture — remove gRPC daemon (ADR-0009)
  - [x] OpenTelemetry observability for all MCP tool handlers
  - [x] Plugin manifest metadata, validate command, env vars, lifecycle hooks
  - [x] Official Claude Code plugin conversion (`${CLAUDE_PLUGIN_ROOT}` paths, `${CLAUDE_PLUGIN_DATA}` persistent storage)
  - [x] GoReleaser plugin archive — per-platform bundles with binary + plugin assets
  - [x] Deprecation notice on legacy `setup --plugin` for Claude Code
  - [x] 120+ MCP tools (41 native + ~80 GitHub API + dynamic plugins), 22 packages

## v4.1.0 - Features & Extensibility (Phase 21)
- **Status:** Complete
- **Goals:**
  - [x] OTLP gRPC exporter for Jaeger/Grafana Tempo
  - [x] Plugin dependency resolution (semver constraints, cycle detection)
  - [x] Configurable security policies (DangerousGitOverrides/DangerousGhOverrides)
  - [x] Streaming progress reporter (MCP notifications/progress)
  - [x] Session improvements (dedup window 15, configurable budget, goal-based grouping)

## v4.2.0 - Integration & Wiring (Phase 22)
- **Status:** Complete
- **Goals:**
  - [x] Goal tagging MCP tools (ctx_set_goal/ctx_clear_goal)
  - [x] Grafana dashboard (5 panels) + provisioning
  - [x] Wire plugin deps into install, security overrides into hooks
  - [x] 14 MCP git integration tests

## v4.3.0 - Ecosystem & Hardening (Phase 23)
- **Status:** Complete
- **Goals:**
  - [x] C/C++ parser (functions, structs, enums, typedefs, defines, includes)
  - [x] MCP rate limiting (token bucket)
  - [x] Multi-project workspace detection (ctx_workspace_info)
  - [x] Session analytics (ctx_session_insights)
  - [x] CONTRIBUTING.md

## v4.4.0 - Developer Experience (Phase 24)
- **Status:** Complete
- **Goals:**
  - [x] Java/Kotlin parser (.java/.kt/.kts)
  - [x] Plugin init scaffolding, conflict detection
  - [x] Stats CLI, session export/import
  - [x] Benchmark suite (10 benchmarks)
  - [x] i18n README translations (Portuguese, Spanish)

## v4.5.0 - Architecture & Quality (Phase 25)
- **Status:** Complete
- **Goals:**
  - [x] Plugin sandboxing (command allowlists, path deny, timeout cap)
  - [x] Web UI dashboard (thimble web, 4 API routes)
  - [x] Content chunking strategies (paragraph/section/sliding)
  - [x] Diff-aware incremental analysis
  - [x] Cross-session knowledge sharing (ctx_shared_index/search/list)

## v4.6.0 - Hardening & Protocol (Phase 26)
- **Status:** Complete
- **Goals:**
  - [x] Fuzz testing for all 8 language parsers
  - [x] Plugin per-tool permission scopes
  - [x] MCP resource providers (protocol-compliant)
  - [x] Plugin marketplace web UI
  - [x] Embedding model auto-detection
  - [x] Event-driven plugin hooks (pub/sub event bus)
  - [x] 46 native MCP tools, 25 packages, 8 parsers
