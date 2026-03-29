# Plugin Authoring Guide

Create custom MCP tools for thimble without writing Go code. Plugins are JSON files that define shell commands exposed as MCP tools to AI assistants.

## Quick Start

Create a file called `my-plugin.json`:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "tools": [
    {
      "name": "ctx_hello",
      "description": "Say hello to someone",
      "command": "echo Hello, {{.name}}!",
      "input_schema": {
        "name": {
          "type": "string",
          "description": "person to greet",
          "required": true
        }
      }
    }
  ]
}
```

Install it:

```bash
# From a local file (copy to plugins directory)
cp my-plugin.json "$(thimble plugin dir)/"

# Or install from a URL
thimble plugin install https://example.com/my-plugin.json

# Or from the registry
thimble plugin install docker
```

The tool is available immediately — thimble hot-reloads new plugins every 10 seconds without restarting your MCP session.

## Plugin Format

### Top-Level Structure

```json
{
  "name": "plugin-name",
  "version": "1.0.0",
  "tools": [ ... ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Unique plugin identifier. Used as the filename (`{name}.json`). |
| `version` | string | yes | Semantic version (e.g., `"1.0.0"`). |
| `tools` | array | yes | One or more tool definitions (see below). Must have at least one. |

### Tool Definition

```json
{
  "name": "ctx_my_tool",
  "description": "What this tool does",
  "command": "my-command --flag {{.arg1}} {{.arg2}}",
  "input_schema": {
    "arg1": {
      "type": "string",
      "description": "first argument",
      "required": true
    },
    "arg2": {
      "type": "string",
      "description": "optional argument"
    }
  },
  "working_dir": "/path/to/dir",
  "timeout_ms": 30000
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Tool name. **Must start with `ctx_`**. Shown to the AI assistant. |
| `description` | string | yes | What the tool does. AI assistants use this to decide when to call it. |
| `command` | string | yes | Shell command to execute. Supports `{{.field}}` template variables. |
| `input_schema` | object | no | Input fields the AI can provide. Keys are field names. |
| `working_dir` | string | no | Working directory for the command. Defaults to the project directory. |
| `timeout_ms` | integer | no | Execution timeout in milliseconds. Default: 30000 (30s). |

### Input Field Definition

```json
{
  "type": "string",
  "description": "what this field is for",
  "required": true
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | Field type: `"string"`, `"number"`, `"boolean"`. |
| `description` | string | yes | Describes the field to the AI assistant. Be specific — the AI uses this to provide correct values. |
| `required` | boolean | no | Whether the field is mandatory. Default: `false`. |

## Command Templates

Commands use Go's [`text/template`](https://pkg.go.dev/text/template) syntax. The most common pattern is `{{.fieldname}}` to substitute input values.

### Basic substitution

```json
"command": "docker logs --tail {{.lines}} {{.container}}"
```

If the AI calls this with `{"container": "web", "lines": "100"}`, the executed command is:

```
docker logs --tail 100 web
```

### Missing fields

Missing (optional) fields are replaced with empty strings. Design your commands to handle this gracefully:

```json
"command": "kubectl get pods {{.namespace}}"
```

If `namespace` is not provided, the command becomes `kubectl get pods ` (trailing space, which is harmless).

For flags that should be omitted entirely when empty, use conditionals:

```json
"command": "kubectl get pods{{if .namespace}} -n {{.namespace}}{{end}}"
```

### Escaping template delimiters

If your command uses `{{` literally (e.g., Docker `--format`), use Go's raw string syntax:

```json
"command": "docker ps --format '{\"table\": \"{{`{{.ID}}`}}\\t{{`{{.Names}}`}}\"}'"
```

Or avoid the conflict by using an alternative approach:

```json
"command": "docker ps --format 'table {{`{{.ID}}`}}\t{{`{{.Names}}`}}'"
```

## Naming Rules

1. **Tool names must start with `ctx_`** — this is enforced on load. Tools without the prefix are rejected.

2. **No conflicts with built-in tools** — if your tool name matches a built-in (e.g., `ctx_git_status`), it is skipped with a warning. Use a unique prefix for your plugin:

   ```
   ctx_mycompany_deploy    (good)
   ctx_git_status           (rejected — built-in conflict)
   ```

3. **Plugin name is the filename** — the plugin is saved as `{name}.json` in the plugins directory.

## Security

Plugin commands run through thimble's security engine:

- **Bash deny policies** apply — commands matching deny patterns in `.claude/settings.json` or `~/.claude/settings.json` are blocked.
- **No shell injection** — input values are substituted as-is into the template. Avoid constructing commands that could be exploited by malicious input.
- **Timeouts enforced** — commands exceeding `timeout_ms` are killed.
- **Output capped** — large outputs are automatically truncated by the executor.

### Recommendations

- Prefer read-only commands. If your tool modifies state, document it clearly in the description.
- Set appropriate timeouts. Long-running commands (terraform plan, large builds) should use higher values.
- Test commands manually before packaging as a plugin.

## Examples

### Database Query Tool

```json
{
  "name": "postgres",
  "version": "1.0.0",
  "tools": [
    {
      "name": "ctx_pg_query",
      "description": "Run a read-only SQL query against the local PostgreSQL database.",
      "command": "psql -h localhost -U {{.user}} -d {{.database}} -c '{{.query}}' --no-password",
      "input_schema": {
        "user": {
          "type": "string",
          "description": "database user",
          "required": true
        },
        "database": {
          "type": "string",
          "description": "database name",
          "required": true
        },
        "query": {
          "type": "string",
          "description": "SQL query to execute (read-only)",
          "required": true
        }
      },
      "timeout_ms": 10000
    },
    {
      "name": "ctx_pg_tables",
      "description": "List all tables in the database.",
      "command": "psql -h localhost -U {{.user}} -d {{.database}} -c '\\dt' --no-password",
      "input_schema": {
        "user": {
          "type": "string",
          "description": "database user",
          "required": true
        },
        "database": {
          "type": "string",
          "description": "database name",
          "required": true
        }
      }
    }
  ]
}
```

### AWS CLI Tool

```json
{
  "name": "aws",
  "version": "1.0.0",
  "tools": [
    {
      "name": "ctx_aws_s3_ls",
      "description": "List S3 buckets or objects in a bucket.",
      "command": "aws s3 ls {{.path}}",
      "input_schema": {
        "path": {
          "type": "string",
          "description": "S3 path (empty for buckets, s3://bucket/prefix for objects)"
        }
      }
    },
    {
      "name": "ctx_aws_ec2_instances",
      "description": "List EC2 instances with status.",
      "command": "aws ec2 describe-instances --query 'Reservations[].Instances[].[InstanceId,State.Name,InstanceType,Tags[?Key==`Name`].Value|[0]]' --output table"
    },
    {
      "name": "ctx_aws_logs",
      "description": "Tail CloudWatch log group.",
      "command": "aws logs tail {{.log_group}} --since {{.since}} --format short",
      "input_schema": {
        "log_group": {
          "type": "string",
          "description": "CloudWatch log group name",
          "required": true
        },
        "since": {
          "type": "string",
          "description": "time period (e.g., '1h', '30m', '2d')",
          "required": true
        }
      },
      "timeout_ms": 15000
    }
  ]
}
```

### System Monitoring Tool

```json
{
  "name": "sysmon",
  "version": "1.0.0",
  "tools": [
    {
      "name": "ctx_sys_disk",
      "description": "Show disk usage for all mounted filesystems.",
      "command": "df -h"
    },
    {
      "name": "ctx_sys_mem",
      "description": "Show memory usage summary.",
      "command": "free -h 2>/dev/null || vm_stat"
    },
    {
      "name": "ctx_sys_top",
      "description": "Show top processes by CPU usage.",
      "command": "ps aux --sort=-%cpu | head -20"
    },
    {
      "name": "ctx_sys_ports",
      "description": "Show listening TCP ports.",
      "command": "ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null || lsof -iTCP -sTCP:LISTEN"
    }
  ]
}
```

## Installation Methods

### From the Registry

```bash
# Browse available plugins
thimble plugin search

# Install by name
thimble plugin install docker
thimble plugin install kubernetes
thimble plugin install terraform
```

The registry is at [github.com/inovacc/thimble-plugins](https://github.com/inovacc/thimble-plugins).

### From a URL

```bash
thimble plugin install https://example.com/my-plugin.json
```

### From GitHub

```bash
# Specific file in a repo
thimble plugin install github.com/user/repo/plugins/my-tool.json

# Root plugin.json in a repo
thimble plugin install github.com/user/repo
```

### Manual Copy

```bash
# Find your plugins directory
thimble plugin dir

# Copy the file there
cp my-plugin.json "$(thimble plugin dir)/"
```

## Management

```bash
# List installed plugins with their tools
thimble plugin list

# Remove a plugin
thimble plugin remove my-plugin

# Show the plugins directory
thimble plugin dir
```

## Hot-Reload

Thimble watches the plugins directory every 10 seconds. When you install a new plugin (via `thimble plugin install` or by copying a file), its tools are registered automatically on the running MCP server — no need to restart your IDE session.

Removed plugins are detected but their tools cannot be unregistered from a running MCP session. They will return errors until the session is restarted.

## Publishing to the Registry

1. Fork [github.com/inovacc/thimble-plugins](https://github.com/inovacc/thimble-plugins)
2. Add your plugin JSON to `plugins/`
3. Add an entry to `registry.json`:
   ```json
   {
     "name": "my-plugin",
     "description": "Short description of what it does",
     "version": "1.0.0",
     "file": "plugins/my-plugin.json",
     "author": "your-name"
   }
   ```
4. Open a pull request

## Troubleshooting

### Plugin not loading

```bash
# Check if the file is valid
thimble plugin list

# Check the plugins directory
thimble plugin dir
ls "$(thimble plugin dir)"
```

Common issues:
- **Missing `ctx_` prefix** on tool names
- **Empty `tools` array** — must have at least one tool
- **Missing `name` or `command`** on a tool
- **Invalid JSON** — check for trailing commas, missing quotes

### Tool conflicts with built-in

If your tool name matches a built-in tool, it's silently skipped. Check the logs (stderr) for warnings:

```
WARN  plugin tool conflicts with built-in tool, skipping  plugin=my-plugin tool=ctx_git_status
```

Use a unique prefix: `ctx_mycompany_` or `ctx_myproject_`.

### Command template errors

If `{{.field}}` substitution fails, the tool returns a template error. Test your command with all required fields provided.

### Security policy blocks

If a command matches a deny pattern in your project's `.claude/settings.json`, the tool returns a "denied by policy" error. Check your security settings.
