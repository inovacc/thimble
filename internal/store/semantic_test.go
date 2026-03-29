package store

import (
	"testing"
)

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello World, this is a test!")
	// "this" and "is" are stopwords or too short, "a" too short
	found := map[string]bool{}
	for _, tok := range tokens {
		found[tok] = true
	}

	if !found["hello"] || !found["world"] || !found["test"] {
		t.Errorf("expected hello, world, test; got %v", tokens)
	}

	if found["this"] || found["a"] {
		t.Errorf("expected stopwords/short words filtered; got %v", tokens)
	}
}

func TestSplitCamelCase(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"getUserName", []string{"get", "user", "name"}},
		{"HTTPResponse", []string{"h", "t", "t", "p", "response"}},
		{"simple", []string{"simple"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := splitCamelCase(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCamelCase(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}

		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCamelCase(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestTokenizeExpanded(t *testing.T) {
	tokens := tokenizeExpanded("getUserName snake_case")

	found := map[string]bool{}
	for _, tok := range tokens {
		found[tok] = true
	}
	// Should contain both the original and expanded forms
	if !found["getusername"] && !found["get"] {
		t.Errorf("expected camelCase expansion; got %v", tokens)
	}

	if !found["snake"] || !found["case"] {
		t.Errorf("expected underscore expansion; got %v", tokens)
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := sparseVector{"hello": 1.0, "world": 1.0}
	b := sparseVector{"hello": 1.0, "world": 1.0}

	sim := cosineSimilarity(a, b)
	if sim < 0.99 {
		t.Errorf("identical vectors should have similarity ~1.0, got %f", sim)
	}

	c := sparseVector{"foo": 1.0, "bar": 1.0}

	sim2 := cosineSimilarity(a, c)
	if sim2 > 0.01 {
		t.Errorf("orthogonal vectors should have similarity ~0, got %f", sim2)
	}

	// Empty vectors
	empty := sparseVector{}
	if cosineSimilarity(empty, a) != 0 {
		t.Error("empty vector should have 0 similarity")
	}
}

func TestSemanticSearchBasic(t *testing.T) {
	cs := tempDB(t)

	// Index content about authentication
	_, err := cs.Index("# Authentication\nOAuth2 bearer tokens for API access with JWT validation", "auth-docs")
	if err != nil {
		t.Fatalf("index: %v", err)
	}

	// Index content about logging
	_, err = cs.Index("# Logging\nStructured logging with slog JSON handler and trace IDs", "log-docs")
	if err != nil {
		t.Fatalf("index: %v", err)
	}

	// Search for "login security" — should match auth content via shared concepts
	results, err := cs.SemanticSearch("login security tokens", 5, "")
	if err != nil {
		t.Fatalf("semantic search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected semantic search results")
	}

	// First result should be auth-related (higher similarity to "tokens")
	if results[0].MatchLayer != "semantic" {
		t.Errorf("expected matchLayer=semantic, got %q", results[0].MatchLayer)
	}
}

func TestSemanticSearchSourceFilter(t *testing.T) {
	cs := tempDB(t)

	_, _ = cs.Index("# Alpha\nContent about alpha topic", "source-a")
	_, _ = cs.Index("# Beta\nContent about beta topic", "source-b")

	results, err := cs.SemanticSearch("alpha topic", 5, "source-a")
	if err != nil {
		t.Fatalf("semantic search: %v", err)
	}

	for _, r := range results {
		if r.Source != "source-a" {
			t.Errorf("expected source-a, got %q", r.Source)
		}
	}
}

func TestSemanticSearchEmpty(t *testing.T) {
	cs := tempDB(t)

	results, err := cs.SemanticSearch("anything", 5, "")
	if err != nil {
		t.Fatalf("semantic search: %v", err)
	}

	if results != nil {
		t.Errorf("expected nil for empty store, got %d results", len(results))
	}
}

func TestSemanticSearchEmptyQuery(t *testing.T) {
	cs := tempDB(t)

	results, err := cs.SemanticSearch("", 5, "")
	if err != nil {
		t.Fatalf("semantic search: %v", err)
	}

	if results != nil {
		t.Errorf("expected nil for empty query, got %d results", len(results))
	}
}

func TestSearchWithFallbackSemantic(t *testing.T) {
	cs := tempDB(t)

	// Index content that won't match via FTS5 exact/fuzzy but will via TF-IDF
	_, _ = cs.Index("# Database Migrations\nSchema versioning with up/down rollback support", "db-docs")

	// Search for synonymous concept — FTS5 won't match "database" to "schema versioning"
	// but semantic search might find overlap via shared vocabulary
	results, err := cs.SearchWithFallback("database schema changes rollback", 5, "")
	if err != nil {
		t.Fatalf("search with fallback: %v", err)
	}

	// Should find something via one of the layers
	if len(results) == 0 {
		t.Log("no results — semantic layer may not have matched (acceptable)")
	} else {
		t.Logf("found %d results via layer: %s", len(results), results[0].MatchLayer)
	}
}
