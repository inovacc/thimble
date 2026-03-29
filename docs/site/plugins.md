# Plugin System

Thimble supports dynamic tool registration via JSON plugin definitions. Plugins extend the MCP server with new tools at runtime, with hot-reload support and a community marketplace.

## Installing Plugins

```bash
# Browse available plugins from the registry
thimble plugin search

# Install from the registry by name
thimble plugin install docker
thimble plugin install kubernetes
thimble plugin install terraform

# Install from a GitHub path
thimble plugin install github.com/user/repo/my-plugin.json

# Install from a URL
thimble plugin install https://example.com/plugin.json

# Install to a specific scope
thimble plugin install docker --scope project
thimble plugin install docker --scope local
```

## Managing Plugins

```bash
thimble plugin list              # List installed plugins and their tools
thimble plugin update            # Update all plugins from registry
thimble plugin update docker     # Update a specific plugin
thimble plugin remove docker     # Uninstall a plugin
thimble plugin dir               # Show the plugins directory path
thimble plugin validate path.json  # Validate a plugin definition
```

## Plugin Scopes

Plugins can be installed at three levels, with narrower scopes overriding broader ones:

| Scope | Directory | Use Case |
|-------|-----------|----------|
| `user` (default) | `{DataDir}/plugins/` | Personal tools, available in all projects |
| `project` | `{ProjectDir}/.thimble/plugins/` | Shared tools, committed to VCS |
| `local` | `{ProjectDir}/.thimble/plugins.local/` | Private tools, gitignored |

Priority: `local` > `project` > `user`.

## Plugin JSON Format

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "My custom tools",
  "author": {
    "name": "Your Name",
    "email": "you@example.com",
    "url": "https://github.com/you"
  },
  "homepage": "https://github.com/you/my-plugin",
  "repository": "https://github.com/you/my-plugin",
  "license": "MIT",
  "keywords": ["docker", "containers"],
  "tools": [
    {
      "name": "ctx_my_tool",
      "description": "Does something useful",
      "command": "my-command {{.input_arg}}",
      "input_schema": {
        "input_arg": {
          "type": "string",
          "description": "The argument to pass",
          "required": true
        }
      },
      "working_dir": "{{.projectDir}}",
      "timeout_ms": 30000
    }
  ],
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "ctx_my_.*",
        "command": "echo 'pre-hook check'"
      }
    ]
  }
}
```

## Tool Definition Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Must start with `ctx_` prefix |
| `description` | string | Yes | Shown in MCP tool listing |
| `command` | string | Yes | Shell command with Go template substitution |
| `input_schema` | object | Yes | Input parameters (type, description, required) |
| `working_dir` | string | No | Working directory for the command |
| `timeout_ms` | int | No | Execution timeout in milliseconds |

### Template Variables

Commands support Go template substitution:

- `{{.param_name}}` -- input parameter values
- `{{.projectDir}}` -- auto-injected project directory

## Plugin Metadata Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique plugin identifier |
| `version` | No | Semantic version |
| `description` | No | Brief description |
| `author` | No | Author object (name, email, url) |
| `homepage` | No | Plugin homepage URL |
| `repository` | No | Source repository URL |
| `license` | No | License identifier |
| `keywords` | No | Array of tags for search |
| `tools` | Yes | Array of tool definitions (at least one) |
| `hooks` | No | Map of event name to hook arrays |

## Plugin Hooks

Plugins can define hooks that run on lifecycle events alongside the built-in hooks:

| Event | Description |
|-------|-------------|
| `PreToolUse` | Runs before tool execution; can use `matcher` to filter by tool name |
| `PostToolUse` | Runs after tool execution |
| `SessionStart` | Runs when a session begins |
| `PreCompact` | Runs before context compaction |

## Validation

Validate a plugin definition before installing:

```bash
thimble plugin validate my-plugin.json
```

Validation checks:

- Valid JSON syntax
- `name` field is present
- At least one tool defined
- All tool names have `ctx_` prefix
- No duplicate tool names
- Input schema fields have valid types
- Hook events are valid lifecycle events

## Registry

The plugin registry is hosted at [github.com/inovacc/thimble-plugins](https://github.com/inovacc/thimble-plugins).

Available plugins:

| Plugin | Tools | Description |
|--------|-------|-------------|
| docker | `ctx_docker_ps`, `ctx_docker_logs`, `ctx_docker_images`, `ctx_docker_stats` | Container management |
| kubernetes | `ctx_k8s_pods`, `ctx_k8s_logs`, `ctx_k8s_describe`, `ctx_k8s_events` | Cluster operations |
| terraform | `ctx_tf_plan`, `ctx_tf_state`, `ctx_tf_output`, `ctx_tf_validate` | Infrastructure management |

To submit a plugin to the registry, see the [plugin authoring guide](https://github.com/inovacc/thimble-plugins#create-your-own-plugin).
