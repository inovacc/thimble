package security

import (
	"regexp"
	"strings"
)

// Shell-escape patterns for detecting shell calls in non-shell languages.
// Go's RE2 engine doesn't support backreferences, so we use separate patterns
// for single-quoted and double-quoted strings.
var shellEscapePatterns = map[string][]*regexp.Regexp{
	"python": {
		regexp.MustCompile(`os\.system\(\s*'([^']*?)'\s*\)`),
		regexp.MustCompile(`os\.system\(\s*"([^"]*?)"\s*\)`),
		regexp.MustCompile(`subprocess\.(?:run|call|Popen|check_output|check_call)\(\s*'([^']*?)'`),
		regexp.MustCompile(`subprocess\.(?:run|call|Popen|check_output|check_call)\(\s*"([^"]*?)"`),
	},
	"javascript": {
		regexp.MustCompile(`exec(?:Sync|File|FileSync)?\(\s*'([^']*?)'`),
		regexp.MustCompile(`exec(?:Sync|File|FileSync)?\(\s*"([^"]*?)"`),
		regexp.MustCompile("exec(?:Sync|File|FileSync)?\\(\\s*`([^`]*?)`"),
		regexp.MustCompile(`spawn(?:Sync)?\(\s*'([^']*?)'`),
		regexp.MustCompile(`spawn(?:Sync)?\(\s*"([^"]*?)"`),
	},
	"typescript": {
		regexp.MustCompile(`exec(?:Sync|File|FileSync)?\(\s*'([^']*?)'`),
		regexp.MustCompile(`exec(?:Sync|File|FileSync)?\(\s*"([^"]*?)"`),
		regexp.MustCompile("exec(?:Sync|File|FileSync)?\\(\\s*`([^`]*?)`"),
		regexp.MustCompile(`spawn(?:Sync)?\(\s*'([^']*?)'`),
		regexp.MustCompile(`spawn(?:Sync)?\(\s*"([^"]*?)"`),
	},
	"ruby": {
		regexp.MustCompile(`system\(\s*'([^']*?)'`),
		regexp.MustCompile(`system\(\s*"([^"]*?)"`),
		regexp.MustCompile("`([^`]*?)`"),
	},
	"go": {
		regexp.MustCompile(`exec\.Command\(\s*"([^"]*?)"`),
	},
	"php": {
		regexp.MustCompile(`shell_exec\(\s*'([^']*?)'`),
		regexp.MustCompile(`shell_exec\(\s*"([^"]*?)"`),
		regexp.MustCompile(`(?:^|[^.])exec\(\s*'([^']*?)'`),
		regexp.MustCompile(`(?:^|[^.])exec\(\s*"([^"]*?)"`),
		regexp.MustCompile(`(?:^|[^.])system\(\s*'([^']*?)'`),
		regexp.MustCompile(`(?:^|[^.])system\(\s*"([^"]*?)"`),
		regexp.MustCompile(`passthru\(\s*'([^']*?)'`),
		regexp.MustCompile(`passthru\(\s*"([^"]*?)"`),
		regexp.MustCompile(`proc_open\(\s*'([^']*?)'`),
		regexp.MustCompile(`proc_open\(\s*"([^"]*?)"`),
	},
	"rust": {
		regexp.MustCompile(`Command::new\(\s*"([^"]*?)"`),
	},
	"perl": {
		regexp.MustCompile(`(?:^|[^.])system\(\s*'([^']*?)'`),
		regexp.MustCompile(`(?:^|[^.])system\(\s*"([^"]*?)"`),
		regexp.MustCompile(`(?:^|[^.])exec\(\s*'([^']*?)'`),
		regexp.MustCompile(`(?:^|[^.])exec\(\s*"([^"]*?)"`),
		regexp.MustCompile("`([^`]*?)`"),
		regexp.MustCompile(`qx\{([^}]*?)\}`),
		regexp.MustCompile(`qx\(([^)]*?)\)`),
	},
	"r": {
		regexp.MustCompile(`(?:^|[^.])system\(\s*'([^']*?)'`),
		regexp.MustCompile(`(?:^|[^.])system\(\s*"([^"]*?)"`),
		regexp.MustCompile(`system2\(\s*'([^']*?)'`),
		regexp.MustCompile(`system2\(\s*"([^"]*?)"`),
		regexp.MustCompile(`shell\(\s*'([^']*?)'`),
		regexp.MustCompile(`shell\(\s*"([^"]*?)"`),
	},
	"elixir": {
		regexp.MustCompile(`System\.cmd\(\s*"([^"]*?)"`),
		regexp.MustCompile(`:os\.cmd\(\s*'([^']*?)'`),
	},
}

var pythonSubprocessListRe = regexp.MustCompile(`subprocess\.(?:run|call|Popen|check_output|check_call)\(\s*\[([^\]]+)\]`)
var pythonListArgRe = regexp.MustCompile(`['"]([^'"]*?)['"]`)

// ExtractShellCommands scans non-shell code for shell-escape calls and extracts
// the embedded command strings.
func ExtractShellCommands(code, language string) []string {
	patterns := shellEscapePatterns[language]
	if patterns == nil && language != "python" {
		return nil
	}

	var commands []string

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(code, -1)
		for _, m := range matches {
			// Command is in the last capture group.
			cmd := m[len(m)-1]
			if cmd != "" {
				commands = append(commands, cmd)
			}
		}
	}

	// Python: also extract subprocess list-form args.
	if language == "python" {
		for _, m := range pythonSubprocessListRe.FindAllStringSubmatch(code, -1) {
			listContent := m[1]
			argMatches := pythonListArgRe.FindAllStringSubmatch(listContent, -1)

			var args []string
			for _, am := range argMatches {
				args = append(args, am[1])
			}

			if len(args) > 0 {
				commands = append(commands, strings.Join(args, " "))
			}
		}
	}

	return commands
}
