package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// helper: write a plugin JSON to the given directory.
func writePluginJSON(t *testing.T, dir string, p PluginDef) {
	t.Helper()

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, p.Name+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// helper: create a script file that echoes the given text and return the
// command string to invoke it. On Windows it creates a .cmd file; on
// Unix a .sh file.
func createEchoScript(t *testing.T, dir, name, text string) string {
	t.Helper()

	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, name+".cmd")

		content := "@echo off\r\necho " + text + "\r\n"
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}

		return path
	}

	path := filepath.Join(dir, name+".sh")

	content := "#!/bin/sh\necho " + text + "\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	return "bash " + path
}

// helper: create a script that exits with a non-zero code.
func createFailScript(t *testing.T, dir, name string, code int) string {
	t.Helper()

	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, name+".cmd")

		content := "@echo off\r\necho failing >&2\r\nexit /b " + itoa(code) + "\r\n"
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}

		return path
	}

	path := filepath.Join(dir, name+".sh")

	content := "#!/bin/sh\necho failing >&2\nexit " + itoa(code) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	return "bash " + path
}

// helper: create a script that writes to both stdout and stderr.
func createStdoutStderrScript(t *testing.T, dir, name, stdoutText, stderrText string) string {
	t.Helper()

	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, name+".cmd")

		content := "@echo off\r\necho " + stdoutText + "\r\necho " + stderrText + " >&2\r\n"
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}

		return path
	}

	path := filepath.Join(dir, name+".sh")

	content := "#!/bin/sh\necho " + stdoutText + "\necho " + stderrText + " >&2\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	return "bash " + path
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

// --- Integration Tests ---

func TestIntegration_PreToolUse_OutputCapture(t *testing.T) {
	pluginDir := t.TempDir()
	scriptDir := t.TempDir()

	cmd := createEchoScript(t, scriptDir, "pre-check", "BLOCK_TOOL")

	writePluginJSON(t, pluginDir, PluginDef{
		Name:    "pre-tool-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_pre", Description: "test", Command: "echo noop"}},
		Hooks: map[string][]PluginHook{
			"PreToolUse": {
				{Matcher: "Bash|Write", Command: cmd},
			},
		},
	})

	runner := NewHookRunner(pluginDir)

	// Match: tool name is "Bash"
	results := runner("PreToolUse", "Bash")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Err != nil {
		t.Errorf("unexpected error: %v", results[0].Err)
	}

	if !strings.Contains(results[0].Stdout, "BLOCK_TOOL") {
		t.Errorf("expected stdout to contain BLOCK_TOOL, got %q", results[0].Stdout)
	}

	if results[0].Plugin != "pre-tool-plugin" {
		t.Errorf("expected plugin name pre-tool-plugin, got %s", results[0].Plugin)
	}

	// Match: tool name is "Write"
	results = runner("PreToolUse", "Write")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for Write, got %d", len(results))
	}

	if !strings.Contains(results[0].Stdout, "BLOCK_TOOL") {
		t.Errorf("expected stdout to contain BLOCK_TOOL, got %q", results[0].Stdout)
	}

	// No match: tool name is "Read"
	results = runner("PreToolUse", "Read")
	if len(results) != 0 {
		t.Errorf("expected 0 results for Read, got %d", len(results))
	}
}

func TestIntegration_PostToolUse_Execution(t *testing.T) {
	pluginDir := t.TempDir()
	scriptDir := t.TempDir()

	cmd := createStdoutStderrScript(t, scriptDir, "post-hook", "POST_OK", "POST_WARN")

	writePluginJSON(t, pluginDir, PluginDef{
		Name:    "post-tool-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_post", Description: "test", Command: "echo noop"}},
		Hooks: map[string][]PluginHook{
			"PostToolUse": {
				{Matcher: "Edit", Command: cmd},
			},
		},
	})

	runner := NewHookRunner(pluginDir)

	results := runner("PostToolUse", "Edit")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Err != nil {
		t.Errorf("unexpected error: %v", results[0].Err)
	}

	if !strings.Contains(results[0].Stdout, "POST_OK") {
		t.Errorf("expected stdout to contain POST_OK, got %q", results[0].Stdout)
	}

	if !strings.Contains(results[0].Stderr, "POST_WARN") {
		t.Errorf("expected stderr to contain POST_WARN, got %q", results[0].Stderr)
	}
}

func TestIntegration_SessionStart_Execution(t *testing.T) {
	pluginDir := t.TempDir()
	scriptDir := t.TempDir()

	cmd := createEchoScript(t, scriptDir, "session-init", "SESSION_READY")

	writePluginJSON(t, pluginDir, PluginDef{
		Name:    "session-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_sess", Description: "test", Command: "echo noop"}},
		Hooks: map[string][]PluginHook{
			"SessionStart": {
				{Command: cmd},
			},
		},
	})

	runner := NewHookRunner(pluginDir)

	results := runner("SessionStart", "")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Err != nil {
		t.Errorf("unexpected error: %v", results[0].Err)
	}

	if !strings.Contains(results[0].Stdout, "SESSION_READY") {
		t.Errorf("expected stdout to contain SESSION_READY, got %q", results[0].Stdout)
	}
}

func TestIntegration_PreCompact_Execution(t *testing.T) {
	pluginDir := t.TempDir()
	scriptDir := t.TempDir()

	cmd := createEchoScript(t, scriptDir, "compact-hook", "COMPACTING")

	writePluginJSON(t, pluginDir, PluginDef{
		Name:    "compact-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_compact", Description: "test", Command: "echo noop"}},
		Hooks: map[string][]PluginHook{
			"PreCompact": {
				{Command: cmd},
			},
		},
	})

	runner := NewHookRunner(pluginDir)

	results := runner("PreCompact", "")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Err != nil {
		t.Errorf("unexpected error: %v", results[0].Err)
	}

	if !strings.Contains(results[0].Stdout, "COMPACTING") {
		t.Errorf("expected stdout to contain COMPACTING, got %q", results[0].Stdout)
	}
}

func TestIntegration_HookFailure_NonZeroExit(t *testing.T) {
	pluginDir := t.TempDir()
	scriptDir := t.TempDir()

	cmd := createFailScript(t, scriptDir, "fail-hook", 1)

	writePluginJSON(t, pluginDir, PluginDef{
		Name:    "fail-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_fail", Description: "test", Command: "echo noop"}},
		Hooks: map[string][]PluginHook{
			"PreToolUse": {
				{Matcher: "Bash", Command: cmd},
			},
		},
	})

	runner := NewHookRunner(pluginDir)

	results := runner("PreToolUse", "Bash")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Err == nil {
		t.Error("expected non-nil error for non-zero exit")
	}

	if !strings.Contains(results[0].Stderr, "failing") {
		t.Errorf("expected stderr to contain 'failing', got %q", results[0].Stderr)
	}
}

func TestIntegration_HookFailure_MissingCommand(t *testing.T) {
	pluginDir := t.TempDir()

	// Reference a command that does not exist.
	missingCmd := filepath.Join(t.TempDir(), "nonexistent-cmd-that-does-not-exist")

	writePluginJSON(t, pluginDir, PluginDef{
		Name:    "missing-cmd-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_miss", Description: "test", Command: "echo noop"}},
		Hooks: map[string][]PluginHook{
			"SessionStart": {
				{Command: missingCmd},
			},
		},
	})

	runner := NewHookRunner(pluginDir)

	results := runner("SessionStart", "")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Err == nil {
		t.Error("expected non-nil error for missing command")
	}
}

func TestIntegration_MultiplePlugins_ExecutionOrder(t *testing.T) {
	pluginDir := t.TempDir()
	scriptDir := t.TempDir()

	// Create 3 plugins, each with a PreToolUse hook that outputs the plugin name.
	pluginNames := []string{"alpha-plugin", "beta-plugin", "gamma-plugin"}

	for _, name := range pluginNames {
		cmd := createEchoScript(t, scriptDir, name, name)
		writePluginJSON(t, pluginDir, PluginDef{
			Name:    name,
			Version: "1.0.0",
			Tools:   []ToolDef{{Name: "ctx_" + strings.ReplaceAll(name, "-", "_"), Description: "test", Command: "echo noop"}},
			Hooks: map[string][]PluginHook{
				"PreToolUse": {
					{Matcher: "Bash", Command: cmd},
				},
			},
		})
	}

	runner := NewHookRunner(pluginDir)

	results := runner("PreToolUse", "Bash")
	if len(results) != 3 {
		t.Fatalf("expected 3 results from 3 plugins, got %d", len(results))
	}

	// Verify all plugins ran and each produced expected output.
	seen := make(map[string]bool)

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error from plugin %s: %v", r.Plugin, r.Err)
		}

		seen[r.Plugin] = true
		// Stdout should contain the plugin name.
		if !strings.Contains(r.Stdout, r.Plugin) {
			t.Errorf("expected stdout from %s to contain plugin name, got %q", r.Plugin, r.Stdout)
		}
	}

	for _, name := range pluginNames {
		if !seen[name] {
			t.Errorf("plugin %s did not produce a result", name)
		}
	}
}

func TestIntegration_MultiplePlugins_MixedEvents(t *testing.T) {
	pluginDir := t.TempDir()
	scriptDir := t.TempDir()

	// Plugin A has SessionStart hook.
	cmdA := createEchoScript(t, scriptDir, "pluginA-session", "A_SESSION")
	writePluginJSON(t, pluginDir, PluginDef{
		Name:    "plugin-a",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_a", Description: "test", Command: "echo noop"}},
		Hooks: map[string][]PluginHook{
			"SessionStart": {{Command: cmdA}},
		},
	})

	// Plugin B has PreToolUse and PostToolUse hooks.
	cmdBPre := createEchoScript(t, scriptDir, "pluginB-pre", "B_PRE")
	cmdBPost := createEchoScript(t, scriptDir, "pluginB-post", "B_POST")
	writePluginJSON(t, pluginDir, PluginDef{
		Name:    "plugin-b",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_b", Description: "test", Command: "echo noop"}},
		Hooks: map[string][]PluginHook{
			"PreToolUse":  {{Matcher: ".*", Command: cmdBPre}},
			"PostToolUse": {{Matcher: ".*", Command: cmdBPost}},
		},
	})

	runner := NewHookRunner(pluginDir)

	// SessionStart should only trigger plugin A's hook.
	results := runner("SessionStart", "")
	if len(results) != 1 {
		t.Fatalf("expected 1 SessionStart result, got %d", len(results))
	}

	if results[0].Plugin != "plugin-a" {
		t.Errorf("expected plugin-a, got %s", results[0].Plugin)
	}

	if !strings.Contains(results[0].Stdout, "A_SESSION") {
		t.Errorf("expected A_SESSION in stdout, got %q", results[0].Stdout)
	}

	// PreToolUse should only trigger plugin B's pre hook.
	results = runner("PreToolUse", "Bash")
	if len(results) != 1 {
		t.Fatalf("expected 1 PreToolUse result, got %d", len(results))
	}

	if results[0].Plugin != "plugin-b" {
		t.Errorf("expected plugin-b, got %s", results[0].Plugin)
	}

	if !strings.Contains(results[0].Stdout, "B_PRE") {
		t.Errorf("expected B_PRE in stdout, got %q", results[0].Stdout)
	}

	// PostToolUse should only trigger plugin B's post hook.
	results = runner("PostToolUse", "Write")
	if len(results) != 1 {
		t.Fatalf("expected 1 PostToolUse result, got %d", len(results))
	}

	if !strings.Contains(results[0].Stdout, "B_POST") {
		t.Errorf("expected B_POST in stdout, got %q", results[0].Stdout)
	}
}

func TestIntegration_MultipleHooksPerEvent(t *testing.T) {
	pluginDir := t.TempDir()
	scriptDir := t.TempDir()

	cmd1 := createEchoScript(t, scriptDir, "hook1", "FIRST")
	cmd2 := createEchoScript(t, scriptDir, "hook2", "SECOND")

	writePluginJSON(t, pluginDir, PluginDef{
		Name:    "multi-hook-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_mh", Description: "test", Command: "echo noop"}},
		Hooks: map[string][]PluginHook{
			"SessionStart": {
				{Command: cmd1},
				{Command: cmd2},
			},
		},
	})

	runner := NewHookRunner(pluginDir)

	results := runner("SessionStart", "")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if !strings.Contains(results[0].Stdout, "FIRST") {
		t.Errorf("expected FIRST in first result stdout, got %q", results[0].Stdout)
	}

	if !strings.Contains(results[1].Stdout, "SECOND") {
		t.Errorf("expected SECOND in second result stdout, got %q", results[1].Stdout)
	}
}

func TestIntegration_HookFailure_ContinuesExecution(t *testing.T) {
	pluginDir := t.TempDir()
	scriptDir := t.TempDir()

	// First hook fails, second should still run.
	failCmd := createFailScript(t, scriptDir, "fail", 1)
	okCmd := createEchoScript(t, scriptDir, "ok", "SUCCESS")

	writePluginJSON(t, pluginDir, PluginDef{
		Name:    "resilient-plugin",
		Version: "1.0.0",
		Tools:   []ToolDef{{Name: "ctx_res", Description: "test", Command: "echo noop"}},
		Hooks: map[string][]PluginHook{
			"PreCompact": {
				{Command: failCmd},
				{Command: okCmd},
			},
		},
	})

	runner := NewHookRunner(pluginDir)

	results := runner("PreCompact", "")
	if len(results) != 2 {
		t.Fatalf("expected 2 results (fail + ok), got %d", len(results))
	}

	if results[0].Err == nil {
		t.Error("expected first hook to fail")
	}

	if results[1].Err != nil {
		t.Errorf("expected second hook to succeed, got error: %v", results[1].Err)
	}

	if !strings.Contains(results[1].Stdout, "SUCCESS") {
		t.Errorf("expected SUCCESS in second result, got %q", results[1].Stdout)
	}
}
