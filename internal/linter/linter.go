// Package linter wraps golangci-lint as a subprocess.
package linter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/inovacc/thimble/internal/analysis"
)

// Issue represents a single lint finding.
type Issue struct {
	File       string
	Line       int
	Column     int
	Message    string
	Linter     string
	Severity   string
	SourceLine string // e.g. "in FuncName"
}

// Result holds the output of a lint run.
type Result struct {
	Issues      []Issue
	TotalIssues int
	RawOutput   string
	Success     bool
	ExitCode    int
}

// Run executes golangci-lint in the given directory.
func Run(ctx context.Context, dir string, paths []string, linters []string, fast, fix bool, timeoutSeconds int) (*Result, error) {
	if dir == "" {
		dir, _ = os.Getwd()
	}

	args := []string{"run"}
	if fix {
		args = append(args, "--fix")
	}

	if fast {
		args = append(args, "--fast")
	}

	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}

	args = append(args, fmt.Sprintf("--timeout=%ds", timeoutSeconds))

	for _, l := range linters {
		args = append(args, "--enable", l)
	}

	if len(paths) > 0 {
		args = append(args, paths...)
	} else {
		args = append(args, "./...")
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "golangci-lint", args...)
	cmd.Dir = dir

	var stdout, stderr strings.Builder

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	execErr := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}

		output += stderr.String()
	}

	exitCode := 0
	if execErr != nil {
		exitCode = 1

		if errors.Is(execErr, exec.ErrNotFound) {
			return &Result{
				RawOutput: "golangci-lint not found. Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest",
				ExitCode:  1,
			}, nil
		}
	}

	issues := parseLintIssues(output)
	enrichIssuesWithSymbols(dir, issues)

	return &Result{
		Issues:      issues,
		TotalIssues: len(issues),
		RawOutput:   output,
		Success:     len(issues) == 0 && exitCode == 0,
		ExitCode:    exitCode,
	}, nil
}

var lintIssueRe = regexp.MustCompile(`^(.+?):(\d+):(\d+):\s+(.+?)\s+\((\w[\w-]*)\)\s*$`)

func parseLintIssues(output string) []Issue {
	var issues []Issue

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)

		m := lintIssueRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		lineNum, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		issues = append(issues, Issue{
			File:     m[1],
			Line:     lineNum,
			Column:   col,
			Message:  m[4],
			Linter:   m[5],
			Severity: "error",
		})
	}

	return issues
}

func enrichIssuesWithSymbols(projectDir string, issues []Issue) {
	if len(issues) == 0 {
		return
	}

	byFile := make(map[string][]int) // file -> indices into issues
	for i, issue := range issues {
		byFile[issue.File] = append(byFile[issue.File], i)
	}

	a := analysis.NewAnalyzer(projectDir)

	for file, indices := range byFile {
		fr, err := a.AnalyzeFile(file)
		if err != nil {
			continue
		}

		for _, idx := range indices {
			sym := findEnclosingSymbol(fr.Symbols, issues[idx].Line)
			if sym != "" {
				issues[idx].SourceLine = "in " + sym
			}
		}
	}
}

func findEnclosingSymbol(symbols []analysis.Symbol, line int) string {
	var bestName string

	bestLine := 0

	for _, sym := range symbols {
		switch sym.Kind {
		case analysis.KindFunction, analysis.KindMethod, analysis.KindType,
			analysis.KindStruct, analysis.KindInterface:
		case analysis.KindConstant, analysis.KindVariable, analysis.KindPackage:
			continue
		default:
			continue
		}

		if sym.Line <= line && sym.Line > bestLine {
			bestLine = sym.Line
			if sym.Receiver != "" {
				bestName = sym.Receiver + "." + sym.Name
			} else {
				bestName = sym.Name
			}
		}
	}

	return bestName
}
