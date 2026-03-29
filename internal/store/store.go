// Package store implements the ContentStore — an FTS5-based knowledge base
// for indexing and searching content chunks.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/inovacc/thimble/internal/model"

	_ "modernc.org/sqlite"
)

// MaxChunkBytes limits chunk size. Oversized sections are split at paragraph boundaries.
const MaxChunkBytes = 4096

// ContentStore manages an FTS5 knowledge base backed by SQLite.
type ContentStore struct {
	db       *sql.DB
	dbPath   string
	embedder *EmbeddingProvider
}

// New creates a ContentStore at the given path.
func New(dbPath string) (*ContentStore, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open store db: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping store db: %w", err)
	}

	cs := &ContentStore{db: db, dbPath: dbPath}
	if err := cs.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return cs, nil
}

func (cs *ContentStore) initSchema() error {
	_, err := cs.db.Exec(`
		CREATE TABLE IF NOT EXISTS sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT NOT NULL,
			chunk_count INTEGER NOT NULL DEFAULT 0,
			code_chunk_count INTEGER NOT NULL DEFAULT 0,
			indexed_at TEXT NOT NULL DEFAULT (datetime('now'))
		);

		CREATE VIRTUAL TABLE IF NOT EXISTS chunks USING fts5(
			title,
			content,
			source_id UNINDEXED,
			content_type UNINDEXED,
			tokenize='porter unicode61'
		);

		CREATE VIRTUAL TABLE IF NOT EXISTS chunks_trigram USING fts5(
			title,
			content,
			source_id UNINDEXED,
			content_type UNINDEXED,
			tokenize='trigram'
		);

		CREATE TABLE IF NOT EXISTS vocabulary (
			word TEXT PRIMARY KEY
		);

		CREATE TABLE IF NOT EXISTS chunk_embeddings (
			chunk_rowid INTEGER PRIMARY KEY,
			embedding BLOB NOT NULL
		);
	`)

	return err
}

// Close closes the database connection.
func (cs *ContentStore) Close() {
	if cs.db != nil {
		_ = cs.db.Close()
	}
}

// DBPath returns the database file path.
func (cs *ContentStore) DBPath() string {
	return cs.dbPath
}

// SetEmbeddingProvider configures an optional embedding provider for vector search.
// When set, insertChunks stores embeddings and SearchWithFallback uses vector
// similarity instead of TF-IDF as the final search layer.
func (cs *ContentStore) SetEmbeddingProvider(p *EmbeddingProvider) {
	cs.embedder = p
}

// Index indexes markdown content into the knowledge base.
// When content exceeds chunkStrategyThreshold (2KB) and the configured
// ChunkStrategy (via THIMBLE_CHUNK_STRATEGY) is not "none", content is
// first split using ChunkContent before being passed to insertChunks.
// Small content and "none" strategy fall through to the original
// chunkMarkdown path for backward compatibility.
func (cs *ContentStore) Index(content, label string) (model.IndexResult, error) {
	if content == "" {
		return cs.insertChunks(nil, label, "")
	}

	strategy := defaultChunkStrategy()

	// Use strategy-based chunking for large content; keep original path for
	// small content or when strategy is "none".
	if strategy != ChunkNone && len(content) >= chunkStrategyThreshold {
		parts := ChunkContent(content, strategy, MaxChunkBytes)
		chunks := make([]chunk, len(parts))

		for i, part := range parts {
			hasCode := strings.Contains(part, "```")
			title := chunkTitle(part, i+1)
			chunks[i] = chunk{title: title, content: part, hasCode: hasCode}
		}

		return cs.insertChunks(chunks, label, content)
	}

	chunks := chunkMarkdown(content, MaxChunkBytes)

	return cs.insertChunks(chunks, label, content)
}

// chunkTitle derives a title from a content chunk. If the chunk starts with a
// markdown heading, that heading is used; otherwise a numbered fallback is returned.
func chunkTitle(content string, index int) string {
	firstLine := content
	if nl := strings.IndexByte(content, '\n'); nl > 0 {
		firstLine = content[:nl]
	}

	firstLine = strings.TrimSpace(firstLine)

	if m := headingRe.FindStringSubmatch(firstLine); m != nil {
		return strings.TrimSpace(m[2])
	}

	if len(firstLine) > 80 {
		firstLine = firstLine[:80]
	}

	if firstLine != "" {
		return firstLine
	}

	return fmt.Sprintf("Chunk %d", index)
}

// IndexPlainText indexes plain-text output by splitting into fixed-size line groups.
func (cs *ContentStore) IndexPlainText(content, source string, linesPerChunk int) (model.IndexResult, error) {
	if linesPerChunk <= 0 {
		linesPerChunk = 20
	}

	if strings.TrimSpace(content) == "" {
		return cs.insertChunks(nil, source, "")
	}

	raw := chunkPlainText(content, linesPerChunk)

	chunks := make([]chunk, len(raw))
	for i, c := range raw {
		chunks[i] = chunk{title: c.title, content: c.content, hasCode: false}
	}

	return cs.insertChunks(chunks, source, content)
}

// IndexJSON indexes JSON content by walking the object tree.
func (cs *ContentStore) IndexJSON(content, source string) (model.IndexResult, error) {
	if strings.TrimSpace(content) == "" {
		return cs.IndexPlainText("", source, 20)
	}

	var parsed any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return cs.IndexPlainText(content, source, 20)
	}

	var chunks []chunk
	walkJSON(parsed, nil, &chunks, MaxChunkBytes)

	if len(chunks) == 0 {
		return cs.IndexPlainText(content, source, 20)
	}

	return cs.insertChunks(chunks, source, content)
}

// insertChunks atomically dedup-deletes the previous source with the same label,
// then inserts all chunks into both FTS5 tables within a transaction.
func (cs *ContentStore) insertChunks(chunks []chunk, label, text string) (model.IndexResult, error) {
	codeChunks := 0

	for _, c := range chunks {
		if c.hasCode {
			codeChunks++
		}
	}

	tx, err := cs.db.Begin()
	if err != nil {
		return model.IndexResult{}, fmt.Errorf("begin tx: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	// Dedup: delete previous source with same label.
	// FTS5 tables require DELETE by rowid — UNINDEXED columns can't be used in WHERE.
	if _, err := tx.Exec("DELETE FROM chunks WHERE rowid IN (SELECT chunks.rowid FROM chunks JOIN sources ON sources.id = chunks.source_id WHERE sources.label = ?)", label); err != nil {
		return model.IndexResult{}, err
	}

	if _, err := tx.Exec("DELETE FROM chunks_trigram WHERE rowid IN (SELECT chunks_trigram.rowid FROM chunks_trigram JOIN sources ON sources.id = chunks_trigram.source_id WHERE sources.label = ?)", label); err != nil {
		return model.IndexResult{}, err
	}

	// Delete old embeddings for chunks belonging to this source.
	if _, err := tx.Exec("DELETE FROM chunk_embeddings WHERE chunk_rowid IN (SELECT chunks.rowid FROM chunks JOIN sources ON sources.id = chunks.source_id WHERE sources.label = ?)", label); err != nil {
		return model.IndexResult{}, err
	}

	if _, err := tx.Exec("DELETE FROM sources WHERE label = ?", label); err != nil {
		return model.IndexResult{}, err
	}

	var sourceID int64

	if len(chunks) == 0 {
		res, err := tx.Exec("INSERT INTO sources (label, chunk_count, code_chunk_count) VALUES (?, 0, 0)", label)
		if err != nil {
			return model.IndexResult{}, err
		}

		sourceID, _ = res.LastInsertId()
	} else {
		res, err := tx.Exec("INSERT INTO sources (label, chunk_count, code_chunk_count) VALUES (?, ?, ?)", label, len(chunks), codeChunks)
		if err != nil {
			return model.IndexResult{}, err
		}

		sourceID, _ = res.LastInsertId()

		type insertedChunk struct {
			rowid int64
			text  string
		}

		var inserted []insertedChunk

		for _, c := range chunks {
			ct := "prose"
			if c.hasCode {
				ct = "code"
			}

			res2, err := tx.Exec("INSERT INTO chunks (title, content, source_id, content_type) VALUES (?, ?, ?, ?)", c.title, c.content, sourceID, ct)
			if err != nil {
				return model.IndexResult{}, err
			}

			if _, err := tx.Exec("INSERT INTO chunks_trigram (title, content, source_id, content_type) VALUES (?, ?, ?, ?)", c.title, c.content, sourceID, ct); err != nil {
				return model.IndexResult{}, err
			}

			if cs.embedder != nil {
				rid, _ := res2.LastInsertId()
				inserted = append(inserted, insertedChunk{rowid: rid, text: c.title + " " + c.content})
			}
		}

		// Compute and store embeddings if provider is configured.
		if cs.embedder != nil && len(inserted) > 0 {
			texts := make([]string, len(inserted))
			for i, ic := range inserted {
				texts[i] = ic.text
			}

			vecs, err := cs.embedder.EmbedBatch(context.Background(), texts)
			if err != nil {
				// Log but don't fail — embeddings are optional.
				slog.Warn("embedding failed during indexing, chunks stored without embeddings", "error", err)
			} else {
				for i, ic := range inserted {
					blob := encodeFloat64s(vecs[i])
					if _, err := tx.Exec("INSERT INTO chunk_embeddings (chunk_rowid, embedding) VALUES (?, ?)", ic.rowid, blob); err != nil {
						return model.IndexResult{}, fmt.Errorf("store embedding: %w", err)
					}
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return model.IndexResult{}, fmt.Errorf("commit: %w", err)
	}

	if text != "" {
		cs.extractAndStoreVocabulary(text)
	}

	return model.IndexResult{
		SourceID:    sourceID,
		Label:       label,
		TotalChunks: len(chunks),
		CodeChunks:  codeChunks,
	}, nil
}

// ListSources returns all indexed sources.
func (cs *ContentStore) ListSources() ([]struct {
	Label      string
	ChunkCount int
}, error) {
	rows, err := cs.db.Query("SELECT label, chunk_count FROM sources ORDER BY id DESC")
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var result []struct {
		Label      string
		ChunkCount int
	}
	for rows.Next() {
		var s struct {
			Label      string
			ChunkCount int
		}
		if err := rows.Scan(&s.Label, &s.ChunkCount); err != nil {
			return nil, err
		}

		result = append(result, s)
	}

	return result, rows.Err()
}

// GetChunksBySource returns all chunks for a source ID (bypasses FTS5 MATCH).
func (cs *ContentStore) GetChunksBySource(sourceID int64) ([]model.SearchResult, error) {
	rows, err := cs.db.Query(`
		SELECT c.title, c.content, c.content_type, s.label
		FROM chunks c
		JOIN sources s ON s.id = c.source_id
		WHERE c.source_id = ?
		ORDER BY c.rowid`, sourceID)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var results []model.SearchResult

	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.Title, &r.Content, &r.ContentType, &r.Source); err != nil {
			return nil, err
		}

		results = append(results, r)
	}

	return results, rows.Err()
}

// GetStats returns aggregate statistics.
func (cs *ContentStore) GetStats() (model.StoreStats, error) {
	var stats model.StoreStats

	err := cs.db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM sources),
			(SELECT COUNT(*) FROM chunks),
			(SELECT COUNT(*) FROM chunks WHERE content_type = 'code')
	`).Scan(&stats.Sources, &stats.Chunks, &stats.CodeChunks)

	return stats, err
}
