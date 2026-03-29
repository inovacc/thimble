package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchPlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer srv.Close()

	result, err := Fetch(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if result.Content != "hello world" {
		t.Errorf("Content = %q, want %q", result.Content, "hello world")
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}

	if result.BytesFetched != 11 {
		t.Errorf("BytesFetched = %d, want 11", result.BytesFetched)
	}

	if result.URL != srv.URL {
		t.Errorf("URL = %q, want %q", result.URL, srv.URL)
	}
}

func TestFetchHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<h1>Title</h1><p>Body text</p>"))
	}))
	defer srv.Close()

	result, err := Fetch(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// Should have been converted to markdown.
	if result.Content == "<h1>Title</h1><p>Body text</p>" {
		t.Error("HTML should have been converted to markdown")
	}
}

func TestFetchWithOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("data"))
	}))
	defer srv.Close()

	result, err := Fetch(context.Background(), srv.URL, &Options{
		Timeout:  5 * time.Second,
		MaxBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if result.Content != "data" {
		t.Errorf("Content = %q, want %q", result.Content, "data")
	}
}

func TestFetchMaxBytesLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("abcdefghij")) // 10 bytes
	}))
	defer srv.Close()

	result, err := Fetch(context.Background(), srv.URL, &Options{
		MaxBytes: 5,
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if result.BytesFetched != 5 {
		t.Errorf("BytesFetched = %d, want 5 (limited)", result.BytesFetched)
	}
}

func TestFetchContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Second)

		_, _ = w.Write([]byte("slow"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := Fetch(ctx, srv.URL, &Options{Timeout: 1 * time.Second})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestFetchInvalidURL(t *testing.T) {
	_, err := Fetch(context.Background(), "://invalid", nil)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestFetchServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	}))
	defer srv.Close()

	result, err := Fetch(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// Should still return the response (not error on non-2xx).
	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", result.StatusCode)
	}
}

func TestFetchUserAgent(t *testing.T) {
	var gotUA string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if gotUA != DefaultUserAgent {
		t.Errorf("User-Agent = %q, want %q", gotUA, DefaultUserAgent)
	}
}

func TestIsHTML(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"text/html", true},
		{"text/html; charset=utf-8", true},
		{"application/xhtml+xml", true},
		{"text/plain", false},
		{"application/json", false},
		{"", false},
	}

	for _, tc := range tests {
		if got := isHTML(tc.ct); got != tc.want {
			t.Errorf("isHTML(%q) = %v, want %v", tc.ct, got, tc.want)
		}
	}
}
