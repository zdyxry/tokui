package render

import (
	"sort"
	"strings"

	"github.com/zdyxry/tokui/structure"
)

// updateLanguages collects available languages from current and child entries
func (dm *DirModel) updateLanguages() {
	if dm.nav.Entry() == nil {
		return
	}
	langs := make(map[string]struct{})
	for lang := range dm.nav.Entry().StatsByLang {
		langs[lang] = struct{}{}
	}
	for _, child := range dm.nav.Entry().Child {
		for lang := range child.StatsByLang {
			langs[lang] = struct{}{}
		}
	}

	dm.languages = make([]string, 0, len(langs))
	for lang := range langs {
		dm.languages = append(dm.languages, lang)
	}
	sort.Strings(dm.languages)
}

func (dm *DirModel) formatTreeName(entry *structure.Entry, depth int) string {
	var prefix string
	if entry.IsDir {
		if entry.Expanded {
			prefix = "▾ "
		} else if entry.HasChild() {
			prefix = "▸ "
		} else {
			prefix = "  "
		}
	} else {
		prefix = "  "
	}
	indent := strings.Repeat("  ", depth)
	return indent + prefix + entry.Name()
}

// useMultiLangFilter returns true when one or more languages are selected
// via the multi-language selection overlay.
func (dm *DirModel) useMultiLangFilter() bool {
	if dm.selectedLangs == nil {
		return false
	}
	for _, lang := range dm.languages {
		if dm.selectedLangs[lang] {
			return true
		}
	}
	return false
}

// activeLang returns the currently cycled single-language filter value.
// It returns "" when "All" is selected or when multi-language filter is active.
func (dm *DirModel) activeLang() string {
	if dm.useMultiLangFilter() {
		return ""
	}
	if dm.langFilterIdx > -1 && dm.langFilterIdx < len(dm.languages) {
		return dm.languages[dm.langFilterIdx]
	}
	return ""
}

// selectedLangs returns the list of languages selected in multi-select mode.
func (dm *DirModel) selectedLangsList() []string {
	if dm.selectedLangs == nil {
		return nil
	}
	langs := make([]string, 0)
	for _, lang := range dm.languages {
		if dm.selectedLangs[lang] {
			langs = append(langs, lang)
		}
	}
	return langs
}

// statusLangLabel returns the human-readable language filter label shown in
// the status bar: "All", the single filtered language, or the comma-separated
// list of selected languages.
func (dm *DirModel) statusLangLabel() string {
	if dm.useMultiLangFilter() {
		return strings.Join(dm.selectedLangsList(), ", ")
	}
	if lang := dm.activeLang(); lang != "" {
		return lang
	}
	return "All"
}

// comparableStats returns the CodeStats that should be used for both display
// and sorting under the current language filter. For single-language filter it
// returns that language's stats; for multi-language filter it aggregates the
// selected languages; otherwise it returns the entry's total stats.
func (dm *DirModel) comparableStats(e *structure.Entry) structure.CodeStats {
	if !dm.useMultiLangFilter() {
		return e.GetStats(dm.activeLang())
	}
	var sum structure.CodeStats
	for _, lang := range dm.selectedLangsList() {
		sum.Add(e.GetStats(lang))
	}
	return sum
}

// copyLangSelection returns a shallow copy of the given language selection map.
func copyLangSelection(src map[string]bool) map[string]bool {
	if src == nil {
		return nil
	}
	dst := make(map[string]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
