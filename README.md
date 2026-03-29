[English](README.md) | [Português](docs/README.pt-BR.md) | [Español](docs/README.es.md)

# thimble

A single-binary MCP plugin for AI coding assistants. Provides an FTS5 knowledge base, polyglot code execution, session persistence, and security enforcement — all in-process, no daemon required.

## Features

- **MCP Server** — stdio transport with 46 native tools + ~80 GitHub API tools + dynamic plugins (execute, search, index, fetch, analyze, batch, delegate, reports, git, gh, lint, goals, workspace, shared knowledge)
- **Single Binary** — every instance is standalone; no daemon, no gRPC, no discovery chain
- **FTS5 Knowledge Base** — BM25-ranked search with 5-layer fallback (Porter, trigram, fuzzy, embedding, TF-IDF)
- **Polyglot Executor** — 11 languages (shell, Python, JS/TS, Go, Rust, Ruby, PHP, Perl, R, Elixir)
- **Code Analysis** — 8 parsers (Go, Python, Rust, TypeScript, Protobuf, Shell, C, Java), symbol extraction, cross-language call graphs
- **Git Integration** — 13 git MCP tools (status, diff, log, blame, branches, stash, commit, changelog, merge, rebase, conflicts, validate_branch, lint_commit) + git-aware security policies
- **GitHub Integration** — 8 gh CLI subprocess tools (incl. PR templates) + ~80 GitHub API tools via github-mcp-server import
- **Lint Integration** — golangci-lint v2 via subprocess (requires `golangci-lint` on PATH), auto-fix support
- **Plugin Marketplace** — Install community plugins from the [registry](https://github.com/inovacc/thimble-plugins) (`thimble plugin install docker`), or from any URL/GitHub path. JSON-based tool definitions with template command substitution.
- **Session Persistence** — per-project event tracking, resume snapshots, priority-budgeted context
- **Security** — Bash policy enforcement, file path deny globs, shell-escape detection, git/gh dangerous command blocking
- **9 IDE Adapters** — Claude Code, Gemini CLI, VS Code Copilot, Cursor, OpenCode, Codex, Kiro, OpenClaw, Antigravity
- **Auto-Reports** — Doctor and stats reports in AI-consumable markdown
- **OpenTelemetry Tracing** — Distributed traces across MCP tool calls

## Installation

### Claude Code Plugin (recommended)

```bash
# One-time setup: point npm to GitHub Packages for @inovacc scope
echo @inovacc:registry=https://npm.pkg.github.com >> ~/.npmrc

# Install and register
npm install -g @inovacc/thimble
claude plugin install thimble@npm:@inovacc/thimble
```

**Windows (PowerShell):**
```powershell
Add-Content "$env:USERPROFILE\.npmrc" "@inovacc:registry=https://npm.pkg.github.com"
npm install -g @inovacc/thimble
claude plugin install thimble@npm:@inovacc/thimble
```

### Other Methods

**Linux / macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.ps1 | iex
```

**Go install:**
```bash
go install github.com/inovacc/thimble/cmd/thimble@latest
```

Or download a pre-built binary from [Releases](https://github.com/inovacc/thimble/releases).

**Optional external tools:** `golangci-lint` (for lint tools) and `gh` CLI (for gh tools) must be installed separately if you want to use those features. Both degrade gracefully if not found on PATH.

## Quick Start

### 1. Configure hooks for your IDE

```bash
# Plugin mode (Claude Code — recommended)
claude plugin install thimble@npm:@inovacc/thimble

# Or legacy setup with auto-detection
thimble setup

# Or specify explicitly
thimble setup --client claude
thimble setup --client gemini
thimble setup --client cursor
```

### 2. Use as MCP server (default)

```bash
# Runs MCP server on stdio (used by IDE hook config)
thimble
```

### 3. Diagnostics

```bash
thimble doctor              # Run diagnostic checks
thimble hooklog             # View hook interaction logs
thimble hooklog --blocked   # Show only blocked hooks
```

## Commands

| Command | Description |
|---------|-------------|
| *(default)* | Run MCP server on stdio |
| `hook <platform> <event>` | Dispatch hook event (in-process, ~10ms) |
| `setup --client <name>` | Configure hooks for IDE |
| `doctor` | Run diagnostic checks |
| `report [list\|show\|delete]` | Manage auto-generated reports |
| `upgrade` | Self-update from GitHub Releases |
| `lint [--fix] [--fast]` | Run golangci-lint |
| `plugin list` | List installed plugins and their tools |
| `plugin install <source>` | Install plugin from registry, URL, or GitHub path |
| `plugin remove <name>` | Remove an installed plugin |
| `plugin search` | Browse available plugins from the registry |
| `plugin dir` | Show the plugins directory path |
| `plugin update [name]` | Update plugins from registry (`--check` for dry run) |
| `hooklog [--blocked] [--clear]` | Show hook interaction logs |
| `release-notes` | Generate release notes from git changelog |
| `publish` | Commit, tag, push, and monitor CI pipeline |
| `publish-status` | Check publish/release pipeline status |
| `version` | Print version information |

## MCP Tools

### Native Tools (46)

| Tool | Description |
|------|-------------|
| `ctx_execute` | Execute code in 11 languages, auto-index output |
| `ctx_execute_file` | Execute code with file content via `FILE_CONTENT` variable |
| `ctx_index` | Index content into FTS5 knowledge base |
| `ctx_search` | Search knowledge base with 5-layer fallback |
| `ctx_fetch_and_index` | Fetch URL, convert HTML to Markdown, index |
| `ctx_batch_execute` | Run multiple commands + search queries in one call |
| `ctx_stats` | Knowledge base statistics |
| `ctx_doctor` | Health check and runtime info |
| `ctx_analyze` | Parse codebase, extract symbols, index into knowledge base |
| `ctx_symbols` | Query extracted code symbols by name, kind, or package |
| `ctx_delegate` | Submit background task for async execution |
| `ctx_delegate_status` | Check background task progress/result |
| `ctx_delegate_cancel` | Cancel a running background task |
| `ctx_delegate_list` | List all background tasks with status |
| `ctx_report_list` | List auto-generated reports |
| `ctx_report_show` | Show a specific report |
| `ctx_report_delete` | Delete a report |
| `ctx_git_status` | Repo status, branch, staged/unstaged changes |
| `ctx_git_diff` | Diff with context control, file filtering |
| `ctx_git_log` | Commit history with range filtering |
| `ctx_git_blame` | Per-line attribution with commit info |
| `ctx_git_branches` | List branches with upstream tracking |
| `ctx_git_stash` | List, show, save, pop, drop stashes |
| `ctx_git_commit` | Stage files, create commits with validation |
| `ctx_git_changelog` | Conventional commits changelog generation |
| `ctx_gh` | Run gh CLI commands |
| `ctx_gh_pr_status` | Pull request status for current branch |
| `ctx_gh_run_status` | GitHub Actions workflow run status |
| `ctx_gh_issue_list` | List repository issues |
| `ctx_gh_search` | Search issues, PRs, code across GitHub |
| `ctx_gh_api` | Raw GitHub API requests |
| `ctx_gh_repo_view` | Repository metadata and info |
| `ctx_gh_pr_template` | Get PR template for repository |
| `ctx_git_merge` | Merge branches with conflict detection |
| `ctx_git_rebase` | Rebase with abort/continue/skip |
| `ctx_git_conflicts` | Detect and resolve git conflicts |
| `ctx_git_validate_branch` | Validate branch naming conventions |
| `ctx_git_lint_commit` | Lint commit messages against conventions |
| `ctx_lint` | Run golangci-lint on project/files |
| `ctx_lint_fix` | Run golangci-lint with --fix for auto-fixes |
| `ctx_set_goal` | Set active goal tag for event grouping |
| `ctx_clear_goal` | Clear the active goal tag |
| `ctx_workspace_info` | Multi-project workspace detection |
| `ctx_session_insights` | Session analytics (top tools, error rate, duration) |
| `ctx_plugin_conflicts` | Detect plugin tool name conflicts |
| `ctx_upgrade` | Self-update thimble binary |

### GitHub API Tools (~80)

Imported from `github-mcp-server` v0.33.0 — covers issues, PRs, repos, actions, code scanning, Dependabot, discussions, gists, projects, notifications, labels, security advisories, stars, users/teams, and Copilot. Requires `GITHUB_PERSONAL_ACCESS_TOKEN`.

### Dynamic Plugin Tools

Install community plugins from the [plugin registry](https://github.com/inovacc/thimble-plugins) or any URL:

```bash
# Browse available plugins
thimble plugin search

# Install from registry (by name)
thimble plugin install docker
thimble plugin install kubernetes
thimble plugin install terraform

# Install from GitHub
thimble plugin install github.com/user/repo/my-plugin.json

# Install from URL
thimble plugin install https://example.com/plugin.json

# Manage
thimble plugin list              # show installed
thimble plugin update            # update all from registry
thimble plugin update docker     # update specific plugin
thimble plugin remove docker     # uninstall
```

**Available registry plugins:**

| Plugin | Tools | Description |
|--------|-------|-------------|
| **docker** | `ctx_docker_ps`, `ctx_docker_logs`, `ctx_docker_images`, `ctx_docker_stats` | Container management |
| **kubernetes** | `ctx_k8s_pods`, `ctx_k8s_logs`, `ctx_k8s_describe`, `ctx_k8s_events` | Cluster operations |
| **terraform** | `ctx_tf_plan`, `ctx_tf_state`, `ctx_tf_output`, `ctx_tf_validate` | Infrastructure management |

**Create your own plugin** — see the [plugin authoring guide](https://github.com/inovacc/thimble-plugins#create-your-own-plugin).

## Development

```bash
task build    # Build binary
task test     # Run tests
task lint     # golangci-lint
task release  # GoReleaser (requires git tag)
```

## Architecture

```
thimble (single binary)
  |
  |-- MCP Bridge (stdio) ---- ContentStore (FTS5/SQLite)
  |   (46 native + ~80 GH)    SessionDB (events, snapshots)
  |                            PolyglotExecutor (11 langs)
  |-- Hook Dispatcher -------- Security Engine (policies)
  |   (PreToolUse/PostToolUse) CodeAnalysis (8 parsers)
  |                            TaskDelegate (background)
  |-- CLI Commands ----------- GitOps (13 operations)
  |   (lint, hooklog, doctor)  GhCli (subprocess)
  |                            Linter (subprocess)
  |-- Plugin System ---------- Report Engine
      (hot-reload, registry)   OTel Tracing
```

## License

BSD 3-Clause
