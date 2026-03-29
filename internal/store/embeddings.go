package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// EmbeddingProvider generates vector embeddings for text chunks.
// When configured (via THIMBLE_EMBEDDING_URL), embeddings are computed
// by calling an OpenAI-compatible /v1/embeddings endpoint.
type EmbeddingProvider struct {
	apiURL string // e.g., "http://localhost:11434/v1" or "https://api.openai.com/v1"
	apiKey string // from THIMBLE_EMBEDDING_KEY env var
	model  string // from THIMBLE_EMBEDDING_MODEL env var (default: "text-embedding-3-small")
	dim    int    // embedding dimension (0 = use API default)
	client *http.Client
}

// embeddingRequest is the JSON body sent to the /embeddings endpoint.
type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embeddingResponse is the JSON response from the /embeddings endpoint.
type embeddingResponse struct {
	Data []embeddingData `json:"data"`
}

type embeddingData struct {
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// NewEmbeddingProvider creates a provider from environment variables.
// Returns nil if THIMBLE_EMBEDDING_URL is not set.
func NewEmbeddingProvider() *EmbeddingProvider {
	apiURL := os.Getenv("THIMBLE_EMBEDDING_URL")
	if apiURL == "" {
		return nil
	}

	// Normalize: strip trailing slash.
	apiURL = strings.TrimRight(apiURL, "/")

	model := os.Getenv("THIMBLE_EMBEDDING_MODEL")
	if model == "" {
		model = "text-embedding-3-small"
	}

	return &EmbeddingProvider{
		apiURL: apiURL,
		apiKey: os.Getenv("THIMBLE_EMBEDDING_KEY"),
		model:  model,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Embed returns the embedding vector for a text string.
func (p *EmbeddingProvider) Embed(ctx context.Context, text string) ([]float64, error) {
	vecs, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(vecs) == 0 {
		return nil, fmt.Errorf("embedding API returned no vectors")
	}

	return vecs[0], nil
}

// EmbedBatch returns embeddings for multiple texts in one API call.
func (p *EmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := embeddingRequest{
		Model: p.model,
		Input: texts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	url := p.apiURL + "/embeddings"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding API call: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API returned status %d", resp.StatusCode)
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}

	// Sort by index to ensure correct ordering.
	vecs := make([][]float64, len(texts))
	for _, d := range result.Data {
		if d.Index >= 0 && d.Index < len(vecs) {
			vecs[d.Index] = d.Embedding
		}
	}

	// Validate all slots filled.
	for i, v := range vecs {
		if v == nil {
			return nil, fmt.Errorf("embedding API missing vector for input %d", i)
		}
	}

	// Record dimension from first response if not set.
	if p.dim == 0 && len(vecs) > 0 {
		p.dim = len(vecs[0])
	}

	return vecs, nil
}

// Model returns the configured model name.
func (p *EmbeddingProvider) Model() string { return p.model }

// APIURL returns the configured API URL.
func (p *EmbeddingProvider) APIURL() string { return p.apiURL }
