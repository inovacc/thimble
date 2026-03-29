# Plugin System Gap Analysis

Thimble plugin system vs Claude Code official plugin spec (v1.0.33+).

## Architecture Comparison

| Aspect | Claude Code Plugins | Thimble Plugins |
|--------|-------------------|-----------------|
| **Format** | Directory with `.claude-plugin/plugin.json` | Single JSON file per plugin |
| **Components** | Skills, Agents, Hooks, MCP servers, LSP servers, Output styles, Settings | Tools only (command templates) |
| **Namespacing** | `/plugin-name:skill-name` | `ctx_` prefix enforcement |
| **Discovery** | Marketplace repos, `--plugin-dir` flag | Registry at `github.com/inovacc/thimble-plugins` |
| **Installation** | Scoped (user/project/local/managed) | Global `{DataDir}/plugins/` |
| **Hot-reload** | `/reload-plugins` command | fsnotify watcher (10s polling) |
| **Distribution** | Marketplace repos (git-based) | JSON registry |

## Gap Analysis

### Critical Gaps (Missing Core Concepts)

#### 1. Skills/Commands Support
- **Spec**: Plugins contain `skills/` and `commands/` directories with Markdown files (`SKILL.md`)
- **Thimble**: No concept of skills/commands — plugins only define executable tools
- **Impact**: Cannot ship prompt-based skills, only tool-wrapping commands
- **Recommendation**: Add `skills` array to plugin JSON with `name`, `description`, `content` fields; inject as MCP prompts or resources

#### 2. Agents Support
- **Spec**: Plugins ship `agents/` directory with Markdown agent definitions (model, effort, maxTurns, tools, isolation)
- **Thimble**: No agent concept
- **Impact**: Cannot provide specialized subagents via plugins
- **Recommendation**: Low priority — thimble is an MCP server, not the AI host. Agent definitions are host-side.

#### 3. Hooks Support
- **Spec**: Plugins ship `hooks/hooks.json` with 22+ event types (SessionStart, PreToolUse, PostToolUse, Stop, FileChanged, etc.), 4 hook types (command, http, prompt, agent)
- **Thimble**: Hooks are hardcoded in thimble's own hook system, not extensible via plugins. Plugins only add MCP tools.
- **Impact**: Plugin authors cannot react to lifecycle events
- **Recommendation**: Add optional `hooks` key to plugin JSON; merge into thimble's hook dispatch

#### 4. MCP Server Bundling
- **Spec**: Plugins bundle `.mcp.json` with server definitions; servers auto-start when plugin enabled
- **Thimble**: Plugins define tools that run as shell commands, not MCP servers
- **Impact**: Cannot compose MCP servers from plugins
- **Recommendation**: Low priority — thimble IS the MCP server; nested servers add complexity

#### 5. LSP Server Support
- **Spec**: Plugins ship `.lsp.json` for language server integration (diagnostics, go-to-definition)
- **Thimble**: No LSP concept
- **Impact**: Cannot add code intelligence via plugins
- **Recommendation**: Out of scope — LSP is IDE-side, thimble is MCP-side

#### 6. Plugin Manifest Schema
- **Spec**: Rich manifest with `author`, `homepage`, `repository`, `license`, `keywords`, component paths, `userConfig`, `channels`
- **Thimble**: Minimal: `name`, `version`, `description`, `tools[]`
- **Impact**: Limited discoverability and metadata
- **Recommendation**: Add optional metadata fields to align with spec

### Moderate Gaps

#### 7. Installation Scopes
- **Spec**: 4 scopes (user, project, local, managed) controlling visibility and sharing
- **Thimble**: Single global scope (`{DataDir}/plugins/`)
- **Impact**: No per-project plugin isolation, no team sharing via VCS
- **Recommendation**: Add `--scope` flag to `plugin install`; support project-level plugin dirs

#### 8. Environment Variables
- **Spec**: `${CLAUDE_PLUGIN_ROOT}` and `${CLAUDE_PLUGIN_DATA}` for path resolution and persistent state
- **Thimble**: No equivalent — commands are templates with `{{.fieldname}}` substitution
- **Impact**: Plugin scripts cannot reference their own directory or persistent state
- **Recommendation**: Add `THIMBLE_PLUGIN_ROOT` and `THIMBLE_PLUGIN_DATA` substitution in command templates

#### 9. User Configuration (userConfig)
- **Spec**: Plugin declares config keys, Claude Code prompts at enable time, values stored in settings/keychain
- **Thimble**: No user config concept
- **Impact**: Plugins cannot collect API keys or preferences from users
- **Recommendation**: Add `user_config` to plugin JSON; store in session DB or config file

#### 10. Plugin Validation
- **Spec**: `claude plugin validate` command checks manifest, frontmatter, hooks schema
- **Thimble**: Validates JSON structure, `ctx_` prefix, required fields — but no dedicated validate command
- **Impact**: No standalone validation for plugin authors
- **Recommendation**: Add `thimble plugin validate <path>` command

### Minor Gaps / Differences

#### 11. Settings Bundling
- **Spec**: `settings.json` at plugin root (currently only `agent` key supported)
- **Thimble**: No settings concept per plugin
- **Impact**: Minimal — limited spec support anyway

#### 12. Output Styles
- **Spec**: `output-styles/` directory for custom output formatting
- **Thimble**: No output style concept
- **Impact**: Minimal — cosmetic feature

#### 13. Channels
- **Spec**: Message injection channels (Telegram, Slack, Discord style)
- **Thimble**: No channels concept
- **Impact**: Minimal — niche feature

#### 14. Plugin Caching
- **Spec**: Marketplace plugins copied to `~/.claude/plugins/cache/`
- **Thimble**: Plugins stored directly in `{DataDir}/plugins/`
- **Impact**: No version isolation, no cache management

## Thimble Advantages (Not in Spec)

| Feature | Description |
|---------|-------------|
| **Input schema** | Plugins define typed `input_schema` per tool with validation |
| **Template rendering** | Go `text/template` syntax for command construction |
| **Timeout control** | Per-tool `timeout_ms` configuration |
| **Working directory** | Per-tool `working_dir` override |
| **Auto-indexing** | Plugin tool output auto-indexed into FTS5 knowledge base |
| **Security enforcement** | Bash deny policies applied to plugin commands |
| **Built-in conflict detection** | 67 reserved tool names prevent shadowing |

## Priority Recommendations

| Priority | Gap | Effort | Impact |
|----------|-----|--------|--------|
| P1 | Plugin manifest metadata alignment (#6) | Quick | Better discoverability |
| P1 | Plugin validation command (#10) | Quick | Developer experience |
| P2 | Environment variables (#8) | Medium | Plugin portability |
| P2 | Installation scopes (#7) | Medium | Team sharing |
| P2 | Hooks extensibility (#3) | Large | Plugin expressiveness |
| P3 | Skills support (#1) | Medium | Prompt-based plugins |
| P3 | User configuration (#9) | Medium | API key management |
| -- | Agents (#2), MCP bundling (#4), LSP (#5) | N/A | Out of scope for MCP plugin |
