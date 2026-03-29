package mcp

import (
	"strings"
	"testing"
)

// ── effectiveLimit edge cases ──

func TestEffectiveLimitAllLevels(t *testing.T) {
	tests := []struct {
		name   string
		level  ThrottleLevel
		reqLim int
		want   int
	}{
		{"normal_zero", ThrottleNormal, 0, 2},
		{"normal_positive", ThrottleNormal, 10, 10},
		{"normal_negative", ThrottleNormal, -1, 2},
		{"degraded_zero", ThrottleDegraded, 0, 1},
		{"degraded_positive", ThrottleDegraded, 10, 1},
		{"degraded_negative", ThrottleDegraded, -5, 1},
		{"blocked_zero", ThrottleBlocked, 0, 0},
		{"blocked_positive", ThrottleBlocked, 5, 0},
		{"blocked_negative", ThrottleBlocked, -3, 0},
		{"unknown_zero", ThrottleLevel(99), 0, 2},
		{"unknown_positive", ThrottleLevel(99), 7, 7},
		{"unknown_negative", ThrottleLevel(99), -1, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveLimit(tt.level, tt.reqLim)
			if got != tt.want {
				t.Errorf("effectiveLimit(%d, %d) = %d, want %d", tt.level, tt.reqLim, got, tt.want)
			}
		})
	}
}

// ── isSoftFail edge cases ──

func TestIsSoftFailEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		lang     string
		exitCode int
		stdout   string
		want     bool
	}{
		{"shell_exit1_tabs", "shell", 1, "\t\t", false},
		{"shell_exit1_mixed_ws", "shell", 1, "  \n\t  ", false},
		{"shell_exit1_single_char", "shell", 1, "x", true},
		{"javascript_exit1_output", "javascript", 1, "output", false},
		{"typescript_exit1_output", "typescript", 1, "output", false},
		{"go_exit1_output", "go", 1, "output", false},
		{"shell_exit0_empty", "shell", 0, "", false},
		{"shell_exit255_output", "shell", 255, "output", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSoftFail(tt.lang, tt.exitCode, tt.stdout)
			if got != tt.want {
				t.Errorf("isSoftFail(%q, %d, %q) = %v, want %v",
					tt.lang, tt.exitCode, tt.stdout, got, tt.want)
			}
		})
	}
}

// ── formatExecOutput additional edge cases ──

func TestFormatExecOutputEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		stdout       string
		stderr       string
		exitCode     int
		timedOut     bool
		backgrounded bool
		wantContains []string
		wantExcludes []string
	}{
		{
			name:         "backgrounded_ignores_all",
			stdout:       "data",
			stderr:       "err",
			exitCode:     42,
			timedOut:     true,
			backgrounded: true,
			wantContains: []string{"[backgrounded]"},
			wantExcludes: []string{"data", "err", "exit code", "timed out"},
		},
		{
			name:         "stderr_with_newline_separator",
			stdout:       "line1",
			stderr:       "errline",
			exitCode:     0,
			wantContains: []string{"line1", "\n", "[stderr]", "errline"},
		},
		{
			name:         "exit_code_negative",
			exitCode:     -1,
			wantContains: []string{"[exit code: -1]"},
		},
		{
			name:         "large_exit_code",
			exitCode:     999,
			wantContains: []string{"[exit code: 999]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatExecOutput(tt.stdout, tt.stderr, tt.exitCode, tt.timedOut, tt.backgrounded)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q, got %q", want, got)
				}
			}

			for _, exclude := range tt.wantExcludes {
				if strings.Contains(got, exclude) {
					t.Errorf("output should not contain %q, got %q", exclude, got)
				}
			}
		})
	}
}

// ── formatBytes boundary ──

func TestFormatBytesBoundary(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0.0KB"},
		{1, "0.0KB"},
		{100, "0.1KB"},
		{1023, "1.0KB"},
		{1024*1024 - 1, "1024.0KB"},
		{1024 * 1024, "1.0MB"},
		{1024*1024 + 1, "1.0MB"},
		{10 * 1024 * 1024, "10.0MB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ── extractSnippet more edge cases ──

func TestExtractSnippetContentEqualToWindow(t *testing.T) {
	content := strings.Repeat("word ", 20) // 100 chars

	got := extractSnippet(content, "word", 100)
	if got != content {
		t.Errorf("content <= windowSize should return unchanged")
	}
}

func TestExtractSnippetEmptyContent(t *testing.T) {
	got := extractSnippet("", "term", 500)
	if got != "" {
		t.Errorf("empty content should return empty, got %q", got)
	}
}

func TestExtractSnippetAllShortTermsUsedAsFallback(t *testing.T) {
	// All terms < 3 chars, content has matches.
	content := strings.Repeat("ab cd ef ", 100)

	got := extractSnippet(content, "ab cd", 50)
	if len(got) == 0 {
		t.Error("expected non-empty snippet with short terms fallback")
	}
}

// ── extractDistinctiveTerms edge cases ──

func TestExtractDistinctiveTermsCaseInsensitive(t *testing.T) {
	content := "Kubernetes KUBERNETES kubernetes"

	terms := extractDistinctiveTerms(content, "KUBERNETES", 5)
	if len(terms) == 0 {
		t.Fatal("expected case-insensitive match")
	}

	if terms[0] != "kubernetes" {
		t.Errorf("expected lowercase term, got %q", terms[0])
	}
}

func TestExtractDistinctiveTermsNilContent(t *testing.T) {
	terms := extractDistinctiveTerms("", "query terms", 5)
	if terms != nil {
		t.Errorf("expected nil for empty content, got %v", terms)
	}
}

// ── snapToLineEnd / snapToLineStart boundary cases ──

func TestSnapToLineEndAtNewline(t *testing.T) {
	text := "abc\ndef\nghi"
	// pos exactly at a newline character
	got := snapToLineEnd(text, 4)
	if got != 3 {
		t.Errorf("snapToLineEnd at pos after newline = %d, want 3", got)
	}
}

func TestSnapToLineStartAtZero(t *testing.T) {
	text := "abc\ndef"

	got := snapToLineStart(text, 0)
	if got != 0 {
		t.Errorf("snapToLineStart(0) = %d, want 0", got)
	}
}

func TestSnapToLineStartAtEnd(t *testing.T) {
	text := "abc\ndef"

	got := snapToLineStart(text, len(text))
	if got != len(text) {
		t.Errorf("snapToLineStart(len) = %d, want %d", got, len(text))
	}
}

func TestSnapToLineEndAtEnd(t *testing.T) {
	text := "abc\ndef"

	got := snapToLineEnd(text, len(text)+10)
	if got != len(text) {
		t.Errorf("snapToLineEnd(past end) = %d, want %d", got, len(text))
	}
}

// ── positionsFromHighlight edge cases ──

func TestPositionsFromHighlightTrailingSTX(t *testing.T) {
	// STX without closing ETX
	highlighted := "text\x02orphan"

	positions := positionsFromHighlight(highlighted)
	if len(positions) != 1 {
		t.Errorf("expected 1 position for unclosed STX, got %d", len(positions))
	}

	if len(positions) > 0 && positions[0] != 4 {
		t.Errorf("position = %d, want 4", positions[0])
	}
}

func TestPositionsFromHighlightOnlyMarkers(t *testing.T) {
	highlighted := "\x02x\x03"

	positions := positionsFromHighlight(highlighted)
	if len(positions) != 1 || positions[0] != 0 {
		t.Errorf("expected [0], got %v", positions)
	}
}

// ── extractSnippetHL edge cases ──

func TestExtractSnippetHLContentShorterThanWindow(t *testing.T) {
	got := extractSnippetHL("short", "query", 500, "\x02short\x03")
	if got != "short" {
		t.Errorf("expected unchanged short content, got %q", got)
	}
}

// ── filterResult edge cases ──

func TestFilterResultEmptyInput(t *testing.T) {
	result := filterResult("")
	if result == nil {
		t.Fatal("expected non-nil result for empty input")
	}
}

// ── throttler concurrent safety ──

func TestThrottlerConcurrent(t *testing.T) {
	th := newThrottler()
	done := make(chan bool, 20)

	for range 20 {
		go func() {
			_ = th.check()
			_ = th.used()
			_ = th.remaining()

			done <- true
		}()
	}

	for range 20 {
		<-done
	}
}

// ── coerceJSONArray edge cases ──

func TestCoerceJSONArrayFloatInput(t *testing.T) {
	result := coerceJSONArray(3.14)
	if result != nil {
		t.Errorf("expected nil for float input, got %v", result)
	}
}

func TestCoerceJSONArrayStringNotBracket(t *testing.T) {
	result := coerceJSONArray("hello world")
	if result != nil {
		t.Errorf("expected nil for plain string, got %v", result)
	}
}
