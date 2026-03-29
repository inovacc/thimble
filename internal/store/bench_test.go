package store

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// ── Helpers ──

// benchDB creates a ContentStore in a temp directory for benchmarks.
func benchDB(b *testing.B) *ContentStore {
	b.Helper()
	dir := b.TempDir()

	cs, err := New(filepath.Join(dir, "bench.db"))
	if err != nil {
		b.Fatalf("New: %v", err)
	}

	b.Cleanup(func() { cs.Close() })

	return cs
}

// generateMarkdown builds a realistic markdown document with N sections,
// each containing a heading, prose, and a code block.
func generateMarkdown(sections int) string {
	var sb strings.Builder
	for i := range sections {
		fmt.Fprintf(&sb, "## Section %d: Feature %s\n\n", i+1, featureName(i))
		fmt.Fprintf(&sb, "This section describes the implementation of %s. ", featureName(i))
		fmt.Fprintf(&sb, "It integrates with %s and %s for optimal performance.\n\n", featureName((i+7)%sections), featureName((i+13)%sections))
		fmt.Fprintf(&sb, "```go\nfunc %s() error {\n\treturn nil\n}\n```\n\n", funcName(i))
	}

	return sb.String()
}

// generatePlainText builds realistic plain-text output (like test/build logs).
func generatePlainText(lines int) string {
	var sb strings.Builder

	for i := range lines {
		switch i % 5 {
		case 0:
			fmt.Fprintf(&sb, "=== RUN   TestFeature%d\n", i)
		case 1:
			fmt.Fprintf(&sb, "--- PASS: TestFeature%d (0.%03ds)\n", i-1, i%100)
		case 2:
			fmt.Fprintf(&sb, "    feature_%d_test.go:%d: checking %s integration\n", i, 42+i, featureName(i))
		case 3:
			fmt.Fprintf(&sb, "ok  \tgithub.com/example/pkg%d\t%.3fs\n", i%20, float64(i)*0.01)
		case 4:
			fmt.Fprintf(&sb, "coverage: %.1f%% of statements\n", 70.0+float64(i%30))
		}
	}

	return sb.String()
}

func featureName(i int) string {
	names := []string{
		"authentication", "authorization", "caching", "logging", "metrics",
		"rate-limiting", "pagination", "search", "indexing", "validation",
		"serialization", "encryption", "compression", "routing", "middleware",
		"database", "migration", "webhook", "notification", "scheduler",
	}

	return names[i%len(names)]
}

func funcName(i int) string {
	names := []string{
		"HandleAuth", "ValidateToken", "CacheResult", "LogRequest", "TrackMetric",
		"RateLimit", "Paginate", "SearchIndex", "BuildIndex", "ValidateInput",
		"SerializeJSON", "EncryptPayload", "CompressData", "RouteRequest", "ApplyMiddleware",
		"QueryDB", "RunMigration", "SendWebhook", "PushNotification", "ScheduleJob",
	}

	return names[i%len(names)]
}

// seedStore populates a store with N markdown documents of ~10 sections each.
func seedStore(b *testing.B, cs *ContentStore, docs int) {
	b.Helper()

	for i := range docs {
		content := generateMarkdown(10)

		label := fmt.Sprintf("doc-%d", i)
		if _, err := cs.Index(content, label); err != nil {
			b.Fatalf("Index: %v", err)
		}
	}
}

// ── Index Benchmarks ──

func BenchmarkIndexMarkdown_10sections(b *testing.B) {
	cs := benchDB(b)
	content := generateMarkdown(10)

	b.ResetTimer()

	for i := range b.N {
		_, _ = cs.Index(content, fmt.Sprintf("bench-%d", i))
	}
}

func BenchmarkIndexMarkdown_100sections(b *testing.B) {
	cs := benchDB(b)
	content := generateMarkdown(100)

	b.ResetTimer()

	for i := range b.N {
		_, _ = cs.Index(content, fmt.Sprintf("bench-%d", i))
	}
}

func BenchmarkIndexPlainText_100lines(b *testing.B) {
	cs := benchDB(b)
	content := generatePlainText(100)

	b.ResetTimer()

	for i := range b.N {
		_, _ = cs.IndexPlainText(content, fmt.Sprintf("bench-%d", i), 20)
	}
}

func BenchmarkIndexPlainText_1000lines(b *testing.B) {
	cs := benchDB(b)
	content := generatePlainText(1000)

	b.ResetTimer()

	for i := range b.N {
		_, _ = cs.IndexPlainText(content, fmt.Sprintf("bench-%d", i), 20)
	}
}

func BenchmarkIndexJSON_small(b *testing.B) {
	cs := benchDB(b)
	content := `{"users":[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"},{"id":3,"name":"Charlie"}]}`

	b.ResetTimer()

	for i := range b.N {
		_, _ = cs.IndexJSON(content, fmt.Sprintf("bench-%d", i))
	}
}

func BenchmarkIndexReindex_dedup(b *testing.B) {
	cs := benchDB(b)
	content := generateMarkdown(10)
	// Pre-seed with the same label to trigger dedup path.
	_, _ = cs.Index(content, "dedup-label")

	b.ResetTimer()

	for range b.N {
		_, _ = cs.Index(content, "dedup-label")
	}
}

// ── Search Benchmarks (varying store sizes) ──

func BenchmarkSearchPorter_100chunks(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 10) // ~100 chunks
	b.ResetTimer()

	for range b.N {
		_, _ = cs.Search("authentication token validation", 5, "", "AND")
	}
}

func BenchmarkSearchPorter_1000chunks(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 100) // ~1000 chunks
	b.ResetTimer()

	for range b.N {
		_, _ = cs.Search("authentication token validation", 5, "", "AND")
	}
}

func BenchmarkSearchPorter_10000chunks(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 1000) // ~10000 chunks
	b.ResetTimer()

	for range b.N {
		_, _ = cs.Search("authentication token validation", 5, "", "AND")
	}
}

func BenchmarkSearchTrigram_1000chunks(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 100)
	b.ResetTimer()

	for range b.N {
		_, _ = cs.SearchTrigram("authenticat", 5, "", "AND")
	}
}

func BenchmarkSearchWithFallback_1000chunks(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 100)
	b.ResetTimer()

	for range b.N {
		_, _ = cs.SearchWithFallback("authentication token", 5, "")
	}
}

func BenchmarkSearchWithFallback_noResults(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 100)
	b.ResetTimer()

	for range b.N {
		// Query that won't match anything — exercises all 5 fallback layers.
		_, _ = cs.SearchWithFallback("zyxwvutsrqp nonexistent", 5, "")
	}
}

func BenchmarkSearchSourceFiltered_1000chunks(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 100)
	b.ResetTimer()

	for range b.N {
		_, _ = cs.Search("authentication", 5, "doc-5", "AND")
	}
}

// ── Semantic Search Benchmarks ──

func BenchmarkSemanticSearch_100chunks(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 10)
	b.ResetTimer()

	for range b.N {
		_, _ = cs.SemanticSearch("user login security", 5, "")
	}
}

func BenchmarkSemanticSearch_1000chunks(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 100)
	b.ResetTimer()

	for range b.N {
		_, _ = cs.SemanticSearch("user login security", 5, "")
	}
}

// ── Vocabulary / Fuzzy Benchmarks ──

func BenchmarkFuzzyCorrect_1000chunks(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 100)
	b.ResetTimer()

	for range b.N {
		_ = cs.FuzzyCorrect("autentication") // misspelled
	}
}

func BenchmarkGetDistinctiveTerms(b *testing.B) {
	cs := benchDB(b)
	result, _ := cs.Index(generateMarkdown(50), "terms-source")

	b.ResetTimer()

	for range b.N {
		_, _ = cs.GetDistinctiveTerms(result.SourceID, 40)
	}
}

// ── Stats / List Benchmarks ──

func BenchmarkGetStats_1000chunks(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 100)
	b.ResetTimer()

	for range b.N {
		_, _ = cs.GetStats()
	}
}

func BenchmarkListSources_100sources(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 100)
	b.ResetTimer()

	for range b.N {
		_, _ = cs.ListSources()
	}
}

// ── Memory Benchmark ──

func BenchmarkIndexMemory_100sections(b *testing.B) {
	cs := benchDB(b)
	content := generateMarkdown(100)

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		_, _ = cs.Index(content, fmt.Sprintf("mem-%d", i))
	}
}

func BenchmarkSearchMemory_1000chunks(b *testing.B) {
	cs := benchDB(b)
	seedStore(b, cs, 100)
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_, _ = cs.SearchWithFallback("authentication token", 5, "")
	}
}
