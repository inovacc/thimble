# ADR-0010: Claude Code Plugin Conversion

## Status

Accepted (2026-03-28)

## Context

Thimble was distributed via `setup --client claude` which deep-merged hooks into
the user's `~/.claude/settings.json`. This approach had several issues:

1. **Fragile merge logic** — deep-merging into settings.json required careful
   conflict resolution; user edits could overwrite thimble hooks, and thimble
   updates could clobber user customizations.
2. **No uninstall path** — removing thimble hooks from settings.json required
   manual editing or a dedicated teardown command.
3. **Version drift** — the binary path baked into settings.json became stale
   after upgrades, requiring re-running `setup --client claude`.
4. **No marketplace** — distribution was manual (download binary, run setup).

Claude Code added native plugin support (`--plugin-dir` for development,
`claude plugin install` for marketplace) that provides self-contained plugin
distribution with structured manifests.

## Decision

Convert thimble to the official Claude Code plugin format:

- `.claude-plugin/plugin.json` — plugin manifest (name, version, description,
  entry point, capabilities)
- `.claude-plugin/marketplace.json` — marketplace metadata (author, license,
  keywords, homepage, repository)
- `hooks/hooks.json` — hook dispatcher configuration (PreToolUse, PostToolUse,
  SessionStart, PreCompact events)
- `skills/` — 8 Claude Code skills (analyze, cloud-setup, cloud-status, doctor,
  remember, reports, stats, thimble)
- `agents/` — 4 Claude Code subagents (code-reviewer, security-auditor,
  test-generator, pr-creator)
- `assets/plugin/` — embedded plugin assets for `setup --plugin` deployment

All paths in manifests use `${CLAUDE_PLUGIN_ROOT}` for the plugin installation
directory and `${CLAUDE_PLUGIN_DATA}` (mapped to `THIMBLE_PLUGIN_DATA` env var)
for persistent storage, making the plugin fully relocatable.

## Consequences

### Positive

- **Self-contained distribution** — the entire plugin is a single directory
  with no external file mutations.
- **Marketplace listing** — users can discover and install via
  `claude plugin install thimble@thimble`.
- **Clean lifecycle** — install, update, and remove are handled by Claude Code
  natively; no manual settings.json editing.
- **Variable paths** — `${CLAUDE_PLUGIN_ROOT}` eliminates hardcoded binary
  paths that break on upgrades or relocation.
- **Skills and agents** — structured skill definitions and subagent
  configurations are discoverable by Claude Code without custom integration.

### Negative

- **Claude Code specific** — the plugin format is specific to Claude Code;
  other platforms (VS Code Copilot, Cursor, Gemini CLI, etc.) still require
  the legacy `setup --client <platform>` path.
- **Two distribution paths** — maintaining both plugin format and legacy setup
  adds surface area, though legacy setup is stable and rarely changes.

### Neutral

- **Legacy setup preserved** — `setup --client claude` still works for users
  who prefer manual configuration or run older Claude Code versions.
  The `setup --client <platform>` path remains the primary integration method
  for non-Claude-Code platforms (9 platform adapters in `internal/platform/`).
- **Hook format unchanged** — `hooks/hooks.json` uses the same schema whether
  loaded via plugin or via setup; the dispatcher code is shared.

## Supersedes

- The `setup --client claude` settings.json merge approach for Claude Code
  (other platforms unaffected).
