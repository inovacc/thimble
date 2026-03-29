package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersion_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	versionCmd.SetOut(buf)
	versionCmd.SetErr(buf)

	// Set JSON flag.
	if err := versionCmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("set flag: %v", err)
	}

	defer func() {
		_ = versionCmd.Flags().Set("json", "false")
	}()

	versionCmd.Run(versionCmd, nil)
}

func TestPrintVersion(t *testing.T) {
	// printVersion writes to stdout — just verify it doesn't panic.
	printVersion()
}

func TestVersionVariablesDefaults(t *testing.T) {
	// Version vars should have sensible defaults or be populated by init().
	if Version == "" {
		t.Error("Version should not be empty")
	}

	if GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}

	if GOOS == "" {
		t.Error("GOOS should not be empty")
	}

	if GOARCH == "" {
		t.Error("GOARCH should not be empty")
	}
}

func TestGetVersionJSON_ContainsFields(t *testing.T) {
	s := GetVersionJSON()

	for _, field := range []string{"version", "git_hash", "build_time", "go_version", "goos", "goarch"} {
		if !strings.Contains(s, field) {
			t.Errorf("GetVersionJSON() missing field %q", field)
		}
	}
}
