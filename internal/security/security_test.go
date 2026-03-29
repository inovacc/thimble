package security

import (
	"testing"
)

func TestParseBashPattern(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Bash(sudo *)", "sudo *"},
		{"Bash(echo (foo))", "echo (foo)"},
		{"Read(.env)", ""},
		{"Bash(ls)", "ls"},
		{"NotBash(x)", ""},
	}
	for _, tt := range tests {
		got := ParseBashPattern(tt.input)
		if got != tt.want {
			t.Errorf("ParseBashPattern(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseToolPattern(t *testing.T) {
	tp := ParseToolPattern("Read(.env)")
	if tp == nil || tp.Tool != "Read" || tp.Glob != ".env" {
		t.Errorf("ParseToolPattern(Read(.env)) = %v", tp)
	}

	if ParseToolPattern("invalid") != nil {
		t.Error("expected nil for invalid pattern")
	}
}

func TestGlobToRegex(t *testing.T) {
	tests := []struct {
		glob    string
		input   string
		matches bool
	}{
		{"sudo *", "sudo rm -rf /", true},
		{"sudo *", "ls", false},
		{"ls:*", "ls", true},
		{"ls:*", "ls -la", true},
		{"tree:*", "tree --help", true},
		{"echo *", "echo hello world", true},
	}
	for _, tt := range tests {
		re := GlobToRegex(tt.glob, false)

		got := re.MatchString(tt.input)
		if got != tt.matches {
			t.Errorf("GlobToRegex(%q).Match(%q) = %v, want %v", tt.glob, tt.input, got, tt.matches)
		}
	}
}

func TestFileGlobToRegex(t *testing.T) {
	tests := []struct {
		glob    string
		path    string
		matches bool
	}{
		{"**/.env", "/home/user/project/.env", true},
		{"**/.env", "/home/user/.env", true},
		{"*.go", "main.go", true},
		{"*.go", "src/main.go", false},
		{"src/**/*.go", "src/pkg/main.go", true},
		{"src/**/*.go", "src/main.go", true},
	}
	for _, tt := range tests {
		re := FileGlobToRegex(tt.glob, false)

		got := re.MatchString(tt.path)
		if got != tt.matches {
			t.Errorf("FileGlobToRegex(%q).Match(%q) = %v, want %v", tt.glob, tt.path, got, tt.matches)
		}
	}
}

func TestSplitChainedCommands(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"echo hello && sudo rm -rf /", 2},
		{"echo hello", 1},
		{"a; b; c", 3},
		{"a || b && c", 3},
		{"echo 'a && b'", 1}, // quoted — should not split
		{"a | b | c", 3},
	}
	for _, tt := range tests {
		got := SplitChainedCommands(tt.input)
		if len(got) != tt.want {
			t.Errorf("SplitChainedCommands(%q) = %v (%d parts), want %d", tt.input, got, len(got), tt.want)
		}
	}
}

func TestEvaluateCommand(t *testing.T) {
	policies := []SecurityPolicy{
		{
			Allow: []string{"Bash(ls:*)"},
			Deny:  []string{"Bash(sudo *)"},
			Ask:   []string{"Bash(git push:*)"},
		},
	}

	// Deny wins.
	d := EvaluateCommand("sudo rm -rf /", policies)
	if d.Decision != Deny {
		t.Errorf("sudo: decision = %q, want deny", d.Decision)
	}

	// Allow.
	d = EvaluateCommand("ls -la", policies)
	if d.Decision != Allow {
		t.Errorf("ls: decision = %q, want allow", d.Decision)
	}

	// Ask.
	d = EvaluateCommand("git push origin main", policies)
	if d.Decision != Ask {
		t.Errorf("git push: decision = %q, want ask", d.Decision)
	}

	// Chained command bypass prevention.
	d = EvaluateCommand("echo ok && sudo rm -rf /", policies)
	if d.Decision != Deny {
		t.Errorf("chained sudo: decision = %q, want deny", d.Decision)
	}
}

func TestEvaluateCommandDenyOnly(t *testing.T) {
	policies := []SecurityPolicy{
		{Deny: []string{"Bash(rm -rf *)"}},
	}

	d := EvaluateCommandDenyOnly("rm -rf /", policies)
	if d.Decision != Deny {
		t.Errorf("decision = %q, want deny", d.Decision)
	}

	d = EvaluateCommandDenyOnly("ls -la", policies)
	if d.Decision != Allow {
		t.Errorf("decision = %q, want allow", d.Decision)
	}
}

func TestEvaluateFilePath(t *testing.T) {
	denyGlobs := [][]string{
		{"**/.env", "**/*.key"},
	}

	d := EvaluateFilePath("/home/user/project/.env", denyGlobs)
	if !d.Denied {
		t.Error("expected .env to be denied")
	}

	d = EvaluateFilePath("/home/user/project/main.go", denyGlobs)
	if d.Denied {
		t.Error("expected main.go to be allowed")
	}

	// Windows path normalization.
	d = EvaluateFilePath(`C:\Users\user\.env`, denyGlobs)
	if !d.Denied {
		t.Error("expected Windows .env to be denied")
	}
}

func TestExtractShellCommands(t *testing.T) {
	pythonCode := `
import os
os.system('rm -rf /')
import subprocess
subprocess.run(["ls", "-la", "/tmp"])
`

	cmds := ExtractShellCommands(pythonCode, "python")
	if len(cmds) < 2 {
		t.Errorf("python: got %d commands, want >= 2", len(cmds))
	}

	jsCode := `const { execSync } = require('child_process'); execSync('sudo reboot');`

	cmds = ExtractShellCommands(jsCode, "javascript")
	if len(cmds) < 1 {
		t.Errorf("javascript: got %d commands, want >= 1", len(cmds))
	}

	// Unknown language returns empty.
	cmds = ExtractShellCommands("some code", "brainfuck")
	if len(cmds) != 0 {
		t.Errorf("unknown: got %d commands, want 0", len(cmds))
	}
}
