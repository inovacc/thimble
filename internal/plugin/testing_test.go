package plugin

import (
	"fmt"
	"testing"
)

func TestTestPlugin_ValidPlugin(t *testing.T) {
	t.Parallel()

	def := &PluginDef{
		Name:    "test-plugin",
		Version: "1.0.0",
		Tools: []ToolDef{
			{
				Name:        "ctx_test_echo",
				Description: "echo test",
				Command:     "echo hello",
			},
		},
	}

	results := TestPlugin(def, nil)

	failures := countFailures(results)
	if failures > 0 {
		for _, r := range results {
			if r.Status == "fail" {
				t.Errorf("unexpected failure: check=%s tool=%s error=%s", r.Check, r.ToolName, r.Error)
			}
		}
	}
}

func TestTestPlugin_MissingName(t *testing.T) {
	t.Parallel()

	def := &PluginDef{
		Version: "1.0.0",
		Tools: []ToolDef{
			{Name: "ctx_foo", Description: "foo", Command: "echo foo"},
		},
	}

	results := TestPlugin(def, nil)
	assertCheckFailed(t, results, "schema:name")
}

func TestTestPlugin_MissingVersion(t *testing.T) {
	t.Parallel()

	def := &PluginDef{
		Name: "test",
		Tools: []ToolDef{
			{Name: "ctx_foo", Description: "foo", Command: "echo foo"},
		},
	}

	results := TestPlugin(def, nil)
	assertCheckFailed(t, results, "schema:version")
}

func TestTestPlugin_NoTools(t *testing.T) {
	t.Parallel()

	def := &PluginDef{
		Name:    "test",
		Version: "1.0.0",
		Tools:   nil,
	}

	results := TestPlugin(def, nil)
	assertCheckFailed(t, results, "schema:tools")
}

func TestTestPlugin_MissingCtxPrefix(t *testing.T) {
	t.Parallel()

	def := &PluginDef{
		Name:    "test",
		Version: "1.0.0",
		Tools: []ToolDef{
			{Name: "no_prefix", Description: "bad", Command: "echo bad"},
		},
	}

	results := TestPlugin(def, nil)
	assertCheckFailed(t, results, "prefix:ctx_")
}

func TestTestPlugin_CommandNotOnPath(t *testing.T) {
	// Not parallel: mutates package-level lookPathFunc.

	// Override lookPathFunc to simulate missing binary.
	origLookPath := lookPathFunc
	lookPathFunc = func(name string) (string, error) {
		return "", fmt.Errorf("not found: %s", name)
	}

	t.Cleanup(func() { lookPathFunc = origLookPath })

	def := &PluginDef{
		Name:    "test",
		Version: "1.0.0",
		Tools: []ToolDef{
			{Name: "ctx_missing", Description: "missing cmd", Command: "nonexistent_binary_xyz --flag"},
		},
	}

	results := TestPlugin(def, nil)
	assertCheckFailed(t, results, "command:exists")
}

func TestTestPlugin_CommandOnPath(t *testing.T) {
	// Not parallel: mutates package-level lookPathFunc.

	// Override lookPathFunc to simulate found binary.
	origLookPath := lookPathFunc
	lookPathFunc = func(name string) (string, error) {
		return "/usr/bin/" + name, nil
	}

	t.Cleanup(func() { lookPathFunc = origLookPath })

	def := &PluginDef{
		Name:    "test",
		Version: "1.0.0",
		Tools: []ToolDef{
			{Name: "ctx_found", Description: "found cmd", Command: "mycmd --flag"},
		},
	}

	results := TestPlugin(def, nil)
	assertCheckPassed(t, results, "command:exists")
}

func TestTestPlugin_DependencyMissing(t *testing.T) {
	t.Parallel()

	def := &PluginDef{
		Name:    "test",
		Version: "1.0.0",
		Tools: []ToolDef{
			{Name: "ctx_foo", Description: "foo", Command: "echo foo"},
		},
		Dependencies: []PluginDependency{
			{Name: "missing-dep", Version: ">=1.0.0"},
		},
	}

	// Empty registry — dependency won't be found.
	results := TestPlugin(def, nil)
	assertCheckFailed(t, results, "dependency:missing-dep")
}

func TestTestPlugin_DependencyFound(t *testing.T) {
	t.Parallel()

	def := &PluginDef{
		Name:    "test",
		Version: "1.0.0",
		Tools: []ToolDef{
			{Name: "ctx_foo", Description: "foo", Command: "echo foo"},
		},
		Dependencies: []PluginDependency{
			{Name: "dep-a", Version: ">=1.0.0"},
		},
	}

	registry := []RegistryEntry{
		{Name: "dep-a", Version: "1.2.0"},
	}

	results := TestPlugin(def, registry)
	assertCheckPassed(t, results, "dependency:dep-a")
}

func TestTestPlugin_DependencyVersionMismatch(t *testing.T) {
	t.Parallel()

	def := &PluginDef{
		Name:    "test",
		Version: "1.0.0",
		Tools: []ToolDef{
			{Name: "ctx_foo", Description: "foo", Command: "echo foo"},
		},
		Dependencies: []PluginDependency{
			{Name: "dep-a", Version: ">=2.0.0"},
		},
	}

	registry := []RegistryEntry{
		{Name: "dep-a", Version: "1.5.0"},
	}

	results := TestPlugin(def, registry)
	assertCheckFailed(t, results, "dependency:dep-a")
}

func TestTestPlugin_OptionalDependencyMissing(t *testing.T) {
	t.Parallel()

	def := &PluginDef{
		Name:    "test",
		Version: "1.0.0",
		Tools: []ToolDef{
			{Name: "ctx_foo", Description: "foo", Command: "echo foo"},
		},
		Dependencies: []PluginDependency{
			{Name: "optional-dep", Optional: true},
		},
	}

	results := TestPlugin(def, nil)
	// Optional missing dependency should pass, not fail.
	assertCheckPassed(t, results, "dependency:optional-dep")
}

func TestHasFailures(t *testing.T) {
	t.Parallel()

	pass := []TestResult{{Check: "a", Status: "pass"}}
	fail := []TestResult{{Check: "a", Status: "fail", Error: "bad"}}
	mixed := []TestResult{{Check: "a", Status: "pass"}, {Check: "b", Status: "fail"}}

	if HasFailures(pass) {
		t.Error("expected no failures for all-pass results")
	}

	if !HasFailures(fail) {
		t.Error("expected failures for fail results")
	}

	if !HasFailures(mixed) {
		t.Error("expected failures for mixed results")
	}
}

func TestExtractBinary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		command string
		want    string
	}{
		{"echo hello", "echo"},
		{"FOO=bar mycmd --flag", "mycmd"},
		{"  git status  ", "git"},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractBinary(tt.command)
		if got != tt.want {
			t.Errorf("extractBinary(%q) = %q, want %q", tt.command, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	if got := truncate("short", 10); got != "short" {
		t.Errorf("expected 'short', got %q", got)
	}

	if got := truncate("a long string that needs truncation", 10); got != "a long str..." {
		t.Errorf("expected truncation, got %q", got)
	}
}

// --- helpers ---

func countFailures(results []TestResult) int {
	n := 0

	for _, r := range results {
		if r.Status == "fail" {
			n++
		}
	}

	return n
}

func assertCheckFailed(t *testing.T, results []TestResult, check string) {
	t.Helper()

	for _, r := range results {
		if r.Check == check {
			if r.Status != "fail" {
				t.Errorf("expected check %q to fail, got status=%q", check, r.Status)
			}

			return
		}
	}

	t.Errorf("check %q not found in results", check)
}

func assertCheckPassed(t *testing.T, results []TestResult, check string) {
	t.Helper()

	for _, r := range results {
		if r.Check == check {
			if r.Status != "pass" {
				t.Errorf("expected check %q to pass, got status=%q error=%q", check, r.Status, r.Error)
			}

			return
		}
	}

	t.Errorf("check %q not found in results", check)
}
