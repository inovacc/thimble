package mcp

import (
	"testing"
)

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"string", "hello", `"hello"`},
		{"number", 42, "42"},
		{"map", map[string]string{"a": "b"}, "{\n  \"a\": \"b\"\n}"},
		{"nil", nil, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := marshalJSON(tt.input)
			if got != tt.want {
				t.Errorf("marshalJSON(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMarshalJSON_Error(t *testing.T) {
	// Channels can't be marshaled to JSON.
	ch := make(chan int)

	got := marshalJSON(ch)
	if got == "" {
		t.Error("expected error message for unmarshalable type")
	}
}
