package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// pluginManifest represents a subset of installed_plugins.json entries.
type pluginManifest struct {
	Version string `json:"version"`
	Path    string `json:"path"`
}

// CheckVersionMismatch compares the running binary's version against the version
// recorded in the plugin manifest. Returns a warning message if they differ,
// or empty string if they match or the manifest is not found.
func CheckVersionMismatch(pluginDir string) string {
	manifestPath := filepath.Join(pluginDir, "installed_plugins.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "" // No manifest — nothing to check.
	}

	var manifests map[string]pluginManifest
	if err := json.Unmarshal(data, &manifests); err != nil {
		return ""
	}

	entry, ok := manifests["thimble"]
	if !ok {
		return ""
	}

	if entry.Version != "" && entry.Version != Version {
		return fmt.Sprintf("version mismatch: binary=%s, manifest=%s (path: %s)", Version, entry.Version, entry.Path)
	}

	return ""
}

// SelfHeal checks for version mismatches and optionally copies the binary
// to the expected path. It logs a warning but never fails the hook dispatch.
func SelfHeal(pluginDir string, logger *slog.Logger) {
	warning := CheckVersionMismatch(pluginDir)
	if warning == "" {
		return
	}

	logger.Warn("self-heal: " + warning)

	// Read the manifest to get the expected path.
	manifestPath := filepath.Join(pluginDir, "installed_plugins.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return
	}

	var manifests map[string]pluginManifest
	if err := json.Unmarshal(data, &manifests); err != nil {
		return
	}

	entry, ok := manifests["thimble"]
	if !ok || entry.Path == "" {
		return
	}

	// Try to copy the current binary to the expected path.
	exe, err := os.Executable()
	if err != nil {
		return
	}

	// Don't overwrite if same path.
	absExe, _ := filepath.Abs(exe)

	absTarget, _ := filepath.Abs(entry.Path)
	if absExe == absTarget {
		return
	}

	if err := copyFile(exe, entry.Path, 0o755); err != nil {
		logger.Warn("self-heal: cannot copy binary to expected path", "error", err, "path", entry.Path)
		return
	}

	logger.Info("self-heal: copied binary to expected path", "path", entry.Path)
}
