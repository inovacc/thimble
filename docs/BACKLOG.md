# Backlog

## Priority Levels

| Priority | Timeline |
|----------|----------|
| P1 | This sprint |
| P2 | This quarter |
| P3 | Future |

## Items

### P1 — This Sprint

_No P1 items — all cleared._

### P2 — This Quarter

- ~~**Push to GitHub**~~ — Done. Repo at `github.com/inovacc/thimble`, branch protection configured, v4.0.0 tagged.

- ~~**npm package**~~ — Done. `npm i -g @inovacc/thimble` via GitHub Packages with binary download, checksum verification, Claude Code plugin structure for `claude plugin install`.

- ~~**CI/CD pipeline fixes**~~ — Done. Fixed 15+ lint issues, Windows test timeout, Docker build, release workflow.

- ~~**Documentation site**~~ — Done. 7 pages in `docs/site/` + MkDocs Material config.

### P3 — Future

- **Test coverage to 80%+** — 21/24 packages at 80%+. Gaps: cmd/thimble (51.3%), mcp (54.6%). Web (90.5%) and ghauth (85.2%) now above target.

- ~~**Wire plugin deps into install**~~ — Done. `InstallToScope` auto-installs transitive dependencies via `ResolveDependencies`.

- ~~**Wire security overrides in hooks**~~ — Done. PreToolUse + gh tools use `CustomPoliciesFromSettings` for user-configurable git/gh blocking.

- ~~**Grafana dashboard JSON**~~ — Done. 5-panel dashboard + Jaeger/Grafana docker-compose provisioning.

- ~~**Session goal tagging**~~ — Done. `ctx_set_goal`/`ctx_clear_goal` tools, companion events, goal-based resume grouping.

### Completed

- **Phase 21 features** — OTLP tracing, plugin dependencies, configurable security, streaming progress, session improvements, docs deployment, ADR-0010.
  - Completed: 2026-03-28

- **Phase 20 polish** — Shell parser, CI fixes, docs site, test coverage, Windows CRLF fix.
  - Completed: 2026-03-28

- **Claude Code plugin conversion** — Official plugin structure with `${CLAUDE_PLUGIN_ROOT}` paths, `${CLAUDE_PLUGIN_DATA}` persistent storage, GoReleaser plugin archives, deprecation of legacy setup deploy.
  - Completed: 2026-03-27

- **CI/CD pipeline** — GitHub Actions for build, test, lint (ci.yml), GoReleaser release (release.yaml), Docker multi-platform build (docker.yml).
  - Completed: 2026-03-26

- **Install scripts** — `scripts/install.sh` (curl|bash) and `scripts/install.ps1` (irm|iex) for Linux/macOS/Windows.
  - Completed: 2026-03-23 (v4.0.0)

- **Version bump to v4.0.0** — Breaking change: single-binary, no daemon, session path change.
  - Completed: 2026-03-23

- **Test suite repair** — Fixed all broken tests after v4 rearchitecture. 19/19 packages pass, 0 lint issues.
  - Completed: 2026-03-23

- **Kubernetes manifests** — Helm chart or Kustomize overlays for running thimble as a sidecar.
  - Completed: 2026-03-19 (Phase 15, v2.5.0)


- **Plugin marketplace** — `thimble plugin install/remove/search` from registry. Install from name, URL, or GitHub path. JSON-based registry at github.com/inovacc/thimble-plugins.
  - Completed: 2026-03-19 (Phase 14, v2.3.0)

- **Plugin hot-reload** — fsnotify file watcher on plugins directory triggers dynamic re-registration without daemon restart.
  - Completed: 2026-03-19 (Phase 14, v2.3.0)

- **Release notes CLI** — `thimble release-notes` generates changelog from conventional commits between tags.
  - Completed: 2026-03-19 (v2.3.0)

- **Git conflict resolution** — `ctx_git_conflicts` parses conflict markers (<<<<<<< / ======= / >>>>>>>) with diff3 ancestor support. ListConflicts RPC detects merge vs rebase.
  - Completed: 2026-03-19 (Phase 14)

- **Plugin system** — Runtime tool registration from JSON definitions in `{DataDir}/plugins/`. `ctx_` prefix enforcement, built-in conflict detection, `thimble plugin list/dir` CLI.
  - Completed: 2026-03-19 (Phase 14)

- **Git workflow automation** — Branch naming validation (`ctx_git_validate_branch`), commit message linting (`ctx_git_lint_commit`), PR template generation (`ctx_gh_pr_template`).
  - Completed: 2026-03-19 (Phase 14)

- **`ctx_git_rebase` MCP tool** — Interactive rebase planning, conflict detection.
  - Completed: 2026-03-19 (Phase 12)

- **golangci-lint import** — Import `github.com/golangci/golangci-lint/v2` as Go dependency. `ctx_lint` + `ctx_lint_fix` MCP tools running in-process via `pkg/commands.Execute()`. Structured issue parsing, auto-index results.
  - Completed: 2026-03-19 (Phase 13)

- **`thimble lint` CLI command** — Run golangci-lint via thin client gRPC. Complements MCP tools for direct CLI use.
  - Completed: 2026-03-19 (Phase 13)

- **GitHub CLI (gh) subprocess** — gh invoked as subprocess (replaced in-process `cli/cli/v2` import). Auth read from `GH_TOKEN` env or `~/.config/gh/hosts.yml` via `internal/ghauth`. 7 MCP tools: ctx_gh, ctx_gh_pr_status, ctx_gh_run_status, ctx_gh_issue_list, ctx_gh_search, ctx_gh_api, ctx_gh_repo_view. 14 dangerous gh subcommands denied by default.
  - Completed: Phase 12

- **Git commit + changelog** — `ctx_git_commit` (stage files, create commits with message validation), `ctx_git_changelog` (conventional commits parsing, grouped markdown). Commit/Changelog RPCs added to Git gRPC service.
  - Completed: Phase 12

- **Git gRPC service + MCP tools** — 8 RPCs (Status, Diff, Log, Blame, BranchList, Stash, Commit, Changelog). 8 MCP tools with auto-indexing. Git security policies (13 dangerous subcommands denied). Pushed to GitHub, CI running.
  - Completed: Phase 12

- **GitHub MCP Server integration** — Imported `github.com/github/github-mcp-server` v0.33.0. Glue code in `internal/mcp/github.go` registers 80 GitHub tools (issues, PRs, repos, actions, code scanning, Dependabot, discussions, gists, projects, notifications, labels, security advisories, stars, users/teams, Copilot) via `inventory.RegisterAll()`. Auth via `GITHUB_PERSONAL_ACCESS_TOKEN`, GHE support via `GITHUB_HOST`. Optional — gracefully skipped if no token.
  - Completed: Phase 12

- **ContentStore cleanup/GC** — CleanupByAge (30-day default), CleanupStale, Vacuum, MostRecentIndexedAt. Periodic background sweep (24h interval, 7-day threshold). Cleanup RPC in server services. 5 tests.
  - Completed: Phase 10

- **ctx_delegate tool** — ADR-0007: delegate long-running tasks to background agents with progress tracking. 4 MCP tools (ctx_delegate, ctx_delegate_status, ctx_delegate_cancel, ctx_delegate_list), in-memory task manager, max 5 concurrent, 1MB output cap, context cancellation.
  - Completed: Phase 9

- **Per-project content stores** — Isolate FTS5 knowledge bases per project directory instead of global. Registry pattern with lazy-init, MCP bridge auto-injects projectDir.
  - Completed: Phase 9

- **P0 migration fixes** — UserPromptSubmit hook, tool name aliasing (`mcp__thimble__`), XML routing block with fully-qualified tool names, docs/MIGRATION.md.
  - Completed: Phase 9

- **Intent-based snippet extraction** — `intent` parameter on ctx_search for sliding-window snippet extraction (~500 chars around best term density).
  - Completed: v0.3.0

- **Error classification** — `explain_errors` on ctx_execute/ctx_execute_file/ctx_batch_execute with 12 error categories.
  - Completed: v0.3.0

- **Monitor/supervisor pattern** — Crash-resilient process supervision with restart loop, exponential backoff (1s-2m), crash window reset (60s), max 5 consecutive restarts, JSON-lines crash log.
  - Completed: Phase 8

- **Client discovery tests** — 20 unit tests for 3-step discovery chain, singleton, and probeHealth. Refactored with injectable discoveryDeps.
  - Completed: Phase 8

- **Streaming gRPC** — Server-streaming RPCs (ExecuteStream/ExecuteFileStream) with pipe-based stdout/stderr capture, OutputChunk callback, MCP bridge chunk accumulation.
  - Completed: Phase 8
