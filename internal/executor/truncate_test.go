package executor

import (
	"strings"
	"testing"
)

func TestItoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{12345, "12345"},
		{-1, "-1"},
		{-999, "-999"},
		{1000000, "1000000"},
	}
	for _, tc := range tests {
		got := itoa(tc.n)
		if got != tc.want {
			t.Errorf("itoa(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestSmartTruncateShortInput(t *testing.T) {
	input := "short output"

	got := smartTruncate(input, 100)
	if got != input {
		t.Errorf("expected passthrough for short input, got %q", got)
	}
}

func TestSmartTruncateExactLimit(t *testing.T) {
	input := strings.Repeat("a", 100)

	got := smartTruncate(input, 100)
	if got != input {
		t.Error("expected passthrough at exact limit")
	}
}

func TestSmartTruncateLargeInput(t *testing.T) {
	// Build multi-line input.
	var sb strings.Builder
	for i := range 100 {
		sb.WriteString("line " + itoa(i) + ": " + strings.Repeat("x", 20) + "\n")
	}

	input := sb.String()

	maxBytes := 500
	got := smartTruncate(input, maxBytes)

	// Should contain truncation marker.
	if !strings.Contains(got, "truncated") {
		t.Error("expected truncation marker")
	}

	// Should be roughly within budget (marker adds some bytes).
	if len(got) > maxBytes+200 {
		t.Errorf("truncated output too large: %d bytes (budget %d)", len(got), maxBytes)
	}

	// Should contain beginning and end of original.
	if !strings.HasPrefix(got, "line 0:") {
		t.Error("expected output to start with first line")
	}

	if !strings.Contains(got, "line 99:") {
		t.Error("expected output to contain last line")
	}
}

func TestSmartTruncatePreservesLineBreaks(t *testing.T) {
	var sb strings.Builder
	for i := range 50 {
		sb.WriteString("line" + itoa(i) + "\n")
	}

	input := sb.String()
	got := smartTruncate(input, 200)

	// Result should not cut in the middle of a line.
	lines := strings.SplitSeq(got, "\n")
	for line := range lines {
		if strings.Contains(line, "truncated") {
			continue // separator line
		}
		// Each line should be a complete "lineN" or empty.
		if line != "" && !strings.HasPrefix(line, "line") && !strings.HasPrefix(line, "...") {
			t.Errorf("unexpected partial line: %q", line)
		}
	}
}

