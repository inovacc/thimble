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
	reRustFn     = regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:extern\s+"[^"]*"\s+)?fn\s+(\w+)(?:<[^>]*>)?\s*\(([^)]*)\)(?:\s*->\s*(.+?))?(?:\s*(?:where|{|$))`)
	reRustStruct = regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?struct\s+(\w+)(?:<[^>]*>)?`)
	reRustEnum   = regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?enum\s+(\w+)(?:<[^>]*>)?`)
	reRustTrait  = regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?trait\s+(\w+)(?:<[^>]*>)?`)
	reRustImpl   = regexp.MustCompile(`^impl(?:<[^>]*>)?\s+(?:(\w+)(?:<[^>]*>)?\s+for\s+)?(\w+)(?:<[^>]*>)?`)
	reRustConst  = regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?const\s+(\w+)\s*:\s*(.+?)\s*=`)
	reRustType   = regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?type\s+(\w+)(?:<[^>]*>)?\s*=\s*(.+?)\s*;`)
	reRustUse    = regexp.MustCompile(`^(?:pub\s+)?use\s+(.+?)\s*;`)
	reRustMod    = regexp.MustCompile(`^(?:pub\s+)?mod\s+(\w+)`)
	reRustPub    = regexp.MustCompile(`^pub(?:\([^)]*\))?\s+`)
)

// ParseRustFile parses a Rust file and extracts structs, enums, traits, functions, methods, constants, and imports.
func ParseRustFile(path string) (*FileResult, error) { //nolint:maintidx // complex parser by nature
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	defer func() { _ = f.Close() }()

	result := &FileResult{
		Path:     filepath.ToSlash(path),
		Package:  inferRustPackage(path),
		Language: LangRust,
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0

	var (
		currentImpl  string
		currentTrait string
	)

	braceDepth := 0
	implBraceStart := -1
	traitBraceStart := -1

	var docLines []string

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and line comments.
		if trimmed == "" {
			docLines = nil
			continue
		}

		// Collect doc comments.
		if after, ok := strings.CutPrefix(trimmed, "///"); ok {
			doc := after
			doc = strings.TrimSpace(doc)
			docLines = append(docLines, doc)

			continue
		}

		if strings.HasPrefix(trimmed, "//") {
			docLines = nil
			continue
		}

		// Track brace depth for impl/trait blocks.
		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")

		isPub := reRustPub.MatchString(trimmed)

		// Imports (use statements).
		if m := reRustUse.FindStringSubmatch(trimmed); m != nil {
			result.Imports = append(result.Imports, m[1])
			docLines = nil
			braceDepth += openBraces - closeBraces

			continue
		}

		// Module declarations.
		if reRustMod.MatchString(trimmed) {
			docLines = nil
			braceDepth += openBraces - closeBraces

			continue
		}

		// Impl block.
		if m := reRustImpl.FindStringSubmatch(trimmed); m != nil {
			currentImpl = m[2]
			implBraceStart = braceDepth
			braceDepth += openBraces - closeBraces
			docLines = nil

			continue
		}

		// Trait definition.
		if m := reRustTrait.FindStringSubmatch(trimmed); m != nil {
			sig := "trait " + m[1]
			sym := Symbol{
				Name:      m[1],
				Kind:      KindInterface,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  isPub,
				Doc:       strings.Join(docLines, " "),
			}
			result.Symbols = append(result.Symbols, sym)
			currentTrait = m[1]
			traitBraceStart = braceDepth
			braceDepth += openBraces - closeBraces
			docLines = nil

			continue
		}

		// Struct definition.
		if m := reRustStruct.FindStringSubmatch(trimmed); m != nil {
			sig := "struct " + m[1]
			sym := Symbol{
				Name:      m[1],
				Kind:      KindStruct,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  isPub,
				Doc:       strings.Join(docLines, " "),
			}
			result.Symbols = append(result.Symbols, sym)
			braceDepth += openBraces - closeBraces
			docLines = nil

			continue
		}

		// Enum definition.
		if m := reRustEnum.FindStringSubmatch(trimmed); m != nil {
			sig := "enum " + m[1]
			sym := Symbol{
				Name:      m[1],
				Kind:      KindType,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  isPub,
				Doc:       strings.Join(docLines, " "),
			}
			result.Symbols = append(result.Symbols, sym)
			braceDepth += openBraces - closeBraces
			docLines = nil

			continue
		}

		// Constant definition.
		if m := reRustConst.FindStringSubmatch(trimmed); m != nil {
			sig := fmt.Sprintf("const %s: %s", m[1], m[2])
			sym := Symbol{
				Name:      m[1],
				Kind:      KindConstant,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  isPub,
				Doc:       strings.Join(docLines, " "),
			}
			result.Symbols = append(result.Symbols, sym)
			braceDepth += openBraces - closeBraces
			docLines = nil

			continue
		}

		// Type alias.
		if m := reRustType.FindStringSubmatch(trimmed); m != nil {
			sig := fmt.Sprintf("type %s = %s", m[1], m[2])
			sym := Symbol{
				Name:      m[1],
				Kind:      KindType,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  isPub,
				Doc:       strings.Join(docLines, " "),
			}
			result.Symbols = append(result.Symbols, sym)
			braceDepth += openBraces - closeBraces
			docLines = nil

			continue
		}

		// Function or method.
		if m := reRustFn.FindStringSubmatch(trimmed); m != nil {
			name := m[1]
			params := m[2]
			retType := m[3]

			sig := fmt.Sprintf("fn %s(%s)", name, params)
			if retType != "" {
				sig += " -> " + retType
			}

			kind := KindFunction
			receiver := ""

			if currentImpl != "" {
				kind = KindMethod
				receiver = currentImpl
			} else if currentTrait != "" {
				kind = KindMethod
				receiver = currentTrait
			}

			sym := Symbol{
				Name:      name,
				Kind:      kind,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  isPub,
				Receiver:  receiver,
				Doc:       strings.Join(docLines, " "),
			}
			result.Symbols = append(result.Symbols, sym)
			braceDepth += openBraces - closeBraces
			docLines = nil

			continue
		}

		// Update brace depth.
		braceDepth += openBraces - closeBraces

		// Check if we exited an impl block.
		if currentImpl != "" && braceDepth <= implBraceStart {
			currentImpl = ""
			implBraceStart = -1
		}

		// Check if we exited a trait block.
		if currentTrait != "" && braceDepth <= traitBraceStart {
			currentTrait = ""
			traitBraceStart = -1
		}

		docLines = nil
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	return result, nil
}

func inferRustPackage(path string) string {
	dir := filepath.Dir(path)

	base := filepath.Base(dir)
	if base == "." || base == "/" {
		return "root"
	}

	return base
}
