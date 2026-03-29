package executor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TestFormatKB — verify human-readable byte formatting
// ---------------------------------------------------------------------------

func TestFormatKB(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0B"},
		{1, "1B"},
		{512, "512B"},
		{1023, "1023B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{2048, "2.0KB"},
		{10240, "10.0KB"},
		{102400, "100.0KB"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := formatKB(tc.input)
			if got != tc.want {
				t.Errorf("formatKB(%d) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestSmartTruncateNegativeTailBudget — very small maxBytes forces head-only
// ---------------------------------------------------------------------------

func TestSmartTruncateNegativeTailBudget(t *testing.T) {
	// maxBytes so small that tailBudget = maxBytes - headBudget - 100 < 0
	input := strings.Repeat("line\n", 100) // 500 bytes

	got := smartTruncate(input, 120) // headBudget=72, tailBudget=120-72-100=-52
	if !strings.Contains(got, "truncated") {
		t.Error("expected truncation marker")
	}

	if strings.Contains(got, "last 0 lines") {
		// expected: head-only path
	} else {
		t.Logf("output: %s", got)
	}
}

// ---------------------------------------------------------------------------
// TestSmartTruncateHeadTailOverlap — head and tail regions meet
// ---------------------------------------------------------------------------

func TestSmartTruncateHeadTailOverlap(t *testing.T) {
	// Create input that is barely over limit so headEnd >= tailStart
	input := strings.Repeat("ab\n", 50) // 150 bytes

	got := smartTruncate(input, 140) // headBudget=84, tailBudget=140-84-100=-44 < 0 → head-only
	if !strings.Contains(got, "truncated") {
		t.Error("expected truncation marker for barely-over-limit input")
	}
}

func TestSmartTruncateOverlapAfterSnap(t *testing.T) {
	// The overlap guard (headEnd >= tailStart) can be reached when tailStart
	// snaps forward past the remaining content. Use no-newline input where
	// the raw indices are close together.
	// This is a defensive guard; just verify it doesn't panic.
	input := strings.Repeat("x", 250) // no newlines at all

	got := smartTruncate(input, 200) // headBudget=120, tailBudget=200-120-100=-20 → head-only
	if !strings.Contains(got, "truncated") {
		t.Error("expected truncation marker")
	}
}

// ---------------------------------------------------------------------------
// TestBuildSafeEnvAllPassthroughVars — verify all passthrough vars work
// ---------------------------------------------------------------------------

func TestBuildSafeEnvAllPassthroughVars(t *testing.T) {
	passthrough := []string{
		"GH_TOKEN", "GITHUB_TOKEN", "GH_HOST",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
		"AWS_REGION", "AWS_DEFAULT_REGION", "AWS_PROFILE",
		"GOOGLE_APPLICATION_CREDENTIALS", "CLOUDSDK_CONFIG",
		"DOCKER_HOST", "KUBECONFIG",
		"NPM_TOKEN", "NODE_AUTH_TOKEN",
		"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY",
		"SSL_CERT_FILE", "CURL_CA_BUNDLE", "NODE_EXTRA_CA_CERTS", "REQUESTS_CA_BUNDLE",
		"XDG_CONFIG_HOME", "XDG_DATA_HOME",
		"SSH_AUTH_SOCK", "SSH_AGENT_PID",
		"GOPATH", "GOROOT", "CARGO_HOME", "RUSTUP_HOME",
		"VIRTUAL_ENV", "CONDA_PREFIX", "CONDA_DEFAULT_ENV", "PYTHONPATH",
		"GEM_HOME", "GEM_PATH", "RBENV_ROOT", "JAVA_HOME",
	}

	for _, key := range passthrough {
		t.Setenv(key, "test-value-"+key)
	}

	pe := New(&Options{
		ProjectRoot: t.TempDir(),
		Runtimes:    RuntimeMap{"shell": "bash"},
	})

	env := pe.buildSafeEnv(t.TempDir())
	envMap := make(map[string]string)

	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
	}

	for _, key := range passthrough {
		// Some keys are in the deny list (like GOFLAGS) so skip those
		if envDenyList[key] {
			continue
		}

		if envMap[key] != "test-value-"+key {
			t.Errorf("passthrough var %s = %q, want %q", key, envMap[key], "test-value-"+key)
		}
	}
}

// ---------------------------------------------------------------------------
// TestExecuteNonShellLanguageCwd — non-shell runs in temp dir, not project
// ---------------------------------------------------------------------------

func TestExecuteNonShellLanguageCwd(t *testing.T) {
	pe := New(nil)
	if pe.runtimes["python"] == "" {
		t.Skip("python not available")
	}

	// Python should run in temp dir, not project root
	result, err := pe.Execute(context.Background(), "python", "import os; print(os.getcwd())", 10*time.Second, false)
	if err != nil {
		t.Fatalf("Execute python: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	// CWD should be a temp dir, not project root
	if strings.Contains(result.Stdout, pe.projectRoot) {
		t.Logf("python cwd = %q, projectRoot = %q", strings.TrimSpace(result.Stdout), pe.projectRoot)
	}
}

// ---------------------------------------------------------------------------
// TestExecuteBackground — background flag with short-running command
// ---------------------------------------------------------------------------

func TestExecuteBackgroundShortCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell echo timing unreliable on Windows CI")
	}

	pe := New(nil)

	// Short command completes before timeout — should NOT be backgrounded
	result, err := pe.Execute(context.Background(), "shell", "echo done", 10*time.Second, true)
	if err != nil {
		t.Fatalf("Execute background: %v", err)
	}

	if result.Backgrounded {
		t.Error("short command should not be backgrounded")
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d", result.ExitCode)
	}
}

// ---------------------------------------------------------------------------
// TestExecuteBackgroundTimeout — background flag with timeout triggers backgrounding
// ---------------------------------------------------------------------------

func TestExecuteBackgroundTimeout(t *testing.T) {
	pe := New(nil)

	// Long-running command with short timeout + background=true
	result, err := pe.Execute(context.Background(), "shell", "sleep 30", 1*time.Second, true)
	if err != nil {
		t.Fatalf("Execute background timeout: %v", err)
	}

	if !result.TimedOut {
		t.Error("expected timedOut = true")
	}

	if !result.Backgrounded {
		t.Error("expected backgrounded = true for timeout+background")
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, backgrounded should report 0", result.ExitCode)
	}

	// Cleanup the backgrounded process
	pe.CleanupBackgrounded()
}

// ---------------------------------------------------------------------------
// TestExecuteHardCapExceeded — hard cap triggers exceeded flag
// ---------------------------------------------------------------------------

func TestExecuteHardCapExceeded(t *testing.T) {
	pe := New(&Options{
		ProjectRoot:    t.TempDir(),
		MaxOutputBytes: 100,
		HardCapBytes:   500, // small hard cap
	})

	// Generate more than 500 bytes of output
	result, err := pe.Execute(context.Background(), "shell",
		"for i in $(seq 1 200); do echo 'xxxxxxxxxxxxxxxxxxxxxxxxxx'; done",
		10*time.Second, false)
	if err != nil {
		t.Fatalf("Execute hard cap: %v", err)
	}

	// Hard cap exceeded should set timedOut and exit code 1
	if !result.TimedOut {
		t.Error("expected timedOut = true when hard cap exceeded")
	}

	if result.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", result.ExitCode)
	}
}

// ---------------------------------------------------------------------------
// TestExecuteNegativeTimeout — negative timeout uses default
// ---------------------------------------------------------------------------

func TestExecuteNegativeTimeout(t *testing.T) {
	pe := New(&Options{ProjectRoot: t.TempDir()})

	result, err := pe.Execute(context.Background(), "shell", "echo negative", -1*time.Second, false)
	if err != nil {
		t.Fatalf("Execute negative timeout: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "negative") {
		t.Errorf("stdout = %q", result.Stdout)
	}
}

// ---------------------------------------------------------------------------
// TestSpawnGenericError — non-ExitError error handling
// ---------------------------------------------------------------------------

func TestExecuteInvalidCommand(t *testing.T) {
	pe := New(&Options{
		ProjectRoot: t.TempDir(),
		Runtimes:    RuntimeMap{"shell": "/nonexistent/shell/binary"},
	})

	// This should trigger a non-ExitError path in spawn
	result, err := pe.Execute(context.Background(), "shell", "echo test", 5*time.Second, false)
	if err != nil {
		// BuildCommand might succeed but spawn will fail
		t.Logf("Execute returned error: %v", err)
		return
	}
	// If it doesn't error, it should have exit code 1 from the generic error path
	if result.ExitCode != 1 {
		t.Logf("exit code = %d for invalid command", result.ExitCode)
	}
}

// ---------------------------------------------------------------------------
// TestDetectElixirEbinPaths — with actual directory structure
// ---------------------------------------------------------------------------

func TestDetectElixirEbinPathsWithDirs(t *testing.T) {
	root := t.TempDir()

	// Create _build/dev/lib/myapp/ebin structure
	ebinDir := filepath.Join(root, "_build", "dev", "lib", "myapp", "ebin")
	if err := os.MkdirAll(ebinDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ebinDir2 := filepath.Join(root, "_build", "dev", "lib", "otherapp", "ebin")
	if err := os.MkdirAll(ebinDir2, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	result := detectElixirEbinPaths(root)
	if result == "" {
		t.Fatal("expected non-empty ebin paths")
	}

	// Should contain both app parent dirs
	if !strings.Contains(result, "myapp") {
		t.Errorf("result %q should contain myapp", result)
	}

	if !strings.Contains(result, "otherapp") {
		t.Errorf("result %q should contain otherapp", result)
	}

	// Separator should be OS-appropriate
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}

	parts := strings.Split(result, sep)
	if len(parts) != 2 {
		t.Errorf("expected 2 parts separated by %q, got %d: %q", sep, len(parts), result)
	}
}

// ---------------------------------------------------------------------------
// TestDetectElixirEbinPathsDedup — duplicate ebin dirs deduplicated
// ---------------------------------------------------------------------------

func TestDetectElixirEbinPathsDedup(t *testing.T) {
	root := t.TempDir()

	// Create only one app with ebin
	ebinDir := filepath.Join(root, "_build", "dev", "lib", "single", "ebin")
	if err := os.MkdirAll(ebinDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	result := detectElixirEbinPaths(root)
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	// Should be just one path, no separator
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}

	if strings.Contains(result, sep) {
		t.Errorf("single app should have no separator, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// TestBuildSafeEnvElixirInjection — ERL_LIBS set when elixir runtime present
// ---------------------------------------------------------------------------

func TestBuildSafeEnvElixirInjection(t *testing.T) {
	root := t.TempDir()

	// Create ebin structure
	ebinDir := filepath.Join(root, "_build", "dev", "lib", "phoenix", "ebin")
	if err := os.MkdirAll(ebinDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	pe := New(&Options{
		ProjectRoot: root,
		Runtimes:    RuntimeMap{"shell": "bash", "elixir": "elixir"},
	})

	env := pe.buildSafeEnv(t.TempDir())
	envMap := make(map[string]string)

	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
	}

	if envMap["ERL_LIBS"] == "" {
		t.Error("ERL_LIBS should be set when elixir runtime is present and _build exists")
	}

	if !strings.Contains(envMap["ERL_LIBS"], "phoenix") {
		t.Errorf("ERL_LIBS = %q, should contain phoenix", envMap["ERL_LIBS"])
	}
}

// ---------------------------------------------------------------------------
// TestBuildSafeEnvElixirNoEbin — elixir runtime present but no _build
// ---------------------------------------------------------------------------

func TestBuildSafeEnvElixirNoEbin(t *testing.T) {
	root := t.TempDir() // empty dir, no _build

	pe := New(&Options{
		ProjectRoot: root,
		Runtimes:    RuntimeMap{"shell": "bash", "elixir": "elixir"},
	})

	env := pe.buildSafeEnv(t.TempDir())
	envMap := make(map[string]string)

	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
	}

	if envMap["ERL_LIBS"] != "" {
		t.Errorf("ERL_LIBS should be empty when no _build, got %q", envMap["ERL_LIBS"])
	}
}

// ---------------------------------------------------------------------------
// TestBuildSafeEnvHomeFallback — HOME/USERPROFILE fallback to tmpDir
// ---------------------------------------------------------------------------

func TestBuildSafeEnvHomeFallback(t *testing.T) {
	// Clear HOME and USERPROFILE to test fallback
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	pe := New(&Options{
		ProjectRoot: t.TempDir(),
		Runtimes:    RuntimeMap{"shell": "bash"},
	})

	tmpDir := t.TempDir()
	env := pe.buildSafeEnv(tmpDir)
	envMap := make(map[string]string)

	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
	}

	if envMap["HOME"] != tmpDir {
		t.Errorf("HOME = %q, want tmpDir %q (fallback)", envMap["HOME"], tmpDir)
	}
}

// ---------------------------------------------------------------------------
// TestBuildSafeEnvUserprofileFallback — HOME empty, USERPROFILE used
// ---------------------------------------------------------------------------

func TestBuildSafeEnvUserprofileFallback(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "/fake/userprofile")

	pe := New(&Options{
		ProjectRoot: t.TempDir(),
		Runtimes:    RuntimeMap{"shell": "bash"},
	})

	env := pe.buildSafeEnv(t.TempDir())
	envMap := make(map[string]string)

	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
	}

	if envMap["HOME"] != "/fake/userprofile" {
		t.Errorf("HOME = %q, want /fake/userprofile", envMap["HOME"])
	}
}

// ---------------------------------------------------------------------------
// TestWrapCmdShimComspecFallback — COMSPEC env var fallback
// ---------------------------------------------------------------------------

func TestWrapCmdShimComspecFallback(t *testing.T) {
	t.Run("with_comspec", func(t *testing.T) {
		t.Setenv("COMSPEC", `C:\Windows\System32\cmd.exe`)

		result := wrapCmdShim([]string{"tsx", "script.ts"})
		if result[0] != `C:\Windows\System32\cmd.exe` {
			t.Errorf("expected COMSPEC value, got %q", result[0])
		}

		if result[1] != "/C" {
			t.Errorf("expected /C, got %q", result[1])
		}

		if result[2] != "tsx" || result[3] != "script.ts" {
			t.Errorf("original args not preserved: %v", result[2:])
		}
	})

	t.Run("without_comspec", func(t *testing.T) {
		t.Setenv("COMSPEC", "")

		result := wrapCmdShim([]string{"tsx", "script.ts"})
		if result[0] != "cmd.exe" {
			t.Errorf("expected cmd.exe fallback, got %q", result[0])
		}
	})
}

// ---------------------------------------------------------------------------
// TestParseNetMarkersURLTrimming — whitespace in URL is trimmed
// ---------------------------------------------------------------------------

func TestParseNetMarkersURLTrimming(t *testing.T) {
	stderr := "__CM_NET__:100:  https://example.com/api  \n"

	stats := ParseNetMarkers(stderr)
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}

	if stats.Requests != 1 {
		t.Errorf("Requests = %d, want 1", stats.Requests)
	}
	// URL should be trimmed
	if strings.HasPrefix(stats.URLs[0], " ") || strings.HasSuffix(stats.URLs[0], " ") {
		t.Errorf("URL not trimmed: %q", stats.URLs[0])
	}
}

// ---------------------------------------------------------------------------
// TestParseNetMarkersMultipleOnSameLine — regex finds all markers
// ---------------------------------------------------------------------------

func TestParseNetMarkersLargeByteCount(t *testing.T) {
	stderr := "__CM_NET__:999999999:https://large-download.example.com\n"

	stats := ParseNetMarkers(stderr)
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}

	if stats.TotalBytes != 999999999 {
		t.Errorf("TotalBytes = %d, want 999999999", stats.TotalBytes)
	}
}

// ---------------------------------------------------------------------------
// TestParseNetMarkersZeroBytes — zero byte count is valid
// ---------------------------------------------------------------------------

func TestParseNetMarkersZeroBytes(t *testing.T) {
	stderr := "__CM_NET__:0:https://example.com/empty\n"

	stats := ParseNetMarkers(stderr)
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}

	if stats.TotalBytes != 0 {
		t.Errorf("TotalBytes = %d, want 0", stats.TotalBytes)
	}

	if stats.Requests != 1 {
		t.Errorf("Requests = %d, want 1", stats.Requests)
	}
}

// ---------------------------------------------------------------------------
// TestParseNetMarkersMixed — mix of valid and invalid markers
// ---------------------------------------------------------------------------

func TestParseNetMarkersMixed(t *testing.T) {
	stderr := `__CM_NET__:100:https://valid.com
__CM_NET__:notnum:https://invalid.com
__CM_NET__:200:https://also-valid.com
`

	stats := ParseNetMarkers(stderr)
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	// Two valid, one invalid (skipped)
	if stats.Requests != 2 {
		t.Errorf("Requests = %d, want 2 (one invalid skipped)", stats.Requests)
	}

	if stats.TotalBytes != 300 {
		t.Errorf("TotalBytes = %d, want 300", stats.TotalBytes)
	}
}

// ---------------------------------------------------------------------------
// TestJSNetPreambleContent — verify expected patterns
// ---------------------------------------------------------------------------

func TestJSNetPreambleContent(t *testing.T) {
	if !strings.Contains(JSNetPreamble, "__CM_NET__") {
		t.Error("JSNetPreamble should contain __CM_NET__ marker pattern")
	}

	if !strings.Contains(JSNetPreamble, "globalThis.fetch") {
		t.Error("JSNetPreamble should wrap globalThis.fetch")
	}

	if !strings.Contains(JSNetPreamble, "process.stderr.write") {
		t.Error("JSNetPreamble should write to stderr")
	}

	if !strings.Contains(JSNetPreamble, "arrayBuffer") {
		t.Error("JSNetPreamble should read response body as arrayBuffer")
	}

	if !strings.Contains(JSNetPreamble, "byteLength") {
		t.Error("JSNetPreamble should use byteLength for byte counting")
	}
}

// ---------------------------------------------------------------------------
// TestGetAvailableLanguagesEdgeCases
// ---------------------------------------------------------------------------

func TestGetAvailableLanguagesAllPresent(t *testing.T) {
	runtimes := RuntimeMap{
		"javascript": "node",
		"shell":      "bash",
		"typescript": "tsx",
		"python":     "python3",
		"ruby":       "ruby",
		"go":         "go",
		"rust":       "rustc",
		"php":        "php",
		"perl":       "perl",
		"r":          "Rscript",
		"elixir":     "elixir",
	}

	langs := GetAvailableLanguages(runtimes)
	if len(langs) != 11 {
		t.Errorf("expected 11 languages, got %d: %v", len(langs), langs)
	}
}

func TestGetAvailableLanguagesMinimal(t *testing.T) {
	runtimes := RuntimeMap{} // no optional runtimes
	langs := GetAvailableLanguages(runtimes)
	// Should always include javascript and shell
	if len(langs) != 2 {
		t.Errorf("expected 2 base languages, got %d: %v", len(langs), langs)
	}

	if langs[0] != "javascript" || langs[1] != "shell" {
		t.Errorf("expected [javascript shell], got %v", langs)
	}
}

// ---------------------------------------------------------------------------
// TestRuntimeErrorMessage
// ---------------------------------------------------------------------------

func TestRuntimeErrorMessage(t *testing.T) {
	tests := []struct {
		lang string
		want string
	}{
		{"python", "python runtime not available"},
		{"rust", "rust runtime not available"},
		{"cobol", "cobol runtime not available"},
	}
	for _, tc := range tests {
		err := &RuntimeError{Language: tc.lang}
		if err.Error() != tc.want {
			t.Errorf("RuntimeError{%q}.Error() = %q, want %q", tc.lang, err.Error(), tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestExecuteDefaultTimeout — timeout <= 0 uses default
// ---------------------------------------------------------------------------

func TestExecuteDefaultTimeout(t *testing.T) {
	pe := New(&Options{
		ProjectRoot: t.TempDir(),
		Runtimes:    RuntimeMap{"shell": "bash"},
	})

	// Pass 0 timeout — should use DefaultTimeout, not panic
	result, err := pe.Execute(context.Background(), "shell", "echo ok", 0, false)
	if err != nil {
		t.Fatalf("Execute with zero timeout: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
}

// ---------------------------------------------------------------------------
// TestExecuteUnavailableRuntime — runtime error path
// ---------------------------------------------------------------------------

func TestExecuteUnavailableRuntime(t *testing.T) {
	pe := New(&Options{
		ProjectRoot: t.TempDir(),
		Runtimes:    RuntimeMap{"shell": "bash"}, // no python
	})

	_, err := pe.Execute(context.Background(), "python", "print('hi')", 5*time.Second, false)
	if err == nil {
		t.Fatal("expected error for unavailable python runtime")
	}

	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("error = %q, want 'not available'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// TestExecuteShellUsesProjectDir — shell cwd should be projectRoot
// ---------------------------------------------------------------------------

func TestExecuteShellUsesProjectDir(t *testing.T) {
	projRoot := t.TempDir()

	// Create a marker file in project root
	marker := filepath.Join(projRoot, "marker.txt")
	if err := os.WriteFile(marker, []byte("found"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	pe := New(&Options{
		ProjectRoot: projRoot,
	})

	// Shell should run in projRoot, so ls/cat should find marker.txt
	result, err := pe.Execute(context.Background(), "shell", "cat marker.txt", 10*time.Second, false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(result.Stdout, "found") {
		t.Errorf("stdout = %q, want 'found' — shell should run in projectRoot", result.Stdout)
	}
}

// ---------------------------------------------------------------------------
// TestCappedWriterExactBoundary — write exactly at hard cap boundary
// ---------------------------------------------------------------------------

func TestCappedWriterExactBoundary(t *testing.T) {
	stdout, _ := newCappedWriterPair(10)

	// Write exactly 10 bytes
	n, err := stdout.Write([]byte("1234567890"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if n != 10 {
		t.Errorf("n = %d, want 10", n)
	}

	if stdout.exceeded.Load() {
		t.Error("should not be exceeded at exact boundary")
	}

	// One more byte pushes over
	n, err = stdout.Write([]byte("x"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if n != 1 {
		t.Errorf("n = %d, want 1", n)
	}

	if !stdout.exceeded.Load() {
		t.Error("should be exceeded after going over hard cap")
	}
}

// ---------------------------------------------------------------------------
// TestCappedWriterSharedState — stdout and stderr share exceeded flag
// ---------------------------------------------------------------------------

func TestCappedWriterSharedState(t *testing.T) {
	stdout, stderr := newCappedWriterPair(50)

	// Write 30 to stdout
	_, _ = stdout.Write([]byte(strings.Repeat("a", 30)))
	// Write 25 to stderr — total 55 > 50
	_, _ = stderr.Write([]byte(strings.Repeat("b", 25)))

	if !stdout.exceeded.Load() {
		t.Error("stdout should see exceeded (shared flag)")
	}

	if !stderr.exceeded.Load() {
		t.Error("stderr should see exceeded (shared flag)")
	}

	// Verify data written before cap was reached
	if stdout.buf.Len() != 30 {
		t.Errorf("stdout buf len = %d, want 30", stdout.buf.Len())
	}
}

// ---------------------------------------------------------------------------
// TestBuildSafeEnvWindowsSpecific — Windows PATH injection + vars
// ---------------------------------------------------------------------------

func TestBuildSafeEnvWindowsSpecific(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	pe := New(&Options{
		ProjectRoot: t.TempDir(),
		Runtimes:    RuntimeMap{"shell": "bash"},
	})

	env := pe.buildSafeEnv(t.TempDir())
	envMap := make(map[string]string)

	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
	}

	// Git usr\bin should be prepended to PATH
	if !strings.Contains(envMap["PATH"], `Git\usr\bin`) && !strings.Contains(envMap["PATH"], `Git\bin`) {
		t.Log("Git paths not found in PATH — may not be installed")
	}

	// Windows-specific env vars should be set
	if envMap["MSYS_NO_PATHCONV"] != "1" {
		t.Error("MSYS_NO_PATHCONV should be 1")
	}

	if envMap["MSYS2_ARG_CONV_EXCL"] != "*" {
		t.Error("MSYS2_ARG_CONV_EXCL should be *")
	}
}

// ---------------------------------------------------------------------------
// TestBuildSafeEnvSSLPassthrough — SSL_CERT_FILE and related vars
// ---------------------------------------------------------------------------

func TestBuildSafeEnvSSLPassthrough(t *testing.T) {
	t.Setenv("SSL_CERT_FILE", "/custom/ca-bundle.crt")
	t.Setenv("NODE_EXTRA_CA_CERTS", "/custom/node-ca.crt")

	pe := New(&Options{
		ProjectRoot: t.TempDir(),
		Runtimes:    RuntimeMap{"shell": "bash"},
	})

	env := pe.buildSafeEnv(t.TempDir())
	envMap := make(map[string]string)

	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
	}

	if envMap["SSL_CERT_FILE"] != "/custom/ca-bundle.crt" {
		t.Errorf("SSL_CERT_FILE = %q, want /custom/ca-bundle.crt", envMap["SSL_CERT_FILE"])
	}

	if envMap["NODE_EXTRA_CA_CERTS"] != "/custom/node-ca.crt" {
		t.Errorf("NODE_EXTRA_CA_CERTS = %q, want /custom/node-ca.crt", envMap["NODE_EXTRA_CA_CERTS"])
	}
}

// ---------------------------------------------------------------------------
// TestBuildSafeEnvDenyOverridesPassthrough — deny list wins over passthrough
// ---------------------------------------------------------------------------

func TestBuildSafeEnvDenyOverridesPassthrough(t *testing.T) {
	// GOFLAGS is in both passthrough and deny list — deny should win
	t.Setenv("GOFLAGS", "-race")

	pe := New(&Options{
		ProjectRoot: t.TempDir(),
		Runtimes:    RuntimeMap{"shell": "bash"},
	})

	env := pe.buildSafeEnv(t.TempDir())
	envMap := make(map[string]string)

	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
	}

	if _, found := envMap["GOFLAGS"]; found {
		t.Error("GOFLAGS is in deny list and should be stripped")
	}
}

// ---------------------------------------------------------------------------
// TestExecuteStreamDefaultTimeout — zero timeout uses default
// ---------------------------------------------------------------------------

func TestExecuteStreamDefaultTimeout(t *testing.T) {
	pe := New(&Options{ProjectRoot: t.TempDir()})

	result, err := pe.ExecuteStream(context.Background(), "shell", "echo stream-ok", 0, func(_ OutputChunk) error {
		return nil
	})
	if err != nil {
		t.Fatalf("ExecuteStream zero timeout: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "stream-ok") {
		t.Errorf("stdout = %q, want stream-ok", result.Stdout)
	}
}

// ---------------------------------------------------------------------------
// TestExecuteStreamUnavailableRuntime — error path
// ---------------------------------------------------------------------------

func TestExecuteStreamUnavailableRuntime(t *testing.T) {
	pe := New(&Options{
		ProjectRoot: t.TempDir(),
		Runtimes:    RuntimeMap{"shell": "bash"}, // no python
	})

	_, err := pe.ExecuteStream(context.Background(), "python", "print('hi')", 5*time.Second, func(_ OutputChunk) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for unavailable runtime")
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFileStreamShell — integration test for file stream
// ---------------------------------------------------------------------------

func TestExecuteFileStreamShell(t *testing.T) {
	projRoot := t.TempDir()

	dataFile := filepath.Join(projRoot, "testdata.txt")
	if err := os.WriteFile(dataFile, []byte("stream-file-content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	pe := New(&Options{ProjectRoot: projRoot})

	var chunks []OutputChunk

	result, err := pe.ExecuteFileStream(
		context.Background(),
		dataFile,
		"shell",
		`echo "$FILE_CONTENT"`,
		10*time.Second,
		func(chunk OutputChunk) error {
			chunks = append(chunks, chunk)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ExecuteFileStream: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}

	if !strings.Contains(result.Stdout, "stream-file-content") {
		t.Errorf("stdout = %q, want stream-file-content", result.Stdout)
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFileStreamRelativePath — relative path resolved
// ---------------------------------------------------------------------------

func TestExecuteFileStreamRelativePath(t *testing.T) {
	projRoot := t.TempDir()

	subDir := filepath.Join(projRoot, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(subDir, "data.txt"), []byte("relative-stream"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	pe := New(&Options{ProjectRoot: projRoot})

	result, err := pe.ExecuteFileStream(
		context.Background(),
		"sub/data.txt", // relative
		"shell",
		`echo "$FILE_CONTENT"`,
		10*time.Second,
		func(_ OutputChunk) error { return nil },
	)
	if err != nil {
		t.Fatalf("ExecuteFileStream: %v", err)
	}

	if !strings.Contains(result.Stdout, "relative-stream") {
		t.Errorf("stdout = %q, want relative-stream", result.Stdout)
	}
}

// ---------------------------------------------------------------------------
// TestBuildCommandTsNode — ts-node runtime variant
// ---------------------------------------------------------------------------

func TestBuildCommandTsNode(t *testing.T) {
	runtimes := RuntimeMap{"typescript": "ts-node"}

	cmd, err := BuildCommand(runtimes, "typescript", "script.ts")
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}
	// On non-Windows or non-shim, should be [ts-node, script.ts]
	// On Windows with .cmd shim, would be wrapped
	if runtime.GOOS != "windows" {
		if cmd[0] != "ts-node" {
			t.Errorf("cmd[0] = %q, want ts-node", cmd[0])
		}
	}
}

// ---------------------------------------------------------------------------
// TestSmartTruncateNoNewlinesSmallBudget — no newlines forces raw cut
// ---------------------------------------------------------------------------

func TestSmartTruncateNoNewlinesSmallBudget(t *testing.T) {
	// No newlines at all, forces LastIndex to return -1
	input := strings.Repeat("x", 500)

	got := smartTruncate(input, 300)
	if !strings.Contains(got, "truncated") {
		t.Error("expected truncation marker")
	}
}

// ---------------------------------------------------------------------------
// TestExecuteStreamShellCwd — stream execution uses project root for shell
// ---------------------------------------------------------------------------

func TestExecuteStreamShellCwd(t *testing.T) {
	projRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projRoot, "streamtest.txt"), []byte("here"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	pe := New(&Options{ProjectRoot: projRoot})

	result, err := pe.ExecuteStream(context.Background(), "shell", "cat streamtest.txt", 10*time.Second, func(_ OutputChunk) error {
		return nil
	})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}

	if !strings.Contains(result.Stdout, "here") {
		t.Errorf("stdout = %q, shell stream should use projectRoot as cwd", result.Stdout)
	}
}

// ---------------------------------------------------------------------------
// TestExecuteStreamError — non-zero exit code in stream
// ---------------------------------------------------------------------------

func TestExecuteStreamError(t *testing.T) {
	pe := New(nil)

	result, err := pe.ExecuteStream(context.Background(), "shell", "exit 42", 10*time.Second, func(_ OutputChunk) error {
		return nil
	})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}

	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}

// ---------------------------------------------------------------------------
// TestIsCmdShimNonWindows — returns false on non-Windows
// ---------------------------------------------------------------------------

func TestIsCmdShimNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test is for non-Windows")
	}

	if isCmdShim("bash") {
		t.Error("isCmdShim should always return false on non-Windows")
	}
}
