package main

import (
	"testing"

	"github.com/inovacc/thimble/internal/report"
)

func TestRunDoctor_WithReportFlag(t *testing.T) {
	// Override reports dir so we don't pollute real data.
	tmp := t.TempDir()
	orig := report.ReportsDir
	report.ReportsDir = func() string { return tmp }

	defer func() { report.ReportsDir = orig }()

	// Set and reset the global flag.
	flagDoctorReport = true

	defer func() { flagDoctorReport = false }()

	err := runDoctor(nil, nil)
	if err != nil {
		t.Fatalf("runDoctor() error: %v", err)
	}

	// Verify a report was saved.
	reports, err := report.ListReports()
	if err != nil {
		t.Fatalf("ListReports() error: %v", err)
	}

	if len(reports) == 0 {
		t.Fatal("expected at least 1 report after runDoctor --report")
	}

	r := reports[0]
	if r.Type != report.ReportDoctor {
		t.Errorf("report type = %q, want %q", r.Type, report.ReportDoctor)
	}

	if r.Doctor == nil {
		t.Fatal("report.Doctor is nil")
	}

	if r.Doctor.Summary == "" {
		t.Error("report.Doctor.Summary is empty")
	}

	if len(r.Doctor.Checks) == 0 {
		t.Error("report.Doctor.Checks is empty")
	}
}
