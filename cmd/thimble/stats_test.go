package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/thimble/internal/model"
	"github.com/inovacc/thimble/internal/session"
)

func TestStatsCommandRegistered(t *testing.T) {
	found := false

	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "stats" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("stats command not registered on rootCmd")
	}
}

func TestStatsFlags(t *testing.T) {
	cmd := statsCmd

	jsonFlag := cmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Fatal("--json flag not registered")
	}

	reportFlag := cmd.Flags().Lookup("report")
	if reportFlag == nil {
		t.Fatal("--report flag not registered")
	}
}

func TestGatherStats(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session.db")

	sdb, err := session.New(dbPath)
	if err != nil {
		t.Fatalf("create session db: %v", err)
	}
	defer sdb.Close()

	sid := "test-session-001"

	if err := sdb.EnsureSession(sid, dir); err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	// Insert some events with slight data variation to avoid dedup.
	events := []model.SessionEvent{
		{Type: "tool_call", Category: "tool", Data: `{"tool":"ctx_search","seq":1}`, Priority: 2},
		{Type: "tool_call", Category: "tool", Data: `{"tool":"ctx_execute","seq":2}`, Priority: 2},
		{Type: "tool_call", Category: "tool", Data: `{"tool":"ctx_search","seq":3}`, Priority: 2},
		{Type: "error", Category: "system", Data: `{"message":"something failed","seq":4}`, Priority: 3},
		{Type: "guidance", Category: "advisory", Data: `{"text":"use caution","seq":5}`, Priority: 1},
	}

	for i, ev := range events {
		if err := sdb.InsertEvent(sid, ev, "PostToolUse"); err != nil {
			t.Fatalf("insert event %d: %v", i, err)
		}
	}

	result, err := gatherStats(sdb, sid)
	if err != nil {
		t.Fatalf("gatherStats: %v", err)
	}

	if result.SessionID != sid {
		t.Errorf("SessionID = %q, want %q", result.SessionID, sid)
	}

	if result.TotalEvents != 5 {
		t.Errorf("TotalEvents = %d, want 5", result.TotalEvents)
	}

	if result.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", result.ErrorCount)
	}

	if result.ErrorRate < 19.0 || result.ErrorRate > 21.0 {
		t.Errorf("ErrorRate = %.1f%%, want ~20%%", result.ErrorRate)
	}

	if len(result.EventsByType) == 0 {
		t.Error("EventsByType is empty")
	}

	if tc, ok := result.EventsByType["tool_call"]; !ok || tc != 3 {
		t.Errorf("EventsByType[tool_call] = %d, want 3", tc)
	}
}

func TestPrintStatsJSON(t *testing.T) {
	result := &statsResult{
		SessionID:    "test-123",
		DurationText: "5m30s",
		TotalEvents:  10,
		EventsByType: map[string]int{"tool_call": 8, "error": 2},
		TopTools:     []model.ToolCount{{Name: "ctx_search", Count: 5}},
		ErrorCount:   2,
		ErrorRate:    20.0,
	}

	var buf bytes.Buffer

	cmd := statsCmd
	cmd.SetOut(&buf)

	if err := printStatsJSON(cmd, result); err != nil {
		t.Fatalf("printStatsJSON: %v", err)
	}

	var decoded statsResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal JSON output: %v", err)
	}

	if decoded.SessionID != "test-123" {
		t.Errorf("decoded SessionID = %q, want %q", decoded.SessionID, "test-123")
	}

	if decoded.TotalEvents != 10 {
		t.Errorf("decoded TotalEvents = %d, want 10", decoded.TotalEvents)
	}
}

func TestPrintStatsTable(t *testing.T) {
	result := &statsResult{
		SessionID:    "test-456",
		DurationText: "1h2m3s",
		TotalEvents:  50,
		EventsByType: map[string]int{"tool_call": 40, "error": 5, "guidance": 5},
		TopTools:     []model.ToolCount{{Name: "ctx_execute", Count: 20}, {Name: "ctx_search", Count: 15}},
		ErrorCount:   5,
		ErrorRate:    10.0,
	}

	var buf bytes.Buffer

	cmd := statsCmd
	cmd.SetOut(&buf)

	printStatsTable(cmd, result)

	output := buf.String()

	for _, want := range []string{
		"Session: test-456",
		"Duration: 1h2m3s",
		"Total events: 50",
		"Errors: 5 (10.0%)",
		"Events by Type:",
		"Top 10 Tools:",
		"ctx_execute",
		"ctx_search",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, output)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0s"},
		{"seconds", 45 * time.Second, "45s"},
		{"minutes", 5*time.Minute + 30*time.Second, "5m30s"},
		{"hours", 2*time.Hour + 15*time.Minute + 30*time.Second, "2h15m30s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestStatsNoSessionData(t *testing.T) {
	origFn := statsProjectDir

	defer func() { statsProjectDir = origFn }()

	dir := t.TempDir()
	statsProjectDir = func() string { return dir }

	var outBuf, errBuf bytes.Buffer

	cmd := statsCmd
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(errBuf.String(), "No session data found") {
		t.Errorf("expected 'No session data found' message, got stderr: %q, stdout: %q", errBuf.String(), outBuf.String())
	}
}
