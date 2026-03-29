package main

import (
	"database/sql"
	"slices"
	"strings"
	"testing"

	// Import the SQLite driver to ensure it's registered for checkSQLiteFTS5 coverage.
	_ "modernc.org/sqlite"
)

func TestCheckSQLiteFTS5_WithDriver(t *testing.T) {
	// Now that we've imported the sqlite driver, it should be registered.
	drivers := sql.Drivers()
	found := slices.Contains(drivers, "sqlite")

	if !found {
		t.Skip("sqlite driver not registered")
	}

	results := checkSQLiteFTS5()
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// With the driver loaded, both checks should pass.
	for _, r := range results {
		if r.status == "fail" {
			t.Errorf("%s: unexpected fail: %s", r.name, r.message)
		}

		if r.status == "pass" {
			if r.name == "SQLite" && !strings.Contains(r.message, "modernc") {
				t.Errorf("SQLite message = %q, should mention modernc", r.message)
			}
		}
	}
}

func TestCheckSQLiteFTS5_SQLiteVersion(t *testing.T) {
	// Verify we can query the sqlite version.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Skip("cannot open sqlite")
	}

	defer func() { _ = db.Close() }()

	var version string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&version); err != nil {
		t.Fatalf("version query: %v", err)
	}

	if version == "" {
		t.Error("sqlite version is empty")
	}
}

func TestCheckSQLiteFTS5_FTS5Available(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Skip("cannot open sqlite")
	}

	defer func() { _ = db.Close() }()

	_, err = db.Exec("CREATE VIRTUAL TABLE IF NOT EXISTS _test_fts5 USING fts5(content)")
	if err != nil {
		t.Fatalf("FTS5 not available: %v", err)
	}

	_, _ = db.Exec("DROP TABLE IF EXISTS _test_fts5")
}
