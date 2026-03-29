package mcp

import (
	"testing"
	"time"
)

func TestRateLimiter_NilIsDisabled(t *testing.T) {
	var rl *RateLimiter
	if !rl.Allow("any_tool") {
		t.Fatal("nil RateLimiter should always allow")
	}

	if !rl.AllowGlobal() {
		t.Fatal("nil RateLimiter should always allow global")
	}
}

func TestRateLimiter_ZeroRateDisabled(t *testing.T) {
	rl := NewRateLimiter(0, 10)
	for range 100 {
		if !rl.Allow("tool") {
			t.Fatal("zero-rate limiter should always allow")
		}

		if !rl.AllowGlobal() {
			t.Fatal("zero-rate limiter should always allow global")
		}
	}
}

func TestRateLimiter_AllowsBurst(t *testing.T) {
	rl := NewRateLimiter(1, 5) // 1/sec, burst of 5

	for i := range 5 {
		if !rl.Allow("tool_a") {
			t.Fatalf("burst call %d should be allowed", i)
		}
	}
}

func TestRateLimiter_BlocksAfterBurstExhausted(t *testing.T) {
	rl := NewRateLimiter(1, 3) // 1/sec, burst of 3

	// Exhaust the burst.
	for range 3 {
		if !rl.Allow("tool_a") {
			t.Fatal("burst should be allowed")
		}
	}

	// Next call should be blocked.
	if rl.Allow("tool_a") {
		t.Fatal("should be blocked after burst exhausted")
	}
}

func TestRateLimiter_GlobalBlocksAfterBurstExhausted(t *testing.T) {
	rl := NewRateLimiter(1, 2)

	// Exhaust global burst.
	for range 2 {
		if !rl.AllowGlobal() {
			t.Fatal("global burst should be allowed")
		}
	}

	if rl.AllowGlobal() {
		t.Fatal("global should be blocked after burst exhausted")
	}
}

func TestRateLimiter_PerToolIsolation(t *testing.T) {
	rl := NewRateLimiter(1, 2) // 1/sec, burst of 2

	// Exhaust tool_a's burst.
	for range 2 {
		if !rl.Allow("tool_a") {
			t.Fatal("tool_a burst should be allowed")
		}
	}

	if rl.Allow("tool_a") {
		t.Fatal("tool_a should be blocked")
	}

	// tool_b should still have its own burst.
	if !rl.Allow("tool_b") {
		t.Fatal("tool_b should have its own bucket and still be allowed")
	}
}

func TestRateLimiter_RefillsOverTime(t *testing.T) {
	rl := NewRateLimiter(100, 1) // 100/sec, burst of 1

	// Use the single token.
	if !rl.Allow("tool") {
		t.Fatal("first call should be allowed")
	}

	if rl.Allow("tool") {
		t.Fatal("second call should be blocked (burst=1)")
	}

	// Wait enough for at least 1 token to refill (10ms at 100/sec = 1 token).
	time.Sleep(15 * time.Millisecond)

	if !rl.Allow("tool") {
		t.Fatal("should be allowed after refill")
	}
}

func TestRateLimitFromEnv_Disabled(t *testing.T) {
	// Without env vars set, should return nil.
	t.Setenv("THIMBLE_RATE_LIMIT", "")
	t.Setenv("THIMBLE_RATE_BURST", "")

	rl := RateLimitFromEnv()
	if rl != nil {
		t.Fatal("should return nil when THIMBLE_RATE_LIMIT is empty")
	}
}

func TestRateLimitFromEnv_ZeroDisabled(t *testing.T) {
	t.Setenv("THIMBLE_RATE_LIMIT", "0")

	rl := RateLimitFromEnv()
	if rl != nil {
		t.Fatal("should return nil when rate is 0")
	}
}

func TestRateLimitFromEnv_Configured(t *testing.T) {
	t.Setenv("THIMBLE_RATE_LIMIT", "10")
	t.Setenv("THIMBLE_RATE_BURST", "5")

	rl := RateLimitFromEnv()
	if rl == nil {
		t.Fatal("should return a limiter when rate > 0")
	}

	if rl.rate != 10 {
		t.Fatalf("expected rate 10, got %f", rl.rate)
	}

	if rl.burst != 5 {
		t.Fatalf("expected burst 5, got %d", rl.burst)
	}
}

func TestRateLimitFromEnv_DefaultBurst(t *testing.T) {
	t.Setenv("THIMBLE_RATE_LIMIT", "5")
	t.Setenv("THIMBLE_RATE_BURST", "")

	rl := RateLimitFromEnv()
	if rl == nil {
		t.Fatal("should return a limiter")
	}

	if rl.burst != 20 {
		t.Fatalf("expected default burst 20, got %d", rl.burst)
	}
}
