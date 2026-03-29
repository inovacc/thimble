// Package analysis provides static code analysis for codebase context extraction.
// It parses source files to extract symbols, signatures, and cross-references
// that are indexed into the knowledge base for context-aware search.
package analysis

// Language identifies a programming language for analysis.
type Language string

const (
	LangGo         Language = "go"
	LangTypeScript Language = "typescript"
	LangPython     Language = "python"
	LangRust       Language = "rust"
	LangProto      Language = "proto"
	LangC          Language = "c"
	LangJava       Language = "java"
)

// SymbolKind classifies extracted symbols.
type SymbolKind string

const (
	KindFunction  SymbolKind = "function"
	KindMethod    SymbolKind = "method"
	KindType      SymbolKind = "type"
	KindStruct    SymbolKind = "struct"
	KindInterface SymbolKind = "interface"
	KindConstant  SymbolKind = "constant"
	KindVariable  SymbolKind = "variable"
	KindPackage   SymbolKind = "package"
)

// Symbol represents an extracted code symbol with its location and signature.
type Symbol struct {
	Name      string     `json:"name"`
	Kind      SymbolKind `json:"kind"`
	Package   string     `json:"package"`
	File      string     `json:"file"`
	Line      int        `json:"line"`
	Signature string     `json:"signature"`
	Doc       string     `json:"doc,omitempty"`
	Exported  bool       `json:"exported"`
	Receiver  string     `json:"receiver,omitempty"` // For methods: receiver type name.
}

// Reference represents a cross-reference between symbols.
type Reference struct {
	From     string `json:"from"`      // Fully qualified caller (pkg.Symbol).
	To       string `json:"to"`        // Fully qualified callee (pkg.Symbol).
	Kind     string `json:"kind"`      // "call", "implements", "embeds", "import".
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// FileResult holds the analysis output for a single source file.
type FileResult struct {
	Path       string      `json:"path"`
	Package    string      `json:"package"`
	Language   Language    `json:"language"`
	Symbols    []Symbol    `json:"symbols"`
	References []Reference `json:"references"`
	Imports    []string    `json:"imports"`
}

// AnalysisResult holds the aggregated output of a codebase analysis.
type AnalysisResult struct {
	Files      []FileResult         `json:"files"`
	Packages   map[string]int       `json:"packages"`    // Package name → symbol count.
	DepGraph   map[string][]string  `json:"dep_graph"`   // Package → imported packages.
	Summary    AnalysisSummary      `json:"summary"`
}

// AnalysisSummary provides high-level stats about the analysis.
type AnalysisSummary struct {
	TotalFiles      int            `json:"total_files"`
	TotalSymbols    int            `json:"total_symbols"`
	TotalReferences int            `json:"total_references"`
	ByKind          map[string]int `json:"by_kind"`
	ByLanguage      map[string]int `json:"by_language"`
}
