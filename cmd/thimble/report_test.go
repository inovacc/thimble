package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/thimble/internal/report"
)

func setupReportDir(t *testing.T) func() {
	t.Helper()

	tmp := t.TempDir()
	orig := report.ReportsDir
	report.ReportsDir = func() string { return tmp }

	return func() { report.ReportsDir = orig }
}

func TestReportListCmd_Empty(t *testing.T) {
	cleanup := setupReportDir(t)
	defer cleanup()

	buf := new(bytes.Buffer)
	reportListCmd.SetOut(buf)
	reportListCmd.SetErr(buf)

	if err := reportListCmd.RunE(reportListCmd, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}

	if !strings.Contains(buf.String(), "No reports found") {
		t.Errorf("expected 'No reports found', got %q", buf.String())
	}
}

func TestReportListCmd_WithReports(t *testing.T) {
	cleanup := setupReportDir(t)
	defer cleanup()

	// Save a test report.
	id, err := report.SaveReport(&report.Report{
		Type:      report.ReportDoctor,
		CreatedAt: time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC),
		Doctor: &report.DoctorData{
			Summary: "healthy",
			Checks: []report.Check{
				{Name: "test", Status: "pass", Message: "ok"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveReport error: %v", err)
	}

	buf := new(bytes.Buffer)
	reportListCmd.SetOut(buf)

	if err := reportListCmd.RunE(reportListCmd, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "doctor") {
		t.Errorf("expected 'doctor' in output, got %q", out)
	}

	if !strings.Contains(out, id) {
		t.Errorf("expected report ID %q in output, got %q", id, out)
	}
}

func TestReportShowCmd(t *testing.T) {
	cleanup := setupReportDir(t)
	defer cleanup()

	id, err := report.SaveReport(&report.Report{
		Type: report.ReportStats,
		Stats: &report.StatsData{
			Sources: 42,
			Chunks:  100,
		},
	})
	if err != nil {
		t.Fatalf("SaveReport error: %v", err)
	}

	buf := new(bytes.Buffer)
	reportShowCmd.SetOut(buf)

	if err := reportShowCmd.RunE(reportShowCmd, []string{id}); err != nil {
		t.Fatalf("RunE error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Thimble Report") {
		t.Errorf("expected 'Thimble Report' in output, got %q", out)
	}

	if !strings.Contains(out, "stats") {
		t.Errorf("expected 'stats' in output, got %q", out)
	}
}

func TestReportShowCmd_NotFound(t *testing.T) {
	cleanup := setupReportDir(t)
	defer cleanup()

	err := reportShowCmd.RunE(reportShowCmd, []string{"nonexistent-id"})
	if err == nil {
		t.Fatal("expected error for nonexistent report")
	}
}

func TestReportDeleteCmd(t *testing.T) {
	cleanup := setupReportDir(t)
	defer cleanup()

	id, err := report.SaveReport(&report.Report{
		Type: report.ReportDoctor,
		Doctor: &report.DoctorData{
			Summary: "healthy",
			Checks:  []report.Check{{Name: "x", Status: "pass", Message: "ok"}},
		},
	})
	if err != nil {
		t.Fatalf("SaveReport error: %v", err)
	}

	buf := new(bytes.Buffer)
	reportDeleteCmd.SetOut(buf)

	if err := reportDeleteCmd.RunE(reportDeleteCmd, []string{id}); err != nil {
		t.Fatalf("RunE error: %v", err)
	}

	if !strings.Contains(buf.String(), "Deleted report") {
		t.Errorf("expected 'Deleted report' in output, got %q", buf.String())
	}

	// Verify it's gone.
	_, err = report.ReadReportRaw(id)
	if err == nil {
		t.Error("expected error reading deleted report")
	}
}

func TestReportDeleteCmd_NotFound(t *testing.T) {
	cleanup := setupReportDir(t)
	defer cleanup()

	err := reportDeleteCmd.RunE(reportDeleteCmd, []string{"nonexistent-id"})
	if err == nil {
		t.Fatal("expected error for nonexistent report")
	}
}
