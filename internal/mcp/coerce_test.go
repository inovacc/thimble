package mcp

import (
	"encoding/json"
	"testing"
)

func TestCoerceJSONArrayNormalPassthrough(t *testing.T) {
	input := []any{"hello", "world"}

	result := coerceJSONArray(input)
	if len(result) != 2 || result[0] != "hello" || result[1] != "world" {
		t.Errorf("expected [hello world], got %v", result)
	}
}

func TestCoerceJSONArrayDoubleSerialized(t *testing.T) {
	input := `["query1", "query2", "query3"]`

	result := coerceJSONArray(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(result))
	}

	if result[0] != "query1" || result[2] != "query3" {
		t.Errorf("unexpected values: %v", result)
	}
}

func TestCoerceJSONArrayStringSlice(t *testing.T) {
	input := []string{"a", "b"}

	result := coerceJSONArray(input)
	if len(result) != 2 || result[0] != "a" {
		t.Errorf("expected [a b], got %v", result)
	}
}

func TestCoerceJSONArrayNil(t *testing.T) {
	result := coerceJSONArray(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestCoerceJSONArrayInvalidString(t *testing.T) {
	result := coerceJSONArray("not an array")
	if result != nil {
		t.Errorf("expected nil for non-array string, got %v", result)
	}
}

func TestBatchCommandUnmarshalString(t *testing.T) {
	var bc batchCommand
	if err := json.Unmarshal([]byte(`"ls -la"`), &bc); err != nil {
		t.Fatalf("unmarshal string: %v", err)
	}

	if bc.Command != "ls -la" {
		t.Errorf("Command = %q, want %q", bc.Command, "ls -la")
	}

	if bc.Label != "" {
		t.Errorf("Label = %q, want empty", bc.Label)
	}
}

func TestBatchCommandUnmarshalObject(t *testing.T) {
	var bc batchCommand
	if err := json.Unmarshal([]byte(`{"command":"ls","label":"test"}`), &bc); err != nil {
		t.Fatalf("unmarshal object: %v", err)
	}

	if bc.Command != "ls" {
		t.Errorf("Command = %q, want %q", bc.Command, "ls")
	}

	if bc.Label != "test" {
		t.Errorf("Label = %q, want %q", bc.Label, "test")
	}
}

func TestBatchCommandUnmarshalMixed(t *testing.T) {
	raw := `["ls -la", {"command":"pwd","label":"home"}]`

	var cmds []batchCommand
	if err := json.Unmarshal([]byte(raw), &cmds); err != nil {
		t.Fatalf("unmarshal mixed: %v", err)
	}

	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}

	if cmds[0].Command != "ls -la" {
		t.Errorf("cmds[0].Command = %q, want %q", cmds[0].Command, "ls -la")
	}

	if cmds[0].Label != "" {
		t.Errorf("cmds[0].Label = %q, want empty", cmds[0].Label)
	}

	if cmds[1].Command != "pwd" {
		t.Errorf("cmds[1].Command = %q, want %q", cmds[1].Command, "pwd")
	}

	if cmds[1].Label != "home" {
		t.Errorf("cmds[1].Label = %q, want %q", cmds[1].Label, "home")
	}
}

func TestBatchCommandUnmarshalInvalid(t *testing.T) {
	// Neither a string nor a valid object — should return an error.
	var bc batchCommand

	err := json.Unmarshal([]byte(`123`), &bc)
	if err == nil {
		t.Error("expected error for invalid JSON type (number)")
	}
}

func TestBatchCommandUnmarshalEmptyObject(t *testing.T) {
	var bc batchCommand
	if err := json.Unmarshal([]byte(`{}`), &bc); err != nil {
		t.Fatalf("unmarshal empty object: %v", err)
	}

	if bc.Command != "" {
		t.Errorf("Command = %q, want empty", bc.Command)
	}

	if bc.Label != "" {
		t.Errorf("Label = %q, want empty", bc.Label)
	}
}

func TestBatchCommandUnmarshalInvalidJSON(t *testing.T) {
	var bc batchCommand

	err := json.Unmarshal([]byte(`{invalid`), &bc)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestCoerceJSONArrayMixedTypes(t *testing.T) {
	// []any with non-string elements should skip them.
	input := []any{"hello", 42, "world", true}

	result := coerceJSONArray(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 elements, got %d: %v", len(result), result)
	}

	if result[0] != "hello" || result[1] != "world" {
		t.Errorf("unexpected values: %v", result)
	}
}

func TestCoerceJSONArrayEmptySlice(t *testing.T) {
	result := coerceJSONArray([]any{})
	if len(result) != 0 {
		t.Errorf("expected 0 elements, got %d", len(result))
	}
}

func TestCoerceJSONArrayStringStartingWithBracketButInvalid(t *testing.T) {
	// String that starts with "[" but is not valid JSON.
	result := coerceJSONArray("[not valid json")
	if result != nil {
		t.Errorf("expected nil for invalid JSON array string, got %v", result)
	}
}

func TestCoerceJSONArrayIntegerInput(t *testing.T) {
	result := coerceJSONArray(42)
	if result != nil {
		t.Errorf("expected nil for integer input, got %v", result)
	}
}
