package main

import (
	"strings"
	"testing"
)

func TestCheckRuntimes(t *testing.T) {
	results := checkRuntimes()
	if len(results) == 0 {
		t.Fatal("checkRuntimes() returned no results")
	}

	// Last result should be the summary.
	last := results[len(results)-1]
	if last.name != "Runtimes" {
		t.Errorf("last result name = %q, want %q", last.name, "Runtimes")
	}

	if last.status != "pass" {
		t.Errorf("Runtimes status = %q, want %q", last.status, "pass")
	}

	if !strings.Contains(last.message, "available") {
		t.Errorf("Runtimes message = %q, want to contain 'available'", last.message)
	}
}

func TestCheckRuntimes_HasGo(t *testing.T) {
	results := checkRuntimes()

	var foundGo bool

	for _, r := range results {
		if r.name == "Go" {
			foundGo = true

			if r.status != "pass" {
				t.Errorf("Go runtime status = %q, want 'pass'", r.status)
			}
		}
	}

	if !foundGo {
		t.Error("Go runtime not found in checkRuntimes results")
	}
}

func TestCheckPlatform(t *testing.T) {
	result := checkPlatform()

	if result.name != "Platform" {
		t.Errorf("name = %q, want %q", result.name, "Platform")
	}

	if result.status != "pass" {
		t.Errorf("status = %q, want %q", result.status, "pass")
	}

	if result.message == "" {
		t.Error("platform message is empty")
	}
}

func TestCheckSQLiteFTS5(t *testing.T) {
	results := checkSQLiteFTS5()
	if len(results) == 0 {
		t.Fatal("checkSQLiteFTS5() returned no results")
	}

	// Should have SQLite and FTS5 results.
	names := make([]string, 0, len(results))
	for _, r := range results {
		names = append(names, r.name)
		// Status should be pass or warn (warn if driver not loaded in CLI context).
		if r.status != "pass" && r.status != "warn" {
			t.Errorf("%s: status = %q, want 'pass' or 'warn'", r.name, r.status)
		}
	}

	if len(names) < 2 {
		t.Errorf("expected at least 2 results (SQLite + FTS5), got %d", len(names))
	}
}

func TestCheckServer_SingleBinary(t *testing.T) {
	result := checkServer()

	if result.name != "Architecture" {
		t.Errorf("name = %q, want %q", result.name, "Architecture")
	}

	if result.status != "pass" {
		t.Errorf("status = %q, want 'pass'", result.status)
	}
}

func TestRunDoctor_NoError(t *testing.T) {
	// runDoctor should not return an error.
	err := runDoctor(nil, nil)
	if err != nil {
		t.Fatalf("runDoctor() error: %v", err)
	}
}
