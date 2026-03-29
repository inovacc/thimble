package analysis

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// #include <header> or #include "header"
	reCInclude = regexp.MustCompile(`^#include\s+[<"]([^>"]+)[>"]`)
	// #define NAME value
	reCDefine = regexp.MustCompile(`^#define\s+(\w+)`)
	// struct name { or struct name\n{
	reCStruct = regexp.MustCompile(`^(?:typedef\s+)?struct\s+(\w+)\s*\{?`)
	// enum name { or enum name\n{
	reCEnum = regexp.MustCompile(`^(?:typedef\s+)?enum\s+(\w+)\s*\{?`)
	// typedef ... name; (struct/enum typedefs are handled separately before this check)
	reCTypedef = regexp.MustCompile(`^typedef\s+.+\b(\w+)\s*;`)
	// Function definition: type name(params) { — must start at column 0.
	// Matches: int main(int argc, char **argv) {
	// Matches: static void helper(void)
	// Matches: const char *get_name(void)
	// Does not match indented lines (those are calls, not definitions).
	reCFunc = regexp.MustCompile(`^(?:static\s+)?(?:inline\s+)?(?:const\s+)?(?:unsigned\s+)?(?:signed\s+)?(?:volatile\s+)?(?:extern\s+)?` +
		`(?:void|int|long|short|char|float|double|bool|size_t|ssize_t|uint\d+_t|int\d+_t|` +
		`(?:struct\s+)?\w+)` +
		`\s*[*]*\s*(\w+)\s*\([^)]*\)\s*\{?`)
)

// ParseCFile parses a C/C++ source file and extracts functions, structs, typedefs,
// enums, #include imports, and #define constants.
func ParseCFile(path string) (*FileResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	defer func() { _ = f.Close() }()

	result := &FileResult{
		Path:     filepath.ToSlash(path),
		Language: LangC,
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0
	braceDepth := 0
	inBlockComment := false

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)

		// Handle block comments.
		if inBlockComment {
			if idx := strings.Index(line, "*/"); idx >= 0 {
				inBlockComment = false
				line = strings.TrimSpace(line[idx+2:])
			} else {
				continue
			}
		}

		// Skip empty lines and single-line comments.
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Detect start of block comment.
		if strings.HasPrefix(line, "/*") {
			if !strings.Contains(line, "*/") {
				inBlockComment = true
				continue
			}
			// Single-line block comment, skip it.
			continue
		}

		// #include imports.
		if m := reCInclude.FindStringSubmatch(line); m != nil {
			result.Imports = append(result.Imports, m[1])
			continue
		}

		// #define constants.
		if m := reCDefine.FindStringSubmatch(line); m != nil {
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindConstant,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: line,
				Exported:  true,
			})

			continue
		}

		// Track brace depth — only extract top-level symbols.
		openBraces := strings.Count(line, "{")
		closeBraces := strings.Count(line, "}")
		prevDepth := braceDepth
		braceDepth += openBraces - closeBraces

		if braceDepth < 0 {
			braceDepth = 0
		}

		// Only extract declarations at top level (braceDepth == 0 before this line).
		if prevDepth > 0 {
			continue
		}

		// Struct declaration.
		if m := reCStruct.FindStringSubmatch(line); m != nil {
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindStruct,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: fmt.Sprintf("struct %s", m[1]),
				Exported:  true,
			})

			continue
		}

		// Enum declaration.
		if m := reCEnum.FindStringSubmatch(line); m != nil {
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindType,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: fmt.Sprintf("enum %s", m[1]),
				Exported:  true,
			})

			continue
		}

		// Typedef (non-struct, non-enum).
		if m := reCTypedef.FindStringSubmatch(line); m != nil {
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindType,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: line,
				Exported:  true,
			})

			continue
		}

		// Function definition (must start at column 0, not indented).
		if len(raw) > 0 && raw[0] != ' ' && raw[0] != '\t' {
			if m := reCFunc.FindStringSubmatch(line); m != nil {
				name := m[1]
				// Skip common false positives.
				if name == "if" || name == "for" || name == "while" || name == "switch" || name == "return" {
					continue
				}

				result.Symbols = append(result.Symbols, Symbol{
					Name:      name,
					Kind:      KindFunction,
					File:      filepath.ToSlash(path),
					Line:      lineNum,
					Signature: strings.TrimSuffix(strings.TrimSpace(line), "{"),
					Exported:  true,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	return result, nil
}
