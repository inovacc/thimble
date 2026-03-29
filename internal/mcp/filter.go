package mcp

import (
	"fmt"
	"strings"
	"sync"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ThrottleLevel represents the progressive throttle stage.
type ThrottleLevel int

const (
	// ThrottleNormal means no throttling — full limits apply.
	ThrottleNormal ThrottleLevel = iota
	// ThrottleDegraded means partial throttling — reduced limits.
	ThrottleDegraded
	// ThrottleBlocked means search is blocked until the window resets.
	ThrottleBlocked
)

const (
	// FilterThreshold is the size above which output gets filtered.
	FilterThreshold = 5 * 1024 // 5KB
	// ThrottleWindow is the time window for search throttling.
	ThrottleWindow = 60 * time.Second
	// ThrottleStage1Max is the number of searches before entering degraded mode.
	ThrottleStage1Max = 3
	// ThrottleStage2Max is the number of searches before entering blocked mode.
	ThrottleStage2Max = 8
	// MaxSearchesPerWindow is kept for backward compatibility.
	MaxSearchesPerWindow = ThrottleStage2Max
)

// searchThrottler tracks search frequency to prevent context flooding.
type searchThrottler struct {
	mu         sync.Mutex
	timestamps []time.Time
}

// newThrottler creates a new search throttler instance.
func newThrottler() *searchThrottler {
	return &searchThrottler{}
}

// check returns the current throttle level and records the call unless blocked.
func (t *searchThrottler) check() ThrottleLevel {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-ThrottleWindow)

	// Remove expired timestamps.
	var active []time.Time

	for _, ts := range t.timestamps {
		if ts.After(cutoff) {
			active = append(active, ts)
		}
	}

	t.timestamps = active

	count := len(t.timestamps)

	if count >= ThrottleStage2Max {
		return ThrottleBlocked
	}

	// Record this call (not blocked).
	t.timestamps = append(t.timestamps, now)

	if count >= ThrottleStage1Max {
		return ThrottleDegraded
	}

	return ThrottleNormal
}

// used returns the number of searches in the current window.
func (t *searchThrottler) used() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-ThrottleWindow)

	var count int

	for _, ts := range t.timestamps {
		if ts.After(cutoff) {
			count++
		}
	}

	return count
}

func (t *searchThrottler) remaining() int {
	return ThrottleStage2Max - t.used()
}

// effectiveLimit returns the limit to use based on the throttle level and the
// requested limit. In normal mode, uses the requested limit (default 2).
// In degraded mode, always returns 1.
func effectiveLimit(level ThrottleLevel, requestedLimit int) int {
	switch level {
	case ThrottleNormal:
		if requestedLimit <= 0 {
			return 2
		}

		return requestedLimit
	case ThrottleDegraded:
		return 1
	case ThrottleBlocked:
		return 0
	default:
		if requestedLimit <= 0 {
			return 2
		}

		return requestedLimit
	}
}

// filterResult applies intent-driven filtering to tool output.
// If output exceeds FilterThreshold, it truncates with a summary.
func filterResult(text string) *mcpsdk.CallToolResult {
	if len(text) <= FilterThreshold {
		return textResult(text)
	}

	// Smart truncation: keep first ~60% and last ~20%, insert summary.
	totalBytes := len(text)
	headBytes := int(float64(totalBytes) * 0.6)
	tailBytes := int(float64(totalBytes) * 0.2)

	// Snap to line boundaries.
	headEnd := snapToLineEnd(text, headBytes)
	tailStart := snapToLineStart(text, totalBytes-tailBytes)

	if tailStart <= headEnd {
		// Overlap — just truncate at head.
		headEnd = snapToLineEnd(text, FilterThreshold)
		truncated := totalBytes - headEnd

		return textResult(fmt.Sprintf("%s\n\n[... %d bytes truncated. Use ctx_search to find specific content ...]",
			text[:headEnd], truncated))
	}

	truncated := tailStart - headEnd

	return textResult(fmt.Sprintf("%s\n\n[... %d bytes truncated ...]\n\n%s",
		text[:headEnd], truncated, text[tailStart:]))
}

// extractSnippet returns a focused snippet of content around the best matching
// terms from the intent string. If the content is already short enough or no
// intent terms match, it returns the content unchanged.
func extractSnippet(content, intent string, windowSize int) string {
	if len(content) <= windowSize {
		return content
	}

	if windowSize <= 0 {
		windowSize = 500
	}

	// Extract terms from intent.
	terms := strings.Fields(strings.ToLower(intent))
	if len(terms) == 0 {
		return content
	}

	// Filter out very short terms (noise words).
	var significant []string

	for _, t := range terms {
		if len(t) >= 3 {
			significant = append(significant, t)
		}
	}

	if len(significant) == 0 {
		significant = terms
	}

	// Find the position with the best density of matching terms.
	lower := strings.ToLower(content)
	bestPos := -1
	bestScore := 0

	// Slide a window across the content, scoring by term matches.
	step := max(windowSize/4, 1)

	for pos := 0; pos < len(lower)-step; pos += step {
		end := min(pos+windowSize, len(lower))

		window := lower[pos:end]
		score := 0

		for _, term := range significant {
			score += strings.Count(window, term)
		}

		if score > bestScore {
			bestScore = score
			bestPos = pos
		}
	}

	if bestScore == 0 || bestPos < 0 {
		// No matches found — return head truncation.
		end := snapToLineEnd(content, windowSize)
		if end <= 0 {
			end = windowSize
		}

		end = min(end, len(content))

		return content[:end] + "\n..."
	}

	// Extract the window, snapping to line boundaries.
	start := min(snapToLineStart(content, bestPos), bestPos)

	end := min(bestPos+windowSize, len(content))

	end = snapToLineEnd(content, end)
	if end <= start {
		end = min(bestPos+windowSize, len(content))
	}

	var sb strings.Builder

	if start > 0 {
		sb.WriteString("...")
	}

	sb.WriteString(content[start:end])

	if end < len(content) {
		sb.WriteString("\n...")
	}

	return sb.String()
}

// extractSnippetHL is a highlight-aware version of extractSnippet.
// When highlighted text is provided (FTS5 highlight markers STX/ETX), match
// positions are derived from the markers for precise snippet extraction.
// Falls back to extractSnippet when highlighted is empty.
func extractSnippetHL(content, intent string, windowSize int, highlighted string) string {
	if len(content) <= windowSize || highlighted == "" {
		return extractSnippet(content, intent, windowSize)
	}

	// Parse STX (0x02) / ETX (0x03) markers to find match positions.
	positions := positionsFromHighlight(highlighted)
	if len(positions) == 0 {
		return extractSnippet(content, intent, windowSize)
	}

	// Merge overlapping windows around match positions.
	const window = 300

	type span struct{ start, end int }

	var spans []span

	for _, pos := range positions {
		s := max(0, pos-window)

		e := min(len(content), pos+window)
		if len(spans) > 0 && s <= spans[len(spans)-1].end {
			spans[len(spans)-1].end = e
		} else {
			spans = append(spans, span{s, e})
		}
	}

	// Collect windows until windowSize.
	var sb strings.Builder

	total := 0
	for _, sp := range spans {
		if total >= windowSize {
			break
		}

		end := min(sp.end, sp.start+(windowSize-total))

		part := content[sp.start:end]
		if sp.start > 0 {
			sb.WriteString("...")
		}

		sb.WriteString(part)

		if end < len(content) {
			sb.WriteString("...")
		}

		total += len(part)
	}

	return sb.String()
}

// positionsFromHighlight parses FTS5 highlight markers (STX=0x02, ETX=0x03)
// to find match positions in the original (marker-free) text.
func positionsFromHighlight(highlighted string) []int {
	const (
		stx = '\x02'
		etx = '\x03'
	)

	var positions []int

	cleanOffset := 0

	for i := 0; i < len(highlighted); i++ {
		if highlighted[i] == stx {
			positions = append(positions, cleanOffset)

			i++ // skip STX
			for i < len(highlighted) && highlighted[i] != etx {
				cleanOffset++
				i++
			}
			// i now points to ETX, loop will i++
		} else {
			cleanOffset++
		}
	}

	return positions
}

// extractDistinctiveTerms returns the top N terms from content that best match
// the query terms. Used by batch search to show which terms made a result relevant.
func extractDistinctiveTerms(content, query string, maxTerms int) []string {
	if maxTerms <= 0 {
		maxTerms = 5
	}

	queryTerms := strings.Fields(strings.ToLower(query))
	if len(queryTerms) == 0 {
		return nil
	}

	lower := strings.ToLower(content)

	// Score each query term by frequency in this content.
	type scored struct {
		term  string
		count int
	}

	var matches []scored

	for _, qt := range queryTerms {
		if len(qt) < 2 {
			continue
		}

		cnt := strings.Count(lower, qt)
		if cnt > 0 {
			matches = append(matches, scored{qt, cnt})
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort by count descending.
	for i := range len(matches) {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].count > matches[i].count {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	if len(matches) > maxTerms {
		matches = matches[:maxTerms]
	}

	result := make([]string, len(matches))
	for i, m := range matches {
		result[i] = m.term
	}

	return result
}

// snapToLineEnd finds the nearest newline at or before pos.
func snapToLineEnd(text string, pos int) int {
	if pos >= len(text) {
		return len(text)
	}

	idx := strings.LastIndex(text[:pos], "\n")
	if idx < 0 {
		return pos
	}

	return idx
}

// snapToLineStart finds the nearest newline at or after pos.
func snapToLineStart(text string, pos int) int {
	if pos >= len(text) {
		return len(text)
	}

	if pos <= 0 {
		return 0
	}

	idx := strings.Index(text[pos:], "\n")
	if idx < 0 {
		return pos
	}

	return pos + idx + 1
}
