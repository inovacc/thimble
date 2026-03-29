# @inovacc/thimble

Claude Code plugin for context-aware code intelligence. Single binary: MCP server, hook dispatcher, and CLI tools.

## Features

- **120+ MCP tools** (41 native + ~80 GitHub API + dynamic plugins)
- **FTS5 knowledge base** with 5-layer search fallback and BM25 ranking
- **Polyglot execution** (11 languages with timeout handling)
- **Code analysis** (Go, Python, Rust, TypeScript, Protobuf)
- **Background task delegation** (max 5 concurrent, 1MB output cap)
- **Git/GitHub integration** (13 git operations + 8 gh CLI tools)
- **OpenTelemetry observability** for all tool handlers

## Install

```bash
npm install -g @inovacc/thimble
```

This downloads the pre-built binary for your platform from [GitHub Releases](https://github.com/inovacc/thimble/releases).

## Usage

### As a Claude Code plugin (recommended)

```bash
claude --plugin-dir $(npm root -g)/@inovacc/thimble
```

### Legacy setup (hooks + MCP in settings.json)

```bash
thimble setup --client claude
```

### Other platforms

```bash
thimble setup --client gemini    # Gemini CLI
thimble setup --client cursor    # Cursor
thimble setup --client vscode    # VS Code Copilot
```

### CLI

```bash
thimble doctor     # Run diagnostic checks
thimble upgrade    # Self-update from GitHub Releases
thimble --help     # Show all commands
```

## Supported platforms

| OS      | Architecture |
|---------|-------------|
| macOS   | x64, arm64  |
| Linux   | x64, arm64  |
| Windows | x64, arm64  |

## License

BSD-3-Clause
