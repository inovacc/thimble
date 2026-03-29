package analysis

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/inovacc/thimble/internal/paths"
)

// AnalyzerOption configures optional Analyzer behavior.
type AnalyzerOption func(*Analyzer)

// WithLSP registers LSP client configurations for specific languages.
// When an LSP client is available for a language, parseFile will prefer it
// over the built-in regex parser, falling back on error.
func WithLSP(configs []LSPConfig) AnalyzerOption {
	return func(a *Analyzer) {
		for i := range configs {
			cfg := configs[i]
			for _, lang := range cfg.Languages {
				a.lspConfigs[lang] = cfg
			}
		}
	}
}

// Analyzer performs codebase analysis, extracting symbols and references
// from source files and aggregating results.
type Analyzer struct {
	root       string                  // Project root directory.
	lspConfigs map[Language]LSPConfig   // Opt-in LSP configs per language.
	lspClients map[Language]LSPClient   // Initialized LSP clients (lazy).
	lspMu      sync.Mutex              // Protects lspClients map.
}

// NewAnalyzer creates an analyzer rooted at the given directory.
// Optional AnalyzerOption values configure LSP integration and other features.
func NewAnalyzer(root string, opts ...AnalyzerOption) *Analyzer {
	a := &Analyzer{
		root:       root,
		lspConfigs: make(map[Language]LSPConfig),
		lspClients: make(map[Language]LSPClient),
	}

	// Auto-register LSP configs from environment variables.
	for _, lang := range []Language{LangGo} {
		if lspEnabled(lang) {
			if cfg := defaultLSPConfig(lang); cfg != nil {
				a.lspConfigs[lang] = *cfg
			}
		}
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Analyze walks the project tree and parses all supported source files.
// It returns an aggregated result with all symbols, references, and stats.
func (a *Analyzer) Analyze() (*AnalysisResult, error) {
	result := newEmptyResult()

	err := filepath.WalkDir(a.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip walk errors, continue traversal
		}

		// Skip hidden dirs, vendor, node_modules, testdata.
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "testdata" {
				return filepath.SkipDir
			}

			return nil
		}

		lang := detectLanguage(path)
		if lang == "" {
			return nil
		}

		fr, err := a.parseFile(path, lang)
		if err != nil {
			return nil //nolint:nilerr // skip unparseable files, continue traversal
		}

		result.Files = append(result.Files, *fr)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", a.root, err)
	}

	aggregateStats(result)

	return result, nil
}

// CachePath returns the default analysis cache file path for this analyzer's
// project root, stored under {DataDir}/projects/{digest}/analysis_cache.json.
func (a *Analyzer) CachePath() string {
	return filepath.Join(paths.ProjectDataDir(a.root), "analysis_cache.json")
}

// AnalyzeIncremental performs a diff-aware incremental analysis.
// It loads the on-disk cache, determines which files actually changed
// (via git diff HEAD~1 + file mod-time comparison), re-parses only those
// files, merges with cached results, and persists the updated cache.
//
// Falls back to full analysis when git is unavailable.
func (a *Analyzer) AnalyzeIncremental() (*AnalysisResult, error) {
	return a.AnalyzeIncrementalWithCache(a.CachePath())
}

// AnalyzeIncrementalWithCache is like AnalyzeIncremental but accepts an
// explicit cache path, which is useful for testing.
func (a *Analyzer) AnalyzeIncrementalWithCache(cachePath string) (*AnalysisResult, error) {
	cache, err := LoadCache(cachePath)
	if err != nil {
		cache = NewAnalysisCache()
	}

	// Collect git-changed files (HEAD~1 diff + working-tree changes).
	gitChanged := make(map[string]bool)

	if diffFiles, err := gitDiffHead1(a.root); err == nil {
		for _, f := range diffFiles {
			gitChanged[f] = true
		}
	}

	if wtFiles, err := gitChangedFiles(a.root); err == nil {
		for _, f := range wtFiles {
			gitChanged[f] = true
		}
	}

	// If git is entirely unavailable (both calls returned no data and no
	// cache entries exist), fall back to full analysis.
	if len(gitChanged) == 0 && len(cache.Entries) == 0 {
		return a.Analyze()
	}

	result := newEmptyResult()

	// Walk the project to discover all supported files.
	allFiles := make(map[string]os.FileInfo) // relPath → FileInfo

	_ = filepath.WalkDir(a.root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr
		}

		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "testdata" {
				return filepath.SkipDir
			}

			return nil
		}

		lang := detectLanguage(path)
		if lang == "" {
			return nil
		}

		rel, relErr := filepath.Rel(a.root, path)
		if relErr != nil {
			return nil //nolint:nilerr
		}

		info, statErr := d.Info()
		if statErr != nil {
			return nil //nolint:nilerr
		}

		allFiles[filepath.ToSlash(rel)] = info

		return nil
	})

	// Determine which files need re-parsing.
	for rel, info := range allFiles {
		needsParse := false

		if gitChanged[rel] {
			needsParse = true
		} else if entry, ok := cache.Get(rel); ok {
			if !info.ModTime().Equal(entry.ModTime) {
				needsParse = true
			}
		} else {
			// New file not in cache.
			needsParse = true
		}

		if needsParse {
			fullPath := filepath.Join(a.root, filepath.FromSlash(rel))
			lang := detectLanguage(fullPath)

			fr, parseErr := a.parseFile(fullPath, lang)
			if parseErr != nil {
				continue
			}

			cache.Set(rel, info.ModTime(), *fr)
			result.Files = append(result.Files, *fr)
		} else {
			// Use cached result.
			entry, _ := cache.Get(rel)
			result.Files = append(result.Files, entry.Result)
		}
	}

	// Prune cache entries for files that no longer exist.
	for rel := range cache.Entries {
		if _, exists := allFiles[rel]; !exists {
			cache.Remove(rel)
		}
	}

	// Persist the updated cache (best-effort).
	_ = cache.SaveCache(cachePath)

	aggregateStats(result)

	return result, nil
}

// AnalyzePaths analyzes only the specified files/directories.
func (a *Analyzer) AnalyzePaths(paths []string) (*AnalysisResult, error) {
	result := newEmptyResult()

	for _, p := range paths {
		fullPath := p
		if !filepath.IsAbs(p) {
			fullPath = filepath.Join(a.root, p)
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			continue // skip missing files
		}

		if info.IsDir() {
			// Walk the directory.
			_ = filepath.WalkDir(fullPath, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil //nolint:nilerr // skip walk errors
				}

				lang := detectLanguage(path)
				if lang == "" {
					return nil
				}

				fr, err := a.parseFile(path, lang)
				if err != nil {
					return nil //nolint:nilerr // skip unparseable files
				}

				result.Files = append(result.Files, *fr)

				return nil
			})
		} else {
			lang := detectLanguage(fullPath)
			if lang == "" {
				continue
			}

			fr, err := a.parseFile(fullPath, lang)
			if err != nil {
				continue
			}

			result.Files = append(result.Files, *fr)
		}
	}

	aggregateStats(result)

	return result, nil
}

// newEmptyResult creates an initialized empty AnalysisResult.
func newEmptyResult() *AnalysisResult {
	return &AnalysisResult{
		Packages: make(map[string]int),
		DepGraph: make(map[string][]string),
		Summary: AnalysisSummary{
			ByKind:     make(map[string]int),
			ByLanguage: make(map[string]int),
		},
	}
}

// aggregateStats populates summary stats and dependency graph from file results.
func aggregateStats(result *AnalysisResult) {
	for _, f := range result.Files {
		result.Summary.TotalFiles++
		result.Summary.ByLanguage[string(f.Language)]++

		for _, sym := range f.Symbols {
			result.Summary.TotalSymbols++
			result.Summary.ByKind[string(sym.Kind)]++
			result.Packages[sym.Package]++
		}

		result.Summary.TotalReferences += len(f.References)

		// Build dependency graph from imports.
		if f.Package != "" && len(f.Imports) > 0 {
			existing := result.DepGraph[f.Package]

			seen := make(map[string]bool, len(existing))
			for _, imp := range existing {
				seen[imp] = true
			}

			for _, imp := range f.Imports {
				if !seen[imp] {
					result.DepGraph[f.Package] = append(result.DepGraph[f.Package], imp)
					seen[imp] = true
				}
			}
		}
	}
}

// gitDiffHead1 returns files changed between HEAD~1 and HEAD (the last commit).
func gitDiffHead1(root string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD~1")
	cmd.Dir = root

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff HEAD~1: %w", err)
	}

	var files []string

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		f := strings.TrimSpace(scanner.Text())
		if f != "" {
			files = append(files, f)
		}
	}

	return files, nil
}

// gitChangedFiles returns files changed in the working tree relative to HEAD.
func gitChangedFiles(root string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD")
	cmd.Dir = root

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	// Also include untracked files.
	untrackedCmd := exec.CommandContext(ctx, "git", "ls-files", "--others", "--exclude-standard")
	untrackedCmd.Dir = root
	untrackedOut, _ := untrackedCmd.Output()

	seen := make(map[string]bool)

	var files []string

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		f := strings.TrimSpace(scanner.Text())
		if f != "" && !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}

	scanner = bufio.NewScanner(strings.NewReader(string(untrackedOut)))
	for scanner.Scan() {
		f := strings.TrimSpace(scanner.Text())
		if f != "" && !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}

	return files, nil
}

// AnalyzeFile parses a single file and returns the result.
func (a *Analyzer) AnalyzeFile(path string) (*FileResult, error) {
	lang := detectLanguage(path)
	if lang == "" {
		return nil, fmt.Errorf("unsupported language for %s", path)
	}

	return a.parseFile(path, lang)
}

// FindSymbols searches for symbols matching the given query across all analyzed files.
func FindSymbols(result *AnalysisResult, query string, kind SymbolKind) []Symbol {
	query = strings.ToLower(query)

	var matches []Symbol

	for _, f := range result.Files {
		for _, sym := range f.Symbols {
			if kind != "" && sym.Kind != kind {
				continue
			}

			if strings.Contains(strings.ToLower(sym.Name), query) {
				matches = append(matches, sym)
			}
		}
	}

	return matches
}

// SymbolsInPackage returns all symbols in the given package.
func SymbolsInPackage(result *AnalysisResult, pkg string) []Symbol {
	var symbols []Symbol

	for _, f := range result.Files {
		if f.Package != pkg {
			continue
		}

		symbols = append(symbols, f.Symbols...)
	}

	return symbols
}

func (a *Analyzer) parseFile(path string, lang Language) (*FileResult, error) {
	// Try LSP first if configured for this language.
	if _, ok := a.lspConfigs[lang]; ok {
		if fr, err := a.parseFileWithLSP(path, lang); err == nil {
			return fr, nil
		}
		// LSP failed; fall back to regex parser.
	}

	return a.parseFileRegex(path, lang)
}

// parseFileWithLSP uses an LSP client to extract symbols from the file.
func (a *Analyzer) parseFileWithLSP(path string, lang Language) (*FileResult, error) {
	client, err := a.getLSPClient(lang)
	if err != nil {
		return nil, err
	}

	symbols, err := client.Symbols(path)
	if err != nil {
		return nil, err
	}

	fr := &FileResult{
		Path:     path,
		Language: lang,
		Symbols:  symbols,
	}

	// Detect cross-language invocations.
	code, readErr := os.ReadFile(path)
	if readErr == nil {
		crossRefs := ExtractCrossLangRefs(path, lang, string(code))
		fr.References = append(fr.References, crossRefs...)
	}

	return fr, nil
}

// getLSPClient returns a cached or newly initialized LSP client for the given language.
func (a *Analyzer) getLSPClient(lang Language) (LSPClient, error) {
	a.lspMu.Lock()
	defer a.lspMu.Unlock()

	if client, ok := a.lspClients[lang]; ok {
		return client, nil
	}

	cfg, ok := a.lspConfigs[lang]
	if !ok {
		return nil, fmt.Errorf("no LSP config for %s", lang)
	}

	client, err := NewLSPClient(cfg)
	if err != nil {
		return nil, err
	}

	if err := client.Initialize(a.root); err != nil {
		return nil, err
	}

	a.lspClients[lang] = client

	return client, nil
}

// ShutdownLSP shuts down all active LSP clients. Should be called when analysis is complete.
func (a *Analyzer) ShutdownLSP() {
	a.lspMu.Lock()
	defer a.lspMu.Unlock()

	for lang, client := range a.lspClients {
		_ = client.Shutdown()

		delete(a.lspClients, lang)
	}
}

// parseFileRegex uses the built-in regex-based parsers.
func (a *Analyzer) parseFileRegex(path string, lang Language) (*FileResult, error) {
	var fr *FileResult

	var err error

	switch lang {
	case LangGo:
		fr, err = ParseGoFile(path)
	case LangProto:
		fr, err = ParseProtoFile(path)
	case LangTypeScript:
		fr, err = ParseTSFile(path)
	case LangPython:
		fr, err = ParsePythonFile(path)
	case LangRust:
		fr, err = ParseRustFile(path)
	case LangShell:
		fr, err = ParseShellFile(path)
	case LangC:
		fr, err = ParseCFile(path)
	case LangJava:
		fr, err = ParseJavaFile(path)
	default:
		return nil, fmt.Errorf("parser not implemented for %s", lang)
	}

	if err != nil {
		return nil, err
	}

	// Detect cross-language invocations.
	code, readErr := os.ReadFile(path)
	if readErr == nil {
		crossRefs := ExtractCrossLangRefs(path, lang, string(code))
		fr.References = append(fr.References, crossRefs...)
	}

	return fr, nil
}

func detectLanguage(path string) Language {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return LangGo
	case ".ts", ".tsx", ".js", ".jsx", ".mjs":
		return LangTypeScript
	case ".py":
		return LangPython
	case ".rs":
		return LangRust
	case ".proto":
		return LangProto
	case ".sh", ".bash":
		return LangShell
	case ".c", ".h", ".cpp", ".hpp", ".cc", ".cxx":
		return LangC
	case ".java", ".kt", ".kts":
		return LangJava
	default:
		return ""
	}
}
