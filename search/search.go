// Package search provides an in-memory fuzzy search index over a structure.Entry tree.
package search

import (
	"path/filepath"
	"strings"

	"github.com/sahilm/fuzzy"
	"github.com/zdyxry/tokui/structure"
)

// Item represents a single searchable file or directory.
type Item struct {
	Entry *structure.Entry
	Path  string // path relative to the tree root, using '/' separators
}

// Match represents a single fuzzy match result.
type Match struct {
	Item           Item
	Score          int
	MatchedIndexes []int
}

// Index holds a flat list of all searchable entries in a project tree.
type Index struct {
	items []Item
}

// itemSource adapts []Item to fuzzy.Source.
type itemSource struct {
	items []Item
}

func (s itemSource) String(i int) string {
	if i < 0 || i >= len(s.items) {
		return ""
	}
	return s.items[i].Path
}

func (s itemSource) Len() int {
	return len(s.items)
}

// BuildIndex recursively walks the entry tree and creates a searchable index.
// The returned paths are relative to root.Path and always use '/' separators.
func BuildIndex(root *structure.Entry) *Index {
	if root == nil {
		return &Index{}
	}

	idx := &Index{}
	idx.items = make([]Item, 0, 1024)

	rootPath := filepath.Clean(filepath.FromSlash(root.Path))

	var walk func(entry *structure.Entry)
	walk = func(entry *structure.Entry) {
		if entry == nil {
			return
		}

		relPath := "."
		if entry != root {
			entryPath := filepath.Clean(filepath.FromSlash(entry.Path))
			r, err := filepath.Rel(rootPath, entryPath)
			if err != nil {
				r = entryPath
			}
			relPath = filepath.ToSlash(filepath.Clean(r))
		}

		idx.items = append(idx.items, Item{
			Entry: entry,
			Path:  relPath,
		})

		if entry.IsDir {
			for _, child := range entry.Child {
				if child == nil {
					continue
				}
				walk(child)
			}
		}
	}

	walk(root)
	return idx
}

// Find performs a fuzzy search against the indexed paths.
// Results are sorted from best to worst match.
func (idx *Index) Find(query string) []Match {
	if idx == nil || len(idx.items) == 0 {
		return nil
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	src := itemSource{items: idx.items}
	results := fuzzy.FindFrom(query, src)

	matches := make([]Match, 0, len(results))
	for _, r := range results {
		matches = append(matches, Match{
			Item:           idx.items[r.Index],
			Score:          r.Score,
			MatchedIndexes: r.MatchedIndexes,
		})
	}
	return matches
}

// Items returns the total number of indexed items.
func (idx *Index) Items() int {
	if idx == nil {
		return 0
	}
	return len(idx.items)
}
