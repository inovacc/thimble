# ADR-0009: Remove gRPC Daemon — Single-Binary Architecture

## Status

Accepted (2026-03-23)

## Context

Thimble used a thin-client/gRPC-daemon pattern since ADR-0002:

- The MCP bridge (default command) auto-spawned a background gRPC daemon on first use
- All tool calls were proxied from MCP (stdio) → gRPC (TCP) → daemon services
- The hook CLI discovered and dialed the daemon for every hook event
- Infrastructure included: 4-step discovery chain, singleton file lock, supervisor with crash recovery (exponential backoff), idle timeout with keepAlive pings, server.json state file, port scanning (50351-50355)

The daemon's value proposition was persistence across IDE restarts — the daemon could hold databases open and survive MCP bridge restarts. In practice:

1. **SQLite WAL mode already handles concurrent access** — multiple processes can safely read/write the same database with busy_timeout
2. **The complexity cost was high** — 40+ files in `internal/server/`, 7 files in `internal/client/`, protobuf definitions, generated stubs, supervisor, lockfile (platform-specific), discovery chain
3. **gRPC was 90% transport** — most services were thin wrappers calling existing packages (store, session, executor, analysis)
4. **Binary size and dependencies** — gRPC + protobuf added significant transitive dependencies

## Decision

Remove the gRPC daemon entirely. Every thimble instance is standalone:

- MCP bridge opens SQLite databases directly (content.db, session.db)
- Hook CLI opens databases per-invocation (lazy, only if needed)
- No daemon spawning, no discovery, no port scanning, no singleton lock
- Business logic extracted into standalone packages callable without RPC

## Consequences

### Removed
- `internal/client/` — gRPC client, discovery chain, auto-start, singleton
- `internal/server/` — gRPC daemon, supervisor, lockfile, idle tracker, 11 service wrappers
- `proto/v1/` — Protobuf service definitions
- `pkg/api/v1/` — Generated gRPC stubs
- `service` command (hidden daemon entry point)
- `crashes` command (supervisor crash log)
- All `google.golang.org/grpc` and `google.golang.org/protobuf` dependencies

### Added
- `internal/hooks/` — Hook dispatcher (extracted from server/services.go)
- `internal/gitops/` — Git subprocess operations (extracted from server/services_git.go)
- `internal/ghcli/` — GitHub CLI wrapper (extracted from server/services_gh.go)
- `internal/linter/` — golangci-lint wrapper (extracted from server/services_lint.go)
- `internal/delegate/` — Background task manager (extracted from server/delegate.go)

### Changed
- `internal/mcp/bridge.go` — Direct service references instead of gRPC stubs
- `internal/paths/paths.go` — `projects/` renamed to `sessions/` with one-time migration
- ADR-0003 import guard updated — `internal/mcp/`, `internal/hooks/`, `internal/delegate/` may now import DB packages
- Hook CLI latency: ~5ms (gRPC to running daemon) → ~15ms (SQLite open per invocation). Acceptable.

### Supersedes
- ADR-0002 (thin-client-daemon pattern)
- ADR-0003 partially (server-exclusive DB access relaxed)
