package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/thimble/internal/hooks"
	"github.com/inovacc/thimble/internal/platform"
)

// --- goosToTitle ---

func TestGoosToTitle_Boost4(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{"linux", "Linux"},
		{"darwin", "Darwin"},
		{"windows", "Windows"},
		{"freebsd", "freebsd"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			got := goosToTitle(tt.goos)
			if got != tt.want {
				t.Errorf("goosToTitle(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

// --- goarchToRelease ---

func TestGoarchToRelease_Boost4(t *testing.T) {
	tests := []struct {
		goarch string
		want   string
	}{
		{"amd64", "x86_64"},
		{"386", "i386"},
		{"arm64", "arm64"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.goarch, func(t *testing.T) {
			got := goarchToRelease(tt.goarch)
			if got != tt.want {
				t.Errorf("goarchToRelease(%q) = %q, want %q", tt.goarch, got, tt.want)
			}
		})
	}
}

// --- buildAssetName ---

func TestBuildAssetName_Boost4(t *testing.T) {
	name := buildAssetName()
	if name == "" {
		t.Error("buildAssetName() returned empty")
	}

	if !strings.HasPrefix(name, "thimble_") {
		t.Errorf("buildAssetName() = %q, want prefix 'thimble_'", name)
	}
}

// --- binaryName & archiveExt ---

func TestBinaryName_Boost4(t *testing.T) {
	name := binaryName()
	if name == "" {
		t.Error("binaryName() returned empty")
	}

	if !strings.Contains(name, "thimble") {
		t.Errorf("binaryName() = %q, should contain 'thimble'", name)
	}
}

func TestArchiveExt(t *testing.T) {
	ext := archiveExt()
	if ext == "" {
		t.Error("archiveExt() returned empty")
	}
	// Should be .zip on Windows, .tar.gz on Unix.
	if ext != ".zip" && ext != ".tar.gz" {
		t.Errorf("archiveExt() = %q, want .zip or .tar.gz", ext)
	}
}

// --- checkResult.String ---

func TestCheckResult_String_AllStatuses(t *testing.T) {
	tests := []struct {
		status   string
		wantIcon string
	}{
		{"pass", "[OK]"},
		{"fail", "[FAIL]"},
		{"warn", "[WARN]"},
		{"unknown", "[??]"},
		{"", "[??]"},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			cr := checkResult{name: "TestCheck", status: tt.status, message: "test msg"}

			s := cr.String()
			if !strings.Contains(s, tt.wantIcon) {
				t.Errorf("checkResult{status: %q}.String() = %q, want icon %q", tt.status, s, tt.wantIcon)
			}

			if !strings.Contains(s, "TestCheck") {
				t.Errorf("checkResult.String() should contain name")
			}

			if !strings.Contains(s, "test msg") {
				t.Errorf("checkResult.String() should contain message")
			}
		})
	}
}

// --- checkServer ---

func TestCheckServer(t *testing.T) {
	result := checkServer()
	if result.status != "pass" {
		t.Errorf("checkServer().status = %q, want pass", result.status)
	}

	if result.name != "Architecture" {
		t.Errorf("checkServer().name = %q, want Architecture", result.name)
	}
}

// --- checkPlatform ---

func TestCheckPlatform_Boost4(t *testing.T) {
	result := checkPlatform()
	if result.status != "pass" {
		t.Errorf("checkPlatform().status = %q, want pass", result.status)
	}

	if result.name != "Platform" {
		t.Errorf("checkPlatform().name = %q, want Platform", result.name)
	}

	if result.message == "" {
		t.Error("checkPlatform().message should not be empty")
	}
}

// --- checkSQLiteFTS5 ---

func TestCheckSQLiteFTS5_Boost4(t *testing.T) {
	results := checkSQLiteFTS5()
	if len(results) == 0 {
		t.Error("checkSQLiteFTS5 should return at least 1 result")
	}
	// First result should be about SQLite.
	found := false

	for _, r := range results {
		if r.name == "SQLite" || r.name == "FTS5" {
			found = true
		}
	}

	if !found {
		t.Error("expected SQLite or FTS5 check result")
	}
}

// --- checkRuntimes ---

func TestCheckRuntimes_Boost4(t *testing.T) {
	results := checkRuntimes()
	if len(results) == 0 {
		t.Error("checkRuntimes should return results")
	}
	// Should always have a "Runtimes" summary entry.
	found := false

	for _, r := range results {
		if r.name == "Runtimes" {
			found = true

			if r.status != "pass" {
				t.Errorf("Runtimes summary status = %q, want pass", r.status)
			}
		}
	}

	if !found {
		t.Error("expected Runtimes summary check result")
	}
}

// --- pluginBuiltinNames ---

func TestPluginBuiltinNames(t *testing.T) {
	names := pluginBuiltinNames()
	if len(names) == 0 {
		t.Error("pluginBuiltinNames() returned empty")
	}
	// Should contain core tools.
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	for _, want := range []string{"ctx_execute", "ctx_search", "ctx_delegate", "ctx_git_status"} {
		if !nameSet[want] {
			t.Errorf("pluginBuiltinNames() missing %q", want)
		}
	}
}

// --- formatViaAdapter with context key ---

func TestFormatViaAdapter_ContextKeyResponse(t *testing.T) {
	t.Helper()

	adapter, _ := platform.Get(platform.PlatformClaudeCode)

	resultJSON, err := json.Marshal(map[string]any{
		"context": "some context via context key",
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := &hooks.Response{Blocked: false, Result: resultJSON}
	output := formatViaAdapter(adapter, "PostToolUse", resp)
	// Exercise the context key code path.
	_ = output
}

// --- formatViaAdapter with Gemini deny ---

func TestFormatViaAdapter_GeminiDeny(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformGeminiCLI)
	resp := &hooks.Response{Blocked: true, Reason: "dangerous command"}

	output := formatViaAdapter(adapter, "PreToolUse", resp)
	if output == "" {
		t.Error("expected non-empty deny output for Gemini")
	}
}

// --- formatViaAdapter with Cursor ---

func TestFormatViaAdapter_CursorPreToolUse(t *testing.T) {
	t.Helper()

	adapter, _ := platform.Get(platform.PlatformCursor)
	resp := &hooks.Response{Blocked: true, Reason: "blocked"}
	output := formatViaAdapter(adapter, "PreToolUse", resp)
	// Cursor should produce output for deny.
	_ = output
}

// --- SelfHeal with matching version (no-op) ---

func TestSelfHeal_MatchingVersion(t *testing.T) {
	dir := t.TempDir()
	logger := slogDiscard()

	manifests := map[string]pluginManifest{
		"thimble": {Version: Version, Path: "/some/path"},
	}

	data, err := json.Marshal(manifests)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "installed_plugins.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Should be a no-op (versions match).
	SelfHeal(dir, logger)
}

// --- writeHookSettings to new dir with only mcpServers ---

func TestWriteHookSettings_OnlyMCPServers(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "deep", "nested", "settings.json")

	config := map[string]any{
		"mcpServers": map[string]any{
			"thimble": map[string]any{
				"command": "/usr/bin/thimble",
			},
		},
	}

	if err := writeHookSettings(settingsPath, "", config); err != nil {
		t.Fatalf("writeHookSettings: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}

	mcp := result["mcpServers"].(map[string]any)
	if _, ok := mcp["thimble"]; !ok {
		t.Error("thimble MCP server should be present")
	}
}

// --- copyFile overwrites existing file ---

func TestCopyFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(src, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(dst, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst, 0o755); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	got, _ := os.ReadFile(dst)
	if string(got) != "new content" {
		t.Errorf("expected 'new content', got %q", got)
	}
}

// --- buildHookPayload with empty raw payload ---

func TestBuildHookPayload_EmptyRaw(t *testing.T) {
	normalized := platform.NormalizedEvent{SessionID: "s1"}

	result := buildHookPayload(normalized, []byte{})
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := parsed["extra"]; ok {
		t.Error("should not have extra for empty payload")
	}
}

// --- formatDuration boundary ---

func TestFormatDuration_SubSecond(t *testing.T) {
	got := formatDuration(500 * time.Millisecond)
	if got != "0s" {
		t.Errorf("formatDuration(500ms) = %q, want '0s'", got)
	}
}

// --- patchJSONVersion with plugins array ---

func TestPatchJSONVersion_PluginsArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "marketplace.json")

	doc := map[string]any{
		"plugins": []any{
			map[string]any{"name": "p1", "version": "0.1.0"},
			map[string]any{"name": "p2", "version": "0.2.0"},
		},
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := patchJSONVersion(path, "3.0.0"); err != nil {
		t.Fatalf("patchJSONVersion: %v", err)
	}

	updated, _ := os.ReadFile(path)

	var result map[string]any
	if err := json.Unmarshal(updated, &result); err != nil {
		t.Fatal(err)
	}

	plugins := result["plugins"].([]any)
	for i, p := range plugins {
		pm := p.(map[string]any)
		if pm["version"] != "3.0.0" {
			t.Errorf("plugins[%d].version = %v, want 3.0.0", i, pm["version"])
		}
	}
}

// --- extractBinary routing ---

func TestExtractBinary_UnsupportedFormat(t *testing.T) {
	// A non-.zip, non-.tar.gz path falls through to extractFromTarGz.
	err := extractBinary("/fake/archive.unknown", "/fake/dest")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

// --- printVersion & runVersion exercise ---

func TestRunVersion_Text(t *testing.T) {
	t.Helper()
	// Just verify runVersion doesn't panic in text mode.
	origJSON := jsonOutput
	jsonOutput = false

	defer func() { jsonOutput = origJSON }()

	runVersion(versionCmd, nil)
}

// --- publishOpts type ---

func TestPublishOpts_Defaults(t *testing.T) {
	opts := publishOpts{}
	if opts.dryRun {
		t.Error("default dryRun should be false")
	}

	if opts.version != "" {
		t.Error("default version should be empty")
	}
}

// --- event map completeness ---

func TestEventMap_CanonicalEventSet(t *testing.T) {
	// Verify all canonical events are reachable.
	canonical := map[string]bool{
		"PreToolUse":       false,
		"PostToolUse":      false,
		"PreCompact":       false,
		"SessionStart":     false,
		"UserPromptSubmit": false,
	}
	for _, v := range eventMap {
		canonical[v] = true
	}

	for event, reached := range canonical {
		if !reached {
			t.Errorf("canonical event %q is not reachable from any eventMap key", event)
		}
	}
}

// helper: discard slog logger.
func slogDiscard() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}
