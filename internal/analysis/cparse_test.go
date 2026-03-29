package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

const testCFile = `
#include <stdio.h>
#include <stdlib.h>
#include "myheader.h"

#define MAX_SIZE 1024
#define VERSION "2.0"

typedef unsigned long ulong_t;

typedef struct point {
    int x;
    int y;
} Point;

struct config {
    int timeout;
    char *host;
};

enum color {
    RED,
    GREEN,
    BLUE
};

typedef enum {
    OK = 0,
    ERR = 1
} status_t;

static void helper(void) {
    int x = 42;
}

int main(int argc, char **argv) {
    helper();
    return 0;
}

const char *get_name(void) {
    return "test";
}

void process_data(int *buf, size_t len) {
    for (int i = 0; i < len; i++) {
        buf[i] = 0;
    }
}
`

func writeTempCFile(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.c")

	if err := os.WriteFile(path, []byte(testCFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	return path
}

func TestParseCFileFunctions(t *testing.T) {
	path := writeTempCFile(t)

	result, err := ParseCFile(path)
	if err != nil {
		t.Fatalf("ParseCFile: %v", err)
	}

	functions := filterByKind(result.Symbols, KindFunction)

	names := make(map[string]bool)
	for _, f := range functions {
		names[f.Name] = true
	}

	for _, want := range []string{"helper", "main", "get_name", "process_data"} {
		if !names[want] {
			t.Errorf("missing function: %s", want)
		}
	}
}

func TestParseCFileStructs(t *testing.T) {
	path := writeTempCFile(t)

	result, err := ParseCFile(path)
	if err != nil {
		t.Fatalf("ParseCFile: %v", err)
	}

	structs := filterByKind(result.Symbols, KindStruct)

	names := make(map[string]bool)
	for _, s := range structs {
		names[s.Name] = true
	}

	for _, want := range []string{"point", "config"} {
		if !names[want] {
			t.Errorf("missing struct: %s", want)
		}
	}
}

func TestParseCFileEnums(t *testing.T) {
	path := writeTempCFile(t)

	result, err := ParseCFile(path)
	if err != nil {
		t.Fatalf("ParseCFile: %v", err)
	}

	// Enums are stored as KindType.
	types := filterByKind(result.Symbols, KindType)

	names := make(map[string]bool)
	for _, ty := range types {
		names[ty.Name] = true
	}

	if !names["color"] {
		t.Error("missing enum: color")
	}
}

func TestParseCFileTypedefs(t *testing.T) {
	path := writeTempCFile(t)

	result, err := ParseCFile(path)
	if err != nil {
		t.Fatalf("ParseCFile: %v", err)
	}

	types := filterByKind(result.Symbols, KindType)

	names := make(map[string]bool)
	for _, ty := range types {
		names[ty.Name] = true
	}

	if !names["ulong_t"] {
		t.Error("missing typedef: ulong_t")
	}
}

func TestParseCFileDefines(t *testing.T) {
	path := writeTempCFile(t)

	result, err := ParseCFile(path)
	if err != nil {
		t.Fatalf("ParseCFile: %v", err)
	}

	constants := filterByKind(result.Symbols, KindConstant)

	names := make(map[string]bool)
	for _, c := range constants {
		names[c.Name] = true
	}

	for _, want := range []string{"MAX_SIZE", "VERSION"} {
		if !names[want] {
			t.Errorf("missing #define: %s", want)
		}
	}
}

func TestParseCFileIncludes(t *testing.T) {
	path := writeTempCFile(t)

	result, err := ParseCFile(path)
	if err != nil {
		t.Fatalf("ParseCFile: %v", err)
	}

	if len(result.Imports) != 3 {
		t.Fatalf("expected 3 imports, got %d: %v", len(result.Imports), result.Imports)
	}

	expected := []string{"stdio.h", "stdlib.h", "myheader.h"}
	for i, want := range expected {
		if result.Imports[i] != want {
			t.Errorf("import[%d] = %q, want %q", i, result.Imports[i], want)
		}
	}
}

func TestParseCFileLanguage(t *testing.T) {
	path := writeTempCFile(t)

	result, err := ParseCFile(path)
	if err != nil {
		t.Fatalf("ParseCFile: %v", err)
	}

	if result.Language != LangC {
		t.Errorf("Language = %q, want %q", result.Language, LangC)
	}
}

func TestDetectLanguageC(t *testing.T) {
	tests := []struct {
		path string
		want Language
	}{
		{"main.c", LangC},
		{"header.h", LangC},
		{"lib.cpp", LangC},
		{"lib.hpp", LangC},
		{"lib.cc", LangC},
		{"lib.cxx", LangC},
		{"other.txt", ""},
	}

	for _, tt := range tests {
		if got := detectLanguage(tt.path); got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
