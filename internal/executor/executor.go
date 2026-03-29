package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	syncatomic "sync/atomic"
	"time"

	"github.com/inovacc/thimble/internal/model"
)

const (
	// DefaultMaxOutputBytes is the soft cap for stdout/stderr.
	DefaultMaxOutputBytes = 102_400
	// DefaultHardCapBytes is the hard cap before killing the process.
	DefaultHardCapBytes = 100 * 1024 * 1024 // 100MB
	// DefaultTimeout for execution.
	DefaultTimeout = 30 * time.Second
)

// envDenyList contains environment variables that are stripped from the
// subprocess environment because they can alter tool behaviour in
// security-sensitive ways (e.g. LD_PRELOAD, GIT_SSH, BASH_ENV).
var envDenyList = map[string]bool{
	"GIT_SSH": true, "GIT_SSH_COMMAND": true, "GIT_CONFIG_GLOBAL": true,
	"GIT_CONFIG_SYSTEM": true, "GIT_ASKPASS": true, "GIT_EDITOR": true,
	"GIT_PAGER": true, "GIT_EXEC_PATH": true, "GIT_TEMPLATE_DIR": true,
	"GIT_PROXY_COMMAND": true, "GIT_CONFIG_COUNT": true,
	"PYTHONSTARTUP": true, "PYTHONHOME": true, "PYTHONWARNINGS": true,
	"PYTHONBREAKPOINT": true, "PYTHONINSPECT": true,
	"RUSTC": true, "RUSTC_WRAPPER": true, "RUSTC_WORKSPACE_WRAPPER": true,
	"RUSTFLAGS": true, "RUSTDOCFLAGS": true,
	"CARGO_BUILD_RUSTC": true, "CARGO_BUILD_RUSTC_WRAPPER": true,
	"PS4": true, "PROMPT_COMMAND": true, "BASH_ENV": true, "ENV": true,
	"CDPATH": true, "GLOBIGNORE": true, "SHELLOPTS": true, "BASHOPTS": true,
	"BASH_XTRACEFD": true,
	"LD_PRELOAD":    true, "LD_LIBRARY_PATH": true,
	"DYLD_INSERT_LIBRARIES": true, "DYLD_LIBRARY_PATH": true,
	"NODE_OPTIONS": true, "NODE_PATH": true,
	"PERL5OPT": true, "PERL5LIB": true, "PERLLIB": true, "PERL5DB": true,
	"RUBYOPT": true, "RUBYLIB": true,
	"PHP_INI_SCAN_DIR": true, "PHPRC": true,
	"R_PROFILE": true, "R_PROFILE_USER": true, "R_HOME": true,
	"ERL_AFLAGS": true, "ERL_FLAGS": true, "ELIXIR_ERL_OPTIONS": true,
	"OPENSSL_CONF": true, "OPENSSL_ENGINES": true,
	"GOFLAGS": true, "CGO_CFLAGS": true, "CGO_LDFLAGS": true,
	"CC": true, "CXX": true, "AR": true,
	"EDITOR": true, "VISUAL": true, "PAGER": true,
	"LESS": true, "LESSOPEN": true, "LESSCLOSE": true,
	"INPUTRC": true,
}

// envDenyPrefixes contains prefixes of environment variable names that should
// be stripped. Any env var whose name starts with one of these is denied.
var envDenyPrefixes = []string{
	"BASH_FUNC_",
}

// PolyglotExecutor manages subprocess execution for 11 language runtimes.
type PolyglotExecutor struct {
	maxOutputBytes int
	hardCapBytes   int
	projectRoot    string
	runtimes       RuntimeMap

	mu               sync.Mutex
	backgroundedPids map[int]struct{}
}

// Options configures the executor.
type Options struct {
	MaxOutputBytes int
	HardCapBytes   int
	ProjectRoot    string
	Runtimes       RuntimeMap
}

// New creates a PolyglotExecutor.
func New(opts *Options) *PolyglotExecutor {
	pe := &PolyglotExecutor{
		maxOutputBytes:   DefaultMaxOutputBytes,
		hardCapBytes:     DefaultHardCapBytes,
		backgroundedPids: make(map[int]struct{}),
	}

	if opts != nil {
		if opts.MaxOutputBytes > 0 {
			pe.maxOutputBytes = opts.MaxOutputBytes
		}

		if opts.HardCapBytes > 0 {
			pe.hardCapBytes = opts.HardCapBytes
		}

		if opts.ProjectRoot != "" {
			pe.projectRoot = opts.ProjectRoot
		}

		if opts.Runtimes != nil {
			pe.runtimes = opts.Runtimes
		}
	}

	if pe.projectRoot == "" {
		pe.projectRoot, _ = os.Getwd()
	}

	if pe.runtimes == nil {
		pe.runtimes = DetectRuntimes()
	}

	return pe
}

// Runtimes returns the detected runtime map.
func (pe *PolyglotExecutor) Runtimes() RuntimeMap {
	return pe.runtimes
}

// Execute runs code in the specified language.
func (pe *PolyglotExecutor) Execute(ctx context.Context, language, code string, timeout time.Duration, background bool) (model.ExecResult, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	tmpDir, err := os.MkdirTemp("", "thimble-exec-")
	if err != nil {
		return model.ExecResult{}, fmt.Errorf("mkdtemp: %w", err)
	}

	filePath, err := pe.writeScript(tmpDir, code, language)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return model.ExecResult{}, err
	}

	cmd, err := BuildCommand(pe.runtimes, language, filePath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return model.ExecResult{}, err
	}

	// Rust: compile then run.
	if cmd[0] == "__rust_compile_run__" {
		result := pe.compileAndRun(ctx, filePath, tmpDir, timeout)
		_ = os.RemoveAll(tmpDir)

		return result, nil
	}

	// Shell runs in project dir; other languages run in temp dir.
	cwd := tmpDir
	if language == "shell" {
		cwd = pe.projectRoot
	}

	result := pe.spawn(ctx, cmd, cwd, timeout, background)

	if !result.Backgrounded {
		_ = os.RemoveAll(tmpDir)
	}

	return result, nil
}

// ExecuteFile runs code with a file path injected as a variable.
func (pe *PolyglotExecutor) ExecuteFile(ctx context.Context, path, language, code string, timeout time.Duration) (model.ExecResult, error) {
	absolutePath := path
	if !filepath.IsAbs(path) {
		absolutePath = filepath.Join(pe.projectRoot, path)
	}

	wrappedCode := wrapWithFileContent(absolutePath, language, code)

	return pe.Execute(ctx, language, wrappedCode, timeout, false)
}

// CleanupBackgrounded kills all backgrounded processes.
func (pe *PolyglotExecutor) CleanupBackgrounded() {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	for pid := range pe.backgroundedPids {
		killPid(pid)
	}

	pe.backgroundedPids = make(map[int]struct{})
}

// fileExtension returns the file extension for a language.
func fileExtension(language string) string {
	switch language {
	case "javascript":
		return "js"
	case "typescript":
		return "ts"
	case "python":
		return "py"
	case "shell":
		return "sh"
	case "ruby":
		return "rb"
	case "go":
		return "go"
	case "rust":
		return "rs"
	case "php":
		return "php"
	case "perl":
		return "pl"
	case "r":
		return "R"
	case "elixir":
		return "exs"
	default:
		return "txt"
	}
}

func (pe *PolyglotExecutor) writeScript(tmpDir, code, language string) (string, error) {
	// Go: wrap if no package declaration.
	if language == "go" && !strings.Contains(code, "package ") {
		code = "package main\n\nimport \"fmt\"\n\nfunc main() {\n" + code + "\n}\n"
	}
	// PHP: add opening tag if missing.
	if language == "php" && !strings.HasPrefix(strings.TrimSpace(code), "<?") {
		code = "<?php\n" + code
	}

	fp := filepath.Join(tmpDir, "script."+fileExtension(language))

	perm := os.FileMode(0o644)
	if language == "shell" {
		perm = 0o700
	}

	if err := os.WriteFile(fp, []byte(code), perm); err != nil {
		return "", fmt.Errorf("write script: %w", err)
	}

	return fp, nil
}

func (pe *PolyglotExecutor) compileAndRun(ctx context.Context, srcPath, cwd string, timeout time.Duration) model.ExecResult {
	binSuffix := ""
	if runtime.GOOS == "windows" {
		binSuffix = ".exe"
	}

	binPath := strings.TrimSuffix(srcPath, ".rs") + binSuffix

	compileCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	compileCmd := exec.CommandContext(compileCtx, "rustc", srcPath, "-o", binPath)
	compileCmd.Dir = cwd

	var stderr bytes.Buffer

	compileCmd.Stderr = &stderr

	if err := compileCmd.Run(); err != nil {
		return model.ExecResult{
			Stderr:   "Compilation failed:\n" + stderr.String(),
			ExitCode: 1,
		}
	}

	return pe.spawn(ctx, []string{binPath}, cwd, timeout, false)
}

// cappedWriter is an io.Writer that silently discards data once a shared hard
// cap is exceeded.  The exceeded flag and total counter are shared between the
// stdout and stderr writers so a single process cannot exhaust memory by
// flooding either stream.
type cappedWriter struct {
	buf      bytes.Buffer
	hardCap  int
	exceeded *syncatomic.Bool
	totalCtr *syncatomic.Int64
}

func newCappedWriterPair(hardCap int) (*cappedWriter, *cappedWriter) {
	exceeded := &syncatomic.Bool{}
	totalCtr := &syncatomic.Int64{}

	return &cappedWriter{hardCap: hardCap, exceeded: exceeded, totalCtr: totalCtr},
		&cappedWriter{hardCap: hardCap, exceeded: exceeded, totalCtr: totalCtr}
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if w.exceeded.Load() {
		return len(p), nil
	}

	newTotal := w.totalCtr.Add(int64(len(p)))
	if int(newTotal) > w.hardCap {
		w.exceeded.Store(true)
		return len(p), nil
	}

	return w.buf.Write(p)
}

func (pe *PolyglotExecutor) spawn(ctx context.Context, cmd []string, cwd string, timeout time.Duration, background bool) model.ExecResult {
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := exec.CommandContext(execCtx, cmd[0], cmd[1:]...)
	c.Dir = cwd
	c.Env = pe.buildSafeEnv(cwd)

	setProcGroup(c)
	c.Cancel = func() error { killProcessGroup(c); return nil }
	c.WaitDelay = 2 * time.Second

	stdoutCap, stderrCap := newCappedWriterPair(pe.hardCapBytes)
	c.Stdout = stdoutCap
	c.Stderr = stderrCap

	err := c.Run()

	timedOut := execCtx.Err() == context.DeadlineExceeded

	// If the hard cap was exceeded, treat it like a timeout/kill.
	if stdoutCap.exceeded.Load() {
		timedOut = true
	}

	if background && timedOut {
		if c.Process != nil {
			pe.mu.Lock()
			pe.backgroundedPids[c.Process.Pid] = struct{}{}
			pe.mu.Unlock()
		}

		return model.ExecResult{
			Stdout:       smartTruncate(stdoutCap.buf.String(), pe.maxOutputBytes),
			Stderr:       smartTruncate(stderrCap.buf.String(), pe.maxOutputBytes),
			ExitCode:     0,
			TimedOut:     true,
			Backgrounded: true,
		}
	}

	exitCode := 0

	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	if timedOut {
		exitCode = 1
	}

	return model.ExecResult{
		Stdout:   smartTruncate(stdoutCap.buf.String(), pe.maxOutputBytes),
		Stderr:   smartTruncate(stderrCap.buf.String(), pe.maxOutputBytes),
		ExitCode: exitCode,
		TimedOut: timedOut,
	}
}

func (pe *PolyglotExecutor) buildSafeEnv(tmpDir string) []string {
	isWin := runtime.GOOS == "windows"

	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}

	if home == "" {
		home = tmpDir
	}

	env := map[string]string{
		"PATH":                    os.Getenv("PATH"),
		"HOME":                    home,
		"TMPDIR":                  tmpDir,
		"LANG":                    "en_US.UTF-8",
		"PYTHONDONTWRITEBYTECODE": "1",
		"PYTHONUNBUFFERED":        "1",
		"PYTHONUTF8":              "1",
		"NO_COLOR":                "1",
	}

	if isWin {
		for _, key := range []string{"SYSTEMROOT", "SystemRoot", "COMSPEC", "PATHEXT",
			"USERPROFILE", "APPDATA", "LOCALAPPDATA", "TEMP", "TMP"} {
			if v := os.Getenv(key); v != "" {
				env[key] = v
			}
		}

		env["MSYS_NO_PATHCONV"] = "1"
		env["MSYS2_ARG_CONV_EXCL"] = "*"

		gitUsrBin := `C:\Program Files\Git\usr\bin`

		gitBin := `C:\Program Files\Git\bin`
		if !strings.Contains(env["PATH"], gitUsrBin) {
			env["PATH"] = gitUsrBin + ";" + gitBin + ";" + env["PATH"]
		}
	}

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
		if v := os.Getenv(key); v != "" {
			env[key] = v
		}
	}

	// Auto-detect CA certificate bundle on non-Windows systems.
	if !isWin && env["SSL_CERT_FILE"] == "" {
		if caPath := detectCACertPath(); caPath != "" {
			env["SSL_CERT_FILE"] = caPath
			if env["NODE_EXTRA_CA_CERTS"] == "" {
				env["NODE_EXTRA_CA_CERTS"] = caPath
			}
		}
	}

	// Elixir: inject BEAM ebin paths from _build.
	if pe.runtimes["elixir"] != "" {
		if ebinPaths := detectElixirEbinPaths(pe.projectRoot); ebinPaths != "" {
			env["ERL_LIBS"] = ebinPaths
		}
	}

	// Strip denied environment variables.
	for k := range envDenyList {
		delete(env, k)
	}

	// Strip env vars matching denied prefixes.
	for k := range env {
		for _, prefix := range envDenyPrefixes {
			if strings.HasPrefix(k, prefix) {
				delete(env, k)
				break
			}
		}
	}

	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}

	return result
}

// detectCACertPath returns the first existing CA certificate bundle path.
func detectCACertPath() string {
	paths := []string{
		"/etc/ssl/certs/ca-certificates.crt",
		"/etc/pki/tls/certs/ca-bundle.crt",
		"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
		"/etc/ssl/cert.pem",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Fallback: check for the certs directory itself.
	if info, err := os.Stat("/etc/ssl/certs"); err == nil && info.IsDir() {
		return "/etc/ssl/certs"
	}

	return ""
}

// detectElixirEbinPaths scans _build/dev/lib/*/ebin and returns a
// colon-separated list suitable for ERL_LIBS.
func detectElixirEbinPaths(projectRoot string) string {
	pattern := filepath.Join(projectRoot, "_build", "dev", "lib", "*", "ebin")

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	// ERL_LIBS expects the parent dirs (lib/<app>), not the ebin dirs.
	seen := map[string]struct{}{}

	var parts []string

	for _, m := range matches {
		parent := filepath.Dir(m) // …/lib/<app>
		if _, ok := seen[parent]; !ok {
			seen[parent] = struct{}{}
			parts = append(parts, parent)
		}
	}

	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}

	return strings.Join(parts, sep)
}

// wrapWithFileContent prepends code to read a file into FILE_CONTENT variable.
func wrapWithFileContent(absolutePath, language, code string) string {
	escaped, err := json.Marshal(absolutePath)
	if err != nil {
		escaped = []byte(`"` + absolutePath + `"`)
	}

	ep := string(escaped)

	switch language {
	case "javascript", "typescript":
		return fmt.Sprintf("const FILE_CONTENT_PATH = %s;\nconst file_path = FILE_CONTENT_PATH;\nconst FILE_CONTENT = require(\"fs\").readFileSync(FILE_CONTENT_PATH, \"utf-8\");\n%s", ep, code)
	case "python":
		return fmt.Sprintf("FILE_CONTENT_PATH = %s\nfile_path = FILE_CONTENT_PATH\nwith open(FILE_CONTENT_PATH, \"r\", encoding=\"utf-8\") as _f:\n    FILE_CONTENT = _f.read()\n%s", ep, code)
	case "shell":
		sq := "'" + strings.ReplaceAll(absolutePath, "'", "'\\''") + "'"
		return fmt.Sprintf("FILE_CONTENT_PATH=%s\nfile_path=%s\nFILE_CONTENT=$(cat %s)\n%s", sq, sq, sq, code)
	case "ruby":
		return fmt.Sprintf("FILE_CONTENT_PATH = %s\nfile_path = FILE_CONTENT_PATH\nFILE_CONTENT = File.read(FILE_CONTENT_PATH, encoding: \"utf-8\")\n%s", ep, code)
	case "go":
		return fmt.Sprintf("package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nvar FILE_CONTENT_PATH = %s\nvar file_path = FILE_CONTENT_PATH\n\nfunc main() {\n\tb, _ := os.ReadFile(FILE_CONTENT_PATH)\n\tFILE_CONTENT := string(b)\n\t_ = FILE_CONTENT\n\t_ = fmt.Sprint()\n%s\n}\n", ep, code)
	case "rust":
		return fmt.Sprintf("use std::fs;\n\nfn main() {\n    let file_content_path = %s;\n    let file_path = file_content_path;\n    let file_content = fs::read_to_string(file_content_path).unwrap();\n%s\n}\n", ep, code)
	case "php":
		return fmt.Sprintf("<?php\n$FILE_CONTENT_PATH = %s;\n$file_path = $FILE_CONTENT_PATH;\n$FILE_CONTENT = file_get_contents($FILE_CONTENT_PATH);\n%s", ep, code)
	case "perl":
		return fmt.Sprintf("my $FILE_CONTENT_PATH = %s;\nmy $file_path = $FILE_CONTENT_PATH;\nopen(my $fh, '<:encoding(UTF-8)', $FILE_CONTENT_PATH) or die \"Cannot open: $!\";\nmy $FILE_CONTENT = do { local $/; <$fh> };\nclose($fh);\n%s", ep, code)
	case "r":
		return fmt.Sprintf("FILE_CONTENT_PATH <- %s\nfile_path <- FILE_CONTENT_PATH\nFILE_CONTENT <- readLines(FILE_CONTENT_PATH, warn=FALSE, encoding=\"UTF-8\")\nFILE_CONTENT <- paste(FILE_CONTENT, collapse=\"\\n\")\n%s", ep, code)
	case "elixir":
		return fmt.Sprintf("file_content_path = %s\nfile_path = file_content_path\nfile_content = File.read!(file_content_path)\n%s", ep, code)
	default:
		return code
	}
}
