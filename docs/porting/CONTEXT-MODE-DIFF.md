# context-mode vs thimble: Logic Differences

Fresh comparison as of v0.9.1 — [context-mode](https://github.com/mksglu/context-mode) (TypeScript, single-process MCP server) vs thimble (Go, thin-client + gRPC daemon).

---

## Legend

| Status | Meaning |
|--------|---------|
| PORTED | Logic exists in thimble with equivalent behavior |
| DIFFERENT | Both have the feature but with intentionally different implementation |
| MISSING | Logic does not exist in thimble |
| N/A | Not applicable to thimble's architecture |
| DEFERRED | Explicitly deferred (see reason) |

---

## 1. Server / MCP Bridge

### 1.1 Tool Registration

| Feature | context-mode | thimble | Status |
|---------|-------------|---------|--------|
| ctx_execute | `server.ts` | `tools.go:handleExecute` | PORTED |
| ctx_execute_file | `server.ts` | `tools.go:handleExecuteFile` | PORTED |
| ctx_index | `server.ts` | `tools.go:handleIndex` | PORTED |
| ctx_search | `server.ts` | `tools.go:handleSearch` | PORTED |
| ctx_fetch_and_index | `server.ts` | `tools.go:handleFetchAndIndex` | PORTED |
| ctx_batch_execute | `server.ts` | `tools.go:handleBatchExecute` | PORTED |
| ctx_stats | `server.ts` | `tools.go:handleStats` | PORTED |
| ctx_doctor | `server.ts` (returns shell cmd) | `tools.go:handleDoctor` (direct health check) | PORTED |
| ctx_upgrade | `server.ts` (returns shell cmd) | `tools.go:handleUpgrade` (returns install cmd) | PORTED |
| ctx_analyze | N/A | `tools.go:handleAnalyze` | thimble-only |
| ctx_symbols | N/A | `tools.go:handleSymbols` | thimble-only |
| ctx_delegate{,_status,_cancel,_list} | N/A | `tools.go:handleDelegate*` | thimble-only |
| ctx_report_{list,show,delete} | N/A | `tools_report.go` | thimble-only |

### 1.2 Snippet Extraction

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | `positionsFromHighlight()` — parses FTS5 STX/ETX markers | `positionsFromHighlight()` in filter.go | PORTED |
| 2 | `extractSnippet()` — highlight-first, indexOf fallback, window merging | `extractSnippetHL()` + `extractSnippet()` | PORTED |

### 1.3 Intent Search

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | `intentSearch()` — indexes, searches, returns title-only + vocab hints | `intentPreview()` — same flow via gRPC | PORTED |
| 2 | `getDistinctiveTerms()` — IDF-scored vocabulary hints | `GetDistinctiveTerms` RPC | PORTED |

### 1.4 Fetch & Index

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | Subprocess fetch via `buildFetchCode` writing to temp file (bypasses 100KB truncation) | Go `net/http` directly (no truncation issue — Go handles large bodies) | DIFFERENT |
| 2 | Turndown + GFM plugin for HTML→Markdown | `JohannesKaufmann/html-to-markdown/v2` (pure Go) | PORTED |
| 3 | ~3KB inline preview | Same ~3KB preview | PORTED |
| 4 | Content-Type routing (json/html/plain) | Same routing | PORTED |

### 1.5 Batch Execute

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | Section inventory via `getChunksBySource` | Combined index + `GetChunksBySource` RPC | PORTED |
| 2 | Cross-source fallback with warning | Two-tier fallback with warning | PORTED |
| 3 | `getDistinctiveTerms` vocabulary hints | Via `GetDistinctiveTerms` RPC | PORTED |
| 4 | 80KB total query output cap | Same 80KB cap | PORTED |
| 5 | 3KB snippet cap per result | Same `batchSnippetCap = 3KB` | PORTED |
| 6 | `coerceCommandsArray` string→object | `batchCommand.UnmarshalJSON` | PORTED |

### 1.6 Stats Report

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | Token estimation (bytes/4) per tool | Same `~tokens` column | PORTED |
| 2 | Total data processed / kept in sandbox / entered context | Same metrics | PORTED |
| 3 | Savings ratio `Nx (Y% reduction)` | Same format | PORTED |
| 4 | Session continuity — event counts by category | `GetSnapshot` → category breakdown table | PORTED |
| 5 | Knowledge base source/chunk/code counts | Same | PORTED |

### 1.7 Session Events Auto-Index

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | `maybeIndexSessionEvents()` — scans `*-events.md` files, indexes, deletes | Thimble uses gRPC SessionDB + directives (different architecture, same goal) | DIFFERENT |

---

## 2. Executor

### 2.1 Environment

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | Start with full parent env, strip denied vars (permissive) | Build minimal env, add explicit passthrough (restrictive) | DIFFERENT (thimble more secure) |
| 2 | `INPUTRC`, `BASH_ENV`, `NODE_OPTIONS`, etc. in deny set | Same deny list + more | PORTED |
| 3 | `ERL_LIBS` denied | thimble ADDS `ERL_LIBS` for Elixir projects | DIFFERENT |
| 4 | `BASH_FUNC_*` prefix stripping | Same | PORTED |
| 5 | SSL_CERT_FILE auto-detection | Same CA cert scanning | PORTED |
| 6 | Windows: MSYS_NO_PATHCONV, Git Bash path fixes | Same | PORTED |

### 2.2 Execution

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | Background JS/TS keepalive `setInterval` | Same keepalive | PORTED |
| 2 | Elixir BEAM path injection | Via `ERL_LIBS` env var | PORTED |
| 3 | Windows Bash resolution (skip WSL, prefer Git Bash) | Simpler `LookPath` (Go handles shims) | DIFFERENT |
| 4 | `needsShell` for tsx/ts-node/elixir `.cmd` shims | Go `exec.Command` handles `.cmd` via `LookPath` | N/A |
| 5 | JS/TS network byte tracking (`__CM_NET__` markers) | Same marker protocol + `CleanNetMarkers` | PORTED |
| 6 | Hard cap (100MB) kills process | Same `cappedWriter` approach | PORTED |
| 7 | 60%/40% smart truncation | Same | PORTED |

### 2.3 Exit Classification

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | `classifyNonZeroExit()` — shell exit 1 with stdout = soft fail | `isSoftFail()` — same logic | PORTED |
| 2 | Soft fail returns stdout only (no error markers) | Resets exitCode to 0, so `formatExecOutput` omits markers | PORTED |

---

## 3. Content Store (FTS5)

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | `better-sqlite3` (native C++) | `modernc.org/sqlite` (pure Go, no CGO) | DIFFERENT |
| 2 | Prepared statement caching | `database/sql` manages prepared statement pool internally | DIFFERENT |
| 3 | Dual FTS5 tables (porter + trigram) | Same | PORTED |
| 4 | Vocabulary table + fuzzy correction | Same | PORTED |
| 5 | Dedup-on-reindex (atomic delete+insert) | Same | PORTED |
| 6 | 4-layer search fallback | Same | PORTED |
| 7 | Source-filtered search | Same via `source` proto field | PORTED |
| 8 | `highlight()` with STX/ETX markers | Same, exposed via `Highlighted` proto field | PORTED |
| 9 | `getChunksBySource` | Same via gRPC RPC | PORTED |
| 10 | `getDistinctiveTerms` (IDF scoring) | Same via gRPC RPC | PORTED |
| 11 | `listSources` | Same via gRPC RPC | PORTED |
| 12 | JSON key-path indexing | Same | PORTED |
| 13 | Plain text line-group chunking | Same | PORTED |
| 14 | Markdown heading-based chunking | Same | PORTED |

---

## 4. Security

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | Bash pattern parsing + glob→regex | Same | PORTED |
| 2 | Chained command splitting (&&, ||, ;, |) | Same | PORTED |
| 3 | Full allow/deny/ask evaluation | Deny-only (server has no UI for ask) | DIFFERENT |
| 4 | File path deny with `**` globstar | Same | PORTED |
| 5 | Shell-escape detection (7 languages) | Same patterns | PORTED |
| 6 | Python subprocess list-form extraction | Same | PORTED |

---

## 5. Lifecycle

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | Parent PID monitoring | Same in `monitorParent()` | PORTED |
| 2 | Stdin close detection | MCP transport handles stdin EOF | DEFERRED |
| 3 | Signal handling (SIGTERM, SIGINT, SIGHUP) | Same in `signal.go` | PORTED |
| 4 | TTY detection (skip lifecycle guard) | Not needed (daemon architecture) | N/A |

---

## 6. Truncation

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | `smartTruncate` (60/40 head/tail) | Same | PORTED |
| 2 | `truncateString` | Same in session `truncate()` | PORTED |
| 3 | `truncateJSON` (binary search, UTF-8 safe) | `TruncateJSON` in executor | PORTED |
| 4 | `capBytes` (binary search) | `CapBytes` in executor | PORTED |
| 5 | `escapeXML` | Not needed (plain string formatting for directives) | N/A |

---

## 7. Session / Hooks

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | `ExtractEvents` (tool-based) | Same | PORTED |
| 2 | `ExtractUserEvents` (user message) | Same, called in PostToolUse | PORTED |
| 3 | Worktree suffix for session DB | Same in `workspace.go` | PORTED |
| 4 | Session DB (events, meta, resume) | Same schema | PORTED |
| 5 | All hook dispatchers (pre/post tool, session start, pre-compact) | Same | PORTED |
| 6 | Guidance advisories | Same with `guidanceTracker` | PORTED |
| 7 | Agent/Task routing injection | Same | PORTED |

---

## 8. Platform Detection

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | 9 platform adapters | Same 9 adapters | PORTED |
| 2 | `CONTEXT_MODE_PLATFORM` override | `THIMBLE_PLATFORM` override | PORTED |
| 3 | MCP clientInfo→platform mapping | Same in `clientmap.go` | PORTED |
| 4 | Per-platform routing instructions | Same | PORTED |

---

## 9. Skills

| # | context-mode | thimble | Status |
|---|-------------|---------|--------|
| 1 | Main routing skill | Same | PORTED |
| 2 | Doctor skill | Same | PORTED |
| 3 | Stats skill | Same | PORTED |
| 4 | Upgrade skill | MCP tool instead of skill | PORTED |
| 5 | Cloud setup/status skills | `skills/cloud-setup/` + `skills/cloud-status/` | PORTED |
| 6 | Anti-pattern reference docs (JS, Python, Shell) | No equivalent (low value) | DEFERRED |

---

## Summary: Remaining Gaps

Only **intentional architectural differences** and **low-value items** remain:

### Intentional Differences (not bugs — by design)

| # | Area | context-mode | thimble | Rationale |
|---|------|-------------|---------|-----------|
| 1 | Env construction | Permissive (full env minus denies) | Restrictive (minimal env plus passthrough) | More secure |
| 2 | `ERL_LIBS` | Denied | Added for Elixir projects | Enables Mix project execution |
| 3 | SQLite driver | `better-sqlite3` (native) | `modernc.org/sqlite` (pure Go) | No CGO dependency |
| 4 | Stmt caching | Explicit prepared stmts | `database/sql` internal pool | Go idiomatic |
| 5 | Security eval | Full allow/deny/ask | Deny-only (no UI for ask) | Server-side only |
| 6 | Fetch mechanism | Subprocess to temp file | Direct `net/http` | No truncation issue in Go |
| 7 | Session events | File-based markdown → lazy index | gRPC SessionDB + directives | Tighter integration |
| 8 | Architecture | Single-process MCP server | Thin client + gRPC daemon | Crash resilience, resource sharing |

### Remaining Deferred Items

| # | Item | Reason |
|---|------|--------|
| 1 | Anti-pattern reference docs (JS, Python, Shell patterns) | Documentation-only, no code impact |

**Everything else is PORTED.**
