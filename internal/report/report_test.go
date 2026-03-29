package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveAndReadReport(t *testing.T) {
	dir := t.TempDir()
	ReportsDir = func() string { return dir }

	t.Cleanup(func() { ReportsDir = defaultReportsDir })

	r := &Report{
		Type:      ReportDoctor,
		CreatedAt: time.Now(),
		Doctor: &DoctorData{
			Summary: "healthy",
			Checks: []Check{
				{Name: "Go version", Status: "pass", Message: "go1.23"},
				{Name: "SQLite", Status: "fail", Message: "driver not found"},
			},
		},
	}

	id, err := SaveReport(r)
	if err != nil {
		t.Fatalf("SaveReport: %v", err)
	}

	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// Verify file exists.
	path := filepath.Join(dir, id+".txt")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("report file not found: %v", err)
	}

	// Read back.
	got, err := ReadReport(id)
	if err != nil {
		t.Fatalf("ReadReport: %v", err)
	}

	if got.ID != id {
		t.Errorf("ID = %q, want %q", got.ID, id)
	}

	if got.Type != ReportDoctor {
		t.Errorf("Type = %q, want %q", got.Type, ReportDoctor)
	}

	if got.Doctor == nil {
		t.Fatal("Doctor data is nil")
	}

	if len(got.Doctor.Checks) != 2 {
		t.Errorf("Checks count = %d, want 2", len(got.Doctor.Checks))
	}
}

func TestReadReportRaw(t *testing.T) {
	dir := t.TempDir()
	ReportsDir = func() string { return dir }

	t.Cleanup(func() { ReportsDir = defaultReportsDir })

	r := &Report{
		Type: ReportCrash,
		Crash: &CrashData{
			Total: 3,
			Entries: []CrashEntry{
				{Timestamp: time.Now(), Error: "panic: nil pointer", UptimeMs: 5000, CrashNum: 1, BackoffMs: 1000},
			},
		},
	}

	id, err := SaveReport(r)
	if err != nil {
		t.Fatalf("SaveReport: %v", err)
	}

	raw, err := ReadReportRaw(id)
	if err != nil {
		t.Fatalf("ReadReportRaw: %v", err)
	}

	if !strings.Contains(raw, "# Thimble Report") {
		t.Error("expected markdown header")
	}

	if !strings.Contains(raw, "```json") {
		t.Error("expected JSON code block")
	}

	if !strings.Contains(raw, "panic: nil pointer") {
		t.Error("expected crash error in raw output")
	}
}

func TestListReports(t *testing.T) {
	dir := t.TempDir()
	ReportsDir = func() string { return dir }

	t.Cleanup(func() { ReportsDir = defaultReportsDir })

	// Empty list.
	reports, err := ListReports()
	if err != nil {
		t.Fatalf("ListReports (empty): %v", err)
	}

	if len(reports) != 0 {
		t.Errorf("expected 0 reports, got %d", len(reports))
	}

	// Save two reports.
	_, _ = SaveReport(&Report{Type: ReportDoctor, Doctor: &DoctorData{Summary: "ok"}})

	time.Sleep(10 * time.Millisecond)

	_, _ = SaveReport(&Report{Type: ReportCrash, Crash: &CrashData{Total: 1}})

	reports, err = ListReports()
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}

	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}

	// Newest first.
	if reports[0].Type != ReportCrash {
		t.Errorf("expected newest (crash) first, got %s", reports[0].Type)
	}
}

func TestDeleteReport(t *testing.T) {
	dir := t.TempDir()
	ReportsDir = func() string { return dir }

	t.Cleanup(func() { ReportsDir = defaultReportsDir })

	id, _ := SaveReport(&Report{Type: ReportStats, Stats: &StatsData{Sources: 10}})

	if err := DeleteReport(id); err != nil {
		t.Fatalf("DeleteReport: %v", err)
	}

	_, err := ReadReport(id)
	if err == nil {
		t.Error("expected error reading deleted report")
	}
}

func TestIssueCount(t *testing.T) {
	tests := []struct {
		name   string
		report Report
		want   int
	}{
		{
			name: "doctor with failures",
			report: Report{Doctor: &DoctorData{Checks: []Check{
				{Status: "pass"},
				{Status: "fail"},
				{Status: "warn"},
			}}},
			want: 2,
		},
		{
			name:   "crash report",
			report: Report{Crash: &CrashData{Total: 5}},
			want:   5,
		},
		{
			name:   "stats report (no issues)",
			report: Report{Stats: &StatsData{}},
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.report.IssueCount(); got != tt.want {
				t.Errorf("IssueCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRenderReportContainsInstructions(t *testing.T) {
	tests := []struct {
		name   string
		report *Report
		want   string
	}{
		{
			name:   "doctor instructions",
			report: &Report{ID: "test-1", Type: ReportDoctor, Doctor: &DoctorData{}},
			want:   "## Instructions",
		},
		{
			name:   "crash instructions",
			report: &Report{ID: "test-2", Type: ReportCrash, Crash: &CrashData{}},
			want:   "## Instructions",
		},
		{
			name:   "stats instructions",
			report: &Report{ID: "test-3", Type: ReportStats, Stats: &StatsData{}},
			want:   "## Instructions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderReport(tt.report)
			if !strings.Contains(got, tt.want) {
				t.Errorf("renderReport missing %q", tt.want)
			}
		})
	}
}

func TestListReportsNonExistentDir(t *testing.T) {
	ReportsDir = func() string { return filepath.Join(t.TempDir(), "nonexistent") }

	t.Cleanup(func() { ReportsDir = defaultReportsDir })

	reports, err := ListReports()
	if err != nil {
		t.Fatalf("ListReports on nonexistent dir: %v", err)
	}

	if reports != nil {
		t.Errorf("expected nil, got %v", reports)
	}
}
