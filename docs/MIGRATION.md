# Migration: context-mode → thimble

## Overview

Replace the context-mode Node.js MCP plugin (v1.0.18) with thimble (Go MCP plugin).

**Key architectural shift**: context-mode is an ephemeral Node.js process (temp DB per PID). Thimble is a persistent gRPC daemon with per-project stores.

---

## 1. Feature Parity Matrix

### MCP Tools

| context-mode | thimble | Status | Gap |
|---|---|---|---|
| `ctx_execute` | `ctx_execute` | ✅ Parity | `intent` + `explain_errors` added in v0.3.0 |
| `ctx_execute_file` | `ctx_execute_file` | ✅ Parity | Param rename: `path` → `file_path` |
| `ctx_index` | `ctx_index` | ✅ Parity | — |
| `ctx_search` | `ctx_search` | ✅ Parity | `intent` snippet extraction added in v0.3.0 |
| `ctx_fetch_and_index` | `ctx_fetch_and_index` | ✅ Parity | — |
| `ctx_batch_execute` | `ctx_batch_execute` | ✅ Parity | Per-query `intent` added in v0.3.0 |
| `ctx_stats` | `ctx_stats` | ✅ Parity | Missing: `runtimes`, context savings % |
| `ctx_doctor` | `ctx_doctor` | ✅ Parity | Different output format |
| `ctx_upgrade` | N/A | ❌ Missing | CLI-only (`thimble upgrade`), no MCP tool |
| N/A | `ctx_analyze` | ✨ New | Code analysis (5 languages) |
| N/A | `ctx_symbols` | ✨ New | Symbol query |
| N/A | `ctx_delegate` | ✨ New | Background task delegation |
| N/A | `ctx_delegate_status` | ✨ New | Task status polling |
| N/A | `ctx_delegate_cancel` | ✨ New | Task cancellation |
| N/A | `ctx_delegate_list` | ✨ New | List delegated tasks |

### Hooks

| Hook | context-mode | thimble | Gap |
|---|---|---|---|
| PreToolUse | 9 separate matchers | 1 pipe-delimited matcher | Functionally equivalent |
| PostToolUse | ✅ Catch-all | ✅ Catch-all | — |
| PreCompact | ✅ Catch-all | ✅ Catch-all | — |
| SessionStart | ✅ Catch-all | ✅ Catch-all | — |
| UserPromptSubmit | ✅ Catch-all | ✅ Catch-all | Added in Phase 9 |

### Platform Support

| Platform | context-mode | thimble |
|---|---|---|
| Claude Code | Full | Full |
| Gemini CLI | Full | Full |
| VS Code Copilot | Full | Full |
| Cursor | Full | Full |
| OpenCode | Partial | MCP-only |
| Codex | Full | MCP-only |

### Database Schema

Schemas are **identical** between both systems:
- `sources`: `id, label, chunk_count, code_chunk_count, indexed_at`
- `chunks` FTS5: `title, content, source_id, content_type` (porter unicode61)
- `chunks_trigram` FTS5: same columns (trigram tokenizer)
- `vocabulary`: `word TEXT PRIMARY KEY`
- Session tables (`session_events`, `session_meta`, `session_resume`): identical

---

## 2. Breaking Changes

### 2.1 Fully-Qualified Tool Names Change
```
BEFORE: mcp__plugin_context-mode_context-mode__ctx_execute
AFTER:  mcp__thimble__ctx_execute
```
All CLAUDE.md routing blocks, hook matchers, and user habits referencing the old prefix will break.

### 2.2 Input Parameter Differences
- `ctx_execute_file`: `path` → `file_path`
- `ctx_execute`: lost `intent`, `explain_errors` params
- `ctx_search`: lost `intent` param (affects snippet quality)

### 2.3 Storage Lifecycle Change
- **Before**: Ephemeral temp DB (`/tmp/context-mode-{pid}.db`), fresh every session
- **After**: Persistent per-project DB, accumulates knowledge across sessions
- **Impact**: Better knowledge retention, but needs cleanup/GC for stale data

### 2.4 Architecture Change
- **Before**: Single Node.js process (MCP + hooks in same process)
- **After**: gRPC daemon + thin CLI + MCP bridge (3-component architecture)
- **Impact**: Daemon must be running (auto-started), hooks are gRPC calls (~5ms vs ~50ms Node.js cold start)

---

## 3. Gaps to Close Before Migration

### P0 — Must Fix (all resolved)
1. ~~**UserPromptSubmit hook**~~: ✅ Added in Phase 9
2. ~~**Tool name aliasing**~~: ✅ Added in Phase 9
3. ~~**Routing block format**~~: ✅ Added in Phase 9

### P1 — Should Fix
4. **`ctx_upgrade` MCP tool**: Wrap CLI upgrade as MCP tool (still open)
5. ~~**`intent` parameter**~~: ✅ Added in v0.3.0
6. ~~**`explain_errors`**~~: ✅ Added in v0.3.0
7. ~~**ContentStore cleanup/GC**~~: ✅ Added in Phase 10

### P2 — Nice to Have
8. **Build tool interception**: Redirect gradle/maven commands to sandbox
9. **Context savings %**: Add to `ctx_stats` output
10. **`runtimes` in stats**: List available language runtimes

---

## 4. Data Migration

### ContentStore
**No migration needed.** context-mode's content DB is ephemeral (temp file, dies with process). Thimble starts fresh and accumulates knowledge persistently.

### SessionDB
**Direct copy possible.** Identical schemas.
```
SOURCE: ~/.claude/context-mode/session.db
TARGET: ~/.claude/thimble/sessions/session.db
```
Verify target path from `internal/platform/claude_code.go`.

### What Is Lost
- Current-session indexed content (always lost when context-mode exits anyway)
- Session events file pipeline (thimble uses gRPC instead)

---

## 5. Configuration Migration

### ~/.claude/settings.json

**Remove:**
```json
"enabledPlugins": { "context-mode@context-mode": true }
"extraKnownMarketplaces": { "context-mode": { ... } }
```

**Add (via `thimble setup claude-code`):**
- Hook entries: PreToolUse, PostToolUse, PreCompact, SessionStart
- Hook commands: `thimble hook claude-code <event>`

### MCP Server Config

**Add to project `.mcp.json`:**
```json
{ "mcpServers": { "thimble": { "command": "thimble", "args": [] } } }
```

### Routing Instructions
- Re-run `thimble setup claude-code --instructions`
- Remove stale `mcp__plugin_context-mode_context-mode__*` references from CLAUDE.md files
- Update `~/.claude/CLAUDE.md` `<context_window_protection>` block

### Permissions/Deny Rules
**No changes.** Thimble reads the same 3-tier settings chain.

---

## 6. Rollback Plan

### Pre-Migration Backup
```bash
cp ~/.claude/settings.json ~/.claude/settings.json.pre-thimble
cp -r ~/.claude/context-mode/ ~/.claude/context-mode.backup/
```

### Rollback Steps
1. Restore `~/.claude/settings.json` from backup
2. Remove thimble MCP config from `.mcp.json`
3. Re-enable context-mode: `"context-mode@context-mode": true`
4. Restart Claude Code

**Do NOT run both simultaneously** — hook conflicts cause double-processing.

---

## 7. Migration Procedure

### Prerequisites
- thimble binary built and on PATH
- Claude Code session closed

### Steps

1. **Build thimble**: `task build` → verify `thimble --version`
2. **Backup**: Copy `~/.claude/settings.json` and `~/.claude/context-mode/`
3. **Disable context-mode**: Set `"context-mode@context-mode": false` in settings
4. **Run setup**: `thimble setup claude-code --instructions`
5. **Add MCP server**: Create/update `.mcp.json` with thimble entry
6. **Migrate sessions** (optional): Copy session.db to thimble's session dir
7. **Clean routing**: Remove old `mcp__plugin_context-mode_context-mode__*` refs from CLAUDE.md files
8. **Update global CLAUDE.md**: Replace `<context_window_protection>` block tool references
9. **Test**: Open Claude Code, verify `ctx_execute`, `ctx_search`, `ctx_stats`, hooks
10. **Clean up**: After 2-3 successful sessions, remove backups

### Post-Migration Gaps to Address
- [x] Add UserPromptSubmit hook to setup (Phase 9)
- [ ] Add ctx_upgrade MCP tool
- [x] Add intent-based snippet extraction (v0.3.0)
- [x] Add explain_errors classification (v0.3.0)
- [x] Add tool name aliasing for cross-platform (Phase 9)
- [x] Add ContentStore cleanup/GC (Phase 10)
