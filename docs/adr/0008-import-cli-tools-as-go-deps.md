# ADR-0008: Import CLI Tools as Go Library Dependencies

## Status

Superseded (partially) — gh CLI and golangci-lint reverted to subprocess;
github-mcp-server remains as Go library import.

## Context

Thimble needs to provide GitHub CLI, GitHub API, and golangci-lint functionality
to AI assistants via MCP tools. Three approaches were considered:

1. **Subprocess wrapping** — shell out to `gh`, `golangci-lint` binaries.
2. **REST/GraphQL API clients** — build our own HTTP clients.
3. **Import as Go library dependencies** — use their internal APIs in-process.

## Original Decision

Import all three as Go dependencies and execute them in-process.

## Revised Decision

After experience with the in-process approach, gh CLI and golangci-lint have
been reverted to subprocess execution:

| Dependency | Version | Integration Pattern |
|------------|---------|---------------------|
| `github.com/github/github-mcp-server` | v0.33.0 | 80 GitHub API tools via inventory registration (still in-process) |
| `gh` (subprocess) | user's installed version | Subprocess execution via `os/exec` |
| `golangci-lint` (subprocess) | user's installed version | Subprocess execution via `os/exec` |

**Reasons for reverting gh CLI and golangci-lint:**

- **Binary size reduction** — removing `cli/cli/v2` saves ~3 MB direct + transitive deps;
  removing `golangci-lint/v2` saved ~4 MB + ~300 transitive modules.
- **Panic safety** — the embedded gh CLI panicked on auth errors (e.g.,
  `AuthError.Error()` panic when gh has no auth configured), requiring defensive
  `recover()` wrappers.
- **Process isolation** — subprocess crashes do not affect the daemon.
- **Version flexibility** — users can update gh/golangci-lint independently.

Auth for the gh subprocess is handled by the user's `gh auth` config. A
standalone `internal/ghauth` package reads tokens from `GH_TOKEN`,
`GITHUB_TOKEN`, or `~/.config/gh/hosts.yml` for direct API access if needed.

## Consequences

### Positive

- **Smaller binary** — ~7 MB less from removing both in-process CLI deps.
- **Simpler code** — no Cobra command tree construction, no IOStreams capture,
  no panic recovery wrappers.
- **github-mcp-server still in-process** — 80 GitHub API tools with zero
  external dependency (uses go-github + shurcooL/githubv4 under the hood).

### Negative

- **Requires gh and golangci-lint on PATH** — not fully self-contained for
  those features.
- **Subprocess overhead** — fork/exec per invocation (negligible in practice).

### Neutral

- `github-mcp-server` remains as a Go library import, providing the bulk of
  GitHub API coverage without requiring any external binary.

## Alternatives Considered

- **Keeping in-process gh CLI** — rejected due to panic issues and binary bloat.
- **Building our own GitHub API client** was rejected — `github-mcp-server`
  already provides 80 tools with complete coverage.
