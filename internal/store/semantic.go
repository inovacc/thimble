package store

import (
	"math"
	"strings"
	"unicode"

	"github.com/inovacc/thimble/internal/model"
)

// semanticStopwords is a minimal set of high-frequency function words that
// carry almost no semantic weight. Filtering these improves TF-IDF precision
// by preventing common words from dominating the cosine similarity score.
var semanticStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "are": true, "but": true,
	"not": true, "you": true, "all": true, "can": true, "had": true,
	"was": true, "one": true, "our": true, "has": true, "his": true,
	"how": true, "its": true, "may": true, "new": true, "now": true,
	"see": true, "way": true, "who": true, "did": true, "get": true,
	"let": true, "say": true, "too": true, "use": true, "will": true,
	"with": true, "this": true, "that": true, "from": true, "they": true,
	"been": true, "have": true, "some": true, "them": true, "than": true,
	"each": true, "make": true, "like": true, "just": true, "over": true,
	"into": true, "your": true, "also": true, "more": true, "most": true,
	"only": true, "very": true, "when": true, "what": true, "then": true,
	"here": true, "were": true, "much": true, "does": true, "done": true,
	"both": true, "same": true, "while": true, "where": true, "there": true,
	"which": true, "their": true, "other": true, "about": true, "could": true,
	"would": true, "should": true, "these": true, "those": true, "being": true,
}

// wordSplitRe is defined in vocabulary.go

// tokenize splits text into normalized lowercase tokens, filtering stopwords
// and tokens shorter than 2 characters.
func tokenize(text string) []string {
	parts := wordSplitRe.Split(strings.ToLower(text), -1)

	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) >= 2 && !semanticStopwords[p] {
			tokens = append(tokens, p)
		}
	}

	return tokens
}

// splitCamelCase splits camelCase and PascalCase identifiers into sub-tokens.
// "getUserName" → ["get", "user", "name"]
func splitCamelCase(s string) []string {
	var (
		parts   []string
		current []rune
	)

	for _, r := range s {
		if unicode.IsUpper(r) && len(current) > 0 {
			parts = append(parts, strings.ToLower(string(current)))
			current = []rune{r}
		} else {
			current = append(current, r)
		}
	}

	if len(current) > 0 {
		parts = append(parts, strings.ToLower(string(current)))
	}

	return parts
}

// tokenizeExpanded tokenizes text with camelCase expansion for identifier matching.
func tokenizeExpanded(text string) []string {
	base := tokenize(text)

	expanded := make([]string, 0, len(base)*2)
	for _, t := range base {
		expanded = append(expanded, t)
		// Expand camelCase/PascalCase identifiers.
		if strings.ContainsFunc(t, unicode.IsUpper) || strings.Contains(t, "_") {
			for _, sub := range splitCamelCase(t) {
				if len(sub) >= 2 && !semanticStopwords[sub] {
					expanded = append(expanded, sub)
				}
			}
			// Also split on underscores.
			for sub := range strings.SplitSeq(t, "_") {
				if len(sub) >= 2 && !semanticStopwords[sub] {
					expanded = append(expanded, strings.ToLower(sub))
				}
			}
		}
	}

	return expanded
}

// tfidfVector computes a TF-IDF-weighted sparse vector for a token list.
// docFreq maps each token to the number of documents it appears in.
// totalDocs is the total number of documents in the corpus.
type sparseVector map[string]float64

func tfidfVector(tokens []string, docFreq map[string]int, totalDocs int) sparseVector {
	// Term frequency (within this document).
	tf := make(map[string]int, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}

	vec := make(sparseVector, len(tf))
	for term, count := range tf {
		// TF: log-normalized to reduce the impact of very frequent terms.
		tfScore := 1 + math.Log(float64(count))
		// IDF: inverse document frequency with smoothing.
		df := docFreq[term]
		if df == 0 {
			df = 1
		}

		idfScore := math.Log(float64(totalDocs+1) / float64(df+1))
		vec[term] = tfScore * idfScore
	}

	return vec
}

// cosineSimilarity computes the cosine similarity between two sparse vectors.
func cosineSimilarity(a, b sparseVector) float64 {
	var dot, normA, normB float64
	for term, va := range a {
		normA += va * va
		if vb, ok := b[term]; ok {
			dot += va * vb
		}
	}

	for _, vb := range b {
		normB += vb * vb
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// SemanticSearch performs TF-IDF cosine similarity search across all chunks.
// This is a concept-matching layer that complements the exact-match FTS5 search.
// It finds chunks that share vocabulary with the query even when no exact
// terms match (e.g., query "authentication" matches content about "login tokens").
func (cs *ContentStore) SemanticSearch(query string, limit int, source string) ([]model.SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	queryTokens := tokenizeExpanded(query)
	if len(queryTokens) == 0 {
		return nil, nil
	}

	// Load all chunks (potentially expensive for very large stores, but
	// thimble stores are per-project and typically <1000 chunks).
	var (
		sqlQuery string
		args     []any
	)

	if source != "" {
		sqlQuery = `SELECT c.rowid, c.title, c.content, c.content_type, s.label
			FROM chunks c JOIN sources s ON s.id = c.source_id
			WHERE s.label LIKE ?
			ORDER BY c.rowid`
		args = []any{"%" + source + "%"}
	} else {
		sqlQuery = `SELECT c.rowid, c.title, c.content, c.content_type, s.label
			FROM chunks c JOIN sources s ON s.id = c.source_id
			ORDER BY c.rowid`
	}

	rows, err := cs.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	type doc struct {
		rowid       int64
		title       string
		content     string
		contentType string
		source      string
		tokens      []string
	}

	var docs []doc

	docFreq := make(map[string]int)

	for rows.Next() {
		var d doc
		if err := rows.Scan(&d.rowid, &d.title, &d.content, &d.contentType, &d.source); err != nil {
			return nil, err
		}

		d.tokens = tokenizeExpanded(d.title + " " + d.content)

		// Track document frequency for IDF.
		seen := make(map[string]bool, len(d.tokens))
		for _, t := range d.tokens {
			if !seen[t] {
				docFreq[t]++
				seen[t] = true
			}
		}

		docs = append(docs, d)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(docs) == 0 {
		return nil, nil
	}

	totalDocs := len(docs)
	queryVec := tfidfVector(queryTokens, docFreq, totalDocs)

	// Score each document.
	type scored struct {
		doc   doc
		score float64
	}

	var scoredDocs []scored

	for _, d := range docs {
		docVec := tfidfVector(d.tokens, docFreq, totalDocs)

		sim := cosineSimilarity(queryVec, docVec)
		if sim > 0.01 { // Skip near-zero similarity.
			scoredDocs = append(scoredDocs, scored{d, sim})
		}
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
			Title:       sd.doc.title,
			Content:     sd.doc.content,
			Source:      sd.doc.source,
			Rank:        sd.score,
			ContentType: sd.doc.contentType,
			MatchLayer:  "semantic",
		}
	}

	return results, nil
}
