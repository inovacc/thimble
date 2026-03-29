// Package fetch implements URL fetching with HTML-to-Markdown conversion
// using JohannesKaufmann/html-to-markdown (pure Go, replaces turndown).
package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	htmltomd "github.com/JohannesKaufmann/html-to-markdown/v2"
)

const (
	// DefaultTimeout for HTTP requests.
	DefaultTimeout = 30 * time.Second
	// DefaultMaxBytes limits response body size.
	DefaultMaxBytes = 5 * 1024 * 1024 // 5MB
	// DefaultUserAgent identifies the fetcher.
	DefaultUserAgent = "Thimble/1.0 (context-mode)"
)

// Result of fetching a URL.
type Result struct {
	URL          string `json:"url"`
	Content      string `json:"content"`
	ContentType  string `json:"contentType"`
	StatusCode   int    `json:"statusCode"`
	BytesFetched int    `json:"bytesFetched"`
}

// Options configures the fetcher.
type Options struct {
	Timeout  time.Duration
	MaxBytes int64
}

// Fetch retrieves a URL and converts HTML to Markdown.
func Fetch(ctx context.Context, url string, opts *Options) (*Result, error) {
	timeout := DefaultTimeout
	maxBytes := int64(DefaultMaxBytes)

	if opts != nil {
		if opts.Timeout > 0 {
			timeout = opts.Timeout
		}

		if opts.MaxBytes > 0 {
			maxBytes = opts.MaxBytes
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,application/json,*/*;q=0.8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}

	defer func() { _ = resp.Body.Close() }()

	limited := io.LimitReader(resp.Body, maxBytes)

	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	content := string(body)

	// Convert HTML to Markdown.
	if isHTML(contentType) {
		md, err := htmltomd.ConvertString(content)
		if err == nil && md != "" {
			content = md
		}
	}

	return &Result{
		URL:          url,
		Content:      content,
		ContentType:  contentType,
		StatusCode:   resp.StatusCode,
		BytesFetched: len(body),
	}, nil
}

func isHTML(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}
