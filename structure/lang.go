package structure

import (
	"path/filepath"
	"strings"
)

// extLanguages maps common file extensions to language names. It is used only
// for files missing from the provider result (e.g. deleted files in Diff
// mode), where the language cannot come from the S2 snapshot. Unknown
// extensions fall back to "Other".
var extLanguages = map[string]string{
	"go":   "Go",
	"py":   "Python",
	"js":   "JavaScript",
	"jsx":  "JavaScript",
	"ts":   "TypeScript",
	"tsx":  "TypeScript",
	"java": "Java",
	"c":    "C",
	"h":    "C",
	"cpp":  "C++",
	"cc":   "C++",
	"cxx":  "C++",
	"hpp":  "C++",
	"rs":   "Rust",
	"rb":   "Ruby",
	"md":   "Markdown",
	"json": "JSON",
	"yaml": "YAML",
	"yml":  "YAML",
	"toml": "TOML",
	"html": "HTML",
	"css":  "CSS",
	"sh":   "Shell",
}

// languageByExt guesses the language of a file from its extension, returning
// "Other" for unknown extensions.
func languageByExt(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if lang, ok := extLanguages[ext]; ok {
		return lang
	}
	return "Other"
}
