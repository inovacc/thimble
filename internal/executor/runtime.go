// Package executor implements the PolyglotExecutor — subprocess management
// for 11 language runtimes.
package executor

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// RuntimeMap maps language names to their detected runtime commands.
// An empty string means the runtime is not available.
type RuntimeMap map[string]string

// commandExists checks if a command is available in PATH.
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// resolveWindowsBash finds a non-WSL bash on Windows.
func resolveWindowsBash() string {
	knownPaths := []string{
		`C:\Program Files\Git\usr\bin\bash.exe`,
		`C:\Program Files (x86)\Git\usr\bin\bash.exe`,
	}
	for _, p := range knownPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Fallback: scan PATH via `where bash`, skipping WSL entries.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "where", "bash").Output()
	if err == nil {
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)

			lower := strings.ToLower(line)
			if strings.Contains(lower, "system32") || strings.Contains(lower, "windowsapps") {
				continue
			}

			return line
		}
	}

	return ""
}

// DetectRuntimes probes the system for available language runtimes.
func DetectRuntimes() RuntimeMap {
	isWin := runtime.GOOS == "windows"
	hasBun := commandExists("bun")

	m := RuntimeMap{}

	// JavaScript
	if hasBun {
		m["javascript"] = "bun"
	} else {
		m["javascript"] = "node"
	}

	// TypeScript
	switch {
	case hasBun:
		m["typescript"] = "bun"
	case commandExists("tsx"):
		m["typescript"] = "tsx"
	case commandExists("ts-node"):
		m["typescript"] = "ts-node"
	}

	// Python
	switch {
	case commandExists("python3"):
		m["python"] = "python3"
	case commandExists("python"):
		m["python"] = "python"
	}

	// Shell
	if isWin {
		if bash := resolveWindowsBash(); bash != "" {
			m["shell"] = bash
		} else if commandExists("sh") { //nolint:gocritic // if-else chain is clearer for runtime detection
			m["shell"] = "sh"
		} else if commandExists("powershell") {
			m["shell"] = "powershell"
		} else {
			m["shell"] = "cmd.exe"
		}
	} else {
		if commandExists("bash") {
			m["shell"] = "bash"
		} else {
			m["shell"] = "sh"
		}
	}

	// Optional runtimes
	if commandExists("ruby") {
		m["ruby"] = "ruby"
	}

	if commandExists("go") {
		m["go"] = "go"
	}

	if commandExists("rustc") {
		m["rust"] = "rustc"
	}

	if commandExists("php") {
		m["php"] = "php"
	}

	if commandExists("perl") {
		m["perl"] = "perl"
	}

	if commandExists("Rscript") {
		m["r"] = "Rscript"
	} else if commandExists("r") {
		m["r"] = "r"
	}

	if commandExists("elixir") {
		m["elixir"] = "elixir"
	}

	return m
}

// BuildCommand returns the command and args to execute a script for a given language.
func BuildCommand(runtimes RuntimeMap, language, filePath string) ([]string, error) {
	switch language {
	case "javascript":
		if runtimes["javascript"] == "bun" {
			return []string{"bun", "run", filePath}, nil
		}

		return []string{"node", filePath}, nil

	case "typescript":
		rt := runtimes["typescript"]
		if rt == "" {
			return nil, &RuntimeError{Language: "typescript"}
		}

		if rt == "bun" {
			return []string{"bun", "run", filePath}, nil
		}

		cmd := []string{rt, filePath}
		if isCmdShim(rt) {
			cmd = wrapCmdShim(cmd)
		}

		return cmd, nil

	case "python":
		rt := runtimes["python"]
		if rt == "" {
			return nil, &RuntimeError{Language: "python"}
		}

		return []string{rt, filePath}, nil

	case "shell":
		return []string{runtimes["shell"], filePath}, nil

	case "ruby":
		if runtimes["ruby"] == "" {
			return nil, &RuntimeError{Language: "ruby"}
		}

		return []string{"ruby", filePath}, nil

	case "go":
		if runtimes["go"] == "" {
			return nil, &RuntimeError{Language: "go"}
		}

		return []string{"go", "run", filePath}, nil

	case "rust":
		if runtimes["rust"] == "" {
			return nil, &RuntimeError{Language: "rust"}
		}

		return []string{"__rust_compile_run__", filePath}, nil

	case "php":
		if runtimes["php"] == "" {
			return nil, &RuntimeError{Language: "php"}
		}

		return []string{"php", filePath}, nil

	case "perl":
		if runtimes["perl"] == "" {
			return nil, &RuntimeError{Language: "perl"}
		}

		return []string{"perl", filePath}, nil

	case "r":
		rt := runtimes["r"]
		if rt == "" {
			return nil, &RuntimeError{Language: "r"}
		}

		return []string{rt, filePath}, nil

	case "elixir":
		if runtimes["elixir"] == "" {
			return nil, &RuntimeError{Language: "elixir"}
		}

		cmd := []string{"elixir", filePath}
		if isCmdShim("elixir") {
			cmd = wrapCmdShim(cmd)
		}

		return cmd, nil

	default:
		return nil, &RuntimeError{Language: language}
	}
}

// isCmdShim returns true if the resolved path of a command points to a .cmd
// shim (common for npm-installed tools on Windows).
func isCmdShim(name string) bool {
	if runtime.GOOS != "windows" {
		return false
	}

	p, err := exec.LookPath(name)
	if err != nil {
		return false
	}

	lower := strings.ToLower(p)

	return strings.HasSuffix(lower, ".cmd") || strings.HasSuffix(lower, ".bat")
}

// wrapCmdShim wraps a .cmd shim invocation so it runs correctly via
// cmd.exe /C on Windows.
func wrapCmdShim(args []string) []string {
	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = "cmd.exe"
	}

	return append([]string{shell, "/C"}, args...)
}

// RuntimeError indicates a language runtime is not available.
type RuntimeError struct {
	Language string
}

func (e *RuntimeError) Error() string {
	return e.Language + " runtime not available"
}

// GetAvailableLanguages returns the list of languages with detected runtimes.
func GetAvailableLanguages(runtimes RuntimeMap) []string {
	langs := []string{"javascript", "shell"} // always available

	for _, lang := range []string{"typescript", "python", "ruby", "go", "rust", "php", "perl", "r", "elixir"} {
		if runtimes[lang] != "" {
			langs = append(langs, lang)
		}
	}

	return langs
}
