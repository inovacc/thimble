package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

const testRustFile = `use std::collections::HashMap;
use crate::config::Settings;

pub const MAX_SIZE: usize = 1024;
const INTERNAL_LIMIT: u32 = 100;

pub type Result<T> = std::result::Result<T, Error>;

/// A user in the system.
pub struct User<T> {
    pub name: String,
    age: u32,
    data: T,
}

struct InternalCache {
    entries: Vec<String>,
}

pub enum Status {
    Active,
    Inactive,
    Pending(String),
}

pub trait Serializable {
    fn serialize(&self) -> Vec<u8>;
    fn deserialize(data: &[u8]) -> Self;
}

impl User<String> {
    pub fn new(name: String, age: u32) -> Self {
        Self { name, age, data: String::new() }
    }

    fn validate(&self) -> bool {
        self.age > 0
    }
}

impl Serializable for User<String> {
    fn serialize(&self) -> Vec<u8> {
        vec![]
    }

    fn deserialize(data: &[u8]) -> Self {
        todo!()
    }
}

pub fn process_data(input: &str) -> Result<String> {
    Ok(input.to_string())
}

pub async fn fetch_remote(url: &str) -> Result<Vec<u8>> {
    todo!()
}

fn internal_helper(x: i32, y: i32) -> i32 {
    x + y
}
`

func writeTempRustFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	path := filepath.Join(dir, "lib.rs")
	if err := os.WriteFile(path, []byte(testRustFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	return path
}

func TestParseRustFile_Package(t *testing.T) {
	path := writeTempRustFile(t)

	result, err := ParseRustFile(path)
	if err != nil {
		t.Fatalf("ParseRustFile: %v", err)
	}

	if result.Package == "" {
		t.Error("package should not be empty")
	}

	if result.Language != LangRust {
		t.Errorf("language = %q, want %q", result.Language, LangRust)
	}
}

func TestParseRustFile_Imports(t *testing.T) {
	path := writeTempRustFile(t)

	result, err := ParseRustFile(path)
	if err != nil {
		t.Fatalf("ParseRustFile: %v", err)
	}

	imports := make(map[string]bool)
	for _, imp := range result.Imports {
		imports[imp] = true
	}

	expected := []string{"std::collections::HashMap", "crate::config::Settings"}
	for _, e := range expected {
		if !imports[e] {
			t.Errorf("missing import: %s", e)
		}
	}
}

func TestParseRustFile_Functions(t *testing.T) {
	path := writeTempRustFile(t)

	result, err := ParseRustFile(path)
	if err != nil {
		t.Fatalf("ParseRustFile: %v", err)
	}

	funcs := filterByKind(result.Symbols, KindFunction)

	names := make(map[string]bool)
	for _, f := range funcs {
		names[f.Name] = true
	}

	if !names["process_data"] {
		t.Error("missing function: process_data")
	}

	if !names["fetch_remote"] {
		t.Error("missing function: fetch_remote")
	}

	if !names["internal_helper"] {
		t.Error("missing function: internal_helper")
	}

	// Check export status.
	for _, f := range funcs {
		if f.Name == "internal_helper" && f.Exported {
			t.Error("internal_helper should not be exported")
		}

		if f.Name == "process_data" && !f.Exported {
			t.Error("process_data should be exported")
		}
	}

	// Check return type in signature.
	for _, f := range funcs {
		if f.Name == "process_data" {
			expected := "fn process_data(input: &str) -> Result<String>"
			if f.Signature != expected {
				t.Errorf("process_data signature = %q, want %q", f.Signature, expected)
			}
		}
	}
}

func TestParseRustFile_Structs(t *testing.T) {
	path := writeTempRustFile(t)

	result, err := ParseRustFile(path)
	if err != nil {
		t.Fatalf("ParseRustFile: %v", err)
	}

	structs := filterByKind(result.Symbols, KindStruct)
	if len(structs) != 2 {
		t.Fatalf("expected 2 structs, got %d", len(structs))
	}

	names := make(map[string]bool)
	for _, s := range structs {
		names[s.Name] = true
	}

	if !names["User"] {
		t.Error("missing struct: User")
	}

	if !names["InternalCache"] {
		t.Error("missing struct: InternalCache")
	}

	// User is pub, InternalCache is not.
	for _, s := range structs {
		if s.Name == "User" && !s.Exported {
			t.Error("User should be exported")
		}

		if s.Name == "InternalCache" && s.Exported {
			t.Error("InternalCache should not be exported")
		}
	}
}

func TestParseRustFile_Enums(t *testing.T) {
	path := writeTempRustFile(t)

	result, err := ParseRustFile(path)
	if err != nil {
		t.Fatalf("ParseRustFile: %v", err)
	}

	// Enums and type aliases are both KindType.
	types := filterByKind(result.Symbols, KindType)

	names := make(map[string]bool)
	for _, ty := range types {
		names[ty.Name] = true
	}

	if !names["Status"] {
		t.Error("missing enum: Status")
	}

	if !names["Result"] {
		t.Error("missing type alias: Result")
	}
}

func TestParseRustFile_Traits(t *testing.T) {
	path := writeTempRustFile(t)

	result, err := ParseRustFile(path)
	if err != nil {
		t.Fatalf("ParseRustFile: %v", err)
	}

	traits := filterByKind(result.Symbols, KindInterface)
	if len(traits) != 1 {
		t.Fatalf("expected 1 trait, got %d", len(traits))
	}

	if traits[0].Name != "Serializable" {
		t.Errorf("trait name = %q, want Serializable", traits[0].Name)
	}

	if !traits[0].Exported {
		t.Error("Serializable should be exported")
	}
}

func TestParseRustFile_Methods(t *testing.T) {
	path := writeTempRustFile(t)

	result, err := ParseRustFile(path)
	if err != nil {
		t.Fatalf("ParseRustFile: %v", err)
	}

	methods := filterByKind(result.Symbols, KindMethod)
	names := make(map[string]bool)
	receivers := make(map[string]string)

	for _, m := range methods {
		names[m.Name] = true
		receivers[m.Name] = m.Receiver
	}

	if !names["new"] {
		t.Error("missing method: new")
	}

	if !names["validate"] {
		t.Error("missing method: validate")
	}

	if !names["serialize"] {
		t.Error("missing method: serialize")
	}

	if !names["deserialize"] {
		t.Error("missing method: deserialize")
	}

	// Check receivers.
	if receivers["new"] != "User" {
		t.Errorf("new receiver = %q, want User", receivers["new"])
	}

	if receivers["validate"] != "User" {
		t.Errorf("validate receiver = %q, want User", receivers["validate"])
	}
	// Trait impl methods should have the impl target type as receiver.
	if receivers["serialize"] != "User" {
		t.Errorf("serialize receiver = %q, want User", receivers["serialize"])
	}
}

func TestParseRustFile_Constants(t *testing.T) {
	path := writeTempRustFile(t)

	result, err := ParseRustFile(path)
	if err != nil {
		t.Fatalf("ParseRustFile: %v", err)
	}

	consts := filterByKind(result.Symbols, KindConstant)

	names := make(map[string]bool)
	for _, c := range consts {
		names[c.Name] = true
	}

	if !names["MAX_SIZE"] {
		t.Error("missing constant: MAX_SIZE")
	}

	if !names["INTERNAL_LIMIT"] {
		t.Error("missing constant: INTERNAL_LIMIT")
	}

	// Check signature.
	for _, c := range consts {
		if c.Name == "MAX_SIZE" {
			expected := "const MAX_SIZE: usize"
			if c.Signature != expected {
				t.Errorf("MAX_SIZE signature = %q, want %q", c.Signature, expected)
			}
		}
	}
}

func TestParseRustFile_Visibility(t *testing.T) {
	path := writeTempRustFile(t)

	result, err := ParseRustFile(path)
	if err != nil {
		t.Fatalf("ParseRustFile: %v", err)
	}

	exportMap := make(map[string]bool)
	for _, sym := range result.Symbols {
		exportMap[sym.Name] = sym.Exported
	}

	pubItems := []string{"MAX_SIZE", "Result", "User", "Status", "Serializable", "process_data", "fetch_remote", "new"}
	for _, name := range pubItems {
		if !exportMap[name] {
			t.Errorf("%s should be exported", name)
		}
	}

	privateItems := []string{"INTERNAL_LIMIT", "InternalCache", "internal_helper", "validate"}
	for _, name := range privateItems {
		if exportMap[name] {
			t.Errorf("%s should not be exported", name)
		}
	}
}
