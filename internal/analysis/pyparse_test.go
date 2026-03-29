package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

const testPyFile = `import os
import sys
from pathlib import Path
from typing import Optional, List

MAX_RETRIES: int = 3
DEFAULT_TIMEOUT = 30
API_VERSION = "2.0"

class UserRepository:
    """Handles user persistence."""

    def __init__(self, db):
        self.db = db

    def find_by_email(self, email: str) -> Optional[dict]:
        return None

    def list_all(self) -> List[dict]:
        return []

    def _internal_helper(self):
        pass

class AdminRepository(UserRepository):
    def delete_user(self, user_id: str) -> bool:
        return True

def create_app(config: dict) -> object:
    return object()

async def fetch_users(limit: int = 10) -> list:
    return []

def _private_helper():
    pass
`

func writeTempPyFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	path := filepath.Join(dir, "app.py")
	if err := os.WriteFile(path, []byte(testPyFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	return path
}

func TestParsePythonFileImports(t *testing.T) {
	path := writeTempPyFile(t)

	result, err := ParsePythonFile(path)
	if err != nil {
		t.Fatalf("ParsePythonFile: %v", err)
	}

	imports := make(map[string]bool)
	for _, imp := range result.Imports {
		imports[imp] = true
	}

	expected := []string{"os", "sys", "pathlib", "typing"}
	for _, e := range expected {
		if !imports[e] {
			t.Errorf("missing import: %s", e)
		}
	}
}

func TestParsePythonFileClasses(t *testing.T) {
	path := writeTempPyFile(t)

	result, err := ParsePythonFile(path)
	if err != nil {
		t.Fatalf("ParsePythonFile: %v", err)
	}

	classes := filterByKind(result.Symbols, KindStruct)
	if len(classes) != 2 {
		t.Fatalf("expected 2 classes, got %d", len(classes))
	}

	names := make(map[string]bool)
	for _, c := range classes {
		names[c.Name] = true
	}

	if !names["UserRepository"] {
		t.Error("missing class: UserRepository")
	}

	if !names["AdminRepository"] {
		t.Error("missing class: AdminRepository")
	}

	// Check inheritance.
	for _, c := range classes {
		if c.Name == "AdminRepository" && c.Signature != "class AdminRepository(UserRepository)" {
			t.Errorf("AdminRepository signature = %q", c.Signature)
		}
	}
}

func TestParsePythonFileFunctions(t *testing.T) {
	path := writeTempPyFile(t)

	result, err := ParsePythonFile(path)
	if err != nil {
		t.Fatalf("ParsePythonFile: %v", err)
	}

	funcs := filterByKind(result.Symbols, KindFunction)

	names := make(map[string]bool)
	for _, f := range funcs {
		names[f.Name] = true
	}

	if !names["create_app"] {
		t.Error("missing function: create_app")
	}

	if !names["fetch_users"] {
		t.Error("missing function: fetch_users")
	}

	if !names["_private_helper"] {
		t.Error("missing function: _private_helper")
	}

	// Check export status.
	for _, f := range funcs {
		if f.Name == "_private_helper" && f.Exported {
			t.Error("_private_helper should not be exported")
		}

		if f.Name == "create_app" && !f.Exported {
			t.Error("create_app should be exported")
		}
	}
}

func TestParsePythonFileMethods(t *testing.T) {
	path := writeTempPyFile(t)

	result, err := ParsePythonFile(path)
	if err != nil {
		t.Fatalf("ParsePythonFile: %v", err)
	}

	methods := filterByKind(result.Symbols, KindMethod)
	names := make(map[string]bool)
	receivers := make(map[string]string)

	for _, m := range methods {
		names[m.Name] = true
		receivers[m.Name] = m.Receiver
	}

	// __init__ is included, dunder methods like __str__ would be skipped.
	if !names["__init__"] {
		t.Error("missing method: __init__")
	}

	if !names["find_by_email"] {
		t.Error("missing method: find_by_email")
	}

	if !names["delete_user"] {
		t.Error("missing method: delete_user")
	}

	// Check receiver assignment.
	if receivers["find_by_email"] != "UserRepository" {
		t.Errorf("find_by_email receiver = %q, want UserRepository", receivers["find_by_email"])
	}

	if receivers["delete_user"] != "AdminRepository" {
		t.Errorf("delete_user receiver = %q, want AdminRepository", receivers["delete_user"])
	}

	// _internal_helper should not be exported.
	for _, m := range methods {
		if m.Name == "_internal_helper" && m.Exported {
			t.Error("_internal_helper should not be exported")
		}
	}
}

func TestParsePythonFileConstants(t *testing.T) {
	path := writeTempPyFile(t)

	result, err := ParsePythonFile(path)
	if err != nil {
		t.Fatalf("ParsePythonFile: %v", err)
	}

	consts := filterByKind(result.Symbols, KindConstant)

	names := make(map[string]bool)
	for _, c := range consts {
		names[c.Name] = true
	}

	if !names["MAX_RETRIES"] {
		t.Error("missing constant: MAX_RETRIES")
	}

	if !names["DEFAULT_TIMEOUT"] {
		t.Error("missing constant: DEFAULT_TIMEOUT")
	}

	if !names["API_VERSION"] {
		t.Error("missing constant: API_VERSION")
	}

	// Check type annotation.
	for _, c := range consts {
		if c.Name == "MAX_RETRIES" && c.Signature != "MAX_RETRIES: int" {
			t.Errorf("MAX_RETRIES signature = %q, want %q", c.Signature, "MAX_RETRIES: int")
		}
	}
}

func TestParsePythonFileReturnTypes(t *testing.T) {
	path := writeTempPyFile(t)

	result, err := ParsePythonFile(path)
	if err != nil {
		t.Fatalf("ParsePythonFile: %v", err)
	}

	for _, sym := range result.Symbols {
		if sym.Name == "create_app" {
			if sym.Signature != "def create_app(config: dict) -> object" {
				t.Errorf("create_app signature = %q", sym.Signature)
			}
		}
	}
}

func TestDetectLanguagePython(t *testing.T) {
	if got := detectLanguage("app.py"); got != LangPython {
		t.Errorf("detectLanguage('app.py') = %q, want %q", got, LangPython)
	}
}
