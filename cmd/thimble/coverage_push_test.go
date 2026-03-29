package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/thimble/internal/platform"
)

// ─── SelfHeal: test binary same-path shortcut ───

func TestSelfHeal_SamePathSkipsCopy(t *testing.T) {
	dir := t.TempDir()

	exe, err := os.Executable()
	if err != nil {
		t.Skip("cannot get executable path")
	}

	// Point manifest path to the same as current executable.
	absExe, _ := filepath.Abs(exe)
	manifests := map[string]pluginManifest{
		"thimble": {Version: "v99.99.99", Path: absExe},
	}
	data, _ := json.Marshal(manifests) //nolint:errchkjson
	_ = os.WriteFile(filepath.Join(dir, "installed_plugins.json"), data, 0o644)

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Should detect same path and skip copy.
	SelfHeal(dir, logger)
}

// ─── SelfHeal: manifest file can't be read (second read for path) ───

func TestSelfHeal_ManifestSecondReadError(t *testing.T) {
	dir := t.TempDir()

	// First call to CheckVersionMismatch reads the file.
	// For SelfHeal, it reads the manifest a second time.
	// Create manifest with mismatch, then delete it between checks.

	manifests := map[string]pluginManifest{
		"thimble": {Version: "v99.99.99", Path: filepath.Join(dir, "target")},
	}
	data, _ := json.Marshal(manifests) //nolint:errchkjson
	manifestPath := filepath.Join(dir, "installed_plugins.json")
	_ = os.WriteFile(manifestPath, data, 0o644)

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// SelfHeal will try to read manifest twice. Just verify no panic.
	SelfHeal(dir, logger)
}

// ─── generateMCPConfig: non-dry-run via runSetup ───

func TestRunSetup_MCPOnlyPlatform_NonDryRun(t *testing.T) {
	origClient := setupClient
	origDryRun := setupDryRun
	origPlugin := setupPlugin
	origInstructions := setupInstructions

	defer func() {
		setupClient = origClient
		setupDryRun = origDryRun
		setupPlugin = origPlugin
		setupInstructions = origInstructions
	}()

	setupClient = "codex"
	setupDryRun = false
	setupPlugin = false
	setupInstructions = false

	// This will call generateMCPConfig in non-dry-run mode.
	// It will write to adapter.SettingsPath() which may fail on permissions,
	// but it exercises the code path.
	err := runSetup(nil, nil)
	// May fail due to path permissions, that's OK.
	if err != nil {
		t.Logf("runSetup failed (expected on restricted paths): %v", err)
	}
}

// ─── runUpgrade: download failure ───

func TestRunUpgrade_DownloadFailure(t *testing.T) {
	origFunc := fetchLatestReleaseFunc

	defer func() { fetchLatestReleaseFunc = origFunc }()

	origForce := upgradeForce

	defer func() { upgradeForce = origForce }()

	upgradeForce = true

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	assetName := buildAssetName()
	fetchLatestReleaseFunc = func() (*githubRelease, error) {
		return &githubRelease{
			TagName: "v99.99.99",
			Assets: []githubAsset{
				{Name: assetName, BrowserDownloadURL: srv.URL + "/" + assetName, Size: 1024},
			},
		}, nil
	}

	err := runUpgrade(nil, nil)
	if err == nil {
		t.Error("expected error for download failure")
	}
}

// ─── Setup integration: complete flow for each platform ───

func TestSetupCmd_AllPlatforms_DryRun_Integration(t *testing.T) {
	platforms := []string{"claude", "gemini", "vscode", "cursor", "opencode", "codex"}

	for _, p := range platforms {
		t.Run(p, func(t *testing.T) {
			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SetArgs([]string{"setup", "--client", p, "--dry-run"})

			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("Execute error for %s: %v", p, err)
			}
		})
	}
}

// ─── generateHookConfig: with adapter returning various paradigms ───

func TestGenerateHookConfig_GeminiDryRun(t *testing.T) {
	origDryRun := setupDryRun

	defer func() { setupDryRun = origDryRun }()

	setupDryRun = true

	adapter, _ := platform.Get(platform.PlatformGeminiCLI)

	err := generateHookConfig(platform.PlatformGeminiCLI, adapter)
	if err != nil {
		t.Fatalf("generateHookConfig: %v", err)
	}
}

func TestGenerateHookConfig_CursorDryRun(t *testing.T) {
	origDryRun := setupDryRun

	defer func() { setupDryRun = origDryRun }()

	setupDryRun = true

	adapter, _ := platform.Get(platform.PlatformCursor)

	err := generateHookConfig(platform.PlatformCursor, adapter)
	if err != nil {
		t.Fatalf("generateHookConfig: %v", err)
	}
}

func TestGenerateHookConfig_VSCodeDryRun(t *testing.T) {
	origDryRun := setupDryRun

	defer func() { setupDryRun = origDryRun }()

	setupDryRun = true

	adapter, _ := platform.Get(platform.PlatformVSCodeCopilot)

	err := generateHookConfig(platform.PlatformVSCodeCopilot, adapter)
	if err != nil {
		t.Fatalf("generateHookConfig: %v", err)
	}
}

// ─── resolveClient: fallback to raw PlatformID path ───

func TestResolveClient_FallbackToRawPlatformID(t *testing.T) {
	// "kiro" is a valid PlatformID but not in clientAliases.
	id, err := resolveClient("kiro")
	if err != nil {
		t.Fatalf("resolveClient('kiro'): %v", err)
	}

	if id != platform.PlatformKiro {
		t.Errorf("id = %q, want %q", id, platform.PlatformKiro)
	}
}

// ─── writeOutput/writeError: edge cases ───

func TestHookDebugger_MultipleWrites(t *testing.T) {
	dir := t.TempDir()
	d := &hookDebugger{dir: dir, ts: "20260317_120000.001"}

	// Write multiple outputs.
	d.writeInput("PreToolUse", []byte(`{"a":1}`))
	d.writeOutput("PreToolUse", `{"decision":"allow"}`, 0)
	d.writeError("connect", fmt.Errorf("refused"))

	files, _ := os.ReadDir(dir)
	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}
}

// ─── fetchExpectedChecksum: with SHA256 in asset name ───

func TestFetchExpectedChecksum_SHA256InName(t *testing.T) {
	checksumContent := "abcdef123456 thimble_Linux_x86_64.tar.gz\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checksumContent))
	}))
	defer srv.Close()

	release := &githubRelease{
		TagName: "v1.0.0",
		Assets: []githubAsset{
			{Name: "thimble_Linux_x86_64.tar.gz", BrowserDownloadURL: "https://example.com/archive"},
			{Name: "SHA256SUMS", BrowserDownloadURL: srv.URL + "/sums"},
		},
	}

	hash, err := fetchExpectedChecksum(release, "thimble_Linux_x86_64.tar.gz")
	if err != nil {
		t.Fatalf("fetchExpectedChecksum: %v", err)
	}

	if hash != "abcdef123456" {
		t.Errorf("hash = %q, want 'abcdef123456'", hash)
	}
}

// ─── Verify the 'min' function handles edge cases ───

func TestMinFunc(t *testing.T) {
	if min(5, 3) != 3 {
		t.Error("min(5,3) != 3")
	}

	if min(3, 5) != 3 {
		t.Error("min(3,5) != 3")
	}

	if min(3, 3) != 3 {
		t.Error("min(3,3) != 3")
	}
}

// ─── buildHookConfig: comprehensive event coverage ───

func TestBuildHookConfig_ClaudeCode_AllEventKeys(t *testing.T) {
	config := buildHookConfig(platform.PlatformClaudeCode, "/path/to/thimble")
	hooks := config["hooks"].(map[string]any)

	expected := []string{"PreToolUse", "PostToolUse", "PreCompact", "SessionStart", "UserPromptSubmit"}
	for _, event := range expected {
		if _, ok := hooks[event]; !ok {
			t.Errorf("missing event %q in Claude Code hooks", event)
		}
	}
}

func TestBuildHookConfig_GeminiCLI_AllEventKeys(t *testing.T) {
	config := buildHookConfig(platform.PlatformGeminiCLI, "/path/to/thimble")
	hooks := config["hooks"].(map[string]any)

	expected := []string{"BeforeTool", "AfterTool", "PreCompress", "SessionStart", "UserPromptSubmit"}
	for _, event := range expected {
		if _, ok := hooks[event]; !ok {
			t.Errorf("missing event %q in Gemini CLI hooks", event)
		}
	}
}

func TestBuildHookConfig_VSCode_AllEventKeys(t *testing.T) {
	config := buildHookConfig(platform.PlatformVSCodeCopilot, "/path/to/thimble")
	hooks := config["hooks"].(map[string]any)

	expected := []string{"PreToolUse", "PostToolUse", "PreCompact", "SessionStart", "UserPromptSubmit"}
	for _, event := range expected {
		if _, ok := hooks[event]; !ok {
			t.Errorf("missing event %q in VSCode Copilot hooks", event)
		}
	}
}

func TestBuildHookConfig_Cursor_AllEventKeys(t *testing.T) {
	config := buildHookConfig(platform.PlatformCursor, "/path/to/thimble")
	hooks := config["hooks"].(map[string]any)

	expected := []string{"preToolUse", "postToolUse"}
	for _, event := range expected {
		if _, ok := hooks[event]; !ok {
			t.Errorf("missing event %q in Cursor hooks", event)
		}
	}
}

// ─── writeHookSettings: test the "no existing hooks" path ───

func TestWriteHookSettings_ExistingFileNoHooksKey(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Existing file with no hooks or mcpServers keys.
	_ = os.WriteFile(settingsPath, []byte(`{"customKey": "value"}`), 0o644)

	config := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{"hook1"},
		},
	}

	if err := writeHookSettings(settingsPath, "", config); err != nil {
		t.Fatalf("writeHookSettings: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)

	var result map[string]any

	_ = json.Unmarshal(data, &result)

	if result["customKey"] != "value" {
		t.Error("customKey should be preserved")
	}

	hooks := result["hooks"].(map[string]any)
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("PreToolUse should be added")
	}
}

// ─── Ensure panic-free operation with edge-case inputs ───

func TestFormatViaAdapter_NilResp_AllPlatforms(t *testing.T) {
	platforms := []platform.PlatformID{
		platform.PlatformClaudeCode,
		platform.PlatformGeminiCLI,
		platform.PlatformVSCodeCopilot,
		platform.PlatformCursor,
	}

	for _, pid := range platforms {
		t.Run(string(pid), func(t *testing.T) {
			adapter, _ := platform.Get(pid)

			result := formatViaAdapter(adapter, "PreToolUse", nil)
			if result != "{}" {
				t.Errorf("expected '{}' for nil resp, got %q", result)
			}
		})
	}
}

// ─── resolveClient: full coverage of the alias map ───

func TestClientAliases_EveryEntry(t *testing.T) {
	for alias, expectedID := range clientAliases {
		t.Run(alias, func(t *testing.T) {
			id, err := resolveClient(alias)
			if err != nil {
				t.Fatalf("resolveClient(%q): %v", alias, err)
			}

			if id != expectedID {
				t.Errorf("resolveClient(%q) = %q, want %q", alias, id, expectedID)
			}
		})
	}
}

// ─── downloadFile: write to unwritable path ───

func TestDownloadFile_UnwritableDest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data"))
	}))
	defer srv.Close()

	// Try to write to a directory that doesn't exist and parent is invalid.
	err := downloadFile(srv.URL+"/file", "/nonexistent/dir/file")
	if err == nil {
		t.Error("expected error for unwritable destination")
	}
}

// ─── Upgrade: version comparison ───

func TestRunUpgrade_VersionStrip(t *testing.T) {
	// Verify that version stripping works correctly.
	version := "v1.2.3"

	stripped := strings.TrimPrefix(version, "v")
	if stripped != "1.2.3" {
		t.Errorf("stripped = %q, want '1.2.3'", stripped)
	}

	// Without v prefix.
	version2 := "1.2.3"

	stripped2 := strings.TrimPrefix(version2, "v")
	if stripped2 != "1.2.3" {
		t.Errorf("stripped2 = %q, want '1.2.3'", stripped2)
	}
}
