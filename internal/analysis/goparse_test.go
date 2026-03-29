package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

const testGoFile = `package example

import (
	"fmt"
	"strings"
)

// Config holds application configuration.
type Config struct {
	Name    string
	Port    int
	Verbose bool
}

// Runner defines something that can run.
type Runner interface {
	Run() error
	Stop()
}

// MaxRetries is the maximum number of retries.
const MaxRetries = 3

var defaultName = "example"

// NewConfig creates a new Config with defaults.
func NewConfig(name string) *Config {
	return &Config{Name: name, Port: 8080}
}

// Start starts the config runner.
func (c *Config) Start() error {
	fmt.Println(strings.Join([]string{c.Name, "starting"}, " "))
	return nil
}
`

func writeTempGoFile(t *testing.T, content string) string { //nolint:unparam // test helper with fixed content
	t.Helper()
	dir := t.TempDir()

	path := filepath.Join(dir, "example.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	return path
}

func TestParseGoFilePackage(t *testing.T) {
	path := writeTempGoFile(t, testGoFile)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	if result.Package != "example" {
		t.Errorf("Package = %q, want %q", result.Package, "example")
	}

	if result.Language != LangGo {
		t.Errorf("Language = %q, want %q", result.Language, LangGo)
	}
}

func TestParseGoFileImports(t *testing.T) {
	path := writeTempGoFile(t, testGoFile)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	imports := make(map[string]bool)
	for _, imp := range result.Imports {
		imports[imp] = true
	}

	if !imports["fmt"] {
		t.Error("missing import: fmt")
	}

	if !imports["strings"] {
		t.Error("missing import: strings")
	}
}

func TestParseGoFileSymbolCounts(t *testing.T) {
	path := writeTempGoFile(t, testGoFile)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	kinds := make(map[SymbolKind]int)
	for _, sym := range result.Symbols {
		kinds[sym.Kind]++
	}

	tests := []struct {
		kind SymbolKind
		want int
	}{
		{KindStruct, 1},    // Config
		{KindInterface, 1}, // Runner
		{KindFunction, 1},  // NewConfig
		{KindMethod, 1},    // Config.Start
		{KindConstant, 1},  // MaxRetries
		{KindVariable, 1},  // defaultName
	}

	for _, tt := range tests {
		if got := kinds[tt.kind]; got != tt.want {
			t.Errorf("kind %s: got %d, want %d", tt.kind, got, tt.want)
		}
	}
}

func TestParseGoFileExported(t *testing.T) {
	path := writeTempGoFile(t, testGoFile)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	exported := make(map[string]bool)
	for _, sym := range result.Symbols {
		exported[sym.Name] = sym.Exported
	}

	if !exported["Config"] {
		t.Error("Config should be exported")
	}

	if !exported["Runner"] {
		t.Error("Runner should be exported")
	}

	if !exported["NewConfig"] {
		t.Error("NewConfig should be exported")
	}

	if !exported["MaxRetries"] {
		t.Error("MaxRetries should be exported")
	}

	if exported["defaultName"] {
		t.Error("defaultName should NOT be exported")
	}
}

func TestParseGoFileMethodReceiver(t *testing.T) {
	path := writeTempGoFile(t, testGoFile)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	for _, sym := range result.Symbols {
		if sym.Name == "Start" {
			if sym.Kind != KindMethod {
				t.Errorf("Start kind = %q, want %q", sym.Kind, KindMethod)
			}

			if sym.Receiver != "*Config" {
				t.Errorf("Start receiver = %q, want %q", sym.Receiver, "*Config")
			}

			return
		}
	}

	t.Error("Start method not found")
}

func TestParseGoFileSignatures(t *testing.T) {
	path := writeTempGoFile(t, testGoFile)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	sigs := make(map[string]string)
	for _, sym := range result.Symbols {
		sigs[sym.Name] = sym.Signature
	}

	tests := []struct {
		name     string
		contains string
	}{
		{"NewConfig", "func NewConfig(name string) *Config"},
		{"Start", "func (*Config) Start() error"},
		{"Config", "type Config struct { Name string; Port int; Verbose bool }"},
		{"Runner", "type Runner interface { Run(); Stop() }"},
	}

	for _, tt := range tests {
		sig, ok := sigs[tt.name]
		if !ok {
			t.Errorf("symbol %q not found", tt.name)
			continue
		}

		if sig != tt.contains {
			t.Errorf("%s signature = %q, want %q", tt.name, sig, tt.contains)
		}
	}
}

func TestParseGoFileDoc(t *testing.T) {
	path := writeTempGoFile(t, testGoFile)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	docs := make(map[string]string)
	for _, sym := range result.Symbols {
		docs[sym.Name] = sym.Doc
	}

	if docs["Config"] == "" {
		t.Error("Config should have a doc comment")
	}

	if docs["NewConfig"] == "" {
		t.Error("NewConfig should have a doc comment")
	}
}

func TestParseGoFileInvalidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.go")
	_ = os.WriteFile(path, []byte("not valid go"), 0o644)

	_, err := ParseGoFile(path)
	if err == nil {
		t.Error("expected error for invalid Go file")
	}
}

func TestFormatExprVariousTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "types.go")
	validSrc := `package fmttest

import "time"

type MapField struct {
	Data map[string]int
}

type SliceField struct {
	Items []string
}

type ChanField struct {
	Ch chan int
}

type InterfaceField struct {
	Handler interface{}
}

type FuncField struct {
	Callback func()
}

type PointerField struct {
	Ref *string
}

type SelectorField struct {
	Timer time.Timer
}

func Variadic(args ...string) {}

type NestedInterface interface {
	io.Reader
}
`
	_ = os.WriteFile(path, []byte(validSrc), 0o644)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	sigs := make(map[string]string)
	for _, sym := range result.Symbols {
		sigs[sym.Name] = sym.Signature
	}

	tests := []struct {
		name     string
		contains string
	}{
		{"MapField", "map[string]int"},
		{"SliceField", "[]string"},
		{"ChanField", "chan int"},
		{"InterfaceField", "interface{}"},
		{"FuncField", "func()"},
		{"PointerField", "*string"},
		{"SelectorField", "time.Timer"},
		{"Variadic", "...string"},
	}

	for _, tt := range tests {
		sig, ok := sigs[tt.name]
		if !ok {
			t.Errorf("symbol %q not found", tt.name)
			continue
		}

		found := false

		for i := 0; i <= len(sig)-len(tt.contains); i++ {
			if sig[i:i+len(tt.contains)] == tt.contains {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("%s signature %q should contain %q", tt.name, sig, tt.contains)
		}
	}
}

func TestFormatExprEmbeddedInterface(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "iface.go")
	_ = os.WriteFile(path, []byte(`package iface

type Base interface {
	Base()
}

type Extended interface {
	Base
	Extra()
}
`), 0o644)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	for _, sym := range result.Symbols {
		if sym.Name == "Extended" {
			if sym.Kind != KindInterface {
				t.Errorf("Extended kind = %q, want interface", sym.Kind)
			}

			// Signature should mention Base (embedded).
			found := false

			for i := 0; i <= len(sym.Signature)-len("Base"); i++ {
				if sym.Signature[i:i+len("Base")] == "Base" {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Extended signature %q should contain embedded Base", sym.Signature)
			}

			return
		}
	}

	t.Error("Extended interface not found")
}

func TestFormatExprEmbeddedStruct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "embed.go")
	_ = os.WriteFile(path, []byte(`package embed

type Base struct {
	Name string
}

type Child struct {
	Base
	Extra int
}
`), 0o644)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	for _, sym := range result.Symbols {
		if sym.Name == "Child" {
			// Signature should show embedded Base.
			found := false

			for i := 0; i <= len(sym.Signature)-len("Base"); i++ {
				if sym.Signature[i:i+len("Base")] == "Base" {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Child signature %q should contain embedded Base", sym.Signature)
			}

			return
		}
	}

	t.Error("Child struct not found")
}

func TestFormatFuncMultipleResults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.go")
	_ = os.WriteFile(path, []byte(`package multi

func Split(s string) (string, error) {
	return s, nil
}

func Named(s string) (result string, err error) {
	return s, nil
}
`), 0o644)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	sigs := make(map[string]string)
	for _, sym := range result.Symbols {
		sigs[sym.Name] = sym.Signature
	}

	// Multiple return values should be wrapped in parens.
	splitSig := sigs["Split"]
	if splitSig == "" {
		t.Fatal("Split not found")
	}

	// Named return values should also be in parens.
	namedSig := sigs["Named"]
	if namedSig == "" {
		t.Fatal("Named not found")
	}

	for _, s := range []string{splitSig, namedSig} {
		found := false

		for i := 0; i <= len(s)-len("("); i++ {
			// Look for closing paren after the params.
			if s[i:i+1] == ")" {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("signature %q should have parens", s)
		}
	}
}

func TestParseGoFileTypeAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alias.go")
	_ = os.WriteFile(path, []byte(`package alias

type ID string
type Count int
`), 0o644)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	for _, sym := range result.Symbols {
		if sym.Name == "ID" {
			if sym.Kind != KindType {
				t.Errorf("ID kind = %q, want type", sym.Kind)
			}

			return
		}
	}

	t.Error("ID type not found")
}

func TestFormatStructEmptyFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.go")
	_ = os.WriteFile(path, []byte(`package empty

type Empty struct{}
`), 0o644)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	for _, sym := range result.Symbols {
		if sym.Name == "Empty" {
			if sym.Signature != "type Empty struct{}" {
				t.Errorf("Empty signature = %q, want %q", sym.Signature, "type Empty struct{}")
			}

			return
		}
	}

	t.Error("Empty struct not found")
}

func TestFormatInterfaceEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.go")
	_ = os.WriteFile(path, []byte(`package empty

type Empty interface{}
`), 0o644)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	for _, sym := range result.Symbols {
		if sym.Name == "Empty" {
			if sym.Signature != "type Empty interface{}" {
				t.Errorf("Empty signature = %q, want %q", sym.Signature, "type Empty interface{}")
			}

			return
		}
	}

	t.Error("Empty interface not found")
}
