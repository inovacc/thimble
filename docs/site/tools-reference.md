# Tools Reference

Thimble exposes 120+ MCP tools: 41 native tools, ~80 GitHub API tools, and dynamic plugin tools.

## Native Tools (41)

### Knowledge Base

| Tool | Description |
|------|-------------|
| `ctx_index` | Index content into FTS5 knowledge base |
| `ctx_search` | Search knowledge base with 5-layer fallback (Porter, trigram, fuzzy, embedding, TF-IDF) |
| `ctx_stats` | Knowledge base statistics (document count, index size) |
| `ctx_fetch_and_index` | Fetch URL, convert HTML to Markdown, index result |

### Execution

| Tool | Description |
|------|-------------|
| `ctx_execute` | Execute code in 11 languages, auto-index output |
| `ctx_execute_file` | Execute code with file content injected via `FILE_CONTENT` variable |
| `ctx_batch_execute` | Run multiple commands + search queries in one call |

### Code Analysis

| Tool | Description |
|------|-------------|
| `ctx_analyze` | Parse codebase, extract symbols, index into knowledge base |
| `ctx_symbols` | Query extracted code symbols by name, kind, or package |

### Background Delegation

| Tool | Description |
|------|-------------|
| `ctx_delegate` | Submit a task for background execution (max 5 concurrent, 1MB output cap) |
| `ctx_delegate_status` | Check background task progress/result by ID |
| `ctx_delegate_cancel` | Cancel a running background task by ID |
| `ctx_delegate_list` | List all background tasks with status summary |

### Reports

| Tool | Description |
|------|-------------|
| `ctx_report_list` | List auto-generated reports |
| `ctx_report_show` | Show a specific report |
| `ctx_report_delete` | Delete a report |

### Git Operations (13 tools)

| Tool | Description |
|------|-------------|
| `ctx_git_status` | Repo status, branch, staged/unstaged changes |
| `ctx_git_diff` | Diff with context control, file filtering |
| `ctx_git_log` | Commit history with range filtering |
| `ctx_git_blame` | Per-line attribution with commit info |
| `ctx_git_branches` | List branches with upstream tracking |
| `ctx_git_stash` | List, show, save, pop, drop stashes |
| `ctx_git_commit` | Stage files, create commits with validation |
| `ctx_git_changelog` | Conventional commits changelog generation |
| `ctx_git_merge` | Merge branches with conflict detection |
| `ctx_git_rebase` | Rebase with abort/continue/skip |
| `ctx_git_conflicts` | Detect and resolve git conflicts |
| `ctx_git_validate_branch` | Validate branch naming conventions |
| `ctx_git_lint_commit` | Lint commit messages against conventions |

### GitHub CLI Tools (8 tools)

| Tool | Description |
|------|-------------|
| `ctx_gh` | Run gh CLI commands (general) |
| `ctx_gh_pr_status` | Pull request status for current branch |
| `ctx_gh_run_status` | GitHub Actions workflow run status |
| `ctx_gh_issue_list` | List repository issues |
| `ctx_gh_search` | Search issues, PRs, code across GitHub |
| `ctx_gh_api` | Raw GitHub API requests |
| `ctx_gh_repo_view` | Repository metadata and info |
| `ctx_gh_pr_template` | Get PR template for repository |

### Lint Tools (2 tools)

| Tool | Description |
|------|-------------|
| `ctx_lint` | Run golangci-lint on project/files |
| `ctx_lint_fix` | Run golangci-lint with `--fix` for auto-fixes |

### System

| Tool | Description |
|------|-------------|
| `ctx_doctor` | Health check and runtime info |
| `ctx_upgrade` | Self-update thimble binary |

## GitHub API Tools (~80)

Imported from `github-mcp-server` v0.33.0. These require `GITHUB_PERSONAL_ACCESS_TOKEN` to be set.

Coverage areas:

| Category | Examples |
|----------|---------|
| Issues | Create, read, update, list, search, comment, label |
| Pull Requests | Create, merge, review, list files, request reviewers |
| Repositories | Create, fork, list, get contents, manage branches |
| Actions | List workflows, trigger runs, download artifacts |
| Code Scanning | List alerts, get alert details |
| Dependabot | List alerts, manage security updates |
| Discussions | List, create, comment on discussions |
| Gists | Create, list, update gists |
| Projects | List, create, update project items |
| Notifications | List, mark as read |
| Labels | Create, update, delete labels |
| Security Advisories | List, create advisories |
| Stars | Star/unstar repos, list starred |
| Users/Teams | Get user info, list teams, manage membership |
| Copilot | Usage metrics and seat management |

## Dynamic Plugin Tools

Plugin tools are loaded at runtime from JSON definitions. All plugin tool names must start with `ctx_`.

Registry plugins available:

| Plugin | Tools | Description |
|--------|-------|-------------|
| docker | `ctx_docker_ps`, `ctx_docker_logs`, `ctx_docker_images`, `ctx_docker_stats` | Container management |
| kubernetes | `ctx_k8s_pods`, `ctx_k8s_logs`, `ctx_k8s_describe`, `ctx_k8s_events` | Cluster operations |
| terraform | `ctx_tf_plan`, `ctx_tf_state`, `ctx_tf_output`, `ctx_tf_validate` | Infrastructure management |

Install plugins to add more tools:

```bash
thimble plugin search           # Browse available plugins
thimble plugin install docker   # Install from registry
thimble plugin list             # Show installed plugins and their tools
```

See [Plugins](plugins.md) for details on creating custom plugins.
