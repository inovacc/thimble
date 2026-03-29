package executor

import (
	"context"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestExecuteShell(t *testing.T) {
	pe := New(nil)

	result, err := pe.Execute(context.Background(), "shell", "echo hello world", 10*time.Second, false)
	if err != nil {
		t.Fatalf("Execute shell: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}

	if !strings.Contains(result.Stdout, "hello world") {
		t.Errorf("stdout = %q, want 'hello world'", result.Stdout)
	}
}

func TestExecutePython(t *testing.T) {
	pe := New(nil)
	if pe.runtimes["python"] == "" {
		t.Skip("python not available")
	}

	result, err := pe.Execute(context.Background(), "python", "print('hello from python')", 10*time.Second, false)
	if err != nil {
		t.Fatalf("Execute python: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}

	if !strings.Contains(result.Stdout, "hello from python") {
		t.Errorf("stdout = %q", result.Stdout)
	}
}

func TestExecuteTimeout(t *testing.T) {
	pe := New(nil)

	result, err := pe.Execute(context.Background(), "shell", "sleep 30", 1*time.Second, false)
	if err != nil {
		t.Fatalf("Execute timeout: %v", err)
	}

	if !result.TimedOut {
		t.Error("expected timedOut = true")
	}

	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code on timeout")
	}
}

func TestExecuteError(t *testing.T) {
	pe := New(nil)

	result, err := pe.Execute(context.Background(), "shell", "exit 42", 10*time.Second, false)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}

func TestSmartTruncate(t *testing.T) {
	short := "hello"
	if smartTruncate(short, 100) != short {
		t.Error("short string should not be truncated")
	}

	long := strings.Repeat("line\n", 1000)

	truncated := smartTruncate(long, 500)
	if len(truncated) > 600 { // some overhead for separator
		t.Errorf("truncated len = %d, want <= ~600", len(truncated))
	}

	if !strings.Contains(truncated, "truncated") {
		t.Error("expected truncation marker")
	}
}

func TestBuildCommandUnavailableRuntime(t *testing.T) {
	empty := RuntimeMap{}

	_, err := BuildCommand(empty, "python", "script.py")
	if err == nil {
		t.Fatal("expected error for unavailable runtime")
	}

	var re *RuntimeError

	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("error = %q, want 'not available'", err.Error())
	}

	_ = re
}

func TestDetectRuntimes(t *testing.T) {
	runtimes := DetectRuntimes()
	// Shell should always be available.
	if runtimes["shell"] == "" {
		t.Error("shell runtime should always be detected")
	}
	// JavaScript should always be available (node or bun).
	if runtimes["javascript"] == "" {
		t.Error("javascript runtime should always be detected")
	}
}

func TestExecuteStreamShell(t *testing.T) {
	pe := New(nil)

	var (
		chunks []OutputChunk
		mu     sync.Mutex
	)

	result, err := pe.ExecuteStream(context.Background(), "shell", "echo line1\necho line2\necho line3", 10*time.Second, func(chunk OutputChunk) error {
		mu.Lock()
		defer mu.Unlock()

		chunks = append(chunks, chunk)

		return nil
	})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(chunks) < 3 {
		t.Errorf("got %d chunks, want at least 3", len(chunks))
	}

	// All chunks should be stdout.
	for _, c := range chunks {
		if c.Stream != "stdout" {
			t.Errorf("chunk stream = %q, want stdout", c.Stream)
		}
	}

	// Verify the final result has all output.
	if !strings.Contains(result.Stdout, "line1") || !strings.Contains(result.Stdout, "line3") {
		t.Errorf("stdout = %q, want line1..line3", result.Stdout)
	}
}

func TestExecuteStreamStderr(t *testing.T) {
	pe := New(nil)

	var (
		chunks []OutputChunk
		mu     sync.Mutex
	)

	result, err := pe.ExecuteStream(context.Background(), "shell", "echo out1\necho err1 >&2\necho out2", 10*time.Second, func(chunk OutputChunk) error {
		mu.Lock()
		defer mu.Unlock()

		chunks = append(chunks, chunk)

		return nil
	})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d", result.ExitCode)
	}

	mu.Lock()
	defer mu.Unlock()

	// Should have both stdout and stderr chunks.
	var hasStdout, hasStderr bool

	for _, c := range chunks {
		if c.Stream == "stdout" {
			hasStdout = true
		}

		if c.Stream == "stderr" {
			hasStderr = true
		}
	}

	if !hasStdout {
		t.Error("expected at least one stdout chunk")
	}

	if !hasStderr {
		t.Error("expected at least one stderr chunk")
	}
}

func TestExecuteStreamTimeout(t *testing.T) {
	pe := New(nil)

	result, err := pe.ExecuteStream(context.Background(), "shell", "sleep 30", 1*time.Second, func(_ OutputChunk) error {
		return nil
	})
	if err != nil {
		t.Fatalf("ExecuteStream timeout: %v", err)
	}

	if !result.TimedOut {
		t.Error("expected timedOut = true")
	}

	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code on timeout")
	}
}

func TestGetAvailableLanguages(t *testing.T) {
	runtimes := RuntimeMap{
		"javascript": "node",
		"shell":      "bash",
		"python":     "python3",
	}

	langs := GetAvailableLanguages(runtimes)
	if len(langs) < 3 {
		t.Errorf("languages = %v, want at least 3", langs)
	}
}

func TestEnvDenyListBlocksDangerousVars(t *testing.T) {
	pe := New(nil)

	dangerous := []string{
		"GIT_SSH", "GIT_SSH_COMMAND", "GIT_ASKPASS", "GIT_CONFIG_COUNT",
		"PYTHONSTARTUP", "PYTHONHOME", "PYTHONBREAKPOINT", "PYTHONINSPECT",
		"RUSTC", "RUSTC_WRAPPER", "RUSTFLAGS", "RUSTDOCFLAGS",
		"CARGO_BUILD_RUSTC", "CARGO_BUILD_RUSTC_WRAPPER",
		"PS4", "PROMPT_COMMAND", "BASH_ENV", "SHELLOPTS", "BASHOPTS",
		"BASH_XTRACEFD",
		"LD_PRELOAD", "DYLD_INSERT_LIBRARIES",
		"NODE_OPTIONS", "NODE_PATH",
		"PERL5OPT", "PERL5DB", "RUBYOPT", "PERLLIB",
		"PHP_INI_SCAN_DIR", "PHPRC",
		"R_PROFILE", "R_PROFILE_USER", "R_HOME",
		"ERL_AFLAGS", "ERL_FLAGS", "ELIXIR_ERL_OPTIONS",
		"OPENSSL_CONF", "OPENSSL_ENGINES",
		"GOFLAGS", "CGO_CFLAGS", "CGO_LDFLAGS",
		"CC", "CXX", "AR",
	}

	for _, key := range dangerous {
		t.Setenv(key, "/evil/path")
	}

	env := pe.buildSafeEnv(os.TempDir())
	envMap := make(map[string]string)

	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
	}

	for _, key := range dangerous {
		if _, found := envMap[key]; found {
			t.Errorf("denied env var %q leaked through", key)
		}
	}
}

func TestEnvDenyPrefixBlocking(t *testing.T) {
	pe := New(nil)

	t.Setenv("BASH_FUNC_evil%%", "() { evil; }")
	t.Setenv("BASH_FUNC_sneaky%%", "() { sneaky; }")

	env := pe.buildSafeEnv(os.TempDir())
	envMap := make(map[string]string)

	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
	}

	for k := range envMap {
		if strings.HasPrefix(k, "BASH_FUNC_") {
			t.Errorf("BASH_FUNC_ prefix var %q leaked through", k)
		}
	}
}

func TestEnvPassthroughNotBlocked(t *testing.T) {
	pe := New(nil)

	t.Setenv("GH_TOKEN", "test-token")
	t.Setenv("GOPATH", "/go")

	env := pe.buildSafeEnv(os.TempDir())
	envMap := make(map[string]string)

	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
	}

	if envMap["GH_TOKEN"] != "test-token" {
		t.Error("GH_TOKEN should pass through")
	}

	if envMap["GOPATH"] != "/go" {
		t.Error("GOPATH should pass through")
	}
}

func TestCappedWriterExceedsHardCap(t *testing.T) {
	stdout, stderr := newCappedWriterPair(100)

	_, _ = stdout.Write([]byte(strings.Repeat("a", 60)))
	_, _ = stderr.Write([]byte(strings.Repeat("b", 60)))

	if !stdout.exceeded.Load() {
		t.Error("expected exceeded flag after exceeding hard cap")
	}

	// Further writes silently dropped.
	n, err := stdout.Write([]byte("more"))
	if err != nil {
		t.Errorf("Write after exceeded should not error: %v", err)
	}

	if n != 4 {
		t.Errorf("Write after exceeded should report full len, got %d", n)
	}

	if stdout.buf.Len() != 60 {
		t.Errorf("stdout buf len = %d, want 60", stdout.buf.Len())
	}
}

func TestCappedWriterUnderCap(t *testing.T) {
	stdout, stderr := newCappedWriterPair(1000)

	_, _ = stdout.Write([]byte("hello"))
	_, _ = stderr.Write([]byte("world"))

	if stdout.exceeded.Load() {
		t.Error("should not be exceeded")
	}

	if stdout.buf.String() != "hello" {
		t.Errorf("stdout = %q, want hello", stdout.buf.String())
	}

	if stderr.buf.String() != "world" {
		t.Errorf("stderr = %q, want world", stderr.buf.String())
	}
}

func TestCACertDetection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CA cert detection is Unix-only")
	}
	// Just verify it doesn't panic and returns a string.
	path := detectCACertPath()
	_ = path // may be empty on minimal systems
}

func TestElixirEbinPathsNoMixProject(t *testing.T) {
	// Non-existent path should return empty.
	result := detectElixirEbinPaths("/nonexistent/path/12345")
	if result != "" {
		t.Errorf("expected empty for non-existent path, got %q", result)
	}
}

func TestIsCmdShim(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	// "where.exe" exists on Windows and is not a .cmd shim.
	if isCmdShim("where") {
		t.Error("where.exe should not be a .cmd shim")
	}
}

func TestWrapCmdShimNoOp(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("wrapCmdShim is Windows-specific")
	}

	cmd := []string{"node", "script.js"}
	wrapped := wrapCmdShim(cmd)
	// On Windows, wrapCmdShim should prepend cmd.exe /C.
	if len(wrapped) != len(cmd)+2 {
		t.Errorf("wrapCmdShim should prepend cmd.exe /C, got %v", wrapped)
	}
}

func TestSmartTruncateInformativeFormat(t *testing.T) {
	// Build output with enough lines to trigger truncation.
	var sb strings.Builder
	for range 200 {
		sb.WriteString("line content here\n")
	}

	raw := sb.String()
	truncated := smartTruncate(raw, 500)

	// Should contain truncation marker with byte count.
	if !strings.Contains(truncated, "truncated") {
		t.Error("truncation separator should mention truncated")
	}

	if !strings.Contains(truncated, "bytes") {
		t.Error("truncation separator should mention bytes")
	}
}
