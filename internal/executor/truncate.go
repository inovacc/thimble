package executor

import (
	"fmt"
	"strings"
)

// smartTruncate keeps head (60%) + tail (40%) of output, snapping to line boundaries.
func smartTruncate(raw string, maxBytes int) string {
	if len(raw) <= maxBytes {
		return raw
	}

	headBudget := maxBytes * 60 / 100
	tailBudget := maxBytes - headBudget - 50 // reserve for separator

	// Guard against negative tail budget when input barely exceeds limit.
	if tailBudget <= 0 {
		if headBudget > len(raw) {
			headBudget = len(raw)
		}

		return raw[:headBudget] + "\n\n... [truncated " + itoa(len(raw)-headBudget) + " bytes] ...\n\n"
	}

	// Snap to line boundaries.
	headEnd := headBudget
	if idx := strings.LastIndex(raw[:headEnd], "\n"); idx > 0 {
		headEnd = idx + 1
	}

	tailStart := max(len(raw)-tailBudget, 0)

	if idx := strings.Index(raw[tailStart:], "\n"); idx >= 0 {
		tailStart += idx + 1
	}

	// Guard against overlap when head and tail regions meet.
	if headEnd >= tailStart {
		return raw[:headEnd] + "\n\n... [truncated " + itoa(len(raw)-headEnd) + " bytes] ...\n\n"
	}

	omitted := len(raw) - headEnd - (len(raw) - tailStart)
	separator := "\n\n... [truncated " + itoa(omitted) + " bytes] ...\n\n"

	return raw[:headEnd] + separator + raw[tailStart:]
}

// formatKB formats a byte count as a human-readable string.
func formatKB(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}

	return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
}

// itoa is a simple int-to-string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	neg := false
	if n < 0 {
		neg = true
		n = -n
	}

	var buf [20]byte

	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	if neg {
		i--
		buf[i] = '-'
	}

	return string(buf[i:])
}
