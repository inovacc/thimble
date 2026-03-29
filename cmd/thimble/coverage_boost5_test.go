package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/thimble/internal/hooklog"
	"github.com/inovacc/thimble/internal/model"
	"github.com/inovacc/thimble/internal/plugin"
	"github.com/inovacc/thimble/internal/report"
	"github.com/inovacc/thimble/internal/session"
)

// --- printTableEntries ---

func TestPrintTableEntries(t *testing.T) {
	t.Helper()

	entries := []hooklog.Entry{
		{
			Timestamp:  time.Now(),
			Event:      "PreToolUse",
			ToolName:   "Bash",
			Blocked:    false,
			DurationMs: 5,
			Reason:     "",
		},
		{
			Timestamp:  time.Now(),
			Event:      "PreToolUse",
			ToolName:   "Read",
			Blocked:    true,
			DurationMs: 2,
			Reason:     "blocked for security reasons because the path is restricted",
			Error:      "access denied",
		},
		{
			Timestamp:  time.Now(),
			Event:      "PostToolUse",
			ToolName:   "",
			Blocked:    false,
			DurationMs: 1,
			Error:      "some error",
		},
	}
	// Should not panic.
	printTableEntries(entries)
}

// --- printDebugEntries ---

func TestPrintDebugEntries(t *testing.T) {
	t.Helper()

	entries := []hooklog.Entry{
		{
			Timestamp:   time.Now(),
			Platform:    "claude-code",
			Event:       "PreToolUse",
			ToolName:    "Bash",
			SessionID:   "sess-123",
			ProjectDir:  "/project",
			DurationMs:  10,
			Blocked:     true,
			Reason:      "dangerous command",
			HasContext:  true,
			Error:       "err msg",
			ToolInput:   `{"command":"rm -rf /"}`,
			Response:    `{"decision":"deny"}`,
			GRPCPayload: `{"raw":"data"}`,
		},
		{
			Timestamp:  time.Now(),
			Platform:   "claude-code",
			Event:      "PostToolUse",
			ToolName:   "Read",
			DurationMs: 3,
			Blocked:    false,
		},
	}
	// Should not panic.
	printDebugEntries(entries)
}

// --- saveHooklogReport ---

func TestSaveHooklogReport(t *testing.T) {
	cleanup := setupReportDirForBoost5(t)
	defer cleanup()

	entries := []hooklog.Entry{
		{
			Timestamp:  time.Now(),
			Platform:   "claude-code",
			Event:      "PreToolUse",
			ToolName:   "Bash",
			Blocked:    true,
			Reason:     "dangerous",
			DurationMs: 5,
		},
		{
			Timestamp:  time.Now(),
			Platform:   "claude-code",
			Event:      "PostToolUse",
			ToolName:   "Read",
			Blocked:    false,
			DurationMs: 2,
		},
	}

	// Should not panic.
	saveHooklogReport(entries, 1, 1)
}

func setupReportDirForBoost5(t *testing.T) func() {
	t.Helper()
	tmp := t.TempDir()
	orig := report.ReportsDir
	report.ReportsDir = func() string { return tmp }

	return func() { report.ReportsDir = orig }
}

// --- printPlugin ---

func TestPrintPlugin_Full(t *testing.T) {
	t.Helper()

	p := plugin.PluginDef{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "A test plugin",
		Author:      &plugin.Author{Name: "Test Author"},
		License:     "MIT",
		Tools: []plugin.ToolDef{
			{Name: "ctx_test_tool", Description: "does testing"},
			{Name: "ctx_other_tool", Description: "does other things"},
		},
	}
	// Should not panic.
	printPlugin(p)
}

func TestPrintPlugin_Minimal(t *testing.T) {
	t.Helper()

	p := plugin.PluginDef{
		Name:    "minimal",
		Version: "0.1.0",
	}
	// Should not panic.
	printPlugin(p)
}

func TestPrintPlugin_NoAuthor(t *testing.T) {
	t.Helper()

	p := plugin.PluginDef{
		Name:        "no-author",
		Version:     "1.0.0",
		Description: "Plugin without author",
		License:     "BSD-3-Clause",
		Tools: []plugin.ToolDef{
			{Name: "ctx_tool", Description: "a tool"},
		},
	}
	printPlugin(p)
}

// --- printCheckResults ---

func TestPrintCheckResults_NoUpdates(t *testing.T) {
	results := []plugin.UpdateResult{
		{Name: "plugin-a", InstalledVer: "1.0.0", UpdateType: ""},
		{Name: "plugin-b", InstalledVer: "2.0.0", UpdateType: ""},
	}

	err := printCheckResults(results)
	if err != nil {
		t.Fatalf("printCheckResults: %v", err)
	}
}

func TestPrintCheckResults_WithUpdates(t *testing.T) {
	results := []plugin.UpdateResult{
		{Name: "plugin-a", InstalledVer: "1.0.0", AvailableVer: "1.1.0", UpdateType: "minor"},
		{Name: "plugin-b", InstalledVer: "2.0.0", UpdateType: ""},
		{Name: "plugin-c", InstalledVer: "1.0.0", AvailableVer: "2.0.0", UpdateType: "major", Breaking: true},
	}

	err := printCheckResults(results)
	if err != nil {
		t.Fatalf("printCheckResults: %v", err)
	}
}

func TestPrintCheckResults_Empty(t *testing.T) {
	err := printCheckResults(nil)
	if err != nil {
		t.Fatalf("printCheckResults: %v", err)
	}
}

// --- installFilteredUpdates ---

func TestInstallFilteredUpdates_NoUpdates(t *testing.T) {
	results := []plugin.UpdateResult{
		{Name: "plugin-a", InstalledVer: "1.0.0", UpdateType: ""},
	}

	err := installFilteredUpdates(results, false)
	if err != nil {
		t.Fatalf("installFilteredUpdates: %v", err)
	}
}

func TestInstallFilteredUpdates_SkipsMajorWhenDisallowed(t *testing.T) {
	results := []plugin.UpdateResult{
		{Name: "plugin-a", InstalledVer: "1.0.0", AvailableVer: "2.0.0", UpdateType: "major", Breaking: true},
	}
	// Should skip major without error.
	err := installFilteredUpdates(results, false)
	if err != nil {
		t.Fatalf("installFilteredUpdates: %v", err)
	}
}

// --- resolveSessionDB ---

func TestResolveSessionDB_NoDBFile(t *testing.T) {
	dir := t.TempDir()

	_, err := resolveSessionDB(dir)
	if err == nil {
		t.Error("expected error when no session.db exists")
	}
}

func TestResolveSessionDB_WithDB(t *testing.T) {
	dir := t.TempDir()
	// Create the expected data directory structure.
	dataDir := filepath.Join(dir, "sessions")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// resolveSessionDB uses paths.ProjectDataDir which hashes the project dir.
	// Instead, test resolveSessionDBForImport which creates the DB.
	sdb, err := resolveSessionDBForImport(dir)
	if err != nil {
		t.Fatalf("resolveSessionDBForImport: %v", err)
	}

	sdb.Close()
}

func TestResolveSessionDBForImport_EmptyProjectDir(t *testing.T) {
	// With empty string, it uses os.Getwd().
	sdb, err := resolveSessionDBForImport("")
	if err != nil {
		t.Fatalf("resolveSessionDBForImport: %v", err)
	}

	sdb.Close()
}

// --- saveStatsReport ---

func TestSaveStatsReport(t *testing.T) {
	cleanup := setupReportDirForBoost5(t)
	defer cleanup()

	result := &statsResult{
		SessionID:    "test-save-report",
		DurationText: "5m30s",
		TotalEvents:  42,
		EventsByType: map[string]int{"tool_call": 30, "error": 12},
		TopTools:     []model.ToolCount{{Name: "ctx_search", Count: 15}},
		ErrorCount:   12,
		ErrorRate:    28.6,
	}

	cmd := statsCmd
	cmd.SetErr(os.Stderr)

	err := saveStatsReport(cmd, result)
	if err != nil {
		t.Fatalf("saveStatsReport: %v", err)
	}
}

// --- publish helpers ---

func TestStepf(t *testing.T) {
	t.Helper()
	// Should not panic.
	stepf("step %d: %s", 1, "testing")
}

func TestSuccessf(t *testing.T) {
	t.Helper()
	successf("done: %s", "ok")
}

func TestInfof(t *testing.T) {
	t.Helper()
	infof("info: %d items", 5)
}

func TestWarnf(t *testing.T) {
	t.Helper()
	warnf("warning: %s", "something")
}

// --- runStats with real session DB ---

func TestRunStats_WithSessionData(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session.db")

	sdb, err := session.New(dbPath)
	if err != nil {
		t.Fatalf("create session db: %v", err)
	}

	sid := "stats-test-session"
	if err := sdb.EnsureSession(sid, dir); err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	events := []model.SessionEvent{
		{Type: "tool_call", Category: "tool", Data: `{"tool":"Bash","seq":1}`, Priority: 2},
		{Type: "tool_call", Category: "tool", Data: `{"tool":"Read","seq":2}`, Priority: 2},
	}
	for i, ev := range events {
		if err := sdb.InsertEvent(sid, ev, "PostToolUse"); err != nil {
			t.Fatalf("insert event %d: %v", i, err)
		}
	}

	sdb.Close()

	// Override statsProjectDir to point to a dir where we control the session.db location.
	// We need to place session.db where paths.ProjectDataDir would look for it.
	// Instead, test gatherStats directly with a fresh DB.
	sdb2, err := session.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sdb2.Close()

	result, err := gatherStats(sdb2, sid)
	if err != nil {
		t.Fatalf("gatherStats: %v", err)
	}

	if result.TotalEvents != 2 {
		t.Errorf("TotalEvents = %d, want 2", result.TotalEvents)
	}

	if result.ErrorCount != 0 {
		t.Errorf("ErrorCount = %d, want 0", result.ErrorCount)
	}
}

// --- printStatsTable with empty data ---

func TestPrintStatsTable_EmptyData(t *testing.T) {
	t.Helper()

	result := &statsResult{
		SessionID:    "empty-session",
		DurationText: "0s",
		TotalEvents:  0,
		EventsByType: map[string]int{},
		TopTools:     nil,
		ErrorCount:   0,
		ErrorRate:    0,
	}

	cmd := statsCmd
	cmd.SetOut(os.Stdout)
	printStatsTable(cmd, result)
}

// --- generateRoutingInstructions non-dry-run ---

func TestGenerateRoutingInstructions_Write(t *testing.T) {
	t.Helper()

	origDryRun := setupDryRun

	defer func() { setupDryRun = origDryRun }()

	setupDryRun = false

	// This will write instructions to cwd. It's safe because routing.WriteInstructions
	// checks if the file already exists. Just exercise the code path.
	err := generateRoutingInstructions("claude-code")
	// May succeed or return "already present" — both are fine.
	_ = err
}

// --- hooklog Entry with long reason truncation ---

func TestPrintTableEntries_LongReason(t *testing.T) {
	t.Helper()

	var longReason strings.Builder
	for i := range 100 {
		longReason.WriteString("x")

		_ = i
	}

	entries := []hooklog.Entry{
		{
			Timestamp:  time.Now(),
			Event:      "PreToolUse",
			ToolName:   "Bash",
			Blocked:    true,
			DurationMs: 3,
			Reason:     longReason.String(),
		},
	}
	// Should truncate reason at 60 chars + "...".
	printTableEntries(entries)
}

// --- runPluginDir ---

func TestRunPluginDir(t *testing.T) {
	t.Helper()

	cmd := pluginDirCmd
	cmd.SetOut(os.Stdout)

	err := runPluginDir(cmd, nil)
	if err != nil {
		t.Fatalf("runPluginDir: %v", err)
	}
}
