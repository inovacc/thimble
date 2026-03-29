# Configuration

## Environment Variables

### Core

| Variable | Description | Default |
|----------|-------------|---------|
| `CLAUDE_PLUGIN_ROOT` | Plugin root directory (set by Claude Code) | Auto-detected |
| `CLAUDE_PLUGIN_DATA` | Plugin data directory (set by Claude Code) | Auto-detected |
| `THIMBLE_PLUGIN_DATA` | Override for plugin data directory | `CLAUDE_PLUGIN_DATA` value |

### GitHub

| Variable | Description | Priority |
|----------|-------------|----------|
| `GITHUB_PERSONAL_ACCESS_TOKEN` | GitHub API token (for ~80 API tools) | 1 (highest) |
| `GH_TOKEN` | GitHub token (used by gh CLI and API tools) | 2 |
| `GITHUB_TOKEN` | GitHub token (CI/CD environments) | 3 |
| *(gh CLI auth)* | `~/.config/gh/hosts.yml` | 4 (lowest) |

### Tracing

| Variable | Description | Default |
|----------|-------------|---------|
| `THIMBLE_TRACING` | Enable OpenTelemetry tracing (`1` to enable) | Disabled |
| `OTEL_TRACES_EXPORTER` | OTel exporter type | stdout (to stderr) |

## Data Directories

### Platform-Specific Base

| OS | Base Directory |
|----|---------------|
| Windows | `%LOCALAPPDATA%\Thimble` |
| macOS | `~/Library/Application Support/thimble` |
| Linux | `~/.thimble` |

### Directory Layout

```
{DataDir}/
  sessions/
    {16-char-sha256-digest}/    # Per-project data
      content.db                # FTS5 knowledge base
      content.db-wal
      session.db                # Session events + metadata
      session.db-wal
  plugins/                      # Installed plugin definitions (user scope)
  hooklog.jsonl                 # Hook interaction log
  debug/                        # Hook debug payloads
```

### Per-Project Data

Each project gets its own session directory, identified by a 16-character SHA-256 digest of the project path. This ensures:

- No data stored inside project directories
- No conflicts between projects
- Automatic isolation of knowledge bases

## SQLite Settings

Thimble uses `modernc.org/sqlite` (pure Go, no CGO) with these defaults:

| Setting | Value | Purpose |
|---------|-------|---------|
| Journal mode | WAL | Concurrent reads during writes |
| Busy timeout | 5000ms | Wait for locks instead of failing |
| Synchronous | NORMAL | Balance between safety and speed |

### FTS5 Knowledge Base

The content store uses SQLite FTS5 with a 5-layer search fallback:

1. **Porter stemming** -- Standard full-text search with stemming
2. **Trigram** -- Character-level matching for partial words
3. **Fuzzy** -- Approximate string matching
4. **Embedding** -- Optional OpenAI-compatible vector embeddings
5. **TF-IDF cosine similarity** -- Statistical relevance scoring

Results are ranked using BM25.

### Optional Embeddings

For vector search, configure an OpenAI-compatible embedding endpoint:

| Variable | Description |
|----------|-------------|
| `THIMBLE_EMBEDDING_URL` | Embedding API endpoint |
| `THIMBLE_EMBEDDING_MODEL` | Model name |
| `THIMBLE_EMBEDDING_API_KEY` | API key |

When embeddings are not configured, the knowledge base uses the first 4 search layers only.

## Plugin Directories

| Scope | Directory | Description |
|-------|-----------|-------------|
| User | `{DataDir}/plugins/` | Default scope, available everywhere |
| Project | `{ProjectDir}/.thimble/plugins/` | Committed to VCS, shared with team |
| Local | `{ProjectDir}/.thimble/plugins.local/` | Gitignored, private to developer |

## Hook Configuration File

The hook configuration is stored in `hooks/hooks.json` within the plugin directory. It defines which tools trigger which hook events.

Key configuration points:

- **Matcher patterns** -- Regex matching tool names (empty string matches all)
- **Command** -- Uses `${CLAUDE_PLUGIN_ROOT}` for portable binary paths
- **Events** -- PreToolUse, PostToolUse, SessionStart, PreCompact, UserPromptSubmit

## Logging

All logging uses `log/slog` with JSON format, written to stderr (stdout is reserved for MCP JSON-RPC protocol messages).

Log output goes to stderr because the MCP protocol uses stdout for its JSON-RPC transport.

## Background Delegation

| Setting | Value |
|---------|-------|
| Max concurrent tasks | 5 |
| Max output per task | 1MB |

These limits are enforced by the `internal/delegate/` task manager and are not configurable via environment variables.

## Skills and Agents

Thimble ships with 8 Claude Code skills and 4 subagents:

**Skills** (in `skills/`): analyze, cloud-setup, cloud-status, doctor, remember, reports, stats, thimble

**Subagents** (in `agents/`): code-reviewer, security-auditor, test-generator, pr-creator
