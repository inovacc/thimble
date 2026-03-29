// Package plugin provides runtime tool registration from JSON plugin definition files.
// Plugins are loaded from the plugins directory under the thimble data directory
// ({DataDir}/plugins/) and define MCP tools that execute shell commands.
//
// Plugins can be installed from URLs or GitHub repos:
//
//	thimble plugin install https://example.com/my-plugin.json
//	thimble plugin install github.com/user/repo/plugin.json
//	thimble plugin install github.com/user/thimble-plugins/tools/docker.json
package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/inovacc/thimble/internal/paths"
)

// ValidHookEvents lists the lifecycle events that plugins can hook into.
var ValidHookEvents = map[string]bool{
	"PreToolUse":   true,
	"PostToolUse":  true,
	"SessionStart": true,
	"PreCompact":   true,
}

// PluginHook defines a hook command that runs on a lifecycle event.
type PluginHook struct {
	Matcher string `json:"matcher,omitempty"` // regex pattern to match tool names (for Pre/PostToolUse)
	Command string `json:"command"`           // shell command to execute
}

// Scope represents where a plugin is installed.
type Scope string

const (
	// ScopeUser is the default scope — plugins stored in {DataDir}/plugins/.
	ScopeUser Scope = "user"
	// ScopeProject stores plugins in {ProjectDir}/.thimble/plugins/ (committed to VCS).
	ScopeProject Scope = "project"
	// ScopeLocal stores plugins in {ProjectDir}/.thimble/plugins.local/ (gitignored).
	ScopeLocal Scope = "local"
)

// ParseScope converts a string to a Scope, returning an error for invalid values.
func ParseScope(s string) (Scope, error) {
	switch Scope(s) {
	case ScopeUser, ScopeProject, ScopeLocal:
		return Scope(s), nil
	default:
		return "", fmt.Errorf("invalid scope %q: must be user, project, or local", s)
	}
}

// ScopeDir returns the plugin directory for a given scope and project directory.
// For ScopeProject and ScopeLocal, projectDir must be non-empty.
func ScopeDir(scope Scope, projectDir string) string {
	switch scope {
	case ScopeUser:
		return PluginDir()
	case ScopeProject:
		return filepath.Join(projectDir, ".thimble", "plugins")
	case ScopeLocal:
		return filepath.Join(projectDir, ".thimble", "plugins.local")
	}

	return PluginDir()
}

// ScopedPlugin pairs a PluginDef with the scope it was loaded from.
type ScopedPlugin struct {
	PluginDef

	Scope Scope
}

// LoadAllScopes loads plugins from all scopes, with project/local overriding user.
// Plugins from narrower scopes (local > project > user) take priority by name.
func LoadAllScopes(projectDir string) ([]ScopedPlugin, error) {
	type scopeEntry struct {
		scope Scope
		dir   string
	}

	scopes := []scopeEntry{
		{ScopeUser, PluginDir()},
	}

	if projectDir != "" {
		scopes = append(scopes,
			scopeEntry{ScopeProject, ScopeDir(ScopeProject, projectDir)},
			scopeEntry{ScopeLocal, ScopeDir(ScopeLocal, projectDir)},
		)
	}

	seen := make(map[string]int) // plugin name -> index in result

	var result []ScopedPlugin

	for _, se := range scopes {
		plugins, err := LoadPlugins(se.dir)
		if err != nil {
			return nil, fmt.Errorf("load %s plugins: %w", se.scope, err)
		}

		for _, p := range plugins {
			sp := ScopedPlugin{PluginDef: p, Scope: se.scope}

			if idx, exists := seen[p.Name]; exists {
				// Narrower scope overrides broader scope.
				result[idx] = sp
			} else {
				seen[p.Name] = len(result)
				result = append(result, sp)
			}
		}
	}

	return result, nil
}

// InstallToScope downloads a plugin and saves it to the directory for the given scope.
// For project/local scopes, projectDir must be non-empty.
// If the plugin declares dependencies, they are resolved against the registry and
// any missing ones are auto-installed to the same scope before the requested plugin.
func InstallToScope(source string, scope Scope, projectDir string) (*PluginDef, error) {
	dir := ScopeDir(scope, projectDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create plugin dir: %w", err)
	}

	url := resolveSource(source)

	body, err := httpGet(url)
	if err != nil {
		return nil, fmt.Errorf("download plugin: %w", err)
	}

	// Validate before saving.
	var p PluginDef
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("invalid plugin JSON: %w", err)
	}

	if p.Name == "" {
		return nil, fmt.Errorf("plugin has no name")
	}

	if len(p.Tools) == 0 {
		return nil, fmt.Errorf("plugin %q has no tools", p.Name)
	}

	for _, t := range p.Tools {
		if !strings.HasPrefix(t.Name, "ctx_") {
			return nil, fmt.Errorf("tool %q must have ctx_ prefix", t.Name)
		}
	}

	// Resolve and install dependencies before saving the requested plugin.
	if len(p.Dependencies) > 0 {
		if err := installDependencies(p, scope, projectDir); err != nil {
			return nil, fmt.Errorf("install dependencies for %q: %w", p.Name, err)
		}
	}

	// Save with the plugin name as filename.
	dest := filepath.Join(dir, p.Name+".json")

	// Pretty-print for readability.
	formatted, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		formatted = body
	}

	if err := os.WriteFile(dest, formatted, 0o644); err != nil {
		return nil, fmt.Errorf("save plugin: %w", err)
	}

	return &p, nil
}

// installDependencies resolves and installs missing dependencies for a plugin.
// It fetches the registry, resolves the dependency order, skips already-installed
// plugins, and installs each missing dependency to the same scope.
func installDependencies(p PluginDef, scope Scope, projectDir string) error {
	dir := ScopeDir(scope, projectDir)

	// Build installed map from the target scope directory.
	existing, err := LoadPlugins(dir)
	if err != nil {
		return fmt.Errorf("load installed plugins: %w", err)
	}

	installed := make(map[string]PluginDef, len(existing))
	for _, ep := range existing {
		installed[ep.Name] = ep
	}

	// Fetch registry for dependency lookup.
	idx, err := FetchRegistry()
	if err != nil {
		return fmt.Errorf("fetch registry for dependency resolution: %w", err)
	}

	toInstall, err := ResolveDependencies(p, idx.Plugins, installed)
	if err != nil {
		return err
	}

	if len(toInstall) == 0 {
		return nil
	}

	for _, depName := range toInstall {
		slog.Info("auto-installing dependency", "dependency", depName, "required_by", p.Name)

		// Install the dependency using the registry name (recursive call handles
		// its own deps, but since ResolveDependencies already ordered them we
		// install in order and each subsequent call will find prior deps present).
		if _, err := InstallToScope(depName, scope, projectDir); err != nil {
			return fmt.Errorf("install dependency %q: %w", depName, err)
		}
	}

	return nil
}

// RemoveFromScope deletes a plugin by name from the directory for the given scope.
func RemoveFromScope(name string, scope Scope, projectDir string) error {
	dir := ScopeDir(scope, projectDir)

	path := filepath.Join(dir, name+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("plugin %q not found in %s scope", name, scope)
	}

	return os.Remove(path)
}

// PluginConflict describes a tool name collision between a plugin tool and
// either a built-in tool or another plugin's tool.
type PluginConflict struct {
	PluginName    string `json:"plugin_name"`
	ToolName      string `json:"tool_name"`
	ConflictsWith string `json:"conflicts_with"` // "built-in" or other plugin name
}

// DetectConflicts checks all plugin tools against the set of built-in tool names
// and against each other. Returns a slice of conflicts found.
func DetectConflicts(plugins []ScopedPlugin, builtinTools []string) []PluginConflict {
	builtinSet := make(map[string]bool, len(builtinTools))
	for _, name := range builtinTools {
		builtinSet[name] = true
	}

	var conflicts []PluginConflict

	// Track which plugin first registered each tool name.
	toolOwner := make(map[string]string) // tool name -> plugin name

	for _, sp := range plugins {
		for _, td := range sp.Tools {
			// Check against built-in tools.
			if builtinSet[td.Name] {
				conflicts = append(conflicts, PluginConflict{
					PluginName:    sp.Name,
					ToolName:      td.Name,
					ConflictsWith: "built-in",
				})

				continue
			}

			// Check against other plugins.
			if owner, exists := toolOwner[td.Name]; exists {
				conflicts = append(conflicts, PluginConflict{
					PluginName:    sp.Name,
					ToolName:      td.Name,
					ConflictsWith: owner,
				})

				continue
			}

			toolOwner[td.Name] = sp.Name
		}
	}

	return conflicts
}

// Author describes a plugin author.
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// PluginDependency declares a dependency on another plugin.
type PluginDependency struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"` // semver constraint: ">=1.0.0", "^2.1", exact "1.2.3", or empty (any)
	Optional bool   `json:"optional,omitempty"`
}

// EventSubscription declares that a plugin wants to receive an async event bus event.
// When the event fires, Command is executed in a goroutine with a 1s timeout.
type EventSubscription struct {
	Event   string `json:"event"`   // bus event name (e.g. "tool.post", "plugin.installed")
	Command string `json:"command"` // shell command to execute
}

// PluginDef defines a plugin with one or more tool definitions.
type PluginDef struct {
	Name          string                  `json:"name"`
	Version       string                  `json:"version"`
	Description   string                  `json:"description,omitempty"`
	Author        *Author                 `json:"author,omitempty"`
	Homepage      string                  `json:"homepage,omitempty"`
	Repository    string                  `json:"repository,omitempty"`
	License       string                  `json:"license,omitempty"`
	Keywords      []string                `json:"keywords,omitempty"`
	Dependencies  []PluginDependency      `json:"dependencies,omitempty"`
	Tools         []ToolDef               `json:"tools"`
	Hooks         map[string][]PluginHook `json:"hooks,omitempty"`         // event name -> hooks
	Subscriptions []EventSubscription     `json:"subscriptions,omitempty"` // async event bus subscriptions
	Sandbox       *SandboxConfig          `json:"sandbox,omitempty"`       // optional sandbox restrictions
}

// ValidateDependencies checks that all required dependencies are installed and
// satisfy version constraints. Returns the names of unmet dependencies.
func (p *PluginDef) ValidateDependencies(installed map[string]PluginDef) []string {
	var unmet []string

	for _, dep := range p.Dependencies {
		inst, ok := installed[dep.Name]
		if !ok {
			if !dep.Optional {
				unmet = append(unmet, dep.Name)
			}

			continue
		}

		if dep.Version != "" && !SatisfiesConstraint(inst.Version, dep.Version) {
			if !dep.Optional {
				unmet = append(unmet, dep.Name)
			}
		}
	}

	return unmet
}

// ToolPermissions defines per-tool security permissions within a plugin sandbox.
// All permissions default to false (restrictive) when nil or omitted.
type ToolPermissions struct {
	AllowNetwork    bool `json:"allow_network,omitempty"`    // allow curl/wget/fetch commands
	AllowFileWrite  bool `json:"allow_file_write,omitempty"` // allow output redirection (>, >>)
	AllowSubprocess bool `json:"allow_subprocess,omitempty"` // allow pipes (|) and command substitution ($())
	MaxTimeout      int  `json:"max_timeout_sec,omitempty"`  // per-tool timeout override (seconds); 0 = use plugin default
}

// ToolDef defines a single tool within a plugin.
type ToolDef struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Command     string                   `json:"command"`
	InputSchema map[string]InputFieldDef `json:"input_schema"`
	WorkingDir  string                   `json:"working_dir,omitempty"`
	Timeout     int                      `json:"timeout_ms,omitempty"`
	Permissions *ToolPermissions         `json:"permissions,omitempty"`
}

// InputFieldDef describes a single input field for a plugin tool.
type InputFieldDef struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
}

// ValidationIssue describes a single validation problem.
type ValidationIssue struct {
	Level   string // "error" or "warning"
	Message string
}

// ValidationResult holds the outcome of plugin validation.
type ValidationResult struct {
	Valid  bool
	Issues []ValidationIssue
}

// Validate checks a plugin file for errors and best-practice warnings.
// Returns a result with all issues found. Valid is true when there are no errors.
func Validate(path string) (*ValidationResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var result ValidationResult

	addError := func(msg string) {
		result.Issues = append(result.Issues, ValidationIssue{Level: "error", Message: msg})
	}

	addWarning := func(msg string) {
		result.Issues = append(result.Issues, ValidationIssue{Level: "warning", Message: msg})
	}

	var p PluginDef
	if err := json.Unmarshal(data, &p); err != nil {
		addError(fmt.Sprintf("invalid JSON: %v", err))
		return &result, nil
	}

	// Required fields.
	if p.Name == "" {
		addError("name is required")
	}

	if len(p.Tools) == 0 {
		addError(fmt.Sprintf("plugin %q has no tools", p.Name))
	}

	// Tool validation.
	toolNames := make(map[string]bool, len(p.Tools))

	for i, t := range p.Tools {
		if t.Name == "" {
			addError(fmt.Sprintf("tool %d has no name", i))
			continue
		}

		if !strings.HasPrefix(t.Name, "ctx_") {
			addError(fmt.Sprintf("tool %q must have ctx_ prefix", t.Name))
		}

		if t.Command == "" {
			addError(fmt.Sprintf("tool %q has no command", t.Name))
		}

		if t.Description == "" {
			addWarning(fmt.Sprintf("tool %q has no description", t.Name))
		}

		if toolNames[t.Name] {
			addError(fmt.Sprintf("duplicate tool name %q", t.Name))
		}

		toolNames[t.Name] = true
	}

	// Metadata warnings.
	if p.Version == "" {
		addWarning("version is recommended (e.g. \"1.0.0\")")
	}

	if p.Description == "" {
		addWarning("description is recommended for discoverability")
	}

	if p.Author == nil {
		addWarning("author is recommended for attribution")
	}

	if p.License == "" {
		addWarning("license is recommended (e.g. \"MIT\", \"Apache-2.0\")")
	}

	// Hook validation.
	for event, hks := range p.Hooks {
		if !ValidHookEvents[event] {
			addError(fmt.Sprintf("hook event %q is not valid (must be one of PreToolUse, PostToolUse, SessionStart, PreCompact)", event))
		}

		for i, h := range hks {
			if h.Command == "" {
				addError(fmt.Sprintf("hook %d for event %q has no command", i, event))
			}

			if h.Matcher != "" {
				if _, err := regexp.Compile(h.Matcher); err != nil {
					addError(fmt.Sprintf("hook %d for event %q has invalid matcher regex: %v", i, event, err))
				}
			}
		}
	}

	// Subscription validation.
	for i, sub := range p.Subscriptions {
		if sub.Event == "" {
			addError(fmt.Sprintf("subscription %d has no event", i))
		} else if !ValidBusEvents[sub.Event] {
			addError(fmt.Sprintf("subscription %d has invalid event %q", i, sub.Event))
		}

		if sub.Command == "" {
			addError(fmt.Sprintf("subscription %d for event %q has no command", i, sub.Event))
		}
	}

	// Valid if no errors.
	result.Valid = true

	for _, issue := range result.Issues {
		if issue.Level == "error" {
			result.Valid = false
			break
		}
	}

	return &result, nil
}

// ScaffoldPlugin returns a minimal valid PluginDef for the given name.
// The result can be serialized to JSON and used as a starting point for a new plugin.
func ScaffoldPlugin(name string) *PluginDef {
	return &PluginDef{
		Name:        name,
		Version:     "0.1.0",
		Description: "TODO: describe what " + name + " does",
		Tools: []ToolDef{
			{
				Name:        "ctx_" + name + "_hello",
				Description: "Say hello from " + name,
				Command:     "echo \"Hello from " + name + "\"",
				InputSchema: map[string]InputFieldDef{},
			},
		},
		Dependencies: []PluginDependency{},
	}
}

// PluginDir returns the plugins directory path under the thimble data directory.
func PluginDir() string {
	return filepath.Join(paths.DataDir(), "plugins")
}

// PluginDataDir returns the persistent data directory for a named plugin.
// The directory is created if it does not exist. Path: {DataDir}/plugin-data/{name}/
func PluginDataDir(name string) string {
	dir := filepath.Join(paths.DataDir(), "plugin-data", name)
	_ = os.MkdirAll(dir, 0o755)

	return dir
}

// LoadPlugins scans the given directory for *.json files and parses each as a PluginDef.
// Returns all successfully parsed plugins. Invalid files are logged and skipped.
func LoadPlugins(dir string) ([]PluginDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read plugin dir %q: %w", dir, err)
	}

	var plugins []PluginDef

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		p, err := LoadPluginFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			slog.Warn("skipping invalid plugin file",
				"file", entry.Name(),
				"error", err)

			continue
		}

		plugins = append(plugins, *p)
	}

	return plugins, nil
}

// LoadPluginFile reads and validates a single plugin JSON file.
func LoadPluginFile(path string) (*PluginDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var p PluginDef
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	if p.Name == "" {
		return nil, fmt.Errorf("plugin name is required")
	}

	if len(p.Tools) == 0 {
		return nil, fmt.Errorf("plugin %q has no tools", p.Name)
	}

	for i, t := range p.Tools {
		if t.Name == "" {
			return nil, fmt.Errorf("tool %d in plugin %q has no name", i, p.Name)
		}

		if !strings.HasPrefix(t.Name, "ctx_") {
			return nil, fmt.Errorf("tool %q must have ctx_ prefix", t.Name)
		}

		if t.Command == "" {
			return nil, fmt.Errorf("tool %q has no command", t.Name)
		}
	}

	// Validate subscriptions.
	for i, sub := range p.Subscriptions {
		if sub.Event == "" {
			return nil, fmt.Errorf("plugin %q: subscription %d has no event", p.Name, i)
		}

		if !ValidBusEvents[sub.Event] {
			return nil, fmt.Errorf("plugin %q: subscription %d has invalid event %q", p.Name, i, sub.Event)
		}

		if sub.Command == "" {
			return nil, fmt.Errorf("plugin %q: subscription %d for event %q has no command", p.Name, i, sub.Event)
		}
	}

	// Validate hooks.
	for event, hks := range p.Hooks {
		if !ValidHookEvents[event] {
			return nil, fmt.Errorf("plugin %q: hook event %q is not valid", p.Name, event)
		}

		for i, h := range hks {
			if h.Command == "" {
				return nil, fmt.Errorf("plugin %q: hook %d for event %q has no command", p.Name, i, event)
			}

			if h.Matcher != "" {
				if _, err := regexp.Compile(h.Matcher); err != nil {
					return nil, fmt.Errorf("plugin %q: hook %d for event %q has invalid matcher: %w", p.Name, i, event, err)
				}
			}
		}
	}

	return &p, nil
}

const (
	// RegistryBaseURL is the default raw content URL for the plugin registry.
	RegistryBaseURL = "https://raw.githubusercontent.com/inovacc/thimble-plugins/main"
	// RegistryIndexFile is the index file listing available plugins.
	RegistryIndexFile = "registry.json"

	installTimeout = 30 * time.Second
)

// RegistryEntry describes a plugin available in the registry.
type RegistryEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	File        string `json:"file"` // path relative to registry root (e.g. "plugins/docker.json")
	Author      string `json:"author,omitempty"`
}

// RegistryIndex is the top-level structure of registry.json.
type RegistryIndex struct {
	Plugins []RegistryEntry `json:"plugins"`
}

// FetchRegistry downloads and parses the plugin registry index.
func FetchRegistry() (*RegistryIndex, error) {
	url := RegistryBaseURL + "/" + RegistryIndexFile

	body, err := httpGet(url)
	if err != nil {
		return nil, fmt.Errorf("fetch registry: %w", err)
	}

	var idx RegistryIndex
	if err := json.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}

	return &idx, nil
}

// Install downloads a plugin from a URL and saves it to the plugins directory.
// The source can be:
//   - A full URL: https://example.com/plugin.json
//   - A GitHub path: github.com/user/repo/path/to/plugin.json
//   - A registry name: "docker" (looks up in registry.json)
func Install(source string) (*PluginDef, error) {
	return InstallToScope(source, ScopeUser, "")
}

// UpdateResult describes what happened when checking/updating a plugin.
type UpdateResult struct {
	Name         string
	InstalledVer string
	AvailableVer string
	UpdateType   string // "patch", "minor", "major", or "" if no update
	Breaking     bool   // true for major version bumps
	Updated      bool
	Error        error
}

// ClassifyUpdate compares two semver strings and returns "patch", "minor", or "major".
// If versions are equal or oldVersion is newer, returns "".
func ClassifyUpdate(oldVersion, newVersion string) string {
	if CompareVersions(oldVersion, newVersion) >= 0 {
		return ""
	}

	oldParts := parseSemverParts(oldVersion)
	newParts := parseSemverParts(newVersion)

	if oldParts[0] != newParts[0] {
		return "major"
	}

	if oldParts[1] != newParts[1] {
		return "minor"
	}

	return "patch"
}

// parseSemverParts returns [major, minor, patch] as ints, padding missing parts with 0.
func parseSemverParts(version string) [3]int {
	s := strings.TrimPrefix(version, "v")
	parts := strings.SplitN(s, ".", 3)

	var result [3]int

	for i := 0; i < 3 && i < len(parts); i++ {
		result[i] = parseVersionPart(parts[i])
	}

	return result
}

// UpdatePluginsWithCheck compares installed plugins against the registry and optionally
// installs updates. Major version bumps are flagged as Breaking. When dryRun is true,
// no installation occurs — only the list of pending updates is returned.
func UpdatePluginsWithCheck(dir string, registry []RegistryEntry, dryRun bool) ([]UpdateResult, error) {
	installed, err := LoadPlugins(dir)
	if err != nil {
		return nil, fmt.Errorf("load installed plugins: %w", err)
	}

	if len(installed) == 0 {
		return nil, nil
	}

	regMap := make(map[string]RegistryEntry, len(registry))
	for _, entry := range registry {
		regMap[entry.Name] = entry
	}

	var results []UpdateResult

	for _, p := range installed {
		entry, ok := regMap[p.Name]
		if !ok {
			continue
		}

		updateType := ClassifyUpdate(p.Version, entry.Version)
		if updateType == "" {
			// Up to date.
			results = append(results, UpdateResult{
				Name:         p.Name,
				InstalledVer: p.Version,
				AvailableVer: entry.Version,
			})

			continue
		}

		r := UpdateResult{
			Name:         p.Name,
			InstalledVer: p.Version,
			AvailableVer: entry.Version,
			UpdateType:   updateType,
			Breaking:     updateType == "major",
		}

		if dryRun {
			results = append(results, r)
			continue
		}

		// Install the update.
		_, installErr := Install(entry.Name)
		if installErr != nil {
			r.Error = installErr
		} else {
			r.Updated = true
		}

		results = append(results, r)
	}

	return results, nil
}

// CompareVersions compares two semver-like version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareVersions(a, b string) int {
	aParts := strings.Split(strings.TrimPrefix(a, "v"), ".")
	bParts := strings.Split(strings.TrimPrefix(b, "v"), ".")

	// Pad to equal length.
	for len(aParts) < len(bParts) {
		aParts = append(aParts, "0")
	}

	for len(bParts) < len(aParts) {
		bParts = append(bParts, "0")
	}

	for i := range aParts {
		ai := parseVersionPart(aParts[i])
		bi := parseVersionPart(bParts[i])

		if ai < bi {
			return -1
		}

		if ai > bi {
			return 1
		}
	}

	return 0
}

// parseVersionPart converts a version segment to an integer for comparison.
func parseVersionPart(s string) int {
	n := 0

	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}

	return n
}

// CheckUpdates compares installed plugins against the registry and returns available updates.
func CheckUpdates() ([]UpdateResult, error) {
	installed, err := LoadPlugins(PluginDir())
	if err != nil {
		return nil, fmt.Errorf("load installed plugins: %w", err)
	}

	if len(installed) == 0 {
		return nil, nil
	}

	idx, err := FetchRegistry()
	if err != nil {
		return nil, fmt.Errorf("fetch registry: %w", err)
	}

	regMap := make(map[string]RegistryEntry, len(idx.Plugins))
	for _, entry := range idx.Plugins {
		regMap[entry.Name] = entry
	}

	var results []UpdateResult

	for _, p := range installed {
		entry, ok := regMap[p.Name]
		if !ok {
			continue
		}

		r := UpdateResult{
			Name:         p.Name,
			InstalledVer: p.Version,
			AvailableVer: entry.Version,
		}

		if CompareVersions(p.Version, entry.Version) < 0 {
			r.Updated = false // not yet updated, just available
			results = append(results, r)
		}
	}

	return results, nil
}

// Update checks the registry for newer versions and installs them.
// If name is empty, updates all plugins. Returns results for each plugin processed.
func Update(name string) ([]UpdateResult, error) {
	installed, err := LoadPlugins(PluginDir())
	if err != nil {
		return nil, fmt.Errorf("load installed plugins: %w", err)
	}

	if len(installed) == 0 {
		return nil, nil
	}

	idx, err := FetchRegistry()
	if err != nil {
		return nil, fmt.Errorf("fetch registry: %w", err)
	}

	regMap := make(map[string]RegistryEntry, len(idx.Plugins))
	for _, entry := range idx.Plugins {
		regMap[entry.Name] = entry
	}

	var results []UpdateResult

	for _, p := range installed {
		if name != "" && p.Name != name {
			continue
		}

		entry, ok := regMap[p.Name]
		if !ok {
			if name != "" {
				results = append(results, UpdateResult{
					Name:         p.Name,
					InstalledVer: p.Version,
					Error:        fmt.Errorf("plugin %q not found in registry", p.Name),
				})
			}

			continue
		}

		r := UpdateResult{
			Name:         p.Name,
			InstalledVer: p.Version,
			AvailableVer: entry.Version,
		}

		if CompareVersions(p.Version, entry.Version) >= 0 {
			results = append(results, r)
			continue
		}

		// Newer version available — install it.
		_, installErr := Install(entry.Name)
		if installErr != nil {
			r.Error = installErr
		} else {
			r.Updated = true
		}

		results = append(results, r)
	}

	if name != "" && len(results) == 0 {
		return nil, fmt.Errorf("plugin %q is not installed", name)
	}

	return results, nil
}

// Remove deletes a plugin by name from the plugins directory.
func Remove(name string) error {
	return RemoveFromScope(name, ScopeUser, "")
}

// resolveSource converts a source string to a downloadable URL.
func resolveSource(source string) string {
	// Already a URL.
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return source
	}

	// GitHub path: github.com/user/repo/path/to/file.json
	if rest, ok := strings.CutPrefix(source, "github.com/"); ok {
		parts := strings.SplitN(rest, "/", 3)
		if len(parts) >= 3 {
			return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s", parts[0], parts[1], parts[2])
		}

		if len(parts) == 2 {
			// Assume it's a repo with a plugin.json at root
			return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/plugin.json", parts[0], parts[1])
		}
	}

	// Treat as registry name — look up from default registry.
	return RegistryBaseURL + "/plugins/" + source + ".json"
}

// httpGetFunc is the HTTP GET implementation, replaceable for testing.
var httpGetFunc = httpGetDefault

// httpGet performs an HTTP GET using the current httpGetFunc.
func httpGet(url string) ([]byte, error) {
	return httpGetFunc(url)
}

// httpGetDefault performs an HTTP GET with timeout.
func httpGetDefault(url string) ([]byte, error) {
	client := &http.Client{Timeout: installTimeout}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return body, nil
}
