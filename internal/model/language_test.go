package model

import "testing"

func TestLanguageValid(t *testing.T) {
	tests := []struct {
		lang Language
		want bool
	}{
		{LangJavaScript, true},
		{LangTypeScript, true},
		{LangPython, true},
		{LangShell, true},
		{LangRuby, true},
		{LangGo, true},
		{LangRust, true},
		{LangPHP, true},
		{LangPerl, true},
		{LangR, true},
		{LangElixir, true},
		{Language("cobol"), false},
		{Language(""), false},
		{Language("Java"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			if got := tt.lang.Valid(); got != tt.want {
				t.Errorf("Language(%q).Valid() = %v, want %v", tt.lang, got, tt.want)
			}
		})
	}
}

func TestLanguageFileExtension(t *testing.T) {
	tests := []struct {
		lang Language
		want string
	}{
		{LangJavaScript, ".js"},
		{LangTypeScript, ".ts"},
		{LangPython, ".py"},
		{LangShell, ".sh"},
		{LangRuby, ".rb"},
		{LangGo, ".go"},
		{LangRust, ".rs"},
		{LangPHP, ".php"},
		{LangPerl, ".pl"},
		{LangR, ".R"},
		{LangElixir, ".exs"},
		{Language("unknown"), ".txt"},
		{Language(""), ".txt"},
	}

	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			if got := tt.lang.FileExtension(); got != tt.want {
				t.Errorf("Language(%q).FileExtension() = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestAllLanguagesContainsAll(t *testing.T) {
	expected := []Language{
		LangJavaScript, LangTypeScript, LangPython, LangShell,
		LangRuby, LangGo, LangRust, LangPHP, LangPerl, LangR, LangElixir,
	}

	if len(AllLanguages) != len(expected) {
		t.Errorf("AllLanguages has %d entries, want %d", len(AllLanguages), len(expected))
	}

	for _, lang := range expected {
		if !lang.Valid() {
			t.Errorf("expected language %q to be valid", lang)
		}
	}
}
