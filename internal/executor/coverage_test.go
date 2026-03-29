package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TestFileExtension — verify extension mapping for all 11 languages + default
// ---------------------------------------------------------------------------

func TestFileExtension(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{"javascript", "js"},
		{"typescript", "ts"},
		{"python", "py"},
		{"shell", "sh"},
		{"ruby", "rb"},
		{"go", "go"},
		{"rust", "rs"},
		{"php", "php"},
		{"perl", "pl"},
		{"r", "R"},
		{"elixir", "exs"},
		{"unknown", "txt"},
		{"", "txt"},
		{"java", "txt"},
	}
	for _, tc := range tests {
		t.Run(tc.language, func(t *testing.T) {
			got := fileExtension(tc.language)
			if got != tc.want {
				t.Errorf("fileExtension(%q) = %q, want %q", tc.language, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestWriteScript — script file creation, Go wrapping, PHP tag insertion
// ---------------------------------------------------------------------------

func TestWriteScript(t *testing.T) {
	pe := New(&Options{ProjectRoot: t.TempDir()})

	t.Run("go_wraps_bare_code", func(t *testing.T) {
		tmpDir := t.TempDir()

		fp, err := pe.writeScript(tmpDir, `fmt.Println("hi")`, "go")
		if err != nil {
			t.Fatalf("writeScript go: %v", err)
		}

		data, _ := os.ReadFile(fp)

		content := string(data)
		if !strings.Contains(content, "package main") {
			t.Error("expected package main wrapping")
		}

		if !strings.Contains(content, `import "fmt"`) {
			t.Error("expected fmt import")
		}

		if !strings.Contains(content, "func main()") {
			t.Error("expected func main wrapping")
		}

		if !strings.HasSuffix(fp, ".go") {
			t.Errorf("expected .go extension, got %s", fp)
		}
	})

	t.Run("go_preserves_existing_package", func(t *testing.T) {
		tmpDir := t.TempDir()
		code := "package main\n\nfunc main() {}"

		fp, err := pe.writeScript(tmpDir, code, "go")
		if err != nil {
			t.Fatalf("writeScript go: %v", err)
		}

		data, _ := os.ReadFile(fp)
		content := string(data)
		// Should NOT double-wrap
		if strings.Count(content, "package main") != 1 {
			t.Error("should not double-wrap package declaration")
		}
	})

	t.Run("php_adds_opening_tag", func(t *testing.T) {
		tmpDir := t.TempDir()

		fp, err := pe.writeScript(tmpDir, `echo "hello";`, "php")
		if err != nil {
			t.Fatalf("writeScript php: %v", err)
		}

		data, _ := os.ReadFile(fp)

		content := string(data)
		if !strings.HasPrefix(content, "<?php\n") {
			t.Errorf("expected <?php prefix, got %q", content[:20])
		}

		if !strings.HasSuffix(fp, ".php") {
			t.Errorf("expected .php extension, got %s", fp)
		}
	})

	t.Run("php_preserves_existing_tag", func(t *testing.T) {
		tmpDir := t.TempDir()
		code := "<?php\necho 'hi';"

		fp, err := pe.writeScript(tmpDir, code, "php")
		if err != nil {
			t.Fatalf("writeScript php: %v", err)
		}

		data, _ := os.ReadFile(fp)

		content := string(data)
		if strings.Count(content, "<?php") != 1 {
			t.Error("should not double-add PHP opening tag")
		}

		_ = fp
	})

	t.Run("shell_gets_executable_permission", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("file permissions not meaningful on Windows")
		}

		tmpDir := t.TempDir()

		fp, err := pe.writeScript(tmpDir, "echo hi", "shell")
		if err != nil {
			t.Fatalf("writeScript shell: %v", err)
		}

		info, _ := os.Stat(fp)
		if info.Mode()&0o100 == 0 {
			t.Errorf("shell script should be executable, mode = %o", info.Mode())
		}
	})

	t.Run("python_no_special_wrapping", func(t *testing.T) {
		tmpDir := t.TempDir()
		code := "print('hello')"

		fp, err := pe.writeScript(tmpDir, code, "python")
		if err != nil {
			t.Fatalf("writeScript python: %v", err)
		}

		data, _ := os.ReadFile(fp)
		if string(data) != code {
			t.Errorf("python code should not be modified, got %q", string(data))
		}

		if !strings.HasSuffix(fp, ".py") {
			t.Errorf("expected .py extension, got %s", fp)
		}
	})

	t.Run("all_languages_create_correct_extension", func(t *testing.T) {
		langs := []string{"javascript", "typescript", "python", "shell", "ruby", "go", "rust", "php", "perl", "r", "elixir"}
		for _, lang := range langs {
			tmpDir := t.TempDir()

			code := "x"
			if lang == "go" {
				code = "package main\nfunc main(){}"
			}

			fp, err := pe.writeScript(tmpDir, code, lang)
			if err != nil {
				t.Fatalf("writeScript(%s): %v", lang, err)
			}

			wantExt := "." + fileExtension(lang)
			if !strings.HasSuffix(fp, wantExt) {
				t.Errorf("writeScript(%s) file %s, want extension %s", lang, fp, wantExt)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestWrapWithFileContent — language-specific file injection wrapping
// ---------------------------------------------------------------------------

func TestWrapWithFileContent(t *testing.T) {
	testPath := "/tmp/test/data.txt"
	userCode := "// user code here"

	tests := []struct {
		language     string
		wantContains []string
	}{
		{"javascript", []string{"FILE_CONTENT_PATH", "FILE_CONTENT", "require(\"fs\")", "readFileSync", "file_path"}},
		{"typescript", []string{"FILE_CONTENT_PATH", "FILE_CONTENT", "require(\"fs\")", "readFileSync", "file_path"}},
		{"python", []string{"FILE_CONTENT_PATH", "FILE_CONTENT", "open(", "encoding=\"utf-8\"", "file_path"}},
		{"shell", []string{"FILE_CONTENT_PATH=", "FILE_CONTENT=$(cat", "file_path="}},
		{"ruby", []string{"FILE_CONTENT_PATH", "FILE_CONTENT", "File.read", "file_path"}},
		{"go", []string{"package main", "os.ReadFile", "FILE_CONTENT_PATH", "file_path"}},
		{"rust", []string{"fs::read_to_string", "file_content_path", "file_path", "fn main()"}},
		{"php", []string{"<?php", "$FILE_CONTENT_PATH", "$FILE_CONTENT", "file_get_contents", "$file_path"}},
		{"perl", []string{"$FILE_CONTENT_PATH", "$FILE_CONTENT", "open(", "encoding(UTF-8)", "$file_path"}},
		{"r", []string{"FILE_CONTENT_PATH", "FILE_CONTENT", "readLines", "file_path"}},
		{"elixir", []string{"file_content_path", "file_content", "File.read!", "file_path"}},
	}

	for _, tc := range tests {
		t.Run(tc.language, func(t *testing.T) {
			got := wrapWithFileContent(testPath, tc.language, userCode)
			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("wrapWithFileContent(%q) missing %q in:\n%s", tc.language, want, got)
				}
			}
			// All should include the user code
			if !strings.Contains(got, userCode) {
				t.Errorf("wrapWithFileContent(%q) missing user code", tc.language)
			}
		})
	}

	t.Run("unknown_language_passthrough", func(t *testing.T) {
		got := wrapWithFileContent(testPath, "cobol", userCode)
		if got != userCode {
			t.Errorf("unknown language should return code unchanged, got %q", got)
		}
	})

	t.Run("path_with_special_characters", func(t *testing.T) {
		specialPath := `/tmp/my "file" path/data.txt`

		got := wrapWithFileContent(specialPath, "python", "print(FILE_CONTENT)")
		if !strings.Contains(got, `\"file\"`) || !strings.Contains(got, specialPath[:5]) {
			// JSON-escaped quotes should appear
			t.Logf("wrapped output for special path: %s", got)
		}
	})

	t.Run("shell_single_quote_escaping", func(t *testing.T) {
		pathWithQuote := "/tmp/it's/data.txt"
		got := wrapWithFileContent(pathWithQuote, "shell", "echo $FILE_CONTENT")
		// Shell escaping: ' should become '\''
		if !strings.Contains(got, `'\''`) {
			t.Errorf("shell wrapping should escape single quotes, got:\n%s", got)
		}
	})
}

// ---------------------------------------------------------------------------
// TestBuildSafeEnv — environment variable construction
// ---------------------------------------------------------------------------

func TestBuildSafeEnv(t *testing.T) {
	pe := New(&Options{ProjectRoot: t.TempDir()})
	tmpDir := t.TempDir()
	env := pe.buildSafeEnv(tmpDir)

	envMap := make(map[string]string)

	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	t.Run("PATH_preserved", func(t *testing.T) {
		if envMap["PATH"] == "" {
			t.Error("PATH should not be empty")
		}
	})

	t.Run("HOME_set", func(t *testing.T) {
		if envMap["HOME"] == "" {
			t.Error("HOME should be set")
		}
	})

	t.Run("TMPDIR_set_to_param", func(t *testing.T) {
		if envMap["TMPDIR"] != tmpDir {
			t.Errorf("TMPDIR = %q, want %q", envMap["TMPDIR"], tmpDir)
		}
	})

	t.Run("python_env_vars", func(t *testing.T) {
		if envMap["PYTHONDONTWRITEBYTECODE"] != "1" {
			t.Error("PYTHONDONTWRITEBYTECODE should be 1")
		}

		if envMap["PYTHONUNBUFFERED"] != "1" {
			t.Error("PYTHONUNBUFFERED should be 1")
		}

		if envMap["PYTHONUTF8"] != "1" {
			t.Error("PYTHONUTF8 should be 1")
		}
	})

	t.Run("NO_COLOR_set", func(t *testing.T) {
		if envMap["NO_COLOR"] != "1" {
			t.Error("NO_COLOR should be 1")
		}
	})

	t.Run("LANG_set", func(t *testing.T) {
		if envMap["LANG"] != "en_US.UTF-8" {
			t.Errorf("LANG = %q, want en_US.UTF-8", envMap["LANG"])
		}
	})

	t.Run("passthrough_vars_included_when_set", func(t *testing.T) {
		// Set a passthrough var and rebuild
		t.Setenv("GOPATH", "/fake/gopath")

		env2 := pe.buildSafeEnv(tmpDir)
		envMap2 := make(map[string]string)

		for _, e := range env2 {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				envMap2[parts[0]] = parts[1]
			}
		}

		if envMap2["GOPATH"] != "/fake/gopath" {
			t.Errorf("GOPATH = %q, want /fake/gopath", envMap2["GOPATH"])
		}
	})

	if runtime.GOOS == "windows" {
		t.Run("windows_specific_vars", func(t *testing.T) {
			if envMap["MSYS_NO_PATHCONV"] != "1" {
				t.Error("MSYS_NO_PATHCONV should be 1 on Windows")
			}

			if envMap["MSYS2_ARG_CONV_EXCL"] != "*" {
				t.Error("MSYS2_ARG_CONV_EXCL should be * on Windows")
			}
			// SYSTEMROOT should be passed through
			if os.Getenv("SYSTEMROOT") != "" && envMap["SYSTEMROOT"] == "" {
				t.Error("SYSTEMROOT should be passed through on Windows")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestOptionsDefaults — Options merging with defaults
// ---------------------------------------------------------------------------

func TestOptionsDefaults(t *testing.T) {
	t.Run("nil_options", func(t *testing.T) {
		pe := New(nil)
		if pe.maxOutputBytes != DefaultMaxOutputBytes {
			t.Errorf("maxOutputBytes = %d, want %d", pe.maxOutputBytes, DefaultMaxOutputBytes)
		}

		if pe.hardCapBytes != DefaultHardCapBytes {
			t.Errorf("hardCapBytes = %d, want %d", pe.hardCapBytes, DefaultHardCapBytes)
		}

		if pe.projectRoot == "" {
			t.Error("projectRoot should default to cwd")
		}

		if pe.runtimes == nil {
			t.Error("runtimes should be auto-detected")
		}

		if pe.backgroundedPids == nil {
			t.Error("backgroundedPids should be initialized")
		}
	})

	t.Run("custom_options", func(t *testing.T) {
		root := t.TempDir()
		runtimes := RuntimeMap{"shell": "bash", "python": "python3"}

		pe := New(&Options{
			MaxOutputBytes: 1024,
			HardCapBytes:   2048,
			ProjectRoot:    root,
			Runtimes:       runtimes,
		})
		if pe.maxOutputBytes != 1024 {
			t.Errorf("maxOutputBytes = %d, want 1024", pe.maxOutputBytes)
		}

		if pe.hardCapBytes != 2048 {
			t.Errorf("hardCapBytes = %d, want 2048", pe.hardCapBytes)
		}

		if pe.projectRoot != root {
			t.Errorf("projectRoot = %q, want %q", pe.projectRoot, root)
		}

		if pe.runtimes["python"] != "python3" {
			t.Error("custom runtimes not applied")
		}
	})

	t.Run("zero_values_use_defaults", func(t *testing.T) {
		pe := New(&Options{
			MaxOutputBytes: 0,
			HardCapBytes:   0,
			ProjectRoot:    "",
		})
		if pe.maxOutputBytes != DefaultMaxOutputBytes {
			t.Errorf("zero MaxOutputBytes should use default, got %d", pe.maxOutputBytes)
		}

		if pe.hardCapBytes != DefaultHardCapBytes {
			t.Errorf("zero HardCapBytes should use default, got %d", pe.hardCapBytes)
		}
	})

	t.Run("runtimes_accessor", func(t *testing.T) {
		runtimes := RuntimeMap{"shell": "sh"}
		pe := New(&Options{Runtimes: runtimes})

		got := pe.Runtimes()
		if got["shell"] != "sh" {
			t.Errorf("Runtimes() shell = %q, want sh", got["shell"])
		}
	})
}

// ---------------------------------------------------------------------------
// TestExecuteFile_Integration — test with shell scripts (always available)
// ---------------------------------------------------------------------------

func TestExecuteFile_Integration(t *testing.T) {
	// Create a temp file to read
	tmpFile := filepath.Join(t.TempDir(), "testdata.txt")
	if err := os.WriteFile(tmpFile, []byte("hello from file"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	pe := New(&Options{ProjectRoot: t.TempDir()})

	t.Run("shell_reads_file", func(t *testing.T) {
		result, err := pe.ExecuteFile(
			context.Background(),
			tmpFile,
			"shell",
			`echo "$FILE_CONTENT"`,
			10*time.Second,
		)
		if err != nil {
			t.Fatalf("ExecuteFile: %v", err)
		}

		if result.ExitCode != 0 {
			t.Errorf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
		}

		if !strings.Contains(result.Stdout, "hello from file") {
			t.Errorf("stdout = %q, want 'hello from file'", result.Stdout)
		}
	})

	t.Run("relative_path_resolved", func(t *testing.T) {
		// Create file in project root
		projRoot := t.TempDir()
		relFile := "subdir/data.txt"

		absPath := filepath.Join(projRoot, relFile)
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		if err := os.WriteFile(absPath, []byte("relative content"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}

		pe2 := New(&Options{ProjectRoot: projRoot})

		result, err := pe2.ExecuteFile(
			context.Background(),
			relFile,
			"shell",
			`echo "$FILE_CONTENT"`,
			10*time.Second,
		)
		if err != nil {
			t.Fatalf("ExecuteFile relative: %v", err)
		}

		if !strings.Contains(result.Stdout, "relative content") {
			t.Errorf("stdout = %q, want 'relative content'", result.Stdout)
		}
	})
}

// ---------------------------------------------------------------------------
// TestSmartTruncate_EdgeCases — additional truncation edge cases
// ---------------------------------------------------------------------------

func TestSmartTruncate_EdgeCases(t *testing.T) {
	t.Run("empty_string", func(t *testing.T) {
		got := smartTruncate("", 100)
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("single_char", func(t *testing.T) {
		got := smartTruncate("x", 100)
		if got != "x" {
			t.Errorf("expected 'x', got %q", got)
		}
	})

	t.Run("moderately_over_limit", func(t *testing.T) {
		input := strings.Repeat("abcde\n", 100) // 600 bytes

		got := smartTruncate(input, 300)
		if !strings.Contains(got, "truncated") {
			t.Error("expected truncation for input over limit")
		}
	})

	t.Run("no_newlines_large_budget", func(t *testing.T) {
		// Use a large enough budget that head+tail don't overlap
		input := strings.Repeat("x", 2000)

		got := smartTruncate(input, 1000)
		if !strings.Contains(got, "truncated") {
			t.Error("expected truncation marker for long no-newline input")
		}
	})

	t.Run("all_newlines_large_budget", func(t *testing.T) {
		input := strings.Repeat("\n", 2000)

		got := smartTruncate(input, 1000)
		if !strings.Contains(got, "truncated") {
			t.Error("expected truncation marker")
		}
	})

	t.Run("moderate_limit", func(t *testing.T) {
		input := strings.Repeat("line\n", 100) // 500 bytes
		got := smartTruncate(input, 200)
		// Should truncate without issues
		if !strings.Contains(got, "truncated") {
			t.Error("expected truncation marker")
		}
	})

	t.Run("binary_like_content", func(t *testing.T) {
		input := strings.Repeat("\x00\x01\x02\x03\n", 500) // 2500 bytes

		got := smartTruncate(input, 1000)
		if !strings.Contains(got, "truncated") {
			t.Error("expected truncation for binary-like content")
		}
	})
}

// ---------------------------------------------------------------------------
// TestCleanupBackgrounded — process cleanup
// ---------------------------------------------------------------------------

func TestCleanupBackgrounded(t *testing.T) {
	t.Run("cleanup_empty", func(t *testing.T) {
		pe := New(&Options{ProjectRoot: t.TempDir()})
		// Should not panic on empty map
		pe.CleanupBackgrounded()

		if len(pe.backgroundedPids) != 0 {
			t.Error("pids map should be empty after cleanup")
		}
	})

	t.Run("cleanup_with_stale_pids", func(t *testing.T) {
		pe := New(&Options{ProjectRoot: t.TempDir()})
		// Add fake PIDs (non-existent processes)
		pe.mu.Lock()
		pe.backgroundedPids[999999] = struct{}{}
		pe.backgroundedPids[999998] = struct{}{}
		pe.mu.Unlock()

		// Should not panic even with non-existent PIDs
		pe.CleanupBackgrounded()

		if len(pe.backgroundedPids) != 0 {
			t.Error("pids map should be reset after cleanup")
		}
	})

	t.Run("cleanup_resets_map", func(t *testing.T) {
		pe := New(&Options{ProjectRoot: t.TempDir()})
		pe.mu.Lock()
		pe.backgroundedPids[12345] = struct{}{}
		pe.mu.Unlock()

		pe.CleanupBackgrounded()

		// Add new PID after cleanup to verify map is usable
		pe.mu.Lock()
		pe.backgroundedPids[67890] = struct{}{}
		pe.mu.Unlock()

		if len(pe.backgroundedPids) != 1 {
			t.Error("should be able to add PIDs after cleanup")
		}
	})
}

// ---------------------------------------------------------------------------
// TestEnvDenyListContainsINPUTRC — verify INPUTRC is denied
// ---------------------------------------------------------------------------

func TestEnvDenyListContainsINPUTRC(t *testing.T) {
	if !envDenyList["INPUTRC"] {
		t.Error("INPUTRC should be in envDenyList")
	}
}

// ---------------------------------------------------------------------------
// TestBuildCommand_AllLanguages — verify command construction
// ---------------------------------------------------------------------------

func TestBuildCommand_AllLanguages(t *testing.T) {
	runtimes := RuntimeMap{
		"javascript": "node",
		"typescript": "tsx",
		"python":     "python3",
		"shell":      "bash",
		"ruby":       "ruby",
		"go":         "go",
		"rust":       "rustc",
		"php":        "php",
		"perl":       "perl",
		"r":          "Rscript",
		"elixir":     "elixir",
	}

	tests := []struct {
		language string
		wantCmd  string
		wantLen  int
	}{
		{"javascript", "node", 2},
		{"typescript", "tsx", 2},
		{"python", "python3", 2},
		{"shell", "bash", 2},
		{"ruby", "ruby", 2},
		{"go", "go", 3}, // go run <file>
		{"rust", "__rust_compile_run__", 2},
		{"php", "php", 2},
		{"perl", "perl", 2},
		{"r", "Rscript", 2},
		{"elixir", "elixir", 2},
	}

	for _, tc := range tests {
		t.Run(tc.language, func(t *testing.T) {
			cmd, err := BuildCommand(runtimes, tc.language, "script.txt")
			if err != nil {
				t.Fatalf("BuildCommand(%q): %v", tc.language, err)
			}

			if cmd[0] != tc.wantCmd {
				t.Errorf("cmd[0] = %q, want %q", cmd[0], tc.wantCmd)
			}

			if len(cmd) != tc.wantLen {
				t.Errorf("len(cmd) = %d, want %d", len(cmd), tc.wantLen)
			}
		})
	}

	t.Run("bun_javascript", func(t *testing.T) {
		bunRt := RuntimeMap{"javascript": "bun"}

		cmd, err := BuildCommand(bunRt, "javascript", "script.js")
		if err != nil {
			t.Fatalf("BuildCommand: %v", err)
		}

		if cmd[0] != "bun" || cmd[1] != "run" {
			t.Errorf("bun JS cmd = %v, want [bun run ...]", cmd)
		}
	})

	t.Run("bun_typescript", func(t *testing.T) {
		bunRt := RuntimeMap{"typescript": "bun"}

		cmd, err := BuildCommand(bunRt, "typescript", "script.ts")
		if err != nil {
			t.Fatalf("BuildCommand: %v", err)
		}

		if cmd[0] != "bun" || cmd[1] != "run" {
			t.Errorf("bun TS cmd = %v, want [bun run ...]", cmd)
		}
	})

	t.Run("unknown_language_error", func(t *testing.T) {
		_, err := BuildCommand(runtimes, "cobol", "script.cob")
		if err == nil {
			t.Fatal("expected error for unknown language")
		}

		var re *RuntimeError
		//nolint:wsl // adjacent related vars
		rErr := &RuntimeError{}
		if errors.As(err, &rErr) {
			re = rErr
			if re.Language != "cobol" {
				t.Errorf("RuntimeError.Language = %q, want cobol", re.Language)
			}
		} else {
			t.Errorf("expected *RuntimeError, got %T", err)
		}
	})

	t.Run("missing_runtime_errors", func(t *testing.T) {
		empty := RuntimeMap{}
		for _, lang := range []string{"typescript", "python", "ruby", "go", "rust", "php", "perl", "r", "elixir"} {
			_, err := BuildCommand(empty, lang, "script.txt")
			if err == nil {
				t.Errorf("expected error for missing %s runtime", lang)
			}
		}
	})
}
