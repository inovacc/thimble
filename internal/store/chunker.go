package store

import (
	"os"
	"strings"
)

// ChunkStrategy defines the content chunking approach for FTS5 indexing.
type ChunkStrategy string

const (
	// ChunkNone indexes content as a single chunk (original behavior).
	ChunkNone ChunkStrategy = "none"

	// ChunkParagraph splits on double newlines, merging small paragraphs.
	ChunkParagraph ChunkStrategy = "paragraph"

	// ChunkSection splits on markdown headers, keeping header with section body.
	ChunkSection ChunkStrategy = "section"

	// ChunkSliding uses overlapping fixed-size windows with 20% overlap.
	ChunkSliding ChunkStrategy = "sliding"
)

// chunkStrategyThreshold is the minimum content size (bytes) before chunking kicks in.
const chunkStrategyThreshold = 2048

// defaultChunkStrategy returns the configured strategy from THIMBLE_CHUNK_STRATEGY
// or "paragraph" if unset.
func defaultChunkStrategy() ChunkStrategy {
	if v := os.Getenv("THIMBLE_CHUNK_STRATEGY"); v != "" {
		switch ChunkStrategy(v) {
		case ChunkNone, ChunkParagraph, ChunkSection, ChunkSliding:
			return ChunkStrategy(v)
		}
	}

	return ChunkParagraph
}

// ChunkContent splits content into chunks using the specified strategy and max size.
// Content shorter than chunkStrategyThreshold is returned as-is (single chunk).
func ChunkContent(content string, strategy ChunkStrategy, maxChunkSize int) []string {
	if maxChunkSize <= 0 {
		maxChunkSize = MaxChunkBytes
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}

	if len(trimmed) < chunkStrategyThreshold {
		return []string{trimmed}
	}

	switch strategy {
	case ChunkNone:
		return []string{trimmed}
	case ChunkParagraph:
		return chunkByParagraph(trimmed, maxChunkSize)
	case ChunkSection:
		return chunkBySection(trimmed, maxChunkSize)
	case ChunkSliding:
		return chunkBySliding(trimmed, maxChunkSize)
	default:
		return []string{trimmed}
	}
}

// chunkByParagraph splits on double newlines and merges small paragraphs (< 100 chars).
func chunkByParagraph(content string, maxSize int) []string {
	parts := strings.Split(content, "\n\n")

	var chunks []string

	var current strings.Builder

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if current.Len() == 0 {
			current.WriteString(part)

			continue
		}

		// Merge small paragraphs with the current accumulator.
		if current.Len() < 100 {
			current.WriteString("\n\n")
			current.WriteString(part)

			continue
		}

		// If adding this paragraph would exceed max, flush current first.
		if current.Len()+2+len(part) > maxSize {
			chunks = append(chunks, current.String())
			current.Reset()
			current.WriteString(part)

			continue
		}

		current.WriteString("\n\n")
		current.WriteString(part)
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}

// chunkBySection splits on markdown headers (# ## ###), keeping header with its body.
func chunkBySection(content string, maxSize int) []string {
	lines := strings.Split(content, "\n")

	var chunks []string

	var current strings.Builder

	for _, line := range lines {
		isHeader := len(line) > 0 && line[0] == '#' && headingRe.MatchString(line)

		if isHeader && current.Len() > 0 {
			// Flush previous section before starting new one.
			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}

		current.WriteString(line)

		// If the current section exceeds max, flush it.
		if current.Len() > maxSize {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
		}
	}

	if current.Len() > 0 {
		text := strings.TrimSpace(current.String())
		if text != "" {
			chunks = append(chunks, text)
		}
	}

	return chunks
}

// chunkBySliding creates overlapping windows of maxSize with 20% overlap.
func chunkBySliding(content string, maxSize int) []string {
	if len(content) <= maxSize {
		return []string{content}
	}

	overlap := maxSize / 5 // 20% overlap
	step := maxSize - overlap

	if step <= 0 {
		step = 1
	}

	var chunks []string

	for offset := 0; offset < len(content); offset += step {
		end := min(offset+maxSize, len(content))

		c := strings.TrimSpace(content[offset:end])
		if c != "" {
			chunks = append(chunks, c)
		}

		if end == len(content) {
			break
		}
	}

	return chunks
}
