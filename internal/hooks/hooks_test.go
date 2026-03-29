package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/thimble/internal/hooklog"
	"github.com/inovacc/thimble/internal/model"
	"github.com/inovacc/thimble/internal/session"
	"github.com/inovacc/thimble/internal/store"
)

// ── Helpers ──

// newTestSessionDB creates a real SessionDB backed by a temp file.
func newTestSessionDB(t *testing.T) *session.SessionDB {
	t.Helper()

	dir := t.TempDir()

	db, err := session.New(filepath.Join(dir, "session.db"))
	if err != nil {
		t.Fatalf("create session db: %v", err)
	}

	t.Cleanup(func() { db.Close() })

	return db
}

// newTestStore creates a real ContentStore backed by a temp file.
func newTestStore(t *testing.T) *store.ContentStore {
	t.Helper()

	dir := t.TempDir()

	cs, err := store.New(filepath.Join(dir, "content.db"))
	if err != nil {
		t.Fatalf("create content store: %v", err)
	}

	t.Cleanup(func() { cs.Close() })

	return cs
}

// newTestHookLog creates a hooklog.Logger writing to a temp dir.
func newTestHookLog(t *testing.T) *hooklog.Logger {
	t.Helper()

	dir := t.TempDir()

	hl, err := hooklog.New(dir)
	if err != nil {
		t.Fatalf("create hook log: %v", err)
	}

	return hl
}

// makePayload marshals a HookInput to JSON bytes.
func makePayload(t *testing.T, input model.HookInput) []byte {
	t.Helper()

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal hook input: %v", err)
	}

	return data
}

// ── Tests ──

func TestNewDispatcher(t *testing.T) {
	t.Run("with nil getters", func(t *testing.T) {
		d := NewDispatcher(nil, nil, nil)
		if d == nil {
			t.Fatal("expected non-nil dispatcher")
		}

		if d.guidance == nil {
			t.Fatal("expected guidance tracker to be initialized")
		}
	})

	t.Run("with valid getters", func(t *testing.T) {
		sdb := newTestSessionDB(t)
		cs := newTestStore(t)
		hl := newTestHookLog(t)

		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return sdb, nil },
			func(string) (*store.ContentStore, error) { return cs, nil },
			hl,
		)
		if d == nil {
			t.Fatal("expected non-nil dispatcher")
		}
	})
}

func TestGuidanceTracker(t *testing.T) {
	t.Run("ShouldShow returns true first time false on repeat", func(t *testing.T) {
		g := newGuidanceTracker()

		if !g.ShouldShow("sess1", AdvisoryReadForAnalysis) {
			t.Error("expected true on first call")
		}

		if g.ShouldShow("sess1", AdvisoryReadForAnalysis) {
			t.Error("expected false on repeat call")
		}
	})

	t.Run("different sessions are independent", func(t *testing.T) {
		g := newGuidanceTracker()

		if !g.ShouldShow("sess1", AdvisoryGrepContextFlood) {
			t.Error("expected true for sess1")
		}

		if !g.ShouldShow("sess2", AdvisoryGrepContextFlood) {
			t.Error("expected true for sess2 (different session)")
		}
	})

	t.Run("different advisories are independent", func(t *testing.T) {
		g := newGuidanceTracker()

		if !g.ShouldShow("s1", AdvisoryReadForAnalysis) {
			t.Error("expected true for read advisory")
		}

		if !g.ShouldShow("s1", AdvisoryBashLargeOutput) {
			t.Error("expected true for bash advisory (different type)")
		}
	})

	t.Run("Reset clears state", func(t *testing.T) {
		g := newGuidanceTracker()

		_ = g.ShouldShow("s1", AdvisoryReadForAnalysis)

		g.Reset()

		if !g.ShouldShow("s1", AdvisoryReadForAnalysis) {
			t.Error("expected true after reset")
		}
	})
}

func newPreToolUseDispatcher(t *testing.T) *Dispatcher {
	t.Helper()

	sdb := newTestSessionDB(t)
	cs := newTestStore(t)
	hl := newTestHookLog(t)

	return NewDispatcher(
		func(string) (*session.SessionDB, error) { return sdb, nil },
		func(string) (*store.ContentStore, error) { return cs, nil },
		hl,
	)
}

func TestPreToolUse_BasicPayload(t *testing.T) {
	projectDir := t.TempDir()

	t.Run("invalid payload returns error", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		_, err := d.PreToolUse([]byte("claude"), []byte("not json"))
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("nil tool call passes through", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		payload := makePayload(t, model.HookInput{
			ProjectDir: projectDir,
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Blocked {
			t.Error("expected not blocked for nil tool call")
		}
	})

	t.Run("default session ID when empty", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "", // empty -> should use "default"
			ToolCall: &model.ToolCall{
				ToolName:  "Read",
				ToolInput: map[string]any{"file_path": "/tmp/file.go"},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should get advisory (first time for "default" session).
		var out model.HookOutput

		_ = json.Unmarshal(resp.Result, &out)

		result, _ := out.Result.(string)
		if result == "" {
			t.Error("expected advisory for default session ID")
		}
	})
}

func TestPreToolUse_SafeTools(t *testing.T) {
	projectDir := t.TempDir()

	safeTools := []struct {
		name     string
		toolName string
		input    map[string]any
	}{
		{"Read tool", "Read", map[string]any{"file_path": "/tmp/test.go"}},
		{"Glob tool", "Glob", map[string]any{"pattern": "*.go"}},
		{"Edit tool", "Edit", map[string]any{"file_path": "/tmp/test.go", "old_string": "a", "new_string": "b"}},
	}

	for _, tc := range safeTools {
		t.Run(tc.name+" passes through", func(t *testing.T) {
			d := newPreToolUseDispatcher(t)

			payload := makePayload(t, model.HookInput{
				ProjectDir: projectDir,
				SessionID:  "safe-test",
				ToolCall: &model.ToolCall{
					ToolName:  tc.toolName,
					ToolInput: tc.input,
				},
			})

			resp, err := d.PreToolUse([]byte("claude"), payload)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Blocked {
				t.Errorf("expected %s to not be blocked", tc.toolName)
			}
		})
	}
}

func TestPreToolUse_SecurityBlocking(t *testing.T) {
	t.Run("Bash with denied command is blocked", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		projectDir := t.TempDir()

		settingsDir := filepath.Join(projectDir, ".claude")
		if err := os.MkdirAll(settingsDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		policy := map[string]any{
			"permissions": map[string]any{
				"deny": []string{"Bash(rm -rf *)"},
			},
		}

		policyData, _ := json.Marshal(policy)
		if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), policyData, 0o644); err != nil {
			t.Fatalf("write policy: %v", err)
		}

		payload := makePayload(t, model.HookInput{
			ProjectDir: projectDir,
			SessionID:  "bash-deny",
			ToolCall: &model.ToolCall{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "rm -rf /"},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp.Blocked {
			t.Error("expected Bash rm -rf to be blocked")
		}
	})

	t.Run("shell escape in ctx_execute blocked", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		pd := t.TempDir()
		settingsDir := filepath.Join(pd, ".claude")
		_ = os.MkdirAll(settingsDir, 0o755)

		policy := map[string]any{
			"permissions": map[string]any{
				"deny": []string{"Bash(rm -rf *)"},
			},
		}

		policyData, _ := json.Marshal(policy)
		_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), policyData, 0o644)

		payload := makePayload(t, model.HookInput{
			ProjectDir: pd,
			SessionID:  "shell-escape-test",
			ToolCall: &model.ToolCall{
				ToolName: "ctx_execute",
				ToolInput: map[string]any{
					"language": "python",
					"code":     "import os; os.system('rm -rf /')",
				},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp.Blocked {
			t.Error("expected shell escape in ctx_execute to be blocked")
		}
	})

	t.Run("batch execute with denied command blocked", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		pd := t.TempDir()
		settingsDir := filepath.Join(pd, ".claude")
		_ = os.MkdirAll(settingsDir, 0o755)

		policy := map[string]any{
			"permissions": map[string]any{
				"deny": []string{"Bash(rm -rf *)"},
			},
		}

		policyData, _ := json.Marshal(policy)
		_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), policyData, 0o644)

		payload := makePayload(t, model.HookInput{
			ProjectDir: pd,
			SessionID:  "batch-deny-test",
			ToolCall: &model.ToolCall{
				ToolName: "ctx_batch_execute",
				ToolInput: map[string]any{
					"commands": []any{"ls -la", "rm -rf /"},
				},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp.Blocked {
			t.Error("expected batch execute with rm -rf to be blocked")
		}
	})

	t.Run("file path deny blocks Read", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		pd := t.TempDir()
		settingsDir := filepath.Join(pd, ".claude")
		_ = os.MkdirAll(settingsDir, 0o755)

		policy := map[string]any{
			"permissions": map[string]any{
				"deny": []string{"Read(/etc/shadow)"},
			},
		}

		policyData, _ := json.Marshal(policy)
		_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), policyData, 0o644)

		payload := makePayload(t, model.HookInput{
			ProjectDir: pd,
			SessionID:  "file-deny-test",
			ToolCall: &model.ToolCall{
				ToolName:  "Read",
				ToolInput: map[string]any{"file_path": "/etc/shadow"},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp.Blocked {
			t.Error("expected Read of /etc/shadow to be blocked")
		}
	})
}

func TestPreToolUse_Advisories(t *testing.T) {
	t.Run("Read advisory fires once per session", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "read-advisory-test",
			ToolCall: &model.ToolCall{
				ToolName:  "Read",
				ToolInput: map[string]any{"file_path": "/tmp/analysis.go"},
			},
		})

		resp1, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out1 model.HookOutput
		if err := json.Unmarshal(resp1.Result, &out1); err != nil {
			t.Fatalf("unmarshal output: %v", err)
		}

		result1, _ := out1.Result.(string)
		if result1 == "" {
			t.Error("expected advisory text on first Read call")
		}

		resp2, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out2 model.HookOutput
		if err := json.Unmarshal(resp2.Result, &out2); err != nil {
			t.Fatalf("unmarshal output: %v", err)
		}

		if out2.Result != nil {
			t.Error("expected no advisory on second Read call (same session)")
		}
	})

	t.Run("Grep advisory fires once per session", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "grep-advisory-test",
			ToolCall: &model.ToolCall{
				ToolName:  "Grep",
				ToolInput: map[string]any{"pattern": "TODO"},
			},
		})

		resp1, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out1 model.HookOutput

		_ = json.Unmarshal(resp1.Result, &out1)

		result1, _ := out1.Result.(string)
		if result1 == "" {
			t.Error("expected advisory on first Grep call")
		}

		// Second call: no advisory.
		resp2, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out2 model.HookOutput

		_ = json.Unmarshal(resp2.Result, &out2)

		if out2.Result != nil {
			t.Error("expected no advisory on repeat Grep call")
		}
	})

	t.Run("Bash advisory fires once then empty", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "bash-advisory-test",
			ToolCall: &model.ToolCall{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "ls"},
			},
		})

		resp1, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out1 model.HookOutput

		_ = json.Unmarshal(resp1.Result, &out1)

		result1, _ := out1.Result.(string)
		if result1 == "" {
			t.Error("expected advisory on first Bash call")
		}

		resp2, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out2 model.HookOutput

		_ = json.Unmarshal(resp2.Result, &out2)

		if out2.Result != nil {
			t.Error("expected no advisory on second Bash call")
		}
	})

	t.Run("curl command triggers network advisory", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "curl-test",
			ToolCall: &model.ToolCall{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "curl https://example.com"},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Blocked {
			t.Error("curl should not be blocked, just advisory")
		}

		var out model.HookOutput

		_ = json.Unmarshal(resp.Result, &out)

		result, _ := out.Result.(string)
		if result == "" {
			t.Error("expected advisory for curl command")
		}
	})

	t.Run("WebFetch is blocked with advisory", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "webfetch-test",
			ToolCall: &model.ToolCall{
				ToolName:  "WebFetch",
				ToolInput: map[string]any{"url": "https://example.com"},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out model.HookOutput

		_ = json.Unmarshal(resp.Result, &out)

		result, _ := out.Result.(string)
		if result == "" {
			t.Error("expected advisory for WebFetch")
		}
	})
}

func TestPostToolUse(t *testing.T) {
	t.Run("invalid payload returns error", func(t *testing.T) {
		d := NewDispatcher(nil, nil, nil)

		_, err := d.PostToolUse("claude", "PostToolUse", []byte("bad json"))
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("records file event from tool call", func(t *testing.T) {
		sdb := newTestSessionDB(t)
		cs := newTestStore(t)
		hl := newTestHookLog(t)

		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return sdb, nil },
			func(string) (*store.ContentStore, error) { return cs, nil },
			hl,
		)

		projectDir := t.TempDir()

		// Ensure session exists first.
		_ = sdb.EnsureSession("default", projectDir)

		payload := makePayload(t, model.HookInput{
			ProjectDir: projectDir,
			SessionID:  "post-test",
			ToolCall: &model.ToolCall{
				ToolName:     "Read",
				ToolInput:    map[string]any{"file_path": "/tmp/hello.go"},
				ToolResponse: "package main\n",
			},
		})

		resp, err := d.PostToolUse("claude", "PostToolUse", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Blocked {
			t.Error("PostToolUse should never block")
		}
	})

	t.Run("handles nil getters gracefully", func(t *testing.T) {
		hl := newTestHookLog(t)
		d := NewDispatcher(
			func(string) (*session.SessionDB, error) {
				return nil, os.ErrNotExist
			},
			func(string) (*store.ContentStore, error) {
				return nil, os.ErrNotExist
			},
			hl,
		)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			ToolCall: &model.ToolCall{
				ToolName:  "Read",
				ToolInput: map[string]any{"file_path": "/tmp/x.go"},
			},
		})

		resp, err := d.PostToolUse("claude", "PostToolUse", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Blocked {
			t.Error("should not block even with failing getters")
		}
	})

	t.Run("auto-indexes readable file", func(t *testing.T) {
		sdb := newTestSessionDB(t)
		cs := newTestStore(t)
		hl := newTestHookLog(t)

		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return sdb, nil },
			func(string) (*store.ContentStore, error) { return cs, nil },
			hl,
		)

		projectDir := t.TempDir()
		_ = sdb.EnsureSession("default", projectDir)

		// Create a real file to be auto-indexed.
		testFile := filepath.Join(projectDir, "hello.go")
		if err := os.WriteFile(testFile, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
			t.Fatalf("write test file: %v", err)
		}

		payload := makePayload(t, model.HookInput{
			ProjectDir: projectDir,
			SessionID:  "auto-index",
			ToolCall: &model.ToolCall{
				ToolName:     "Read",
				ToolInput:    map[string]any{"file_path": testFile},
				ToolResponse: "package main\nfunc main() {}\n",
			},
		})

		_, err := d.PostToolUse("claude", "PostToolUse", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// The auto-index code path was exercised without error.
		// We cannot easily verify store contents without a Stats method,
		// but the absence of errors confirms the path works.
	})

	t.Run("records user message events", func(t *testing.T) {
		sdb := newTestSessionDB(t)
		hl := newTestHookLog(t)

		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return sdb, nil },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			hl,
		)

		projectDir := t.TempDir()
		_ = sdb.EnsureSession("default", projectDir)

		payload := makePayload(t, model.HookInput{
			ProjectDir: projectDir,
			SessionID:  "user-msg-test",
			Message: &model.UserMessage{
				Content: "Please fix the login bug",
			},
		})

		resp, err := d.PostToolUse("claude", "UserPromptSubmit", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Blocked {
			t.Error("should not block")
		}
	})
}

func TestSessionStart(t *testing.T) {
	t.Run("invalid payload returns error", func(t *testing.T) {
		d := NewDispatcher(nil, nil, nil)

		_, err := d.SessionStart("claude", []byte("bad"))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("startup with project dir returns routing instructions", func(t *testing.T) {
		sdb := newTestSessionDB(t)
		hl := newTestHookLog(t)

		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return sdb, nil },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			hl,
		)

		projectDir := t.TempDir()

		payload := makePayload(t, model.HookInput{
			ProjectDir: projectDir,
			SessionID:  "start-test",
		})

		resp, err := d.SessionStart("claude", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Result) == 0 {
			t.Error("expected non-empty routing instructions for startup")
		}

		if resp.Blocked {
			t.Error("SessionStart should never block")
		}
	})

	t.Run("clear source returns empty result", func(t *testing.T) {
		sdb := newTestSessionDB(t)
		hl := newTestHookLog(t)

		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return sdb, nil },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			hl,
		)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "clear-test",
			Extra:      map[string]any{"source": "clear"},
		})

		resp, err := d.SessionStart("claude", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// For "clear" source, GenerateInstructions is skipped.
		var out model.HookOutput

		_ = json.Unmarshal(resp.Result, &out)

		if out.Result != nil {
			t.Error("expected nil result for clear source")
		}
	})

	t.Run("resume source includes directive when events exist", func(t *testing.T) {
		sdb := newTestSessionDB(t)
		hl := newTestHookLog(t)

		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return sdb, nil },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			hl,
		)

		projectDir := t.TempDir()
		_ = sdb.EnsureSession("default", projectDir)

		// Insert a session event so resume has data.
		_ = sdb.InsertEvent("default", model.SessionEvent{
			Type:     "file_read",
			Category: "file",
			Data:     "/tmp/test.go",
			Priority: model.PriorityNormal,
		}, "test")

		payload := makePayload(t, model.HookInput{
			ProjectDir: projectDir,
			SessionID:  "resume-test",
			Extra:      map[string]any{"source": "resume"},
		})

		resp, err := d.SessionStart("claude", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Result) == 0 {
			t.Error("expected non-empty result for resume with events")
		}
	})

	t.Run("no project dir still returns instructions", func(t *testing.T) {
		hl := newTestHookLog(t)

		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return nil, os.ErrNotExist },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			hl,
		)

		payload := makePayload(t, model.HookInput{
			SessionID: "no-project-test",
		})

		resp, err := d.SessionStart("claude", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should still return routing instructions even without projectDir.
		if len(resp.Result) == 0 {
			t.Error("expected routing instructions even without project dir")
		}
	})
}

func TestPreCompact(t *testing.T) {
	t.Run("invalid payload returns error", func(t *testing.T) {
		d := NewDispatcher(nil, nil, nil)

		_, err := d.PreCompact("claude", []byte("{bad"))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no project dir returns empty", func(t *testing.T) {
		hl := newTestHookLog(t)

		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return nil, os.ErrNotExist },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			hl,
		)

		payload := makePayload(t, model.HookInput{
			SessionID: "compact-no-dir",
		})

		resp, err := d.PreCompact("claude", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out model.HookOutput

		_ = json.Unmarshal(resp.Result, &out)

		if out.Result != nil {
			t.Error("expected nil result without project dir")
		}
	})

	t.Run("with events builds snapshot and directive", func(t *testing.T) {
		sdb := newTestSessionDB(t)
		hl := newTestHookLog(t)

		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return sdb, nil },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			hl,
		)

		projectDir := t.TempDir()
		_ = sdb.EnsureSession("default", projectDir)

		// Insert events.
		for i := range 3 {
			_ = sdb.InsertEvent("default", model.SessionEvent{
				Type:     "file_read",
				Category: "file",
				Data:     filepath.Join("/tmp", "file"+string(rune('a'+i))+".go"),
				Priority: model.PriorityNormal,
			}, "test")
		}

		payload := makePayload(t, model.HookInput{
			ProjectDir: projectDir,
			SessionID:  "compact-test",
		})

		resp, err := d.PreCompact("claude", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out model.HookOutput

		_ = json.Unmarshal(resp.Result, &out)

		result, _ := out.Result.(string)
		if result == "" {
			t.Error("expected non-empty directive from PreCompact with events")
		}

		// Verify resume was upserted.
		resume, err := sdb.GetResume("default")
		if err != nil {
			t.Fatalf("get resume: %v", err)
		}

		if resume == nil {
			t.Error("expected resume snapshot to be upserted")
		}
	})

	t.Run("session getter error returns empty gracefully", func(t *testing.T) {
		hl := newTestHookLog(t)

		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return nil, os.ErrNotExist },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			hl,
		)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "compact-err",
		})

		resp, err := d.PreCompact("claude", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out model.HookOutput

		_ = json.Unmarshal(resp.Result, &out)

		if out.Result != nil {
			t.Error("expected nil result when session getter fails")
		}
	})
}

func TestHookLogIntegration(t *testing.T) {
	t.Run("PreToolUse logs entry", func(t *testing.T) {
		hl := newTestHookLog(t)
		hl.SetDebug(true) // enable full payload logging

		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return nil, os.ErrNotExist },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			hl,
		)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "log-test",
			ToolCall: &model.ToolCall{
				ToolName:  "Glob",
				ToolInput: map[string]any{"pattern": "*.go"},
			},
		})

		_, _ = d.PreToolUse([]byte("claude"), payload)

		entries, err := hl.Read(nil)
		if err != nil {
			t.Fatalf("read hook log: %v", err)
		}

		if len(entries) == 0 {
			t.Error("expected at least one log entry after PreToolUse")
		}

		if entries[0].Event != "PreToolUse" {
			t.Errorf("expected event PreToolUse, got %s", entries[0].Event)
		}

		if entries[0].ToolName != "Glob" {
			t.Errorf("expected tool name Glob, got %s", entries[0].ToolName)
		}
	})

	t.Run("nil hook log does not panic", func(t *testing.T) {
		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return nil, os.ErrNotExist },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			nil, // nil hookLog
		)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			ToolCall: &model.ToolCall{
				ToolName:  "Read",
				ToolInput: map[string]any{"file_path": "/tmp/x"},
			},
		})

		// Should not panic.
		_, _ = d.PreToolUse([]byte("claude"), payload)
		_, _ = d.PostToolUse("claude", "PostToolUse", payload)
	})
}

func TestPluginHookRunner(t *testing.T) {
	t.Run("nil runner does not panic", func(t *testing.T) {
		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return nil, os.ErrNotExist },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			nil,
		)

		// No SetPluginHookRunner called — runner is nil.
		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			ToolCall: &model.ToolCall{
				ToolName:  "Glob",
				ToolInput: map[string]any{"pattern": "*.go"},
			},
		})

		// Should not panic.
		_, _ = d.PreToolUse([]byte("claude"), payload)
		_, _ = d.PostToolUse("claude", "PostToolUse", payload)
		_, _ = d.SessionStart("claude", makePayload(t, model.HookInput{ProjectDir: t.TempDir()}))
		_, _ = d.PreCompact("claude", makePayload(t, model.HookInput{ProjectDir: t.TempDir()}))
	})

	t.Run("runner is called on PreToolUse", func(t *testing.T) {
		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return nil, os.ErrNotExist },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			nil,
		)

		var calledEvent, calledTool string

		d.SetPluginHookRunner(func(event string, toolName string) []PluginHookResult {
			calledEvent = event
			calledTool = toolName

			return nil
		})

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "runner-test",
			ToolCall: &model.ToolCall{
				ToolName:  "Glob",
				ToolInput: map[string]any{"pattern": "*.go"},
			},
		})

		_, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if calledEvent != "PreToolUse" {
			t.Errorf("expected event PreToolUse, got %q", calledEvent)
		}

		if calledTool != "Glob" {
			t.Errorf("expected tool Glob, got %q", calledTool)
		}
	})

	t.Run("runner is called on SessionStart", func(t *testing.T) {
		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return nil, os.ErrNotExist },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			nil,
		)

		var calledEvent string

		d.SetPluginHookRunner(func(event string, toolName string) []PluginHookResult {
			calledEvent = event

			return nil
		})

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "session-runner-test",
		})

		_, err := d.SessionStart("claude", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if calledEvent != "SessionStart" {
			t.Errorf("expected event SessionStart, got %q", calledEvent)
		}
	})

	t.Run("runner results with errors are handled", func(t *testing.T) {
		d := NewDispatcher(
			func(string) (*session.SessionDB, error) { return nil, os.ErrNotExist },
			func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
			nil,
		)

		d.SetPluginHookRunner(func(event string, toolName string) []PluginHookResult {
			return []PluginHookResult{
				{
					Plugin:  "test-plugin",
					Command: "false",
					Err:     os.ErrNotExist,
				},
			}
		})

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "error-runner-test",
		})

		// Should not panic or fail — errors are logged, not propagated.
		resp, err := d.PreCompact("claude", payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Blocked {
			t.Error("plugin hook errors should not block")
		}
	})
}

func TestPreToolUse_DangerousGitGhBlocking(t *testing.T) {
	t.Run("default dangerous git command is blocked", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "git-dangerous-default",
			ToolCall: &model.ToolCall{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "git push --force origin main"},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp.Blocked {
			t.Error("expected git push --force to be blocked by default")
		}

		if resp.Reason == "" {
			t.Error("expected reason to be set")
		}
	})

	t.Run("default dangerous gh command is blocked", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "gh-dangerous-default",
			ToolCall: &model.ToolCall{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "gh repo delete owner/repo"},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp.Blocked {
			t.Error("expected gh repo delete to be blocked by default")
		}
	})

	t.Run("chained dangerous git command is blocked", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "git-chain-test",
			ToolCall: &model.ToolCall{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "echo hello && git reset --hard HEAD~1"},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp.Blocked {
			t.Error("expected chained git reset --hard to be blocked")
		}
	})

	t.Run("custom git override replaces defaults", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		pd := t.TempDir()
		settingsDir := filepath.Join(pd, ".claude")
		_ = os.MkdirAll(settingsDir, 0o755)

		// Custom override: only block "stash drop", NOT "push --force".
		settings := map[string]any{
			"permissions":             map[string]any{},
			"dangerous_git_overrides": []string{"stash drop"},
		}

		data, _ := json.Marshal(settings)
		_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0o644)

		// "git push --force" should NOT be blocked (not in custom list).
		payload := makePayload(t, model.HookInput{
			ProjectDir: pd,
			SessionID:  "git-custom-override",
			ToolCall: &model.ToolCall{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "git push --force origin main"},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Blocked {
			t.Error("expected git push --force to be allowed with custom override")
		}

		// "git stash drop" should be blocked (in custom list).
		payload2 := makePayload(t, model.HookInput{
			ProjectDir: pd,
			SessionID:  "git-custom-override-2",
			ToolCall: &model.ToolCall{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "git stash drop"},
			},
		})

		resp2, err := d.PreToolUse([]byte("claude"), payload2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp2.Blocked {
			t.Error("expected git stash drop to be blocked by custom override")
		}
	})

	t.Run("custom gh override replaces defaults", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		pd := t.TempDir()
		settingsDir := filepath.Join(pd, ".claude")
		_ = os.MkdirAll(settingsDir, 0o755)

		// Custom override: only block "workflow disable", NOT "repo delete".
		settings := map[string]any{
			"permissions":            map[string]any{},
			"dangerous_gh_overrides": []string{"workflow disable"},
		}

		data, _ := json.Marshal(settings)
		_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0o644)

		// "gh repo delete" should NOT be blocked (not in custom list).
		payload := makePayload(t, model.HookInput{
			ProjectDir: pd,
			SessionID:  "gh-custom-override",
			ToolCall: &model.ToolCall{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "gh repo delete owner/repo"},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Blocked {
			t.Error("expected gh repo delete to be allowed with custom override")
		}

		// "gh workflow disable" should be blocked (in custom list).
		payload2 := makePayload(t, model.HookInput{
			ProjectDir: pd,
			SessionID:  "gh-custom-override-2",
			ToolCall: &model.ToolCall{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "gh workflow disable my-workflow"},
			},
		})

		resp2, err := d.PreToolUse([]byte("claude"), payload2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp2.Blocked {
			t.Error("expected gh workflow disable to be blocked by custom override")
		}
	})

	t.Run("safe git command passes through", func(t *testing.T) {
		d := newPreToolUseDispatcher(t)

		payload := makePayload(t, model.HookInput{
			ProjectDir: t.TempDir(),
			SessionID:  "git-safe-test",
			ToolCall: &model.ToolCall{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "git status"},
			},
		})

		resp, err := d.PreToolUse([]byte("claude"), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Blocked {
			t.Error("expected git status to pass through")
		}
	})
}

func TestSetGoalProvider(t *testing.T) {
	d := NewDispatcher(nil, nil, nil)

	if d.goalProvider != nil {
		t.Error("expected nil goalProvider on new dispatcher")
	}

	d.SetGoalProvider(func() string { return "test-goal" })

	if d.goalProvider == nil {
		t.Error("expected non-nil goalProvider after SetGoalProvider")
	}
}

func TestRecordEventsWithGoalTag(t *testing.T) {
	sdb := newTestSessionDB(t)

	d := NewDispatcher(
		func(string) (*session.SessionDB, error) { return sdb, nil },
		func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
		nil,
	)

	// Record without goal.
	events := []model.SessionEvent{
		{Type: "file_read", Category: "file", Data: "/tmp/test.go", Priority: 1},
	}

	d.recordEvents("/tmp/project", events)

	stored, err := sdb.GetEvents("default", nil)
	if err != nil {
		t.Fatalf("GetEvents error: %v", err)
	}

	if len(stored) != 1 {
		t.Fatalf("expected 1 event without goal, got %d", len(stored))
	}

	// Set goal and record more events.
	d.SetGoalProvider(func() string { return "implement auth" })

	events2 := []model.SessionEvent{
		{Type: "file_edit", Category: "file", Data: "/tmp/auth.go", Priority: 1},
	}

	d.recordEvents("/tmp/project", events2)

	stored2, err := sdb.GetEvents("default", nil)
	if err != nil {
		t.Fatalf("GetEvents error: %v", err)
	}

	// Should have: 1 original + 1 original + 1 companion = 3 events.
	if len(stored2) != 3 {
		t.Fatalf("expected 3 events with goal, got %d", len(stored2))
	}

	// The companion event should have JSON data with goal.
	companion := stored2[2]
	if companion.Category != "file" {
		t.Errorf("companion category = %q, want %q", companion.Category, "file")
	}

	if !strings.Contains(companion.Data, `"goal":"implement auth"`) {
		t.Errorf("companion data should contain goal, got %q", companion.Data)
	}

	if !strings.Contains(companion.Data, `"type":"file_edit"`) {
		t.Errorf("companion data should contain original type, got %q", companion.Data)
	}
}

func TestRecordEventsWithoutGoalProvider(t *testing.T) {
	sdb := newTestSessionDB(t)

	d := NewDispatcher(
		func(string) (*session.SessionDB, error) { return sdb, nil },
		func(string) (*store.ContentStore, error) { return nil, os.ErrNotExist },
		nil,
	)

	events := []model.SessionEvent{
		{Type: "file_read", Category: "file", Data: "/tmp/test.go", Priority: 1},
	}

	d.recordEvents("/tmp/project", events)

	stored, err := sdb.GetEvents("default", nil)
	if err != nil {
		t.Fatalf("GetEvents error: %v", err)
	}

	// Without goal provider, only original event should be recorded.
	if len(stored) != 1 {
		t.Fatalf("expected 1 event, got %d", len(stored))
	}
}

func TestTruncateForGoal(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"truncated", "hello world", 5, "hello"},
		{"with quotes", `say "hello"`, 20, `say \"hello\"`},
		{"with backslash", `path\to\file`, 20, `path\\to\\file`},
		{"empty", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateForGoal(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateForGoal(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
