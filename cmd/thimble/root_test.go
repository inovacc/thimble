package main

import (
	"testing"
)

func TestRootCmd_HasSubcommands(t *testing.T) {
	cmds := rootCmd.Commands()
	if len(cmds) == 0 {
		t.Fatal("rootCmd has no subcommands")
	}

	want := map[string]bool{
		"doctor":  false,
		"hook":    false,
		"setup":   false,
		"report":  false,
		"upgrade": false,
		"version": false,
	}

	for _, c := range cmds {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("expected subcommand %q not found", name)
		}
	}
}

func TestRootCmd_UseString(t *testing.T) {
	if rootCmd.Use != "thimble" {
		t.Errorf("Use = %q, want %q", rootCmd.Use, "thimble")
	}
}

func TestRootCmd_HasVersion(t *testing.T) {
	v := rootCmd.Version
	if v == "" {
		t.Error("rootCmd.Version is empty")
	}
}

func TestRootCmd_CompletionDisabled(t *testing.T) {
	if !rootCmd.CompletionOptions.DisableDefaultCmd {
		t.Error("completion default command should be disabled")
	}
}

func TestVersionCmd_HasJSONFlag(t *testing.T) {
	f := versionCmd.Flags().Lookup("json")
	if f == nil {
		t.Fatal("version command missing --json flag")
	}

	if f.Shorthand != "j" {
		t.Errorf("json flag shorthand = %q, want %q", f.Shorthand, "j")
	}
}
