package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/plugin"
)

// BuiltinToolNamesList returns the list of built-in MCP tool names for use
// in conflict detection from external packages.
func BuiltinToolNamesList() []string {
	names := make([]string, 0, len(builtinToolNames))
	for name := range builtinToolNames {
		names = append(names, name)
	}

	return names
}

// builtinToolNames is the set of all built-in MCP tool names.
// Plugin tools that conflict with these names are rejected.
var builtinToolNames = map[string]bool{
	"ctx_execute":             true,
	"ctx_execute_file":        true,
	"ctx_index":               true,
	"ctx_search":              true,
	"ctx_fetch_and_index":     true,
	"ctx_batch_execute":       true,
	"ctx_stats":               true,
	"ctx_doctor":              true,
	"ctx_analyze":             true,
	"ctx_symbols":             true,
	"ctx_upgrade":             true,
	"ctx_delegate":            true,
	"ctx_delegate_status":     true,
	"ctx_delegate_cancel":     true,
	"ctx_delegate_list":       true,
	"ctx_report_list":         true,
	"ctx_report_show":         true,
	"ctx_report_delete":       true,
	"ctx_git_status":          true,
	"ctx_git_diff":            true,
	"ctx_git_log":             true,
	"ctx_git_blame":           true,
	"ctx_git_branches":        true,
	"ctx_git_stash":           true,
	"ctx_git_commit":          true,
	"ctx_git_changelog":       true,
	"ctx_git_merge":           true,
	"ctx_git_rebase":          true,
	"ctx_git_validate_branch": true,
	"ctx_git_lint_commit":     true,
	"ctx_gh":                  true,
	"ctx_gh_pr_status":        true,
	"ctx_gh_run_status":       true,
	"ctx_gh_issue_list":       true,
	"ctx_gh_search":           true,
	"ctx_gh_api":              true,
	"ctx_gh_repo_view":        true,
	"ctx_lint":                true,
	"ctx_lint_fix":            true,
	"ctx_plugin_conflicts":    true,
	"ctx_github_pr_list":      true,
	"ctx_github_pr_get":       true,
	"ctx_github_pr_create":    true,
	"ctx_github_issue_list":   true,
	"ctx_github_issue_get":    true,
	"ctx_github_issue_create": true,
}

// registerPluginTools loads plugins from all scopes (user, project, local)
// and registers their tools on the MCP server. Tools that conflict with
// built-in names or other plugin tools are skipped with a warning and recorded
// in b.pluginConflicts for reporting via ctx_plugin_conflicts.
func (b *Bridge) registerPluginTools() {
	allPlugins, err := plugin.LoadAllScopes(b.projectDir)
	if err != nil {
		b.logger.Warn("failed to load plugins", "error", err)
		return
	}

	if len(allPlugins) == 0 {
		return
	}

	// Detect all conflicts up front.
	conflicts := plugin.DetectConflicts(allPlugins, BuiltinToolNamesList())
	b.pluginConflicts = conflicts

	// Log each conflict.
	for _, c := range conflicts {
		b.logger.Warn("plugin tool conflict detected, skipping",
			"plugin", c.PluginName,
			"tool", c.ToolName,
			"conflicts_with", c.ConflictsWith)
	}

	// Build a set of conflicting (plugin, tool) pairs for fast lookup during registration.
	type conflictKey struct{ plugin, tool string }

	conflictSet := make(map[conflictKey]bool, len(conflicts))
	for _, c := range conflicts {
		conflictSet[conflictKey{c.PluginName, c.ToolName}] = true
	}

	registered := 0

	for _, sp := range allPlugins {
		for _, toolDef := range sp.Tools {
			if conflictSet[conflictKey{sp.Name, toolDef.Name}] {
				continue
			}

			// Capture toolDef and pluginDef for the closure.
			td := toolDef
			pName := sp.Name
			pDef := sp.PluginDef

			mcpsdk.AddTool(b.server, &mcpsdk.Tool{
				Name:        td.Name,
				Description: fmt.Sprintf("[plugin:%s:%s] %s", pName, sp.Scope, td.Description),
			}, b.makePluginHandler(pName, td, &pDef))

			registered++

			b.logger.Info("registered plugin tool",
				"plugin", pName,
				"tool", td.Name,
				"scope", string(sp.Scope))
		}
	}

	if registered > 0 {
		b.logger.Info("plugin tools registered", "count", registered)
	}

	// Register the conflicts reporting tool.
	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_plugin_conflicts",
		Description: "List active plugin tool conflicts (tool name collisions with built-in tools or other plugins).",
	}, b.handlePluginConflicts)
}

// handlePluginConflicts returns the list of detected plugin tool conflicts as JSON.
func (b *Bridge) handlePluginConflicts(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, struct{}, error) {
	if len(b.pluginConflicts) == 0 {
		return textResult("No plugin conflicts detected."), struct{}{}, nil
	}

	data, err := json.MarshalIndent(b.pluginConflicts, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("marshal conflicts: %v", err)), struct{}{}, nil
	}

	header := fmt.Sprintf("%d plugin conflict(s) detected:\n\n", len(b.pluginConflicts))

	return textResult(header + string(data)), struct{}{}, nil
}

// makePluginHandler creates an MCP tool handler for a plugin tool definition.
// The handler substitutes template variables in the command and executes via
// the executor gRPC service.
func (b *Bridge) makePluginHandler(pluginName string, td plugin.ToolDef, pDef *plugin.PluginDef) func(ctx context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, struct{}, error) {
	// Build sandbox for this plugin (once per handler creation).
	sandbox := plugin.SandboxFromPlugin(pDef)

	return func(ctx context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, struct{}, error) {
		b.recordToolCall(ctx, td.Name, false)

		// Build template data from request arguments (json.RawMessage).
		data := make(map[string]string)

		if len(req.Params.Arguments) > 0 {
			var args map[string]any
			if err := json.Unmarshal(req.Params.Arguments, &args); err == nil {
				for k, v := range args {
					switch val := v.(type) {
					case string:
						data[k] = val
					default:
						data[k] = fmt.Sprintf("%v", v)
					}
				}
			}
		}

		// Inject plugin environment variables into template data.
		pluginRoot := plugin.PluginDir()
		pluginData := plugin.PluginDataDir(pluginName)
		data["THIMBLE_PLUGIN_ROOT"] = pluginRoot
		data["THIMBLE_PLUGIN_DATA"] = pluginData

		// Substitute template variables in the command.
		command, err := renderCommand(td.Command, data)
		if err != nil {
			return errorResult(fmt.Sprintf("template error: %v", err)), struct{}{}, nil
		}

		// Plugin sandbox enforcement: validate command against allowlist.
		if err := plugin.ValidateCommand(sandbox, command); err != nil {
			return errorResult("sandbox: " + err.Error()), struct{}{}, nil //nolint:nilerr // MCP tools surface errors as results
		}

		// Per-tool permission enforcement.
		if err := plugin.ValidateToolPermissions(sandbox, td.Permissions, command); err != nil {
			return errorResult("sandbox: " + err.Error()), struct{}{}, nil //nolint:nilerr // MCP tools surface errors as results
		}

		// Prepend environment variable exports so child processes can access them.
		command = fmt.Sprintf("THIMBLE_PLUGIN_ROOT=%s THIMBLE_PLUGIN_DATA=%s %s",
			shellQuote(pluginRoot), shellQuote(pluginData), command)

		// Security check (global deny policies).
		if err := b.checkCommandDeny(command); err != nil {
			return errorResult("denied: " + err.Error()), struct{}{}, nil //nolint:nilerr // MCP tools surface errors as results
		}

		// Determine timeout: per-tool permissions override > tool-level > sandbox max.
		effectiveMax := plugin.EffectiveTimeout(sandbox, td.Permissions)
		timeoutMs := int32(effectiveMax.Milliseconds())

		if td.Timeout > 0 {
			toolMs := int32(td.Timeout)
			if toolMs < timeoutMs {
				timeoutMs = toolMs
			}
		}

		// Enforce sandbox max timeout as hard cap.
		sandboxMaxMs := int32(sandbox.MaxTimeout.Milliseconds())
		if timeoutMs > sandboxMaxMs {
			timeoutMs = sandboxMaxMs
		}

		// Execute via executor.
		timeout := time.Duration(timeoutMs) * time.Millisecond

		result, err := b.executor.Execute(ctx, "shell", command, timeout, false)
		if err != nil {
			return errorResult("execution failed: " + err.Error()), struct{}{}, nil //nolint:nilerr // MCP tools surface errors as results
		}

		output := formatExecOutput(result.Stdout, result.Stderr, result.ExitCode, result.TimedOut, false)

		// Auto-index output into knowledge base.
		if len(output) > 0 {
			label := fmt.Sprintf("plugin:%s:%s:%d", pluginName, td.Name, time.Now().Unix())
			_, _ = b.content.IndexPlainText(output, label, 20)
		}

		return filterResult(output), struct{}{}, nil
	}
}

// pluginWatchInterval is how often the bridge polls the plugin directory for new files.
const pluginWatchInterval = 10 * time.Second

// watchPlugins polls all plugin scope directories for newly added or removed plugin files.
// New plugins are loaded and their tools registered immediately. Removed plugins
// are logged as warnings since the MCP SDK does not support tool unregistration;
// calls to removed plugin tools return a "plugin removed" error.
func (b *Bridge) watchPlugins(ctx context.Context) {
	// Build list of directories to watch (user scope always, project/local if projectDir set).
	dirs := []string{plugin.PluginDir()}

	if b.projectDir != "" {
		dirs = append(dirs,
			plugin.ScopeDir(plugin.ScopeProject, b.projectDir),
			plugin.ScopeDir(plugin.ScopeLocal, b.projectDir),
		)
	}

	// Track known files per directory: dir -> set of filenames.
	knownPerDir := make(map[string]map[string]bool, len(dirs))

	for _, dir := range dirs {
		known := make(map[string]bool)

		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
					known[e.Name()] = true
				}
			}
		}

		knownPerDir[dir] = known
	}

	ticker := time.NewTicker(pluginWatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, dir := range dirs {
				known := knownPerDir[dir]

				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}

				// Detect new plugin files.
				current := make(map[string]bool, len(entries))
				for _, e := range entries {
					if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
						continue
					}

					current[e.Name()] = true

					if known[e.Name()] {
						continue
					}

					// New plugin file detected.
					p, err := plugin.LoadPluginFile(filepath.Join(dir, e.Name()))
					if err != nil {
						b.logger.Warn("hot-reload: invalid plugin", "file", e.Name(), "dir", dir, "error", err)
						continue
					}

					// Register new tools, checking for conflicts.
					for _, td := range p.Tools {
						if builtinToolNames[td.Name] {
							b.pluginConflicts = append(b.pluginConflicts, plugin.PluginConflict{
								PluginName:    p.Name,
								ToolName:      td.Name,
								ConflictsWith: "built-in",
							})

							b.logger.Warn("hot-reload: plugin tool conflicts with built-in, skipping",
								"plugin", p.Name, "tool", td.Name)

							continue
						}

						mcpsdk.AddTool(b.server, &mcpsdk.Tool{
							Name:        td.Name,
							Description: fmt.Sprintf("[plugin:%s] %s", p.Name, td.Description),
						}, b.makePluginHandler(p.Name, td, p))

						b.logger.Info("hot-reload: registered plugin tool",
							"tool", td.Name, "plugin", p.Name, "dir", dir)
					}

					known[e.Name()] = true
				}

				// Detect removed plugin files.
				for name := range known {
					if !current[name] {
						b.logger.Warn("hot-reload: plugin file removed (tools remain registered but will be inactive)",
							"file", name, "dir", dir)
						delete(known, name)
					}
				}
			}
		}
	}
}

// shellQuote wraps a string in single quotes for safe shell embedding.
// Any embedded single quotes are escaped as '\''.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// renderCommand substitutes {{.fieldname}} placeholders in the command template.
// Missing fields are replaced with empty strings.
func renderCommand(cmdTemplate string, data map[string]string) (string, error) {
	tmpl, err := template.New("cmd").Option("missingkey=zero").Parse(cmdTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	result := strings.TrimSpace(buf.String())
	if result == "" {
		return "", fmt.Errorf("command template produced empty result")
	}

	return result, nil
}
