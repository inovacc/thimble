package report

import (
	"strings"
	"testing"
	"time"
)

func TestRenderHookLogReport_BasicSummary(t *testing.T) {
	r := &Report{
		ID:   "hooklog-1",
		Type: ReportHookLog,
		HookLog: &HookLogData{
			Total:   10,
			Allowed: 7,
			Blocked: 3,
		},
	}

	got := renderReport(r)

	if !strings.Contains(got, "Total interactions:** 10") {
		t.Error("expected total interactions count")
	}

	if !strings.Contains(got, "Allowed:** 7") {
		t.Error("expected allowed count")
	}

	if !strings.Contains(got, "Blocked:** 3") {
		t.Error("expected blocked count")
	}

	if !strings.Contains(got, "## Instructions") {
		t.Error("expected instructions section")
	}
}

func TestRenderHookLogReport_WithEvents(t *testing.T) {
	r := &Report{
		ID:   "hooklog-2",
		Type: ReportHookLog,
		HookLog: &HookLogData{
			Total:   5,
			Allowed: 5,
			Events:  map[string]int{"PreToolUse": 3, "PostToolUse": 2},
		},
	}

	got := renderReport(r)

	if !strings.Contains(got, "## Events by Type") {
		t.Error("expected Events by Type section")
	}

	if !strings.Contains(got, "PreToolUse: 3") {
		t.Error("expected PreToolUse event count")
	}

	if !strings.Contains(got, "PostToolUse: 2") {
		t.Error("expected PostToolUse event count")
	}
}

func TestRenderHookLogReport_WithTools(t *testing.T) {
	r := &Report{
		ID:   "hooklog-3",
		Type: ReportHookLog,
		HookLog: &HookLogData{
			Total:   4,
			Allowed: 4,
			Tools:   map[string]int{"Bash": 3, "Read": 1},
		},
	}

	got := renderReport(r)

	if !strings.Contains(got, "## Tools by Frequency") {
		t.Error("expected Tools by Frequency section")
	}

	if !strings.Contains(got, "Bash: 3") {
		t.Error("expected Bash tool count")
	}
}

func TestRenderHookLogReport_WithBlockedEntries(t *testing.T) {
	r := &Report{
		ID:   "hooklog-4",
		Type: ReportHookLog,
		HookLog: &HookLogData{
			Total:   2,
			Blocked: 1,
			Entries: []HookLogEntry{
				{
					Timestamp: time.Date(2026, 3, 18, 14, 30, 0, 0, time.UTC),
					Event:     "PreToolUse",
					ToolName:  "Bash",
					Blocked:   true,
					Reason:    "dangerous command",
				},
				{
					Timestamp: time.Date(2026, 3, 18, 14, 31, 0, 0, time.UTC),
					Event:     "PostToolUse",
					ToolName:  "Read",
					Blocked:   false,
				},
			},
		},
	}

	got := renderReport(r)

	if !strings.Contains(got, "## Blocked Interactions") {
		t.Error("expected Blocked Interactions section")
	}

	if !strings.Contains(got, "dangerous command") {
		t.Error("expected blocked reason in output")
	}

	// Non-blocked entry should NOT appear in blocked section.
	if strings.Contains(got, "## Blocked Interactions") && !strings.Contains(got, "`Bash`") {
		t.Error("expected blocked tool name Bash")
	}
}

func TestRenderHookLogReport_Empty(t *testing.T) {
	r := &Report{
		ID:   "hooklog-5",
		Type: ReportHookLog,
		HookLog: &HookLogData{
			Total: 0,
		},
	}

	got := renderReport(r)

	if !strings.Contains(got, "Total interactions:** 0") {
		t.Error("expected zero total")
	}

	// Should NOT contain event/tool sections when empty.
	if strings.Contains(got, "## Events by Type") {
		t.Error("should not have Events section when empty")
	}

	if strings.Contains(got, "## Tools by Frequency") {
		t.Error("should not have Tools section when empty")
	}
}

func TestSaveAndReadHookLogReport(t *testing.T) {
	dir := t.TempDir()
	ReportsDir = func() string { return dir }

	t.Cleanup(func() { ReportsDir = defaultReportsDir })

	r := &Report{
		Type: ReportHookLog,
		HookLog: &HookLogData{
			Total:   5,
			Allowed: 3,
			Blocked: 2,
			Events:  map[string]int{"PreToolUse": 5},
			Tools:   map[string]int{"Bash": 5},
		},
	}

	id, err := SaveReport(r)
	if err != nil {
		t.Fatalf("SaveReport: %v", err)
	}

	got, err := ReadReport(id)
	if err != nil {
		t.Fatalf("ReadReport: %v", err)
	}

	if got.HookLog == nil {
		t.Fatal("HookLog data is nil")
	}

	if got.HookLog.Total != 5 {
		t.Errorf("Total = %d, want 5", got.HookLog.Total)
	}

	if got.HookLog.Blocked != 2 {
		t.Errorf("Blocked = %d, want 2", got.HookLog.Blocked)
	}
}
