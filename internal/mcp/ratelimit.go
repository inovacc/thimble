package mcp

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RateLimiter implements a token bucket algorithm for rate limiting MCP tool
// invocations. It supports both global rate limiting (across all tools) and
// per-tool rate limiting (each tool gets its own bucket).
type RateLimiter struct {
	mu      sync.Mutex
	global  *tokenBucket
	perTool map[string]*tokenBucket
	rate    float64
	burst   int
}

// tokenBucket implements a simple token bucket for rate limiting.
type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	rate       float64 // tokens per second
	lastRefill time.Time
}

func newTokenBucket(rate float64, burst int) *tokenBucket {
	return &tokenBucket{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		rate:       rate,
		lastRefill: time.Now(),
	}
}

// allow checks if a token is available and consumes one if so.
func (tb *tokenBucket) allow(now time.Time) bool {
	// Refill tokens based on elapsed time.
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.rate

	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}

	tb.lastRefill = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}

	return false
}

// NewRateLimiter creates a rate limiter with the given rate (tokens per second)
// and burst size. If ratePerSecond is 0, the limiter is effectively disabled
// (Allow always returns true).
func NewRateLimiter(ratePerSecond float64, burst int) *RateLimiter {
	if burst < 1 {
		burst = 1
	}

	return &RateLimiter{
		global:  newTokenBucket(ratePerSecond, burst),
		perTool: make(map[string]*tokenBucket),
		rate:    ratePerSecond,
		burst:   burst,
	}
}

// Allow checks whether a tool invocation is permitted under the per-tool
// rate limit. Each tool has its own independent token bucket.
func (rl *RateLimiter) Allow(toolName string) bool {
	if rl == nil || rl.rate <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	tb, ok := rl.perTool[toolName]
	if !ok {
		tb = newTokenBucket(rl.rate, rl.burst)
		rl.perTool[toolName] = tb
	}

	return tb.allow(time.Now())
}

// AllowGlobal checks whether a tool invocation is permitted under the global
// rate limit shared across all tools.
func (rl *RateLimiter) AllowGlobal() bool {
	if rl == nil || rl.rate <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	return rl.global.allow(time.Now())
}

// RateLimitFromEnv creates a RateLimiter from environment variables.
// THIMBLE_RATE_LIMIT: calls per second (default 0 = disabled).
// THIMBLE_RATE_BURST: burst size (default 20).
// Returns nil if rate limiting is disabled (rate=0).
func RateLimitFromEnv() *RateLimiter {
	rateStr := os.Getenv("THIMBLE_RATE_LIMIT")
	burstStr := os.Getenv("THIMBLE_RATE_BURST")

	rate, err := strconv.ParseFloat(rateStr, 64)
	if err != nil || rate <= 0 {
		return nil
	}

	burst := 20

	if b, err := strconv.Atoi(burstStr); err == nil && b > 0 {
		burst = b
	}

	return NewRateLimiter(rate, burst)
}

// rateLimitResult returns a CallToolResult indicating the rate limit was exceeded.
func rateLimitResult(toolName string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: fmt.Sprintf("rate limit exceeded for tool %q — try again shortly", toolName),
			},
		},
		IsError: true,
	}
}

// withRateLimit wraps a typed tool handler with rate limit checks.
// If the Bridge's rateLimiter is nil or allows the request, the original
// handler is invoked. Otherwise, an error result is returned immediately.
func withRateLimit[T any](b *Bridge, toolName string, handler func(context.Context, *mcpsdk.CallToolRequest, T) (*mcpsdk.CallToolResult, struct{}, error)) func(context.Context, *mcpsdk.CallToolRequest, T) (*mcpsdk.CallToolResult, struct{}, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, input T) (*mcpsdk.CallToolResult, struct{}, error) {
		if b.rateLimiter != nil {
			if !b.rateLimiter.AllowGlobal() || !b.rateLimiter.Allow(toolName) {
				return rateLimitResult(toolName), struct{}{}, nil
			}
		}

		return handler(ctx, req, input)
	}
}
