package session

import (
	"strings"
	"testing"

	"github.com/inovacc/thimble/internal/model"
)

func TestBuildSessionDirectiveCompact(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "file_write", Category: "file", Data: "/tmp/main.go"},
		{Type: "task_create", Category: "task", Data: "implement feature X"},
		{Type: "user_request", Category: "intent", Data: "build the login page"},
	}

	result := BuildSessionDirective("compact", events)

	if !strings.Contains(result, `source="compact"`) {
		t.Error("expected compact source")
	}

	if !strings.Contains(result, "main.go") {
		t.Error("expected file in output")
	}

	if !strings.Contains(result, "implement feature X") {
		t.Error("expected task in output")
	}
}

func TestBuildSessionDirectiveResume(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "decision", Category: "decision", Data: "use PostgreSQL"},
	}

	result := BuildSessionDirective("resume", events)

	if !strings.Contains(result, `source="resume"`) {
		t.Error("expected resume source")
	}

	if !strings.Contains(result, "PostgreSQL") {
		t.Error("expected decision in output")
	}
}

func TestBuildSessionDirectiveEmpty(t *testing.T) {
	result := BuildSessionDirective("", nil)
	if result != "" {
		t.Errorf("expected empty for nil events and empty source, got %q", result)
	}
}

func TestBuildSessionDirectiveStartup(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "rule", Category: "rule", Data: "use slog for logging"},
	}

	result := BuildSessionDirective("startup", events)

	if !strings.Contains(result, `source="startup"`) {
		t.Error("expected startup source")
	}

	if !strings.Contains(result, "use slog for logging") {
		t.Error("expected rule in output")
	}
}

func TestBuildSessionDirectivePlanMode(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "plan_enter", Category: "plan", Data: "entered plan mode"},
		{Type: "plan_approved", Category: "plan", Data: "plan approved"},
	}

	result := BuildSessionDirective("compact", events)

	if !strings.Contains(result, "plan_approved") {
		t.Error("expected plan_approved event type")
	}

	if !strings.Contains(result, "plan approved") {
		t.Error("expected plan approved data")
	}
}

func TestBuildSessionDirectiveMixedEvents(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "file_edit", Category: "file", Data: "/src/app.go"},
		{Type: "error", Category: "error", Data: "compilation failed"},
		{Type: "git_commit", Category: "git", Data: "feat: add login"},
		{Type: "mcp_tool_call", Category: "mcp", Data: "ctx_search: query"},
		{Type: "env_change", Category: "env", Data: "GOPATH=/go"},
	}

	result := BuildSessionDirective("compact", events)

	if !strings.Contains(result, "app.go") {
		t.Error("expected file")
	}

	if !strings.Contains(result, "compilation failed") {
		t.Error("expected error")
	}

	if !strings.Contains(result, "feat: add login") {
		t.Error("expected git state")
	}

	if !strings.Contains(result, "ctx_search") {
		t.Error("expected mcp tool")
	}
}

func TestBuildSessionDirectiveDedup(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "file_edit", Category: "file", Data: "old.go"},
		{Type: "file_edit", Category: "file", Data: "new.go"},
	}

	result := BuildSessionDirective("compact", events)

	// Should keep last occurrence (new.go) and drop old.go.
	if strings.Contains(result, "old.go") {
		t.Error("expected old duplicate to be removed")
	}

	if !strings.Contains(result, "new.go") {
		t.Error("expected new.go to be kept")
	}
}
