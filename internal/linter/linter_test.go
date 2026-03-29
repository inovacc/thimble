package linter

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/thimble/internal/analysis"
)

func TestParseLintIssues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []Issue
	}{
		{
			name:  "standard issue",
			input: "file.go:10:5: error message here (errcheck)",
			expected: []Issue{
				{File: "file.go", Line: 10, Column: 5, Message: "error message here", Linter: "errcheck", Severity: "error"},
			},
		},
		{
			name: "multiple issues",
			input: "main.go:1:1: missing package doc (stylecheck)\n" +
				"utils.go:42:12: unused variable (deadcode)",
			expected: []Issue{
				{File: "main.go", Line: 1, Column: 1, Message: "missing package doc", Linter: "stylecheck", Severity: "error"},
				{File: "utils.go", Line: 42, Column: 12, Message: "unused variable", Linter: "deadcode", Severity: "error"},
			},
		},
		{
			name:     "empty output",
			input:    "",
			expected: nil,
		},
		{
			name:     "malformed lines skipped",
			input:    "this is not a lint issue\nRUNNING linter...\nfile.go:10:5: valid issue (govet)\nalso not valid",
			expected: []Issue{{File: "file.go", Line: 10, Column: 5, Message: "valid issue", Linter: "govet", Severity: "error"}},
		},
		{
			name:     "line without linter name in parens",
			input:    "file.go:10:5: error message without linter",
			expected: nil,
		},
		{
			name:  "path with directory",
			input: "internal/pkg/server.go:100:3: something wrong (gosimple)",
			expected: []Issue{
				{File: "internal/pkg/server.go", Line: 100, Column: 3, Message: "something wrong", Linter: "gosimple", Severity: "error"},
			},
		},
		{
			name:  "hyphenated linter name",
			input: "file.go:5:1: some lint finding (my-linter)",
			expected: []Issue{
				{File: "file.go", Line: 5, Column: 1, Message: "some lint finding", Linter: "my-linter", Severity: "error"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseLintIssues(tt.input)

			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d issues, got %d: %+v", len(tt.expected), len(got), got)
			}

			for i, want := range tt.expected {
				g := got[i]
				if g.File != want.File {
					t.Errorf("[%d] File: got %q, want %q", i, g.File, want.File)
				}

				if g.Line != want.Line {
					t.Errorf("[%d] Line: got %d, want %d", i, g.Line, want.Line)
				}

				if g.Column != want.Column {
					t.Errorf("[%d] Column: got %d, want %d", i, g.Column, want.Column)
				}

				if g.Message != want.Message {
					t.Errorf("[%d] Message: got %q, want %q", i, g.Message, want.Message)
				}

				if g.Linter != want.Linter {
					t.Errorf("[%d] Linter: got %q, want %q", i, g.Linter, want.Linter)
				}

				if g.Severity != want.Severity {
					t.Errorf("[%d] Severity: got %q, want %q", i, g.Severity, want.Severity)
				}
			}
		})
	}
}

func TestFindEnclosingSymbol(t *testing.T) {
	t.Parallel()

	symbols := []analysis.Symbol{
		{Name: "init", Kind: analysis.KindFunction, Line: 5},
		{Name: "Server", Kind: analysis.KindStruct, Line: 15},
		{Name: "Start", Kind: analysis.KindMethod, Receiver: "Server", Line: 25},
		{Name: "helper", Kind: analysis.KindFunction, Line: 50},
		{Name: "MaxRetries", Kind: analysis.KindConstant, Line: 3},  // should be skipped
		{Name: "globalVar", Kind: analysis.KindVariable, Line: 10},  // should be skipped
		{Name: "main", Kind: analysis.KindPackage, Line: 1},         // should be skipped
	}

	tests := []struct {
		name     string
		line     int
		expected string
	}{
		{
			name:     "inside init function",
			line:     10,
			expected: "init",
		},
		{
			name:     "inside struct declaration",
			line:     20,
			expected: "Server",
		},
		{
			name:     "inside method with receiver",
			line:     30,
			expected: "Server.Start",
		},
		{
			name:     "inside helper function",
			line:     55,
			expected: "helper",
		},
		{
			name:     "before any function",
			line:     1,
			expected: "",
		},
		{
			name:     "exact line of function",
			line:     5,
			expected: "init",
		},
		{
			name:     "exact line of method",
			line:     25,
			expected: "Server.Start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := findEnclosingSymbol(symbols, tt.line)
			if got != tt.expected {
				t.Errorf("findEnclosingSymbol(symbols, %d) = %q, want %q", tt.line, got, tt.expected)
			}
		})
	}
}

func TestFindEnclosingSymbol_Empty(t *testing.T) {
	t.Parallel()

	got := findEnclosingSymbol(nil, 10)
	if got != "" {
		t.Errorf("expected empty string for nil symbols, got %q", got)
	}
}

func TestRun_MissingBinary(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("golangci-lint"); err == nil {
		t.Skip("golangci-lint is installed; cannot test missing-binary path")
	}

	res, err := Run(context.Background(), t.TempDir(), nil, nil, false, false, 10)
	if err != nil {
		t.Fatalf("expected nil error for missing golangci-lint, got: %v", err)
	}

	if res.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", res.ExitCode)
	}

	if res.RawOutput == "" {
		t.Error("expected non-empty RawOutput with install instructions")
	}
}

func TestRun_ContextCanceled(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not installed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	res, err := Run(ctx, t.TempDir(), nil, nil, false, false, 5)
	if err != nil {
		t.Fatalf("expected nil error (errors returned in Result), got: %v", err)
	}

	// With a canceled context, the process should fail
	if res.Success {
		t.Error("expected Success=false for canceled context")
	}
}

func TestRun_DefaultTimeout(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not installed")
	}

	// Pass timeout <= 0 to trigger the default (300s)
	res, err := Run(context.Background(), t.TempDir(), nil, nil, false, false, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The command will fail (no Go files in temp dir) but should still return a Result
	if res == nil {
		t.Fatal("expected non-nil result")
	}

	if strings.Contains(res.RawOutput, "not found") {
		t.Skip("golangci-lint not actually callable")
	}
}

func TestRun_WithPaths(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not installed")
	}

	res, err := Run(context.Background(), t.TempDir(), []string{"./cmd/..."}, nil, false, false, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestRun_WithLinters(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not installed")
	}

	res, err := Run(context.Background(), t.TempDir(), nil, []string{"errcheck", "govet"}, false, false, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestRun_FastAndFix(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not installed")
	}

	res, err := Run(context.Background(), t.TempDir(), nil, nil, true, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestRun_EmptyDir(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not installed")
	}

	// Run in empty temp dir — golangci-lint should fail but not panic
	res, err := Run(context.Background(), t.TempDir(), nil, nil, false, false, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestRun_EmptyDirFallsBackToGetwd(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not installed")
	}

	// Pass empty string for dir — should fallback to os.Getwd()
	res, err := Run(context.Background(), "", nil, nil, false, false, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestParseLintIssues_WhitespaceHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "leading whitespace",
			input:    "  file.go:10:5: error message (errcheck)  ",
			expected: 1,
		},
		{
			name:     "trailing newlines",
			input:    "file.go:10:5: error message (errcheck)\n\n\n",
			expected: 1,
		},
		{
			name:     "carriage returns",
			input:    "file.go:10:5: error message (errcheck)\r\n",
			expected: 1, // TrimSpace strips \r so the line still matches
		},
		{
			name:     "only whitespace",
			input:    "   \n   \n   ",
			expected: 0,
		},
		{
			name:     "tabs in non-matching lines",
			input:    "\t\tfile.go:10:5: error message (errcheck)",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseLintIssues(tt.input)
			if len(got) != tt.expected {
				t.Errorf("expected %d issues, got %d: %+v", tt.expected, len(got), got)
			}
		})
	}
}

func TestParseLintIssues_WindowsPath(t *testing.T) {
	t.Parallel()

	// The regex uses ^(.+?): which should match Windows paths like C:\foo\bar.go
	// but they typically get normalized. Test the general case.
	input := "some/deep/path/to/file.go:99:1: unused (deadcode)"
	issues := parseLintIssues(input)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}

	if issues[0].File != "some/deep/path/to/file.go" {
		t.Errorf("File = %q, want %q", issues[0].File, "some/deep/path/to/file.go")
	}

	if issues[0].Line != 99 {
		t.Errorf("Line = %d, want 99", issues[0].Line)
	}
}

func TestParseLintIssues_LargeLineAndColumn(t *testing.T) {
	t.Parallel()

	input := "file.go:999999:888: very long error message text here (staticcheck)"
	issues := parseLintIssues(input)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}

	if issues[0].Line != 999999 {
		t.Errorf("Line = %d, want 999999", issues[0].Line)
	}

	if issues[0].Column != 888 {
		t.Errorf("Column = %d, want 888", issues[0].Column)
	}
}

func TestEnrichIssuesWithSymbols_NoIssues(t *testing.T) {
	t.Parallel()

	// Should not panic with empty issues
	enrichIssuesWithSymbols(t.TempDir(), nil)
}

func TestEnrichIssuesWithSymbols_NonexistentFile(t *testing.T) {
	t.Parallel()

	issues := []Issue{
		{File: "nonexistent.go", Line: 10, Message: "test"},
	}

	// Should not panic when file doesn't exist
	enrichIssuesWithSymbols(t.TempDir(), issues)

	// SourceLine should remain empty since analysis fails
	if issues[0].SourceLine != "" {
		t.Errorf("expected empty SourceLine, got %q", issues[0].SourceLine)
	}
}

func TestEnrichIssuesWithSymbols_RealGoFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")

	content := `package main

func hello() {
	x := 1
	_ = x
}

func goodbye() {
	y := 2
	_ = y
}
`
	if err := os.WriteFile(goFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// enrichIssuesWithSymbols passes issue.File directly to the analyzer,
	// which reads from the filesystem. Use the absolute path so it resolves.
	absFile := filepath.Join(dir, "main.go")

	issues := []Issue{
		{File: absFile, Line: 4, Message: "unused"},
		{File: absFile, Line: 9, Message: "unused"},
	}

	enrichIssuesWithSymbols(dir, issues)

	if issues[0].SourceLine != "in hello" {
		t.Errorf("issues[0].SourceLine = %q, want %q", issues[0].SourceLine, "in hello")
	}

	if issues[1].SourceLine != "in goodbye" {
		t.Errorf("issues[1].SourceLine = %q, want %q", issues[1].SourceLine, "in goodbye")
	}
}

func TestFindEnclosingSymbol_InterfaceKind(t *testing.T) {
	t.Parallel()

	symbols := []analysis.Symbol{
		{Name: "Handler", Kind: analysis.KindInterface, Line: 10},
		{Name: "Serve", Kind: analysis.KindMethod, Receiver: "Handler", Line: 20},
	}

	got := findEnclosingSymbol(symbols, 15)
	if got != "Handler" {
		t.Errorf("expected %q, got %q", "Handler", got)
	}

	got = findEnclosingSymbol(symbols, 25)
	if got != "Handler.Serve" {
		t.Errorf("expected %q, got %q", "Handler.Serve", got)
	}
}

func TestFindEnclosingSymbol_TypeKind(t *testing.T) {
	t.Parallel()

	symbols := []analysis.Symbol{
		{Name: "Config", Kind: analysis.KindType, Line: 5},
		{Name: "Load", Kind: analysis.KindFunction, Line: 20},
	}

	got := findEnclosingSymbol(symbols, 10)
	if got != "Config" {
		t.Errorf("expected %q, got %q", "Config", got)
	}
}

func TestFindEnclosingSymbol_AllSkippableKinds(t *testing.T) {
	t.Parallel()

	symbols := []analysis.Symbol{
		{Name: "c1", Kind: analysis.KindConstant, Line: 1},
		{Name: "v1", Kind: analysis.KindVariable, Line: 5},
		{Name: "pkg", Kind: analysis.KindPackage, Line: 10},
	}

	got := findEnclosingSymbol(symbols, 15)
	if got != "" {
		t.Errorf("expected empty for all-skippable symbols, got %q", got)
	}
}

func TestResult_StructFields(t *testing.T) {
	t.Parallel()

	r := Result{
		Issues:      []Issue{{File: "a.go", Line: 1}},
		TotalIssues: 1,
		RawOutput:   "some output",
		Success:     false,
		ExitCode:    1,
	}

	if r.TotalIssues != 1 {
		t.Errorf("TotalIssues = %d, want 1", r.TotalIssues)
	}

	if r.Success {
		t.Error("expected Success=false")
	}

	if r.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", r.ExitCode)
	}

	if r.RawOutput != "some output" {
		t.Errorf("RawOutput = %q", r.RawOutput)
	}
}

func TestIssue_StructFields(t *testing.T) {
	t.Parallel()

	i := Issue{
		File:       "test.go",
		Line:       42,
		Column:     7,
		Message:    "unused var",
		Linter:     "deadcode",
		Severity:   "error",
		SourceLine: "in main",
	}

	if i.SourceLine != "in main" {
		t.Errorf("SourceLine = %q, want %q", i.SourceLine, "in main")
	}
}
