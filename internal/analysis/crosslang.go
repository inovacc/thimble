package analysis

import (
	"regexp"
	"strings"
)

// crossLangKind is the reference kind for cross-language invocations.
const crossLangKind = "cross-lang-call"

// langPattern pairs a compiled regex with a function that extracts the target
// language and command from a match.
type langPattern struct {
	re     *regexp.Regexp
	target func(match []string) (lang, command string)
}

// goPatterns detects exec.Command / exec.CommandContext calls in Go source.
var goPatterns = []langPattern{
	{
		// exec.Command("python", "script.py", ...) or exec.CommandContext(ctx, "python", ...)
		re: regexp.MustCompile(`exec\.Command(?:Context)?\([^,]*?,?\s*"(python3?|node|npx|cargo|rustc|ruby|php|perl|bash|sh)"(?:\s*,\s*"([^"]*)")?`),
		target: func(m []string) (string, string) {
			cmd := m[1]

			arg := ""
			if len(m) > 2 {
				arg = m[2]
			}

			lang := commandToLang(cmd)

			return lang, buildTarget(cmd, arg)
		},
	},
}

// pythonPatterns detects subprocess/os.system calls in Python source.
var pythonPatterns = []langPattern{
	{
		// subprocess.run(["go", "run", ...]) or subprocess.call(["node", ...])
		re: regexp.MustCompile(`subprocess\.(?:run|call|check_call|check_output|Popen)\(\s*\[\s*"(go|node|npx|cargo|rustc|python3?|ruby|php|perl|bash|sh)"(?:\s*,\s*"([^"]*)")?`),
		target: func(m []string) (string, string) {
			cmd := m[1]

			arg := ""
			if len(m) > 2 {
				arg = m[2]
			}

			lang := commandToLang(cmd)

			return lang, buildTarget(cmd, arg)
		},
	},
	{
		// os.system("cargo run ...") or os.system("python script.py")
		re: regexp.MustCompile(`os\.system\(\s*["'](go|node|npx|cargo|rustc|python3?|ruby|php|perl|bash|sh)\s+([^\s"']*)`),
		target: func(m []string) (string, string) {
			cmd := m[1]

			arg := ""
			if len(m) > 2 {
				arg = m[2]
			}

			lang := commandToLang(cmd)

			return lang, buildTarget(cmd, arg)
		},
	},
}

// tsPatterns detects child_process calls in TypeScript/JavaScript source.
var tsPatterns = []langPattern{
	{
		// execSync("go run ...") or exec("python script.py")
		re: regexp.MustCompile(`exec(?:Sync)?\(\s*["'](go|node|npx|cargo|rustc|python3?|ruby|php|perl|bash|sh)\s+([^\s"']*)`),
		target: func(m []string) (string, string) {
			cmd := m[1]

			arg := ""
			if len(m) > 2 {
				arg = m[2]
			}

			lang := commandToLang(cmd)

			return lang, buildTarget(cmd, arg)
		},
	},
	{
		// spawn("cargo", ["run", ...]) or spawnSync("python", [...])
		re: regexp.MustCompile(`spawn(?:Sync)?\(\s*["'](go|node|npx|cargo|rustc|python3?|ruby|php|perl|bash|sh)["'](?:\s*,\s*\[\s*["']([^"']*))?`),
		target: func(m []string) (string, string) {
			cmd := m[1]

			arg := ""
			if len(m) > 2 {
				arg = m[2]
			}

			lang := commandToLang(cmd)

			return lang, buildTarget(cmd, arg)
		},
	},
}

// shellPatterns detects direct invocations in shell scripts.
var shellPatterns = []langPattern{
	{
		// python script.py, python3 -m module, node app.js, go run main.go, cargo run
		re: regexp.MustCompile(`(?:^|\s|;|&&|\|\|)(python3?|node|npx|go|cargo|rustc|ruby|php|perl)\s+([^\s;|&]+)`),
		target: func(m []string) (string, string) {
			cmd := m[1]

			arg := ""
			if len(m) > 2 {
				arg = m[2]
			}

			lang := commandToLang(cmd)

			return lang, buildTarget(cmd, arg)
		},
	},
}

// rustPatterns detects Command::new calls in Rust source.
var rustPatterns = []langPattern{
	{
		// Command::new("python") or Command::new("node")
		re: regexp.MustCompile(`Command::new\(\s*"(python3?|node|npx|cargo|rustc|go|ruby|php|perl|bash|sh)"`),
		target: func(m []string) (string, string) {
			cmd := m[1]
			lang := commandToLang(cmd)

			return lang, cmd
		},
	},
}

// LangShell represents shell script files (.sh, .bash).
const LangShell Language = "shell"

// ExtractCrossLangRefs scans source code for cross-language invocation patterns.
// Returns references with Kind = "cross-lang-call" and To = "lang:command".
func ExtractCrossLangRefs(path string, language Language, code string) []Reference {
	var patterns []langPattern

	switch language {
	case LangGo:
		patterns = goPatterns
	case LangPython:
		patterns = pythonPatterns
	case LangTypeScript:
		patterns = tsPatterns
	case LangShell:
		patterns = shellPatterns
	case LangRust:
		patterns = rustPatterns
	case LangProto, LangC, LangJava:
		return nil
	default:
		return nil
	}

	lines := strings.Split(code, "\n")

	var refs []Reference

	for lineIdx, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comments to avoid false positives.
		if isComment(trimmed, language) {
			continue
		}

		for _, p := range patterns {
			matches := p.re.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				lang, command := p.target(match)
				if lang == "" {
					continue
				}

				refs = append(refs, Reference{
					From: path,
					To:   lang + ":" + command,
					Kind: crossLangKind,
					File: path,
					Line: lineIdx + 1,
				})
			}
		}
	}

	return refs
}

// isComment checks if a trimmed line is a comment for the given language.
func isComment(trimmed string, lang Language) bool {
	switch lang {
	case LangGo, LangTypeScript, LangRust:
		return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*")
	case LangPython, LangShell:
		return strings.HasPrefix(trimmed, "#")
	case LangProto, LangC, LangJava:
		return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*")
	default:
		return false
	}
}

// commandToLang maps an executable name to a language identifier.
func commandToLang(cmd string) string {
	switch cmd {
	case "python", "python3":
		return "python"
	case "node", "npx":
		return "node"
	case "cargo", "rustc":
		return "rust"
	case "go":
		return "go"
	case "ruby":
		return "ruby"
	case "php":
		return "php"
	case "perl":
		return "perl"
	case "bash", "sh":
		return "shell"
	default:
		return ""
	}
}

// buildTarget creates a target string from a command and its first argument.
func buildTarget(cmd, arg string) string {
	if arg == "" {
		return cmd
	}

	return cmd + " " + arg
}
