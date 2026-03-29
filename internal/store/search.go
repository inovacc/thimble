package store

import (
	"context"
	"regexp"
	"strings"

	"github.com/inovacc/thimble/internal/model"
)

// sanitizeQuery cleans a query string for FTS5 Porter search.
func sanitizeQuery(query, mode string) string {
	re := regexp.MustCompile(`['"(){}\[\]*:^~]`)
	cleaned := re.ReplaceAllString(query, " ")
	words := strings.Fields(cleaned)

	var filtered []string

	reserved := map[string]bool{"AND": true, "OR": true, "NOT": true, "NEAR": true}
	for _, w := range words {
		if !reserved[strings.ToUpper(w)] && len(w) > 0 {
			filtered = append(filtered, `"`+w+`"`)
		}
	}

	if len(filtered) == 0 {
		return `""`
	}

	sep := " "
	if mode == "OR" {
		sep = " OR "
	}

	return strings.Join(filtered, sep)
}

// sanitizeTrigramQuery cleans a query string for FTS5 trigram search.
func sanitizeTrigramQuery(query, mode string) string {
	re := regexp.MustCompile(`["'(){}\[\]*:^~]`)

	cleaned := strings.TrimSpace(re.ReplaceAllString(query, ""))
	if len(cleaned) < 3 {
		return ""
	}

	words := strings.Fields(cleaned)

	var filtered []string

	for _, w := range words {
		if len(w) >= 3 {
			filtered = append(filtered, `"`+w+`"`)
		}
	}

	if len(filtered) == 0 {
		return ""
	}

	sep := " "
	if mode == "OR" {
		sep = " OR "
	}

	return strings.Join(filtered, sep)
}

// Search performs a Porter-stemmed FTS5 search.
func (cs *ContentStore) Search(query string, limit int, source, mode string) ([]model.SearchResult, error) {
	if limit <= 0 {
		limit = 3
	}

	if mode == "" {
		mode = "AND"
	}

	sanitized := sanitizeQuery(query, mode)

	var (
		q    string
		args []any
	)

	if source != "" {
		q = `SELECT chunks.title, chunks.content, chunks.content_type, sources.label,
			bm25(chunks, 2.0, 1.0) AS rank,
			highlight(chunks, 1, char(2), char(3)) AS highlighted
			FROM chunks JOIN sources ON sources.id = chunks.source_id
			WHERE chunks MATCH ? AND sources.label LIKE ?
			ORDER BY rank LIMIT ?`
		args = []any{sanitized, "%" + source + "%", limit}
	} else {
		q = `SELECT chunks.title, chunks.content, chunks.content_type, sources.label,
			bm25(chunks, 2.0, 1.0) AS rank,
			highlight(chunks, 1, char(2), char(3)) AS highlighted
			FROM chunks JOIN sources ON sources.id = chunks.source_id
			WHERE chunks MATCH ?
			ORDER BY rank LIMIT ?`
		args = []any{sanitized, limit}
	}

	return cs.queryResults(q, args, "porter")
}

// SearchTrigram performs a trigram-based FTS5 search (layer 2).
func (cs *ContentStore) SearchTrigram(query string, limit int, source, mode string) ([]model.SearchResult, error) {
	if limit <= 0 {
		limit = 3
	}

	if mode == "" {
		mode = "AND"
	}

	sanitized := sanitizeTrigramQuery(query, mode)
	if sanitized == "" {
		return nil, nil
	}

	var (
		q    string
		args []any
	)

	if source != "" {
		q = `SELECT chunks_trigram.title, chunks_trigram.content, chunks_trigram.content_type,
			sources.label, bm25(chunks_trigram, 2.0, 1.0) AS rank,
			highlight(chunks_trigram, 1, char(2), char(3)) AS highlighted
			FROM chunks_trigram JOIN sources ON sources.id = chunks_trigram.source_id
			WHERE chunks_trigram MATCH ? AND sources.label LIKE ?
			ORDER BY rank LIMIT ?`
		args = []any{sanitized, "%" + source + "%", limit}
	} else {
		q = `SELECT chunks_trigram.title, chunks_trigram.content, chunks_trigram.content_type,
			sources.label, bm25(chunks_trigram, 2.0, 1.0) AS rank,
			highlight(chunks_trigram, 1, char(2), char(3)) AS highlighted
			FROM chunks_trigram JOIN sources ON sources.id = chunks_trigram.source_id
			WHERE chunks_trigram MATCH ?
			ORDER BY rank LIMIT ?`
		args = []any{sanitized, limit}
	}

	return cs.queryResults(q, args, "trigram")
}

// FuzzyCorrect attempts to correct a misspelled query word using the vocabulary table.
func (cs *ContentStore) FuzzyCorrect(query string) string {
	word := strings.ToLower(strings.TrimSpace(query))
	if len(word) < 3 {
		return ""
	}

	maxDist := maxEditDistance(len(word))
	minLen := len(word) - maxDist
	maxLen := len(word) + maxDist

	rows, err := cs.db.Query("SELECT word FROM vocabulary WHERE length(word) BETWEEN ? AND ?", minLen, maxLen)
	if err != nil {
		return ""
	}

	defer func() { _ = rows.Close() }()

	bestWord := ""
	bestDist := maxDist + 1

	for rows.Next() {
		var candidate string
		if err := rows.Scan(&candidate); err != nil {
			continue
		}

		if candidate == word {
			return "" // exact match — no correction needed
		}

		dist := levenshtein(word, candidate)
		if dist < bestDist {
			bestDist = dist
			bestWord = candidate
		}
	}

	if err := rows.Err(); err != nil {
		return ""
	}

	if bestDist <= maxDist {
		return bestWord
	}

	return ""
}

// SearchWithFallback implements the 4-layer fallback search chain:
// 1a. Porter AND → 1b. Porter OR → 2a. Trigram AND → 2b. Trigram OR → 3. Fuzzy correction + retry
func (cs *ContentStore) SearchWithFallback(query string, limit int, source string) ([]model.SearchResult, error) {
	if limit <= 0 {
		limit = 3
	}

	// Layer 1a: Porter AND
	results, err := cs.Search(query, limit, source, "AND")
	if err != nil {
		return nil, err
	}

	if len(results) > 0 {
		return setMatchLayer(results, "porter"), nil
	}

	// Layer 1b: Porter OR
	results, err = cs.Search(query, limit, source, "OR")
	if err != nil {
		return nil, err
	}

	if len(results) > 0 {
		return setMatchLayer(results, "porter"), nil
	}

	// Layer 2a: Trigram AND
	results, err = cs.SearchTrigram(query, limit, source, "AND")
	if err != nil {
		return nil, err
	}

	if len(results) > 0 {
		return setMatchLayer(results, "trigram"), nil
	}

	// Layer 2b: Trigram OR
	results, err = cs.SearchTrigram(query, limit, source, "OR")
	if err != nil {
		return nil, err
	}

	if len(results) > 0 {
		return setMatchLayer(results, "trigram"), nil
	}

	// Layer 3: Fuzzy correction + re-search
	words := strings.Fields(strings.ToLower(strings.TrimSpace(query)))

	var significant []string

	for _, w := range words {
		if len(w) >= 3 {
			significant = append(significant, w)
		}
	}

	original := strings.Join(significant, " ")

	correctedWords := make([]string, len(significant))
	for i, w := range significant {
		if corrected := cs.FuzzyCorrect(w); corrected != "" {
			correctedWords[i] = corrected
		} else {
			correctedWords[i] = w
		}
	}

	correctedQuery := strings.Join(correctedWords, " ")

	if correctedQuery != original {
		for _, m := range []string{"AND", "OR"} {
			results, err = cs.Search(correctedQuery, limit, source, m)
			if err != nil {
				return nil, err
			}

			if len(results) > 0 {
				return setMatchLayer(results, "fuzzy"), nil
			}
		}

		for _, m := range []string{"AND", "OR"} {
			results, err = cs.SearchTrigram(correctedQuery, limit, source, m)
			if err != nil {
				return nil, err
			}

			if len(results) > 0 {
				return setMatchLayer(results, "fuzzy"), nil
			}
		}
	}

	// Layer 4: Semantic — embedding vector search (if configured) or TF-IDF cosine similarity.
	if cs.embedder != nil {
		results, err = cs.EmbeddingSearch(context.Background(), query, limit, source, cs.embedder)
	} else {
		results, err = cs.SemanticSearch(query, limit, source)
	}

	if err != nil {
		return nil, err
	}

	if len(results) > 0 {
		return results, nil // matchLayer set by EmbeddingSearch or SemanticSearch
	}

	return nil, nil
}

// queryResults executes a search query and maps rows to SearchResult.
func (cs *ContentStore) queryResults(q string, args []any, layer string) ([]model.SearchResult, error) {
	rows, err := cs.db.Query(q, args...)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var results []model.SearchResult

	for rows.Next() {
		var (
			r           model.SearchResult
			highlighted string
		)

		if err := rows.Scan(&r.Title, &r.Content, &r.ContentType, &r.Source, &r.Rank, &highlighted); err != nil {
			return nil, err
		}

		r.Highlighted = highlighted
		r.MatchLayer = layer
		results = append(results, r)
	}

	return results, rows.Err()
}

func setMatchLayer(results []model.SearchResult, layer string) []model.SearchResult {
	for i := range results {
		results[i].MatchLayer = layer
	}

	return results
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}

	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	for i := range prev {
		prev[i] = i
	}

	for i := 1; i <= len(a); i++ {
		curr := make([]int, len(b)+1)

		curr[0] = i
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1]
			} else {
				curr[j] = 1 + min(prev[j], curr[j-1], prev[j-1])
			}
		}

		prev = curr
	}

	return prev[len(b)]
}

func maxEditDistance(wordLen int) int {
	if wordLen <= 4 {
		return 1
	}

	if wordLen <= 12 {
		return 2
	}

	return 3
}
