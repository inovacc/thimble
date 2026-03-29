package session

import (
	"strings"
	"testing"

	"github.com/inovacc/thimble/internal/model"
)

func TestBuildSnapshotEmpty(t *testing.T) {
	snap := BuildResumeSnapshot(nil, nil)
	if !strings.Contains(snap, "session_resume") {
		t.Error("expected session_resume XML wrapper")
	}

	if !strings.Contains(snap, `events_captured="0"`) {
		t.Error("expected events_captured=0")
	}
}

func TestBuildSnapshotWithFiles(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "file_read", Category: "file", Data: "/src/main.go", Priority: 1},
		{Type: "file_edit", Category: "file", Data: "/src/main.go", Priority: 1},
		{Type: "file_write", Category: "file", Data: "/src/config.go", Priority: 1},
	}

	snap := BuildResumeSnapshot(events, nil)
	if !strings.Contains(snap, "active_files") {
		t.Error("expected active_files section")
	}

	if !strings.Contains(snap, "main.go") {
		t.Error("expected main.go in output")
	}
}

func TestBuildSnapshotWithEnvironment(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "cwd", Category: "cwd", Data: "/home/user/project", Priority: 2},
		{Type: "git", Category: "git", Data: "commit", Priority: 2},
		{Type: "env", Category: "env", Data: "export NODE_ENV=***", Priority: 2},
	}

	snap := BuildResumeSnapshot(events, nil)
	if !strings.Contains(snap, "environment") {
		t.Error("expected environment section")
	}

	if !strings.Contains(snap, "cwd") {
		t.Error("expected cwd in environment")
	}
}

func TestBuildSnapshotWithErrors(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "error_tool", Category: "error", Data: "compilation failed", Priority: 2},
	}

	snap := BuildResumeSnapshot(events, nil)
	if !strings.Contains(snap, "errors_encountered") {
		t.Error("expected errors_encountered section")
	}
}

func TestBuildSnapshotBudgetTrimming(t *testing.T) {
	// Create events that would exceed a tiny budget.
	events := make([]model.StoredEvent, 0, 50)
	for range 50 {
		events = append(events, model.StoredEvent{
			Type: "file_read", Category: "file",
			Data: strings.Repeat("x", 100), Priority: 1,
		})
	}

	snap := BuildResumeSnapshot(events, &BuildSnapshotOpts{MaxBytes: 200})
	if len(snap) > 200 {
		t.Errorf("snapshot = %d bytes, want <= 200", len(snap))
	}
}

func TestBuildSnapshotWithIntent(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "intent", Category: "intent", Data: "investigate", Priority: 4},
	}

	snap := BuildResumeSnapshot(events, nil)
	if !strings.Contains(snap, `mode="investigate"`) {
		t.Error("expected intent mode in output")
	}
}

func TestBuildSnapshotWithSubagents(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "subagent_launched", Category: "subagent", Data: "[launched] search files", Priority: 3},
		{Type: "subagent_completed", Category: "subagent", Data: "[completed] found config", Priority: 2},
	}

	snap := BuildResumeSnapshot(events, nil)
	if !strings.Contains(snap, "subagents") {
		t.Error("expected subagents section")
	}
}

func TestBuildSnapshotWithMcpTools(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "mcp", Category: "mcp", Data: "ctx_search: kubernetes", Priority: 3},
		{Type: "mcp", Category: "mcp", Data: "ctx_search: deploy", Priority: 3},
		{Type: "mcp", Category: "mcp", Data: "ctx_execute: run test", Priority: 3},
	}

	snap := BuildResumeSnapshot(events, nil)
	if !strings.Contains(snap, "mcp_tools") {
		t.Error("expected mcp_tools section")
	}

	if !strings.Contains(snap, `calls="2"`) {
		t.Error("expected ctx_search calls=2")
	}
}

func TestBuildSnapshotPlanMode(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "plan_enter", Category: "plan", Data: "entered plan mode", Priority: 2},
	}

	snap := BuildResumeSnapshot(events, nil)
	if !strings.Contains(snap, `plan_mode status="active"`) {
		t.Error("expected active plan_mode")
	}
}

func TestBuildResumeSnapshotPlanApproved(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "plan_enter", Category: "plan", Data: "entered plan mode", Priority: 3},
		{Type: "plan_approved", Category: "plan", Data: "plan approved", Priority: 3},
	}

	snapshot := BuildResumeSnapshot(events, &BuildSnapshotOpts{MaxBytes: 4096})

	if !strings.Contains(snapshot, `status="approved"`) {
		t.Error("expected approved plan status")
	}

	if !strings.Contains(snapshot, "Do NOT re-propose") {
		t.Error("expected instruction not to re-propose")
	}
}

func TestBuildResumeSnapshotPlanRejected(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "plan_enter", Category: "plan", Data: "entered plan mode", Priority: 3},
		{Type: "plan_rejected", Category: "plan", Data: "plan rejected", Priority: 3},
	}

	snapshot := BuildResumeSnapshot(events, &BuildSnapshotOpts{MaxBytes: 4096})

	if !strings.Contains(snapshot, `status="rejected"`) {
		t.Error("expected rejected plan status")
	}

	if !strings.Contains(snapshot, "ask the user") {
		t.Error("expected instruction to ask user")
	}
}

func TestSnapshotBudgetDefault(t *testing.T) {
	t.Setenv("THIMBLE_SNAPSHOT_BUDGET", "")

	budget := SnapshotBudget()
	if budget != DefaultMaxBytes {
		t.Errorf("SnapshotBudget() = %d, want %d", budget, DefaultMaxBytes)
	}
}

func TestSnapshotBudgetFromEnv(t *testing.T) {
	t.Setenv("THIMBLE_SNAPSHOT_BUDGET", "4096")

	budget := SnapshotBudget()
	if budget != 4096 {
		t.Errorf("SnapshotBudget() = %d, want 4096", budget)
	}
}

func TestSnapshotBudgetInvalidEnv(t *testing.T) {
	t.Setenv("THIMBLE_SNAPSHOT_BUDGET", "not-a-number")

	budget := SnapshotBudget()
	if budget != DefaultMaxBytes {
		t.Errorf("SnapshotBudget() = %d, want %d (fallback)", budget, DefaultMaxBytes)
	}
}

func TestSnapshotBudgetZeroEnv(t *testing.T) {
	t.Setenv("THIMBLE_SNAPSHOT_BUDGET", "0")

	budget := SnapshotBudget()
	if budget != DefaultMaxBytes {
		t.Errorf("SnapshotBudget() = %d, want %d (zero should fallback)", budget, DefaultMaxBytes)
	}
}

func TestSnapshotBudgetUsedByBuildSnapshot(t *testing.T) {
	t.Setenv("THIMBLE_SNAPSHOT_BUDGET", "8192")

	// Generate events that would exceed 2048 but fit in 8192.
	events := make([]model.StoredEvent, 0, 30)
	for i := range 30 {
		events = append(events, model.StoredEvent{
			Type: "file_read", Category: "file",
			Data: "/src/" + strings.Repeat("a", 50) + "_" + strings.Repeat("b", i) + ".go", Priority: 1,
		})
	}

	snap := BuildResumeSnapshot(events, nil) // opts=nil, so budget comes from env
	if len(snap) > 8192 {
		t.Errorf("snapshot = %d bytes, want <= 8192", len(snap))
	}
}

func TestRenderGoalsEmpty(t *testing.T) {
	result := renderGoals(nil)
	if result != "" {
		t.Errorf("renderGoals(nil) = %q, want empty", result)
	}
}

func TestRenderGoalsGrouping(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "file_edit", Category: "file", Data: `{"goal":"auth-refactor","path":"auth.go"}`, Priority: 1},
		{Type: "decision", Category: "decision", Data: `{"goal":"auth-refactor","choice":"use JWT"}`, Priority: 2},
		{Type: "file_edit", Category: "file", Data: `{"goal":"auth-refactor","path":"token.go"}`, Priority: 1},
		{Type: "file_edit", Category: "file", Data: `{"goal":"fix-build","path":"main.go"}`, Priority: 1},
	}

	result := renderGoals(events)
	if !strings.Contains(result, "<goals>") {
		t.Error("expected <goals> wrapper")
	}

	if !strings.Contains(result, `name="auth-refactor"`) {
		t.Error("expected auth-refactor goal")
	}

	if !strings.Contains(result, `events="3"`) {
		t.Error("expected 3 events for auth-refactor")
	}

	if !strings.Contains(result, `name="fix-build"`) {
		t.Error("expected fix-build goal")
	}

	if !strings.Contains(result, `events="1"`) {
		t.Error("expected 1 event for fix-build")
	}
}

func TestCollectGoalEventIgnoresNonJSON(t *testing.T) {
	ev := model.StoredEvent{Type: "file_read", Category: "file", Data: "/plain/path.go", Priority: 1}

	result := collectGoalEvent(nil, ev)
	if len(result) != 0 {
		t.Errorf("expected 0 events, got %d", len(result))
	}
}

func TestCollectGoalEventIgnoresMissingGoalKey(t *testing.T) {
	ev := model.StoredEvent{Type: "file_read", Category: "file", Data: `{"path":"a.go"}`, Priority: 1}

	result := collectGoalEvent(nil, ev)
	if len(result) != 0 {
		t.Errorf("expected 0 events, got %d", len(result))
	}
}

func TestCollectGoalEventCollects(t *testing.T) {
	ev := model.StoredEvent{Type: "file_edit", Category: "file", Data: `{"goal":"my-goal","path":"a.go"}`, Priority: 1}

	result := collectGoalEvent(nil, ev)
	if len(result) != 1 {
		t.Errorf("expected 1 event, got %d", len(result))
	}
}

func TestBuildSnapshotWithGoals(t *testing.T) {
	events := []model.StoredEvent{
		{Type: "file_edit", Category: "file", Data: `{"goal":"refactor-db","path":"db.go"}`, Priority: 1},
		{Type: "decision", Category: "decision", Data: `{"goal":"refactor-db","choice":"use sqlc"}`, Priority: 2},
	}

	snap := BuildResumeSnapshot(events, &BuildSnapshotOpts{MaxBytes: 4096})
	if !strings.Contains(snap, "<goals>") {
		t.Error("expected goals section in snapshot")
	}

	if !strings.Contains(snap, `name="refactor-db"`) {
		t.Error("expected refactor-db goal in snapshot")
	}
}
