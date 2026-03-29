package mcp

import (
	"fmt"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestFilterResultSmallInput(t *testing.T) {
	text := "hello world"

	result := filterResult(text)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Small input should pass through unchanged.
	content := result.Content[0]

	tc, ok := content.(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected *mcpsdk.TextContent, got %T", content)
	}

	if tc.Text != text {
		t.Errorf("text = %q, want %q", tc.Text, text)
	}
}

func TestFilterResultLargeInput(t *testing.T) {
	// Build a string larger than FilterThreshold (5KB).
	var sb strings.Builder
	for range 600 {
		sb.WriteString("line " + strings.Repeat("x", 10) + "\n")
	}

	text := sb.String()
	if len(text) <= FilterThreshold {
		t.Skip("test text not large enough")
	}

	result := filterResult(text)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	content := result.Content[0]

	tc, ok := content.(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected *mcpsdk.TextContent, got %T", content)
	}

	// Should contain truncation marker.
	if !strings.Contains(tc.Text, "truncated") {
		t.Error("expected truncation marker in filtered output")
	}

	// Filtered output should be smaller than original.
	if len(tc.Text) >= len(text) {
		t.Error("filtered output should be smaller than original")
	}
}

func TestFilterResultExactThreshold(t *testing.T) {
	// Text exactly at threshold should pass through.
	text := strings.Repeat("a", FilterThreshold)
	result := filterResult(text)
	content := result.Content[0]

	tc := content.(*mcpsdk.TextContent)
	if tc.Text != text {
		t.Error("text at exact threshold should pass through unchanged")
	}
}

func TestSnapToLineEnd(t *testing.T) {
	text := "line1\nline2\nline3\nline4\n"

	tests := []struct {
		pos  int
		want int
	}{
		{0, 0},    // Before first newline, no newline found → returns pos.
		{6, 5},    // Just after first newline → snaps back to \n at pos 5.
		{12, 11},  // After second newline → snaps to \n at pos 11.
		{100, 24}, // Past end → returns len(text).
	}

	for _, tc := range tests {
		got := snapToLineEnd(text, tc.pos)
		if got != tc.want {
			t.Errorf("snapToLineEnd(%d) = %d, want %d", tc.pos, got, tc.want)
		}
	}
}

func TestSnapToLineStart(t *testing.T) {
	text := "line1\nline2\nline3\n"

	tests := []struct {
		pos  int
		want int
	}{
		{0, 0},    // Start at 0 → pos <= 0 returns 0.
		{6, 12},   // At start of line2 → finds next \n at 11, returns 12.
		{100, 18}, // Past end → returns len(text).
	}

	for _, tc := range tests {
		got := snapToLineStart(text, tc.pos)
		if got != tc.want {
			t.Errorf("snapToLineStart(%d) = %d, want %d", tc.pos, got, tc.want)
		}
	}
}

func TestSnapToLineStartNegative(t *testing.T) {
	text := "hello\nworld\n"

	got := snapToLineStart(text, -5)
	if got != 0 {
		t.Errorf("snapToLineStart(-5) = %d, want 0", got)
	}
}

func TestThrottler3Tier(t *testing.T) {
	th := &searchThrottler{}

	// Stage 1: first ThrottleStage1Max calls should be Normal.
	for i := range ThrottleStage1Max {
		level := th.check()
		if level != ThrottleNormal {
			t.Fatalf("call %d: got level %d, want ThrottleNormal", i+1, level)
		}
	}

	// Stage 2: next calls up to ThrottleStage2Max should be Degraded.
	for i := ThrottleStage1Max; i < ThrottleStage2Max; i++ {
		level := th.check()
		if level != ThrottleDegraded {
			t.Fatalf("call %d: got level %d, want ThrottleDegraded", i+1, level)
		}
	}

	// Stage 3: next call should be Blocked.
	level := th.check()
	if level != ThrottleBlocked {
		t.Fatalf("call %d: got level %d, want ThrottleBlocked", ThrottleStage2Max+1, level)
	}

	// Verify used count is ThrottleStage2Max (blocked call was not recorded).
	if u := th.used(); u != ThrottleStage2Max {
		t.Errorf("used() = %d, want %d", u, ThrottleStage2Max)
	}
}

func TestExtractSnippet(t *testing.T) {
	// Build a long content string with identifiable sections.
	var sb strings.Builder
	for i := range 50 {
		sb.WriteString(strings.Repeat("filler ", 20))
		sb.WriteString("\n")

		if i == 25 {
			sb.WriteString("The database connection pool handles timeout errors gracefully.\n")
			sb.WriteString("When a connection timeout occurs, the pool retries with backoff.\n")
		}
	}

	longContent := sb.String()

	tests := []struct {
		name       string
		content    string
		intent     string
		windowSize int
		wantSubstr string
		wantShort  bool // true if result should be shorter than content
	}{
		{
			name:       "short content returned unchanged",
			content:    "hello world",
			intent:     "greeting",
			windowSize: 500,
			wantSubstr: "hello world",
			wantShort:  false,
		},
		{
			name:       "empty intent returns full content",
			content:    longContent,
			intent:     "",
			windowSize: 500,
			wantSubstr: "", // no specific substring expected
			wantShort:  false,
		},
		{
			name:       "intent finds matching section",
			content:    longContent,
			intent:     "database connection timeout",
			windowSize: 500,
			wantSubstr: "database connection pool",
			wantShort:  true,
		},
		{
			name:       "intent with no matches returns head",
			content:    longContent,
			intent:     "xyzzynonexistent",
			windowSize: 500,
			wantSubstr: "...",
			wantShort:  true,
		},
		{
			name:       "short terms filtered out",
			content:    longContent,
			intent:     "a b c database",
			windowSize: 500,
			wantSubstr: "database",
			wantShort:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSnippet(tt.content, tt.intent, tt.windowSize)

			if tt.wantSubstr != "" && !strings.Contains(got, tt.wantSubstr) {
				t.Errorf("extractSnippet() missing expected substring %q, got length %d", tt.wantSubstr, len(got))
			}

			if tt.wantShort && len(got) >= len(tt.content) {
				t.Errorf("extractSnippet() should be shorter than content (%d >= %d)", len(got), len(tt.content))
			}

			if !tt.wantShort && got != tt.content {
				// For cases where content should pass through unchanged.
				if tt.intent == "" && len(tt.content) > 500 {
					// Empty intent on long content — just verify it's returned as-is.
					if got != tt.content {
						t.Errorf("empty intent should return content unchanged")
					}
				}
			}
		})
	}
}

func TestExtractSnippetEllipsis(t *testing.T) {
	// Verify ellipsis markers are added when snippet is from the middle.
	content := strings.Repeat("aaa ", 200) + "TARGET_KEYWORD here " + strings.Repeat("bbb ", 200)

	got := extractSnippet(content, "TARGET_KEYWORD", 100)

	if !strings.Contains(got, "...") {
		t.Error("expected ellipsis marker in snippet from middle of content")
	}

	if !strings.Contains(got, "TARGET_KEYWORD") {
		t.Error("expected snippet to contain the target keyword")
	}
}

func TestFilterResultOverlap(t *testing.T) {
	// Build content where the 60% head and 20% tail overlap.
	// This happens when the content is just above threshold and mostly single line.
	// Use a single very long line (no newlines) just above threshold.
	text := strings.Repeat("x", FilterThreshold+100)

	result := filterResult(text)
	tc := result.Content[0].(*mcpsdk.TextContent)

	if !strings.Contains(tc.Text, "truncated") {
		t.Error("expected truncation marker for large input")
	}

	if len(tc.Text) >= len(text) {
		t.Error("filtered output should be smaller than original")
	}
}

func TestFilterResultWithNewlines(t *testing.T) {
	// Content with newlines that allows head/tail split without overlap.
	var sb strings.Builder
	for i := range 1000 {
		sb.WriteString(strings.Repeat("a", 10))

		if i%10 == 0 {
			sb.WriteString("\n")
		}
	}

	text := sb.String()
	if len(text) <= FilterThreshold {
		t.Skip("content not large enough")
	}

	result := filterResult(text)
	tc := result.Content[0].(*mcpsdk.TextContent)

	if !strings.Contains(tc.Text, "truncated") {
		t.Error("expected truncation marker")
	}
}

func TestExtractSnippetZeroWindowSize(t *testing.T) {
	content := strings.Repeat("word ", 200)
	got := extractSnippet(content, "word", 0)
	// Should use default window size of 500.
	if len(got) == 0 {
		t.Error("expected non-empty snippet")
	}
}

func TestExtractSnippetOnlyShortTerms(t *testing.T) {
	// All intent terms are < 3 chars, so they should be used as-is.
	content := strings.Repeat("ab cd ", 200)
	got := extractSnippet(content, "ab cd", 100)
	// Should still produce a result (falls back to using short terms).
	if len(got) == 0 {
		t.Error("expected non-empty snippet")
	}
}

func TestSnapToLineEndNoNewline(t *testing.T) {
	text := "no newlines here"
	got := snapToLineEnd(text, 5)
	// No newline found before pos 5, should return pos.
	if got != 5 {
		t.Errorf("snapToLineEnd() = %d, want 5", got)
	}
}

func TestSnapToLineStartNoNewline(t *testing.T) {
	text := "no newlines here"
	got := snapToLineStart(text, 5)
	// No newline found after pos 5, should return pos.
	if got != 5 {
		t.Errorf("snapToLineStart() = %d, want 5", got)
	}
}

func TestNewThrottler(t *testing.T) {
	th := newThrottler()
	if th == nil {
		t.Fatal("newThrottler() returned nil")
	}

	if th.remaining() != MaxSearchesPerWindow {
		t.Errorf("remaining() = %d, want %d", th.remaining(), MaxSearchesPerWindow)
	}
}

func TestThrottlerRemaining(t *testing.T) {
	th := &searchThrottler{}

	if r := th.remaining(); r != MaxSearchesPerWindow {
		t.Errorf("remaining() = %d, want %d", r, MaxSearchesPerWindow)
	}

	th.check()
	th.check()

	if r := th.remaining(); r != MaxSearchesPerWindow-2 {
		t.Errorf("remaining() = %d, want %d", r, MaxSearchesPerWindow-2)
	}
}

func TestThrottleEffectiveLimit(t *testing.T) {
	if got := effectiveLimit(ThrottleNormal, 0); got != 2 {
		t.Errorf("normal default = %d, want 2", got)
	}

	if got := effectiveLimit(ThrottleNormal, 5); got != 5 {
		t.Errorf("normal requested = %d, want 5", got)
	}

	if got := effectiveLimit(ThrottleDegraded, 5); got != 1 {
		t.Errorf("degraded = %d, want 1", got)
	}
}

func TestThrottleWindowIs60s(t *testing.T) {
	if ThrottleWindow != 60*time.Second {
		t.Errorf("ThrottleWindow = %v, want 60s", ThrottleWindow)
	}
}

func TestSearchTotalOutputCap(t *testing.T) {
	if SearchTotalOutputCap != 40*1024 {
		t.Errorf("SearchTotalOutputCap = %d, want %d", SearchTotalOutputCap, 40*1024)
	}
}

func TestThrottleBlockedNotRecorded(t *testing.T) {
	th := &searchThrottler{}

	// Fill to max.
	for range ThrottleStage2Max {
		th.check()
	}

	// Blocked call should NOT be recorded.
	level := th.check()
	if level != ThrottleBlocked {
		t.Fatalf("expected blocked, got %d", level)
	}

	if th.used() != ThrottleStage2Max {
		t.Errorf("used() = %d, want %d (blocked call should not be counted)", th.used(), ThrottleStage2Max)
	}
}

func TestExtractDistinctiveTermsBasic(t *testing.T) {
	content := "The kubernetes deployment uses rolling updates with readiness probes and liveness probes for health checking."
	query := "kubernetes deployment probes health"

	terms := extractDistinctiveTerms(content, query, 5)
	if len(terms) == 0 {
		t.Fatal("expected at least one distinctive term")
	}

	// "probes" appears twice, should rank highest.
	if terms[0] != "probes" {
		t.Errorf("first term = %q, want probes (highest frequency)", terms[0])
	}
}

func TestExtractDistinctiveTermsNoMatch(t *testing.T) {
	content := "This is about databases and SQL queries."
	query := "kubernetes deployment"

	terms := extractDistinctiveTerms(content, query, 5)
	if len(terms) != 0 {
		t.Errorf("expected 0 terms for non-matching content, got %v", terms)
	}
}

func TestExtractDistinctiveTermsEmptyQuery(t *testing.T) {
	terms := extractDistinctiveTerms("some content", "", 5)
	if len(terms) != 0 {
		t.Errorf("expected 0 terms for empty query, got %v", terms)
	}
}

func TestExtractDistinctiveTermsMaxLimit(t *testing.T) {
	content := "alpha beta gamma delta epsilon zeta eta theta iota kappa"
	query := "alpha beta gamma delta epsilon zeta eta theta iota kappa"

	terms := extractDistinctiveTerms(content, query, 3)
	if len(terms) > 3 {
		t.Errorf("expected max 3 terms, got %d", len(terms))
	}
}

func TestExtractDistinctiveTermsDefaultMax(t *testing.T) {
	content := "alpha beta gamma delta epsilon zeta eta theta"
	query := "alpha beta gamma delta epsilon zeta eta theta"

	terms := extractDistinctiveTerms(content, query, 0) // 0 = default to 5
	if len(terms) > 5 {
		t.Errorf("expected max 5 terms (default), got %d", len(terms))
	}
}

func TestExtractDistinctiveTermsShortTermsFiltered(t *testing.T) {
	content := "the go programming language is great for APIs"
	query := "a go is"

	terms := extractDistinctiveTerms(content, query, 5)
	// Single-char terms "a" should be filtered.
	for _, term := range terms {
		if len(term) < 2 {
			t.Errorf("expected terms >= 2 chars, got %q", term)
		}
	}
}

func TestPositionsFromHighlight(t *testing.T) {
	tests := []struct {
		name        string
		highlighted string
		wantLen     int
		wantFirst   int // expected first position (-1 if empty)
	}{
		{
			name:        "empty string",
			highlighted: "",
			wantLen:     0,
			wantFirst:   -1,
		},
		{
			name:        "no markers",
			highlighted: "plain text without markers",
			wantLen:     0,
			wantFirst:   -1,
		},
		{
			name:        "single highlight",
			highlighted: "hello \x02world\x03 end",
			wantLen:     1,
			wantFirst:   6, // "hello " = 6 chars before STX
		},
		{
			name:        "multiple highlights",
			highlighted: "\x02first\x03 gap \x02second\x03",
			wantLen:     2,
			wantFirst:   0, // starts at beginning
		},
		{
			name:        "adjacent highlights",
			highlighted: "\x02ab\x03\x02cd\x03",
			wantLen:     2,
			wantFirst:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			positions := positionsFromHighlight(tt.highlighted)
			if len(positions) != tt.wantLen {
				t.Errorf("len(positions) = %d, want %d", len(positions), tt.wantLen)
			}

			if tt.wantFirst >= 0 && len(positions) > 0 && positions[0] != tt.wantFirst {
				t.Errorf("positions[0] = %d, want %d", positions[0], tt.wantFirst)
			}
		})
	}
}

func TestExtractSnippetHL(t *testing.T) {
	// Build a long content string.
	var sb strings.Builder
	for range 50 {
		sb.WriteString(strings.Repeat("padding ", 20))
		sb.WriteString("\n")
	}

	middle := sb.Len()
	sb.WriteString("The target keyword appears here in the middle of content.\n")

	for range 50 {
		sb.WriteString(strings.Repeat("filler ", 20))
		sb.WriteString("\n")
	}

	longContent := sb.String()

	tests := []struct {
		name        string
		content     string
		intent      string
		windowSize  int
		highlighted string
		wantSubstr  string
	}{
		{
			name:        "short content returns unchanged",
			content:     "short content",
			intent:      "anything",
			windowSize:  500,
			highlighted: "",
			wantSubstr:  "short content",
		},
		{
			name:        "empty highlighted falls back to extractSnippet",
			content:     longContent,
			intent:      "target keyword",
			windowSize:  500,
			highlighted: "",
			wantSubstr:  "target keyword",
		},
		{
			name:       "highlighted markers guide extraction",
			content:    longContent,
			intent:     "target",
			windowSize: 600,
			// Put an STX/ETX marker at position corresponding to "target" in the middle.
			highlighted: strings.Repeat("x", middle) + "\x02target\x03 keyword appears here",
			wantSubstr:  "target keyword",
		},
		{
			name:        "no valid markers falls back",
			content:     longContent,
			intent:      "target keyword",
			windowSize:  500,
			highlighted: "no markers here at all",
			wantSubstr:  "target keyword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSnippetHL(tt.content, tt.intent, tt.windowSize, tt.highlighted)
			if tt.wantSubstr != "" && !strings.Contains(got, tt.wantSubstr) {
				t.Errorf("extractSnippetHL() missing %q (len=%d)", tt.wantSubstr, len(got))
			}
		})
	}
}

func TestExtractSnippetHLWindowSizeCap(t *testing.T) {
	// Build content with multiple highlighted regions.
	var sb strings.Builder
	for i := range 200 {
		sb.WriteString(strings.Repeat("word ", 20))

		if i%20 == 0 {
			sb.WriteString("MARKER ")
		}

		sb.WriteString("\n")
	}

	content := sb.String()

	// Create highlighted with multiple markers spread out.
	var hlSb strings.Builder
	for i := range 200 {
		hlSb.WriteString(strings.Repeat("w", 20))

		if i%20 == 0 {
			hlSb.WriteString("\x02MARKER\x03 ")
		}

		hlSb.WriteString("\n")
	}

	highlighted := hlSb.String()

	got := extractSnippetHL(content, "MARKER", 300, highlighted)
	// Should respect windowSize cap.
	if len(got) > 1000 { // 300 + overhead for ellipsis markers
		t.Errorf("extractSnippetHL output too large: %d bytes (windowSize=300)", len(got))
	}
}

func TestExtractSnippetHLMergesOverlappingWindows(t *testing.T) {
	// Create content with adjacent markers that should merge.
	content := strings.Repeat("a", 100) + "FIRST " + "SECOND " + strings.Repeat("b", 500)
	highlighted := strings.Repeat("x", 100) + "\x02FIRST\x03 \x02SECOND\x03 " + strings.Repeat("y", 500)

	got := extractSnippetHL(content, "test", 600, highlighted)
	if !strings.Contains(got, "FIRST") || !strings.Contains(got, "SECOND") {
		t.Errorf("expected merged window to contain both markers, got %q", got)
	}
}

func TestPositionsFromHighlightEmpty(t *testing.T) {
	positions := positionsFromHighlight("")
	if len(positions) != 0 {
		t.Errorf("expected 0 positions for empty string, got %d", len(positions))
	}
}

func TestPositionsFromHighlightNoMarkers(t *testing.T) {
	positions := positionsFromHighlight("no markers here")
	if len(positions) != 0 {
		t.Errorf("expected 0 positions for text without markers, got %d", len(positions))
	}
}

func TestPositionsFromHighlightMultipleMarkers(t *testing.T) {
	// "hello \x02world\x03 foo \x02bar\x03"
	highlighted := "hello \x02world\x03 foo \x02bar\x03"

	positions := positionsFromHighlight(highlighted)
	if len(positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(positions))
	}
	// First match starts at offset 6 ("hello " = 6 chars)
	if positions[0] != 6 {
		t.Errorf("positions[0] = %d, want 6", positions[0])
	}
}

func TestExtractSnippetHLShortContent(t *testing.T) {
	// Content shorter than windowSize — should return unchanged.
	got := extractSnippetHL("short content", "query", 500, "\x02short\x03 content")
	if got != "short content" {
		t.Errorf("expected unchanged short content, got %q", got)
	}
}

func TestExtractSnippetHLEmptyHighlight(t *testing.T) {
	// Empty highlighted string should fall back to extractSnippet.
	content := strings.Repeat("x", 1000)

	got := extractSnippetHL(content, "query", 200, "")
	if len(got) > 250 { // some overhead for ellipsis
		t.Errorf("expected truncated output, got %d bytes", len(got))
	}
}

func TestFilterResultExactThresholdBoundary(t *testing.T) {
	// Exactly at threshold should not truncate.
	text := strings.Repeat("a", FilterThreshold)
	result := filterResult(text)

	tc := result.Content[0].(*mcpsdk.TextContent)
	if tc.Text != text {
		t.Error("expected unchanged output at exact threshold")
	}
}

func TestFilterResultJustOverThresholdTruncates(t *testing.T) {
	text := strings.Repeat("line\n", FilterThreshold/5+1) // Just over threshold
	result := filterResult(text)

	tc := result.Content[0].(*mcpsdk.TextContent)
	if !strings.Contains(tc.Text, "truncated") {
		t.Error("expected truncation marker for text over threshold")
	}
}

func TestExtractDistinctiveTermsSingleTermHit(t *testing.T) {
	content := "the foo bar foo baz foo"

	terms := extractDistinctiveTerms(content, "foo", 3)
	if len(terms) != 1 || terms[0] != "foo" {
		t.Errorf("expected [foo], got %v", terms)
	}
}

func TestExtractDistinctiveTermsNoMatchAll(t *testing.T) {
	content := "alpha beta gamma"

	terms := extractDistinctiveTerms(content, "xyz", 5)
	if len(terms) != 0 {
		t.Errorf("expected no terms, got %v", terms)
	}
}

func TestExtractDistinctiveTermsShortTermsIgnored(t *testing.T) {
	content := "a b c d e"

	terms := extractDistinctiveTerms(content, "a b", 5)
	if terms != nil {
		t.Errorf("expected nil for short terms, got %v", terms)
	}
}

func TestFilterResultNoNewlines(t *testing.T) {
	// Content with no newlines, just over threshold.
	text := strings.Repeat("x", FilterThreshold+50)
	result := filterResult(text)

	tc := result.Content[0].(*mcpsdk.TextContent)
	if !strings.Contains(tc.Text, "truncated") {
		t.Error("expected truncation marker for no-newline content")
	}
}

func TestExtractSnippetSnapToLineEndReturnsZero(t *testing.T) {
	// Content with no newlines — snapToLineEnd returns pos.
	// If windowSize is chosen so that snapToLineEnd returns 0, the fallback triggers.
	content := strings.Repeat("x", 600) + " TARGET " + strings.Repeat("y", 600)
	got := extractSnippet(content, "nonexistent_term_xyz", 200)
	// No match found → head truncation path with snapToLineEnd.
	if len(got) == 0 {
		t.Error("expected non-empty snippet")
	}

	if !strings.Contains(got, "...") {
		t.Error("expected ellipsis in head truncation")
	}
}

func TestExtractSnippetEndLeStart(t *testing.T) {
	// Test that extractSnippet handles edge cases where the best match
	// is near the end and window is very small. The snippet should still
	// contain something meaningful.
	var sb strings.Builder
	for range 100 {
		sb.WriteString("ab\n")
	}

	sb.WriteString("TARGETWORD here\n")

	for range 5 {
		sb.WriteString("cd\n")
	}

	content := sb.String()
	got := extractSnippet(content, "TARGETWORD", 50)
	// With a 50-byte window the snippet should contain something.
	if len(got) == 0 {
		t.Error("expected non-empty snippet")
	}
	// The snippet should have ellipsis since it's from the middle/end.
	if !strings.Contains(got, "...") {
		t.Error("expected ellipsis markers")
	}
}

func TestExtractSnippetHLWindowSizeExceeded(t *testing.T) {
	// Create content with highlighted positions spread far enough apart
	// that they form separate spans. Each span window is 300 chars wide.
	// With windowSize=50, the first span already fills the budget,
	// so the second span should trigger the `total >= windowSize` break.
	content := strings.Repeat("a", 1000) + "FIRST" + strings.Repeat("b", 1000) + "SECOND" + strings.Repeat("c", 1000)
	highlighted := strings.Repeat("a", 1000) + "\x02FIRST\x03" + strings.Repeat("b", 1000) + "\x02SECOND\x03" + strings.Repeat("c", 1000)

	// Use a small windowSize that will be filled by the first span alone.
	got := extractSnippetHL(content, "test", 50, highlighted)
	// The output should be truncated — only the first match window.
	if strings.Contains(got, "SECOND") {
		t.Error("expected SECOND to be excluded when windowSize is exceeded")
	}
}

func TestFilterResultWithNewlinesHeadTailSplit(t *testing.T) {
	// Content with many newlines, well above threshold, should get head + tail split.
	var sb strings.Builder
	for i := range FilterThreshold / 5 {
		fmt.Fprintf(&sb, "line %04d: %s\n", i, strings.Repeat("x", 10))
	}

	text := sb.String()
	if len(text) <= FilterThreshold {
		t.Skip("content not large enough")
	}

	result := filterResult(text)

	tc := result.Content[0].(*mcpsdk.TextContent)
	if !strings.Contains(tc.Text, "truncated") {
		t.Error("expected truncation marker for large content")
	}
}

func TestFilterResultSingleLongLine(t *testing.T) {
	// A single very long line (no newlines) just over threshold.
	text := strings.Repeat("a", FilterThreshold+500)
	result := filterResult(text)

	tc := result.Content[0].(*mcpsdk.TextContent)
	if !strings.Contains(tc.Text, "truncated") {
		t.Error("expected truncation marker for single long line")
	}
	// Should be shorter than original.
	if len(tc.Text) >= len(text) {
		t.Error("filtered output should be shorter")
	}
}
