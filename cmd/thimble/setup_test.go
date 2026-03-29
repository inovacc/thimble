package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/thimble/internal/platform"
)

func TestIsTestBinary(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/usr/local/bin/thimble", false},
		{`C:\Users\dev\go\bin\thimble.exe`, false},
		{"/tmp/go-build123/thimble.test", true},
		{`C:\Users\dev\AppData\Local\Temp\go-build123\thimble.test.exe`, true},
		{"/home/user/.cache/go-build/pkg.test", true},
		{"thimble", false},
		{"something.test", true},
		{"something.test.exe", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isTestBinary(tt.path)
			if got != tt.want {
				t.Errorf("isTestBinary(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestBuildHookConfigClaudeCode(t *testing.T) {
	config := buildHookConfig(platform.PlatformClaudeCode, "/usr/local/bin/thimble")

	hooks, ok := config["hooks"].(map[string]any)
	if !ok {
		t.Fatal("config should have hooks key")
	}

	expectedEvents := []string{"PreToolUse", "PostToolUse", "PreCompact", "SessionStart"}
	for _, event := range expectedEvents {
		if _, ok := hooks[event]; !ok {
			t.Errorf("missing hook event: %s", event)
		}
	}
}

func TestBuildHookConfigGeminiCLI(t *testing.T) {
	config := buildHookConfig(platform.PlatformGeminiCLI, "/usr/local/bin/thimble")

	hooks, ok := config["hooks"].(map[string]any)
	if !ok {
		t.Fatal("config should have hooks key")
	}

	// Gemini uses different event names.
	expectedEvents := []string{"BeforeTool", "AfterTool", "PreCompress", "SessionStart"}
	for _, event := range expectedEvents {
		if _, ok := hooks[event]; !ok {
			t.Errorf("missing hook event: %s", event)
		}
	}
}

func TestWriteHookSettingsMerge(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Write existing settings with a custom key.
	existing := map[string]any{
		"customKey": "preserved",
		"hooks": map[string]any{
			"ExistingHook": []any{"keep-this"},
		},
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		t.Fatalf("marshal existing settings: %v", err)
	}

	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatalf("write existing settings: %v", err)
	}

	// Write thimble hooks.
	config := buildHookConfig(platform.PlatformClaudeCode, "/usr/local/bin/thimble")
	if err := writeHookSettings(settingsPath, platform.PlatformClaudeCode, config); err != nil {
		t.Fatalf("writeHookSettings: %v", err)
	}

	// Read back and verify merge.
	merged, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read merged settings: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(merged, &result); err != nil {
		t.Fatalf("unmarshal merged: %v", err)
	}

	// Custom key should be preserved.
	if result["customKey"] != "preserved" {
		t.Error("customKey should be preserved after merge")
	}

	// Hooks should contain both existing and new.
	hooks, ok := result["hooks"].(map[string]any)
	if !ok {
		t.Fatal("merged result should have hooks")
	}

	if _, ok := hooks["ExistingHook"]; !ok {
		t.Error("ExistingHook should be preserved")
	}

	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("PreToolUse should be added")
	}

	if _, ok := hooks["SessionStart"]; !ok {
		t.Error("SessionStart should be added")
	}
}

func TestWriteHookSettingsCreatesFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "subdir", "settings.json")

	config := buildHookConfig(platform.PlatformClaudeCode, "/usr/local/bin/thimble")
	if err := writeHookSettings(settingsPath, platform.PlatformClaudeCode, config); err != nil {
		t.Fatalf("writeHookSettings: %v", err)
	}

	// File should exist.
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("settings file should exist: %v", err)
	}

	// Should be valid JSON.
	data, _ := os.ReadFile(settingsPath)

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("should be valid JSON: %v", err)
	}

	hooks, ok := result["hooks"].(map[string]any)
	if !ok {
		t.Fatal("should have hooks")
	}

	if len(hooks) != 5 {
		t.Errorf("expected 5 hook events, got %d", len(hooks))
	}
}

func TestBuildHookConfigCommandFormat(t *testing.T) {
	config := buildHookConfig(platform.PlatformClaudeCode, "/path/to/thimble")

	hooks := config["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)
	entry := preToolUse[0].(map[string]any)
	hookList := entry["hooks"].([]any)
	hook := hookList[0].(map[string]any)

	cmd := hook["command"].(string)

	expected := "/path/to/thimble hook claude-code pretooluse"
	if cmd != expected {
		t.Errorf("command = %q, want %q", cmd, expected)
	}
}
