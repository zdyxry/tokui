// structure/entry.go
package structure

import (
	"cmp"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
)

type CodeStats struct {
	Code     int64
	Comments int64
	Blanks   int64
}

func (cs CodeStats) Total() int64 {
	return cs.Code + cs.Comments + cs.Blanks
}

func (cs *CodeStats) Add(other CodeStats) {
	cs.Code += other.Code
	cs.Comments += other.Comments
	cs.Blanks += other.Blanks
}

type Entry struct {
	Path        string
	Child       []*Entry
	mx          sync.RWMutex
	IsDir       bool
	StatsByLang map[string]CodeStats
	TotalStats  CodeStats
}

func NewDirEntry(path string) *Entry {
	return &Entry{
		Path:        path,
		Child:       make([]*Entry, 0),
		IsDir:       true,
		StatsByLang: make(map[string]CodeStats),
	}
}

func NewFileEntry(path string, stats map[string]CodeStats) *Entry {
	e := &Entry{
		Path:        path,
		StatsByLang: stats,
		IsDir:       false,
	}
	// Calculate file totals
	for _, s := range stats {
		e.TotalStats.Add(s)
	}
	return e
}

func (e *Entry) Name() string {
	return filepath.Base(e.Path)
}

func (e *Entry) Ext() string {
	// Using filepath.Ext is more robust, and remove the leading dot
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(e.Name()), "."))
}

func (e *Entry) GetChild(name string) *Entry {
	e.mx.RLock()
	defer e.mx.RUnlock()

	for _, child := range e.Child {
		if child.Name() == name {
			return child
		}
	}
	return nil
}

func (e *Entry) AddChild(child *Entry) {
	e.mx.Lock()
	defer e.mx.Unlock()

	if e.Child == nil {
		e.Child = make([]*Entry, 0, 10)
	}
	e.Child = append(e.Child, child)
}

func (e *Entry) HasChild() bool {
	return len(e.Child) != 0
}

func (e *Entry) SortChild() *Entry {
	slices.SortFunc(e.Child, func(a, b *Entry) int {
		return cmp.Compare(b.TotalStats.Total(), a.TotalStats.Total())
	})
	return e
}

func (e *Entry) GetStats(langFilter string) CodeStats {
	if langFilter == "" || langFilter == "All" {
		return e.TotalStats
	}
	return e.StatsByLang[langFilter]
}

func (e *Entry) Languages() []string {
	if e.StatsByLang == nil {
		return nil
	}
	langs := make([]string, 0, len(e.StatsByLang))
	for lang := range e.StatsByLang {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return langs
}

// AggregateStats recursively updates directory statistics from child nodes
func (e *Entry) AggregateStats() {
	if !e.IsDir {
		return
	}

	e.TotalStats = CodeStats{}
	e.StatsByLang = make(map[string]CodeStats)

	for _, child := range e.Child {
		if child.IsDir {
			child.AggregateStats()
		}

		e.TotalStats.Add(child.TotalStats)
		for lang, stats := range child.StatsByLang {
			currentLangStats := e.StatsByLang[lang]
			currentLangStats.Add(stats)
			e.StatsByLang[lang] = currentLangStats
		}
	}
}
