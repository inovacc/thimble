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
	// Export patterns.
	reExportFunction   = regexp.MustCompile(`^export\s+(?:async\s+)?function\s+(\w+)\s*(<[^>]*>)?\s*\(([^)]*)\)(?:\s*:\s*([^\s{]+))?`)
	reExportClass      = regexp.MustCompile(`^export\s+(?:abstract\s+)?class\s+(\w+)(?:\s+extends\s+(\w+))?(?:\s+implements\s+([\w,\s]+))?\s*\{?`)
	reExportInterface  = regexp.MustCompile(`^export\s+interface\s+(\w+)(?:\s+extends\s+([\w,\s]+))?\s*\{?`)
	reExportType       = regexp.MustCompile(`^export\s+type\s+(\w+)`)
	reExportConst      = regexp.MustCompile(`^export\s+const\s+(\w+)(?:\s*:\s*(\S+))?\s*=`)
	reExportEnum       = regexp.MustCompile(`^export\s+enum\s+(\w+)`)
	reExportDefault    = regexp.MustCompile(`^export\s+default\s+(?:class|function|abstract class)\s+(\w+)`)
	reImport           = regexp.MustCompile(`^import\s+.*from\s+['"]([^'"]+)['"]`)
	reImportSideEffect = regexp.MustCompile(`^import\s+['"]([^'"]+)['"]`)
)

// ParseTSFile parses a TypeScript/JavaScript file and extracts exported symbols.
func ParseTSFile(path string) (*FileResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	defer func() { _ = f.Close() }()

	lang := LangTypeScript

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".js" || ext == ".jsx" || ext == ".mjs" || ext == ".cjs" {
		lang = LangTypeScript // treat JS as TS for symbol purposes
	}

	result := &FileResult{
		Path:     filepath.ToSlash(path),
		Package:  inferTSPackage(path),
		Language: lang,
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines.
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") {
			continue
		}

		// Imports.
		if m := reImport.FindStringSubmatch(line); m != nil {
			result.Imports = append(result.Imports, m[1])
			continue
		}

		if m := reImportSideEffect.FindStringSubmatch(line); m != nil {
			result.Imports = append(result.Imports, m[1])
			continue
		}

		// Export default.
		if m := reExportDefault.FindStringSubmatch(line); m != nil {
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindFunction,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: strings.TrimSpace(line),
				Exported:  true,
			})

			continue
		}

		// Export function.
		if m := reExportFunction.FindStringSubmatch(line); m != nil {
			sig := fmt.Sprintf("function %s(%s)", m[1], m[3])
			if m[4] != "" {
				sig += ": " + m[4]
			}

			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindFunction,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  true,
			})

			continue
		}

		// Export class.
		if m := reExportClass.FindStringSubmatch(line); m != nil {
			sig := "class " + m[1]
			if m[2] != "" {
				sig += " extends " + m[2]
			}

			if m[3] != "" {
				sig += " implements " + strings.TrimSpace(m[3])
			}

			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindStruct,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  true,
			})

			continue
		}

		// Export interface.
		if m := reExportInterface.FindStringSubmatch(line); m != nil {
			sig := "interface " + m[1]
			if m[2] != "" {
				sig += " extends " + strings.TrimSpace(m[2])
			}

			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindInterface,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  true,
			})

			continue
		}

		// Export type alias.
		if m := reExportType.FindStringSubmatch(line); m != nil {
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindType,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: "type " + m[1],
				Exported:  true,
			})

			continue
		}

		// Export const.
		if m := reExportConst.FindStringSubmatch(line); m != nil {
			sig := "const " + m[1]
			if m[2] != "" {
				sig += ": " + m[2]
			}

			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindConstant,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  true,
			})

			continue
		}

		// Export enum.
		if m := reExportEnum.FindStringSubmatch(line); m != nil {
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindType,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: "enum " + m[1],
				Exported:  true,
			})

			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	return result, nil
}

// inferTSPackage extracts a package name from the directory structure.
func inferTSPackage(path string) string {
	dir := filepath.Dir(path)

	base := filepath.Base(dir)
	if base == "." || base == "/" {
		return "root"
	}

	return base
}
