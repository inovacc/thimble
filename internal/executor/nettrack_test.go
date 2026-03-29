package executor

import "testing"

func TestCleanNetMarkers_NoMarkers(t *testing.T) {
	input := "line1\nline2\nline3"
	got := CleanNetMarkers(input)

	if got != input {
		t.Errorf("CleanNetMarkers with no markers should return input unchanged\ngot:  %q\nwant: %q", got, input)
	}
}

func TestCleanNetMarkers_WithMarkers(t *testing.T) {
	input := "normal output\n__CM_NET__:1234:https://example.com\nmore output\n__CM_NET__:5678:https://api.test.com\nfinal line"

	got := CleanNetMarkers(input)

	want := "normal output\nmore output\nfinal line"
	if got != want {
		t.Errorf("CleanNetMarkers =\n%q\nwant:\n%q", got, want)
	}
}

func TestCleanNetMarkers_AllMarkers(t *testing.T) {
	input := "__CM_NET__:100:http://a.com\n__CM_NET__:200:http://b.com"

	got := CleanNetMarkers(input)

	// All lines removed, only empty join remains.
	want := ""
	if got != want {
		t.Errorf("CleanNetMarkers all markers = %q, want %q", got, want)
	}
}

func TestCleanNetMarkers_EmptyInput(t *testing.T) {
	got := CleanNetMarkers("")
	if got != "" {
		t.Errorf("CleanNetMarkers empty = %q, want empty", got)
	}
}

func TestCleanNetMarkers_PreservesEmptyLines(t *testing.T) {
	input := "line1\n\nline3"
	got := CleanNetMarkers(input)

	if got != input {
		t.Errorf("CleanNetMarkers should preserve empty lines\ngot:  %q\nwant: %q", got, input)
	}
}
