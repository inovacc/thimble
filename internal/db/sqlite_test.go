package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAndClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	conn, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Verify we can execute a query.
	var result int
	if err := conn.QueryRow("SELECT 1").Scan(&result); err != nil {
		t.Fatalf("query: %v", err)
	}

	if result != 1 {
		t.Errorf("SELECT 1 = %d, want 1", result)
	}

	Close(conn)
}

func TestOpenCreatesFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "new.db")

	conn, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer Close(conn)

	// Force WAL creation by writing data.
	_, err = conn.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file should exist after Open")
	}
}

func TestOpenInvalidPath(t *testing.T) {
	// Try to open a db in a non-existent deeply nested directory.
	_, err := Open("/nonexistent/deeply/nested/path/test.db")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestDeleteFiles(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "del.db")

	// Create all three SQLite files.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.WriteFile(dbPath+suffix, []byte("x"), 0o644); err != nil {
			t.Fatalf("create %s: %v", dbPath+suffix, err)
		}
	}

	DeleteFiles(dbPath)

	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, err := os.Stat(dbPath + suffix); !os.IsNotExist(err) {
			t.Errorf("file %s should have been deleted", dbPath+suffix)
		}
	}
}

func TestDeleteFilesNonexistent(t *testing.T) {
	// Should not panic when files don't exist.
	DeleteFiles(filepath.Join(t.TempDir(), "nonexistent.db"))
}

func TestCloseNil(t *testing.T) {
	// Should not panic on nil.
	Close(nil)
}

func TestWALMode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "wal.db")

	conn, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer Close(conn)

	var journalMode string
	if err := conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}

	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}
}
