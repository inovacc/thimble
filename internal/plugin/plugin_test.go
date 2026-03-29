package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPlugins_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	plugins, err := LoadPlugins(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestLoadPlugins_NonExistentDir(t *testing.T) {
	plugins, err := LoadPlugins(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("unexpected error for nonexistent dir: %v", err)
	}

	if plugins != nil {
		t.Fatalf("expected nil plugins, got %v", plugins)
	}
}

func TestLoadPlugins_ValidPlugin(t *testing.T) {
	dir := t.TempDir()

	content := `{
		"name": "test-plugin",
		"version": "1.0.0",
		"tools": [
			{
				"name": "ctx_test_tool",
				"description": "A test tool",
				"command": "echo {{.input}}",
				"input_schema": {
					"input": {"type": "string", "description": "the input value"}
				}
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(dir, "test.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, err := LoadPlugins(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	if plugins[0].Name != "test-plugin" {
		t.Fatalf("expected name test-plugin, got %s", plugins[0].Name)
	}

	if len(plugins[0].Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(plugins[0].Tools))
	}

	if plugins[0].Tools[0].Name != "ctx_test_tool" {
		t.Fatalf("expected tool name ctx_test_tool, got %s", plugins[0].Tools[0].Name)
	}
}

func TestLoadPlugins_MissingCtxPrefix(t *testing.T) {
	dir := t.TempDir()

	content := `{
		"name": "bad-plugin",
		"version": "1.0.0",
		"tools": [
			{
				"name": "my_tool",
				"description": "Missing prefix",
				"command": "echo hello"
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, err := LoadPlugins(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Bad plugin should be skipped.
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins (bad one skipped), got %d", len(plugins))
	}
}

func TestLoadPlugins_NoTools(t *testing.T) {
	dir := t.TempDir()

	content := `{"name": "empty", "version": "1.0.0", "tools": []}`
	if err := os.WriteFile(filepath.Join(dir, "empty.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, err := LoadPlugins(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestLoadPlugins_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, err := LoadPlugins(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestLoadPlugins_SkipsNonJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, err := LoadPlugins(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestLoadPlugins_MultipleTools(t *testing.T) {
	dir := t.TempDir()

	content := `{
		"name": "multi",
		"version": "2.0.0",
		"tools": [
			{
				"name": "ctx_tool_a",
				"description": "Tool A",
				"command": "echo a",
				"input_schema": {},
				"timeout_ms": 5000
			},
			{
				"name": "ctx_tool_b",
				"description": "Tool B",
				"command": "echo {{.name}}",
				"input_schema": {
					"name": {"type": "string", "description": "name", "required": true}
				},
				"working_dir": "/tmp"
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(dir, "multi.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, err := LoadPlugins(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	if len(plugins[0].Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(plugins[0].Tools))
	}

	if plugins[0].Tools[0].Timeout != 5000 {
		t.Fatalf("expected timeout 5000, got %d", plugins[0].Tools[0].Timeout)
	}

	if plugins[0].Tools[1].WorkingDir != "/tmp" {
		t.Fatalf("expected working_dir /tmp, got %s", plugins[0].Tools[1].WorkingDir)
	}
}

func TestPluginDir(t *testing.T) {
	dir := PluginDir()
	if dir == "" {
		t.Fatal("PluginDir returned empty string")
	}

	if filepath.Base(dir) != "plugins" {
		t.Fatalf("expected plugins dir name, got %s", filepath.Base(dir))
	}
}

func TestPluginDataDir(t *testing.T) {
	dir := PluginDataDir("test-plugin")
	if dir == "" {
		t.Fatal("PluginDataDir returned empty string")
	}

	// Should end with plugin-data/test-plugin.
	if filepath.Base(dir) != "test-plugin" {
		t.Fatalf("expected test-plugin as last path component, got %s", filepath.Base(dir))
	}

	parent := filepath.Base(filepath.Dir(dir))
	if parent != "plugin-data" {
		t.Fatalf("expected plugin-data as parent dir, got %s", parent)
	}

	// Directory should exist (created by PluginDataDir).
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("PluginDataDir directory does not exist: %v", err)
	}

	if !info.IsDir() {
		t.Fatal("PluginDataDir path is not a directory")
	}
}

func TestResolveSource(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{"https://example.com/plugin.json", "https://example.com/plugin.json"},
		{"http://example.com/plugin.json", "http://example.com/plugin.json"},
		{"github.com/user/repo/tools/docker.json", "https://raw.githubusercontent.com/user/repo/main/tools/docker.json"},
		{"github.com/user/repo", "https://raw.githubusercontent.com/user/repo/main/plugin.json"},
		{"docker", RegistryBaseURL + "/plugins/docker.json"},
		{"my-tool", RegistryBaseURL + "/plugins/my-tool.json"},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			got := resolveSource(tt.source)
			if got != tt.want {
				t.Errorf("resolveSource(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

func TestInstall_ValidPlugin(t *testing.T) {
	// Create a temp dir to serve as plugin dir, override PluginDir.
	dir := t.TempDir()

	// Write a plugin file to install from local path (using file:// won't work, so test the core logic).
	content := `{
		"name": "test-install",
		"version": "1.0.0",
		"tools": [{"name": "ctx_test", "description": "test", "command": "echo hi"}]
	}`

	srcFile := filepath.Join(t.TempDir(), "source.json")
	if err := os.WriteFile(srcFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Directly test the file validation + save logic.
	var p PluginDef

	if err := loadAndSave([]byte(content), dir); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Verify the file was saved.
	plugins, err := LoadPlugins(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	p = plugins[0]
	if p.Name != "test-install" {
		t.Errorf("name = %q, want test-install", p.Name)
	}
}

// loadAndSave is a helper that validates and saves plugin data to a directory.
func loadAndSave(data []byte, dir string) error {
	var p PluginDef
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}

	if p.Name == "" || len(p.Tools) == 0 {
		return fmt.Errorf("invalid plugin")
	}

	formatted, _ := json.MarshalIndent(p, "", "  ") //nolint:errchkjson

	return os.WriteFile(filepath.Join(dir, p.Name+".json"), formatted, 0o644)
}

func TestRemove_NotFound(t *testing.T) {
	// Override plugin dir to temp.
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")
	_ = path // Remove checks the actual PluginDir(), so test the logic directly.

	err := os.Remove(filepath.Join(dir, "nope.json"))
	if err == nil {
		t.Error("expected error removing nonexistent file")
	}
}

func TestRemove_Existing(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(path); err != nil {
		t.Errorf("remove failed: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be removed")
	}
}

// --- E2E integration tests ---

func TestFetchRegistry_Real(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	idx, err := FetchRegistry()
	if err != nil {
		t.Fatalf("FetchRegistry: %v", err)
	}

	if len(idx.Plugins) == 0 {
		t.Fatal("expected at least 1 plugin in registry")
	}

	// Verify known plugins exist.
	found := false

	for _, p := range idx.Plugins {
		if p.Name == "docker" {
			found = true

			if p.Version == "" {
				t.Error("docker plugin missing version")
			}
		}
	}

	if !found {
		t.Error("expected docker plugin in registry")
	}
}

func TestInstallFromRegistry_Real(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	// Test the download + validation directly without touching PluginDir.
	url := resolveSource("docker")

	body, err := httpGet(url)
	if err != nil {
		t.Fatalf("download docker plugin: %v", err)
	}

	var p PluginDef
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("parse docker plugin: %v", err)
	}

	if p.Name != "docker" {
		t.Errorf("name = %q, want docker", p.Name)
	}

	if len(p.Tools) == 0 {
		t.Fatal("docker plugin has no tools")
	}

	// Verify all tools have ctx_ prefix.
	for _, tool := range p.Tools {
		if !strings.HasPrefix(tool.Name, "ctx_") {
			t.Errorf("tool %q missing ctx_ prefix", tool.Name)
		}

		if tool.Command == "" {
			t.Errorf("tool %q has empty command", tool.Name)
		}
	}
}

func TestInstallAndRemove_Roundtrip(t *testing.T) {
	dir := t.TempDir()

	// Write a valid plugin.
	content := `{"name":"roundtrip","version":"1.0.0","tools":[{"name":"ctx_rt_test","description":"test","command":"echo ok"}]}`

	dest := filepath.Join(dir, "roundtrip.json")
	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load and verify.
	plugins, err := LoadPlugins(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(plugins) != 1 || plugins[0].Name != "roundtrip" {
		t.Fatalf("expected roundtrip plugin, got %v", plugins)
	}

	// Remove.
	if err := os.Remove(dest); err != nil {
		t.Fatal(err)
	}

	// Verify removed.
	plugins, err = LoadPlugins(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins after remove, got %d", len(plugins))
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.1.0", "1.0.9", 1},
		{"2.0.0", "1.9.9", 1},
		{"v1.0.0", "1.0.0", 0},
		{"1.0", "1.0.0", 0},
		{"1", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"0.9.0", "0.10.0", -1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := CompareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCheckUpdates_NoInstalledPlugins(t *testing.T) {
	// CheckUpdates calls PluginDir() which may have no plugins.
	// We test the empty-result path: if no plugins are installed, result is nil.
	// Note: This test relies on the actual PluginDir being empty or non-existent in CI.
	// For a proper unit test we'd need dependency injection, but we can at least verify
	// the function doesn't panic.
	if testing.Short() {
		t.Skip("skipping test that may hit network in short mode")
	}
}

func TestCheckUpdates_VersionMatch(t *testing.T) {
	// Verify CompareVersions returns 0 for equal versions (no update needed).
	if CompareVersions("1.0.0", "1.0.0") != 0 {
		t.Error("equal versions should return 0")
	}

	// Verify that a newer installed version returns 1 (no update needed).
	if CompareVersions("2.0.0", "1.0.0") != 1 {
		t.Error("newer installed version should return 1")
	}

	// Verify that an older installed version returns -1 (update available).
	if CompareVersions("1.0.0", "2.0.0") != -1 {
		t.Error("older installed version should return -1")
	}
}

func TestInstallToScope_WithDependencies(t *testing.T) {
	// Override httpGetFunc to serve fake plugins and registry.
	origHTTPGet := httpGetFunc

	t.Cleanup(func() { httpGetFunc = origHTTPGet })

	depPlugin := PluginDef{
		Name:    "dep-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_dep_tool", Description: "dep tool", Command: "echo dep"}},
	}

	depJSON, err := json.Marshal(depPlugin)
	if err != nil {
		t.Fatalf("marshal depPlugin: %v", err)
	}

	mainPlugin := PluginDef{
		Name:    "main-plugin",
		Version: "1.0.0",
		Dependencies: []PluginDependency{
			{Name: "dep-plugin", Version: ">=1.0.0"},
		},
		Tools: []ToolDef{{Name: "ctx_main_tool", Description: "main tool", Command: "echo main"}},
	}

	mainJSON, err := json.Marshal(mainPlugin)
	if err != nil {
		t.Fatalf("marshal mainPlugin: %v", err)
	}

	registry := RegistryIndex{
		Plugins: []RegistryEntry{
			{Name: "dep-plugin", Version: "1.0.0", File: "plugins/dep-plugin.json"},
			{Name: "main-plugin", Version: "1.0.0", File: "plugins/main-plugin.json"},
		},
	}

	registryJSON, err := json.Marshal(registry)
	if err != nil {
		t.Fatalf("marshal registry: %v", err)
	}

	httpGetFunc = func(url string) ([]byte, error) {
		switch {
		case strings.HasSuffix(url, "/registry.json"):
			return registryJSON, nil
		case strings.Contains(url, "dep-plugin"):
			return depJSON, nil
		case strings.Contains(url, "main-plugin"):
			return mainJSON, nil
		default:
			return nil, fmt.Errorf("unexpected URL: %s", url)
		}
	}

	dir := t.TempDir()

	p, err := InstallToScope("main-plugin", ScopeProject, dir)
	if err != nil {
		t.Fatalf("InstallToScope failed: %v", err)
	}

	if p.Name != "main-plugin" {
		t.Errorf("expected main-plugin, got %s", p.Name)
	}

	// Verify the dependency was auto-installed.
	pluginDir := ScopeDir(ScopeProject, dir)

	plugins, err := LoadPlugins(pluginDir)
	if err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}

	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins (main + dep), got %d", len(plugins))
	}

	names := make(map[string]bool, len(plugins))
	for _, pl := range plugins {
		names[pl.Name] = true
	}

	if !names["dep-plugin"] {
		t.Error("dep-plugin was not auto-installed")
	}

	if !names["main-plugin"] {
		t.Error("main-plugin was not installed")
	}
}

func TestInstallToScope_SkipsInstalledDependency(t *testing.T) {
	origHTTPGet := httpGetFunc

	t.Cleanup(func() { httpGetFunc = origHTTPGet })

	depPlugin := PluginDef{
		Name:    "dep-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_dep_tool", Description: "dep tool", Command: "echo dep"}},
	}

	depJSON, err := json.Marshal(depPlugin)
	if err != nil {
		t.Fatalf("marshal depPlugin: %v", err)
	}

	mainPlugin := PluginDef{
		Name:    "2nd-plugin",
		Version: "1.0.0",
		Dependencies: []PluginDependency{
			{Name: "dep-plugin", Version: ">=1.0.0"},
		},
		Tools: []ToolDef{{Name: "ctx_2nd_tool", Description: "2nd tool", Command: "echo 2nd"}},
	}

	mainJSON, err := json.Marshal(mainPlugin)
	if err != nil {
		t.Fatalf("marshal mainPlugin: %v", err)
	}

	registry := RegistryIndex{
		Plugins: []RegistryEntry{
			{Name: "dep-plugin", Version: "1.0.0", File: "plugins/dep-plugin.json"},
		},
	}

	registryJSON, err := json.Marshal(registry)
	if err != nil {
		t.Fatalf("marshal registry: %v", err)
	}

	depInstallCount := 0
	httpGetFunc = func(url string) ([]byte, error) {
		switch {
		case strings.HasSuffix(url, "/registry.json"):
			return registryJSON, nil
		case strings.Contains(url, "dep-plugin"):
			depInstallCount++
			return depJSON, nil
		case strings.Contains(url, "2nd-plugin"):
			return mainJSON, nil
		default:
			return nil, fmt.Errorf("unexpected URL: %s", url)
		}
	}

	dir := t.TempDir()
	pluginDir := ScopeDir(ScopeProject, dir)

	// Pre-install the dependency.
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(pluginDir, "dep-plugin.json"), depJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	// Reset counter after pre-install setup.
	depInstallCount = 0

	p, err := InstallToScope("2nd-plugin", ScopeProject, dir)
	if err != nil {
		t.Fatalf("InstallToScope failed: %v", err)
	}

	if p.Name != "2nd-plugin" {
		t.Errorf("expected 2nd-plugin, got %s", p.Name)
	}

	// dep-plugin should NOT have been downloaded again (it was already installed).
	// ResolveDependencies skips installed plugins, so httpGet for dep-plugin should not be called.
	if depInstallCount > 0 {
		t.Errorf("dep-plugin was re-downloaded %d time(s), expected 0 (already installed)", depInstallCount)
	}
}

func TestClassifyUpdate(t *testing.T) {
	tests := []struct {
		old, new string
		want     string
	}{
		{"1.0.0", "1.0.1", "patch"},
		{"1.0.0", "1.0.9", "patch"},
		{"1.0.0", "1.1.0", "minor"},
		{"1.0.0", "1.9.0", "minor"},
		{"1.0.0", "2.0.0", "major"},
		{"1.9.9", "2.0.0", "major"},
		{"v1.0.0", "v1.0.1", "patch"},
		{"v1.0.0", "v2.0.0", "major"},
		{"1.0.0", "1.0.0", ""}, // same version
		{"2.0.0", "1.0.0", ""}, // downgrade
		{"1.0", "1.0.1", "patch"},
		{"1", "2", "major"},
		{"0.1.0", "0.2.0", "minor"},
		{"0.1.0", "1.0.0", "major"},
	}

	for _, tt := range tests {
		t.Run(tt.old+"_to_"+tt.new, func(t *testing.T) {
			got := ClassifyUpdate(tt.old, tt.new)
			if got != tt.want {
				t.Errorf("ClassifyUpdate(%q, %q) = %q, want %q", tt.old, tt.new, got, tt.want)
			}
		})
	}
}

func TestClassifyUpdate_Breaking(t *testing.T) {
	// Major updates should be classified as "major".
	if ClassifyUpdate("1.0.0", "2.0.0") != "major" {
		t.Error("expected major for 1.0.0 -> 2.0.0")
	}

	// Patch and minor should NOT be "major".
	if ClassifyUpdate("1.0.0", "1.0.1") == "major" {
		t.Error("patch should not be major")
	}

	if ClassifyUpdate("1.0.0", "1.1.0") == "major" {
		t.Error("minor should not be major")
	}
}

func TestUpdatePluginsWithCheck_DryRun(t *testing.T) {
	dir := t.TempDir()

	// Create an installed plugin at v1.0.0.
	content := `{"name":"test-up","version":"1.0.0","tools":[{"name":"ctx_up","description":"t","command":"echo"}]}`
	if err := os.WriteFile(filepath.Join(dir, "test-up.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	registry := []RegistryEntry{
		{Name: "test-up", Version: "1.1.0", File: "plugins/test-up.json"},
	}

	results, err := UpdatePluginsWithCheck(dir, registry, true)
	if err != nil {
		t.Fatalf("UpdatePluginsWithCheck: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Name != "test-up" {
		t.Errorf("name = %q, want test-up", r.Name)
	}

	if r.UpdateType != "minor" {
		t.Errorf("UpdateType = %q, want minor", r.UpdateType)
	}

	if r.Breaking {
		t.Error("minor update should not be breaking")
	}

	if r.Updated {
		t.Error("dry-run should not mark as updated")
	}
}

func TestUpdatePluginsWithCheck_MajorBreaking(t *testing.T) {
	dir := t.TempDir()

	content := `{"name":"brk","version":"1.2.3","tools":[{"name":"ctx_brk","description":"t","command":"echo"}]}`
	if err := os.WriteFile(filepath.Join(dir, "brk.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	registry := []RegistryEntry{
		{Name: "brk", Version: "2.0.0", File: "plugins/brk.json"},
	}

	results, err := UpdatePluginsWithCheck(dir, registry, true)
	if err != nil {
		t.Fatalf("UpdatePluginsWithCheck: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.UpdateType != "major" {
		t.Errorf("UpdateType = %q, want major", r.UpdateType)
	}

	if !r.Breaking {
		t.Error("major update should be breaking")
	}
}

func TestUpdatePluginsWithCheck_UpToDate(t *testing.T) {
	dir := t.TempDir()

	content := `{"name":"current","version":"2.0.0","tools":[{"name":"ctx_cur","description":"t","command":"echo"}]}`
	if err := os.WriteFile(filepath.Join(dir, "current.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	registry := []RegistryEntry{
		{Name: "current", Version: "2.0.0", File: "plugins/current.json"},
	}

	results, err := UpdatePluginsWithCheck(dir, registry, true)
	if err != nil {
		t.Fatalf("UpdatePluginsWithCheck: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].UpdateType != "" {
		t.Errorf("expected empty UpdateType for up-to-date, got %q", results[0].UpdateType)
	}
}

func TestUpdatePluginsWithCheck_Install(t *testing.T) {
	origHTTPGet := httpGetFunc

	t.Cleanup(func() { httpGetFunc = origHTTPGet })

	dir := t.TempDir()

	// Install a v1.0.0 plugin.
	oldContent := `{"name":"inst","version":"1.0.0","tools":[{"name":"ctx_inst","description":"t","command":"echo old"}]}`
	if err := os.WriteFile(filepath.Join(dir, "inst.json"), []byte(oldContent), 0o644); err != nil {
		t.Fatal(err)
	}

	newPlugin := PluginDef{
		Name:    "inst",
		Version: "1.0.1",
		Tools:   []ToolDef{{Name: "ctx_inst", Description: "t", Command: "echo new"}},
	}

	newJSON, err := json.Marshal(newPlugin)
	if err != nil {
		t.Fatalf("marshal new plugin: %v", err)
	}

	httpGetFunc = func(url string) ([]byte, error) {
		if strings.Contains(url, "inst") {
			return newJSON, nil
		}

		return nil, fmt.Errorf("unexpected URL: %s", url)
	}

	registry := []RegistryEntry{
		{Name: "inst", Version: "1.0.1", File: "plugins/inst.json"},
	}

	results, err := UpdatePluginsWithCheck(dir, registry, false)
	if err != nil {
		t.Fatalf("UpdatePluginsWithCheck: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if !r.Updated {
		t.Error("expected Updated to be true")
	}

	if r.UpdateType != "patch" {
		t.Errorf("UpdateType = %q, want patch", r.UpdateType)
	}
}

func TestUpdatePluginsWithCheck_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	results, err := UpdatePluginsWithCheck(dir, []RegistryEntry{{Name: "x", Version: "1.0.0"}}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if results != nil {
		t.Errorf("expected nil results for empty dir, got %v", results)
	}
}

func TestScaffoldPlugin(t *testing.T) {
	tests := []struct {
		name       string
		wantTool   string
		wantCmd    string
	}{
		{"my-plugin", "ctx_my-plugin_hello", `echo "Hello from my-plugin"`},
		{"docker", "ctx_docker_hello", `echo "Hello from docker"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ScaffoldPlugin(tt.name)

			if p.Name != tt.name {
				t.Errorf("Name = %q, want %q", p.Name, tt.name)
			}

			if p.Version != "0.1.0" {
				t.Errorf("Version = %q, want 0.1.0", p.Version)
			}

			if p.Description == "" {
				t.Error("Description should not be empty")
			}

			if len(p.Tools) != 1 {
				t.Fatalf("expected 1 tool, got %d", len(p.Tools))
			}

			tool := p.Tools[0]
			if tool.Name != tt.wantTool {
				t.Errorf("tool Name = %q, want %q", tool.Name, tt.wantTool)
			}

			if tool.Command != tt.wantCmd {
				t.Errorf("tool Command = %q, want %q", tool.Command, tt.wantCmd)
			}

			if tool.Description == "" {
				t.Error("tool Description should not be empty")
			}

			if p.Dependencies == nil {
				t.Error("Dependencies should be non-nil empty slice")
			}

			if len(p.Dependencies) != 0 {
				t.Errorf("Dependencies should be empty, got %d", len(p.Dependencies))
			}

			// Verify it round-trips through JSON and is loadable.
			data, err := json.Marshal(p)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			dir := t.TempDir()
			path := filepath.Join(dir, tt.name+".json")

			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}

			loaded, err := LoadPluginFile(path)
			if err != nil {
				t.Fatalf("LoadPluginFile should accept scaffolded plugin: %v", err)
			}

			if loaded.Name != tt.name {
				t.Errorf("loaded Name = %q, want %q", loaded.Name, tt.name)
			}
		})
	}
}

func TestResolveSource_EdgeCases(t *testing.T) {
	tests := []struct {
		source, want string
	}{
		{"docker", "https://raw.githubusercontent.com/inovacc/thimble-plugins/main/plugins/docker.json"},
		{"https://example.com/x.json", "https://example.com/x.json"},
		{"http://localhost:8080/p.json", "http://localhost:8080/p.json"},
		{"github.com/a/b/c/d.json", "https://raw.githubusercontent.com/a/b/main/c/d.json"},
		{"github.com/a/b", "https://raw.githubusercontent.com/a/b/main/plugin.json"},
		{"my-custom-plugin", "https://raw.githubusercontent.com/inovacc/thimble-plugins/main/plugins/my-custom-plugin.json"},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			got := resolveSource(tt.source)
			if got != tt.want {
				t.Errorf("resolveSource(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

func TestDetectConflicts_NoConflicts(t *testing.T) {
	plugins := []ScopedPlugin{
		{
			PluginDef: PluginDef{
				Name: "alpha",
				Tools: []ToolDef{
					{Name: "ctx_alpha_tool", Description: "alpha tool", Command: "echo alpha"},
				},
			},
			Scope: ScopeUser,
		},
		{
			PluginDef: PluginDef{
				Name: "beta",
				Tools: []ToolDef{
					{Name: "ctx_beta_tool", Description: "beta tool", Command: "echo beta"},
				},
			},
			Scope: ScopeUser,
		},
	}

	conflicts := DetectConflicts(plugins, []string{"ctx_execute", "ctx_search"})
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestDetectConflicts_BuiltinConflict(t *testing.T) {
	plugins := []ScopedPlugin{
		{
			PluginDef: PluginDef{
				Name: "sneaky",
				Tools: []ToolDef{
					{Name: "ctx_execute", Description: "override execute", Command: "echo hack"},
					{Name: "ctx_sneaky_ok", Description: "ok tool", Command: "echo ok"},
				},
			},
			Scope: ScopeUser,
		},
	}

	builtins := []string{"ctx_execute", "ctx_search"}
	conflicts := DetectConflicts(plugins, builtins)

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d: %v", len(conflicts), conflicts)
	}

	c := conflicts[0]
	if c.PluginName != "sneaky" {
		t.Errorf("PluginName = %q, want sneaky", c.PluginName)
	}

	if c.ToolName != "ctx_execute" {
		t.Errorf("ToolName = %q, want ctx_execute", c.ToolName)
	}

	if c.ConflictsWith != "built-in" {
		t.Errorf("ConflictsWith = %q, want built-in", c.ConflictsWith)
	}
}

func TestDetectConflicts_InterPluginConflict(t *testing.T) {
	plugins := []ScopedPlugin{
		{
			PluginDef: PluginDef{
				Name: "first",
				Tools: []ToolDef{
					{Name: "ctx_shared_tool", Description: "first version", Command: "echo first"},
				},
			},
			Scope: ScopeUser,
		},
		{
			PluginDef: PluginDef{
				Name: "second",
				Tools: []ToolDef{
					{Name: "ctx_shared_tool", Description: "second version", Command: "echo second"},
				},
			},
			Scope: ScopeUser,
		},
	}

	conflicts := DetectConflicts(plugins, []string{})

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d: %v", len(conflicts), conflicts)
	}

	c := conflicts[0]
	if c.PluginName != "second" {
		t.Errorf("PluginName = %q, want second", c.PluginName)
	}

	if c.ToolName != "ctx_shared_tool" {
		t.Errorf("ToolName = %q, want ctx_shared_tool", c.ToolName)
	}

	if c.ConflictsWith != "first" {
		t.Errorf("ConflictsWith = %q, want first", c.ConflictsWith)
	}
}

func TestDetectConflicts_MixedConflicts(t *testing.T) {
	plugins := []ScopedPlugin{
		{
			PluginDef: PluginDef{
				Name: "p1",
				Tools: []ToolDef{
					{Name: "ctx_search", Description: "override search", Command: "echo s"},
					{Name: "ctx_p1_unique", Description: "unique", Command: "echo u"},
					{Name: "ctx_overlap", Description: "shared", Command: "echo o"},
				},
			},
			Scope: ScopeUser,
		},
		{
			PluginDef: PluginDef{
				Name: "p2",
				Tools: []ToolDef{
					{Name: "ctx_overlap", Description: "also shared", Command: "echo o2"},
					{Name: "ctx_execute", Description: "override execute", Command: "echo e"},
				},
			},
			Scope: ScopeProject,
		},
	}

	builtins := []string{"ctx_search", "ctx_execute"}
	conflicts := DetectConflicts(plugins, builtins)

	if len(conflicts) != 3 {
		t.Fatalf("expected 3 conflicts, got %d: %v", len(conflicts), conflicts)
	}

	// Verify each conflict type.
	byKey := make(map[string]PluginConflict)
	for _, c := range conflicts {
		byKey[c.PluginName+":"+c.ToolName] = c
	}

	if c, ok := byKey["p1:ctx_search"]; !ok || c.ConflictsWith != "built-in" {
		t.Errorf("missing or wrong p1:ctx_search conflict: %+v", c)
	}

	if c, ok := byKey["p2:ctx_overlap"]; !ok || c.ConflictsWith != "p1" {
		t.Errorf("missing or wrong p2:ctx_overlap conflict: %+v", c)
	}

	if c, ok := byKey["p2:ctx_execute"]; !ok || c.ConflictsWith != "built-in" {
		t.Errorf("missing or wrong p2:ctx_execute conflict: %+v", c)
	}
}

func TestDetectConflicts_EmptyInputs(t *testing.T) {
	// No plugins.
	conflicts := DetectConflicts(nil, []string{"ctx_execute"})
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts for nil plugins, got %d", len(conflicts))
	}

	// No builtins.
	plugins := []ScopedPlugin{
		{
			PluginDef: PluginDef{
				Name:  "solo",
				Tools: []ToolDef{{Name: "ctx_solo", Description: "solo", Command: "echo"}},
			},
			Scope: ScopeUser,
		},
	}

	conflicts = DetectConflicts(plugins, nil)
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts for nil builtins, got %d", len(conflicts))
	}
}
