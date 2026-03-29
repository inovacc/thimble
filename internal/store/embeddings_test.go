package store

import (
	"context"
	"math"
	"testing"
)

func TestNewEmbeddingProviderNilWhenEnvNotSet(t *testing.T) {
	// Ensure env vars are not set (they shouldn't be in CI).
	t.Setenv("THIMBLE_EMBEDDING_URL", "")

	p := NewEmbeddingProvider()
	if p != nil {
		t.Error("expected nil provider when THIMBLE_EMBEDDING_URL is not set")
	}
}

func TestNewEmbeddingProviderReturnsProviderWhenEnvSet(t *testing.T) {
	t.Setenv("THIMBLE_EMBEDDING_URL", "http://localhost:11434/v1")
	t.Setenv("THIMBLE_EMBEDDING_KEY", "test-key")
	t.Setenv("THIMBLE_EMBEDDING_MODEL", "nomic-embed-text")

	p := NewEmbeddingProvider()
	if p == nil {
		t.Fatal("expected non-nil provider")
	}

	if p.APIURL() != "http://localhost:11434/v1" {
		t.Errorf("apiURL = %q, want %q", p.APIURL(), "http://localhost:11434/v1")
	}

	if p.Model() != "nomic-embed-text" {
		t.Errorf("model = %q, want %q", p.Model(), "nomic-embed-text")
	}

	if p.apiKey != "test-key" {
		t.Errorf("apiKey = %q, want %q", p.apiKey, "test-key")
	}
}

func TestNewEmbeddingProviderDefaultModel(t *testing.T) {
	t.Setenv("THIMBLE_EMBEDDING_URL", "https://api.openai.com/v1")
	t.Setenv("THIMBLE_EMBEDDING_MODEL", "")

	p := NewEmbeddingProvider()
	if p == nil {
		t.Fatal("expected non-nil provider")
	}

	if p.Model() != "text-embedding-3-small" {
		t.Errorf("default model = %q, want %q", p.Model(), "text-embedding-3-small")
	}
}

func TestNewEmbeddingProviderTrimsTrailingSlash(t *testing.T) {
	t.Setenv("THIMBLE_EMBEDDING_URL", "http://localhost:11434/v1/")

	p := NewEmbeddingProvider()
	if p == nil {
		t.Fatal("expected non-nil provider")
	}

	if p.APIURL() != "http://localhost:11434/v1" {
		t.Errorf("apiURL = %q, want trailing slash trimmed", p.APIURL())
	}
}

func TestEncodeDecodeFloat64s(t *testing.T) {
	tests := []struct {
		name string
		vec  []float64
	}{
		{"empty", nil},
		{"single", []float64{1.5}},
		{"typical", []float64{0.1, -0.2, 0.3, 0.0, -1.0, math.Pi}},
		{"zeros", []float64{0, 0, 0, 0}},
		{"extremes", []float64{math.MaxFloat64, math.SmallestNonzeroFloat64, -math.MaxFloat64}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blob := encodeFloat64s(tt.vec)

			// Verify blob length.
			expectedLen := len(tt.vec) * 8
			if len(blob) != expectedLen {
				t.Errorf("blob length = %d, want %d", len(blob), expectedLen)
			}

			decoded := decodeFloat64s(blob)
			if len(decoded) != len(tt.vec) {
				t.Fatalf("decoded length = %d, want %d", len(decoded), len(tt.vec))
			}

			for i := range tt.vec {
				if decoded[i] != tt.vec[i] {
					t.Errorf("decoded[%d] = %v, want %v", i, decoded[i], tt.vec[i])
				}
			}
		})
	}
}

func TestVectorCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []float64
		want float64
		tol  float64
	}{
		{
			name: "identical",
			a:    []float64{1, 0, 0},
			b:    []float64{1, 0, 0},
			want: 1.0,
			tol:  1e-10,
		},
		{
			name: "orthogonal",
			a:    []float64{1, 0, 0},
			b:    []float64{0, 1, 0},
			want: 0.0,
			tol:  1e-10,
		},
		{
			name: "opposite",
			a:    []float64{1, 0, 0},
			b:    []float64{-1, 0, 0},
			want: -1.0,
			tol:  1e-10,
		},
		{
			name: "45 degrees",
			a:    []float64{1, 0},
			b:    []float64{1, 1},
			want: 1.0 / math.Sqrt(2),
			tol:  1e-10,
		},
		{
			name: "empty a",
			a:    []float64{},
			b:    []float64{1, 0},
			want: 0.0,
			tol:  1e-10,
		},
		{
			name: "length mismatch",
			a:    []float64{1, 0},
			b:    []float64{1, 0, 0},
			want: 0.0,
			tol:  1e-10,
		},
		{
			name: "zero vector",
			a:    []float64{0, 0, 0},
			b:    []float64{1, 2, 3},
			want: 0.0,
			tol:  1e-10,
		},
		{
			name: "scaled identical",
			a:    []float64{1, 2, 3},
			b:    []float64{2, 4, 6},
			want: 1.0,
			tol:  1e-10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vectorCosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > tt.tol {
				t.Errorf("vectorCosineSimilarity = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmbeddingSearchFallsBackWhenProviderNil(t *testing.T) {
	cs := tempDB(t)

	// Index content.
	_, err := cs.Index("# Auth\nOAuth2 tokens for login", "auth-docs")
	if err != nil {
		t.Fatalf("index: %v", err)
	}

	// EmbeddingSearch with nil provider should fall back to SemanticSearch.
	results, err := cs.EmbeddingSearch(context.Background(), "authentication tokens", 5, "", nil)
	if err != nil {
		t.Fatalf("EmbeddingSearch: %v", err)
	}

	// Should get results from SemanticSearch fallback.
	if len(results) == 0 {
		t.Log("no results from TF-IDF fallback (acceptable if no vocabulary overlap)")
	} else if results[0].MatchLayer != "semantic" {
		t.Errorf("expected matchLayer=semantic from fallback, got %q", results[0].MatchLayer)
	}
}

func TestEmbeddingSearchFallsBackWhenNoEmbeddingsStored(t *testing.T) {
	cs := tempDB(t)

	// Index content WITHOUT embedder set — no embeddings stored.
	_, err := cs.Index("# Database\nSQL schema migrations and rollback", "db-docs")
	if err != nil {
		t.Fatalf("index: %v", err)
	}

	// Create a provider (won't be used for search since no embeddings exist).
	t.Setenv("THIMBLE_EMBEDDING_URL", "http://localhost:99999/v1")

	provider := NewEmbeddingProvider()

	// Even with a provider, should fall back to SemanticSearch because
	// the DB has no embeddings (the query embedding call will fail since
	// there's no real server, so it falls back to TF-IDF).
	results, err := cs.EmbeddingSearch(context.Background(), "database schema", 5, "", provider)
	if err != nil {
		t.Fatalf("EmbeddingSearch: %v", err)
	}

	if len(results) > 0 && results[0].MatchLayer == "embedding" {
		t.Error("did not expect embedding match layer with no real embedding server")
	}
}

func TestChunkEmbeddingsTableCreated(t *testing.T) {
	cs := tempDB(t)

	// Verify the chunk_embeddings table exists by inserting/querying.
	_, err := cs.db.Exec("INSERT INTO chunk_embeddings (chunk_rowid, embedding) VALUES (?, ?)", 999, encodeFloat64s([]float64{0.1, 0.2}))
	if err != nil {
		t.Fatalf("insert into chunk_embeddings: %v", err)
	}

	var blob []byte

	err = cs.db.QueryRow("SELECT embedding FROM chunk_embeddings WHERE chunk_rowid = ?", 999).Scan(&blob)
	if err != nil {
		t.Fatalf("query chunk_embeddings: %v", err)
	}

	decoded := decodeFloat64s(blob)
	if len(decoded) != 2 || decoded[0] != 0.1 || decoded[1] != 0.2 {
		t.Errorf("round-trip failed: got %v", decoded)
	}
}

func TestSetEmbeddingProvider(t *testing.T) {
	cs := tempDB(t)

	// Initially nil.
	if cs.embedder != nil {
		t.Error("expected nil embedder initially")
	}

	t.Setenv("THIMBLE_EMBEDDING_URL", "http://localhost:11434/v1")

	p := NewEmbeddingProvider()

	cs.SetEmbeddingProvider(p)

	if cs.embedder == nil {
		t.Error("expected non-nil embedder after SetEmbeddingProvider")
	}

	if cs.embedder != p {
		t.Error("embedder should be the same pointer")
	}

	// Setting nil clears it.
	cs.SetEmbeddingProvider(nil)

	if cs.embedder != nil {
		t.Error("expected nil embedder after setting nil")
	}
}
