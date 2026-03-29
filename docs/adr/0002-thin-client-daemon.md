# ADR-0002: Thin Client with Auto-Start Daemon Pattern

## Status
Accepted

## Context
Thimble needs to run as both a CLI tool and a long-lived MCP server. Starting a full server on every CLI invocation is wasteful. The kody project demonstrates a proven pattern: thin client that auto-discovers or spawns a gRPC daemon.

## Decision
Adopt the thin-client/daemon pattern from kody:

1. **Server discovery chain:** server.json file → port probing → config → default
2. **Auto-start:** If no server found, spawn detached process with hidden `service` command
3. **Singleton client:** `sync.Once` lazy init with gRPC health check
4. **Idle shutdown:** Server auto-stops after configurable idle timeout
5. **Server info file:** `~/.cache/thimble/server.json` with address, port, PID
6. **Platform-specific detachment:** Separate `proc_windows.go` / `proc_unix.go`

All CLI commands are thin wrappers that forward to the gRPC daemon.

## Consequences

### Positive
- Single binary serves all roles (CLI, daemon, MCP server)
- Fast CLI startup (no server init on each call)
- Resource-efficient (idle shutdown, shared state)
- Proven pattern from production use in kody

### Negative
- More complex than direct execution
- Requires platform-specific process management
- Server discovery adds latency on first call
