package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

const testShellFile = `#!/bin/bash
# A test shell script

set -euo pipefail

readonly VERSION="1.0.0"
declare -r CONFIG_DIR="/etc/myapp"
MAX_RETRIES=5
export LOG_LEVEL=info

source ./lib/helpers.sh
. /usr/local/lib/common.sh

alias ll='ls -la'

setup() {
    echo "setting up"
    local tmp_dir
    tmp_dir=$(mktemp -d)
}

function cleanup {
    echo "cleaning up"
}

function run-tests() {
    echo "running tests"
    local INNER_CONST=42
}

main() {
    setup
    run-tests
    cleanup
}

main "$@"
`

func writeTempShellFile(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.sh")

	if err := os.WriteFile(path, []byte(testShellFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	return path
}

func TestParseShellFileFunctions(t *testing.T) {
	path := writeTempShellFile(t)

	result, err := ParseShellFile(path)
	if err != nil {
		t.Fatalf("ParseShellFile: %v", err)
	}

	functions := filterByKind(result.Symbols, KindFunction)

	names := make(map[string]bool)
	for _, f := range functions {
		names[f.Name] = true
	}

	for _, want := range []string{"setup", "cleanup", "run-tests", "main", "ll"} {
		if !names[want] {
			t.Errorf("missing function: %s", want)
		}
	}
}

func TestParseShellFileConstants(t *testing.T) {
	path := writeTempShellFile(t)

	result, err := ParseShellFile(path)
	if err != nil {
		t.Fatalf("ParseShellFile: %v", err)
	}

	constants := filterByKind(result.Symbols, KindConstant)

	names := make(map[string]bool)
	for _, c := range constants {
		names[c.Name] = true
	}

	for _, want := range []string{"VERSION", "CONFIG_DIR", "MAX_RETRIES"} {
		if !names[want] {
			t.Errorf("missing constant: %s", want)
		}
	}
}

func TestParseShellFileExports(t *testing.T) {
	path := writeTempShellFile(t)

	result, err := ParseShellFile(path)
	if err != nil {
		t.Fatalf("ParseShellFile: %v", err)
	}

	vars := filterByKind(result.Symbols, KindVariable)
	if len(vars) != 1 {
		t.Fatalf("expected 1 exported variable, got %d", len(vars))
	}

	if vars[0].Name != "LOG_LEVEL" {
		t.Errorf("export name = %q, want %q", vars[0].Name, "LOG_LEVEL")
	}
}

func TestParseShellFileImports(t *testing.T) {
	path := writeTempShellFile(t)

	result, err := ParseShellFile(path)
	if err != nil {
		t.Fatalf("ParseShellFile: %v", err)
	}

	if len(result.Imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(result.Imports))
	}

	if result.Imports[0] != "./lib/helpers.sh" {
		t.Errorf("import[0] = %q, want %q", result.Imports[0], "./lib/helpers.sh")
	}

	if result.Imports[1] != "/usr/local/lib/common.sh" {
		t.Errorf("import[1] = %q, want %q", result.Imports[1], "/usr/local/lib/common.sh")
	}
}

func TestParseShellFileLanguage(t *testing.T) {
	path := writeTempShellFile(t)

	result, err := ParseShellFile(path)
	if err != nil {
		t.Fatalf("ParseShellFile: %v", err)
	}

	if result.Language != LangShell {
		t.Errorf("Language = %q, want %q", result.Language, LangShell)
	}
}

func TestParseShellFileNoBodyVars(t *testing.T) {
	path := writeTempShellFile(t)

	result, err := ParseShellFile(path)
	if err != nil {
		t.Fatalf("ParseShellFile: %v", err)
	}

	for _, sym := range result.Symbols {
		if sym.Name == "INNER_CONST" {
			t.Error("should not extract INNER_CONST from inside function body")
		}
	}
}

func TestDetectLanguageShell(t *testing.T) {
	tests := []struct {
		path string
		want Language
	}{
		{"script.sh", LangShell},
		{"build.bash", LangShell},
		{"Makefile", ""},
	}

	for _, tt := range tests {
		if got := detectLanguage(tt.path); got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
