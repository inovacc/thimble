package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeCodeCapabilities(t *testing.T) {
	adapter, _ := Get(PlatformClaudeCode)
	caps := adapter.Capabilities()

	assertCap(t, "PreToolUse", caps.PreToolUse, true)
	assertCap(t, "PostToolUse", caps.PostToolUse, true)
	assertCap(t, "PreCompact", caps.PreCompact, true)
	assertCap(t, "SessionStart", caps.SessionStart, true)
	assertCap(t, "CanModifyArgs", caps.CanModifyArgs, false)
	assertCap(t, "CanModifyOutput", caps.CanModifyOutput, true)
	assertCap(t, "CanInjectSessionContext", caps.CanInjectSessionContext, true)
}

func TestGeminiCLICapabilities(t *testing.T) {
	adapter, _ := Get(PlatformGeminiCLI)
	caps := adapter.Capabilities()

	assertCap(t, "PreToolUse", caps.PreToolUse, true)
	assertCap(t, "PostToolUse", caps.PostToolUse, true)
	assertCap(t, "PreCompact", caps.PreCompact, true)
	assertCap(t, "SessionStart", caps.SessionStart, true)
	assertCap(t, "CanModifyArgs", caps.CanModifyArgs, false)
	assertCap(t, "CanModifyOutput", caps.CanModifyOutput, false)
	assertCap(t, "CanInjectSessionContext", caps.CanInjectSessionContext, true)
}

func TestVSCodeCopilotCapabilities(t *testing.T) {
	adapter, _ := Get(PlatformVSCodeCopilot)
	caps := adapter.Capabilities()

	assertCap(t, "PreToolUse", caps.PreToolUse, true)
	assertCap(t, "PostToolUse", caps.PostToolUse, true)
	assertCap(t, "PreCompact", caps.PreCompact, true)
	assertCap(t, "SessionStart", caps.SessionStart, true)
	assertCap(t, "CanModifyArgs", caps.CanModifyArgs, false)
	assertCap(t, "CanModifyOutput", caps.CanModifyOutput, false)
	assertCap(t, "CanInjectSessionContext", caps.CanInjectSessionContext, false)
}

func TestCursorCapabilities(t *testing.T) {
	adapter, _ := Get(PlatformCursor)
	caps := adapter.Capabilities()

	assertCap(t, "PreToolUse", caps.PreToolUse, true)
	assertCap(t, "PostToolUse", caps.PostToolUse, true)
	assertCap(t, "PreCompact", caps.PreCompact, false)
	assertCap(t, "SessionStart", caps.SessionStart, false)
	assertCap(t, "CanModifyArgs", caps.CanModifyArgs, false)
	assertCap(t, "CanModifyOutput", caps.CanModifyOutput, false)
	assertCap(t, "CanInjectSessionContext", caps.CanInjectSessionContext, false)
}

func TestOpenCodeCapabilities(t *testing.T) {
	adapter, _ := Get(PlatformOpenCode)
	caps := adapter.Capabilities()

	assertCap(t, "PreToolUse", caps.PreToolUse, true)
	assertCap(t, "PostToolUse", caps.PostToolUse, true)
	assertCap(t, "PreCompact", caps.PreCompact, true)
	assertCap(t, "SessionStart", caps.SessionStart, false)
	assertCap(t, "CanModifyArgs", caps.CanModifyArgs, true)
	assertCap(t, "CanModifyOutput", caps.CanModifyOutput, true)
	assertCap(t, "CanInjectSessionContext", caps.CanInjectSessionContext, false)
}

func TestCodexAllCapabilitiesFalse(t *testing.T) {
	adapter, _ := Get(PlatformCodex)
	caps := adapter.Capabilities()

	assertCap(t, "PreToolUse", caps.PreToolUse, false)
	assertCap(t, "PostToolUse", caps.PostToolUse, false)
	assertCap(t, "PreCompact", caps.PreCompact, false)
	assertCap(t, "SessionStart", caps.SessionStart, false)
	assertCap(t, "CanModifyArgs", caps.CanModifyArgs, false)
	assertCap(t, "CanModifyOutput", caps.CanModifyOutput, false)
	assertCap(t, "CanInjectSessionContext", caps.CanInjectSessionContext, false)
}

func TestOpenCodeCanModifyArgs(t *testing.T) {
	adapter, _ := Get(PlatformOpenCode)
	caps := adapter.Capabilities()

	if !caps.CanModifyArgs {
		t.Error("OpenCode should have CanModifyArgs=true")
	}
}

func TestValidateHooksSettingsFileMissing(t *testing.T) {
	// Use the common validator with a non-existent path to test MISSING diagnostics.
	caps := Capabilities{
		PreToolUse:   true,
		PostToolUse:  true,
		PreCompact:   true,
		SessionStart: true,
	}
	hookKeys := map[string]string{
		"PreToolUse":   "PreToolUse",
		"PostToolUse":  "PostToolUse",
		"PreCompact":   "PreCompact",
		"SessionStart": "SessionStart",
	}
	diags := validateHooksCommon(filepath.Join(t.TempDir(), "nonexistent.json"), caps, hookKeys)

	if len(diags) == 0 {
		t.Fatal("expected diagnostics, got none")
	}

	if diags[0] != "settings file exists: MISSING" {
		t.Errorf("first diag = %q, want %q", diags[0], "settings file exists: MISSING")
	}
	// All hook entries should also be MISSING.
	for _, d := range diags[1:] {
		if !hasSuffix(d, "MISSING") {
			t.Errorf("expected MISSING diagnostic, got %q", d)
		}
	}
}

func TestValidateHooksSettingsFileExists(t *testing.T) {
	// Create a temp settings file with some hook entries.
	tmpDir := t.TempDir()
	settingsFile := filepath.Join(tmpDir, "settings.json")

	content := `{
		"hooks": {
			"PreToolUse": [{"command": "thimble hook pre-tool-use"}],
			"PostToolUse": [{"command": "thimble hook post-tool-use"}]
		}
	}`
	if err := os.WriteFile(settingsFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Use the common validator directly to test with our temp path.
	caps := Capabilities{
		PreToolUse:   true,
		PostToolUse:  true,
		PreCompact:   true,
		SessionStart: true,
	}
	hookKeys := map[string]string{
		"PreToolUse":   "PreToolUse",
		"PostToolUse":  "PostToolUse",
		"PreCompact":   "PreCompact",
		"SessionStart": "SessionStart",
	}
	diags := validateHooksCommon(settingsFile, caps, hookKeys)

	if diags[0] != "settings file exists: OK" {
		t.Errorf("first diag = %q, want %q", diags[0], "settings file exists: OK")
	}

	// PreToolUse and PostToolUse should be OK, PreCompact and SessionStart MISSING.
	expected := map[string]string{
		"PreToolUse hook configured":   "OK",
		"PostToolUse hook configured":  "OK",
		"PreCompact hook configured":   "MISSING",
		"SessionStart hook configured": "MISSING",
	}

	for _, d := range diags[1:] {
		found := false

		for prefix, suffix := range expected {
			if hasPrefix(d, prefix) {
				if !hasSuffix(d, suffix) {
					t.Errorf("diag %q should end with %q", d, suffix)
				}

				found = true

				break
			}
		}

		if !found {
			t.Errorf("unexpected diagnostic: %q", d)
		}
	}
}

func TestCodexValidateHooksNoOp(t *testing.T) {
	adapter, _ := Get(PlatformCodex)
	diags := adapter.ValidateHooks()

	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for Codex, got %d", len(diags))
	}

	if !hasPrefix(diags[0], "settings file exists: N/A") {
		t.Errorf("diag = %q, want N/A prefix", diags[0])
	}
}

func assertCap(t *testing.T, name string, got, want bool) {
	t.Helper()

	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
