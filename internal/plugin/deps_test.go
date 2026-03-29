package plugin

import (
	"encoding/json"
	"strings"
	"testing"
)

func makePluginDef(name, version string, deps []PluginDependency) PluginDef {
	return PluginDef{
		Name:         name,
		Version:      version,
		Dependencies: deps,
		Tools: []ToolDef{
			{Name: "ctx_" + name, Description: "test tool", Command: "echo ok"},
		},
	}
}

func TestSatisfiesConstraint(t *testing.T) {
	tests := []struct {
		installed  string
		constraint string
		want       bool
	}{
		{"1.0.0", "", true},
		{"1.0.0", "*", true},
		{"1.2.3", "1.2.3", true},
		{"1.2.3", "1.2.4", false},
		{"1.2.0", ">=1.0.0", true},
		{"1.0.0", ">=1.0.0", true},
		{"0.9.0", ">=1.0.0", false},
		{"2.0.0", "<=2.0.0", true},
		{"2.0.1", "<=2.0.0", false},
		{"1.1.0", ">1.0.0", true},
		{"1.0.0", ">1.0.0", false},
		{"0.9.0", "<1.0.0", true},
		{"1.0.0", "<1.0.0", false},
		{"1.5.0", "^1.2.0", true},
		{"1.1.0", "^1.2.0", false},
		{"2.0.0", "^1.2.0", false},
		{"1.2.5", "~1.2.0", true},
		{"1.3.0", "~1.2.0", false},
		{"1.2.0", "~1.2.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.installed+"_"+tt.constraint, func(t *testing.T) {
			got := SatisfiesConstraint(tt.installed, tt.constraint)
			if got != tt.want {
				t.Errorf("SatisfiesConstraint(%q, %q) = %v, want %v",
					tt.installed, tt.constraint, got, tt.want)
			}
		})
	}
}

func TestValidateDependencies_NoDeps(t *testing.T) {
	p := makePluginDef("alpha", "1.0.0", nil)
	unmet := p.ValidateDependencies(nil)

	if len(unmet) != 0 {
		t.Fatalf("expected no unmet deps, got %v", unmet)
	}
}

func TestValidateDependencies_AllMet(t *testing.T) {
	p := makePluginDef("alpha", "1.0.0", []PluginDependency{
		{Name: "beta", Version: ">=1.0.0"},
	})

	installed := map[string]PluginDef{
		"beta": makePluginDef("beta", "1.2.0", nil),
	}

	unmet := p.ValidateDependencies(installed)
	if len(unmet) != 0 {
		t.Fatalf("expected no unmet deps, got %v", unmet)
	}
}

func TestValidateDependencies_Missing(t *testing.T) {
	p := makePluginDef("alpha", "1.0.0", []PluginDependency{
		{Name: "beta", Version: ">=1.0.0"},
	})

	unmet := p.ValidateDependencies(map[string]PluginDef{})
	if len(unmet) != 1 || unmet[0] != "beta" {
		t.Fatalf("expected [beta], got %v", unmet)
	}
}

func TestValidateDependencies_VersionMismatch(t *testing.T) {
	p := makePluginDef("alpha", "1.0.0", []PluginDependency{
		{Name: "beta", Version: ">=2.0.0"},
	})

	installed := map[string]PluginDef{
		"beta": makePluginDef("beta", "1.5.0", nil),
	}

	unmet := p.ValidateDependencies(installed)
	if len(unmet) != 1 || unmet[0] != "beta" {
		t.Fatalf("expected [beta], got %v", unmet)
	}
}

func TestValidateDependencies_OptionalMissing(t *testing.T) {
	p := makePluginDef("alpha", "1.0.0", []PluginDependency{
		{Name: "beta", Version: ">=1.0.0", Optional: true},
	})

	unmet := p.ValidateDependencies(map[string]PluginDef{})
	if len(unmet) != 0 {
		t.Fatalf("expected no unmet deps for optional, got %v", unmet)
	}
}

func TestResolveDependencies_NoDeps(t *testing.T) {
	p := makePluginDef("alpha", "1.0.0", nil)

	order, err := ResolveDependencies(p, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 0 {
		t.Fatalf("expected empty order, got %v", order)
	}
}

func TestResolveDependencies_SingleDep(t *testing.T) {
	p := makePluginDef("alpha", "1.0.0", []PluginDependency{
		{Name: "beta", Version: ">=1.0.0"},
	})

	registry := []RegistryEntry{
		{Name: "beta", Version: "1.2.0", File: "plugins/beta.json"},
	}

	order, err := ResolveDependencies(p, registry, map[string]PluginDef{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 1 || order[0] != "beta" {
		t.Fatalf("expected [beta], got %v", order)
	}
}

func TestResolveDependencies_AlreadyInstalled(t *testing.T) {
	p := makePluginDef("alpha", "1.0.0", []PluginDependency{
		{Name: "beta", Version: ">=1.0.0"},
	})

	installed := map[string]PluginDef{
		"beta": makePluginDef("beta", "1.5.0", nil),
	}

	order, err := ResolveDependencies(p, nil, installed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 0 {
		t.Fatalf("expected empty order (already installed), got %v", order)
	}
}

func TestResolveDependencies_InstalledVersionMismatch(t *testing.T) {
	p := makePluginDef("alpha", "1.0.0", []PluginDependency{
		{Name: "beta", Version: ">=2.0.0"},
	})

	installed := map[string]PluginDef{
		"beta": makePluginDef("beta", "1.0.0", nil),
	}

	_, err := ResolveDependencies(p, nil, installed)
	if err == nil {
		t.Fatal("expected error for version mismatch")
	}

	if !strings.Contains(err.Error(), "does not satisfy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveDependencies_NotInRegistry(t *testing.T) {
	p := makePluginDef("alpha", "1.0.0", []PluginDependency{
		{Name: "beta"},
	})

	_, err := ResolveDependencies(p, nil, map[string]PluginDef{})
	if err == nil {
		t.Fatal("expected error for missing registry entry")
	}

	if !strings.Contains(err.Error(), "not found in registry") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveDependencies_OptionalMissing(t *testing.T) {
	p := makePluginDef("alpha", "1.0.0", []PluginDependency{
		{Name: "beta", Optional: true},
	})

	order, err := ResolveDependencies(p, nil, map[string]PluginDef{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 0 {
		t.Fatalf("expected empty order for optional missing, got %v", order)
	}
}

func TestResolveDependenciesDeep_TransitiveDeps(t *testing.T) {
	// A depends on B, B depends on C.
	c := makePluginDef("c", "1.0.0", nil)
	b := makePluginDef("b", "1.0.0", []PluginDependency{
		{Name: "c", Version: ">=1.0.0"},
	})
	a := makePluginDef("a", "1.0.0", []PluginDependency{
		{Name: "b", Version: ">=1.0.0"},
	})

	available := map[string]PluginDef{
		"b": b,
		"c": c,
	}

	order, err := ResolveDependenciesDeep(a, available, map[string]PluginDef{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 2 {
		t.Fatalf("expected 2 deps, got %v", order)
	}

	// C must come before B (install order).
	if order[0] != "c" || order[1] != "b" {
		t.Fatalf("expected [c, b], got %v", order)
	}
}

func TestResolveDependenciesDeep_CircularDependency(t *testing.T) {
	// A depends on B, B depends on A.
	a := makePluginDef("a", "1.0.0", []PluginDependency{
		{Name: "b"},
	})
	b := makePluginDef("b", "1.0.0", []PluginDependency{
		{Name: "a"},
	})

	available := map[string]PluginDef{
		"a": a,
		"b": b,
	}

	_, err := ResolveDependenciesDeep(a, available, map[string]PluginDef{})
	if err == nil {
		t.Fatal("expected error for circular dependency")
	}

	if !strings.Contains(err.Error(), "circular dependency") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveDependenciesDeep_VersionMismatch(t *testing.T) {
	a := makePluginDef("a", "1.0.0", []PluginDependency{
		{Name: "b", Version: ">=3.0.0"},
	})
	b := makePluginDef("b", "2.0.0", nil)

	available := map[string]PluginDef{
		"b": b,
	}

	_, err := ResolveDependenciesDeep(a, available, map[string]PluginDef{})
	if err == nil {
		t.Fatal("expected error for version mismatch")
	}

	if !strings.Contains(err.Error(), "does not satisfy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveDependenciesDeep_OptionalMissingNoError(t *testing.T) {
	a := makePluginDef("a", "1.0.0", []PluginDependency{
		{Name: "b", Optional: true},
		{Name: "c", Version: ">=1.0.0"},
	})
	c := makePluginDef("c", "1.0.0", nil)

	available := map[string]PluginDef{
		"c": c,
	}

	order, err := ResolveDependenciesDeep(a, available, map[string]PluginDef{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 1 || order[0] != "c" {
		t.Fatalf("expected [c], got %v", order)
	}
}

func TestResolveDependenciesDeep_NoDeps(t *testing.T) {
	a := makePluginDef("a", "1.0.0", nil)

	order, err := ResolveDependenciesDeep(a, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 0 {
		t.Fatalf("expected empty order, got %v", order)
	}
}

func TestPluginDef_DependenciesJSON(t *testing.T) {
	// Verify that dependencies round-trip through JSON and that
	// plugins without dependencies produce no "dependencies" key.
	p := makePluginDef("alpha", "1.0.0", []PluginDependency{
		{Name: "beta", Version: ">=1.0.0", Optional: true},
	})

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded PluginDef
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.Dependencies) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(decoded.Dependencies))
	}

	if decoded.Dependencies[0].Name != "beta" {
		t.Fatalf("expected dep name beta, got %s", decoded.Dependencies[0].Name)
	}

	// Plugin without deps should not emit "dependencies" in JSON.
	p2 := makePluginDef("gamma", "1.0.0", nil)

	data2, err := json.Marshal(p2)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(data2), `"dependencies"`) {
		t.Fatal("expected no dependencies key in JSON for plugin without deps")
	}
}
