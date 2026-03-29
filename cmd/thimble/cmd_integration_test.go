package main

import (
	"bytes"
	"testing"
)

func TestSetupCmd_DryRunClaude(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"setup", "--client", "claude", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
}

func TestSetupCmd_DryRunGemini(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"setup", "--client", "gemini", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
}

func TestSetupCmd_DryRunCursor(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"setup", "--client", "cursor", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
}

func TestSetupCmd_DryRunOpenCode(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"setup", "--client", "opencode", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
}

func TestSetupCmd_DryRunVSCode(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"setup", "--client", "vscode", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
}

func TestSetupCmd_DryRunCodex(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"setup", "--client", "codex", "--dry-run"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
}

func TestSetupCmd_DryRunWithInstructions(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"setup", "--client", "claude", "--dry-run", "--instructions"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
}

func TestSetupCmd_DryRunWithPluginFlag(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"setup", "--client", "claude", "--dry-run", "--plugin"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
}

func TestSetupCmd_InvalidClient(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"setup", "--client", "invalid-platform"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid client")
	}
}

func TestVersionCmd_Execute(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
}

func TestVersionCmd_ExecuteJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
}

func TestDoctorCmd_Execute(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"doctor"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
}

func TestHookCmd_MissingArgs(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"hook"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("hook without args should fail")
	}
}
