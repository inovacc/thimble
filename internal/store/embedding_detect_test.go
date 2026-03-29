package store

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectEmbeddingProvider_Ollama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.NotFound(w, r)
			return
		}

		resp := ollamaTagsResponse{
			Models: []struct {
				Name string `json:"name"`
			}{
				{Name: "llama3"},
				{Name: "nomic-embed-text"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// Override known providers to point to the test server.
	orig := knownProviders
	knownProviders = []localProvider{
		{
			name:     "ollama",
			probeURL: srv.URL + "/api/tags",
			apiURL:   srv.URL + "/v1",
			model:    "nomic-embed-text",
		},
	}

	defer func() { knownProviders = orig }()

	p := DetectEmbeddingProvider(context.Background())
	if p == nil {
		t.Fatal("expected non-nil provider for Ollama")
	}

	if p.APIURL() != srv.URL+"/v1" {
		t.Errorf("apiURL = %q, want %q", p.APIURL(), srv.URL+"/v1")
	}

	if p.Model() != "nomic-embed-text" {
		t.Errorf("model = %q, want %q", p.Model(), "nomic-embed-text")
	}
}

func TestDetectEmbeddingProvider_LMStudio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}

		resp := openAIModelsResponse{
			Data: []struct {
				ID      string `json:"id"`
				OwnedBy string `json:"owned_by"`
			}{
				{ID: "text-embedding-nomic-embed-text-v1.5", OwnedBy: "lmstudio"},
				{ID: "llama-3.1-8b", OwnedBy: "lmstudio"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	orig := knownProviders
	knownProviders = []localProvider{
		{
			name:     "lmstudio",
			probeURL: srv.URL + "/v1/models",
			apiURL:   srv.URL + "/v1",
			model:    "", // should pick from response
		},
	}

	defer func() { knownProviders = orig }()

	p := DetectEmbeddingProvider(context.Background())
	if p == nil {
		t.Fatal("expected non-nil provider for LM Studio")
	}

	if p.Model() != "text-embedding-nomic-embed-text-v1.5" {
		t.Errorf("model = %q, want %q", p.Model(), "text-embedding-nomic-embed-text-v1.5")
	}
}

func TestDetectEmbeddingProvider_LocalAI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}

		resp := openAIModelsResponse{
			Data: []struct {
				ID      string `json:"id"`
				OwnedBy string `json:"owned_by"`
			}{
				{ID: "bert-embeddings", OwnedBy: "localai"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	orig := knownProviders
	knownProviders = []localProvider{
		{
			name:     "localai",
			probeURL: srv.URL + "/v1/models",
			apiURL:   srv.URL + "/v1",
			model:    "",
		},
	}

	defer func() { knownProviders = orig }()

	p := DetectEmbeddingProvider(context.Background())
	if p == nil {
		t.Fatal("expected non-nil provider for LocalAI")
	}

	if p.Model() != "bert-embeddings" {
		t.Errorf("model = %q, want %q", p.Model(), "bert-embeddings")
	}
}

func TestDetectEmbeddingProvider_NoneAvailable(t *testing.T) {
	orig := knownProviders
	knownProviders = []localProvider{
		{
			name:     "fake",
			probeURL: "http://127.0.0.1:1/nonexistent",
			apiURL:   "http://127.0.0.1:1/v1",
			model:    "test",
		},
	}

	defer func() { knownProviders = orig }()

	p := DetectEmbeddingProvider(context.Background())
	if p != nil {
		t.Error("expected nil provider when no providers available")
	}
}

func TestDetectEmbeddingProvider_ServerReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := knownProviders
	knownProviders = []localProvider{
		{
			name:     "ollama",
			probeURL: srv.URL + "/api/tags",
			apiURL:   srv.URL + "/v1",
			model:    "nomic-embed-text",
		},
	}

	defer func() { knownProviders = orig }()

	p := DetectEmbeddingProvider(context.Background())
	if p != nil {
		t.Error("expected nil provider when server returns error")
	}
}

func TestDetectEmbeddingProvider_EmptyModelList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}

		resp := openAIModelsResponse{Data: nil}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	orig := knownProviders
	knownProviders = []localProvider{
		{
			name:     "lmstudio",
			probeURL: srv.URL + "/v1/models",
			apiURL:   srv.URL + "/v1",
			model:    "", // needs to pick from response
		},
	}

	defer func() { knownProviders = orig }()

	p := DetectEmbeddingProvider(context.Background())
	if p != nil {
		t.Error("expected nil provider when model list is empty")
	}
}

func TestDetectEmbeddingProvider_PriorityOrder(t *testing.T) {
	// Both servers respond, but Ollama should win (it's first).
	ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := ollamaTagsResponse{
			Models: []struct {
				Name string `json:"name"`
			}{{Name: "nomic-embed-text"}},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ollamaSrv.Close()

	lmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := openAIModelsResponse{
			Data: []struct {
				ID      string `json:"id"`
				OwnedBy string `json:"owned_by"`
			}{{ID: "lm-embed", OwnedBy: "lmstudio"}},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer lmSrv.Close()

	orig := knownProviders
	knownProviders = []localProvider{
		{
			name:     "ollama",
			probeURL: ollamaSrv.URL + "/api/tags",
			apiURL:   ollamaSrv.URL + "/v1",
			model:    "nomic-embed-text",
		},
		{
			name:     "lmstudio",
			probeURL: lmSrv.URL + "/v1/models",
			apiURL:   lmSrv.URL + "/v1",
			model:    "",
		},
	}

	defer func() { knownProviders = orig }()

	p := DetectEmbeddingProvider(context.Background())
	if p == nil {
		t.Fatal("expected non-nil provider")
	}

	if p.APIURL() != ollamaSrv.URL+"/v1" {
		t.Errorf("expected Ollama to win priority, got apiURL = %q", p.APIURL())
	}
}

func TestDetectedProviderInfo(t *testing.T) {
	if got := DetectedProviderInfo(nil); got != "none" {
		t.Errorf("DetectedProviderInfo(nil) = %q, want %q", got, "none")
	}

	p := &EmbeddingProvider{apiURL: "http://localhost:11434/v1", model: "nomic-embed-text"}
	got := DetectedProviderInfo(p)
	want := "url=http://localhost:11434/v1 model=nomic-embed-text"

	if got != want {
		t.Errorf("DetectedProviderInfo = %q, want %q", got, want)
	}
}

func TestPickOllamaModel_PrefersEmbedding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := ollamaTagsResponse{
			Models: []struct {
				Name string `json:"name"`
			}{
				{Name: "llama3"},
				{Name: "mxbai-embed-large"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	orig := knownProviders
	knownProviders = []localProvider{
		{
			name:     "ollama",
			probeURL: srv.URL + "/api/tags",
			apiURL:   srv.URL + "/v1",
			model:    "", // force model picking
		},
	}

	defer func() { knownProviders = orig }()

	p := DetectEmbeddingProvider(context.Background())
	if p == nil {
		t.Fatal("expected non-nil provider")
	}

	if p.Model() != "mxbai-embed-large" {
		t.Errorf("model = %q, want %q", p.Model(), "mxbai-embed-large")
	}
}
