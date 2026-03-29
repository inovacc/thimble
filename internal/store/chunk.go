package store

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// chunk is an internal representation of a content section.
type chunk struct {
	title   string
	content string
	hasCode bool
}

var (
	headingRe   = regexp.MustCompile(`^(#{1,4})\s+(.+)$`)
	hrRe        = regexp.MustCompile(`^[-_*]{3,}\s*$`)
	codeFenceRe = regexp.MustCompile("^(`{3,})(.*)?$")
)

// chunkMarkdown splits markdown content by headings, keeping code blocks intact.
func chunkMarkdown(text string, maxChunkBytes int) []chunk {
	var chunks []chunk

	lines := strings.Split(text, "\n")

	var (
		headingStack   []headingEntry
		currentContent []string
		currentHeading string
	)

	flush := func() {
		joined := strings.TrimSpace(strings.Join(currentContent, "\n"))
		if joined == "" {
			currentContent = nil
			return
		}

		title := buildTitle(headingStack, currentHeading)
		hasCode := false

		for _, l := range currentContent {
			if strings.HasPrefix(l, "```") {
				hasCode = true
				break
			}
		}

		if len(joined) <= maxChunkBytes {
			chunks = append(chunks, chunk{title: title, content: joined, hasCode: hasCode})
			currentContent = nil

			return
		}

		// Split oversized chunk at paragraph boundaries.
		paragraphs := splitParagraphs(joined)

		var accumulator []string

		partIndex := 1

		flushAcc := func() {
			if len(accumulator) == 0 {
				return
			}

			part := strings.TrimSpace(strings.Join(accumulator, "\n\n"))
			if part == "" {
				return
			}

			partTitle := title
			if len(paragraphs) > 1 {
				partTitle = fmt.Sprintf("%s (%d)", title, partIndex)
			}

			partIndex++

			chunks = append(chunks, chunk{
				title:   partTitle,
				content: part,
				hasCode: strings.Contains(part, "```"),
			})
			accumulator = nil
		}

		for _, para := range paragraphs {
			accumulator = append(accumulator, para)

			candidate := strings.Join(accumulator, "\n\n")
			if len(candidate) > maxChunkBytes && len(accumulator) > 1 {
				accumulator = accumulator[:len(accumulator)-1]

				flushAcc()

				accumulator = []string{para}
			}
		}

		flushAcc()

		currentContent = nil
	}

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Horizontal rule separator.
		if hrRe.MatchString(line) {
			flush()

			i++

			continue
		}

		// Heading (H1-H4).
		if m := headingRe.FindStringSubmatch(line); m != nil {
			flush()

			level := len(m[1])
			heading := strings.TrimSpace(m[2])

			for len(headingStack) > 0 && headingStack[len(headingStack)-1].level >= level {
				headingStack = headingStack[:len(headingStack)-1]
			}

			headingStack = append(headingStack, headingEntry{level: level, text: heading})
			currentHeading = heading

			currentContent = append(currentContent, line)
			i++

			continue
		}

		// Code block — collect entire block.
		if m := codeFenceRe.FindStringSubmatch(line); m != nil {
			fence := m[1]
			codeLines := []string{line}

			i++
			for i < len(lines) {
				codeLines = append(codeLines, lines[i])
				if strings.HasPrefix(lines[i], fence) && strings.TrimSpace(lines[i]) == fence {
					i++
					break
				}

				i++
			}

			currentContent = append(currentContent, codeLines...)

			continue
		}

		currentContent = append(currentContent, line)
		i++
	}

	flush()

	return chunks
}

type headingEntry struct {
	level int
	text  string
}

func buildTitle(stack []headingEntry, currentHeading string) string {
	if len(stack) == 0 {
		if currentHeading != "" {
			return currentHeading
		}

		return "Untitled"
	}

	parts := make([]string, len(stack))
	for i, h := range stack {
		parts[i] = h.text
	}

	return strings.Join(parts, " > ")
}

func splitParagraphs(text string) []string {
	re := regexp.MustCompile(`\n\n+`)
	return re.Split(text, -1)
}

// chunkPlainText splits plain text into sections.
func chunkPlainText(text string, linesPerChunk int) []chunk {
	// Try blank-line splitting for naturally-sectioned output.
	sections := strings.Split(text, "\n\n")
	if len(sections) >= 3 && len(sections) <= 200 {
		allSmall := true

		for _, s := range sections {
			if len(s) >= 5000 {
				allSmall = false
				break
			}
		}

		if allSmall {
			var result []chunk

			for i, sec := range sections {
				trimmed := strings.TrimSpace(sec)
				if trimmed == "" {
					continue
				}

				firstLine := strings.SplitN(trimmed, "\n", 2)[0]
				if len(firstLine) > 80 {
					firstLine = firstLine[:80]
				}

				title := firstLine
				if title == "" {
					title = fmt.Sprintf("Section %d", i+1)
				}

				result = append(result, chunk{title: title, content: trimmed})
			}

			return result
		}
	}

	lines := strings.Split(text, "\n")

	if len(lines) <= linesPerChunk {
		return []chunk{{title: "Output", content: text}}
	}

	// Fixed-size line groups with 2-line overlap.
	var chunks []chunk

	overlap := 2

	step := max(linesPerChunk-overlap, 1)

	for i := 0; i < len(lines); i += step {
		end := min(i+linesPerChunk, len(lines))

		sl := lines[i:end]
		if len(sl) == 0 {
			break
		}

		startLine := i + 1

		endLine := min(i+len(sl), len(lines))

		firstLine := strings.TrimSpace(sl[0])
		if len(firstLine) > 80 {
			firstLine = firstLine[:80]
		}

		title := firstLine
		if title == "" {
			title = fmt.Sprintf("Lines %d-%d", startLine, endLine)
		}

		chunks = append(chunks, chunk{title: title, content: strings.Join(sl, "\n")})
	}

	return chunks
}

// walkJSON recursively walks a parsed JSON value, producing chunks.
func walkJSON(value any, path []string, chunks *[]chunk, maxChunkBytes int) {
	title := "(root)"
	if len(path) > 0 {
		title = strings.Join(path, " > ")
	}

	serialized, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return
	}

	serializedStr := string(serialized)

	// Small enough — emit as a single chunk (unless it has nested structure).
	if len(serializedStr) <= maxChunkBytes {
		shouldRecurse := false

		if m, ok := value.(map[string]any); ok {
			for _, v := range m {
				switch v.(type) {
				case map[string]any, []any:
					shouldRecurse = true
				}

				if shouldRecurse {
					break
				}
			}
		}

		if !shouldRecurse {
			*chunks = append(*chunks, chunk{title: title, content: serializedStr, hasCode: true})
			return
		}
	}

	// Object — recurse into each key.
	if m, ok := value.(map[string]any); ok {
		if len(m) > 0 {
			for key, val := range m {
				walkJSON(val, append(append([]string{}, path...), key), chunks, maxChunkBytes)
			}

			return
		}

		*chunks = append(*chunks, chunk{title: title, content: serializedStr, hasCode: true})

		return
	}

	// Array — batch by size.
	if arr, ok := value.([]any); ok {
		chunkJSONArray(arr, path, chunks, maxChunkBytes)
		return
	}

	// Primitive that exceeds maxChunkBytes.
	*chunks = append(*chunks, chunk{title: title, content: serializedStr, hasCode: false})
}

// findIdentityField scans the first element of an array of objects for
// a recognizable identity field.
func findIdentityField(arr []any) string {
	if len(arr) == 0 {
		return ""
	}

	first, ok := arr[0].(map[string]any)
	if !ok {
		return ""
	}

	candidates := []string{"id", "name", "title", "path", "slug", "key", "label"}
	for _, field := range candidates {
		if v, exists := first[field]; exists {
			switch v.(type) {
			case string, float64:
				return field
			}
		}
	}

	return ""
}

func jsonBatchTitle(prefix string, startIdx, endIdx int, batch []any, identityField string) string {
	sep := ""
	if prefix != "" {
		sep = prefix + " > "
	}

	if identityField == "" {
		if startIdx == endIdx {
			return fmt.Sprintf("%s[%d]", sep, startIdx)
		}

		return fmt.Sprintf("%s[%d-%d]", sep, startIdx, endIdx)
	}

	getID := func(item any) string {
		if m, ok := item.(map[string]any); ok {
			return fmt.Sprintf("%v", m[identityField])
		}

		return ""
	}

	if len(batch) == 1 {
		return fmt.Sprintf("%s%s", sep, getID(batch[0]))
	}

	if len(batch) <= 3 {
		ids := make([]string, len(batch))
		for i, item := range batch {
			ids[i] = getID(item)
		}

		return sep + strings.Join(ids, ", ")
	}

	return fmt.Sprintf("%s%s\u2026%s", sep, getID(batch[0]), getID(batch[len(batch)-1]))
}

func chunkJSONArray(arr []any, path []string, chunks *[]chunk, maxChunkBytes int) {
	prefix := "(root)"
	if len(path) > 0 {
		prefix = strings.Join(path, " > ")
	}

	identityField := findIdentityField(arr)

	var batch []any

	batchStart := 0

	flushBatch := func(batchEnd int) {
		if len(batch) == 0 {
			return
		}

		title := jsonBatchTitle(prefix, batchStart, batchEnd, batch, identityField)

		serialized, err := json.MarshalIndent(batch, "", "  ")
		if err != nil {
			return
		}

		*chunks = append(*chunks, chunk{title: title, content: string(serialized), hasCode: true})
	}

	for i := range arr {
		batch = append(batch, arr[i])

		candidate, err := json.MarshalIndent(batch, "", "  ")
		if err != nil {
			continue
		}

		if len(candidate) > maxChunkBytes && len(batch) > 1 {
			batch = batch[:len(batch)-1]

			flushBatch(i - 1)
			batch = []any{arr[i]}
			batchStart = i
		}
	}

	flushBatch(batchStart + len(batch) - 1)
}
