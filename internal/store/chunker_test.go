package store

import (
	"strings"
	"testing"
)

func TestChunkContent_SmallContent(t *testing.T) {
	// Content below threshold returns as single chunk regardless of strategy.
	small := "This is small content."
	for _, strategy := range []ChunkStrategy{ChunkNone, ChunkParagraph, ChunkSection, ChunkSliding} {
		got := ChunkContent(small, strategy, 4096)
		if len(got) != 1 {
			t.Errorf("strategy=%q: expected 1 chunk for small content, got %d", strategy, len(got))
		}
	}
}

func TestChunkContent_Empty(t *testing.T) {
	got := ChunkContent("", ChunkParagraph, 4096)
	if got != nil {
		t.Errorf("expected nil for empty content, got %v", got)
	}

	got = ChunkContent("   \n  ", ChunkParagraph, 4096)
	if got != nil {
		t.Errorf("expected nil for whitespace content, got %v", got)
	}
}

func TestChunkContent_NoneStrategy(t *testing.T) {
	content := strings.Repeat("Hello world. ", 300) // well over 2KB

	got := ChunkContent(content, ChunkNone, 4096)
	if len(got) != 1 {
		t.Errorf("ChunkNone: expected 1 chunk, got %d", len(got))
	}
}

func TestChunkContent_Paragraph(t *testing.T) {
	// Build content with multiple paragraphs exceeding 2KB.
	paragraphs := make([]string, 0, 20)

	for range 20 {
		paragraphs = append(paragraphs, strings.Repeat("Paragraph content here. ", 20))
	}

	content := strings.Join(paragraphs, "\n\n")

	got := ChunkContent(content, ChunkParagraph, 2048)
	if len(got) < 2 {
		t.Fatalf("expected multiple chunks from paragraph strategy, got %d", len(got))
	}

	// Each chunk should be <= maxChunkSize (with some tolerance for merging).
	for i, c := range got {
		if len(c) > 2048+500 {
			t.Errorf("chunk %d too large: %d bytes", i, len(c))
		}
	}

	// Reconstructed content should preserve all paragraphs.
	joined := strings.Join(got, "\n\n")

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if !strings.Contains(joined, p) {
			t.Error("paragraph content lost during chunking")
			break
		}
	}
}

func TestChunkContent_ParagraphMergesSmall(t *testing.T) {
	// Small paragraphs (<100 chars) should be merged with the next.
	content := "Hi\n\n" + "OK\n\n" + strings.Repeat("This is a proper paragraph with enough content. ", 80)
	got := ChunkContent(content, ChunkParagraph, 4096)

	// "Hi" and "OK" are <100 chars so should be merged — expect fewer chunks
	// than the number of double-newline-separated parts.
	if len(got) == 3 {
		t.Error("expected small paragraphs to be merged, but got 3 separate chunks")
	}
}

func TestChunkContent_Section(t *testing.T) {
	sections := make([]string, 0, 10)

	for i := range 10 {
		body := strings.Repeat("Section body content here. ", 40)
		sections = append(sections, "## Section "+string(rune('A'+i))+"\n\n"+body)
	}

	content := strings.Join(sections, "\n\n")

	got := ChunkContent(content, ChunkSection, 4096)
	if len(got) < 2 {
		t.Fatalf("expected multiple chunks from section strategy, got %d", len(got))
	}

	// Each chunk (except possibly last) should start with a heading.
	headingCount := 0

	for _, c := range got {
		if strings.HasPrefix(strings.TrimSpace(c), "##") {
			headingCount++
		}
	}

	if headingCount < 2 {
		t.Errorf("expected sections to retain headers, only %d chunks start with heading", headingCount)
	}
}

func TestChunkContent_Sliding(t *testing.T) {
	content := strings.Repeat("ABCDE", 1000) // 5000 bytes
	maxSize := 1000
	got := ChunkContent(content, ChunkSliding, maxSize)

	if len(got) < 2 {
		t.Fatalf("expected multiple sliding chunks, got %d", len(got))
	}

	// Verify overlap: each chunk (except last) should share 20% with the next.
	overlap := maxSize / 5
	for i := 0; i < len(got)-1; i++ {
		suffix := got[i][len(got[i])-overlap:]
		if !strings.HasPrefix(got[i+1], suffix) {
			t.Errorf("chunks %d and %d do not have expected 20%% overlap", i, i+1)
		}
	}

	// All content should be covered.
	if !strings.HasPrefix(got[0], content[:10]) {
		t.Error("first chunk doesn't start at content beginning")
	}

	lastChunk := got[len(got)-1]
	if !strings.HasSuffix(content, lastChunk[len(lastChunk)-10:]) {
		t.Error("last chunk doesn't reach content end")
	}
}

func TestChunkContent_SlidingSmall(t *testing.T) {
	// Content that fits in one window returns single chunk.
	content := strings.Repeat("X", chunkStrategyThreshold+100)

	got := ChunkContent(content, ChunkSliding, len(content)+500)
	if len(got) != 1 {
		t.Errorf("expected 1 chunk for content fitting in window, got %d", len(got))
	}
}

func TestDefaultChunkStrategy(t *testing.T) {
	// Default is paragraph.
	t.Setenv("THIMBLE_CHUNK_STRATEGY", "")

	if got := defaultChunkStrategy(); got != ChunkParagraph {
		t.Errorf("default strategy = %q, want %q", got, ChunkParagraph)
	}

	// Valid values.
	for _, s := range []ChunkStrategy{ChunkNone, ChunkParagraph, ChunkSection, ChunkSliding} {
		t.Setenv("THIMBLE_CHUNK_STRATEGY", string(s))

		if got := defaultChunkStrategy(); got != s {
			t.Errorf("strategy = %q, want %q", got, s)
		}
	}

	// Invalid value falls back to paragraph.
	t.Setenv("THIMBLE_CHUNK_STRATEGY", "invalid")

	if got := defaultChunkStrategy(); got != ChunkParagraph {
		t.Errorf("invalid strategy = %q, want %q", got, ChunkParagraph)
	}
}

func TestIndexUsesChunkStrategy(t *testing.T) {
	cs := tempDB(t)

	// Build content >2KB to trigger strategy-based chunking.
	paragraphs := make([]string, 0, 15)

	for range 15 {
		paragraphs = append(paragraphs, strings.Repeat("Integration test paragraph. ", 25))
	}

	content := strings.Join(paragraphs, "\n\n")

	t.Setenv("THIMBLE_CHUNK_STRATEGY", "paragraph")

	result, err := cs.Index(content, "chunked-doc")
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if result.TotalChunks < 2 {
		t.Errorf("expected multiple chunks with paragraph strategy, got %d", result.TotalChunks)
	}

	// Verify search still works on chunked content.
	results, err := cs.Search("Integration test paragraph", 5, "", "AND")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected search results from chunked content")
	}
}

func TestIndexSmallContentUnchanged(t *testing.T) {
	cs := tempDB(t)

	// Small content should use original chunkMarkdown path.
	t.Setenv("THIMBLE_CHUNK_STRATEGY", "paragraph")

	result, err := cs.Index("# Title\n\nSmall doc.", "small-doc")
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk for small content, got %d", result.TotalChunks)
	}
}

func TestIndexNoneStrategyLargeContent(t *testing.T) {
	cs := tempDB(t)

	// With "none" strategy, even large content uses original chunkMarkdown.
	t.Setenv("THIMBLE_CHUNK_STRATEGY", "none")

	content := "# Big\n\n" + strings.Repeat("Word ", 600)

	result, err := cs.Index(content, "none-doc")
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	// chunkMarkdown will still produce chunks based on headings/size.
	if result.TotalChunks == 0 {
		t.Error("expected at least one chunk")
	}
}
