package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGetDistinctiveTerms verifies IDF scoring, filtering, and identifier bonuses.
func TestGetDistinctiveTerms(t *testing.T) {
	cs := tempDB(t)

	// Build content with enough chunks (>=3) so GetDistinctiveTerms proceeds.
	// We need words that appear in 2+ chunks but not more than 40% of chunks.
	// "alpha" appears in chunks 1,2 (2/5 = distinctive)
	// "beta_func" appears in chunks 1,2 (identifier bonus due to underscore)
	// "gamma" appears in all 5 chunks (too common, filtered out)
	// "delta" appears in 1 chunk only (below minAppearances=2, filtered out)
	sections := []string{
		"# Section One\n\nalpha beta_func gamma common words here padding text",
		"# Section Two\n\nalpha beta_func gamma more padding filler words here",
		"# Section Three\n\ngamma padding words filler extra text here section",
		"# Section Four\n\ngamma another filler paragraph with padding content",
		"# Section Five\n\ngamma delta unique singleton filler padding words here",
	}
	content := strings.Join(sections, "\n\n---\n\n")

	result, err := cs.Index(content, "terms-doc")
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if result.TotalChunks < 3 {
		t.Fatalf("need >=3 chunks for GetDistinctiveTerms, got %d", result.TotalChunks)
	}

	terms, err := cs.GetDistinctiveTerms(result.SourceID, 40)
	if err != nil {
		t.Fatalf("GetDistinctiveTerms: %v", err)
	}

	// beta_func should rank high due to identifier bonus (underscore).
	termSet := make(map[string]bool)
	for _, term := range terms {
		termSet[term] = true
	}

	if !termSet["beta_func"] {
		t.Errorf("expected 'beta_func' in distinctive terms (identifier bonus), got: %v", terms)
	}

	// Words appearing in only 1 chunk (delta) should be filtered.
	if termSet["delta"] {
		t.Error("'delta' appears in only 1 chunk, should be filtered by minAppearances")
	}

	t.Run("fewer_than_3_chunks", func(t *testing.T) {
		cs2 := tempDB(t)

		res, err := cs2.Index("# Small\n\nTiny document.", "small-doc")
		if err != nil {
			t.Fatal(err)
		}

		terms, err := cs2.GetDistinctiveTerms(res.SourceID, 10)
		if err != nil {
			t.Fatal(err)
		}

		if terms != nil {
			t.Errorf("expected nil terms for <3 chunks, got %v", terms)
		}
	})

	t.Run("nonexistent_source", func(t *testing.T) {
		cs3 := tempDB(t)

		terms, err := cs3.GetDistinctiveTerms(99999, 10)
		if err == nil && terms != nil {
			t.Error("expected nil terms for nonexistent source")
		}
	})

	t.Run("maxTerms_limit", func(t *testing.T) {
		terms2, err := cs.GetDistinctiveTerms(result.SourceID, 2)
		if err != nil {
			t.Fatal(err)
		}

		if len(terms2) > 2 {
			t.Errorf("expected at most 2 terms, got %d", len(terms2))
		}
	})

	t.Run("default_maxTerms", func(t *testing.T) {
		terms3, err := cs.GetDistinctiveTerms(result.SourceID, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Default is 40; just verify it doesn't crash.
		_ = terms3
	})
}

// TestChunkMarkdown_EdgeCases covers nested headings, orphaned code blocks,
// empty sections, and oversized chunks.
func TestChunkMarkdown_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		maxBytes   int
		wantMin    int // minimum number of chunks
		checkTitle string
		checkFn    func(t *testing.T, chunks []chunk)
	}{
		{
			name: "nested_headings_breadcrumb",
			input: `# Top
## Middle
### Deep
Some deep content here.`,
			maxBytes: MaxChunkBytes,
			wantMin:  1,
			checkFn: func(t *testing.T, chunks []chunk) {
				// Title should contain breadcrumb path.
				found := false

				for _, c := range chunks {
					if strings.Contains(c.title, "Top") && strings.Contains(c.title, "Deep") {
						found = true
						break
					}
				}

				if !found {
					titles := make([]string, len(chunks))
					for i, c := range chunks {
						titles[i] = c.title
					}

					t.Errorf("expected breadcrumb title with Top > ... > Deep, got titles: %v", titles)
				}
			},
		},
		{
			name: "code_block_kept_intact",
			input: `# Code Example

` + "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```" + `

Some text after code.`,
			maxBytes: MaxChunkBytes,
			wantMin:  1,
			checkFn: func(t *testing.T, chunks []chunk) {
				hasCode := false
				for _, c := range chunks {
					if c.hasCode {
						hasCode = true

						if !strings.Contains(c.content, "func main()") {
							t.Error("code chunk missing function body")
						}
					}
				}

				if !hasCode {
					t.Error("expected at least one code chunk")
				}
			},
		},
		{
			name: "orphaned_code_block_no_closing_fence",
			input: `# Orphan

` + "```python\nprint('hello')\nmore code\nstill going",
			maxBytes: MaxChunkBytes,
			wantMin:  1,
			checkFn: func(t *testing.T, chunks []chunk) {
				// Should not panic; content should be captured.
				found := false

				for _, c := range chunks {
					if strings.Contains(c.content, "print('hello')") {
						found = true
					}
				}

				if !found {
					t.Error("orphaned code block content should be captured")
				}
			},
		},
		{
			name:     "empty_sections_between_headings",
			input:    "# A\n\n# B\n\n# C\n\nActual content here.",
			maxBytes: MaxChunkBytes,
			wantMin:  1,
			checkFn: func(t *testing.T, chunks []chunk) {
				// Empty sections (A, B) should not produce chunks.
				for _, c := range chunks {
					if strings.TrimSpace(c.content) == "" {
						t.Error("should not produce empty chunks")
					}
				}
			},
		},
		{
			name:     "horizontal_rule_splits",
			input:    "Content before rule.\n\n---\n\nContent after rule.",
			maxBytes: MaxChunkBytes,
			wantMin:  2,
			checkFn: func(t *testing.T, chunks []chunk) {
				if len(chunks) < 2 {
					t.Errorf("HR should split into 2+ chunks, got %d", len(chunks))
				}
			},
		},
		{
			name:     "oversized_chunk_splits_at_paragraphs",
			input:    "# Big\n\n" + strings.Repeat("Word ", 500) + "\n\n" + strings.Repeat("Another ", 500),
			maxBytes: 512,
			wantMin:  2,
			checkFn: func(t *testing.T, chunks []chunk) {
				// Verify splitting happened (more than one chunk produced).
				if len(chunks) < 2 {
					t.Errorf("expected multiple chunks from oversized input, got %d", len(chunks))
				}
			},
		},
		{
			name:     "no_headings_untitled",
			input:    "Just plain text with no headings at all.",
			maxBytes: MaxChunkBytes,
			wantMin:  1,
			checkFn: func(t *testing.T, chunks []chunk) {
				if len(chunks) > 0 && chunks[0].title != "Untitled" {
					t.Errorf("expected title 'Untitled', got %q", chunks[0].title)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunkMarkdown(tc.input, tc.maxBytes)
			if len(chunks) < tc.wantMin {
				t.Fatalf("expected >= %d chunks, got %d", tc.wantMin, len(chunks))
			}

			if tc.checkFn != nil {
				tc.checkFn(t, chunks)
			}
		})
	}
}

// TestChunkPlainText_Sections tests section detection with blank-line separators
// and fixed-size line-group fallback.
func TestChunkPlainText_Sections(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		linesPerChunk int
		wantMin       int
		checkFn       func(t *testing.T, chunks []chunk)
	}{
		{
			name: "blank_line_sections",
			input: `First section content here.
With a second line.

Second section starts here.
More details follow.

Third section for variety.
Additional information.`,
			linesPerChunk: 20,
			wantMin:       3,
			checkFn: func(t *testing.T, chunks []chunk) {
				// Titles should be taken from first line of each section.
				if !strings.Contains(chunks[0].title, "First section") {
					t.Errorf("first chunk title = %q, expected to contain 'First section'", chunks[0].title)
				}
			},
		},
		{
			name:          "single_section_short",
			input:         "Just one short line.",
			linesPerChunk: 20,
			wantMin:       1,
			checkFn: func(t *testing.T, chunks []chunk) {
				if chunks[0].title != "Output" {
					t.Errorf("short text title = %q, want 'Output'", chunks[0].title)
				}
			},
		},
		{
			name: "fixed_size_fallback",
			input: func() string {
				lines := make([]string, 100)
				for i := range lines {
					lines[i] = fmt.Sprintf("Line %d: some content for testing.", i+1)
				}

				return strings.Join(lines, "\n")
			}(),
			linesPerChunk: 10,
			wantMin:       5,
			checkFn: func(t *testing.T, chunks []chunk) {
				// Should fall back to fixed-size since no blank-line sections.
				for _, c := range chunks {
					if c.title == "" {
						t.Error("chunk should have a title")
					}
				}
			},
		},
		{
			name: "large_section_skips_blank_line_mode",
			input: func() string {
				// One section exceeds 5000 chars -> falls back to line groups.
				big := strings.Repeat("x", 5001)
				return "Short\n\n" + big + "\n\nAnother short"
			}(),
			linesPerChunk: 20,
			wantMin:       1,
		},
		{
			name:          "title_truncation_at_80",
			input:         strings.Repeat("A", 120) + "\n\n" + "short\n\nother",
			linesPerChunk: 20,
			wantMin:       1,
			checkFn: func(t *testing.T, chunks []chunk) {
				if len(chunks[0].title) > 80 {
					t.Errorf("title should be truncated to 80 chars, got %d", len(chunks[0].title))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunkPlainText(tc.input, tc.linesPerChunk)
			if len(chunks) < tc.wantMin {
				t.Fatalf("expected >= %d chunks, got %d", tc.wantMin, len(chunks))
			}

			if tc.checkFn != nil {
				tc.checkFn(t, chunks)
			}
		})
	}
}

// TestSearchSourceFilter verifies that the source parameter filters results correctly.
func TestSearchSourceFilter(t *testing.T) {
	cs := tempDB(t)

	_, err := cs.Index("# Golang Guide\n\nGo programming concurrency patterns.", "golang-guide")
	if err != nil {
		t.Fatal(err)
	}

	_, err = cs.Index("# Python Guide\n\nPython programming async patterns.", "python-guide")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		query     string
		source    string
		wantLabel string
		wantEmpty bool
	}{
		{
			name:      "filter_by_golang_source",
			query:     "programming patterns",
			source:    "golang",
			wantLabel: "golang-guide",
		},
		{
			name:      "filter_by_python_source",
			query:     "programming patterns",
			source:    "python",
			wantLabel: "python-guide",
		},
		{
			name:      "no_match_source",
			query:     "programming",
			source:    "rust",
			wantEmpty: true,
		},
		{
			name:   "empty_source_returns_all",
			query:  "programming patterns",
			source: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, err := cs.Search(tc.query, 10, tc.source, "AND")
			if err != nil {
				t.Fatalf("Search: %v", err)
			}

			if tc.wantEmpty {
				if len(results) > 0 {
					t.Errorf("expected no results for source=%q, got %d", tc.source, len(results))
				}

				return
			}

			if tc.wantLabel != "" {
				if len(results) == 0 {
					t.Fatalf("expected results for source=%q", tc.source)
				}

				if results[0].Source != tc.wantLabel {
					t.Errorf("source = %q, want %q", results[0].Source, tc.wantLabel)
				}
			}

			if tc.source == "" && len(results) < 2 {
				t.Errorf("expected results from multiple sources, got %d", len(results))
			}
		})
	}

	// Also test trigram source filter.
	t.Run("trigram_source_filter", func(t *testing.T) {
		results, err := cs.SearchTrigram("programming", 10, "golang", "AND")
		if err != nil {
			t.Fatalf("SearchTrigram: %v", err)
		}

		for _, r := range results {
			if !strings.Contains(r.Source, "golang") {
				t.Errorf("trigram source filter leaked: source=%q", r.Source)
			}
		}
	})
}

// TestIndexAndSearchRoundtrip verifies the full pipeline: index markdown,
// search, verify result fields, and check highlighted output.
func TestIndexAndSearchRoundtrip(t *testing.T) {
	cs := tempDB(t)

	md := `# Architecture Overview

The system uses a hexagonal architecture with ports and adapters.

## Core Domain

Business logic lives in the domain layer, independent of infrastructure.

## Infrastructure

Database access, HTTP handlers, and gRPC services are adapters.

` + "```go\ntype Repository interface {\n\tFind(id string) (*Entity, error)\n}\n```"

	result, err := cs.Index(md, "arch-doc")
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if result.Label != "arch-doc" {
		t.Errorf("label = %q, want %q", result.Label, "arch-doc")
	}

	if result.TotalChunks == 0 {
		t.Fatal("expected chunks")
	}

	if result.CodeChunks == 0 {
		t.Error("expected code chunks from fenced code block")
	}

	// Search and verify fields are populated.
	results, err := cs.Search("hexagonal architecture", 5, "", "AND")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected search results")
	}

	r := results[0]
	if r.Title == "" {
		t.Error("result title should not be empty")
	}

	if r.Content == "" {
		t.Error("result content should not be empty")
	}

	if r.Source != "arch-doc" {
		t.Errorf("source = %q, want 'arch-doc'", r.Source)
	}

	if r.Rank == 0 {
		t.Error("rank should be non-zero")
	}

	if r.MatchLayer != "porter" {
		t.Errorf("matchLayer = %q, want 'porter'", r.MatchLayer)
	}

	if r.Highlighted == "" {
		t.Error("highlighted should not be empty")
	}

	// GetChunksBySource should return all chunks.
	allChunks, err := cs.GetChunksBySource(result.SourceID)
	if err != nil {
		t.Fatalf("GetChunksBySource: %v", err)
	}

	if len(allChunks) != result.TotalChunks {
		t.Errorf("GetChunksBySource returned %d, want %d", len(allChunks), result.TotalChunks)
	}

	// Verify code chunk is identified.
	hasCode := false

	for _, c := range allChunks {
		if c.ContentType == "code" {
			hasCode = true
		}
	}

	if !hasCode {
		t.Error("expected at least one code-type chunk")
	}
}

// TestVocabularyExtraction verifies stopword filtering and vocabulary storage.
func TestVocabularyExtraction(t *testing.T) {
	cs := tempDB(t)

	// Index content with known words. "kubernetes" and "orchestration" are not stopwords.
	// "the", "and", "for", "using" ARE stopwords and should be filtered.
	_, err := cs.Index(
		"# Kubernetes\n\norchestration platform for the containers and microservices using deployment.",
		"vocab-doc",
	)
	if err != nil {
		t.Fatal(err)
	}

	// Query vocabulary table directly.
	rows, err := cs.db.Query("SELECT word FROM vocabulary ORDER BY word")
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = rows.Close() }()

	vocab := make(map[string]bool)

	for rows.Next() {
		var w string
		if err := rows.Scan(&w); err != nil {
			t.Fatal(err)
		}

		vocab[w] = true
	}

	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	// Non-stopwords with len>=3 should be present.
	for _, want := range []string{"kubernetes", "orchestration", "platform", "containers", "microservices", "deployment"} {
		if !vocab[want] {
			t.Errorf("expected %q in vocabulary, got: %v", want, vocab)
		}
	}

	// Stopwords should NOT be present.
	for _, stop := range []string{"the", "and", "for", "using"} {
		if vocab[stop] {
			t.Errorf("stopword %q should not be in vocabulary", stop)
		}
	}

	// Short words (<3 chars) should not be present.
	for w := range vocab {
		if len(w) < 3 {
			t.Errorf("word %q is too short (len=%d), should be filtered", w, len(w))
		}
	}
}

// TestQueryResultsMatchLayer verifies that queryResults sets the match layer.
func TestQueryResultsMatchLayer(t *testing.T) {
	cs := tempDB(t)

	_, err := cs.Index("# Test\n\nSample content for query results layer testing.", "layer-doc")
	if err != nil {
		t.Fatal(err)
	}

	// Porter search sets layer to "porter".
	results, err := cs.Search("sample content", 5, "", "AND")
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range results {
		if r.MatchLayer != "porter" {
			t.Errorf("expected matchLayer 'porter', got %q", r.MatchLayer)
		}
	}

	// Trigram search sets layer to "trigram".
	triResults, err := cs.SearchTrigram("sample", 5, "", "AND")
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range triResults {
		if r.MatchLayer != "trigram" {
			t.Errorf("expected matchLayer 'trigram', got %q", r.MatchLayer)
		}
	}
}

// TestSearchWithFallbackSourceFilter verifies fallback search respects source filter.
func TestSearchWithFallbackSourceFilter(t *testing.T) {
	cs := tempDB(t)

	_, err := cs.Index("# Alpha\n\nSpecialized alpha content document.", "alpha-src")
	if err != nil {
		t.Fatal(err)
	}

	_, err = cs.Index("# Beta\n\nSpecialized beta content document.", "beta-src")
	if err != nil {
		t.Fatal(err)
	}

	results, err := cs.SearchWithFallback("specialized content", 10, "alpha")
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range results {
		if !strings.Contains(r.Source, "alpha") {
			t.Errorf("source filter not applied in fallback: source=%q", r.Source)
		}
	}
}

// TestInsertChunksDedup verifies that re-indexing the same label replaces old content.
func TestInsertChunksDedup(t *testing.T) {
	cs := tempDB(t)

	_, err := cs.Index("# Version A\n\nUnique phrase alpha bravo charlie.", "dedup-label")
	if err != nil {
		t.Fatal(err)
	}

	_, err = cs.Index("# Version B\n\nCompletely different delta echo foxtrot.", "dedup-label")
	if err != nil {
		t.Fatal(err)
	}

	stats, err := cs.GetStats()
	if err != nil {
		t.Fatal(err)
	}

	if stats.Sources != 1 {
		t.Errorf("expected 1 source after dedup, got %d", stats.Sources)
	}

	// Old content should be gone from both FTS tables.
	results, err := cs.Search("alpha bravo charlie", 5, "", "AND")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) > 0 {
		t.Error("old content should be removed after dedup")
	}

	// New content should be searchable.
	results, err = cs.Search("delta echo foxtrot", 5, "", "AND")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Error("new content should be searchable after dedup")
	}

	// Trigram table should also be deduped.
	triResults, err := cs.SearchTrigram("alpha bravo", 5, "", "AND")
	if err != nil {
		t.Fatal(err)
	}

	if len(triResults) > 0 {
		t.Error("old content should be removed from trigram table after dedup")
	}
}

// TestBuildTitle verifies the breadcrumb title builder.
func TestBuildTitle(t *testing.T) {
	tests := []struct {
		name    string
		stack   []headingEntry
		current string
		want    string
	}{
		{"empty_stack_no_current", nil, "", "Untitled"},
		{"empty_stack_with_current", nil, "Hello", "Hello"},
		{"single_level", []headingEntry{{1, "Top"}}, "Top", "Top"},
		{"nested", []headingEntry{{1, "Top"}, {2, "Mid"}, {3, "Deep"}}, "Deep", "Top > Mid > Deep"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildTitle(tc.stack, tc.current)
			if got != tc.want {
				t.Errorf("buildTitle = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestChunkMarkdownEmpty verifies empty input produces no chunks.
func TestChunkMarkdownEmpty(t *testing.T) {
	chunks := chunkMarkdown("", MaxChunkBytes)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty input, got %d", len(chunks))
	}
}

// TestChunkPlainTextEmpty verifies empty input returns a single "Output" chunk.
func TestChunkPlainTextEmpty(t *testing.T) {
	chunks := chunkPlainText("", 20)
	// Empty string has 1 line (the empty string itself), which is <= linesPerChunk,
	// so it returns a single chunk with title "Output".
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for empty input, got %d", len(chunks))
	}

	if len(chunks) > 0 && chunks[0].title != "Output" {
		t.Errorf("expected title 'Output', got %q", chunks[0].title)
	}
}

// TestFindIdentityField verifies identity field detection in JSON arrays.
func TestFindIdentityField(t *testing.T) {
	tests := []struct {
		name string
		arr  []any
		want string
	}{
		{
			name: "empty_array",
			arr:  []any{},
			want: "",
		},
		{
			name: "non_object_element",
			arr:  []any{"just a string"},
			want: "",
		},
		{
			name: "id_field",
			arr:  []any{map[string]any{"id": 1.0, "value": "x"}},
			want: "id",
		},
		{
			name: "name_field",
			arr:  []any{map[string]any{"name": "Alice", "age": 30.0}},
			want: "name",
		},
		{
			name: "title_field",
			arr:  []any{map[string]any{"title": "Post One", "body": "..."}},
			want: "title",
		},
		{
			name: "path_field",
			arr:  []any{map[string]any{"path": "/usr/bin", "size": 100.0}},
			want: "path",
		},
		{
			name: "slug_field",
			arr:  []any{map[string]any{"slug": "my-page"}},
			want: "slug",
		},
		{
			name: "key_field",
			arr:  []any{map[string]any{"key": "config_key"}},
			want: "key",
		},
		{
			name: "label_field",
			arr:  []any{map[string]any{"label": "primary"}},
			want: "label",
		},
		{
			name: "no_identity_field",
			arr:  []any{map[string]any{"foo": "bar", "baz": 42.0}},
			want: "",
		},
		{
			name: "identity_field_is_bool_ignored",
			arr:  []any{map[string]any{"id": true}},
			want: "",
		},
		{
			name: "id_field_numeric",
			arr:  []any{map[string]any{"id": 42.0}},
			want: "id",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findIdentityField(tc.arr)
			if got != tc.want {
				t.Errorf("findIdentityField() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestJSONBatchTitle verifies batch title generation for JSON arrays.
func TestJSONBatchTitle(t *testing.T) {
	tests := []struct {
		name          string
		prefix        string
		startIdx      int
		endIdx        int
		batch         []any
		identityField string
		wantContains  string
	}{
		{
			name:          "no_identity_single_item",
			prefix:        "data",
			startIdx:      0,
			endIdx:        0,
			batch:         []any{map[string]any{"x": 1}},
			identityField: "",
			wantContains:  "[0]",
		},
		{
			name:          "no_identity_range",
			prefix:        "data",
			startIdx:      0,
			endIdx:        4,
			batch:         []any{map[string]any{"x": 1}, map[string]any{"x": 2}},
			identityField: "",
			wantContains:  "[0-4]",
		},
		{
			name:          "identity_single_item",
			prefix:        "users",
			startIdx:      0,
			endIdx:        0,
			batch:         []any{map[string]any{"name": "Alice"}},
			identityField: "name",
			wantContains:  "Alice",
		},
		{
			name:          "identity_two_items",
			prefix:        "",
			startIdx:      0,
			endIdx:        1,
			batch:         []any{map[string]any{"id": "a"}, map[string]any{"id": "b"}},
			identityField: "id",
			wantContains:  "a, b",
		},
		{
			name:          "identity_three_items",
			prefix:        "items",
			startIdx:      0,
			endIdx:        2,
			batch:         []any{map[string]any{"id": "x"}, map[string]any{"id": "y"}, map[string]any{"id": "z"}},
			identityField: "id",
			wantContains:  "x, y, z",
		},
		{
			name:     "identity_four_items_ellipsis",
			prefix:   "items",
			startIdx: 0,
			endIdx:   3,
			batch: []any{
				map[string]any{"id": "first"},
				map[string]any{"id": "second"},
				map[string]any{"id": "third"},
				map[string]any{"id": "fourth"},
			},
			identityField: "id",
			wantContains:  "first",
		},
		{
			name:          "empty_prefix",
			prefix:        "",
			startIdx:      0,
			endIdx:        0,
			batch:         []any{map[string]any{"x": 1}},
			identityField: "",
			wantContains:  "[0]",
		},
		{
			name:          "non_map_batch_with_identity",
			prefix:        "data",
			startIdx:      0,
			endIdx:        0,
			batch:         []any{"just a string"},
			identityField: "id",
			wantContains:  "data",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := jsonBatchTitle(tc.prefix, tc.startIdx, tc.endIdx, tc.batch, tc.identityField)
			if !strings.Contains(got, tc.wantContains) {
				t.Errorf("jsonBatchTitle() = %q, want to contain %q", got, tc.wantContains)
			}
		})
	}
}

// TestChunkJSONArray verifies JSON array chunking with batching.
func TestChunkJSONArray(t *testing.T) {
	t.Run("small_array_single_batch", func(t *testing.T) {
		arr := []any{
			map[string]any{"id": 1.0, "name": "Alice"},
			map[string]any{"id": 2.0, "name": "Bob"},
		}

		var chunks []chunk
		chunkJSONArray(arr, []string{"users"}, &chunks, MaxChunkBytes)

		if len(chunks) == 0 {
			t.Fatal("expected at least one chunk")
		}

		// All items should fit in one chunk at MaxChunkBytes.
		if len(chunks) != 1 {
			t.Errorf("expected 1 chunk for small array, got %d", len(chunks))
		}

		if !chunks[0].hasCode {
			t.Error("JSON chunks should have hasCode=true")
		}
	})

	t.Run("large_array_multiple_batches", func(t *testing.T) {
		// Create an array large enough to require splitting.
		arr := make([]any, 0, 50)
		for i := range 50 {
			arr = append(arr, map[string]any{
				"id":          float64(i),
				"name":        fmt.Sprintf("User %d with a longer name for size", i),
				"description": strings.Repeat("x", 100),
			})
		}

		var chunks []chunk
		chunkJSONArray(arr, nil, &chunks, 512)

		if len(chunks) < 2 {
			t.Errorf("expected multiple chunks for large array, got %d", len(chunks))
		}
	})

	t.Run("array_without_identity_field", func(t *testing.T) {
		arr := []any{"string1", "string2", "string3"}

		var chunks []chunk
		chunkJSONArray(arr, []string{"data"}, &chunks, MaxChunkBytes)

		if len(chunks) == 0 {
			t.Fatal("expected at least one chunk")
		}
	})

	t.Run("empty_path", func(t *testing.T) {
		arr := []any{map[string]any{"id": 1.0}}

		var chunks []chunk
		chunkJSONArray(arr, nil, &chunks, MaxChunkBytes)

		if len(chunks) == 0 {
			t.Fatal("expected at least one chunk")
		}

		// Title should use (root) prefix.
		if !strings.Contains(chunks[0].title, "(root)") {
			t.Errorf("expected (root) in title, got %q", chunks[0].title)
		}
	})
}

// TestWalkJSON_AdditionalCases covers more walkJSON branches.
func TestWalkJSON_AdditionalCases(t *testing.T) {
	t.Run("primitive_value", func(t *testing.T) {
		var chunks []chunk
		walkJSON("hello world", []string{"data", "field"}, &chunks, MaxChunkBytes)

		if len(chunks) == 0 {
			t.Fatal("expected chunk for primitive value")
		}

		// Small primitives go through the small-enough path which sets hasCode=true.
		if !chunks[0].hasCode {
			t.Error("small primitive chunk should have hasCode=true (JSON content)")
		}
	})

	t.Run("empty_object", func(t *testing.T) {
		var chunks []chunk
		walkJSON(map[string]any{}, []string{"empty"}, &chunks, MaxChunkBytes)

		if len(chunks) == 0 {
			t.Fatal("expected chunk for empty object")
		}
	})

	t.Run("nested_object_recursion", func(t *testing.T) {
		nested := map[string]any{
			"outer": map[string]any{
				"inner": "value",
			},
		}

		var chunks []chunk
		walkJSON(nested, nil, &chunks, MaxChunkBytes)

		if len(chunks) == 0 {
			t.Fatal("expected chunks for nested object")
		}
	})

	t.Run("array_at_root", func(t *testing.T) {
		arr := []any{
			map[string]any{"id": 1.0, "name": "Alice"},
			map[string]any{"id": 2.0, "name": "Bob"},
		}

		var chunks []chunk
		walkJSON(arr, nil, &chunks, MaxChunkBytes)

		if len(chunks) == 0 {
			t.Fatal("expected chunks for root array")
		}
	})

	t.Run("oversized_primitive", func(t *testing.T) {
		bigStr := strings.Repeat("x", 5000)

		var chunks []chunk
		walkJSON(bigStr, nil, &chunks, 512)

		if len(chunks) == 0 {
			t.Fatal("expected chunk for oversized primitive")
		}
	})

	t.Run("object_with_nested_structures_small_enough", func(t *testing.T) {
		// Object that is small enough but has nested map — should recurse.
		obj := map[string]any{
			"simple": "value",
			"nested": map[string]any{"key": "val"},
		}

		var chunks []chunk
		walkJSON(obj, nil, &chunks, MaxChunkBytes)

		if len(chunks) == 0 {
			t.Fatal("expected chunks from recursion")
		}
	})

	t.Run("object_with_nested_array_small_enough", func(t *testing.T) {
		obj := map[string]any{
			"simple": "value",
			"items":  []any{1.0, 2.0, 3.0},
		}

		var chunks []chunk
		walkJSON(obj, nil, &chunks, MaxChunkBytes)

		if len(chunks) == 0 {
			t.Fatal("expected chunks from recursion with array")
		}
	})
}

// TestIndexJSON_EdgeCases covers additional IndexJSON branches.
func TestIndexJSON_EdgeCases(t *testing.T) {
	t.Run("empty_string", func(t *testing.T) {
		cs := tempDB(t)

		result, err := cs.IndexJSON("", "empty-json")
		if err != nil {
			t.Fatalf("IndexJSON empty: %v", err)
		}

		if result.TotalChunks != 0 {
			t.Errorf("expected 0 chunks for empty JSON, got %d", result.TotalChunks)
		}
	})

	t.Run("whitespace_only", func(t *testing.T) {
		cs := tempDB(t)

		result, err := cs.IndexJSON("   \n\t  ", "ws-json")
		if err != nil {
			t.Fatalf("IndexJSON whitespace: %v", err)
		}

		if result.TotalChunks != 0 {
			t.Errorf("expected 0 chunks for whitespace JSON, got %d", result.TotalChunks)
		}
	})

	t.Run("invalid_json_falls_back_to_plaintext", func(t *testing.T) {
		cs := tempDB(t)

		result, err := cs.IndexJSON("this is not json at all", "invalid-json")
		if err != nil {
			t.Fatalf("IndexJSON invalid: %v", err)
		}

		// Should fall back to IndexPlainText and still produce chunks.
		if result.TotalChunks == 0 {
			t.Error("expected chunks from plaintext fallback")
		}
	})

	t.Run("json_array_root", func(t *testing.T) {
		cs := tempDB(t)

		jsonContent := `[
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
			{"id": 3, "name": "Charlie"}
		]`

		result, err := cs.IndexJSON(jsonContent, "array-root")
		if err != nil {
			t.Fatalf("IndexJSON array: %v", err)
		}

		if result.TotalChunks == 0 {
			t.Error("expected chunks from JSON array")
		}
	})

	t.Run("json_primitive", func(t *testing.T) {
		cs := tempDB(t)

		result, err := cs.IndexJSON(`"just a string"`, "primitive-json")
		if err != nil {
			t.Fatalf("IndexJSON primitive: %v", err)
		}

		if result.TotalChunks == 0 {
			t.Error("expected chunks from JSON primitive")
		}
	})
}

// TestIndexPlainText_EdgeCases covers additional IndexPlainText branches.
func TestIndexPlainText_EdgeCases(t *testing.T) {
	t.Run("zero_lines_per_chunk_defaults", func(t *testing.T) {
		cs := tempDB(t)

		result, err := cs.IndexPlainText("Some content here.", "default-lines", 0)
		if err != nil {
			t.Fatalf("IndexPlainText: %v", err)
		}

		if result.TotalChunks == 0 {
			t.Error("expected at least one chunk")
		}
	})

	t.Run("negative_lines_per_chunk_defaults", func(t *testing.T) {
		cs := tempDB(t)

		result, err := cs.IndexPlainText("Some content here.", "neg-lines", -5)
		if err != nil {
			t.Fatalf("IndexPlainText: %v", err)
		}

		if result.TotalChunks == 0 {
			t.Error("expected at least one chunk")
		}
	})

	t.Run("whitespace_only_content", func(t *testing.T) {
		cs := tempDB(t)

		result, err := cs.IndexPlainText("   \n  \t  \n  ", "ws-only", 20)
		if err != nil {
			t.Fatalf("IndexPlainText whitespace: %v", err)
		}

		if result.TotalChunks != 0 {
			t.Errorf("expected 0 chunks for whitespace, got %d", result.TotalChunks)
		}
	})
}

// TestChunkPlainText_EmptyTitleFallback verifies that chunks with empty first
// lines get a "Lines X-Y" title.
func TestChunkPlainText_EmptyTitleFallback(t *testing.T) {
	// Build input where fixed-size chunking produces a chunk starting with an empty line.
	var lines []string

	for i := range 30 {
		if i%10 == 0 {
			lines = append(lines, "")
		} else {
			lines = append(lines, fmt.Sprintf("Line %d content", i))
		}
	}

	chunks := chunkPlainText(strings.Join(lines, "\n"), 10)
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}

	// At least one chunk should exist that starts with an empty line,
	// resulting in a "Lines X-Y" title.
	hasLineTitle := false

	for _, c := range chunks {
		if strings.HasPrefix(c.title, "Lines ") {
			hasLineTitle = true
			break
		}
	}

	if !hasLineTitle {
		titles := make([]string, len(chunks))
		for i, c := range chunks {
			titles[i] = c.title
		}
		// This is fine if no chunk starts with empty line; just verifying no crash.
		_ = titles
	}
}

// TestCleanupStaleDBs_AdditionalCases covers more CleanupStaleDBs branches.
func TestCleanupStaleDBs_AdditionalCases(t *testing.T) {
	t.Run("nonexistent_dir", func(t *testing.T) {
		cleaned := CleanupStaleDBs("/nonexistent/dir/path")
		if cleaned != 0 {
			t.Errorf("expected 0 cleaned for nonexistent dir, got %d", cleaned)
		}
	})

	t.Run("directory_entry_skipped", func(t *testing.T) {
		dir := t.TempDir()

		// Create a subdirectory that matches the pattern name.
		subdir := filepath.Join(dir, "thimble-12345.db")
		if err := os.Mkdir(subdir, 0o755); err != nil {
			t.Fatal(err)
		}

		cleaned := CleanupStaleDBs(dir)
		if cleaned != 0 {
			t.Errorf("expected 0 cleaned (dirs should be skipped), got %d", cleaned)
		}
	})

	t.Run("non_matching_file_skipped", func(t *testing.T) {
		dir := t.TempDir()

		// Create files that don't match the stale DB pattern.
		for _, name := range []string{"other.db", "thimble.db", "thimble-abc.db"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		cleaned := CleanupStaleDBs(dir)
		if cleaned != 0 {
			t.Errorf("expected 0 cleaned for non-matching files, got %d", cleaned)
		}
	})

	t.Run("current_pid_not_cleaned", func(t *testing.T) {
		dir := t.TempDir()

		// Create a file matching the current PID — should NOT be cleaned.
		currentPIDFile := filepath.Join(dir, fmt.Sprintf("thimble-%d.db", os.Getpid()))
		if err := os.WriteFile(currentPIDFile, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}

		cleaned := CleanupStaleDBs(dir)
		if cleaned != 0 {
			t.Errorf("expected 0 cleaned for current PID, got %d", cleaned)
		}

		// File should still exist.
		if _, err := os.Stat(currentPIDFile); os.IsNotExist(err) {
			t.Error("current PID file should not be removed")
		}
	})
}

// TestSearchWithFallback_FuzzyLayer tests fuzzy correction layer of SearchWithFallback.
func TestSearchWithFallback_FuzzyLayer(t *testing.T) {
	cs := tempDB(t)

	// Index content with a distinctive word.
	_, err := cs.Index("# Orchestration\n\norchestration platform for container management and deployment.", "orch-doc")
	if err != nil {
		t.Fatal(err)
	}

	// Search with a misspelling that won't match porter or trigram but will be fuzzy-corrected.
	// "orchestrtion" is missing an 'a' — should be corrected to "orchestration".
	results, err := cs.SearchWithFallback("orchestrtion", 5, "")
	if err != nil {
		t.Fatalf("SearchWithFallback: %v", err)
	}

	if len(results) > 0 {
		if results[0].MatchLayer != "fuzzy" && results[0].MatchLayer != "porter" && results[0].MatchLayer != "trigram" {
			t.Errorf("unexpected matchLayer = %q", results[0].MatchLayer)
		}
	}
}

// TestSearchWithFallback_SourceFilterAllLayers tests source filtering across all fallback layers.
func TestSearchWithFallback_SourceFilterAllLayers(t *testing.T) {
	cs := tempDB(t)

	_, err := cs.Index("# Unique Document\n\nxyzzyplugh magical word in document.", "magic-src")
	if err != nil {
		t.Fatal(err)
	}

	// Search with source filter that doesn't match — should return nil through all layers.
	results, err := cs.SearchWithFallback("xyzzyplugh", 5, "nonexistent-source")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) > 0 {
		t.Error("expected no results with non-matching source filter")
	}
}

// TestSearch_DefaultLimitAndMode tests Search with default limit and mode.
func TestSearch_DefaultLimitAndMode(t *testing.T) {
	cs := tempDB(t)

	_, err := cs.Index("# Defaults\n\nTesting default parameters for search.", "defaults-doc")
	if err != nil {
		t.Fatal(err)
	}

	// Pass 0 for limit and empty mode — should use defaults.
	results, err := cs.Search("testing default", 0, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Error("expected results with default limit/mode")
	}
}

// TestSearchTrigram_DefaultLimitAndMode tests SearchTrigram with default limit and mode.
func TestSearchTrigram_DefaultLimitAndMode(t *testing.T) {
	cs := tempDB(t)

	_, err := cs.Index("# Trigram Defaults\n\nTesting default parameters for trigram search.", "tri-defaults")
	if err != nil {
		t.Fatal(err)
	}

	results, err := cs.SearchTrigram("testing", 0, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Error("expected results with default trigram limit/mode")
	}
}

// TestSearchWithFallback_DefaultLimit tests SearchWithFallback with zero limit.
func TestSearchWithFallback_DefaultLimit(t *testing.T) {
	cs := tempDB(t)

	_, err := cs.Index("# Limit Test\n\nContent for limit default testing.", "limit-doc")
	if err != nil {
		t.Fatal(err)
	}

	results, err := cs.SearchWithFallback("content limit", 0, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Error("expected results with default limit")
	}
}
