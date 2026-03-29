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
	// function name() { ... } or function name { ... }
	reShellFuncKW = regexp.MustCompile(`^function\s+(\w[\w-]*)\s*(?:\(\s*\))?\s*\{?`)
	// name() { ... }
	reShellFuncParen = regexp.MustCompile(`^(\w[\w-]*)\s*\(\s*\)\s*\{?`)
	// readonly NAME=value or declare -r NAME=value
	reShellReadonly = regexp.MustCompile(`^(?:readonly|declare\s+-r)\s+(\w+)=`)
	// UPPER_CASE=value (constant-style variable at top level)
	reShellConstVar = regexp.MustCompile(`^([A-Z][A-Z0-9_]{2,})=`)
	// export NAME=value
	reShellExport = regexp.MustCompile(`^export\s+(\w+)=`)
	// source or . (imports)
	reShellSource = regexp.MustCompile(`^(?:source|\.)\s+["']?([^"'\s]+)["']?`)
	// alias name=value
	reShellAlias = regexp.MustCompile(`^alias\s+(\w[\w-]*)=`)
)

// ParseShellFile parses a shell script and extracts functions, constants, exports, and source imports.
func ParseShellFile(path string) (*FileResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	defer func() { _ = f.Close() }()

	result := &FileResult{
		Path:     filepath.ToSlash(path),
		Language: LangShell,
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0
	braceDepth := 0

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Track brace depth to distinguish top-level from function-body.
		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
		if braceDepth < 0 {
			braceDepth = 0
		}

		// Function: function name() or name().
		if m := reShellFuncKW.FindStringSubmatch(line); m != nil {
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindFunction,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: fmt.Sprintf("function %s()", m[1]),
				Exported:  true,
			})

			continue
		}

		if m := reShellFuncParen.FindStringSubmatch(line); m != nil {
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindFunction,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: fmt.Sprintf("%s()", m[1]),
				Exported:  true,
			})

			continue
		}

		// Only extract variables/constants at top level.
		if braceDepth > 0 {
			continue
		}

		// Readonly / declare -r constants.
		if m := reShellReadonly.FindStringSubmatch(line); m != nil {
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

		// UPPER_CASE constants.
		if m := reShellConstVar.FindStringSubmatch(line); m != nil {
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

		// Exported variables.
		if m := reShellExport.FindStringSubmatch(line); m != nil {
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindVariable,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: line,
				Exported:  true,
			})

			continue
		}

		// Aliases.
		if m := reShellAlias.FindStringSubmatch(line); m != nil {
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindFunction,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: line,
				Exported:  true,
			})

			continue
		}

		// Source imports.
		if m := reShellSource.FindStringSubmatch(line); m != nil {
			result.Imports = append(result.Imports, m[1])

			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	return result, nil
}
