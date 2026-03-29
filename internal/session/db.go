// Package session implements the SessionDB — persistent per-project session tracking.
//
// Stores raw events captured by hooks during a Claude Code session,
// session metadata, and resume snapshots.
package session

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/thimble/internal/model"

	_ "modernc.org/sqlite"
)

// Constants matching the TypeScript implementation.
const (
	// MaxEventsPerSession before FIFO eviction kicks in.
	MaxEventsPerSession = 1000
	// DedupWindow is the number of recent events to check for deduplication.
	DedupWindow = 15
)

// SessionDB manages session events, metadata, and resume snapshots in SQLite.
type SessionDB struct {
	db     *sql.DB
	dbPath string
}

// New creates a SessionDB at the given path.
func New(dbPath string) (*SessionDB, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open session db: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping session db: %w", err)
	}

	sdb := &SessionDB{db: db, dbPath: dbPath}
	if err := sdb.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return sdb, nil
}

func (sdb *SessionDB) initSchema() error {
	_, err := sdb.db.Exec(`
		CREATE TABLE IF NOT EXISTS session_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			type TEXT NOT NULL,
			category TEXT NOT NULL,
			priority INTEGER NOT NULL DEFAULT 2,
			data TEXT NOT NULL,
			source_hook TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			data_hash TEXT NOT NULL DEFAULT ''
		);

		CREATE INDEX IF NOT EXISTS idx_session_events_session ON session_events(session_id);
		CREATE INDEX IF NOT EXISTS idx_session_events_type ON session_events(session_id, type);
		CREATE INDEX IF NOT EXISTS idx_session_events_priority ON session_events(session_id, priority);

		CREATE TABLE IF NOT EXISTS session_meta (
			session_id TEXT PRIMARY KEY,
			project_dir TEXT NOT NULL,
			started_at TEXT NOT NULL DEFAULT (datetime('now')),
			last_event_at TEXT,
			event_count INTEGER NOT NULL DEFAULT 0,
			compact_count INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS session_resume (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL UNIQUE,
			snapshot TEXT NOT NULL,
			event_count INTEGER NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			consumed INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS session_activity (
			session_id TEXT PRIMARY KEY,
			app_name TEXT NOT NULL DEFAULT '',
			project_dir TEXT NOT NULL DEFAULT '',
			last_used_at TEXT NOT NULL DEFAULT (datetime('now')),
			tool_calls INTEGER NOT NULL DEFAULT 0,
			queries INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);

		CREATE INDEX IF NOT EXISTS idx_session_activity_last_used ON session_activity(last_used_at);

		CREATE TABLE IF NOT EXISTS openclaw_session_map (
			workspace_key TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			project_dir TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		);
	`)

	return err
}

// Close closes the database connection.
func (sdb *SessionDB) Close() {
	if sdb.db != nil {
		_ = sdb.db.Close()
	}
}

// DBPath returns the database file path.
func (sdb *SessionDB) DBPath() string {
	return sdb.dbPath
}

// DB returns the underlying *sql.DB for direct queries.
func (sdb *SessionDB) DB() *sql.DB {
	return sdb.db
}

// dataHash computes a SHA256-based dedup hash (first 16 hex chars).
func dataHash(data string) string {
	h := sha256.Sum256([]byte(data))
	return strings.ToUpper(hex.EncodeToString(h[:8]))
}

// InsertEvent inserts a session event with deduplication and FIFO eviction.
//
// Deduplication: skips if the same type + data_hash appears in the
// last DedupWindow events for this session.
//
// Eviction: if session exceeds MaxEventsPerSession, evicts the
// lowest-priority (then oldest) event.
func (sdb *SessionDB) InsertEvent(sessionID string, event model.SessionEvent, sourceHook string) error {
	if sourceHook == "" {
		sourceHook = "PostToolUse"
	}

	hash := dataHash(event.Data)

	tx, err := sdb.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	// Deduplication check: same type + data_hash in last N events.
	var dup int

	err = tx.QueryRow(`
		SELECT 1 FROM (
			SELECT type, data_hash FROM session_events
			WHERE session_id = ? ORDER BY id DESC LIMIT ?
		) AS recent
		WHERE recent.type = ? AND recent.data_hash = ?
		LIMIT 1`, sessionID, DedupWindow, event.Type, hash).Scan(&dup)
	if err == nil {
		// Duplicate found — skip.
		return tx.Commit()
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// Enforce max events with FIFO eviction of lowest priority.
	var cnt int
	if err := tx.QueryRow("SELECT COUNT(*) FROM session_events WHERE session_id = ?", sessionID).Scan(&cnt); err != nil {
		return err
	}

	if cnt >= MaxEventsPerSession {
		if _, err := tx.Exec(`
			DELETE FROM session_events WHERE id = (
				SELECT id FROM session_events WHERE session_id = ?
				ORDER BY priority ASC, id ASC LIMIT 1
			)`, sessionID); err != nil {
			return err
		}
	}

	// Insert the event.
	if _, err := tx.Exec(
		"INSERT INTO session_events (session_id, type, category, priority, data, source_hook, data_hash) VALUES (?, ?, ?, ?, ?, ?, ?)",
		sessionID, event.Type, event.Category, event.Priority, event.Data, sourceHook, hash,
	); err != nil {
		return err
	}

	// Update meta if session exists.
	if _, err := tx.Exec(
		"UPDATE session_meta SET last_event_at = datetime('now'), event_count = event_count + 1 WHERE session_id = ?",
		sessionID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// GetEventsOpts configures event retrieval.
type GetEventsOpts struct {
	Type        string
	MinPriority int
	Limit       int
}

// GetEvents retrieves events for a session with optional filtering.
func (sdb *SessionDB) GetEvents(sessionID string, opts *GetEventsOpts) ([]model.StoredEvent, error) {
	limit := 1000
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	var (
		q    string
		args []any
	)

	switch {
	case opts != nil && opts.Type != "" && opts.MinPriority > 0:
		q = `SELECT id, session_id, type, category, priority, data, source_hook, created_at, data_hash
			FROM session_events WHERE session_id = ? AND type = ? AND priority >= ? ORDER BY id ASC LIMIT ?`
		args = []any{sessionID, opts.Type, opts.MinPriority, limit}
	case opts != nil && opts.Type != "":
		q = `SELECT id, session_id, type, category, priority, data, source_hook, created_at, data_hash
			FROM session_events WHERE session_id = ? AND type = ? ORDER BY id ASC LIMIT ?`
		args = []any{sessionID, opts.Type, limit}
	case opts != nil && opts.MinPriority > 0:
		q = `SELECT id, session_id, type, category, priority, data, source_hook, created_at, data_hash
			FROM session_events WHERE session_id = ? AND priority >= ? ORDER BY id ASC LIMIT ?`
		args = []any{sessionID, opts.MinPriority, limit}
	default:
		q = `SELECT id, session_id, type, category, priority, data, source_hook, created_at, data_hash
			FROM session_events WHERE session_id = ? ORDER BY id ASC LIMIT ?`
		args = []any{sessionID, limit}
	}

	rows, err := sdb.db.Query(q, args...)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var results []model.StoredEvent

	for rows.Next() {
		var e model.StoredEvent
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Type, &e.Category, &e.Priority,
			&e.Data, &e.SourceHook, &e.CreatedAt, &e.DataHash); err != nil {
			return nil, err
		}

		results = append(results, e)
	}

	return results, rows.Err()
}

// GetEventCount returns the total event count for a session.
func (sdb *SessionDB) GetEventCount(sessionID string) (int, error) {
	var cnt int

	err := sdb.db.QueryRow("SELECT COUNT(*) FROM session_events WHERE session_id = ?", sessionID).Scan(&cnt)

	return cnt, err
}

// EnsureSession creates a session metadata entry if it doesn't exist (idempotent).
func (sdb *SessionDB) EnsureSession(sessionID, projectDir string) error {
	_, err := sdb.db.Exec("INSERT OR IGNORE INTO session_meta (session_id, project_dir) VALUES (?, ?)", sessionID, projectDir)
	return err
}

// GetSessionStats returns session metadata.
func (sdb *SessionDB) GetSessionStats(sessionID string) (*model.SessionMeta, error) {
	var m model.SessionMeta

	err := sdb.db.QueryRow(
		"SELECT session_id, project_dir, started_at, COALESCE(last_event_at, ''), event_count, compact_count FROM session_meta WHERE session_id = ?",
		sessionID,
	).Scan(&m.SessionID, &m.ProjectDir, &m.StartedAt, &m.LastEventAt, &m.EventCount, &m.CompactCount)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil // nil,nil signals "not found"
	}

	if err != nil {
		return nil, err
	}

	return &m, nil
}

// IncrementCompactCount increments the compact_count for a session.
func (sdb *SessionDB) IncrementCompactCount(sessionID string) error {
	_, err := sdb.db.Exec("UPDATE session_meta SET compact_count = compact_count + 1 WHERE session_id = ?", sessionID)
	return err
}

// UpsertResume upserts a resume snapshot for a session. Resets consumed flag on update.
func (sdb *SessionDB) UpsertResume(sessionID, snapshot string, eventCount int) error {
	_, err := sdb.db.Exec(`
		INSERT INTO session_resume (session_id, snapshot, event_count)
		VALUES (?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			snapshot = excluded.snapshot,
			event_count = excluded.event_count,
			created_at = datetime('now'),
			consumed = 0`,
		sessionID, snapshot, eventCount)

	return err
}

// GetResume retrieves the resume snapshot for a session.
func (sdb *SessionDB) GetResume(sessionID string) (*model.ResumeRow, error) {
	var r model.ResumeRow

	err := sdb.db.QueryRow(
		"SELECT snapshot, event_count, consumed FROM session_resume WHERE session_id = ?",
		sessionID,
	).Scan(&r.Snapshot, &r.EventCount, &r.Consumed)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil // nil,nil signals "not found"
	}

	if err != nil {
		return nil, err
	}

	return &r, nil
}

// MarkResumeConsumed marks the resume snapshot as consumed.
func (sdb *SessionDB) MarkResumeConsumed(sessionID string) error {
	_, err := sdb.db.Exec("UPDATE session_resume SET consumed = 1 WHERE session_id = ?", sessionID)
	return err
}

// DeleteSession removes all data for a session (events, meta, resume).
func (sdb *SessionDB) DeleteSession(sessionID string) error {
	tx, err := sdb.db.Begin()
	if err != nil {
		return err
	}

	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("DELETE FROM session_events WHERE session_id = ?", sessionID); err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM session_resume WHERE session_id = ?", sessionID); err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM session_meta WHERE session_id = ?", sessionID); err != nil {
		return err
	}

	return tx.Commit()
}

// TouchActivity records a tool call or query for the given session, updating last_used_at.
func (sdb *SessionDB) TouchActivity(sessionID, appName, projectDir string, isQuery bool) error {
	col := "tool_calls"
	if isQuery {
		col = "queries"
	}

	_, err := sdb.db.Exec(fmt.Sprintf(`
		INSERT INTO session_activity (session_id, app_name, project_dir, last_used_at, %s)
		VALUES (?, ?, ?, datetime('now'), 1)
		ON CONFLICT(session_id) DO UPDATE SET
			last_used_at = datetime('now'),
			%s = %s + 1`, col, col, col),
		sessionID, appName, projectDir)

	return err
}

// GetActivity returns the activity row for a session.
func (sdb *SessionDB) GetActivity(sessionID string) (*model.SessionActivity, error) {
	var a model.SessionActivity

	err := sdb.db.QueryRow(
		"SELECT session_id, app_name, project_dir, last_used_at, tool_calls, queries, created_at FROM session_activity WHERE session_id = ?",
		sessionID,
	).Scan(&a.SessionID, &a.AppName, &a.ProjectDir, &a.LastUsedAt, &a.ToolCalls, &a.Queries, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil,nil signals "not found"
	}

	if err != nil {
		return nil, err
	}

	return &a, nil
}

// EvictStaleSessions removes sessions (events, meta, resume, activity) that
// haven't been used in maxAgeDays. Returns the count evicted.
func (sdb *SessionDB) EvictStaleSessions(maxAgeDays int) (int, error) {
	if maxAgeDays <= 0 {
		maxAgeDays = 7
	}

	negDays := fmt.Sprintf("-%d", maxAgeDays)

	rows, err := sdb.db.Query(
		"SELECT session_id FROM session_activity WHERE last_used_at < datetime('now', ? || ' days')", negDays)
	if err != nil {
		return 0, err
	}

	defer func() { _ = rows.Close() }()

	var ids []string

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, id := range ids {
		if err := sdb.DeleteSession(id); err != nil {
			return 0, err
		}

		// Also remove from activity table.
		if _, err := sdb.db.Exec("DELETE FROM session_activity WHERE session_id = ?", id); err != nil {
			return 0, err
		}
	}

	return len(ids), nil
}

// CleanupOldSessions removes sessions older than maxAgeDays. Returns the count deleted.
func (sdb *SessionDB) CleanupOldSessions(maxAgeDays int) (int, error) {
	if maxAgeDays <= 0 {
		maxAgeDays = 7
	}

	negDays := fmt.Sprintf("-%d", maxAgeDays)

	rows, err := sdb.db.Query("SELECT session_id FROM session_meta WHERE started_at < datetime('now', ? || ' days')", negDays)
	if err != nil {
		return 0, err
	}

	defer func() { _ = rows.Close() }()

	var ids []string

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, id := range ids {
		if err := sdb.DeleteSession(id); err != nil {
			return 0, err
		}
	}

	return len(ids), nil
}

// ListSessionIDs returns all session IDs from session_meta, ordered by most recent first.
func (sdb *SessionDB) ListSessionIDs() ([]string, error) {
	rows, err := sdb.db.Query("SELECT session_id FROM session_meta ORDER BY started_at DESC")
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var ids []string

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// ErrorCount returns the number of events with type "error" for the given session.
func (sdb *SessionDB) ErrorCount(sessionID string) (int, error) {
	var cnt int

	err := sdb.db.QueryRow(
		"SELECT COUNT(*) FROM session_events WHERE session_id = ? AND type = 'error'",
		sessionID,
	).Scan(&cnt)

	return cnt, err
}

// EventsByType returns event counts grouped by type for the given session.
func (sdb *SessionDB) EventsByType(sessionID string) (map[string]int, error) {
	rows, err := sdb.db.Query(
		"SELECT type, COUNT(*) FROM session_events WHERE session_id = ? GROUP BY type ORDER BY COUNT(*) DESC",
		sessionID,
	)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	result := make(map[string]int)

	for rows.Next() {
		var (
			typ string
			cnt int
		)

		if err := rows.Scan(&typ, &cnt); err != nil {
			return nil, err
		}

		result[typ] = cnt
	}

	return result, rows.Err()
}

// TopTools returns the most-called tools by name for the given session, limited to `limit` results.
// It extracts tool names from event data JSON using json_extract.
func (sdb *SessionDB) TopTools(sessionID string, limit int) ([]model.ToolCount, error) {
	if limit <= 0 {
		limit = 10
	}

	// The data field for tool_call events contains JSON with a "tool" key.
	// Use json_valid to skip non-JSON rows, then json_extract to pull tool names.
	rows, err := sdb.db.Query(`
		SELECT json_extract(data, '$.tool') AS tool_name, COUNT(*) AS cnt
		FROM session_events
		WHERE session_id = ? AND json_valid(data) AND json_extract(data, '$.tool') IS NOT NULL
		GROUP BY tool_name
		ORDER BY cnt DESC
		LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var results []model.ToolCount

	for rows.Next() {
		var tc model.ToolCount

		if err := rows.Scan(&tc.Name, &tc.Count); err != nil {
			return nil, err
		}

		results = append(results, tc)
	}

	return results, rows.Err()
}

// SessionDuration returns the duration between the first and last event for the given session.
// Returns 0 if no events exist.
func (sdb *SessionDB) SessionDuration(sessionID string) (time.Duration, error) {
	var first, last sql.NullString

	err := sdb.db.QueryRow(`
		SELECT MIN(created_at), MAX(created_at)
		FROM session_events
		WHERE session_id = ?`, sessionID).Scan(&first, &last)
	if err != nil {
		return 0, err
	}

	if !first.Valid || !last.Valid {
		return 0, nil
	}

	const layout = "2006-01-02 15:04:05"

	t1, err := time.Parse(layout, first.String)
	if err != nil {
		return 0, fmt.Errorf("parse first event time: %w", err)
	}

	t2, err := time.Parse(layout, last.String)
	if err != nil {
		return 0, fmt.Errorf("parse last event time: %w", err)
	}

	return t2.Sub(t1), nil
}
