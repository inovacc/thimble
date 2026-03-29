package mcp

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestMain enforces a hard timeout on all mcp package tests to prevent
// zombie test processes from accumulating (integration tests spawn servers).
func TestMain(m *testing.M) {
	timeout := 120 * time.Second

	if v := os.Getenv("TEST_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}

	done := make(chan int, 1)

	go func() {
		done <- m.Run()
	}()

	select {
	case code := <-done:
		os.Exit(code)
	case <-time.After(timeout):
		_, _ = fmt.Fprintf(os.Stderr, "FATAL: mcp tests exceeded %s hard timeout — forcing exit to prevent zombie processes\n", timeout)

		os.Exit(1)
	}
}
