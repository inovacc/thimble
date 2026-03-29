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
	// package com.example.myapp; (Java) or package com.example.app (Kotlin)
	reJavaPkg = regexp.MustCompile(`^package\s+([\w.]+)\s*;?`)
	// import com.example.Foo; or import static com.example.Foo.bar; or Kotlin import without semicolon
	reJavaImport = regexp.MustCompile(`^import\s+(?:static\s+)?([\w.*]+)\s*;?`)
	// public class Foo / private class Foo / class Foo / abstract class Foo
	reJavaClass = regexp.MustCompile(`^(?:(?:public|private|protected)\s+)?(?:abstract\s+)?(?:final\s+)?class\s+(\w+)`)
	// public interface Foo / interface Foo
	reJavaInterface = regexp.MustCompile(`^(?:(?:public|private|protected)\s+)?interface\s+(\w+)`)
	// public enum Foo / enum Foo
	reJavaEnum = regexp.MustCompile(`^(?:(?:public|private|protected)\s+)?enum\s+(\w+)`)
	// @Annotation or @Annotation(...)
	reJavaAnnotation = regexp.MustCompile(`^@(\w+)`)
	// Method: visibility [static] [abstract] [final] [synchronized] ReturnType name(params) { or ;
	reJavaMethod = regexp.MustCompile(`^(?:(public|private|protected)\s+)?(?:static\s+)?(?:abstract\s+)?(?:final\s+)?(?:synchronized\s+)?(?:default\s+)?` +
		`(?:[\w<>\[\]?,\s]+)\s+(\w+)\s*\([^)]*\)\s*(?:throws\s+[\w,\s]+)?[{;]?`)
	// Kotlin: fun name(params): ReturnType { or fun name(params) {
	reKotlinFun = regexp.MustCompile(`^(?:(?:public|private|protected|internal)\s+)?(?:override\s+)?(?:open\s+)?(?:abstract\s+)?(?:suspend\s+)?fun\s+(\w+)\s*\(`)
	// Kotlin: class/data class/object/sealed class
	reKotlinClass = regexp.MustCompile(`^(?:(?:public|private|protected|internal)\s+)?(?:data\s+|sealed\s+|abstract\s+|open\s+|inner\s+)?class\s+(\w+)`)
	// Kotlin: object Foo
	reKotlinObject = regexp.MustCompile(`^(?:(?:public|private|protected|internal)\s+)?(?:companion\s+)?object\s+(\w+)`)
	// Kotlin: interface Foo
	reKotlinInterface = regexp.MustCompile(`^(?:(?:public|private|protected|internal)\s+)?(?:sealed\s+)?interface\s+(\w+)`)
	// Kotlin: enum class Foo
	reKotlinEnum = regexp.MustCompile(`^(?:(?:public|private|protected|internal)\s+)?enum\s+class\s+(\w+)`)
)

// ParseJavaFile parses a Java or Kotlin source file and extracts packages,
// imports, classes, interfaces, enums, methods, and annotations.
func ParseJavaFile(path string) (*FileResult, error) { //nolint:maintidx
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	defer func() { _ = f.Close() }()

	ext := strings.ToLower(filepath.Ext(path))
	isKotlin := ext == ".kt" || ext == ".kts"

	result := &FileResult{
		Path:     filepath.ToSlash(path),
		Language: LangJava,
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0
	braceDepth := 0
	inBlockComment := false

	var pendingAnnotations []string

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

		// Package declaration.
		if m := reJavaPkg.FindStringSubmatch(line); m != nil {
			result.Package = m[1]
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindPackage,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: line,
				Exported:  true,
			})

			continue
		}

		// Import statements.
		if m := reJavaImport.FindStringSubmatch(line); m != nil {
			result.Imports = append(result.Imports, m[1])

			continue
		}

		// Collect annotations (attached to the next declaration).
		if m := reJavaAnnotation.FindStringSubmatch(line); m != nil {
			// Only collect if the line is purely an annotation (not an annotation on same line as decl).
			if !strings.Contains(line, "class ") && !strings.Contains(line, "interface ") &&
				!strings.Contains(line, "enum ") && !strings.Contains(line, "fun ") {
				pendingAnnotations = append(pendingAnnotations, "@"+m[1])

				continue
			}
		}

		// Track brace depth.
		openBraces := strings.Count(line, "{")
		closeBraces := strings.Count(line, "}")
		prevDepth := braceDepth
		braceDepth += openBraces - closeBraces

		if braceDepth < 0 {
			braceDepth = 0
		}

		annotations := strings.Join(pendingAnnotations, " ")
		pendingAnnotations = nil

		if isKotlin {
			// Kotlin enum class (check before class).
			if m := reKotlinEnum.FindStringSubmatch(line); m != nil {
				result.Symbols = append(result.Symbols, Symbol{
					Name:      m[1],
					Kind:      KindType,
					File:      filepath.ToSlash(path),
					Line:      lineNum,
					Signature: fmt.Sprintf("enum class %s", m[1]),
					Exported:  isKotlinExported(line),
					Doc:       annotations,
				})

				continue
			}

			// Kotlin interface.
			if m := reKotlinInterface.FindStringSubmatch(line); m != nil {
				result.Symbols = append(result.Symbols, Symbol{
					Name:      m[1],
					Kind:      KindInterface,
					File:      filepath.ToSlash(path),
					Line:      lineNum,
					Signature: fmt.Sprintf("interface %s", m[1]),
					Exported:  isKotlinExported(line),
					Doc:       annotations,
				})

				continue
			}

			// Kotlin class/data class/sealed class.
			if m := reKotlinClass.FindStringSubmatch(line); m != nil {
				result.Symbols = append(result.Symbols, Symbol{
					Name:      m[1],
					Kind:      KindType,
					File:      filepath.ToSlash(path),
					Line:      lineNum,
					Signature: extractKotlinClassSig(line, m[1]),
					Exported:  isKotlinExported(line),
					Doc:       annotations,
				})

				continue
			}

			// Kotlin object.
			if m := reKotlinObject.FindStringSubmatch(line); m != nil {
				result.Symbols = append(result.Symbols, Symbol{
					Name:      m[1],
					Kind:      KindType,
					File:      filepath.ToSlash(path),
					Line:      lineNum,
					Signature: fmt.Sprintf("object %s", m[1]),
					Exported:  isKotlinExported(line),
					Doc:       annotations,
				})

				continue
			}

			// Kotlin fun (at any depth for method extraction).
			if m := reKotlinFun.FindStringSubmatch(line); m != nil {
				kind := KindFunction
				if prevDepth > 0 {
					kind = KindMethod
				}

				result.Symbols = append(result.Symbols, Symbol{
					Name:      m[1],
					Kind:      kind,
					File:      filepath.ToSlash(path),
					Line:      lineNum,
					Signature: strings.TrimSuffix(strings.TrimSpace(line), "{"),
					Exported:  isKotlinExported(line),
					Doc:       annotations,
				})

				continue
			}
		} else {
			// Java enum (check before class).
			if m := reJavaEnum.FindStringSubmatch(line); m != nil {
				result.Symbols = append(result.Symbols, Symbol{
					Name:      m[1],
					Kind:      KindType,
					File:      filepath.ToSlash(path),
					Line:      lineNum,
					Signature: fmt.Sprintf("enum %s", m[1]),
					Exported:  isJavaPublic(line),
					Doc:       annotations,
				})

				continue
			}

			// Java interface.
			if m := reJavaInterface.FindStringSubmatch(line); m != nil {
				result.Symbols = append(result.Symbols, Symbol{
					Name:      m[1],
					Kind:      KindInterface,
					File:      filepath.ToSlash(path),
					Line:      lineNum,
					Signature: fmt.Sprintf("interface %s", m[1]),
					Exported:  isJavaPublic(line),
					Doc:       annotations,
				})

				continue
			}

			// Java class.
			if m := reJavaClass.FindStringSubmatch(line); m != nil {
				result.Symbols = append(result.Symbols, Symbol{
					Name:      m[1],
					Kind:      KindType,
					File:      filepath.ToSlash(path),
					Line:      lineNum,
					Signature: extractJavaClassSig(line, m[1]),
					Exported:  isJavaPublic(line),
					Doc:       annotations,
				})

				continue
			}

			// Java method (only inside a class, i.e., prevDepth > 0).
			if prevDepth > 0 {
				if m := reJavaMethod.FindStringSubmatch(line); m != nil {
					name := m[2]
					// Skip control flow false positives.
					if name == "if" || name == "for" || name == "while" || name == "switch" || name == "return" || name == "catch" || name == "new" {
						continue
					}

					visibility := m[1]
					exported := visibility == "public" || visibility == ""

					result.Symbols = append(result.Symbols, Symbol{
						Name:      name,
						Kind:      KindMethod,
						File:      filepath.ToSlash(path),
						Line:      lineNum,
						Signature: strings.TrimSuffix(strings.TrimSpace(line), "{"),
						Exported:  exported,
						Doc:       annotations,
					})
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	return result, nil
}

// isJavaPublic checks if a declaration line has public visibility.
func isJavaPublic(line string) bool {
	return strings.Contains(line, "public ")
}

// isKotlinExported checks if a Kotlin declaration is exported (not private/protected).
func isKotlinExported(line string) bool {
	return !strings.HasPrefix(strings.TrimSpace(line), "private ") &&
		!strings.HasPrefix(strings.TrimSpace(line), "protected ")
}

// extractJavaClassSig builds a readable signature for a Java class.
func extractJavaClassSig(line, name string) string {
	if strings.Contains(line, "abstract ") {
		return fmt.Sprintf("abstract class %s", name)
	}

	return fmt.Sprintf("class %s", name)
}

// extractKotlinClassSig builds a readable signature for a Kotlin class.
func extractKotlinClassSig(line, name string) string {
	if strings.Contains(line, "data ") {
		return fmt.Sprintf("data class %s", name)
	}

	if strings.Contains(line, "sealed ") {
		return fmt.Sprintf("sealed class %s", name)
	}

	return fmt.Sprintf("class %s", name)
}
