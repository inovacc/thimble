# ADR-0007: Background Task Delegation (ctx_delegate)

## Status

Accepted

## Context

Long-running tasks (code analysis, large file indexing, test suites, builds) block
the MCP request-response cycle. The LLM must wait for completion before proceeding,
wasting context window budget and wall-clock time. A fire-and-forget model with
polling enables the LLM to start expensive work, continue with other tasks, and
check back when needed.

## Decision

Add an in-memory task manager to the gRPC daemon via a new `TaskDelegate` service
with four RPCs:

| RPC | Purpose |
|-----|---------|
| `StartTask` | Launch a background goroutine, return task_id immediately |
| `GetTaskStatus` | Poll task state, stdout/stderr, exit code |
| `CancelTask` | Cancel a running task via context propagation |
| `ListTasks` | List all tasks with optional status filter |

Four MCP tools (`ctx_delegate`, `ctx_delegate_status`, `ctx_delegate_cancel`,
`ctx_delegate_list`) expose the service to LLM clients.

Tasks run as goroutines using `ExecuteStream` for real-time output capture.
Each task gets its own `context.WithCancel` for clean cancellation.

### Constraints

- **Max 5 concurrent tasks** (`DefaultMaxTasks`) to prevent resource exhaustion.
- **1 MB output cap** per stream (stdout/stderr) to prevent memory issues.
- **UUID task IDs** via `crypto/rand` (no external dependencies).

## Consequences

- Tasks are **ephemeral** (daemon-scoped, no persistence across restarts).
- The daemon's `Close()` method calls `Shutdown()` which cancels all running tasks
  and waits for goroutines to drain.
- Completed task output is auto-indexed into the knowledge base when retrieved
  via `ctx_delegate_status`, making results searchable.
- ADR-0003 (server-exclusive DB access) is respected: the delegate service lives
  in `internal/server/` and does not import DB packages directly.
