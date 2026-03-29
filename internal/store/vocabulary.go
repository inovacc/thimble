package store

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// Stopwords — common English words and code/changelog noise filtered from vocabulary.
var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "are": true, "but": true, "not": true,
	"you": true, "all": true, "can": true, "had": true, "her": true, "was": true,
	"one": true, "our": true, "out": true, "has": true, "his": true, "how": true,
	"its": true, "may": true, "new": true, "now": true, "old": true, "see": true,
	"way": true, "who": true, "did": true, "get": true, "got": true, "let": true,
	"say": true, "she": true, "too": true, "use": true, "will": true, "with": true,
	"this": true, "that": true, "from": true, "they": true, "been": true, "have": true,
	"many": true, "some": true, "them": true, "than": true, "each": true, "make": true,
	"like": true, "just": true, "over": true, "such": true, "take": true, "into": true,
	"year": true, "your": true, "good": true, "could": true, "would": true, "about": true,
	"which": true, "their": true, "there": true, "other": true, "after": true,
	"should": true, "through": true, "also": true, "more": true, "most": true,
	"only": true, "very": true, "when": true, "what": true, "then": true, "these": true,
	"those": true, "being": true, "does": true, "done": true, "both": true, "same": true,
	"still": true, "while": true, "where": true, "here": true, "were": true, "much": true,
	// Common in code/changelogs
	"update": true, "updates": true, "updated": true, "deps": true, "dev": true,
	"tests": true, "test": true, "add": true, "added": true, "fix": true, "fixed": true,
	"run": true, "running": true, "using": true,
}

var wordSplitRe = regexp.MustCompile(`[^\p{L}\p{N}_-]+`)

// extractAndStoreVocabulary extracts unique words from content and stores them
// in the vocabulary table for fuzzy correction.
func (cs *ContentStore) extractAndStoreVocabulary(content string) {
	words := wordSplitRe.Split(strings.ToLower(content), -1)

	seen := make(map[string]bool)

	for _, w := range words {
		if len(w) >= 3 && !stopwords[w] {
			seen[w] = true
		}
	}

	tx, err := cs.db.Begin()
	if err != nil {
		return
	}

	for word := range seen {
		_, _ = tx.Exec("INSERT OR IGNORE INTO vocabulary (word) VALUES (?)", word)
	}

	_ = tx.Commit()
}

type scoredTerm struct {
	word  string
	score float64
}

// GetDistinctiveTerms returns IDF-scored distinctive terms for a source.
func (cs *ContentStore) GetDistinctiveTerms(sourceID int64, maxTerms int) ([]string, error) {
	if maxTerms <= 0 {
		maxTerms = 40
	}

	var chunkCount int

	err := cs.db.QueryRow("SELECT chunk_count FROM sources WHERE id = ?", sourceID).Scan(&chunkCount)
	if err != nil || chunkCount < 3 {
		return nil, err
	}

	totalChunks := chunkCount
	minAppearances := 2
	maxAppearances := int(math.Max(3, math.Ceil(float64(totalChunks)*0.4)))

	// Stream chunks and count document frequency per word.
	rows, err := cs.db.Query("SELECT content FROM chunks WHERE source_id = ?", sourceID)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	docFreq := make(map[string]int)

	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			continue
		}

		words := wordSplitRe.Split(strings.ToLower(content), -1)
		unique := make(map[string]bool)

		for _, w := range words {
			if len(w) >= 3 && !stopwords[w] {
				unique[w] = true
			}
		}

		for w := range unique {
			docFreq[w]++
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Filter and score.
	var scored []scoredTerm

	for word, count := range docFreq {
		if count < minAppearances || count > maxAppearances {
			continue
		}

		idf := math.Log(float64(totalChunks) / float64(count))
		lenBonus := math.Min(float64(len(word))/20.0, 0.5)

		identifierBonus := 0.0
		if strings.Contains(word, "_") {
			identifierBonus = 1.5
		} else if len(word) >= 12 {
			identifierBonus = 0.8
		}

		scored = append(scored, scoredTerm{word: word, score: idf + lenBonus + identifierBonus})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if len(scored) > maxTerms {
		scored = scored[:maxTerms]
	}

	result := make([]string, len(scored))
	for i, s := range scored {
		result[i] = s.word
	}

	return result, nil
}
