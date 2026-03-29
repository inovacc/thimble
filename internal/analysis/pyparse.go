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
	rePyClass     = regexp.MustCompile(`^class\s+(\w+)(?:\(([^)]*)\))?\s*:`)
	rePyFunc      = regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*(\S+))?\s*:`)
	rePyMethod    = regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*(\S+))?\s*:`)
	rePyImport    = regexp.MustCompile(`^import\s+(.+)`)
	rePyFromImpt  = regexp.MustCompile(`^from\s+(\S+)\s+import`)
	rePyConst     = regexp.MustCompile(`^([A-Z][A-Z0-9_]+)\s*(?::\s*(\S+)\s*)?=`)
	rePyDecorator = regexp.MustCompile(`^@(\w+)`)
)

// ParsePythonFile parses a Python file and extracts classes, functions, constants, and imports.
func ParsePythonFile(path string) (*FileResult, error) { //nolint:maintidx // complex parser by nature
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	defer func() { _ = f.Close() }()

	result := &FileResult{
		Path:     filepath.ToSlash(path),
		Package:  inferPyPackage(path),
		Language: LangPython,
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0

	var (
		currentClass      string
		pendingDecorators []string
		docLines          []string
	)

	inDocstring := false
	docstringQuote := ""

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Track docstrings (triple quotes).
		if inDocstring {
			if strings.Contains(trimmed, docstringQuote) {
				inDocstring = false
			}

			continue
		}

		for _, q := range []string{`"""`, `'''`} {
			if strings.HasPrefix(trimmed, q) {
				if strings.Count(trimmed, q) == 1 {
					// Multi-line docstring starts.
					inDocstring = true
					docstringQuote = q
				}
				// Single-line docstring — skip.
				continue
			}
		}

		// Skip comments and empty lines.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Collect docstring from comment above symbol.
		if strings.HasPrefix(trimmed, "#") {
			docLines = append(docLines, strings.TrimPrefix(trimmed, "# "))
			continue
		}

		// Decorators.
		if m := rePyDecorator.FindStringSubmatch(trimmed); m != nil {
			pendingDecorators = append(pendingDecorators, m[1])
			continue
		}

		// Imports.
		if m := rePyImport.FindStringSubmatch(trimmed); m != nil {
			for mod := range strings.SplitSeq(m[1], ",") {
				mod = strings.TrimSpace(strings.Split(mod, " as ")[0])
				if mod != "" {
					result.Imports = append(result.Imports, mod)
				}
			}

			pendingDecorators = nil
			docLines = nil

			continue
		}

		if m := rePyFromImpt.FindStringSubmatch(trimmed); m != nil {
			result.Imports = append(result.Imports, m[1])
			pendingDecorators = nil
			docLines = nil

			continue
		}

		// Class.
		if m := rePyClass.FindStringSubmatch(trimmed); m != nil {
			currentClass = m[1]

			sig := "class " + m[1]
			if m[2] != "" {
				sig += "(" + m[2] + ")"
			}

			sym := Symbol{
				Name:      m[1],
				Kind:      KindStruct,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  !strings.HasPrefix(m[1], "_"),
				Doc:       strings.Join(docLines, " "),
			}
			result.Symbols = append(result.Symbols, sym)
			pendingDecorators = nil
			docLines = nil

			continue
		}

		// Function or method — check indentation to distinguish.
		if m := rePyFunc.FindStringSubmatch(trimmed); m != nil {
			isIndented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")

			if !isIndented || currentClass == "" {
				// Top-level function.
				currentClass = ""

				sig := fmt.Sprintf("def %s(%s)", m[1], m[2])
				if m[3] != "" {
					sig += " -> " + m[3]
				}

				sym := Symbol{
					Name:      m[1],
					Kind:      KindFunction,
					Package:   result.Package,
					File:      filepath.ToSlash(path),
					Line:      lineNum,
					Signature: sig,
					Exported:  !strings.HasPrefix(m[1], "_"),
					Doc:       strings.Join(docLines, " "),
				}
				result.Symbols = append(result.Symbols, sym)
				pendingDecorators = nil
				docLines = nil

				continue
			}

			// Method (indented def inside a class).
			m = rePyMethod.FindStringSubmatch(trimmed)
			if m == nil {
				continue
			}
			// Skip dunder methods except __init__.
			name := m[1]
			if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") && name != "__init__" {
				pendingDecorators = nil
				docLines = nil

				continue
			}

			sig := fmt.Sprintf("def %s(%s)", name, m[2])
			if m[3] != "" {
				sig += " -> " + m[3]
			}

			sym := Symbol{
				Name:      name,
				Kind:      KindMethod,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  !strings.HasPrefix(name, "_"),
				Receiver:  currentClass,
				Doc:       strings.Join(docLines, " "),
			}
			result.Symbols = append(result.Symbols, sym)
			pendingDecorators = nil
			docLines = nil

			continue
		}

		// Top-level constant (ALL_CAPS = ...).
		if m := rePyConst.FindStringSubmatch(trimmed); m != nil && currentClass == "" {
			sig := m[1]
			if m[2] != "" {
				sig += ": " + m[2]
			}

			sym := Symbol{
				Name:      m[1],
				Kind:      KindConstant,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  true,
			}
			result.Symbols = append(result.Symbols, sym)
			pendingDecorators = nil
			docLines = nil

			continue
		}

		// Non-class top-level line resets class context.
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			currentClass = ""
		}

		_ = pendingDecorators // used for future decorator tracking
		docLines = nil
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	return result, nil
}

func inferPyPackage(path string) string {
	dir := filepath.Dir(path)

	base := filepath.Base(dir)
	if base == "." || base == "/" {
		return "root"
	}

	return base
}
