package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNewHookRunner_NoPlugins(t *testing.T) {
	dir := t.TempDir()
	runner := NewHookRunner(dir)

	results := runner("PreToolUse", "Bash")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestNewHookRunner_NonExistentDir(t *testing.T) {
	runner := NewHookRunner(filepath.Join(t.TempDir(), "nonexistent"))

	results := runner("SessionStart", "")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestNewHookRunner_MatchingHook(t *testing.T) {
	dir := t.TempDir()

	echoCmd := "echo hello"
	if runtime.GOOS == "windows" {
		echoCmd = "echo hello"
	}

	p := PluginDef{
		Name:    "test-hooks",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_test", Description: "test", Command: "echo test"}},
		Hooks: map[string][]PluginHook{
			"PostToolUse": {
				{Matcher: "Write|Edit", Command: echoCmd},
			},
		},
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "test-hooks.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	runner := NewHookRunner(dir)

	// Should match "Write".
	results := runner("PostToolUse", "Write")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for Write, got %d", len(results))
	}

	if results[0].Err != nil {
		t.Errorf("unexpected error: %v", results[0].Err)
	}

	if results[0].Plugin != "test-hooks" {
		t.Errorf("expected plugin test-hooks, got %s", results[0].Plugin)
	}
}

func TestNewHookRunner_NonMatchingHook(t *testing.T) {
	dir := t.TempDir()

	p := PluginDef{
		Name:    "test-hooks",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_test", Description: "test", Command: "echo test"}},
		Hooks: map[string][]PluginHook{
			"PostToolUse": {
				{Matcher: "Write|Edit", Command: "echo hello"},
			},
		},
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "test-hooks.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	runner := NewHookRunner(dir)

	// Should NOT match "Read".
	results := runner("PostToolUse", "Read")
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-matching tool, got %d", len(results))
	}
}

func TestNewHookRunner_SessionStartHook(t *testing.T) {
	dir := t.TempDir()

	echoCmd := "echo started"
	if runtime.GOOS == "windows" {
		echoCmd = "echo started"
	}

	p := PluginDef{
		Name:    "startup-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_start", Description: "test", Command: "echo test"}},
		Hooks: map[string][]PluginHook{
			"SessionStart": {
				{Command: echoCmd},
			},
		},
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "startup-plugin.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	runner := NewHookRunner(dir)

	results := runner("SessionStart", "")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for SessionStart, got %d", len(results))
	}

	if results[0].Err != nil {
		t.Errorf("unexpected error: %v", results[0].Err)
	}
}

func TestNewHookRunner_WrongEvent(t *testing.T) {
	dir := t.TempDir()

	p := PluginDef{
		Name:    "event-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_ev", Description: "test", Command: "echo test"}},
		Hooks: map[string][]PluginHook{
			"PreToolUse": {
				{Command: "echo pre"},
			},
		},
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "event-plugin.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	runner := NewHookRunner(dir)

	// SessionStart should not trigger PreToolUse hooks.
	results := runner("SessionStart", "")
	if len(results) != 0 {
		t.Errorf("expected 0 results for wrong event, got %d", len(results))
	}
}

func TestNewHookRunner_MatcherSkippedWhenNoToolName(t *testing.T) {
	dir := t.TempDir()

	p := PluginDef{
		Name:    "matcher-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_m", Description: "test", Command: "echo test"}},
		Hooks: map[string][]PluginHook{
			"SessionStart": {
				{Matcher: "Bash", Command: "echo should-skip"},
				{Command: "echo should-run"},
			},
		},
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "matcher-plugin.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	runner := NewHookRunner(dir)

	// With no tool name, matcher hook should be skipped but command-only hook should run.
	results := runner("SessionStart", "")
	if len(results) != 1 {
		t.Fatalf("expected 1 result (matcher skipped), got %d", len(results))
	}
}

func TestNewHookRunner_MultiplePlugins(t *testing.T) {
	dir := t.TempDir()

	echoCmd := "echo ok"

	for _, name := range []string{"plugin-a", "plugin-b"} {
		p := PluginDef{
			Name:    name,
			Version: "1.0.0",
			Tools:   []ToolDef{{Name: "ctx_" + name, Description: "test", Command: "echo test"}},
			Hooks: map[string][]PluginHook{
				"PreCompact": {{Command: echoCmd}},
			},
		}

		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	runner := NewHookRunner(dir)

	results := runner("PreCompact", "")
	if len(results) != 2 {
		t.Errorf("expected 2 results from 2 plugins, got %d", len(results))
	}
}

func TestLoadPluginFile_WithValidHooks(t *testing.T) {
	dir := t.TempDir()

	content := `{
		"name": "hook-plugin",
		"version": "1.0.0",
		"tools": [{"name": "ctx_hp", "description": "test", "command": "echo"}],
		"hooks": {
			"PostToolUse": [{"matcher": "Write", "command": "fmt"}],
			"SessionStart": [{"command": "echo loaded"}]
		}
	}`

	path := filepath.Join(dir, "hook-plugin.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPluginFile(path)
	if err != nil {
		t.Fatalf("LoadPluginFile: %v", err)
	}

	if len(p.Hooks) != 2 {
		t.Errorf("expected 2 hook events, got %d", len(p.Hooks))
	}

	if len(p.Hooks["PostToolUse"]) != 1 {
		t.Errorf("expected 1 PostToolUse hook, got %d", len(p.Hooks["PostToolUse"]))
	}

	if p.Hooks["PostToolUse"][0].Matcher != "Write" {
		t.Errorf("expected matcher Write, got %s", p.Hooks["PostToolUse"][0].Matcher)
	}
}

func TestLoadPluginFile_InvalidHookEvent(t *testing.T) {
	dir := t.TempDir()

	content := `{
		"name": "bad-hook",
		"version": "1.0.0",
		"tools": [{"name": "ctx_bh", "description": "test", "command": "echo"}],
		"hooks": {"InvalidEvent": [{"command": "echo"}]}
	}`

	path := filepath.Join(dir, "bad-hook.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPluginFile(path)
	if err == nil {
		t.Fatal("expected error for invalid hook event")
	}
}

func TestLoadPluginFile_HookEmptyCommand(t *testing.T) {
	dir := t.TempDir()

	content := `{
		"name": "empty-cmd-hook",
		"version": "1.0.0",
		"tools": [{"name": "ctx_ec", "description": "test", "command": "echo"}],
		"hooks": {"PreToolUse": [{"matcher": "Bash"}]}
	}`

	path := filepath.Join(dir, "empty-cmd-hook.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPluginFile(path)
	if err == nil {
		t.Fatal("expected error for hook with empty command")
	}
}

func TestLoadPluginFile_HookInvalidRegex(t *testing.T) {
	dir := t.TempDir()

	content := `{
		"name": "bad-regex-hook",
		"version": "1.0.0",
		"tools": [{"name": "ctx_br", "description": "test", "command": "echo"}],
		"hooks": {"PreToolUse": [{"matcher": "[invalid", "command": "echo"}]}
	}`

	path := filepath.Join(dir, "bad-regex-hook.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPluginFile(path)
	if err == nil {
		t.Fatal("expected error for invalid matcher regex")
	}
}
