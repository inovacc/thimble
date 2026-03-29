package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	pluginassets "github.com/inovacc/thimble/assets/plugin"
	"github.com/inovacc/thimble/internal/platform"
	"github.com/inovacc/thimble/internal/routing"
	"github.com/spf13/cobra"
)

// clientAliases maps short --client names to platform IDs.
var clientAliases = map[string]platform.PlatformID{
	"claude":   platform.PlatformClaudeCode,
	"gemini":   platform.PlatformGeminiCLI,
	"vscode":   platform.PlatformVSCodeCopilot,
	"copilot":  platform.PlatformVSCodeCopilot,
	"cursor":   platform.PlatformCursor,
	"opencode": platform.PlatformOpenCode,
	"codex":    platform.PlatformCodex,
	// Full names also accepted.
	"claude-code":    platform.PlatformClaudeCode,
	"gemini-cli":     platform.PlatformGeminiCLI,
	"vscode-copilot": platform.PlatformVSCodeCopilot,
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure hooks and deploy plugin for a platform",
	Long: `Generate and install hook configuration for the specified platform.

Use --client to specify the target:
  thimble setup --client claude
  thimble setup --client gemini
  thimble setup --client cursor
  thimble setup --client vscode

Use --plugin to deploy the embedded Claude Code plugin:
  thimble setup --client claude --plugin
  thimble setup --client claude --plugin --plugin-dir ~/.claude/plugins/thimble

Supported clients: claude, gemini, vscode (copilot), cursor, opencode, codex.
If --client is omitted, auto-detects the current platform.`,
	Args: cobra.NoArgs,
	RunE: runSetup,
}

var (
	setupClient       string
	setupDryRun       bool
	setupInstructions bool
	setupPlugin       bool
	setupPluginDir    string
)

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().StringVarP(&setupClient, "client", "c", "", "target client (claude, gemini, vscode, cursor, opencode, codex)")
	setupCmd.Flags().BoolVarP(&setupDryRun, "dry-run", "n", false, "Print config without writing")
	setupCmd.Flags().BoolVarP(&setupInstructions, "instructions", "i", false, "Also generate routing instructions file")
	setupCmd.Flags().BoolVar(&setupPlugin, "plugin", false, "Deploy embedded plugin files (skills, hooks, MCP config)")
	setupCmd.Flags().StringVar(&setupPluginDir, "plugin-dir", "", "Custom plugin output directory (default: ~/.claude/plugins/thimble)")
}

// resolveClient resolves a --client value to a PlatformID.
func resolveClient(name string) (platform.PlatformID, error) {
	if id, ok := clientAliases[name]; ok {
		return id, nil
	}

	// Try as raw PlatformID.
	id := platform.PlatformID(name)
	if _, err := platform.Get(id); err == nil {
		return id, nil
	}

	return "", fmt.Errorf("unknown client %q (supported: claude, gemini, vscode, cursor, opencode, codex)", name)
}

func runSetup(_ *cobra.Command, _ []string) error {
	var platformID platform.PlatformID

	if setupClient != "" {
		var err error

		platformID, err = resolveClient(setupClient)
		if err != nil {
			return err
		}
	} else {
		signal := platform.Detect()
		platformID = signal.Platform
		_, _ = fmt.Fprintf(os.Stderr, "Auto-detected platform: %s (%s)\n", platformID, signal.Reason)
	}

	adapter, err := platform.Get(platformID)
	if err != nil {
		return fmt.Errorf("unsupported platform: %s", platformID)
	}

	// Deploy plugin if requested.
	if setupPlugin {
		if platformID == platform.PlatformClaudeCode {
			_, _ = fmt.Fprintln(os.Stderr, "NOTE: For Claude Code, prefer the npm plugin install:")
			_, _ = fmt.Fprintln(os.Stderr, "  npm install -g @inovacc/thimble && claude plugin install thimble@npm:@inovacc/thimble")
			_, _ = fmt.Fprintln(os.Stderr, "Falling back to legacy deploy for backward compatibility.")
		}

		if err := deployPlugin(platformID); err != nil {
			return fmt.Errorf("deploy plugin: %w", err)
		}
	}

	if adapter.Paradigm() == platform.ParadigmMCPOnly {
		_, _ = fmt.Fprintf(os.Stderr, "%s is MCP-only (no hooks). Only MCP server config needed.\n", adapter.Name())
		return generateMCPConfig(platformID, adapter)
	}

	if adapter.Paradigm() == platform.ParadigmTSPlugin {
		_, _ = fmt.Fprintf(os.Stderr, "%s uses TS plugin paradigm. Hook config must be set in opencode.json.\n", adapter.Name())
		return nil
	}

	if err := generateHookConfig(platformID, adapter); err != nil {
		return err
	}

	if setupInstructions {
		return generateRoutingInstructions(platformID)
	}

	return nil
}

// deployPlugin extracts the embedded plugin assets to disk.
func deployPlugin(platformID platform.PlatformID) error {
	destDir := setupPluginDir
	if destDir == "" {
		destDir = defaultPluginDir(platformID)
	}

	if setupDryRun {
		_, _ = fmt.Fprintf(os.Stderr, "Would deploy plugin to %s\n", destDir)
		_, _ = fmt.Fprintf(os.Stderr, "Files:\n")

		return fs.WalkDir(pluginassets.FS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}

			_, _ = fmt.Fprintf(os.Stderr, "  %s\n", path)

			return nil
		})
	}

	var count int

	err := fs.WalkDir(pluginassets.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, filepath.FromSlash(path))

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := pluginassets.FS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", path, err)
		}

		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}

		count++

		return nil
	})
	if err != nil {
		return err
	}

	// Copy running binary to plugin directory so hook commands resolve.
	exe, exeErr := os.Executable()
	if exeErr == nil && !isTestBinary(exe) {
		binDest := filepath.Join(destDir, binaryName())

		if err := copyFile(exe, binDest, 0o755); err == nil {
			count++
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: could not copy binary to %s: %v\n", binDest, err)
		}
	}

	// Patch version in deployed manifests.
	_ = patchJSONVersion(filepath.Join(destDir, ".claude-plugin", "plugin.json"), Version)
	_ = patchJSONVersion(filepath.Join(destDir, ".claude-plugin", "marketplace.json"), Version)

	_, _ = fmt.Fprintf(os.Stderr, "Plugin deployed: %d files to %s\n", count, destDir)

	return nil
}

// defaultPluginDir returns the default plugin deployment directory for a platform.
func defaultPluginDir(platformID platform.PlatformID) string {
	home, _ := os.UserHomeDir()

	switch platformID {
	case platform.PlatformClaudeCode:
		return filepath.Join(home, ".claude", "plugins", "thimble")
	case platform.PlatformGeminiCLI:
		return filepath.Join(home, ".gemini", "plugins", "thimble")
	case platform.PlatformVSCodeCopilot:
		return filepath.Join(home, ".github", "plugins", "thimble")
	case platform.PlatformCursor:
		return filepath.Join(home, ".cursor", "plugins", "thimble")
	case platform.PlatformOpenCode, platform.PlatformCodex, platform.PlatformKiro, platform.PlatformOpenClaw, platform.PlatformAntigravity, platform.PlatformUnknown:
		return filepath.Join(home, ".thimble", "plugin")
	}

	return filepath.Join(home, ".thimble", "plugin")
}

// isTestBinary returns true if the binary path looks like a temporary Go test binary.
// This prevents `go test` from polluting real settings files with ephemeral paths.
func isTestBinary(path string) bool {
	base := filepath.Base(path)

	// Go test binaries are named <pkg>.test or <pkg>.test.exe.
	if filepath.Ext(base) == ".test" {
		return true
	}

	// On Windows: thimble.test.exe → strip .exe, check for .test.
	noExt := strings.TrimSuffix(base, ".exe")

	return filepath.Ext(noExt) == ".test"
}

// copyFile copies src to dst using streaming io.Copy to avoid loading
// the entire file into memory (important for large binaries).
func copyFile(src, dst string, perm os.FileMode) error { //nolint:unparam // perm is always 0o755 today but kept for generality
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}

func generateHookConfig(platformID platform.PlatformID, adapter platform.Adapter) error {
	// Skip writing manual hooks if the plugin is already enabled and has a binary.
	if !setupDryRun && isPluginActive(adapter.SettingsPath(), platformID) {
		_, _ = fmt.Fprintln(os.Stderr, "Plugin thimble@thimble is enabled — skipping manual hook registration.")
		_, _ = fmt.Fprintln(os.Stderr, "Plugin hooks from hooks.json will handle all events.")

		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		execPath = "thimble"
	} else {
		execPath, _ = filepath.EvalSymlinks(execPath)
	}

	config := buildHookConfig(platformID, execPath)

	// Add mcpServers to make thimble a global plugin.
	config["mcpServers"] = map[string]any{
		"thimble": map[string]any{
			"command": execPath,
			"args":    []string{},
		},
	}

	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if setupDryRun {
		_, _ = fmt.Fprintf(os.Stderr, "Hook config for %s:\n", adapter.Name())

		_, _ = fmt.Fprintln(os.Stdout, string(configJSON))

		return nil
	}

	// Guard: refuse to register a temporary test binary into real settings.
	// Placed after dry-run check so dry-run always works (it only prints, never writes).
	if isTestBinary(execPath) {
		return fmt.Errorf("refusing to register test binary %q into settings — run 'go install ./cmd/thimble/' first, then 'thimble setup'", execPath)
	}

	settingsPath := adapter.SettingsPath()

	return writeHookSettings(settingsPath, platformID, config)
}

func buildHookConfig(platformID platform.PlatformID, binaryPath string) map[string]any {
	hookCmd := func(event string) string {
		return fmt.Sprintf("%s hook %s %s", binaryPath, platformID, event)
	}

	switch platformID {
	case platform.PlatformClaudeCode:
		return map[string]any{
			"hooks": map[string]any{
				"PreToolUse": []any{
					map[string]any{
						"matcher": "Bash|Read|Edit|Write|WebFetch|Grep|Agent|Task|mcp__thimble__ctx_execute|mcp__thimble__ctx_execute_file|mcp__thimble__ctx_batch_execute",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("pretooluse")},
						},
					},
				},
				"PostToolUse": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("posttooluse")},
						},
					},
				},
				"PreCompact": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("precompact")},
						},
					},
				},
				"SessionStart": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("sessionstart")},
						},
					},
				},
				"UserPromptSubmit": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("userpromptsubmit")},
						},
					},
				},
			},
		}

	case platform.PlatformGeminiCLI:
		return map[string]any{
			"hooks": map[string]any{
				"BeforeTool": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("beforetool")},
						},
					},
				},
				"AfterTool": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("aftertool")},
						},
					},
				},
				"PreCompress": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("precompress")},
						},
					},
				},
				"SessionStart": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("sessionstart")},
						},
					},
				},
				"UserPromptSubmit": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("userpromptsubmit")},
						},
					},
				},
			},
		}

	case platform.PlatformVSCodeCopilot:
		return map[string]any{
			"hooks": map[string]any{
				"PreToolUse": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("pretooluse")},
						},
					},
				},
				"PostToolUse": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("posttooluse")},
						},
					},
				},
				"PreCompact": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("precompact")},
						},
					},
				},
				"SessionStart": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("sessionstart")},
						},
					},
				},
				"UserPromptSubmit": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("userpromptsubmit")},
						},
					},
				},
			},
		}

	case platform.PlatformOpenCode, platform.PlatformCodex, platform.PlatformKiro, platform.PlatformOpenClaw, platform.PlatformAntigravity, platform.PlatformUnknown:
		return map[string]any{}

	case platform.PlatformCursor:
		return map[string]any{
			"hooks": map[string]any{
				"preToolUse": []any{
					map[string]any{
						"matcher": "Shell|Read|Edit|Write|Grep|WebFetch|mcp_web_fetch|Task|MCP:ctx_execute|MCP:ctx_execute_file|MCP:ctx_batch_execute",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("pretooluse")},
						},
					},
				},
				"postToolUse": []any{
					map[string]any{
						"matcher": "",
						"hooks": []any{
							map[string]any{"type": "command", "command": hookCmd("posttooluse")},
						},
					},
				},
			},
		}

	}

	return map[string]any{}
}

func writeHookSettings(settingsPath string, _ platform.PlatformID, config map[string]any) error {
	var existing map[string]any

	data, err := os.ReadFile(settingsPath)
	if err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	if existing == nil {
		existing = make(map[string]any)
	}

	if newHooks, ok := config["hooks"].(map[string]any); ok {
		existingHooks, _ := existing["hooks"].(map[string]any)
		if existingHooks == nil {
			existingHooks = make(map[string]any)
		}

		maps.Copy(existingHooks, newHooks)

		existing["hooks"] = existingHooks
	}

	if newMCP, ok := config["mcpServers"].(map[string]any); ok {
		existingMCP, _ := existing["mcpServers"].(map[string]any)
		if existingMCP == nil {
			existingMCP = make(map[string]any)
		}

		maps.Copy(existingMCP, newMCP)

		existing["mcpServers"] = existingMCP
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	output, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, append(output, '\n'), 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Hooks and MCP server configured in %s\n", settingsPath)

	return nil
}

func generateMCPConfig(_ platform.PlatformID, adapter platform.Adapter) error {
	execPath, err := os.Executable()
	if err != nil {
		execPath = "thimble"
	} else {
		execPath, _ = filepath.EvalSymlinks(execPath)
	}

	config := map[string]any{
		"mcpServers": map[string]any{
			"thimble": map[string]any{
				"command": execPath,
				"args":    []string{},
			},
		},
	}

	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if setupDryRun {
		_, _ = fmt.Fprintf(os.Stderr, "MCP server config for %s:\n", adapter.Name())

		_, _ = fmt.Fprintln(os.Stdout, string(configJSON))

		_, _ = fmt.Fprintf(os.Stderr, "\nWould write to %s\n", adapter.SettingsPath())

		return nil
	}

	if isTestBinary(execPath) {
		return fmt.Errorf("refusing to register test binary %q into settings — run 'go install ./cmd/thimble/' first", execPath)
	}

	settingsPath := adapter.SettingsPath()

	return writeHookSettings(settingsPath, "", config)
}

func generateRoutingInstructions(platformID platform.PlatformID) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	if setupDryRun {
		content := routing.GenerateInstructions(platformID)
		config := routing.GetConfig(platformID)
		_, _ = fmt.Fprintf(os.Stderr, "\nRouting instructions (%s):\n", config.FileName)

		_, _ = fmt.Fprintln(os.Stdout, content)

		return nil
	}

	path, err := routing.WriteInstructions(cwd, platformID)
	if err != nil {
		return fmt.Errorf("write routing instructions: %w", err)
	}

	if path == "" {
		_, _ = fmt.Fprintln(os.Stderr, "Routing instructions already present.")
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "Routing instructions written to %s\n", path)
	}

	return nil
}

// isPluginActive checks if the thimble plugin is enabled in settings and has a binary deployed.
func isPluginActive(settingsPath string, platformID platform.PlatformID) bool {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}

	enabled, _ := settings["enabledPlugins"].(map[string]any)
	if enabled == nil {
		return false
	}

	active, _ := enabled["thimble@thimble"].(bool)
	if !active {
		return false
	}

	// Verify the plugin binary exists.
	pluginDir := defaultPluginDir(platformID)

	if _, err := os.Stat(filepath.Join(pluginDir, binaryName())); err != nil {
		return false // Plugin enabled but binary missing — allow manual hooks.
	}

	return true
}

// patchRegistryVersion updates the plugin registry JSON with the given version and install path.
func patchRegistryVersion(registryPath, installPath, version string) error {
	data, err := os.ReadFile(registryPath)
	if err != nil {
		return err
	}

	var registry map[string]any
	if err := json.Unmarshal(data, &registry); err != nil {
		return err
	}

	plugins, _ := registry["plugins"].(map[string]any)
	if plugins == nil {
		return nil
	}

	entries, _ := plugins["thimble@thimble"].([]any)
	if len(entries) == 0 {
		plugins["thimble@thimble"] = []any{
			map[string]any{
				"scope":       "user",
				"installPath": installPath,
				"version":     version,
				"installedAt": "2026-03-16T00:00:00.000Z",
				"lastUpdated": time.Now().UTC().Format(time.RFC3339),
			},
		}
	} else {
		for _, e := range entries {
			entry, _ := e.(map[string]any)
			if entry != nil {
				entry["version"] = version
				entry["installPath"] = installPath
			}
		}
	}

	output, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(registryPath, append(output, '\n'), 0o644)
}

// patchJSONVersion updates "version" fields in a JSON manifest file.
func patchJSONVersion(path, version string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}

	changed := false

	if _, ok := doc["version"]; ok {
		doc["version"] = version
		changed = true
	}

	if meta, ok := doc["metadata"].(map[string]any); ok {
		if _, ok := meta["version"]; ok {
			meta["version"] = version
			changed = true
		}
	}

	if pluginsList, ok := doc["plugins"].([]any); ok {
		for _, p := range pluginsList {
			if pm, ok := p.(map[string]any); ok {
				if _, ok := pm["version"]; ok {
					pm["version"] = version
					changed = true
				}
			}
		}
	}

	if !changed {
		return nil
	}

	output, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(output, '\n'), 0o644)
}
