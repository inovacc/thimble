package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/thimble/internal/platform"
)

func TestDeployPluginDryRun(t *testing.T) {
	// Save and restore global flags.
	origDryRun := setupDryRun
	origPluginDir := setupPluginDir

	defer func() {
		setupDryRun = origDryRun
		setupPluginDir = origPluginDir
	}()

	setupDryRun = true
	setupPluginDir = t.TempDir()

	err := deployPlugin(platform.PlatformClaudeCode)
	if err != nil {
		t.Fatalf("deployPlugin dry-run: %v", err)
	}

	// Dry-run should NOT create files.
	entries, _ := os.ReadDir(setupPluginDir)
	if len(entries) > 0 {
		t.Errorf("dry-run should not create files, got %d entries", len(entries))
	}
}

func TestDeployPluginActual(t *testing.T) {
	origDryRun := setupDryRun
	origPluginDir := setupPluginDir

	defer func() {
		setupDryRun = origDryRun
		setupPluginDir = origPluginDir
	}()

	setupDryRun = false
	setupPluginDir = t.TempDir()

	err := deployPlugin(platform.PlatformClaudeCode)
	if err != nil {
		t.Fatalf("deployPlugin: %v", err)
	}

	// Should have created files.
	mcpPath := filepath.Join(setupPluginDir, ".mcp.json")
	if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
		t.Error("expected .mcp.json to be deployed")
	}

	pluginPath := filepath.Join(setupPluginDir, ".claude-plugin", "plugin.json")
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		t.Error("expected .claude-plugin/plugin.json to be deployed")
	}
}

func TestDeployPluginCustomDir(t *testing.T) {
	origDryRun := setupDryRun
	origPluginDir := setupPluginDir

	defer func() {
		setupDryRun = origDryRun
		setupPluginDir = origPluginDir
	}()

	customDir := filepath.Join(t.TempDir(), "custom-plugin-dir")
	setupDryRun = false
	setupPluginDir = customDir

	err := deployPlugin(platform.PlatformGeminiCLI)
	if err != nil {
		t.Fatalf("deployPlugin custom dir: %v", err)
	}

	if _, err := os.Stat(customDir); os.IsNotExist(err) {
		t.Error("expected custom dir to be created")
	}
}

func TestPatchRegistryVersionCreatesEntry(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "installed_plugins.json")

	// Create an empty registry.
	registry := map[string]any{
		"plugins": map[string]any{},
	}

	data, _ := json.MarshalIndent(registry, "", "  ") //nolint:errchkjson
	if err := os.WriteFile(registryPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	installPath := filepath.Join(dir, "thimble")

	if err := patchRegistryVersion(registryPath, installPath, "1.0.0"); err != nil {
		t.Fatalf("patchRegistryVersion: %v", err)
	}

	// Read back and verify.
	updated, _ := os.ReadFile(registryPath)

	var result map[string]any
	if err := json.Unmarshal(updated, &result); err != nil {
		t.Fatal(err)
	}

	plugins := result["plugins"].(map[string]any)

	entries := plugins["thimble@thimble"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0].(map[string]any)
	if entry["version"] != "1.0.0" {
		t.Errorf("version = %v, want 1.0.0", entry["version"])
	}

	if entry["installPath"] != installPath {
		t.Errorf("installPath = %v, want %s", entry["installPath"], installPath)
	}
}

func TestPatchRegistryVersionUpdatesExisting(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "installed_plugins.json")

	registry := map[string]any{
		"plugins": map[string]any{
			"thimble@thimble": []any{
				map[string]any{
					"scope":       "user",
					"installPath": "/old/path",
					"version":     "0.1.0",
				},
			},
		},
	}

	data, _ := json.MarshalIndent(registry, "", "  ") //nolint:errchkjson
	if err := os.WriteFile(registryPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := patchRegistryVersion(registryPath, "/new/path", "2.0.0"); err != nil {
		t.Fatalf("patchRegistryVersion: %v", err)
	}

	updated, _ := os.ReadFile(registryPath)

	var result map[string]any
	if err := json.Unmarshal(updated, &result); err != nil {
		t.Fatal(err)
	}

	plugins := result["plugins"].(map[string]any)
	entries := plugins["thimble@thimble"].([]any)
	entry := entries[0].(map[string]any)

	if entry["version"] != "2.0.0" {
		t.Errorf("version = %v, want 2.0.0", entry["version"])
	}

	if entry["installPath"] != "/new/path" {
		t.Errorf("installPath = %v, want /new/path", entry["installPath"])
	}
}

func TestPatchRegistryVersionNoPluginsKey(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "registry.json")

	// Registry with plugins key but no thimble entry — and plugins is not a map.
	data := []byte(`{"plugins": {}}`)
	if err := os.WriteFile(registryPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Should create the entry.
	err := patchRegistryVersion(registryPath, "/path", "1.0.0")
	if err != nil {
		t.Fatalf("patchRegistryVersion: %v", err)
	}
}

func TestPatchRegistryVersionNonExistent(t *testing.T) {
	err := patchRegistryVersion("/nonexistent/registry.json", "/path", "1.0.0")
	if err == nil {
		t.Error("expected error for non-existent registry")
	}
}
