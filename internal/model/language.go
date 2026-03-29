package model

import "slices"

// Language represents a supported execution language.
type Language string

const (
	LangJavaScript Language = "javascript"
	LangTypeScript Language = "typescript"
	LangPython     Language = "python"
	LangShell      Language = "shell"
	LangRuby       Language = "ruby"
	LangGo         Language = "go"
	LangRust       Language = "rust"
	LangPHP        Language = "php"
	LangPerl       Language = "perl"
	LangR          Language = "r"
	LangElixir     Language = "elixir"
)

// AllLanguages is the complete list of supported languages.
var AllLanguages = []Language{
	LangJavaScript, LangTypeScript, LangPython, LangShell,
	LangRuby, LangGo, LangRust, LangPHP, LangPerl, LangR, LangElixir,
}

// Valid returns true if the language is recognized.
func (l Language) Valid() bool {
	return slices.Contains(AllLanguages, l)
}

// FileExtension returns the file extension for the language.
func (l Language) FileExtension() string {
	switch l {
	case LangJavaScript:
		return ".js"
	case LangTypeScript:
		return ".ts"
	case LangPython:
		return ".py"
	case LangShell:
		return ".sh"
	case LangRuby:
		return ".rb"
	case LangGo:
		return ".go"
	case LangRust:
		return ".rs"
	case LangPHP:
		return ".php"
	case LangPerl:
		return ".pl"
	case LangR:
		return ".R"
	case LangElixir:
		return ".exs"
	default:
		return ".txt"
	}
}
