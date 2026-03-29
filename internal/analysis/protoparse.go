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
	reService = regexp.MustCompile(`^service\s+(\w+)\s*\{`)
	reRPC     = regexp.MustCompile(`^\s*rpc\s+(\w+)\s*\((\w+)\)\s*returns\s*\((\w+)\)`)
	reMessage = regexp.MustCompile(`^message\s+(\w+)\s*\{`)
	reEnum    = regexp.MustCompile(`^enum\s+(\w+)\s*\{`)
	rePackage = regexp.MustCompile(`^package\s+([\w.]+)\s*;`)
)

// ParseProtoFile parses a .proto file and extracts services, RPCs, messages, and enums as symbols.
func ParseProtoFile(path string) (*FileResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	defer func() { _ = f.Close() }()

	result := &FileResult{
		Path:     filepath.ToSlash(path),
		Language: LangProto,
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0

	var currentService string

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines.
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Package declaration.
		if m := rePackage.FindStringSubmatch(line); m != nil {
			result.Package = m[1]
			continue
		}

		// Service declaration.
		if m := reService.FindStringSubmatch(line); m != nil {
			currentService = m[1]
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindInterface,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: fmt.Sprintf("service %s", m[1]),
				Exported:  true,
			})

			continue
		}

		// RPC declaration (inside a service).
		if m := reRPC.FindStringSubmatch(line); m != nil {
			sig := fmt.Sprintf("rpc %s(%s) returns (%s)", m[1], m[2], m[3])
			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindMethod,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: sig,
				Exported:  true,
				Receiver:  currentService,
			})

			continue
		}

		// Message declaration.
		if m := reMessage.FindStringSubmatch(line); m != nil {
			currentService = "" // reset service context

			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindStruct,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: fmt.Sprintf("message %s", m[1]),
				Exported:  true,
			})

			continue
		}

		// Enum declaration.
		if m := reEnum.FindStringSubmatch(line); m != nil {
			currentService = "" // reset service context

			result.Symbols = append(result.Symbols, Symbol{
				Name:      m[1],
				Kind:      KindType,
				Package:   result.Package,
				File:      filepath.ToSlash(path),
				Line:      lineNum,
				Signature: fmt.Sprintf("enum %s", m[1]),
				Exported:  true,
			})

			continue
		}

		// Closing brace resets service context.
		if line == "}" {
			currentService = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	return result, nil
}
