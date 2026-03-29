package session

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/inovacc/thimble/internal/model"
)

// SessionExport is a JSON-serializable representation of a full session.
type SessionExport struct {
	Version        string                `json:"version"`
	ExportedAt     time.Time             `json:"exported_at"`
	SessionID      string                `json:"session_id"`
	Events         []model.StoredEvent   `json:"events"`
	Metadata       map[string]string     `json:"metadata"`
	ResumeSnapshot string                `json:"resume_snapshot,omitempty"`
}

// ExportSession exports all events, metadata, and resume snapshot for a session.
func (sdb *SessionDB) ExportSession(sessionID string) (*SessionExport, error) {
	events, err := sdb.GetEvents(sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("get events: %w", err)
	}

	meta, err := sdb.GetSessionStats(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session stats: %w", err)
	}

	metadata := make(map[string]string)
	if meta != nil {
		metadata["project_dir"] = meta.ProjectDir
		metadata["started_at"] = meta.StartedAt
		metadata["last_event_at"] = meta.LastEventAt
		metadata["event_count"] = fmt.Sprintf("%d", meta.EventCount)
		metadata["compact_count"] = fmt.Sprintf("%d", meta.CompactCount)
	}

	var resumeSnapshot string

	resume, err := sdb.GetResume(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get resume: %w", err)
	}

	if resume != nil {
		resumeSnapshot = resume.Snapshot
	}

	return &SessionExport{
		Version:        "1",
		ExportedAt:     time.Now().UTC(),
		SessionID:      sessionID,
		Events:         events,
		Metadata:       metadata,
		ResumeSnapshot: resumeSnapshot,
	}, nil
}

// ImportSession imports events from a SessionExport into the database,
// preserving timestamps and dedup hashes.
func (sdb *SessionDB) ImportSession(data *SessionExport) error {
	if data == nil {
		return fmt.Errorf("nil session export")
	}

	if data.SessionID == "" {
		return fmt.Errorf("empty session ID in export")
	}

	tx, err := sdb.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	// Ensure session meta exists.
	projectDir := data.Metadata["project_dir"]
	if _, err := tx.Exec(
		"INSERT OR IGNORE INTO session_meta (session_id, project_dir) VALUES (?, ?)",
		data.SessionID, projectDir,
	); err != nil {
		return fmt.Errorf("ensure session meta: %w", err)
	}

	// Insert events preserving original timestamps and hashes.
	for _, e := range data.Events {
		hash := e.DataHash
		if hash == "" {
			hash = dataHash(e.Data)
		}

		if _, err := tx.Exec(
			`INSERT INTO session_events (session_id, type, category, priority, data, source_hook, created_at, data_hash)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			data.SessionID, e.Type, e.Category, e.Priority, e.Data, e.SourceHook, e.CreatedAt, hash,
		); err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
	}

	// Update meta event count.
	if _, err := tx.Exec(
		"UPDATE session_meta SET event_count = ?, last_event_at = ? WHERE session_id = ?",
		len(data.Events), data.Metadata["last_event_at"], data.SessionID,
	); err != nil {
		return fmt.Errorf("update session meta: %w", err)
	}

	// Import resume snapshot if present.
	if data.ResumeSnapshot != "" {
		if _, err := tx.Exec(`
			INSERT INTO session_resume (session_id, snapshot, event_count)
			VALUES (?, ?, ?)
			ON CONFLICT(session_id) DO UPDATE SET
				snapshot = excluded.snapshot,
				event_count = excluded.event_count,
				created_at = datetime('now'),
				consumed = 0`,
			data.SessionID, data.ResumeSnapshot, len(data.Events),
		); err != nil {
			return fmt.Errorf("upsert resume: %w", err)
		}
	}

	return tx.Commit()
}

// ListSessions returns metadata for all sessions with event counts and last activity.
func (sdb *SessionDB) ListSessions() ([]model.SessionMeta, error) {
	rows, err := sdb.db.Query(`
		SELECT session_id, project_dir, started_at,
		       COALESCE(last_event_at, ''), event_count, compact_count
		FROM session_meta
		ORDER BY COALESCE(last_event_at, started_at) DESC`)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var results []model.SessionMeta

	for rows.Next() {
		var m model.SessionMeta
		if err := rows.Scan(&m.SessionID, &m.ProjectDir, &m.StartedAt,
			&m.LastEventAt, &m.EventCount, &m.CompactCount); err != nil {
			return nil, err
		}

		results = append(results, m)
	}

	return results, rows.Err()
}

// ExportJSON serializes a SessionExport to indented JSON bytes.
func ExportJSON(exp *SessionExport) ([]byte, error) {
	return json.MarshalIndent(exp, "", "  ")
}

// ImportJSON deserializes JSON bytes into a SessionExport.
func ImportJSON(data []byte) (*SessionExport, error) {
	var exp SessionExport
	if err := json.Unmarshal(data, &exp); err != nil {
		return nil, fmt.Errorf("unmarshal session export: %w", err)
	}

	return &exp, nil
}
