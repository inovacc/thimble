package session

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

// WorkspaceSessionMap manages OpenClaw workspace-to-session mappings.
// It provides stable session IDs for workspace keys so that sessions persist
// across gateway restarts.
type WorkspaceSessionMap struct {
	db *SessionDB
}

// NewWorkspaceSessionMap creates a WorkspaceSessionMap backed by the given SessionDB.
func NewWorkspaceSessionMap(db *SessionDB) *WorkspaceSessionMap {
	return &WorkspaceSessionMap{db: db}
}

// ResolveSession returns a stable session ID for a workspace key.
// If no mapping exists, a new session ID is generated from the workspace key
// and project directory, and the mapping is persisted.
func (w *WorkspaceSessionMap) ResolveSession(workspaceKey, projectDir string) (string, error) {
	var sessionID string

	err := w.db.db.QueryRow(
		"SELECT session_id FROM openclaw_session_map WHERE workspace_key = ?",
		workspaceKey,
	).Scan(&sessionID)
	if err == nil {
		return sessionID, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("query workspace session: %w", err)
	}

	// Generate deterministic session ID from workspace key + project dir.
	h := sha256.Sum256([]byte(workspaceKey + "\x00" + projectDir))
	sessionID = "ws-" + hex.EncodeToString(h[:8])

	_, err = w.db.db.Exec(
		"INSERT INTO openclaw_session_map (workspace_key, session_id, project_dir) VALUES (?, ?, ?)",
		workspaceKey, sessionID, projectDir,
	)
	if err != nil {
		return "", fmt.Errorf("insert workspace session: %w", err)
	}

	return sessionID, nil
}

// RenameSession updates the workspace key for an existing mapping.
// This is used when a workspace is re-keyed (e.g., after gateway restart)
// but should continue using the same session.
func (w *WorkspaceSessionMap) RenameSession(oldKey, newKey string) error {
	result, err := w.db.db.Exec(
		"UPDATE openclaw_session_map SET workspace_key = ? WHERE workspace_key = ?",
		newKey, oldKey,
	)
	if err != nil {
		return fmt.Errorf("rename workspace session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("workspace key %q not found", oldKey)
	}

	return nil
}
