package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inovacc/thimble/internal/plugin"
	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage runtime plugin tools",
	Long: `Manage JSON-based plugin tool definitions loaded from the plugins directory.

Install plugins from URLs, GitHub repos, or the plugin registry:
  thimble plugin install docker
  thimble plugin install --scope project docker
  thimble plugin install --scope local github.com/user/repo/my-plugin.json
  thimble plugin install https://example.com/plugin.json

List and remove installed plugins:
  thimble plugin list
  thimble plugin list --scope project
  thimble plugin remove my-plugin

Browse available plugins:
  thimble plugin search

Scopes:
  user    (default) — stored in {DataDir}/plugins/
  project — stored in {ProjectDir}/.thimble/plugins/ (committed to VCS)
  local   — stored in {ProjectDir}/.thimble/plugins.local/ (gitignored)`,
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins and their tools",
	RunE:  runPluginList,
}

var pluginDirCmd = &cobra.Command{
	Use:   "dir",
	Short: "Show the plugins directory path",
	RunE:  runPluginDir,
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <source>",
	Short: "Install a plugin from URL, GitHub path, or registry name",
	Long: `Install a plugin tool definition.

Sources:
  thimble plugin install docker                          # from registry (user scope)
  thimble plugin install --scope project docker          # project scope
  thimble plugin install --scope local docker            # local scope (gitignored)
  thimble plugin install github.com/user/repo/tool.json  # from GitHub
  thimble plugin install https://example.com/plugin.json # from URL`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginInstall,
}

var pluginRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an installed plugin",
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginRemove,
}

var pluginSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Browse available plugins from the registry",
	RunE:  runPluginSearch,
}

var pluginUpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update installed plugins from the registry",
	Long: `Check for plugin updates and install newer versions.

  thimble plugin update          # update all plugins
  thimble plugin update docker   # update specific plugin
  thimble plugin update --check  # just check, don't install`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPluginUpdate,
}

var pluginInitCmd = &cobra.Command{
	Use:   "init <name>",
	Short: "Scaffold a new plugin definition file",
	Long: `Create a minimal plugin JSON file in the current directory.

The generated file contains a starter tool definition that you can
customize. Tool names are prefixed with ctx_ as required by the
plugin system.

Example:
  thimble plugin init my-plugin
  # creates my-plugin.json in the current directory`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginInit,
}

var pluginTestCmd = &cobra.Command{
	Use:   "test [path.json]",
	Short: "Run the plugin testing framework against a plugin definition",
	Long: `Validate and test a plugin definition file.

Checks performed:
  - Schema validation (name, version, tools non-empty)
  - Tool name prefix (ctx_ required for all tools)
  - Command existence (verify tool commands exist on PATH)
  - Dry-run each tool command with a 5s timeout
  - Dependency check (declared dependencies exist in registry)

If no path is given, looks for *.json files in the current directory.

Exit code 0 if all checks pass, 1 if any check fails.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPluginTest,
}

var pluginValidateCmd = &cobra.Command{
	Use:   "validate <path>",
	Short: "Validate a plugin JSON file for errors and warnings",
	Long: `Validate a plugin definition file against the schema.

Checks for:
  - Valid JSON syntax
  - Required fields (name, tools with ctx_ prefix, command)
  - Duplicate tool names
  - Missing metadata (version, description, author, license)

Exit code 0 if valid (warnings only), 1 if errors found.`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginValidate,
}

func init() {
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginDirCmd)
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginRemoveCmd)
	pluginCmd.AddCommand(pluginSearchCmd)
	pluginCmd.AddCommand(pluginUpdateCmd)
	pluginCmd.AddCommand(pluginInitCmd)
	pluginCmd.AddCommand(pluginTestCmd)
	pluginCmd.AddCommand(pluginValidateCmd)
	pluginUpdateCmd.Flags().Bool("check", false, "Only check for updates without installing")
	pluginUpdateCmd.Flags().Bool("major", false, "Allow major version updates (may have breaking changes)")

	// Scope flags for install, remove, list, and dir.
	pluginInstallCmd.Flags().StringP("scope", "s", "user", "Installation scope: user, project, or local")
	pluginRemoveCmd.Flags().StringP("scope", "s", "user", "Scope to remove from: user, project, or local")
	pluginListCmd.Flags().StringP("scope", "s", "", "Filter by scope: user, project, or local (empty = all)")
	pluginDirCmd.Flags().StringP("scope", "s", "user", "Scope to show directory for: user, project, or local")

	rootCmd.AddCommand(pluginCmd)
}

// resolveScope parses the --scope flag and returns the scope + project directory.
// For project/local scopes, it uses the current working directory as projectDir.
func resolveScope(cmd *cobra.Command) (plugin.Scope, string, error) {
	scopeStr, _ := cmd.Flags().GetString("scope")

	scope, err := plugin.ParseScope(scopeStr)
	if err != nil {
		return "", "", err
	}

	var projectDir string
	if scope == plugin.ScopeProject || scope == plugin.ScopeLocal {
		projectDir, err = os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("get working directory: %w", err)
		}
	}

	return scope, projectDir, nil
}

func runPluginList(cmd *cobra.Command, _ []string) error {
	scopeFilter, _ := cmd.Flags().GetString("scope")

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// If a specific scope is requested, show only that scope.
	if scopeFilter != "" {
		scope, err := plugin.ParseScope(scopeFilter)
		if err != nil {
			return err
		}

		dir := plugin.ScopeDir(scope, projectDir)

		plugins, err := plugin.LoadPlugins(dir)
		if err != nil {
			return fmt.Errorf("load plugins: %w", err)
		}

		if len(plugins) == 0 {
			_, _ = fmt.Fprintf(os.Stdout, "No plugins installed in %s scope.\n", scope)
			_, _ = fmt.Fprintf(os.Stdout, "Plugin directory: %s\n", dir)

			return nil
		}

		_, _ = fmt.Fprintf(os.Stdout, "%d plugin(s) in %s scope:\n\n", len(plugins), scope)

		for _, p := range plugins {
			printPlugin(p)
		}

		return nil
	}

	// Show all scopes.
	allPlugins, err := plugin.LoadAllScopes(projectDir)
	if err != nil {
		return fmt.Errorf("load plugins: %w", err)
	}

	if len(allPlugins) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No plugins installed.")
		_, _ = fmt.Fprintf(os.Stdout, "Plugin directory: %s\n", plugin.PluginDir())
		_, _ = fmt.Fprintln(os.Stdout, "\nInstall plugins with: thimble plugin install <name>")
		_, _ = fmt.Fprintln(os.Stdout, "Browse available:     thimble plugin search")

		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "%d plugin(s) installed:\n\n", len(allPlugins))

	// Detect conflicts to annotate tool listings.
	conflicts := plugin.DetectConflicts(allPlugins, pluginBuiltinNames())
	conflictMap := make(map[string]map[string]string) // plugin -> tool -> conflicts_with

	for _, c := range conflicts {
		if conflictMap[c.PluginName] == nil {
			conflictMap[c.PluginName] = make(map[string]string)
		}

		conflictMap[c.PluginName][c.ToolName] = c.ConflictsWith
	}

	for _, sp := range allPlugins {
		_, _ = fmt.Fprintf(os.Stdout, "  %s (v%s) [%s] — %d tool(s)\n", sp.Name, sp.Version, sp.Scope, len(sp.Tools))

		if sp.Description != "" {
			_, _ = fmt.Fprintf(os.Stdout, "    %s\n", sp.Description)
		}

		if sp.Author != nil && sp.Author.Name != "" {
			_, _ = fmt.Fprintf(os.Stdout, "    by %s\n", sp.Author.Name)
		}

		if sp.License != "" {
			_, _ = fmt.Fprintf(os.Stdout, "    license: %s\n", sp.License)
		}

		for _, t := range sp.Tools {
			if cw, ok := conflictMap[sp.Name][t.Name]; ok {
				_, _ = fmt.Fprintf(os.Stdout, "    - %s: %s [CONFLICT: %s]\n", t.Name, t.Description, cw)
			} else {
				_, _ = fmt.Fprintf(os.Stdout, "    - %s: %s\n", t.Name, t.Description)
			}
		}
	}

	if len(conflicts) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "\n%d conflict(s) detected. Conflicting tools are skipped at runtime.\n", len(conflicts))
	}

	return nil
}

// pluginBuiltinNames returns the well-known built-in tool names for conflict
// detection in the CLI layer. This avoids importing the mcp package which
// carries heavy dependencies; we duplicate the authoritative list here.
func pluginBuiltinNames() []string {
	return []string{
		"ctx_execute", "ctx_execute_file", "ctx_index", "ctx_search",
		"ctx_fetch_and_index", "ctx_batch_execute", "ctx_stats", "ctx_doctor",
		"ctx_analyze", "ctx_symbols", "ctx_upgrade",
		"ctx_delegate", "ctx_delegate_status", "ctx_delegate_cancel", "ctx_delegate_list",
		"ctx_report_list", "ctx_report_show", "ctx_report_delete",
		"ctx_git_status", "ctx_git_diff", "ctx_git_log", "ctx_git_blame",
		"ctx_git_branches", "ctx_git_stash", "ctx_git_commit", "ctx_git_changelog",
		"ctx_git_merge", "ctx_git_rebase", "ctx_git_validate_branch", "ctx_git_lint_commit",
		"ctx_gh", "ctx_gh_pr_status", "ctx_gh_run_status", "ctx_gh_issue_list",
		"ctx_gh_search", "ctx_gh_api", "ctx_gh_repo_view",
		"ctx_lint", "ctx_lint_fix", "ctx_plugin_conflicts",
		"ctx_github_pr_list", "ctx_github_pr_get", "ctx_github_pr_create",
		"ctx_github_issue_list", "ctx_github_issue_get", "ctx_github_issue_create",
	}
}

func runPluginDir(cmd *cobra.Command, _ []string) error {
	scope, projectDir, err := resolveScope(cmd)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(os.Stdout, plugin.ScopeDir(scope, projectDir))

	return nil
}

func runPluginInstall(cmd *cobra.Command, args []string) error {
	scope, projectDir, err := resolveScope(cmd)
	if err != nil {
		return err
	}

	source := args[0]
	_, _ = fmt.Fprintf(os.Stdout, "Installing plugin from %s (scope: %s)...\n", source, scope)

	p, err := plugin.InstallToScope(source, scope, projectDir)
	if err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Installed %s (v%s) with %d tool(s):\n", p.Name, p.Version, len(p.Tools))

	for _, t := range p.Tools {
		_, _ = fmt.Fprintf(os.Stdout, "  - %s: %s\n", t.Name, t.Description)
	}

	_, _ = fmt.Fprintln(os.Stdout, "\nRestart your MCP session to load the new tools.")

	return nil
}

func runPluginRemove(cmd *cobra.Command, args []string) error {
	scope, projectDir, err := resolveScope(cmd)
	if err != nil {
		return err
	}

	name := args[0]
	if err := plugin.RemoveFromScope(name, scope, projectDir); err != nil {
		return fmt.Errorf("remove failed: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Removed plugin %q from %s scope.\n", name, scope)
	_, _ = fmt.Fprintln(os.Stdout, "Restart your MCP session to unload the tools.")

	return nil
}

func runPluginUpdate(cmd *cobra.Command, args []string) error {
	checkOnly, _ := cmd.Flags().GetBool("check")
	allowMajor, _ := cmd.Flags().GetBool("major")

	var name string
	if len(args) > 0 {
		name = args[0]
	}

	// Fetch registry once for both check and update paths.
	_, _ = fmt.Fprintln(os.Stdout, "Fetching plugin registry...")

	idx, err := plugin.FetchRegistry()
	if err != nil {
		return fmt.Errorf("fetch registry: %w", err)
	}

	dir := plugin.PluginDir()

	// Use dry-run mode for --check, otherwise install.
	results, err := plugin.UpdatePluginsWithCheck(dir, idx.Plugins, checkOnly)
	if err != nil {
		return fmt.Errorf("check updates: %w", err)
	}

	// Filter by name if specified.
	if name != "" {
		var filtered []plugin.UpdateResult

		for _, r := range results {
			if r.Name == name {
				filtered = append(filtered, r)
			}
		}

		if len(filtered) == 0 {
			return fmt.Errorf("plugin %q is not installed", name)
		}

		results = filtered
	}

	if checkOnly {
		return printCheckResults(results)
	}

	// For non-dry-run: if --major is not set, we need to re-run skipping majors.
	// Since UpdatePluginsWithCheck already installed, we handle this by doing a dry-run
	// first, then selectively installing.
	if !allowMajor {
		// Re-run as dry-run to classify, then install non-major only.
		dryResults, err := plugin.UpdatePluginsWithCheck(dir, idx.Plugins, true)
		if err != nil {
			return fmt.Errorf("check updates: %w", err)
		}

		if name != "" {
			var filtered []plugin.UpdateResult

			for _, r := range dryResults {
				if r.Name == name {
					filtered = append(filtered, r)
				}
			}

			dryResults = filtered
		}

		return installFilteredUpdates(dryResults, false)
	}

	// --major is set: do full install.
	dryResults, err := plugin.UpdatePluginsWithCheck(dir, idx.Plugins, true)
	if err != nil {
		return fmt.Errorf("check updates: %w", err)
	}

	if name != "" {
		var filtered []plugin.UpdateResult

		for _, r := range dryResults {
			if r.Name == name {
				filtered = append(filtered, r)
			}
		}

		dryResults = filtered
	}

	return installFilteredUpdates(dryResults, true)
}

// printCheckResults displays the list of available updates without installing.
func printCheckResults(results []plugin.UpdateResult) error {
	hasUpdates := false

	for _, r := range results {
		if r.UpdateType != "" {
			hasUpdates = true

			break
		}
	}

	if !hasUpdates {
		_, _ = fmt.Fprintln(os.Stdout, "All plugins are up to date.")
		return nil
	}

	_, _ = fmt.Fprintln(os.Stdout, "")

	for _, r := range results {
		if r.UpdateType == "" {
			_, _ = fmt.Fprintf(os.Stdout, "  %-20s %s (up to date)\n", r.Name, r.InstalledVer)
			continue
		}

		line := fmt.Sprintf("  %-20s %s -> %s (%s)", r.Name, r.InstalledVer, r.AvailableVer, r.UpdateType)
		if r.Breaking {
			line += " ⚠ BREAKING"
		}

		_, _ = fmt.Fprintln(os.Stdout, line)
	}

	_, _ = fmt.Fprintln(os.Stdout, "\nRun 'thimble plugin update' to install updates.")
	_, _ = fmt.Fprintln(os.Stdout, "Use 'thimble plugin update --major' to include major version updates.")

	return nil
}

// installFilteredUpdates installs updates, skipping major bumps unless allowMajor is true.
func installFilteredUpdates(results []plugin.UpdateResult, allowMajor bool) error {
	updated := 0
	skipped := 0

	for _, r := range results {
		if r.UpdateType == "" {
			_, _ = fmt.Fprintf(os.Stdout, "  %s: %s (up to date)\n", r.Name, r.InstalledVer)
			continue
		}

		if r.Breaking && !allowMajor {
			_, _ = fmt.Fprintf(os.Stderr, "  ⚠ %s: v%s → v%s (major update, may have breaking changes) — skipped\n",
				r.Name, r.InstalledVer, r.AvailableVer)

			skipped++

			continue
		}

		// Install the update.
		_, err := plugin.Install(r.Name)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "  %s: error: %v\n", r.Name, err)
			continue
		}

		label := r.UpdateType
		if r.Breaking {
			label += ", breaking"
		}

		_, _ = fmt.Fprintf(os.Stdout, "  %s: %s -> %s (updated, %s)\n", r.Name, r.InstalledVer, r.AvailableVer, label)
		updated++
	}

	if skipped > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "\n%d major update(s) skipped. Use --major to allow breaking updates.\n", skipped)
	}

	if updated > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "\n%d plugin(s) updated. Restart your MCP session to load changes.\n", updated)
	} else if skipped == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "\nAll plugins are up to date.")
	}

	return nil
}

func runPluginInit(_ *cobra.Command, args []string) error {
	name := args[0]

	p := plugin.ScaffoldPlugin(name)

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugin: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	dest := filepath.Join(cwd, name+".json")

	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("file already exists: %s", dest)
	}

	if err := os.WriteFile(dest, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write plugin file: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Created %s\n", dest)

	return nil
}

func runPluginTest(_ *cobra.Command, args []string) error {
	var path string

	if len(args) > 0 {
		path = args[0]
	} else {
		// Look for *.json in current directory.
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		matches, err := filepath.Glob(filepath.Join(cwd, "*.json"))
		if err != nil {
			return fmt.Errorf("glob: %w", err)
		}

		if len(matches) == 0 {
			return fmt.Errorf("no *.json files found in current directory")
		}

		if len(matches) > 1 {
			return fmt.Errorf("multiple *.json files found; specify one: %v", matches)
		}

		path = matches[0]
	}

	_, _ = fmt.Fprintf(os.Stdout, "Testing plugin: %s\n\n", path)

	def, err := plugin.LoadPluginFile(path)
	if err != nil {
		return fmt.Errorf("load plugin: %w", err)
	}

	// Try to fetch registry for dependency checks (non-fatal if offline).
	var registry []plugin.RegistryEntry

	idx, regErr := plugin.FetchRegistry()
	if regErr == nil && idx != nil {
		registry = idx.Plugins
	}

	results := plugin.TestPlugin(def, registry)

	passed, failed := 0, 0

	for _, r := range results {
		var prefix, suffix string

		if r.Status == "pass" {
			prefix = "PASS"
			passed++
		} else {
			prefix = "FAIL"
			failed++
		}

		if r.Error != "" {
			suffix = " — " + r.Error
		}

		if r.ToolName != "" {
			_, _ = fmt.Fprintf(os.Stdout, "  [%s] %s (%s)%s\n", prefix, r.Check, r.ToolName, suffix)
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "  [%s] %s%s\n", prefix, r.Check, suffix)
		}
	}

	_, _ = fmt.Fprintf(os.Stdout, "\n%d passed, %d failed\n", passed, failed)

	if failed > 0 {
		return fmt.Errorf("%d test(s) failed", failed)
	}

	return nil
}

func runPluginValidate(_ *cobra.Command, args []string) error {
	path := args[0]
	_, _ = fmt.Fprintf(os.Stdout, "Validating %s...\n\n", path)

	result, err := plugin.Validate(path)
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	errors, warnings := 0, 0

	for _, issue := range result.Issues {
		switch issue.Level {
		case "error":
			_, _ = fmt.Fprintf(os.Stdout, "  ERROR:   %s\n", issue.Message)
			errors++
		case "warning":
			_, _ = fmt.Fprintf(os.Stdout, "  WARNING: %s\n", issue.Message)
			warnings++
		}
	}

	_, _ = fmt.Fprintln(os.Stdout)

	if result.Valid {
		_, _ = fmt.Fprintf(os.Stdout, "Valid (%d warning(s))\n", warnings)
		return nil
	}

	return fmt.Errorf("invalid: %d error(s), %d warning(s)", errors, warnings)
}

func runPluginSearch(_ *cobra.Command, _ []string) error {
	_, _ = fmt.Fprintln(os.Stdout, "Fetching plugin registry...")

	idx, err := plugin.FetchRegistry()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Registry not available: %v\n", err)

		_, _ = fmt.Fprintln(os.Stdout, "\nYou can still install plugins directly:")
		_, _ = fmt.Fprintln(os.Stdout, "  thimble plugin install https://example.com/plugin.json")
		_, _ = fmt.Fprintln(os.Stdout, "  thimble plugin install github.com/user/repo/plugin.json")

		return nil
	}

	if len(idx.Plugins) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No plugins in registry yet.")
		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "\n%d plugin(s) available:\n\n", len(idx.Plugins))

	for _, p := range idx.Plugins {
		_, _ = fmt.Fprintf(os.Stdout, "  %-20s v%-8s %s\n", p.Name, p.Version, p.Description)

		if p.Author != "" {
			_, _ = fmt.Fprintf(os.Stdout, "  %-20s          by %s\n", "", p.Author)
		}
	}

	_, _ = fmt.Fprintln(os.Stdout, "\nInstall with: thimble plugin install <name>")

	return nil
}

// printPlugin prints a single plugin's details to stdout.
func printPlugin(p plugin.PluginDef) {
	_, _ = fmt.Fprintf(os.Stdout, "  %s (v%s) — %d tool(s)\n", p.Name, p.Version, len(p.Tools))

	if p.Description != "" {
		_, _ = fmt.Fprintf(os.Stdout, "    %s\n", p.Description)
	}

	if p.Author != nil && p.Author.Name != "" {
		_, _ = fmt.Fprintf(os.Stdout, "    by %s\n", p.Author.Name)
	}

	if p.License != "" {
		_, _ = fmt.Fprintf(os.Stdout, "    license: %s\n", p.License)
	}

	for _, t := range p.Tools {
		_, _ = fmt.Fprintf(os.Stdout, "    - %s: %s\n", t.Name, t.Description)
	}
}
