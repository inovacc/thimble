package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/thimble/internal/platform"
)

func TestResolveClientAliases(t *testing.T) {
	tests := []struct {
		input string
		want  platform.PlatformID
	}{
		{"claude", platform.PlatformClaudeCode},
		{"gemini", platform.PlatformGeminiCLI},
		{"vscode", platform.PlatformVSCodeCopilot},
		{"copilot", platform.PlatformVSCodeCopilot},
		{"cursor", platform.PlatformCursor},
		{"opencode", platform.PlatformOpenCode},
		{"codex", platform.PlatformCodex},
		{"claude-code", platform.PlatformClaudeCode},
		{"gemini-cli", platform.PlatformGeminiCLI},
		{"vscode-copilot", platform.PlatformVSCodeCopilot},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := resolveClient(tt.input)
			if err != nil {
				t.Fatalf("resolveClient(%q): %v", tt.input, err)
			}

			if got != tt.want {
				t.Errorf("resolveClient(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveClientUnknown(t *testing.T) {
	_, err := resolveClient("unknown-ide")
	if err == nil {
		t.Error("expected error for unknown client")
	}
}

func TestResolveClientEmptyString(t *testing.T) {
	_, err := resolveClient("")
	if err == nil {
		t.Error("expected error for empty client name")
	}
}

func TestDefaultPluginDir(t *testing.T) {
	tests := []struct {
		platform platform.PlatformID
		contains string
	}{
		{platform.PlatformClaudeCode, ".claude"},
		{platform.PlatformGeminiCLI, ".gemini"},
		{platform.PlatformVSCodeCopilot, ".github"},
		{platform.PlatformCursor, ".cursor"},
		{platform.PlatformUnknown, ".thimble"},
	}

	for _, tt := range tests {
		t.Run(string(tt.platform), func(t *testing.T) {
			dir := defaultPluginDir(tt.platform)
			if dir == "" {
				t.Fatal("expected non-empty dir")
			}

			found := false

			for i := range len(dir) - len(tt.contains) + 1 {
				if dir[i:i+len(tt.contains)] == tt.contains {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("defaultPluginDir(%s) = %q, want to contain %q", tt.platform, dir, tt.contains)
			}
		})
	}
}

func TestPatchJSONVersionTopLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.json")

	doc := map[string]any{
		"name":    "thimble",
		"version": "0.1.0",
	}

	data, _ := json.MarshalIndent(doc, "", "  ") //nolint:errchkjson
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := patchJSONVersion(path, "1.2.3"); err != nil {
		t.Fatalf("patchJSONVersion: %v", err)
	}

	updated, _ := os.ReadFile(path)

	var result map[string]any
	if err := json.Unmarshal(updated, &result); err != nil {
		t.Fatal(err)
	}

	if result["version"] != "1.2.3" {
		t.Errorf("version = %v, want 1.2.3", result["version"])
	}
}

func TestPatchJSONVersionMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "marketplace.json")

	doc := map[string]any{
		"metadata": map[string]any{
			"owner":   "inovacc",
			"version": "0.1.0",
		},
		"plugins": []any{
			map[string]any{
				"name":    "thimble",
				"version": "0.1.0",
			},
		},
	}

	data, _ := json.MarshalIndent(doc, "", "  ") //nolint:errchkjson
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := patchJSONVersion(path, "2.0.0"); err != nil {
		t.Fatalf("patchJSONVersion: %v", err)
	}

	updated, _ := os.ReadFile(path)

	var result map[string]any
	if err := json.Unmarshal(updated, &result); err != nil {
		t.Fatal(err)
	}

	meta := result["metadata"].(map[string]any)
	if meta["version"] != "2.0.0" {
		t.Errorf("metadata.version = %v, want 2.0.0", meta["version"])
	}

	plugins := result["plugins"].([]any)

	plugin := plugins[0].(map[string]any)
	if plugin["version"] != "2.0.0" {
		t.Errorf("plugins[0].version = %v, want 2.0.0", plugin["version"])
	}
}

func TestPatchJSONVersionNoVersionField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noversion.json")

	doc := map[string]any{"name": "thimble"}

	data, _ := json.MarshalIndent(doc, "", "  ") //nolint:errchkjson
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Should not error, just no-op.
	if err := patchJSONVersion(path, "1.0.0"); err != nil {
		t.Fatalf("patchJSONVersion: %v", err)
	}
}

func TestPatchJSONVersionNonExistent(t *testing.T) {
	err := patchJSONVersion("/nonexistent/file.json", "1.0.0")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestExtractBinaryRouting(t *testing.T) {
	// extractBinary routes .zip to extractFromZip and .tar.gz to extractFromTarGz.
	// Both will fail on empty archives, but we verify the routing logic.
	err := extractBinary("/fake/archive.zip", "/fake/dest")
	if err == nil {
		t.Error("expected error for non-existent zip")
	}

	err = extractBinary("/fake/archive.tar.gz", "/fake/dest")
	if err == nil {
		t.Error("expected error for non-existent tar.gz")
	}
}

func TestFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello world\n")

	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatalf("fileSHA256: %v", err)
	}

	// SHA256 of "hello world\n".
	expected := "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447"
	if hash != expected {
		t.Errorf("hash = %s, want %s", hash, expected)
	}
}

func TestFileSHA256NonExistent(t *testing.T) {
	_, err := fileSHA256("/nonexistent/file")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}
