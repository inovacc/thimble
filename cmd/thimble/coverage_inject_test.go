package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/thimble/internal/platform"
)

// ─── prepareHookPayload ───

func TestPrepareHookPayload_Success(t *testing.T) {
	args := []string{"claude-code", "PreToolUse"}
	payload := []byte(`{"tool_name": "Bash", "tool_input": {"command": "echo hi"}}`)

	adapter, canonicalEvent, grpcPayload, dbg, err := prepareHookPayload(args, payload)
	if err != nil {
		t.Fatalf("prepareHookPayload: %v", err)
	}

	if adapter == nil {
		t.Error("adapter should not be nil")
	}

	if canonicalEvent != "PreToolUse" {
		t.Errorf("canonicalEvent = %q, want 'PreToolUse'", canonicalEvent)
	}

	if grpcPayload == nil {
		t.Error("grpcPayload should not be nil")
	}

	if dbg == nil {
		t.Error("dbg should not be nil")
	}
}

func TestPrepareHookPayload_UnknownPlatform(t *testing.T) {
	args := []string{"nonexistent-platform", "PreToolUse"}
	payload := []byte(`{}`)

	_, _, _, _, err := prepareHookPayload(args, payload) //nolint:dogsled // testing error path, return values intentionally discarded
	if err == nil {
		t.Error("expected error for unknown platform")
	}

	if !strings.Contains(err.Error(), "unknown platform") {
		t.Errorf("error = %q, want 'unknown platform'", err.Error())
	}
}

func TestPrepareHookPayload_UnknownEvent(t *testing.T) {
	args := []string{"claude-code", "InvalidEvent"}
	payload := []byte(`{}`)

	_, _, _, _, err := prepareHookPayload(args, payload) //nolint:dogsled // testing error path, return values intentionally discarded
	if err == nil {
		t.Error("expected error for unknown event")
	}

	if !strings.Contains(err.Error(), "unknown event") {
		t.Errorf("error = %q, want 'unknown event'", err.Error())
	}
}

func TestPrepareHookPayload_WithExtraArgs(t *testing.T) {
	args := []string{"claude-code", "PostToolUse", "extra-arg-1", "extra-arg-2"}
	payload := []byte(`{"tool_name": "Read"}`)

	adapter, canonicalEvent, grpcPayload, _, err := prepareHookPayload(args, payload)
	if err != nil {
		t.Fatalf("prepareHookPayload: %v", err)
	}

	if adapter == nil {
		t.Error("adapter should not be nil")
	}

	if canonicalEvent != "PostToolUse" {
		t.Errorf("canonicalEvent = %q, want 'PostToolUse'", canonicalEvent)
	}

	if grpcPayload == nil {
		t.Error("grpcPayload should not be nil")
	}
}

func TestPrepareHookPayload_AllEvents(t *testing.T) {
	events := []struct {
		raw       string
		canonical string
	}{
		{"pretooluse", "PreToolUse"},
		{"posttooluse", "PostToolUse"},
		{"precompact", "PreCompact"},
		{"sessionstart", "SessionStart"},
		{"userpromptsubmit", "UserPromptSubmit"},
		{"beforetool", "PreToolUse"},
		{"aftertool", "PostToolUse"},
		{"precompress", "PreCompact"},
	}

	for _, tt := range events {
		t.Run(tt.raw, func(t *testing.T) {
			args := []string{"claude-code", tt.raw}
			payload := []byte(`{}`)

			_, canonicalEvent, _, _, err := prepareHookPayload(args, payload)
			if err != nil {
				t.Fatalf("prepareHookPayload: %v", err)
			}

			if canonicalEvent != tt.canonical {
				t.Errorf("canonicalEvent = %q, want %q", canonicalEvent, tt.canonical)
			}
		})
	}
}

func TestPrepareHookPayload_GeminiPlatform(t *testing.T) {
	args := []string{"gemini-cli", "beforetool"}
	payload := []byte(`{"tool": "test"}`)

	adapter, canonicalEvent, _, _, err := prepareHookPayload(args, payload)
	if err != nil {
		t.Fatalf("prepareHookPayload: %v", err)
	}

	if adapter.ID() != platform.PlatformGeminiCLI {
		t.Errorf("adapter.ID() = %q, want %q", adapter.ID(), platform.PlatformGeminiCLI)
	}

	if canonicalEvent != "PreToolUse" {
		t.Errorf("canonicalEvent = %q, want 'PreToolUse'", canonicalEvent)
	}
}

func TestPrepareHookPayload_CursorPlatform(t *testing.T) {
	args := []string{"cursor", "pretooluse"}
	payload := []byte(`{"tool": "Shell"}`)

	adapter, canonicalEvent, _, _, err := prepareHookPayload(args, payload)
	if err != nil {
		t.Fatalf("prepareHookPayload: %v", err)
	}

	if adapter.ID() != platform.PlatformCursor {
		t.Errorf("adapter.ID() = %q, want %q", adapter.ID(), platform.PlatformCursor)
	}

	if canonicalEvent != "PreToolUse" {
		t.Errorf("canonicalEvent = %q, want 'PreToolUse'", canonicalEvent)
	}
}

// ─── runUpgrade with mock fetchLatestRelease ───

func TestRunUpgrade_AlreadyUpToDate(t *testing.T) {
	origFunc := fetchLatestReleaseFunc

	defer func() { fetchLatestReleaseFunc = origFunc }()

	origForce := upgradeForce

	defer func() { upgradeForce = origForce }()

	upgradeForce = false

	// Mock: return release with same version as current.
	fetchLatestReleaseFunc = func() (*githubRelease, error) {
		return &githubRelease{
			TagName: "v" + strings.TrimPrefix(Version, "v"),
			Assets:  []githubAsset{},
		}, nil
	}

	err := runUpgrade(nil, nil)
	if err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
}

func TestRunUpgrade_FetchError(t *testing.T) {
	origFunc := fetchLatestReleaseFunc

	defer func() { fetchLatestReleaseFunc = origFunc }()

	fetchLatestReleaseFunc = func() (*githubRelease, error) {
		return nil, fmt.Errorf("network error")
	}

	err := runUpgrade(nil, nil)
	if err == nil {
		t.Error("expected error for fetch failure")
	}

	if !strings.Contains(err.Error(), "fetch latest release") {
		t.Errorf("error = %q, want 'fetch latest release'", err.Error())
	}
}

func TestRunUpgrade_NoAssetForPlatform(t *testing.T) {
	origFunc := fetchLatestReleaseFunc

	defer func() { fetchLatestReleaseFunc = origFunc }()

	origForce := upgradeForce

	defer func() { upgradeForce = origForce }()

	upgradeForce = true

	fetchLatestReleaseFunc = func() (*githubRelease, error) {
		return &githubRelease{
			TagName: "v99.99.99",
			Assets: []githubAsset{
				{Name: "thimble_SomeOtherOS_arm64.tar.gz", BrowserDownloadURL: "https://example.com/other"},
			},
		}, nil
	}

	err := runUpgrade(nil, nil)
	if err == nil {
		t.Error("expected error when no matching asset")
	}

	if !strings.Contains(err.Error(), "no asset found") {
		t.Errorf("error = %q, want 'no asset found'", err.Error())
	}
}

func TestRunUpgrade_FullFlow_ChecksumMismatch(t *testing.T) {
	// Skip if os.Executable() is unreliable (e.g., CI containers, sandboxed environments).
	execPath, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable() unavailable: %v", err)
	}

	if _, err := filepath.EvalSymlinks(execPath); err != nil {
		t.Skipf("cannot resolve executable symlinks: %v", err)
	}

	// Create test HTTP servers for download and checksums.
	archiveSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve a valid archive based on platform.
		if strings.HasSuffix(r.URL.Path, ".zip") {
			createZipToWriter(t, w, binaryName(), []byte("fake binary"))
		} else {
			createTarGzToWriter(t, w, binaryName(), []byte("fake binary"))
		}
	}))
	defer archiveSrv.Close()

	checksumSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a checksum that won't match.
		_, _ = fmt.Fprintf(w, "0000000000000000 %s\n", buildAssetName())
	}))
	defer checksumSrv.Close()

	origFunc := fetchLatestReleaseFunc

	defer func() { fetchLatestReleaseFunc = origFunc }()

	origForce := upgradeForce

	defer func() { upgradeForce = origForce }()

	upgradeForce = true

	assetName := buildAssetName()

	fetchLatestReleaseFunc = func() (*githubRelease, error) {
		return &githubRelease{
			TagName: "v99.99.99",
			Assets: []githubAsset{
				{Name: assetName, BrowserDownloadURL: archiveSrv.URL + "/" + assetName, Size: 1024},
				{Name: "checksums.txt", BrowserDownloadURL: checksumSrv.URL + "/checksums.txt"},
			},
		}, nil
	}

	err = runUpgrade(nil, nil)
	if err == nil {
		t.Error("expected error for checksum mismatch")
	}

	if err != nil && !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error = %q, want 'checksum mismatch'", err.Error())
	}
}

// ─── helper: write tar.gz to http.ResponseWriter ───

func createTarGzToWriter(t *testing.T, w http.ResponseWriter, name string, data []byte) {
	t.Helper()

	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	_ = tw.WriteHeader(&tar.Header{
		Name: name,
		Size: int64(len(data)),
		Mode: 0o755,
	})
	_, _ = tw.Write(data)
	_ = tw.Close()
	_ = gw.Close()
}

func createZipToWriter(t *testing.T, w http.ResponseWriter, name string, data []byte) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.zip")

	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	zf, _ := zw.Create(name)
	_, _ = zf.Write(data)
	_ = zw.Close()
	_ = f.Close()

	content, _ := os.ReadFile(path)
	_, _ = w.Write(content)
}

// ─── generateHookConfig + generateMCPConfig: more non-dry-run paths ───

func TestGenerateHookConfig_VSCode_NonDryRun(t *testing.T) {
	origDryRun := setupDryRun

	defer func() { setupDryRun = origDryRun }()

	setupDryRun = false

	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	adapter, _ := platform.Get(platform.PlatformVSCodeCopilot)

	config := buildHookConfig(platform.PlatformVSCodeCopilot, "/path/to/thimble")
	config["mcpServers"] = map[string]any{
		"thimble": map[string]any{"command": "/path/to/thimble", "args": []string{}},
	}

	if err := writeHookSettings(settingsPath, platform.PlatformVSCodeCopilot, config); err != nil {
		t.Fatalf("writeHookSettings: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)

	var result map[string]any

	_ = json.Unmarshal(data, &result)

	hooks := result["hooks"].(map[string]any)
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("missing PreToolUse hook")
	}

	_ = adapter
}

func TestGenerateMCPConfig_NonDryRun_FullWrite(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "config.json")

	adapter, _ := platform.Get(platform.PlatformCodex)

	config := map[string]any{
		"mcpServers": map[string]any{
			"thimble": map[string]any{
				"command": "thimble",
				"args":    []string{},
			},
		},
	}

	if err := writeHookSettings(settingsPath, platform.PlatformCodex, config); err != nil {
		t.Fatalf("writeHookSettings: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	if !json.Valid(data) {
		t.Error("output is not valid JSON")
	}

	_ = adapter
}
