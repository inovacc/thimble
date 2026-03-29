package store

import (
	"fmt"
	"testing"
)

// BenchmarkIndex indexes 100 documents (each with 10 markdown sections).
func BenchmarkIndex(b *testing.B) {
	cs := benchDB(b)
	content := generateMarkdown(10)

	b.ResetTimer()

	for i := range b.N {
		_, _ = cs.Index(content, fmt.Sprintf("idx-%d", i))
	}
}

// BenchmarkSearch searches across ~1000 indexed chunks (100 docs x ~10 chunks each).
func BenchmarkSearch(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 100) // ~1000 chunks

	b.ResetTimer()

	for range b.N {
		_, _ = cs.Search("authentication caching middleware", 5, "", "AND")
	}
}

// BenchmarkSearchFallback triggers the full FTS5 fallback chain by searching
// for a term that won't match Porter or Trigram, forcing fuzzy correction
// and semantic search layers.
func BenchmarkSearchFallback(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 100) // ~1000 chunks

	b.ResetTimer()

	for range b.N {
		// Nonsense query that won't match any layer, exercising the full chain.
		_, _ = cs.SearchWithFallback("zyxwvutsrqp qwertyuiop", 5, "")
	}
}
