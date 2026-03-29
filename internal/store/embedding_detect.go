package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// localProvider describes a local embedding API provider to probe.
type localProvider struct {
	name     string
	probeURL string // URL to check for availability
	apiURL   string // base URL for the OpenAI-compatible API
	model    string // default model to use (empty = pick from response)
}

// ollamaTagsResponse is the JSON response from Ollama's /api/tags endpoint.
type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// openAIModelsResponse is the JSON response from an OpenAI-compatible /v1/models endpoint.
type openAIModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// knownProviders lists the local providers to probe, in priority order.
var knownProviders = []localProvider{
	{
		name:     "ollama",
		probeURL: "http://localhost:11434/api/tags",
		apiURL:   "http://localhost:11434/v1",
		model:    "nomic-embed-text",
	},
	{
		name:     "lmstudio",
		probeURL: "http://localhost:1234/v1/models",
		apiURL:   "http://localhost:1234/v1",
		model:    "", // pick first embedding model
	},
	{
		name:     "localai",
		probeURL: "http://localhost:8080/v1/models",
		apiURL:   "http://localhost:8080/v1",
		model:    "", // pick first model
	},
}

// DetectEmbeddingProvider probes common local embedding providers and returns
// a configured EmbeddingProvider for the first one that responds. Returns nil
// if no providers are found. Each probe has a 2-second timeout.
//
// Manual env vars (THIMBLE_EMBEDDING_URL) always take priority — callers should
// check those first before calling this function.
func DetectEmbeddingProvider(ctx context.Context) *EmbeddingProvider {
	for _, lp := range knownProviders {
		if ep := probeProvider(ctx, lp); ep != nil {
			return ep
		}
	}

	return nil
}

// probeProvider checks if a local provider is available and returns a configured
// EmbeddingProvider if so. Returns nil on any failure.
func probeProvider(ctx context.Context, lp localProvider) *EmbeddingProvider {
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 2 * time.Second}

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, lp.probeURL, nil)
	if err != nil {
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	model := lp.model

	// If no default model, try to pick one from the response.
	if model == "" {
		model = pickModelFromResponse(resp, lp.name)
	}

	if model == "" {
		return nil
	}

	return &EmbeddingProvider{
		apiURL: lp.apiURL,
		model:  model,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// pickModelFromResponse tries to extract an embedding model name from the
// provider's probe response body.
func pickModelFromResponse(resp *http.Response, providerName string) string {
	switch providerName {
	case "ollama":
		return pickOllamaModel(resp)
	case "lmstudio", "localai":
		return pickOpenAIModel(resp)
	default:
		return ""
	}
}

// pickOllamaModel extracts the first embedding model from an Ollama /api/tags response.
func pickOllamaModel(resp *http.Response) string {
	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return ""
	}

	// Prefer models with "embed" in the name.
	for _, m := range tags.Models {
		if strings.Contains(strings.ToLower(m.Name), "embed") {
			return m.Name
		}
	}

	// Fall back to first model.
	if len(tags.Models) > 0 {
		return tags.Models[0].Name
	}

	return ""
}

// pickOpenAIModel extracts the first model from an OpenAI-compatible /v1/models response.
// Prefers models with "embed" in the ID.
func pickOpenAIModel(resp *http.Response) string {
	var models openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return ""
	}

	// Prefer embedding models.
	for _, m := range models.Data {
		if strings.Contains(strings.ToLower(m.ID), "embed") {
			return m.ID
		}
	}

	// Fall back to first model.
	if len(models.Data) > 0 {
		return models.Data[0].ID
	}

	return ""
}

// DetectedProviderInfo returns a human-readable description of a detected provider.
func DetectedProviderInfo(p *EmbeddingProvider) string {
	if p == nil {
		return "none"
	}

	return fmt.Sprintf("url=%s model=%s", p.apiURL, p.model)
}
