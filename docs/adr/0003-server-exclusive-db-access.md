# ADR-0003: Server Has Exclusive Database Access

## Status
Accepted

## Context
Thimble uses SQLite in WAL mode as its persistence layer (FTS5 knowledge base, session store). SQLite supports only one writer at a time. Multiple processes opening the same database leads to WAL contention, lock timeouts, and corruption risk.

The thin-client/daemon architecture (ADR-0002) naturally centralizes all long-lived state in the server process. Clients (CLI, MCP bridge, hooks) are short-lived and stateless.

## Decision
**Only the server process may open or access the database.** All other components must go through gRPC.

### Enforcement
1. **Import guard** — A test scans `internal/client/`, `internal/mcp/`, `internal/routing/`, `internal/platform/`, and `cmd/` to verify they never import `internal/db`, `internal/store`, `internal/session`, or `modernc.org/sqlite`.
2. **Package layout** — Database packages (`internal/db/`, `internal/store/`, `internal/session/`) are only imported by `internal/server/`.
3. **Code review** — Any new gRPC service method that needs data must add it to the server, not bypass via direct DB access.

### Allowed import graph
```
cmd/           → internal/client/    → gRPC → internal/server/ → internal/db/
internal/mcp/  → internal/client/    → gRPC → internal/server/ → internal/store/
                                              internal/server/ → internal/session/
```

### Forbidden
```
cmd/           ✗→ internal/db/
cmd/           ✗→ internal/store/
cmd/           ✗→ internal/session/
internal/mcp/  ✗→ internal/db/
internal/client/ ✗→ internal/db/
```

## Consequences

### Positive
- No WAL contention — single writer, single process
- Clean API boundary — clients never see SQL, schemas, or migrations
- Testability — mock gRPC service, not database
- Crash isolation — client crash cannot corrupt the database

### Negative
- Every new data operation requires a gRPC method (more boilerplate)
- Cannot do quick ad-hoc queries from CLI without server running
- Server becomes a bottleneck for all data access (mitigated by WAL read concurrency within the single process)
