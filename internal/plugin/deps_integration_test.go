package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegration_PluginDependencyResolution is an end-to-end integration test
// that exercises plugin dependency installation including file I/O, deep
// resolution, and validation against a mock registry.
func TestIntegration_PluginDependencyResolution(t *testing.T) {
	// 1. Set up a temp directory as the plugin scope.
	tmpDir := t.TempDir()

	pluginDir := filepath.Join(tmpDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}

	// 2. Create mock plugin JSON files with a dependency chain: A -> B -> C.
	pluginC := PluginDef{
		Name:    "c",
		Version: "1.0.0",
		Tools: []ToolDef{
			{Name: "ctx_c", Description: "plugin C tool", Command: "echo c"},
		},
	}

	pluginB := PluginDef{
		Name:    "b",
		Version: "1.2.0",
		Dependencies: []PluginDependency{
			{Name: "c"}, // no version constraint
		},
		Tools: []ToolDef{
			{Name: "ctx_b", Description: "plugin B tool", Command: "echo b"},
		},
	}

	pluginA := PluginDef{
		Name:    "a",
		Version: "2.0.0",
		Dependencies: []PluginDependency{
			{Name: "b", Version: ">=1.0.0"},
		},
		Tools: []ToolDef{
			{Name: "ctx_a", Description: "plugin A tool", Command: "echo a"},
		},
	}

	// Write each plugin to the temp plugin directory.
	for _, p := range []PluginDef{pluginA, pluginB, pluginC} {
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			t.Fatalf("marshal plugin %s: %v", p.Name, err)
		}

		path := filepath.Join(pluginDir, p.Name+".json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write plugin %s: %v", p.Name, err)
		}
	}

	// Verify the files can be loaded back via LoadPlugins.
	loaded, err := LoadPlugins(pluginDir)
	if err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}

	if len(loaded) != 3 {
		t.Fatalf("expected 3 loaded plugins, got %d", len(loaded))
	}

	// 3. Build the available map (simulates a registry with full plugin defs).
	available := map[string]PluginDef{
		"b": pluginB,
		"c": pluginC,
	}

	// 4. Call ResolveDependenciesDeep for plugin A with no installed plugins.
	order, err := ResolveDependenciesDeep(pluginA, available, map[string]PluginDef{})
	if err != nil {
		t.Fatalf("ResolveDependenciesDeep: %v", err)
	}

	// 5. Verify the order is [c, b] (dependencies first, then dependents).
	//    Plugin A itself is not included in the order — only its dependencies.
	if len(order) != 2 {
		t.Fatalf("expected 2 dependencies in order, got %v", order)
	}

	if order[0] != "c" || order[1] != "b" {
		t.Fatalf("expected install order [c, b], got %v", order)
	}

	// 6. Simulate all plugins installed — ValidateDependencies should pass.
	allInstalled := map[string]PluginDef{
		"a": pluginA,
		"b": pluginB,
		"c": pluginC,
	}

	unmet := pluginA.ValidateDependencies(allInstalled)
	if len(unmet) != 0 {
		t.Fatalf("expected no unmet deps when all installed, got %v", unmet)
	}

	unmet = pluginB.ValidateDependencies(allInstalled)
	if len(unmet) != 0 {
		t.Fatalf("expected no unmet deps for B when all installed, got %v", unmet)
	}

	// 7. Verify ValidateDependencies fails when B is missing.
	missingB := map[string]PluginDef{
		"a": pluginA,
		"c": pluginC,
	}

	unmet = pluginA.ValidateDependencies(missingB)
	if len(unmet) != 1 || unmet[0] != "b" {
		t.Fatalf("expected [b] unmet when B missing, got %v", unmet)
	}
}

// TestIntegration_PluginDependencyResolution_RegistryBased exercises the shallow
// ResolveDependencies path using a mock RegistryEntry slice, ensuring that the
// registry-based resolver also produces the correct install list.
func TestIntegration_PluginDependencyResolution_RegistryBased(t *testing.T) {
	pluginA := makePluginDef("a", "2.0.0", []PluginDependency{
		{Name: "b", Version: ">=1.0.0"},
	})

	registry := []RegistryEntry{
		{Name: "b", Version: "1.2.0", File: "plugins/b.json"},
		{Name: "c", Version: "1.0.0", File: "plugins/c.json"},
	}

	order, err := ResolveDependencies(pluginA, registry, map[string]PluginDef{})
	if err != nil {
		t.Fatalf("ResolveDependencies: %v", err)
	}

	if len(order) != 1 || order[0] != "b" {
		t.Fatalf("expected [b], got %v", order)
	}
}

// TestIntegration_PluginDependencyResolution_CircularDetection verifies that
// circular dependencies across three plugins are detected.
func TestIntegration_PluginDependencyResolution_CircularDetection(t *testing.T) {
	a := makePluginDef("a", "1.0.0", []PluginDependency{{Name: "b"}})
	b := makePluginDef("b", "1.0.0", []PluginDependency{{Name: "c"}})
	c := makePluginDef("c", "1.0.0", []PluginDependency{{Name: "a"}})

	available := map[string]PluginDef{"a": a, "b": b, "c": c}

	_, err := ResolveDependenciesDeep(a, available, map[string]PluginDef{})
	if err == nil {
		t.Fatal("expected circular dependency error")
	}

	if !strings.Contains(err.Error(), "circular dependency") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestIntegration_PluginDependencyResolution_PartiallyInstalled verifies that
// already-installed plugins are skipped during resolution.
func TestIntegration_PluginDependencyResolution_PartiallyInstalled(t *testing.T) {
	pluginC := makePluginDef("c", "1.0.0", nil)
	pluginB := makePluginDef("b", "1.2.0", []PluginDependency{
		{Name: "c"},
	})
	pluginA := makePluginDef("a", "2.0.0", []PluginDependency{
		{Name: "b", Version: ">=1.0.0"},
	})

	available := map[string]PluginDef{
		"b": pluginB,
		"c": pluginC,
	}

	// C is already installed.
	installed := map[string]PluginDef{
		"c": pluginC,
	}

	order, err := ResolveDependenciesDeep(pluginA, available, installed)
	if err != nil {
		t.Fatalf("ResolveDependenciesDeep: %v", err)
	}

	// Only B should be in the install order (C is already installed).
	if len(order) != 1 || order[0] != "b" {
		t.Fatalf("expected [b] (c already installed), got %v", order)
	}
}
