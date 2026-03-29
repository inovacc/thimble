package main

import "testing"

func TestCheckResultString(t *testing.T) {
	tests := []struct {
		name   string
		result checkResult
		icon   string
	}{
		{
			name:   "pass",
			result: checkResult{name: "Go version", status: "pass", message: "go1.22.0"},
			icon:   "[OK]",
		},
		{
			name:   "fail",
			result: checkResult{name: "SQLite", status: "fail", message: "not available"},
			icon:   "[FAIL]",
		},
		{
			name:   "warn",
			result: checkResult{name: "Ruby", status: "warn", message: "not found"},
			icon:   "[WARN]",
		},
		{
			name:   "unknown",
			result: checkResult{name: "Test", status: "other", message: "???"},
			icon:   "[??]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.String()
			if got == "" {
				t.Fatal("String() returned empty")
			}

			if !containsStr(got, tt.icon) {
				t.Errorf("String() = %q, want icon %q", got, tt.icon)
			}

			if !containsStr(got, tt.result.name) {
				t.Errorf("String() = %q, want name %q", got, tt.result.name)
			}

			if !containsStr(got, tt.result.message) {
				t.Errorf("String() = %q, want message %q", got, tt.result.message)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && findStr(s, substr)
}

func findStr(s, substr string) bool {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
