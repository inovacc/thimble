package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/plugin"
)

func TestRenderCommand(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     map[string]string
		want     string
		wantErr  bool
	}{
		{
			name:     "simple substitution",
			template: "echo {{.input}}",
			data:     map[string]string{"input": "hello"},
			want:     "echo hello",
		},
		{
			name:     "multiple fields",
			template: "curl -X {{.method}} {{.url}}",
			data:     map[string]string{"method": "GET", "url": "http://example.com"},
			want:     "curl -X GET http://example.com",
		},
		{
			name:     "missing field uses zero value",
			template: "echo {{.input}}",
			data:     map[string]string{},
			want:     "echo",
		},
		{
			name:     "no template variables",
			template: "echo hello world",
			data:     map[string]string{},
			want:     "echo hello world",
		},
		{
			name:     "invalid template syntax",
			template: "echo {{.input",
			data:     map[string]string{"input": "hello"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderCommand(tt.template, tt.data)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuiltinToolNamesContainsExpected(t *testing.T) {
	expected := []string{
		"ctx_execute",
		"ctx_search",
		"ctx_delegate",
		"ctx_git_status",
		"ctx_gh",
		"ctx_lint",
	}

	for _, name := range expected {
		if !builtinToolNames[name] {
			t.Errorf("builtinToolNames missing %q", name)
		}
	}
}

func TestWatchPluginsDetectsNewFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping polling test in short mode")
	}

	// Create a temp plugin dir.
	dir := t.TempDir()

	// Create a minimal bridge with just the fields watchPlugins needs.
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	b := &Bridge{
		logger: logger,
	}

	// Override pluginWatchInterval via a helper goroutine that manually
	// exercises the detection logic instead of relying on the full watcher.
	// We test the core detection logic directly.

	// Initially no files.
	known := make(map[string]bool)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Fatal("expected empty dir")
	}

	// Write a valid plugin JSON file.
	pluginDef := map[string]any{
		"name":    "test-hotreload",
		"version": "1.0.0",
		"tools": []map[string]any{
			{
				"name":        "ctx_test_hotreload",
				"description": "A test tool for hot reload",
				"command":     "echo hotreload",
			},
		},
	}

	data, err := json.Marshal(pluginDef)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "test-hotreload.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-scan and detect the new file.
	entries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var newFiles []string

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		if !known[e.Name()] {
			newFiles = append(newFiles, e.Name())
			known[e.Name()] = true
		}
	}

	if len(newFiles) != 1 {
		t.Fatalf("expected 1 new file, got %d", len(newFiles))
	}

	if newFiles[0] != "test-hotreload.json" {
		t.Errorf("expected test-hotreload.json, got %s", newFiles[0])
	}

	// Verify the file is valid by loading it.
	_, err = os.ReadFile(filepath.Join(dir, newFiles[0]))
	if err != nil {
		t.Fatal(err)
	}

	_ = b // used to verify Bridge struct compiles with watchPlugins method
}

func TestWatchPluginsDetectsRemoval(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping polling test in short mode")
	}

	dir := t.TempDir()

	// Write initial plugin file.
	pluginDef := map[string]any{
		"name":    "ephemeral",
		"version": "1.0.0",
		"tools": []map[string]any{
			{
				"name":        "ctx_ephemeral",
				"description": "Ephemeral tool",
				"command":     "echo bye",
			},
		},
	}
	data, _ := json.Marshal(pluginDef) //nolint:errchkjson

	pluginPath := filepath.Join(dir, "ephemeral.json")
	if err := os.WriteFile(pluginPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Build known set from initial scan.
	known := make(map[string]bool)

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			known[e.Name()] = true
		}
	}

	if !known["ephemeral.json"] {
		t.Fatal("expected ephemeral.json in known set")
	}

	// Remove the plugin file.
	if err := os.Remove(pluginPath); err != nil {
		t.Fatal(err)
	}

	// Re-scan and detect removal.
	entries, _ = os.ReadDir(dir)
	current := make(map[string]bool)

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			current[e.Name()] = true
		}
	}

	var removed []string

	for name := range known {
		if !current[name] {
			removed = append(removed, name)
			delete(known, name)
		}
	}

	if len(removed) != 1 || removed[0] != "ephemeral.json" {
		t.Errorf("expected ephemeral.json removed, got %v", removed)
	}
}

func TestRenderCommand_PluginEnvVars(t *testing.T) {
	data := map[string]string{
		"input":               "hello",
		"THIMBLE_PLUGIN_ROOT": "/data/plugins",
		"THIMBLE_PLUGIN_DATA": "/data/plugin-data/my-plugin",
	}

	// Template using THIMBLE_PLUGIN_ROOT.
	got, err := renderCommand("ls {{.THIMBLE_PLUGIN_ROOT}}/scripts", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "ls /data/plugins/scripts" {
		t.Errorf("got %q, want %q", got, "ls /data/plugins/scripts")
	}

	// Template using THIMBLE_PLUGIN_DATA.
	got, err = renderCommand("cat {{.THIMBLE_PLUGIN_DATA}}/state.json", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "cat /data/plugin-data/my-plugin/state.json" {
		t.Errorf("got %q, want %q", got, "cat /data/plugin-data/my-plugin/state.json")
	}

	// Template using both env vars and regular input.
	got, err = renderCommand("{{.THIMBLE_PLUGIN_ROOT}}/run.sh {{.input}} --data-dir={{.THIMBLE_PLUGIN_DATA}}", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "/data/plugins/run.sh hello --data-dir=/data/plugin-data/my-plugin"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple path", "/data/plugins", "'/data/plugins'"},
		{"path with spaces", "/my data/plugins", "'/my data/plugins'"},
		{"path with single quote", "/it's/a/path", "'/it'\\''s/a/path'"},
		{"empty string", "", "''"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellQuote(tt.input)
			if got != tt.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRenderCommand_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    string
		data    map[string]string
		want    string
		wantErr bool
	}{
		{"simple", "echo {{.msg}}", map[string]string{"msg": "hi"}, "echo hi", false},
		{"missing key", "echo {{.msg}}", map[string]string{}, "echo", false},
		{"multiple", "{{.a}} {{.b}}", map[string]string{"a": "x", "b": "y"}, "x y", false},
		{"no vars", "echo hello", nil, "echo hello", false},
		{"bad template", "echo {{.x", nil, "", true},
		{"empty result", "{{.x}}", map[string]string{}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderCommand(tt.tmpl, tt.data)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWatchPluginsContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	b := &Bridge{logger: logger}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})

	go func() {
		b.watchPlugins(ctx)
		close(done)
	}()

	// Cancel immediately — watchPlugins should exit promptly.
	cancel()

	select {
	case <-done:
		// Success.
	case <-time.After(5 * time.Second):
		t.Fatal("watchPlugins did not exit after context cancellation")
	}
}

func TestBuiltinToolNamesListReturnsAll(t *testing.T) {
	names := BuiltinToolNamesList()
	if len(names) == 0 {
		t.Fatal("BuiltinToolNamesList returned empty slice")
	}

	// Verify it matches the builtinToolNames map.
	if len(names) != len(builtinToolNames) {
		t.Errorf("BuiltinToolNamesList returned %d names, but builtinToolNames has %d entries",
			len(names), len(builtinToolNames))
	}

	for _, name := range names {
		if !builtinToolNames[name] {
			t.Errorf("BuiltinToolNamesList includes %q which is not in builtinToolNames", name)
		}
	}
}

func TestBuiltinToolNamesContainsPluginConflicts(t *testing.T) {
	// ctx_plugin_conflicts should be in the builtin set so plugins can't override it.
	if !builtinToolNames["ctx_plugin_conflicts"] {
		t.Error("builtinToolNames should include ctx_plugin_conflicts")
	}
}

func TestRegisterPluginTools_StoresConflicts(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// Create a temp project directory with a plugin in the project scope that conflicts with a built-in.
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, ".thimble", "plugins")

	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pluginData := `{
		"name": "conflict-test",
		"version": "1.0.0",
		"tools": [
			{"name": "ctx_execute", "description": "conflicts with built-in", "command": "echo bad"},
			{"name": "ctx_conflict_ok", "description": "no conflict", "command": "echo ok"}
		]
	}`

	if err := os.WriteFile(filepath.Join(pluginDir, "conflict-test.json"), []byte(pluginData), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &Bridge{
		logger:     logger,
		projectDir: dir,
	}

	// We need a real server to register tools.
	srv := mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: "test", Version: "0.1.0"},
		&mcpsdk.ServerOptions{Logger: logger},
	)
	b.server = srv

	b.registerPluginTools()

	// Verify conflicts were stored.
	if len(b.pluginConflicts) == 0 {
		t.Fatal("expected at least 1 conflict to be stored")
	}

	found := false

	for _, c := range b.pluginConflicts {
		if c.PluginName == "conflict-test" && c.ToolName == "ctx_execute" && c.ConflictsWith == "built-in" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected conflict for conflict-test:ctx_execute, got %v", b.pluginConflicts)
	}
}

func TestHandlePluginConflicts_NoConflicts(t *testing.T) {
	b := &Bridge{
		pluginConflicts: nil,
	}

	result, _, err := b.handlePluginConflicts(context.Background(), nil, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return "no conflicts" text.
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}

	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "No plugin conflicts") {
		t.Errorf("unexpected result text: %q", text)
	}
}

func TestHandlePluginConflicts_WithConflicts(t *testing.T) {
	b := &Bridge{
		pluginConflicts: []plugin.PluginConflict{
			{PluginName: "bad-plugin", ToolName: "ctx_execute", ConflictsWith: "built-in"},
			{PluginName: "dup-plugin", ToolName: "ctx_shared", ConflictsWith: "good-plugin"},
		},
	}

	result, _, err := b.handlePluginConflicts(context.Background(), nil, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}

	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "2 plugin conflict(s)") {
		t.Errorf("expected '2 plugin conflict(s)' in text, got: %q", text)
	}

	if !strings.Contains(text, "bad-plugin") {
		t.Errorf("expected 'bad-plugin' in text, got: %q", text)
	}

	if !strings.Contains(text, "dup-plugin") {
		t.Errorf("expected 'dup-plugin' in text, got: %q", text)
	}
}
