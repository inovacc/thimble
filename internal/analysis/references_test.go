package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

const testRefFile = `package myapp

import "fmt"

type Runner interface {
	Run() error
}

type Server struct {
	Name string
}

func NewServer(name string) *Server {
	s := &Server{Name: name}
	fmt.Println("created", name)
	return s
}

func (s *Server) Start() error {
	fmt.Println("starting", s.Name)
	return s.Run()
}

func (s *Server) Run() error {
	return nil
}
`

func writeTempRefFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	path := filepath.Join(dir, "app.go")
	if err := os.WriteFile(path, []byte(testRefFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	return path
}

func TestExtractGoReferencesCallCount(t *testing.T) {
	path := writeTempRefFile(t)

	refs, err := ExtractGoReferences(path)
	if err != nil {
		t.Fatalf("ExtractGoReferences: %v", err)
	}

	calls := 0

	for _, ref := range refs {
		if ref.Kind == "call" {
			calls++
		}
	}

	// NewServer calls fmt.Println (1)
	// Server.Start calls fmt.Println (1) + s.Run (1)
	if calls < 3 {
		t.Errorf("expected at least 3 call references, got %d", calls)

		for _, ref := range refs {
			t.Logf("  %s -> %s (%s) at %s:%d", ref.From, ref.To, ref.Kind, ref.File, ref.Line)
		}
	}
}

func TestExtractGoReferencesCallerNames(t *testing.T) {
	path := writeTempRefFile(t)

	refs, err := ExtractGoReferences(path)
	if err != nil {
		t.Fatalf("ExtractGoReferences: %v", err)
	}

	callers := make(map[string]bool)

	for _, ref := range refs {
		if ref.Kind == "call" {
			callers[ref.From] = true
		}
	}

	expected := []string{"myapp.NewServer", "myapp.Server.Start"}
	for _, e := range expected {
		if !callers[e] {
			t.Errorf("missing caller: %s", e)
		}
	}
}

func TestExtractGoReferencesCallTargets(t *testing.T) {
	path := writeTempRefFile(t)

	refs, err := ExtractGoReferences(path)
	if err != nil {
		t.Fatalf("ExtractGoReferences: %v", err)
	}

	targets := make(map[string]bool)

	for _, ref := range refs {
		if ref.Kind == "call" {
			targets[ref.To] = true
		}
	}

	if !targets["fmt.Println"] {
		t.Error("expected call to fmt.Println")
	}

	if !targets["s.Run"] {
		t.Error("expected call to s.Run")
	}
}

func TestExtractGoReferencesHaveFileAndLine(t *testing.T) {
	path := writeTempRefFile(t)

	refs, err := ExtractGoReferences(path)
	if err != nil {
		t.Fatalf("ExtractGoReferences: %v", err)
	}

	for _, ref := range refs {
		if ref.File == "" {
			t.Errorf("reference %s->%s has empty file", ref.From, ref.To)
		}

		if ref.Line == 0 {
			t.Errorf("reference %s->%s has line 0", ref.From, ref.To)
		}
	}
}

func TestParseGoFileIncludesReferences(t *testing.T) {
	path := writeTempRefFile(t)

	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	if len(result.References) == 0 {
		t.Error("ParseGoFile should include references")
	}
}

func TestAnalyzerAggregatesReferences(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "app.go"), []byte(testRefFile), 0o644)

	a := NewAnalyzer(dir)

	result, err := a.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.Summary.TotalReferences == 0 {
		t.Error("expected TotalReferences > 0")
	}
}

func TestResolveCalleeVariants(t *testing.T) {
	// Test resolveCallee with func literals, type assertions, parens, generics, etc.
	dir := t.TempDir()
	path := filepath.Join(dir, "callee.go")
	_ = os.WriteFile(path, []byte(`package callee

func example() {
	// Paren call
	(doSomething)()

	// Func lit (anonymous) - should not resolve
	func() {}()

	// Type assertion used as value (not a call target)
	var x interface{}
	_ = x

	// Simple local call
	localFunc()
}

func doSomething() {}
func localFunc() {}
`), 0o644)

	refs, err := ExtractGoReferences(path)
	if err != nil {
		t.Fatalf("ExtractGoReferences: %v", err)
	}

	targets := make(map[string]bool)

	for _, ref := range refs {
		if ref.Kind == "call" {
			targets[ref.To] = true
		}
	}

	if !targets["doSomething"] {
		t.Error("expected call to doSomething (via paren expr)")
	}

	if !targets["localFunc"] {
		t.Error("expected call to localFunc")
	}
}

func TestExtractGoReferencesStructEmbed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "embed.go")
	_ = os.WriteFile(path, []byte(`package embed

type Base struct{}

type Child struct {
	Base
}
`), 0o644)

	refs, err := ExtractGoReferences(path)
	if err != nil {
		t.Fatalf("ExtractGoReferences: %v", err)
	}

	foundEmbed := false

	for _, ref := range refs {
		if ref.Kind == "embeds" && ref.From == "embed.Child" && ref.To == "Base" {
			foundEmbed = true
			break
		}
	}

	if !foundEmbed {
		t.Error("expected struct embed reference from Child to Base")
	}
}

func TestExtractGoReferencesInterfaceEmbed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "iface.go")
	_ = os.WriteFile(path, []byte(`package iface

type Reader interface {
	Read() error
}

type ReadWriter interface {
	Reader
	Write() error
}
`), 0o644)

	refs, err := ExtractGoReferences(path)
	if err != nil {
		t.Fatalf("ExtractGoReferences: %v", err)
	}

	foundEmbed := false

	for _, ref := range refs {
		if ref.Kind == "embeds" && ref.From == "iface.ReadWriter" && ref.To == "Reader" {
			foundEmbed = true
			break
		}
	}

	if !foundEmbed {
		t.Error("expected interface embed reference from ReadWriter to Reader")
	}
}

func TestExtractGoReferencesGenericCall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generic.go")
	_ = os.WriteFile(path, []byte(`package generic

func Transform[T any](v T) T { return v }

func caller() {
	Transform[int](42)
}
`), 0o644)

	refs, err := ExtractGoReferences(path)
	if err != nil {
		t.Fatalf("ExtractGoReferences: %v", err)
	}

	foundGeneric := false

	for _, ref := range refs {
		if ref.Kind == "call" && ref.To == "Transform" {
			foundGeneric = true
			break
		}
	}

	if !foundGeneric {
		t.Error("expected call to generic function Transform")
	}
}
