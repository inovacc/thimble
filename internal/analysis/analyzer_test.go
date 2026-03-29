package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create pkg1.
	pkg1 := filepath.Join(dir, "pkg1")
	_ = os.MkdirAll(pkg1, 0o755)
	_ = os.WriteFile(filepath.Join(pkg1, "main.go"), []byte(`package pkg1

// Server handles requests.
type Server struct {
	Port int
}

// New creates a Server.
func New(port int) *Server {
	return &Server{Port: port}
}

func (s *Server) Start() error { return nil }
`), 0o644)

	// Create pkg2.
	pkg2 := filepath.Join(dir, "pkg2")
	_ = os.MkdirAll(pkg2, 0o755)
	_ = os.WriteFile(filepath.Join(pkg2, "util.go"), []byte(`package pkg2

const Version = "1.0.0"

func Add(a, b int) int { return a + b }
`), 0o644)

	// Create a hidden dir and vendor that should be skipped.
	_ = os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".hidden", "skip.go"), []byte(`package skip
func Skip() {}
`), 0o644)

	_ = os.MkdirAll(filepath.Join(dir, "vendor"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "vendor", "dep.go"), []byte(`package dep
func Dep() {}
`), 0o644)

	return dir
}

func TestAnalyzerWalksProject(t *testing.T) {
	dir := setupTestProject(t)
	a := NewAnalyzer(dir)

	result, err := a.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.Summary.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2 (should skip .hidden and vendor)", result.Summary.TotalFiles)
	}
}

func TestAnalyzerSymbolCount(t *testing.T) {
	dir := setupTestProject(t)
	a := NewAnalyzer(dir)

	result, err := a.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// pkg1: Server (struct), New (func), Start (method) = 3
	// pkg2: Version (const), Add (func) = 2
	if result.Summary.TotalSymbols != 5 {
		t.Errorf("TotalSymbols = %d, want 5", result.Summary.TotalSymbols)
	}
}

func TestAnalyzerByKind(t *testing.T) {
	dir := setupTestProject(t)
	a := NewAnalyzer(dir)

	result, err := a.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	tests := []struct {
		kind string
		want int
	}{
		{"struct", 1},
		{"function", 2},
		{"method", 1},
		{"constant", 1},
	}

	for _, tt := range tests {
		if got := result.Summary.ByKind[tt.kind]; got != tt.want {
			t.Errorf("ByKind[%s] = %d, want %d", tt.kind, got, tt.want)
		}
	}
}

func TestAnalyzerPackages(t *testing.T) {
	dir := setupTestProject(t)
	a := NewAnalyzer(dir)

	result, err := a.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.Packages["pkg1"] != 3 {
		t.Errorf("pkg1 symbols = %d, want 3", result.Packages["pkg1"])
	}

	if result.Packages["pkg2"] != 2 {
		t.Errorf("pkg2 symbols = %d, want 2", result.Packages["pkg2"])
	}
}

func TestFindSymbolsByName(t *testing.T) {
	dir := setupTestProject(t)
	a := NewAnalyzer(dir)

	result, err := a.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	matches := FindSymbols(result, "New", "")
	if len(matches) != 1 {
		t.Fatalf("FindSymbols('New') = %d results, want 1", len(matches))
	}

	if matches[0].Name != "New" {
		t.Errorf("match name = %q, want %q", matches[0].Name, "New")
	}
}

func TestFindSymbolsByKind(t *testing.T) {
	dir := setupTestProject(t)
	a := NewAnalyzer(dir)

	result, err := a.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	matches := FindSymbols(result, "", KindMethod)
	if len(matches) != 1 {
		t.Fatalf("FindSymbols(kind=method) = %d results, want 1", len(matches))
	}

	if matches[0].Name != "Start" {
		t.Errorf("match name = %q, want %q", matches[0].Name, "Start")
	}
}

func TestSymbolsInPackage(t *testing.T) {
	dir := setupTestProject(t)
	a := NewAnalyzer(dir)

	result, err := a.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	symbols := SymbolsInPackage(result, "pkg2")
	if len(symbols) != 2 {
		t.Errorf("pkg2 symbols = %d, want 2", len(symbols))
	}
}

func TestAnalyzeFileUnsupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "readme.txt")
	_ = os.WriteFile(path, []byte("hello"), 0o644)

	a := NewAnalyzer(dir)

	_, err := a.AnalyzeFile(path)
	if err == nil {
		t.Error("expected error for unsupported file type")
	}
}

func TestAnalyzeFileGoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	_ = os.WriteFile(path, []byte(`package main
func Hello() {}
`), 0o644)

	a := NewAnalyzer(dir)

	fr, err := a.AnalyzeFile(path)
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}

	if fr.Package != "main" {
		t.Errorf("Package = %q, want %q", fr.Package, "main")
	}
}

func TestAnalyzePathsWithFiles(t *testing.T) {
	dir := setupTestProject(t)
	a := NewAnalyzer(dir)

	result, err := a.AnalyzePaths([]string{
		filepath.Join("pkg1", "main.go"),
		filepath.Join("pkg2", "util.go"),
	})
	if err != nil {
		t.Fatalf("AnalyzePaths: %v", err)
	}

	if result.Summary.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result.Summary.TotalFiles)
	}

	if result.Summary.TotalSymbols != 5 {
		t.Errorf("TotalSymbols = %d, want 5", result.Summary.TotalSymbols)
	}
}

func TestAnalyzePathsWithDirectory(t *testing.T) {
	dir := setupTestProject(t)
	a := NewAnalyzer(dir)

	result, err := a.AnalyzePaths([]string{"pkg1"})
	if err != nil {
		t.Fatalf("AnalyzePaths: %v", err)
	}

	if result.Summary.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", result.Summary.TotalFiles)
	}
}

func TestAnalyzePathsAbsolutePath(t *testing.T) {
	dir := setupTestProject(t)
	a := NewAnalyzer(dir)

	absPath := filepath.Join(dir, "pkg2", "util.go")

	result, err := a.AnalyzePaths([]string{absPath})
	if err != nil {
		t.Fatalf("AnalyzePaths: %v", err)
	}

	if result.Summary.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", result.Summary.TotalFiles)
	}
}

func TestAnalyzePathsMissingFile(t *testing.T) {
	dir := setupTestProject(t)
	a := NewAnalyzer(dir)

	result, err := a.AnalyzePaths([]string{"nonexistent.go"})
	if err != nil {
		t.Fatalf("AnalyzePaths: %v", err)
	}

	if result.Summary.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0 for missing file", result.Summary.TotalFiles)
	}
}

func TestAnalyzePathsUnsupportedFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0o644)

	a := NewAnalyzer(dir)

	result, err := a.AnalyzePaths([]string{filepath.Join(dir, "readme.txt")})
	if err != nil {
		t.Fatalf("AnalyzePaths: %v", err)
	}

	if result.Summary.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0 for unsupported file", result.Summary.TotalFiles)
	}
}

func TestAnalyzePathsInvalidGoFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "bad.go"), []byte("not valid go code!!!"), 0o644)

	a := NewAnalyzer(dir)

	result, err := a.AnalyzePaths([]string{filepath.Join(dir, "bad.go")})
	if err != nil {
		t.Fatalf("AnalyzePaths: %v", err)
	}

	// Should skip unparseable files.
	if result.Summary.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0 for invalid file", result.Summary.TotalFiles)
	}
}

func TestParseFileAllLanguages(t *testing.T) {
	dir := t.TempDir()
	a := NewAnalyzer(dir)

	// Test proto file parsing via parseFile.
	protoPath := filepath.Join(dir, "test.proto")
	_ = os.WriteFile(protoPath, []byte(`syntax = "proto3";
package testpkg;
message Foo { string name = 1; }
`), 0o644)

	fr, err := a.parseFile(protoPath, LangProto)
	if err != nil {
		t.Fatalf("parseFile proto: %v", err)
	}

	if fr.Language != LangProto {
		t.Errorf("Language = %q, want %q", fr.Language, LangProto)
	}

	// Test TypeScript file parsing via parseFile.
	tsPath := filepath.Join(dir, "test.ts")
	_ = os.WriteFile(tsPath, []byte(`export function greet(name: string): string { return name; }
`), 0o644)

	fr, err = a.parseFile(tsPath, LangTypeScript)
	if err != nil {
		t.Fatalf("parseFile ts: %v", err)
	}

	if fr.Language != LangTypeScript {
		t.Errorf("Language = %q, want %q", fr.Language, LangTypeScript)
	}

	// Test Python file parsing via parseFile.
	pyPath := filepath.Join(dir, "test.py")
	_ = os.WriteFile(pyPath, []byte(`def hello():
    pass
`), 0o644)

	fr, err = a.parseFile(pyPath, LangPython)
	if err != nil {
		t.Fatalf("parseFile python: %v", err)
	}

	if fr.Language != LangPython {
		t.Errorf("Language = %q, want %q", fr.Language, LangPython)
	}

	// Test Rust file parsing via parseFile.
	rsPath := filepath.Join(dir, "test.rs")
	_ = os.WriteFile(rsPath, []byte(`pub fn greet() -> String { String::new() }
`), 0o644)

	fr, err = a.parseFile(rsPath, LangRust)
	if err != nil {
		t.Fatalf("parseFile rust: %v", err)
	}

	if fr.Language != LangRust {
		t.Errorf("Language = %q, want %q", fr.Language, LangRust)
	}

	// Test unsupported language.
	_, err = a.parseFile("test.xyz", Language("xyz"))
	if err == nil {
		t.Error("expected error for unsupported language")
	}
}

// --- Summary tests ---

func TestFileSummaryEmpty(t *testing.T) {
	fr := &FileResult{
		Path:     "empty.go",
		Language: LangGo,
		Package:  "main",
	}

	got := FileSummary(fr)
	if got != "empty.go: empty go file in package main" {
		t.Errorf("FileSummary empty = %q", got)
	}
}

func TestFileSummaryWithSymbols(t *testing.T) {
	fr := &FileResult{
		Path:     "server.go",
		Language: LangGo,
		Package:  "app",
		Symbols: []Symbol{
			{Name: "Server", Kind: KindStruct, Exported: true},
			{Name: "Handler", Kind: KindInterface, Exported: true},
			{Name: "NewServer", Kind: KindFunction, Exported: true},
			{Name: "Start", Kind: KindMethod, Receiver: "Server", Exported: true},
		},
	}

	got := FileSummary(fr)
	if got == "" {
		t.Error("expected non-empty summary")
	}

	// Should contain types, funcs, methods.
	for _, want := range []string{"Server", "Handler", "NewServer", "Server.Start"} {
		if !contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
}

func TestFileSummaryUnexportedOnly(t *testing.T) {
	fr := &FileResult{
		Path:     "internal.go",
		Language: LangGo,
		Package:  "internal",
		Symbols: []Symbol{
			{Name: "helper", Kind: KindFunction, Exported: false},
			{Name: "config", Kind: KindStruct, Exported: false},
		},
	}

	got := FileSummary(fr)
	if !contains(got, "2 unexported symbols") {
		t.Errorf("expected unexported summary, got %q", got)
	}
}

func TestGenerateFileSummaries(t *testing.T) {
	result := &AnalysisResult{
		Files: []FileResult{
			{
				Path:     "main.go",
				Language: LangGo,
				Package:  "main",
				Symbols: []Symbol{
					{Name: "Main", Kind: KindFunction, Exported: true},
				},
			},
			{
				Path:     "empty.go",
				Language: LangGo,
				Package:  "main",
			},
		},
	}

	got := GenerateFileSummaries(result)
	if !contains(got, "# File Summaries") {
		t.Error("missing header")
	}

	if !contains(got, "main.go") {
		t.Error("missing main.go entry")
	}

	if !contains(got, "empty.go") {
		t.Error("missing empty.go entry")
	}
}

func TestGenerateFileSummariesEmpty(t *testing.T) {
	result := &AnalysisResult{}

	got := GenerateFileSummaries(result)
	if got != "No files analyzed." {
		t.Errorf("expected 'No files analyzed.', got %q", got)
	}
}

func TestGenerateDepGraphMarkdown(t *testing.T) {
	result := &AnalysisResult{
		DepGraph: map[string][]string{
			"server": {"fmt", "net/http"},
		},
	}

	got := GenerateDepGraphMarkdown(result)
	if !contains(got, "# Package Dependencies") {
		t.Error("missing header")
	}

	if !contains(got, "server") {
		t.Error("missing server package")
	}

	if !contains(got, "fmt") {
		t.Error("missing fmt dep")
	}
}

func TestGenerateDepGraphMarkdownEmpty(t *testing.T) {
	result := &AnalysisResult{}

	got := GenerateDepGraphMarkdown(result)
	if got != "" {
		t.Errorf("expected empty string for no deps, got %q", got)
	}
}

func TestGenerateMermaidGraph(t *testing.T) {
	result := &AnalysisResult{
		Packages: map[string]int{
			"server": 5,
			"client": 3,
		},
		DepGraph: map[string][]string{
			"server": {"github.com/example/mod/client"},
		},
	}

	got := GenerateMermaidGraph(result, "github.com/example/mod/")
	if !contains(got, "```mermaid") {
		t.Error("missing mermaid header")
	}

	if !contains(got, "graph TD") {
		t.Error("missing graph TD")
	}

	if !contains(got, "server") {
		t.Error("missing server node")
	}

	if !contains(got, "client") {
		t.Error("missing client node")
	}
}

func TestGenerateMermaidGraphEmpty(t *testing.T) {
	result := &AnalysisResult{}

	got := GenerateMermaidGraph(result, "")
	if got != "" {
		t.Errorf("expected empty string for no deps, got %q", got)
	}
}

func TestGenerateMermaidGraphFiltersExternal(t *testing.T) {
	result := &AnalysisResult{
		Packages: map[string]int{
			"server": 5,
		},
		DepGraph: map[string][]string{
			"server": {"fmt", "net/http", "external.com/pkg"},
		},
	}

	got := GenerateMermaidGraph(result, "github.com/example/mod/")
	// Should not contain external packages that don't match module prefix.
	if contains(got, "external.com") {
		t.Error("should not include external packages")
	}
}

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"server", "server"},
		{"internal/server", "internal_server"},
		{"my.pkg", "my_pkg"},
		{"my-app/server", "my_app_server"},
	}

	for _, tt := range tests {
		got := sanitizeMermaidID(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeMermaidID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractPackageName(t *testing.T) {
	tests := []struct {
		importPath   string
		modulePrefix string
		want         string
	}{
		{"github.com/example/mod/server", "github.com/example/mod/", "server"},
		{"github.com/example/mod/internal/db", "github.com/example/mod/", "internal/db"},
		{"fmt", "", "fmt"},
		{"net/http", "", "http"},
		{"github.com/example/mod", "github.com/example/mod", "mod"},
	}

	for _, tt := range tests {
		got := extractPackageName(tt.importPath, tt.modulePrefix)
		if got != tt.want {
			t.Errorf("extractPackageName(%q, %q) = %q, want %q", tt.importPath, tt.modulePrefix, got, tt.want)
		}
	}
}

func TestJoinLimit(t *testing.T) {
	tests := []struct {
		items []string
		limit int
		want  string
	}{
		{[]string{"a", "b", "c"}, 5, "a, b, c"},
		{[]string{"a", "b", "c"}, 3, "a, b, c"},
		{[]string{"a", "b", "c", "d", "e", "f"}, 3, "a, b, c (+3 more)"},
		{[]string{"x"}, 1, "x"},
	}

	for _, tt := range tests {
		got := joinLimit(tt.items, tt.limit)
		if got != tt.want {
			t.Errorf("joinLimit(%v, %d) = %q, want %q", tt.items, tt.limit, got, tt.want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && containsStr(s, sub)
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want Language
	}{
		{"main.go", LangGo},
		{"app.ts", LangTypeScript},
		{"app.tsx", LangTypeScript},
		{"script.py", LangPython},
		{"lib.rs", LangRust},
		{"readme.md", ""},
		{"Makefile", ""},
	}

	for _, tt := range tests {
		got := detectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
