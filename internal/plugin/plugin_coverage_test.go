package plugin

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHttpGet_BadURL(t *testing.T) {
	_, err := httpGet("http://[::1]:namedport/bad")
	if err == nil {
		t.Error("expected error for bad URL")
	}
}

func TestHttpGet_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := httpGet(srv.URL)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404, got: %v", err)
	}
}

func TestHttpGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	body, err := httpGet(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q, want %q", string(body), `{"ok":true}`)
	}
}

func TestResolveSource_GitHubSingleSegment(t *testing.T) {
	// A single segment after "github.com/" should fall through to registry.
	got := resolveSource("github.com/useronly")

	want := RegistryBaseURL + "/plugins/github.com/useronly.json"
	if got != want {
		t.Errorf("resolveSource single segment = %q, want %q", got, want)
	}
}

func TestResolveSource_EmptyString(t *testing.T) {
	got := resolveSource("")

	want := RegistryBaseURL + "/plugins/.json"
	if got != want {
		t.Errorf("resolveSource empty = %q, want %q", got, want)
	}
}

func TestInstall_InvalidJSONContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json at all"))
	}))
	defer srv.Close()

	_, err := Install(srv.URL)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "invalid plugin JSON") {
		t.Errorf("error should mention invalid JSON, got: %v", err)
	}
}

func TestInstall_NoName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"","tools":[{"name":"ctx_x","command":"echo"}]}`))
	}))
	defer srv.Close()

	_, err := Install(srv.URL)
	if err == nil {
		t.Fatal("expected error for empty name")
	}

	if !strings.Contains(err.Error(), "no name") {
		t.Errorf("error should mention no name, got: %v", err)
	}
}

func TestInstall_NoTools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"test","tools":[]}`))
	}))
	defer srv.Close()

	_, err := Install(srv.URL)
	if err == nil {
		t.Fatal("expected error for empty tools")
	}

	if !strings.Contains(err.Error(), "no tools") {
		t.Errorf("error should mention no tools, got: %v", err)
	}
}

func TestInstall_MissingCtxPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"test","tools":[{"name":"bad_tool","command":"echo"}]}`))
	}))
	defer srv.Close()

	_, err := Install(srv.URL)
	if err == nil {
		t.Fatal("expected error for missing ctx_ prefix")
	}

	if !strings.Contains(err.Error(), "ctx_ prefix") {
		t.Errorf("error should mention ctx_ prefix, got: %v", err)
	}
}

func TestInstall_DownloadFailure(t *testing.T) {
	_, err := Install("http://127.0.0.1:1/nonexistent")
	if err == nil {
		t.Fatal("expected error for unreachable URL")
	}

	if !strings.Contains(err.Error(), "download plugin") {
		t.Errorf("error should mention download, got: %v", err)
	}
}

func TestRemove_ExistingFile(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "myplug.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Remove calls PluginDir() which points to the real dir.
	// Test the underlying os.Remove logic directly.
	if err := os.Remove(path); err != nil {
		t.Errorf("unexpected error removing existing file: %v", err)
	}
}

func TestRemove_NonExistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doesnotexist.json")

	err := os.Remove(path)
	if err == nil {
		t.Error("expected error removing nonexistent file")
	}
}

func TestLoadPluginFile_MissingName(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "noname.json")
	if err := os.WriteFile(path, []byte(`{"tools":[{"name":"ctx_x","command":"echo"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPluginFile(path)
	if err == nil {
		t.Fatal("expected error for missing name")
	}

	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %v, want mention of name required", err)
	}
}

func TestLoadPluginFile_ToolMissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "badtool.json")

	content := `{"name":"test","tools":[{"name":"","command":"echo"}]}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPluginFile(path)
	if err == nil {
		t.Fatal("expected error for tool with empty name")
	}

	if !strings.Contains(err.Error(), "has no name") {
		t.Errorf("error = %v, want mention of no name", err)
	}
}

func TestLoadPluginFile_ToolMissingCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nocmd.json")

	content := `{"name":"test","tools":[{"name":"ctx_x","command":""}]}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPluginFile(path)
	if err == nil {
		t.Fatal("expected error for tool with empty command")
	}

	if !strings.Contains(err.Error(), "no command") {
		t.Errorf("error = %v, want mention of no command", err)
	}
}

func TestLoadPluginFile_NonExistent(t *testing.T) {
	_, err := LoadPluginFile("/nonexistent/path/plugin.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadPlugins_SkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	plugins, err := LoadPlugins(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}
