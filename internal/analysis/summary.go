package analysis

import (
	"fmt"
	"strings"
)

// FileSummary generates a 1-2 line description of a file from its symbols.
func FileSummary(fr *FileResult) string {
	if len(fr.Symbols) == 0 {
		return fmt.Sprintf("%s: empty %s file in package %s", fr.Path, fr.Language, fr.Package)
	}

	var types, funcs, methods []string

	for _, sym := range fr.Symbols {
		if !sym.Exported {
			continue
		}

		switch sym.Kind {
		case KindStruct, KindInterface, KindType:
			types = append(types, sym.Name)
		case KindFunction:
			funcs = append(funcs, sym.Name)
		case KindMethod:
			methods = append(methods, sym.Receiver+"."+sym.Name)
		case KindConstant, KindVariable, KindPackage:
			// Not included in file summaries.
		}
	}

	var parts []string
	if len(types) > 0 {
		parts = append(parts, "types: "+joinLimit(types, 5))
	}

	if len(funcs) > 0 {
		parts = append(parts, "funcs: "+joinLimit(funcs, 5))
	}

	if len(methods) > 0 {
		parts = append(parts, "methods: "+joinLimit(methods, 5))
	}

	if len(parts) == 0 {
		return fmt.Sprintf("%s: package %s (%d unexported symbols)", fr.Path, fr.Package, len(fr.Symbols))
	}

	return fmt.Sprintf("%s [%s]: %s", fr.Path, fr.Package, strings.Join(parts, "; "))
}

// GenerateFileSummaries produces a markdown document with one-line descriptions per file.
func GenerateFileSummaries(result *AnalysisResult) string {
	if len(result.Files) == 0 {
		return "No files analyzed."
	}

	var sb strings.Builder
	sb.WriteString("# File Summaries\n\n")

	for i := range result.Files {
		fmt.Fprintf(&sb, "- %s\n", FileSummary(&result.Files[i]))
	}

	return sb.String()
}

// GenerateDepGraphMarkdown formats the dependency graph as markdown.
func GenerateDepGraphMarkdown(result *AnalysisResult) string {
	if len(result.DepGraph) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Package Dependencies\n\n")

	for pkg, deps := range result.DepGraph {
		fmt.Fprintf(&sb, "## %s\n", pkg)

		for _, dep := range deps {
			fmt.Fprintf(&sb, "- %s\n", dep)
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// GenerateMermaidGraph produces a Mermaid flowchart from the dependency graph.
// Only includes internal packages (filters out stdlib and external modules).
func GenerateMermaidGraph(result *AnalysisResult, modulePrefix string) string {
	if len(result.DepGraph) == 0 {
		return ""
	}

	// Collect internal packages.
	internal := make(map[string]bool)
	for pkg := range result.Packages {
		internal[pkg] = true
	}

	var sb strings.Builder
	sb.WriteString("```mermaid\ngraph TD\n")

	seen := make(map[string]bool)

	for pkg, deps := range result.DepGraph {
		if !internal[pkg] {
			continue
		}

		nodeID := sanitizeMermaidID(pkg)

		for _, dep := range deps {
			// Only show internal deps or deps matching the module prefix.
			depPkg := extractPackageName(dep, modulePrefix)
			if depPkg == "" {
				continue
			}

			if !internal[depPkg] && (modulePrefix == "" || !strings.HasPrefix(dep, modulePrefix)) {
				continue
			}

			depID := sanitizeMermaidID(depPkg)

			edge := nodeID + " --> " + depID
			if !seen[edge] {
				fmt.Fprintf(&sb, "    %s[\"%s\"] --> %s[\"%s\"]\n", nodeID, pkg, depID, depPkg)

				seen[edge] = true
			}
		}
	}

	sb.WriteString("```\n")

	return sb.String()
}

// sanitizeMermaidID converts a package name to a valid Mermaid node ID.
func sanitizeMermaidID(name string) string {
	r := strings.NewReplacer("/", "_", ".", "_", "-", "_")
	return r.Replace(name)
}

// extractPackageName extracts a short package name from an import path.
// If modulePrefix is set and the import starts with it, returns the relative path.
// Otherwise returns the last path component.
func extractPackageName(importPath, modulePrefix string) string {
	if modulePrefix != "" && strings.HasPrefix(importPath, modulePrefix) {
		rel := strings.TrimPrefix(importPath, modulePrefix)

		rel = strings.TrimPrefix(rel, "/")
		if rel != "" {
			return rel
		}
	}
	// For internal detection, return last path segment.
	parts := strings.Split(importPath, "/")

	return parts[len(parts)-1]
}

func joinLimit(items []string, limit int) string {
	if len(items) <= limit {
		return strings.Join(items, ", ")
	}

	return strings.Join(items[:limit], ", ") + fmt.Sprintf(" (+%d more)", len(items)-limit)
}
