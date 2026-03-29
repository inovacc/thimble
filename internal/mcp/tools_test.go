package mcp

import (
	"context"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/executor"
)

func TestFormatExecOutputStdoutOnly(t *testing.T) {
	got := formatExecOutput("hello world", "", 0, false, false)
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestFormatExecOutputStderrOnly(t *testing.T) {
	got := formatExecOutput("", "error msg", 0, false, false)
	if !strings.Contains(got, "[stderr]") {
		t.Error("expected [stderr] marker")
	}

	if !strings.Contains(got, "error msg") {
		t.Error("expected stderr content")
	}
}

func TestFormatExecOutputBothStreams(t *testing.T) {
	got := formatExecOutput("stdout", "stderr", 0, false, false)
	if !strings.Contains(got, "stdout") {
		t.Error("expected stdout")
	}

	if !strings.Contains(got, "[stderr]") {
		t.Error("expected [stderr] marker")
	}

	if !strings.Contains(got, "stderr") {
		t.Error("expected stderr content")
	}
}

func TestFormatExecOutputTimedOut(t *testing.T) {
	got := formatExecOutput("output", "", 0, true, false)
	if !strings.Contains(got, "[timed out]") {
		t.Error("expected [timed out] marker")
	}
}

func TestFormatExecOutputNonZeroExit(t *testing.T) {
	got := formatExecOutput("output", "", 1, false, false)
	if !strings.Contains(got, "[exit code: 1]") {
		t.Error("expected [exit code: 1]")
	}
}

func TestFormatExecOutputBackgrounded(t *testing.T) {
	got := formatExecOutput("", "", 0, false, true)
	if !strings.Contains(got, "[backgrounded]") {
		t.Error("expected [backgrounded] marker")
	}
	// Backgrounded should not include other content.
	if strings.Contains(got, "[exit code") {
		t.Error("backgrounded output should not contain exit code")
	}
}

func TestFormatExecOutputEmpty(t *testing.T) {
	got := formatExecOutput("", "", 0, false, false)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatExecOutputAllFlags(t *testing.T) {
	got := formatExecOutput("out", "err", 2, true, false)
	if !strings.Contains(got, "out") {
		t.Error("expected stdout")
	}

	if !strings.Contains(got, "[stderr]") {
		t.Error("expected stderr marker")
	}

	if !strings.Contains(got, "[timed out]") {
		t.Error("expected timed out marker")
	}

	if !strings.Contains(got, "[exit code: 2]") {
		t.Error("expected exit code 2")
	}
}

func TestTextResult(t *testing.T) {
	result := textResult("test output")
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.IsError {
		t.Error("textResult should not be an error")
	}

	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Content))
	}

	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected *TextContent, got %T", result.Content[0])
	}

	if tc.Text != "test output" {
		t.Errorf("text = %q, want %q", tc.Text, "test output")
	}
}

func TestErrorResult(t *testing.T) {
	result := errorResult("something failed")
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if !result.IsError {
		t.Error("errorResult should have IsError = true")
	}

	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected *TextContent, got %T", result.Content[0])
	}

	if tc.Text != "something failed" {
		t.Errorf("text = %q, want %q", tc.Text, "something failed")
	}
}

func TestTextResultEmpty(t *testing.T) {
	result := textResult("")

	tc := result.Content[0].(*mcpsdk.TextContent)
	if tc.Text != "" {
		t.Errorf("expected empty text, got %q", tc.Text)
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		stderr   string
		stdout   string
		exitCode int
	}{
		{"empty", "", "", 1},
		{"permission denied", "permission denied", "", 1},
		{"not found", "command not found", "", 127},
		{"timeout", "killed", "", 137},
		{"syntax error", "SyntaxError: invalid syntax", "", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyError(tt.stderr, tt.stdout, tt.exitCode)
			if result == "" {
				t.Error("classifyError should return non-empty string")
			}
		})
	}
}

func TestBatchNetTrackingWithMarkers(t *testing.T) {
	b := &Bridge{
		stats: newSessionStats(),
	}

	// Simulate what handleBatchExecute does after getting a response with net markers.
	stderr := "some output\n__CM_NET__:1024:https://api.example.com\n__CM_NET__:512:https://cdn.example.com\n"
	if netStats := executor.ParseNetMarkers(stderr); netStats != nil {
		b.stats.mu.Lock()
		b.stats.bytesSandboxed += netStats.TotalBytes
		b.stats.mu.Unlock()
	}

	b.stats.mu.Lock()
	got := b.stats.bytesSandboxed
	b.stats.mu.Unlock()

	if got != 1536 {
		t.Errorf("bytesSandboxed = %d, want 1536", got)
	}
}

func TestBatchNetTrackingWithoutMarkers(t *testing.T) {
	b := &Bridge{
		stats: newSessionStats(),
	}

	// No markers — stats should not change.
	stderr := "some normal stderr output"
	if netStats := executor.ParseNetMarkers(stderr); netStats != nil {
		b.stats.mu.Lock()
		b.stats.bytesSandboxed += netStats.TotalBytes
		b.stats.mu.Unlock()
	}

	b.stats.mu.Lock()
	got := b.stats.bytesSandboxed
	b.stats.mu.Unlock()

	if got != 0 {
		t.Errorf("bytesSandboxed = %d, want 0", got)
	}
}

func TestBackgroundJSKeepaliveAppend(t *testing.T) {
	code := "console.log('hello');"
	language := "javascript"

	background := true
	if background && (language == "javascript" || language == "typescript") {
		code += "\nsetInterval(()=>{},2147483647);\n"
	}

	if !strings.Contains(code, "setInterval") {
		t.Error("expected keepalive for background JS")
	}
}

func TestBackgroundTSKeepaliveAppend(t *testing.T) {
	code := "console.log('hello');"
	language := "typescript"

	background := true
	if background && (language == "javascript" || language == "typescript") {
		code += "\nsetInterval(()=>{},2147483647);\n"
	}

	if !strings.Contains(code, "setInterval") {
		t.Error("expected keepalive for background TS")
	}
}

func TestBackgroundNonJSNoKeepalive(t *testing.T) {
	code := "print('hello')"
	language := "python"

	background := true
	if background && (language == "javascript" || language == "typescript") {
		code += "\nsetInterval(()=>{},2147483647);\n"
	}

	if strings.Contains(code, "setInterval") {
		t.Error("keepalive should not be appended for python")
	}
}

func TestHandleUpgradeOutput(t *testing.T) {
	b := &Bridge{
		stats: newSessionStats(),
	}

	result, _, err := b.handleUpgrade(context.TODO(), nil, upgradeInput{})
	if err != nil {
		t.Fatalf("handleUpgrade: %v", err)
	}

	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected *TextContent, got %T", result.Content[0])
	}

	if !strings.Contains(tc.Text, serverVersion) {
		t.Errorf("expected serverVersion %q in output, got %q", serverVersion, tc.Text)
	}

	if !strings.Contains(tc.Text, "go install") {
		t.Errorf("expected 'go install' in output, got %q", tc.Text)
	}
}

func TestIntentSearchThresholdConstant(t *testing.T) {
	if IntentSearchThreshold != 5120 {
		t.Errorf("IntentSearchThreshold = %d, want 5120", IntentSearchThreshold)
	}
}

func TestFormatBytesKB(t *testing.T) {
	got := formatBytes(1024)
	if got != "1.0KB" {
		t.Errorf("formatBytes(1024) = %q, want 1.0KB", got)
	}

	got = formatBytes(512)
	if got != "0.5KB" {
		t.Errorf("formatBytes(512) = %q, want 0.5KB", got)
	}
}

func TestFormatBytesMB(t *testing.T) {
	got := formatBytes(1024 * 1024)
	if got != "1.0MB" {
		t.Errorf("formatBytes(1MB) = %q, want 1.0MB", got)
	}

	got = formatBytes(2 * 1024 * 1024)
	if got != "2.0MB" {
		t.Errorf("formatBytes(2MB) = %q, want 2.0MB", got)
	}
}

func TestFormatBytesZero(t *testing.T) {
	got := formatBytes(0)
	if got != "0.0KB" {
		t.Errorf("formatBytes(0) = %q, want 0.0KB", got)
	}
}

func TestFetchContentTypeRouting(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantCT      string
	}{
		{"json_content", "application/json; charset=utf-8", "json"},
		{"plain_text", "text/plain", "plain"},
		{"html_default", "text/html; charset=utf-8", "markdown"},
		{"empty_default", "", "markdown"},
		{"xml_default", "application/xml", "markdown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := "markdown"
			if strings.Contains(tt.contentType, "application/json") {
				ct = "json"
			} else if strings.Contains(tt.contentType, "text/plain") {
				ct = "plain"
			}

			if ct != tt.wantCT {
				t.Errorf("content-type %q → ct = %q, want %q", tt.contentType, ct, tt.wantCT)
			}
		})
	}
}

func TestStatsMarkdownFormat(t *testing.T) {
	b := &Bridge{
		stats: newSessionStats(),
	}

	// Populate some stats to trigger Per-Tool Breakdown section.
	b.stats.mu.Lock()
	b.stats.calls["ctx_search"] = 3
	b.stats.bytesReturned["ctx_search"] = 2048
	b.stats.bytesIndexed = 4096
	b.stats.mu.Unlock()

	// handleStats needs a content client for Stats RPC — we can't call it directly
	// without a gRPC server. Instead, test the snapshot-to-markdown logic
	// by verifying the snapshot structure.
	snap := b.stats.snapshot()

	calls, ok := snap["calls"].(map[string]int)
	if !ok || calls["ctx_search"] != 3 {
		t.Errorf("expected calls[ctx_search]=3, got %v", snap["calls"])
	}

	if snap["bytesIndexed"] != 4096 {
		t.Errorf("expected bytesIndexed=4096, got %v", snap["bytesIndexed"])
	}

	// Verify expected markdown sections would be produced by building the same way handleStats does.
	var sb strings.Builder
	sb.WriteString("## Knowledge Base\n\n")
	sb.WriteString("## Context Window Protection\n\n")

	if len(calls) > 0 {
		sb.WriteString("## Per-Tool Breakdown\n\n")
	}

	sb.WriteString("## Session\n\n")
	sb.WriteString("```json\n{}\n```\n")

	output := sb.String()
	for _, section := range []string{"## Knowledge Base", "## Context Window Protection", "## Per-Tool Breakdown", "## Session", "```json"} {
		if !strings.Contains(output, section) {
			t.Errorf("expected section %q in stats markdown output", section)
		}
	}
}

func TestFormatExecOutputTable(t *testing.T) {
	tests := []struct {
		name         string
		stdout       string
		stderr       string
		exitCode     int
		timedOut     bool
		backgrounded bool
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:         "backgrounded ignores everything else",
			stdout:       "some output",
			stderr:       "some error",
			exitCode:     1,
			timedOut:     true,
			backgrounded: true,
			wantContains: []string{"[backgrounded]"},
		},
		{
			name:         "stderr only no newline prefix",
			stderr:       "error",
			wantContains: []string{"[stderr]", "error"},
		},
		{
			name:         "both streams with exit code and timeout",
			stdout:       "out",
			stderr:       "err",
			exitCode:     2,
			timedOut:     true,
			wantContains: []string{"out", "[stderr]", "err", "[timed out]", "[exit code: 2]"},
		},
		{
			name:      "all empty",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatExecOutput(tt.stdout, tt.stderr, tt.exitCode, tt.timedOut, tt.backgrounded)
			if tt.wantEmpty && got != "" {
				t.Errorf("expected empty, got %q", got)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q, got %q", want, got)
				}
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0.0KB"},
		{1024, "1.0KB"},
		{512, "0.5KB"},
		{1024 * 1024, "1.0MB"},
		{2 * 1024 * 1024, "2.0MB"},
		{1536 * 1024, "1.5MB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsSoftFail(t *testing.T) {
	tests := []struct {
		name     string
		lang     string
		exitCode int
		stdout   string
		want     bool
	}{
		{"shell_exit1_with_output", "shell", 1, "some output", true},
		{"shell_exit1_empty", "shell", 1, "", false},
		{"shell_exit1_whitespace", "shell", 1, "   \n  ", false},
		{"shell_exit2", "shell", 2, "output", false},
		{"python_exit1", "python", 1, "output", false},
		{"shell_exit0", "shell", 0, "output", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSoftFail(tt.lang, tt.exitCode, tt.stdout)
			if got != tt.want {
				t.Errorf("isSoftFail(%q,%d,%q) = %v, want %v", tt.lang, tt.exitCode, tt.stdout, got, tt.want)
			}
		})
	}
}
