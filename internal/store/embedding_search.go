package store

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/inovacc/thimble/internal/model"
)

// encodeFloat64s encodes a float64 slice as a little-endian binary blob.
func encodeFloat64s(v []float64) []byte {
	buf := make([]byte, len(v)*8)
	for i, f := range v {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(f))
	}

	return buf
}

// decodeFloat64s decodes a little-endian binary blob into a float64 slice.
func decodeFloat64s(b []byte) []float64 {
	n := len(b) / 8

	v := make([]float64, n)
	for i := range n {
		v[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[i*8:]))
	}

	return v
}

// vectorCosineSimilarity computes cosine similarity between two dense float64 vectors.
func vectorCosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// EmbeddingSearch performs vector similarity search using stored embeddings.
// Falls back to SemanticSearch (TF-IDF) if no embeddings are available or provider is nil.
func (cs *ContentStore) EmbeddingSearch(ctx context.Context, query string, limit int, source string, provider *EmbeddingProvider) ([]model.SearchResult, error) {
	if provider == nil {
		return cs.SemanticSearch(query, limit, source)
	}

	if limit <= 0 {
		limit = 5
	}

	// Embed the query.
	queryVec, err := provider.Embed(ctx, query)
	if err != nil {
		// On embedding failure, fall back to TF-IDF.
		return cs.SemanticSearch(query, limit, source)
	}

	// Load chunks with stored embeddings.
	var (
		sqlQuery string
		args     []any
	)

	if source != "" {
		sqlQuery = `SELECT c.rowid, c.title, c.content, c.content_type, s.label, ce.embedding
			FROM chunks c
			JOIN sources s ON s.id = c.source_id
			JOIN chunk_embeddings ce ON ce.chunk_rowid = c.rowid
			WHERE s.label LIKE ?
			ORDER BY c.rowid`
		args = []any{"%" + source + "%"}
	} else {
		sqlQuery = `SELECT c.rowid, c.title, c.content, c.content_type, s.label, ce.embedding
			FROM chunks c
			JOIN sources s ON s.id = c.source_id
			JOIN chunk_embeddings ce ON ce.chunk_rowid = c.rowid
			ORDER BY c.rowid`
	}

	rows, err := cs.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query chunk embeddings: %w", err)
	}

	defer func() { _ = rows.Close() }()

	type scored struct {
		title       string
		content     string
		contentType string
		source      string
		score       float64
	}

	var scoredDocs []scored

	for rows.Next() {
		var (
			rowid       int64
			title       string
			content     string
			contentType string
			src         string
			blob        []byte
		)

		if err := rows.Scan(&rowid, &title, &content, &contentType, &src, &blob); err != nil {
			return nil, fmt.Errorf("scan chunk embedding: %w", err)
		}

		chunkVec := decodeFloat64s(blob)
		sim := vectorCosineSimilarity(queryVec, chunkVec)

		if sim > 0.01 {
			scoredDocs = append(scoredDocs, scored{
				title:       title,
				content:     content,
				contentType: contentType,
				source:      src,
				score:       sim,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// If no embeddings stored, fall back to TF-IDF.
	if len(scoredDocs) == 0 {
		return cs.SemanticSearch(query, limit, source)
	}

	// Sort by score descending.
	for i := range scoredDocs {
		for j := i + 1; j < len(scoredDocs); j++ {
			if scoredDocs[j].score > scoredDocs[i].score {
				scoredDocs[i], scoredDocs[j] = scoredDocs[j], scoredDocs[i]
			}
		}
	}

	if len(scoredDocs) > limit {
		scoredDocs = scoredDocs[:limit]
	}

	results := make([]model.SearchResult, len(scoredDocs))
	for i, sd := range scoredDocs {
		results[i] = model.SearchResult{
			Title:       sd.title,
			Content:     sd.content,
			Source:      sd.source,
			Rank:        sd.score,
			ContentType: sd.contentType,
			MatchLayer:  "embedding",
		}
	}

	return results, nil
}
