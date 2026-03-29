package analysis

import (
	"testing"
)

func TestExtractCrossLangRefs(t *testing.T) { //nolint:maintidx // table-driven test with many cases
	tests := []struct {
		name     string
		lang     Language
		code     string
		wantRefs []struct {
			to   string
			kind string
			line int
		}
	}{
		// Go patterns
		{
			name: "go exec.Command python",
			lang: LangGo,
			code: `package main

import "os/exec"

func run() {
	cmd := exec.Command("python", "train.py")
	_ = cmd.Run()
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "python:python train.py", kind: crossLangKind, line: 6},
			},
		},
		{
			name: "go exec.Command python3",
			lang: LangGo,
			code: `package main

func run() {
	exec.Command("python3", "script.py")
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "python:python3 script.py", kind: crossLangKind, line: 4},
			},
		},
		{
			name: "go exec.Command node",
			lang: LangGo,
			code: `package main

func run() {
	exec.Command("node", "server.js")
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "node:node server.js", kind: crossLangKind, line: 4},
			},
		},
		{
			name: "go exec.Command npx",
			lang: LangGo,
			code: `package main

func run() {
	exec.Command("npx", "tsx")
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "node:npx tsx", kind: crossLangKind, line: 4},
			},
		},
		{
			name: "go exec.Command cargo",
			lang: LangGo,
			code: `package main

func run() {
	exec.Command("cargo", "run")
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "rust:cargo run", kind: crossLangKind, line: 4},
			},
		},
		{
			name: "go exec.Command bash",
			lang: LangGo,
			code: `package main

func run() {
	exec.Command("bash", "-c")
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "shell:bash -c", kind: crossLangKind, line: 4},
			},
		},
		{
			name: "go exec.CommandContext",
			lang: LangGo,
			code: `package main

func run(ctx context.Context) {
	exec.CommandContext(ctx, "ruby", "script.rb")
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "ruby:ruby script.rb", kind: crossLangKind, line: 4},
			},
		},
		{
			name: "go exec.Command no arg",
			lang: LangGo,
			code: `package main

func run() {
	exec.Command("perl")
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "perl:perl", kind: crossLangKind, line: 4},
			},
		},

		// Python patterns
		{
			name: "python subprocess.run go",
			lang: LangPython,
			code: `import subprocess

def build():
    subprocess.run(["go", "run", "main.go"])
`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "go:go run", kind: crossLangKind, line: 4},
			},
		},
		{
			name: "python subprocess.call node",
			lang: LangPython,
			code: `import subprocess

def run():
    subprocess.call(["node", "app.js"])
`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "node:node app.js", kind: crossLangKind, line: 4},
			},
		},
		{
			name: "python subprocess.Popen",
			lang: LangPython,
			code: `import subprocess

def run():
    subprocess.Popen(["cargo", "build"])
`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "rust:cargo build", kind: crossLangKind, line: 4},
			},
		},
		{
			name: "python os.system cargo run",
			lang: LangPython,
			code: `import os

def deploy():
    os.system("cargo run --release")
`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "rust:cargo run", kind: crossLangKind, line: 4},
			},
		},
		{
			name: "python os.system python",
			lang: LangPython,
			code: `import os

def run():
    os.system("python script.py")
`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "python:python script.py", kind: crossLangKind, line: 4},
			},
		},

		// TypeScript/JavaScript patterns
		{
			name: "ts execSync go run",
			lang: LangTypeScript,
			code: `import { execSync } from 'child_process';

function build() {
    execSync("go run main.go");
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "go:go run", kind: crossLangKind, line: 4},
			},
		},
		{
			name: "ts exec python",
			lang: LangTypeScript,
			code: `const { exec } = require('child_process');

exec("python train.py", (err, stdout) => {});`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "python:python train.py", kind: crossLangKind, line: 3},
			},
		},
		{
			name: "ts spawn cargo",
			lang: LangTypeScript,
			code: `import { spawn } from 'child_process';

const child = spawn("cargo", ["run"]);`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "rust:cargo run", kind: crossLangKind, line: 3},
			},
		},
		{
			name: "ts spawnSync rustc",
			lang: LangTypeScript,
			code: `const { spawnSync } = require('child_process');

spawnSync("rustc", ["main.rs"]);`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "rust:rustc main.rs", kind: crossLangKind, line: 3},
			},
		},

		// Shell patterns
		{
			name: "shell python script",
			lang: LangShell,
			code: `#!/bin/bash

python script.py
node app.js
go run main.go
cargo build`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "python:python script.py", kind: crossLangKind, line: 3},
				{to: "node:node app.js", kind: crossLangKind, line: 4},
				{to: "go:go run", kind: crossLangKind, line: 5},
				{to: "rust:cargo build", kind: crossLangKind, line: 6},
			},
		},
		{
			name: "shell chained commands",
			lang: LangShell,
			code: `#!/bin/bash
python train.py && node serve.js`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "python:python train.py", kind: crossLangKind, line: 2},
				{to: "node:node serve.js", kind: crossLangKind, line: 2},
			},
		},

		// Rust patterns
		{
			name: "rust Command::new python",
			lang: LangRust,
			code: `use std::process::Command;

fn main() {
    Command::new("python")
        .arg("script.py")
        .spawn()
        .unwrap();
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "python:python", kind: crossLangKind, line: 4},
			},
		},
		{
			name: "rust Command::new node",
			lang: LangRust,
			code: `fn run() {
    Command::new("node")
        .arg("server.js")
        .status()
        .unwrap();
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "node:node", kind: crossLangKind, line: 2},
			},
		},
		{
			name: "rust Command::new go",
			lang: LangRust,
			code: `fn build() {
    Command::new("go")
        .args(&["run", "main.go"])
        .output()
        .unwrap();
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "go:go", kind: crossLangKind, line: 2},
			},
		},

		// No false positives
		{
			name: "go comment should be ignored",
			lang: LangGo,
			code: `package main

// exec.Command("python", "train.py") is used for ML
func run() {}`,
			wantRefs: nil,
		},
		{
			name: "python comment should be ignored",
			lang: LangPython,
			code: `# subprocess.run(["go", "run", "main.go"])
def run():
    pass`,
			wantRefs: nil,
		},
		{
			name: "shell comment should be ignored",
			lang: LangShell,
			code: `#!/bin/bash
# python script.py
echo "done"`,
			wantRefs: nil,
		},
		{
			name: "rust comment should be ignored",
			lang: LangRust,
			code: `// Command::new("python") would spawn python
fn main() {}`,
			wantRefs: nil,
		},
		{
			name: "ts comment should be ignored",
			lang: LangTypeScript,
			code: `// execSync("go run main.go") -- example
function noop() {}`,
			wantRefs: nil,
		},
		{
			name:     "unsupported language returns nil",
			lang:     LangProto,
			code:     `syntax = "proto3";`,
			wantRefs: nil,
		},
		{
			name:     "empty code returns nil",
			lang:     LangGo,
			code:     ``,
			wantRefs: nil,
		},

		// Multiple matches in one file
		{
			name: "go multiple exec.Command calls",
			lang: LangGo,
			code: `package main

func run() {
	exec.Command("python", "train.py")
	exec.Command("node", "serve.js")
	exec.Command("cargo", "build")
}`,
			wantRefs: []struct {
				to   string
				kind string
				line int
			}{
				{to: "python:python train.py", kind: crossLangKind, line: 4},
				{to: "node:node serve.js", kind: crossLangKind, line: 5},
				{to: "rust:cargo build", kind: crossLangKind, line: 6},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := ExtractCrossLangRefs("test/file.go", tt.lang, tt.code)

			if len(refs) != len(tt.wantRefs) {
				t.Fatalf("got %d refs, want %d", len(refs), len(tt.wantRefs))

				for _, r := range refs {
					t.Logf("  got: %s -> %s (%s) line %d", r.From, r.To, r.Kind, r.Line)
				}

				return
			}

			for i, want := range tt.wantRefs {
				got := refs[i]
				if got.To != want.to {
					t.Errorf("ref[%d] To = %q, want %q", i, got.To, want.to)
				}

				if got.Kind != want.kind {
					t.Errorf("ref[%d] Kind = %q, want %q", i, got.Kind, want.kind)
				}

				if got.Line != want.line {
					t.Errorf("ref[%d] Line = %d, want %d", i, got.Line, want.line)
				}

				if got.File != "test/file.go" {
					t.Errorf("ref[%d] File = %q, want %q", i, got.File, "test/file.go")
				}

				if got.From != "test/file.go" {
					t.Errorf("ref[%d] From = %q, want %q", i, got.From, "test/file.go")
				}
			}
		})
	}
}

func TestIsComment(t *testing.T) {
	tests := []struct {
		line string
		lang Language
		want bool
	}{
		{"// a comment", LangGo, true},
		{"/* block comment */", LangGo, true},
		{"* continuation", LangGo, true},
		{"code()", LangGo, false},
		{"# a comment", LangPython, true},
		{"code()", LangPython, false},
		{"# a comment", LangShell, true},
		{"echo hello", LangShell, false},
		{"// a comment", LangRust, true},
		{"let x = 1;", LangRust, false},
		{"// a comment", LangTypeScript, true},
		{"const x = 1;", LangTypeScript, false},
	}

	for _, tt := range tests {
		got := isComment(tt.line, tt.lang)
		if got != tt.want {
			t.Errorf("isComment(%q, %s) = %v, want %v", tt.line, tt.lang, got, tt.want)
		}
	}
}

func TestCommandToLang(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"python", "python"},
		{"python3", "python"},
		{"node", "node"},
		{"npx", "node"},
		{"cargo", "rust"},
		{"rustc", "rust"},
		{"go", "go"},
		{"ruby", "ruby"},
		{"php", "php"},
		{"perl", "perl"},
		{"bash", "shell"},
		{"sh", "shell"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		got := commandToLang(tt.cmd)
		if got != tt.want {
			t.Errorf("commandToLang(%q) = %q, want %q", tt.cmd, got, tt.want)
		}
	}
}

func TestBuildTarget(t *testing.T) {
	tests := []struct {
		cmd  string
		arg  string
		want string
	}{
		{"python", "script.py", "python script.py"},
		{"node", "", "node"},
		{"cargo", "run", "cargo run"},
	}

	for _, tt := range tests {
		got := buildTarget(tt.cmd, tt.arg)
		if got != tt.want {
			t.Errorf("buildTarget(%q, %q) = %q, want %q", tt.cmd, tt.arg, got, tt.want)
		}
	}
}
