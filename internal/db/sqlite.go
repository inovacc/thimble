// Package db provides SQLite infrastructure for thimble databases.
package db

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// Open opens a SQLite database with WAL mode and NORMAL synchronous pragma.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}

	// Verify connection works.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite %s: %w", path, err)
	}

	return db, nil
}

// DeleteFiles removes all three SQLite files for a given db path (main, WAL, SHM).
func DeleteFiles(dbPath string) {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(dbPath + suffix)
	}
}

// Close safely closes a database connection, ignoring errors.
func Close(db *sql.DB) {
	if db != nil {
		_ = db.Close()
	}
}
