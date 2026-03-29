package main

import (
	"encoding/json"
	"testing"
)

func TestGetVersionInfo(t *testing.T) {
	info := GetVersionInfo()
	if info == nil {
		t.Fatal("GetVersionInfo() returned nil")
	}

	if info.Version == "" {
		t.Error("Version is empty")
	}

	if info.GoOS == "" {
		t.Error("GoOS is empty")
	}

	if info.GoArch == "" {
		t.Error("GoArch is empty")
	}
}

func TestGetVersionJSON(t *testing.T) {
	s := GetVersionJSON()
	if s == "" {
		t.Fatal("GetVersionJSON() returned empty")
	}

	if !json.Valid([]byte(s)) {
		t.Errorf("GetVersionJSON() is not valid JSON: %s", s)
	}

	var info VersionInfo
	if err := json.Unmarshal([]byte(s), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if info.Version == "" {
		t.Error("version field empty in JSON output")
	}
}

func TestVersionInfoRoundTrip(t *testing.T) {
	orig := &VersionInfo{
		Version:   "v1.2.3",
		GitHash:   "abc123",
		BuildTime: "2026-01-01T00:00:00Z",
		BuildHash: "hash123",
		GoVersion: "go1.22.0",
		GoOS:      "linux",
		GoArch:    "amd64",
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var decoded VersionInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Version != orig.Version {
		t.Errorf("Version = %q, want %q", decoded.Version, orig.Version)
	}

	if decoded.GoOS != orig.GoOS {
		t.Errorf("GoOS = %q, want %q", decoded.GoOS, orig.GoOS)
	}
}
