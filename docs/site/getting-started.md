# Getting Started

## Installation

### Claude Code Plugin (Recommended)

Install from the marketplace:

```bash
claude plugin install thimble@thimble
```

For development, load directly from the source directory:

```bash
claude --plugin-dir /path/to/thimble
```

### npm Package

```bash
npm install @inovacc/thimble
```

### Install Script

**Linux / macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.sh | bash
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.ps1 | iex
```

### Go Install

```bash
go install github.com/inovacc/thimble/cmd/thimble@latest
```

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/inovacc/thimble/releases).

## First Configuration

### 1. Set Up Hooks for Your IDE

```bash
# Auto-detect platform
thimble setup

# Or specify explicitly
thimble setup --client claude
thimble setup --client gemini
thimble setup --client cursor

# Deploy as a full plugin (skills, hooks, MCP config)
thimble setup --client claude --plugin
```

Supported `--client` values: `claude`, `gemini`, `cursor`, `vscode-copilot`, `opencode`, `codex`, `kiro`, `openclaw`, `antigravity`.

### 2. Verify Installation

```bash
thimble doctor
```

The doctor command checks:

- Binary version and build info
- SQLite database connectivity
- External tool availability (`gh`, `golangci-lint`)
- Hook configuration status for your platform
- Data directory permissions

### 3. Run the MCP Server

The default command starts the MCP server on stdio (used by IDE hook configurations):

```bash
thimble
```

### 4. View Hook Logs

```bash
thimble hooklog             # All hook interactions
thimble hooklog --blocked   # Only blocked/denied hooks
thimble hooklog --debug     # Full payloads
thimble hooklog --clear     # Clear the log
```

## CLI Commands

| Command | Description |
|---------|-------------|
| *(default)* | Run MCP server on stdio |
| `hook <platform> <event>` | Dispatch hook event (in-process, ~10ms) |
| `setup --client <name>` | Configure hooks for IDE |
| `doctor` | Run diagnostic checks |
| `report [list\|show\|delete]` | Manage auto-generated reports |
| `upgrade` | Self-update from GitHub Releases |
| `lint [--fix] [--fast]` | Run golangci-lint |
| `plugin list` | List installed plugins and tools |
| `plugin install <source>` | Install from registry, URL, or GitHub path |
| `plugin remove <name>` | Remove an installed plugin |
| `plugin search` | Browse available plugins |
| `plugin dir` | Show plugins directory path |
| `plugin update [name]` | Update plugins from registry |
| `plugin validate <path>` | Validate a plugin definition file |
| `hooklog [--blocked] [--clear]` | Show hook interaction logs |
| `release-notes` | Generate release notes from git changelog |
| `publish` | Commit, tag, push, and monitor CI pipeline |
| `publish-status` | Check publish/release pipeline status |
| `selfheal` | Attempt to repair broken configuration |
| `version` | Print version information |

## Optional External Tools

These tools are not required but enable additional features:

| Tool | Required For | Install |
|------|-------------|---------|
| `gh` | GitHub CLI tools (`ctx_gh_*`) | [cli.github.com](https://cli.github.com/) |
| `golangci-lint` | Lint tools (`ctx_lint`, `ctx_lint_fix`) | [golangci-lint.run](https://golangci-lint.run/) |

Both degrade gracefully if not found on PATH -- all other tools work without them.

## GitHub API Access

For the ~80 GitHub API tools (imported from github-mcp-server v0.33.0), set a personal access token:

```bash
export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_...
```

Token resolution order:

1. `GITHUB_PERSONAL_ACCESS_TOKEN` environment variable
2. `GH_TOKEN` environment variable
3. `GITHUB_TOKEN` environment variable
4. `~/.config/gh/hosts.yml` (gh CLI auth)

## Self-Update

```bash
thimble upgrade
```

Downloads the latest release from GitHub Releases and replaces the current binary. On Windows, uses a rename-and-replace strategy.
