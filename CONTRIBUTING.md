# Contributing to Thimble

Thank you for considering contributing to Thimble. This guide covers everything you need to get started.

## Getting Started

1. Fork and clone the repository:

   ```bash
   git clone https://github.com/<your-user>/thimble.git
   cd thimble
   ```

2. Install dependencies and build:

   ```bash
   go mod download
   task build
   ```

3. Run the test suite:

   ```bash
   task test
   ```

4. Verify everything passes lint:

   ```bash
   task lint
   ```

## Development Setup

### Required Tools

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.22+ | Build and test |
| [Task](https://taskfile.dev/) | 3.x | Task runner (replaces Make) |
| [golangci-lint](https://golangci-lint.run/) | latest | Linting |

### Optional Tools

| Tool | Purpose |
|------|---------|
| `gh` CLI | GitHub CLI tools (for gh-related MCP tools) |
| [GoReleaser](https://goreleaser.com/) | Building release artifacts |

### IDE Setup

Any Go-capable editor works. Recommended settings:

- Enable `goimports` on save
- Enable `golangci-lint` as the linter
- Set Go build tags if needed for platform-specific files (`_windows.go`, `_darwin.go`, `_linux.go`)

### Running During Development

```bash
# Run MCP server directly
go run ./cmd/thimble

# Run with Claude Code as a plugin (development mode)
claude --plugin-dir .

# Run diagnostics
go run ./cmd/thimble doctor
```

See all available tasks with `task --list`.

## Code Style

Follow the conventions documented in `CLAUDE.md`. Key points:

### Error Handling

```go
// Wrap errors with context
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Use errors.Is / errors.As, never == for error comparison
if errors.Is(err, sql.ErrNoRows) { ... }
```

### Logging

Use `log/slog` with structured JSON output. All log output goes to stderr (stdout is reserved for MCP JSON-RPC).

```go
slog.Info("operation completed", "key", value, "count", n)
slog.Error("operation failed", "err", err)
```

### Deferred Closes

```go
defer func() { _ = file.Close() }()
```

### General

- Module path: `github.com/inovacc/thimble`
- SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- Mute unused returns explicitly: `_, _ = fmt.Fprintln(w, output)`
- Platform-specific files use suffixes: `file_windows.go`, `file_linux.go`, `file_darwin.go`

## Testing

### Running Tests

```bash
task test              # Full test suite with race detection and coverage
task test:unit         # Unit tests only (skip integration)
task test:cover        # Tests with coverage percentage summary
task test:coverage     # Generate HTML coverage report
```

### Writing Tests

Use table-driven tests:

```go
func TestParseInput(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    Result
        wantErr bool
    }{
        {
            name:  "valid input",
            input: "hello",
            want:  Result{Value: "hello"},
        },
        {
            name:    "empty input",
            input:   "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseInput(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseInput() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr && got != tt.want {
                t.Errorf("ParseInput() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Coverage

Target 80%+ code coverage. Check your coverage before submitting:

```bash
task test:cover
```

### Test Guidelines

- Never let tests spawn real processes via `os.Executable()` -- use injectable function variables for subprocess calls.
- Tests must pass on all three CI platforms: Ubuntu, macOS, and Windows.
- Use `-short` flag to skip long-running integration tests: `if testing.Short() { t.Skip("skipping integration test") }`.

## Pull Request Process

### Branch Naming

Use descriptive branch names with a type prefix:

- `feat/add-new-tool` -- new features
- `fix/sqlite-wal-timeout` -- bug fixes
- `refactor/extract-executor` -- code restructuring
- `docs/update-readme` -- documentation changes
- `test/add-store-coverage` -- test additions

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add ctx_git_cherry_pick tool
fix: handle nil pointer in session resume
refactor: extract common SQL patterns to db package
docs: update MCP tools table in README
test: add table-driven tests for polyglot executor
chore: bump golangci-lint to v2
```

### Submitting a PR

1. Create a feature branch from `main`.
2. Make your changes, ensuring tests pass and lint is clean.
3. Run the full quality check: `task check` (runs fmt, vet, lint, test).
4. Push your branch and open a pull request against `main`.
5. Fill in the PR description with:
   - A summary of what changed and why.
   - How to test the changes.
   - Any breaking changes or deprecation notes.
6. CI must pass on all platforms (Ubuntu, macOS, Windows) before merge.

### Review Expectations

- PRs are reviewed for correctness, test coverage, and adherence to project conventions.
- Small, focused PRs are preferred over large multi-concern changes.
- Breaking changes must follow the deprecation strategy documented in `CLAUDE.md`.

## Architecture Overview

Thimble is a single-binary application. There is no daemon, no gRPC, no discovery chain. Every instance is standalone.

```
cmd/thimble          -- Cobra CLI entry point
internal/mcp/        -- MCP server bridge (stdio transport)
internal/hooks/      -- Hook dispatcher (PreToolUse/PostToolUse/SessionStart)
internal/gitops/     -- Git subprocess operations
internal/plugin/     -- Plugin system with marketplace
internal/store/      -- FTS5 knowledge base (SQLite)
internal/session/    -- Session persistence
internal/executor/   -- Polyglot code executor (11 languages)
internal/security/   -- Permission enforcement
internal/analysis/   -- Code analysis and symbol extraction
```

For detailed architecture diagrams, see `docs/ARCHITECTURE.md`. For conventions and import rules, see `CLAUDE.md`.

## Plugin Development

Thimble supports community plugins defined as JSON files. Plugins extend the MCP server with additional tools at runtime.

### Plugin Format

A plugin is a JSON file with tool definitions:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "My custom tools",
  "author": "your-name",
  "license": "BSD-3-Clause",
  "tools": [
    {
      "name": "ctx_my_tool",
      "description": "Does something useful",
      "command": "my-command {{.args}}"
    }
  ]
}
```

### Naming Rules

- All tool names **must** start with the `ctx_` prefix. The plugin system enforces this.
- Use descriptive names: `ctx_docker_ps`, `ctx_k8s_pods`, `ctx_tf_plan`.

### Testing Your Plugin

```bash
# Validate the plugin definition
go run ./cmd/thimble plugin validate my-plugin.json

# Install locally for testing
go run ./cmd/thimble plugin install ./my-plugin.json

# Verify it loads
go run ./cmd/thimble plugin list
```

### Publishing

Publish plugins to the [thimble-plugins registry](https://github.com/inovacc/thimble-plugins). See the registry README for submission instructions.

### Plugin Scopes

Plugins can be installed at three scopes: `user` (global), `project` (per-repo), or `local` (directory-level).

## Reporting Issues

Open an issue on [GitHub Issues](https://github.com/inovacc/thimble/issues) with:

1. **Thimble version** -- run `thimble version` and include the output.
2. **Operating system** -- OS name and version (Ubuntu 22.04, macOS 14, Windows 11, etc.).
3. **Steps to reproduce** -- minimal sequence of commands or actions that trigger the bug.
4. **Expected behavior** -- what you expected to happen.
5. **Actual behavior** -- what actually happened, including any error messages.
6. **Logs** -- if relevant, include output from:
   ```bash
   thimble doctor
   thimble hooklog --debug
   ```

For security vulnerabilities, do **not** open a public issue. Instead, email the maintainers directly or use GitHub's private vulnerability reporting.

## License

By contributing, you agree that your contributions will be licensed under the BSD 3-Clause License.
