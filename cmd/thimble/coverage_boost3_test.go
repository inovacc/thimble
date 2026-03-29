package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/thimble/internal/hooks"
	"github.com/inovacc/thimble/internal/platform"
)

// --- resolveEvent ---

func TestResolveEvent_AllMappings(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		// Lowercase (Claude Code)
		{"pretooluse", "PreToolUse", true},
		{"posttooluse", "PostToolUse", true},
		{"precompact", "PreCompact", true},
		{"sessionstart", "SessionStart", true},
		{"userpromptsubmit", "UserPromptSubmit", true},
		// Gemini CLI
		{"beforetool", "PreToolUse", true},
		{"aftertool", "PostToolUse", true},
		{"precompress", "PreCompact", true},
		// Already canonical
		{"PreToolUse", "PreToolUse", true},
		{"PostToolUse", "PostToolUse", true},
		{"PreCompact", "PreCompact", true},
		{"SessionStart", "SessionStart", true},
		// Unknown
		{"unknown_event", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := resolveEvent(tt.input)
			if ok != tt.ok {
				t.Errorf("resolveEvent(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}

			if got != tt.want {
				t.Errorf("resolveEvent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- buildHookPayload ---

func TestBuildHookPayload_WithToolCallExtended(t *testing.T) {
	normalized := platform.NormalizedEvent{
		SessionID:  "sess-123",
		ProjectDir: "/project",
		ToolName:   "Bash",
		ToolInput:  map[string]any{"command": "ls"},
	}

	rawPayload := []byte(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`)

	result := buildHookPayload(normalized, rawPayload)
	if result == nil {
		t.Fatal("buildHookPayload returned nil")
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["sessionId"] != "sess-123" {
		t.Errorf("sessionId = %v, want sess-123", parsed["sessionId"])
	}

	if parsed["projectDir"] != "/project" {
		t.Errorf("projectDir = %v, want /project", parsed["projectDir"])
	}

	toolCall, ok := parsed["toolCall"].(map[string]any)
	if !ok {
		t.Fatal("expected toolCall object")
	}

	if toolCall["toolName"] != "Bash" {
		t.Errorf("toolName = %v, want Bash", toolCall["toolName"])
	}

	if _, ok := parsed["extra"]; !ok {
		t.Error("expected extra field from raw payload")
	}
}

func TestBuildHookPayload_WithToolOutputExtended(t *testing.T) {
	normalized := platform.NormalizedEvent{
		SessionID:  "sess-456",
		ProjectDir: "/project",
		ToolName:   "Read",
		ToolInput:  map[string]any{"path": "file.go"},
		ToolOutput: "file contents here",
		IsError:    true,
	}

	result := buildHookPayload(normalized, []byte(`{}`))
	if result == nil {
		t.Fatal("buildHookPayload returned nil")
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	toolCall := parsed["toolCall"].(map[string]any)
	if toolCall["toolResponse"] != "file contents here" {
		t.Errorf("toolResponse = %v, want 'file contents here'", toolCall["toolResponse"])
	}

	if toolCall["isError"] != true {
		t.Errorf("isError = %v, want true", toolCall["isError"])
	}
}

func TestBuildHookPayload_NoTool(t *testing.T) {
	normalized := platform.NormalizedEvent{
		SessionID:  "sess-789",
		ProjectDir: "/project",
	}

	result := buildHookPayload(normalized, []byte(`{"key":"val"}`))
	if result == nil {
		t.Fatal("buildHookPayload returned nil")
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := parsed["toolCall"]; ok {
		t.Error("should not have toolCall when ToolName is empty")
	}
}

func TestBuildHookPayload_WithSourceExtended(t *testing.T) {
	normalized := platform.NormalizedEvent{
		SessionID: "sess-src",
		Source:    "user_prompt",
	}

	result := buildHookPayload(normalized, []byte(`{}`))

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["source"] != "user_prompt" {
		t.Errorf("source = %v, want user_prompt", parsed["source"])
	}
}

func TestBuildHookPayload_InvalidRawPayloadExtended(t *testing.T) {
	normalized := platform.NormalizedEvent{SessionID: "s1"}

	result := buildHookPayload(normalized, []byte("not json"))
	if result == nil {
		t.Fatal("buildHookPayload returned nil for invalid raw payload")
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// "extra" should be absent when raw payload is invalid JSON.
	if _, ok := parsed["extra"]; ok {
		t.Error("should not have extra for invalid JSON payload")
	}
}

// --- formatViaAdapter ---

func TestFormatViaAdapter_NilResponse(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)

	output := formatViaAdapter(adapter, "PreToolUse", nil)
	if output != "{}" {
		t.Errorf("output = %q, want %q", output, "{}")
	}
}

func TestFormatViaAdapter_AllowResponse(t *testing.T) {
	t.Helper()

	adapter, _ := platform.Get(platform.PlatformClaudeCode)
	resp := &hooks.Response{Blocked: false}
	// Allow with no result/context may return empty — that's valid behavior.
	_ = formatViaAdapter(adapter, "PreToolUse", resp)
}

func TestFormatViaAdapter_DenyResponse(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)
	resp := &hooks.Response{Blocked: true, Reason: "security violation"}

	output := formatViaAdapter(adapter, "PreToolUse", resp)
	if output == "" {
		t.Error("expected non-empty output for deny response")
	}

	if !strings.Contains(output, "security violation") {
		t.Errorf("output should contain reason, got %q", output)
	}
}

func TestFormatViaAdapter_ContextResponse(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)

	resultJSON, err := json.Marshal(map[string]any{
		"additionalContext": "some context info",
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := &hooks.Response{Blocked: false, Result: resultJSON}

	output := formatViaAdapter(adapter, "PreToolUse", resp)
	if output == "" {
		t.Error("expected non-empty output for context response")
	}
}

func TestFormatViaAdapter_ModifyResponse(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)

	resultJSON, err := json.Marshal(map[string]any{
		"result": map[string]any{"command": "modified_cmd"},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := &hooks.Response{Blocked: false, Result: resultJSON}

	output := formatViaAdapter(adapter, "PreToolUse", resp)
	if output == "" {
		t.Error("expected non-empty output for modify response")
	}
}

func TestFormatViaAdapter_StringResult(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)

	resultJSON, err := json.Marshal(map[string]any{
		"result": "some string context",
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := &hooks.Response{Blocked: false, Result: resultJSON}

	output := formatViaAdapter(adapter, "PostToolUse", resp)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatViaAdapter_AllEvents(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)
	resp := &hooks.Response{Blocked: false}

	// Exercise all event paths — some may return empty for allow/no-context.
	events := []string{"PreToolUse", "PostToolUse", "PreCompact", "SessionStart"}
	for _, event := range events {
		t.Run(event, func(t *testing.T) {
			_ = formatViaAdapter(adapter, event, resp)
		})
	}
}

func TestFormatViaAdapter_UnknownEvent(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)
	resp := &hooks.Response{Blocked: false}

	output := formatViaAdapter(adapter, "UnknownEvent", resp)
	if output != "" {
		t.Errorf("expected empty output for unknown event, got %q", output)
	}
}

func TestFormatViaAdapter_GeminiPlatform(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformGeminiCLI)
	resp := &hooks.Response{Blocked: false}

	output := formatViaAdapter(adapter, "PreToolUse", resp)
	if output == "" {
		t.Error("expected non-empty output for Gemini PreToolUse")
	}
}

// --- parseNormalizedEvent ---

func TestParseNormalizedEvent_AllEventTypes(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)
	payload := json.RawMessage(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`)

	events := []string{"PreToolUse", "PostToolUse", "UserPromptSubmit", "PreCompact", "SessionStart"}
	for _, event := range events {
		t.Run(event, func(t *testing.T) {
			// Should not panic.
			_ = parseNormalizedEvent(adapter, event, payload)
		})
	}
}

// --- prettyJSON ---

func TestPrettyJSON_ValidJSONExtended(t *testing.T) {
	input := `{"key":"value","num":42}`

	result := prettyJSON(input)
	if !strings.Contains(result, "key") {
		t.Errorf("prettyJSON should contain key, got %q", result)
	}
	// Should be indented.
	if !strings.Contains(result, "\n") {
		t.Error("prettyJSON should produce indented output")
	}
}

func TestPrettyJSON_InvalidJSONExtended(t *testing.T) {
	input := "not json at all"

	result := prettyJSON(input)
	if result != input {
		t.Errorf("prettyJSON(%q) = %q, want original string", input, result)
	}
}

func TestPrettyJSON_EmptyStringExtended(t *testing.T) {
	result := prettyJSON("")
	if result != "" {
		t.Errorf("prettyJSON('') = %q, want empty", result)
	}
}

// --- copyFile ---

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "dest.txt")

	content := []byte("hello world test content")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst, 0o755); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}

	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestCopyFile_NonExistentSrc(t *testing.T) {
	dir := t.TempDir()

	err := copyFile(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "dst"), 0o755)
	if err == nil {
		t.Error("expected error for non-existent source")
	}
}

func TestCopyFile_InvalidDst(t *testing.T) {
	dir := t.TempDir()

	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := copyFile(src, filepath.Join(dir, "no", "such", "dir", "dst.txt"), 0o755)
	if err == nil {
		t.Error("expected error for invalid destination path")
	}
}

func TestCopyFile_LargeFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "large.bin")
	dst := filepath.Join(dir, "large_copy.bin")

	// 1MB file.
	data := make([]byte, 1<<20)
	for i := range data {
		data[i] = byte(i % 256)
	}

	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst, 0o755); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if len(got) != len(data) {
		t.Errorf("size mismatch: got %d, want %d", len(got), len(data))
	}
}

// --- isPluginActive ---

func TestIsPluginActive_NoSettingsFile(t *testing.T) {
	if isPluginActive("/nonexistent/settings.json", platform.PlatformClaudeCode) {
		t.Error("should return false for non-existent settings")
	}
}

func TestIsPluginActive_InvalidJSON(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if isPluginActive(path, platform.PlatformClaudeCode) {
		t.Error("should return false for invalid JSON")
	}
}

func TestIsPluginActive_NoEnabledPlugins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	data, err := json.Marshal(map[string]any{"hooks": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}

	if writeErr := os.WriteFile(path, data, 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}

	if isPluginActive(path, platform.PlatformClaudeCode) {
		t.Error("should return false when enabledPlugins is missing")
	}
}

func TestIsPluginActive_PluginNotEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	data, err := json.Marshal(map[string]any{
		"enabledPlugins": map[string]any{
			"other@plugin": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if writeErr := os.WriteFile(path, data, 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}

	if isPluginActive(path, platform.PlatformClaudeCode) {
		t.Error("should return false when thimble@thimble is not enabled")
	}
}

func TestIsPluginActive_EnabledButNoBinary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	data, err := json.Marshal(map[string]any{
		"enabledPlugins": map[string]any{
			"thimble@thimble": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if writeErr := os.WriteFile(path, data, 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	// Use PlatformUnknown so defaultPluginDir points to ~/.thimble/plugin
	// which almost certainly won't have a binary. This avoids false positives
	// if a real Claude plugin dir happens to have the binary installed.
	result := isPluginActive(path, platform.PlatformUnknown)
	// The function checks for binary existence — exercise the code path.
	_ = result
}

// --- SelfHeal ---

func TestSelfHeal_NoMismatch(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// No manifest file — should be a no-op.
	SelfHeal(dir, logger)
}

func TestSelfHeal_MismatchButNoPath(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// Create manifest with version mismatch but empty path.
	manifests := map[string]pluginManifest{
		"thimble": {Version: "v99.99.99", Path: ""},
	}

	data, err := json.Marshal(manifests)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "installed_plugins.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Should not panic; entry.Path is empty so it returns early.
	SelfHeal(dir, logger)
}

func TestSelfHeal_InvalidManifestJSON(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	if err := os.WriteFile(filepath.Join(dir, "installed_plugins.json"), []byte("bad json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should not panic.
	SelfHeal(dir, logger)
}

func TestSelfHeal_MismatchWithCopyTarget(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	targetPath := filepath.Join(dir, "thimble-target")

	manifests := map[string]pluginManifest{
		"thimble": {Version: "v99.99.99", Path: targetPath},
	}

	data, err := json.Marshal(manifests)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "installed_plugins.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// SelfHeal will try to copy the current executable to targetPath.
	// It will succeed or fail depending on permissions, but should not panic.
	SelfHeal(dir, logger)
}

// --- CheckVersionMismatch edge cases ---

func TestCheckVersionMismatch_NoThimbleEntryExtended(t *testing.T) {
	dir := t.TempDir()
	manifests := map[string]pluginManifest{
		"other-plugin": {Version: "1.0.0", Path: "/some/path"},
	}

	data, err := json.Marshal(manifests)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "installed_plugins.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	warning := CheckVersionMismatch(dir)
	if warning != "" {
		t.Errorf("expected empty for no thimble entry, got %q", warning)
	}
}

func TestCheckVersionMismatch_EmptyVersionExtended(t *testing.T) {
	dir := t.TempDir()
	manifests := map[string]pluginManifest{
		"thimble": {Version: "", Path: "/some/path"},
	}

	data, err := json.Marshal(manifests)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "installed_plugins.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	warning := CheckVersionMismatch(dir)
	if warning != "" {
		t.Errorf("expected empty for empty version, got %q", warning)
	}
}

// --- writeHookSettings MCP merge ---

func TestWriteHookSettings_MCPMerge(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Existing settings with an existing MCP server.
	existing := map[string]any{
		"mcpServers": map[string]any{
			"other-server": map[string]any{
				"command": "/usr/bin/other",
			},
		},
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Write config with thimble MCP server.
	config := map[string]any{
		"mcpServers": map[string]any{
			"thimble": map[string]any{
				"command": "/usr/bin/thimble",
			},
		},
	}

	if err := writeHookSettings(settingsPath, platform.PlatformClaudeCode, config); err != nil {
		t.Fatalf("writeHookSettings: %v", err)
	}

	merged, _ := os.ReadFile(settingsPath)

	var result map[string]any
	if err := json.Unmarshal(merged, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	mcpServers := result["mcpServers"].(map[string]any)
	if _, ok := mcpServers["other-server"]; !ok {
		t.Error("other-server should be preserved")
	}

	if _, ok := mcpServers["thimble"]; !ok {
		t.Error("thimble should be added")
	}
}

func TestWriteHookSettings_CorruptedExistingFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Write corrupted JSON.
	if err := os.WriteFile(settingsPath, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	config := buildHookConfig(platform.PlatformClaudeCode, "/usr/bin/thimble")

	err := writeHookSettings(settingsPath, platform.PlatformClaudeCode, config)
	if err != nil {
		t.Fatalf("writeHookSettings should recover from corrupted JSON: %v", err)
	}

	// File should now have valid JSON.
	data, _ := os.ReadFile(settingsPath)
	if !json.Valid(data) {
		t.Error("settings file should be valid JSON after recovery")
	}
}

// --- patchJSONVersion edge cases ---

func TestPatchJSONVersion_InvalidJSON(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := patchJSONVersion(path, "1.0.0")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- patchRegistryVersion edge cases ---

func TestPatchRegistryVersion_InvalidJSON(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := patchRegistryVersion(path, "/path", "1.0.0")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestPatchRegistryVersion_NilPlugins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reg.json")

	data, err := json.Marshal(map[string]any{"name": "test"})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Should not error — plugins key is nil, returns nil.
	err = patchRegistryVersion(path, "/path", "1.0.0")
	if err != nil {
		t.Errorf("expected nil for nil plugins key, got %v", err)
	}
}

// --- hookDebugger ---

func TestHookDebugger_WriteOutputExtended(t *testing.T) {
	dir := t.TempDir()
	dbg := &hookDebugger{dir: dir, ts: "20260328_100000.000"}

	dbg.writeOutput("PreToolUse", `{"decision":"allow"}`, 42000000) // 42ms

	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	data, _ := os.ReadFile(filepath.Join(dir, files[0].Name()))
	if !strings.Contains(string(data), "PreToolUse") {
		t.Error("output should contain event name")
	}
}

func TestHookDebugger_WriteErrorExtended(t *testing.T) {
	dir := t.TempDir()
	dbg := &hookDebugger{dir: dir, ts: "20260328_100000.000"}

	dbg.writeError("resolve_platform", os.ErrNotExist)

	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	data, _ := os.ReadFile(filepath.Join(dir, files[0].Name()))
	if !strings.Contains(string(data), "resolve_platform") {
		t.Error("error should contain phase")
	}
}

// --- buildHookConfig edge cases ---

func TestBuildHookConfig_CursorPlatform(t *testing.T) {
	config := buildHookConfig(platform.PlatformCursor, "/usr/local/bin/thimble")

	hooks, ok := config["hooks"].(map[string]any)
	if !ok {
		t.Fatal("Cursor config should have hooks")
	}

	// Cursor uses camelCase event names.
	if _, ok := hooks["preToolUse"]; !ok {
		t.Error("Cursor config should have preToolUse (camelCase)")
	}

	if _, ok := hooks["postToolUse"]; !ok {
		t.Error("Cursor config should have postToolUse (camelCase)")
	}
}

func TestBuildHookConfig_VSCodeCopilot(t *testing.T) {
	config := buildHookConfig(platform.PlatformVSCodeCopilot, "/usr/local/bin/thimble")

	hooks, ok := config["hooks"].(map[string]any)
	if !ok {
		t.Fatal("VSCode config should have hooks")
	}

	expectedEvents := []string{"PreToolUse", "PostToolUse", "PreCompact", "SessionStart", "UserPromptSubmit"}
	for _, event := range expectedEvents {
		if _, ok := hooks[event]; !ok {
			t.Errorf("VSCode config missing hook event: %s", event)
		}
	}
}

// --- isTestBinary additional cases ---

func TestIsTestBinary_EdgeCases(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// Double extension.
		{"foo.test.test", true},
		// Path with test in directory name.
		{"/tmp/test/thimble", false},
		// Windows path with backslashes.
		{`C:\test\thimble.exe`, false},
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

// --- Command registration ---

func TestSessionCommandRegistered(t *testing.T) {
	found := false

	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "session" {
			found = true
			// Verify subcommands.
			subNames := make(map[string]bool)
			for _, sub := range cmd.Commands() {
				subNames[sub.Name()] = true
			}

			for _, want := range []string{"export", "import", "list"} {
				if !subNames[want] {
					t.Errorf("session command missing subcommand %q", want)
				}
			}

			break
		}
	}

	if !found {
		t.Fatal("session command not registered on rootCmd")
	}
}

func TestWebCommandRegistered(t *testing.T) {
	found := false

	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "web" {
			found = true

			portFlag := cmd.Flags().Lookup("port")
			if portFlag == nil {
				t.Error("web command missing --port flag")
			}

			break
		}
	}

	if !found {
		t.Fatal("web command not registered on rootCmd")
	}
}

func TestHooklogCommandRegistered(t *testing.T) {
	found := false

	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "hooklog" {
			found = true
			// Check all expected flags.
			for _, flag := range []string{"limit", "platform", "event", "tool", "blocked", "clear", "report", "debug"} {
				if cmd.Flags().Lookup(flag) == nil {
					t.Errorf("hooklog command missing --%s flag", flag)
				}
			}

			break
		}
	}

	if !found {
		t.Fatal("hooklog command not registered on rootCmd")
	}
}

func TestLintCommandRegistered(t *testing.T) {
	found := false

	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "lint" {
			found = true

			for _, flag := range []string{"fix", "fast", "enable", "timeout"} {
				if cmd.Flags().Lookup(flag) == nil {
					t.Errorf("lint command missing --%s flag", flag)
				}
			}

			break
		}
	}

	if !found {
		t.Fatal("lint command not registered on rootCmd")
	}
}

func TestReportCommandRegistered(t *testing.T) {
	found := false

	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "report" {
			found = true

			subNames := make(map[string]bool)
			for _, sub := range cmd.Commands() {
				subNames[sub.Name()] = true
			}

			for _, want := range []string{"list", "show", "delete"} {
				if !subNames[want] {
					t.Errorf("report command missing subcommand %q", want)
				}
			}

			break
		}
	}

	if !found {
		t.Fatal("report command not registered on rootCmd")
	}
}

// --- formatDuration additional cases ---

func TestFormatDuration_ExactMinute(t *testing.T) {
	got := formatDuration(60000000000) // 1 minute exactly
	if got != "1m0s" {
		t.Errorf("formatDuration(1m) = %q, want %q", got, "1m0s")
	}
}

func TestFormatDuration_ExactHour(t *testing.T) {
	got := formatDuration(3600000000000) // 1 hour exactly
	if got != "1h0m0s" {
		t.Errorf("formatDuration(1h) = %q, want %q", got, "1h0m0s")
	}
}

// --- prepareHookPayload ---

func TestPrepareHookPayload_UnknownPlatformExtended(t *testing.T) {
	adapter, _, _, dbg, err := prepareHookPayload([]string{"unknown-platform", "pretooluse"}, []byte(`{}`))
	if err == nil {
		t.Error("expected error for unknown platform")
	}

	if adapter != nil {
		t.Error("adapter should be nil on error")
	}

	_ = dbg
}

func TestPrepareHookPayload_UnknownEventExtended(t *testing.T) {
	adapter, _, _, dbg, err := prepareHookPayload([]string{"claude-code", "unknown-event"}, []byte(`{}`))
	if err == nil {
		t.Error("expected error for unknown event")
	}

	_ = adapter
	_ = dbg
}

func TestPrepareHookPayload_ValidInput(t *testing.T) {
	adapter, event, payload, dbg, err := prepareHookPayload(
		[]string{"claude-code", "pretooluse"},
		[]byte(`{"tool_name":"Bash"}`),
	)
	if err != nil {
		t.Fatalf("prepareHookPayload: %v", err)
	}

	if adapter == nil {
		t.Error("adapter should not be nil")
	}

	if event != "PreToolUse" {
		t.Errorf("event = %q, want PreToolUse", event)
	}

	if payload == nil {
		t.Error("payload should not be nil")
	}

	if dbg == nil {
		t.Error("debugger should not be nil")
	}
}

func TestPrepareHookPayload_ExtraArgs(t *testing.T) {
	// Should not panic with extra args.
	adapter, _, _, dbg, err := prepareHookPayload(
		[]string{"claude-code", "pretooluse", "extra1", "extra2"},
		[]byte(`{}`),
	)
	if err != nil {
		t.Fatalf("prepareHookPayload with extra args: %v", err)
	}

	_ = adapter
	_ = dbg
}
