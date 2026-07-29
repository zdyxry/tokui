// Package structure builds the in-memory file tree from a provider.Result and
// aggregates per-directory statistics.
package structure

import (
	"path/filepath"
	"strings"

	"github.com/zdyxry/tokui/gitx"
	"github.com/zdyxry/tokui/provider"
)

// Tree represents the code statistics file tree.
type Tree struct {
	root *Entry
}

// NewTree creates a new Tree with the given root entry.
func NewTree(root *Entry) *Tree {
	return &Tree{root: root}
}

// Root returns the current root entry.
func (t *Tree) Root() *Entry {
	return t.root
}

// SetRoot replaces the root entry.
func (t *Tree) SetRoot(root *Entry) {
	t.root = root
}

// BuildFromProvider analyzes the given path using the supplied Provider and
// builds the file tree from the returned per-file statistics.
func (t *Tree) BuildFromProvider(p provider.Provider, path string) error {
	result, err := p.Analyze(path)
	if err != nil {
		return err
	}

	// Use the user-provided original path for the root node so the UI status
	// bar displays what the user typed (e.g. "." or an absolute path).
	t.root = NewDirEntry(path)

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if err := t.buildFromResult(result, absPath); err != nil {
		return err
	}
	t.root.AggregateStats()
	return nil
}

// BuildFromProviderResult builds the file tree from an already-parsed
// provider.Result. The root path is used for path normalization.
func (t *Tree) BuildFromProviderResult(result provider.Result, root string) error {
	absPath, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	t.root = NewDirEntry(root)

	if err := t.buildFromResult(result, absPath); err != nil {
		return err
	}
	t.root.AggregateStats()
	return nil
}

// BuildFromDiff builds the file tree for Diff mode. The full S2-side provider
// result forms the tree (so the "show all" toggle has the complete snapshot),
// and the change set is joined onto the matching file entries. Every changed
// file gets Change.Present set, including zero-churn changes (binary files,
// pure renames). Changed files missing from the S2 result (deleted files, or
// files the provider skipped via its own ignore rules) are added with zeroed
// CodeStats and a language guessed from the file extension; their Kind and
// churn are preserved. Unchanged files keep a zero Change, which the view
// layer uses to filter or de-emphasize them. Change paths and repoRoot are
// interpreted relative to the repository root.
func (t *Tree) BuildFromDiff(changes []gitx.FileChange, s2 provider.Result, repoRoot string) error {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return err
	}

	t.root = NewDirEntry(repoRoot)

	if err := t.buildFromResult(s2, absRoot); err != nil {
		return err
	}

	for _, c := range changes {
		// Change paths are relative to the repo root; anchor them at absRoot
		// before normalizing so the lookup key matches the S2 paths.
		rel := normalizePath(absRoot, filepath.Join(absRoot, filepath.FromSlash(c.Path)))
		if rel == "" {
			continue
		}

		change := Change{
			Added:   c.Added,
			Deleted: c.Deleted,
			Kind:    c.Kind,
			Present: true,
			OldPath: c.OldPath,
		}
		if entry := t.root.findFile(rel); entry != nil {
			entry.Change = change
			continue
		}

		// Missing from the S2 provider result. For Kind == Deleted this is
		// expected; any other Kind means the provider skipped the file (e.g.
		// its own ignore rules) — keep the original Kind and churn either
		// way, with zeroed stats and a language guessed from the extension.
		stats := map[string]CodeStats{languageByExt(rel): {}}
		if entry := t.addFileToTree(t.root, rel, stats); entry != nil {
			entry.Change = change
		}
	}

	t.root.AggregateStats()
	return nil
}

// BuildFromCompare builds the file tree for Compare mode: the union of both
// snapshots forms the tree, each file carries its S2 CodeStats plus the S1
// side values in Change.PrevCode/PrevComplexity. Files only present in S1
// (deleted between the refs) are added with zeroed S2 stats and Kind
// Deleted; files only present in S2 keep zero Prev* values. Files present in
// S1 get Change.Present set; OldPath stays empty because a rename appears as
// delete+add under the path-based join.
func (t *Tree) BuildFromCompare(s1, s2 provider.Result, root string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	t.root = NewDirEntry(root)

	if err := t.buildFromResult(s2, absRoot); err != nil {
		return err
	}

	for _, f := range s1.Files {
		rel := normalizePath(absRoot, f.Path)
		if rel == "" {
			continue
		}
		if entry := t.root.findFile(rel); entry != nil {
			entry.Change.PrevCode = f.Code
			entry.Change.PrevComplexity = f.Complexity
			entry.Change.Present = true
			continue
		}
		lang := f.Language
		if lang == "" {
			lang = languageByExt(rel)
		}
		stats := map[string]CodeStats{lang: {}}
		if entry := t.addFileToTree(t.root, rel, stats); entry != nil {
			entry.Change = Change{
				Kind:           gitx.Deleted,
				Present:        true,
				PrevCode:       f.Code,
				PrevComplexity: f.Complexity,
			}
		}
	}

	t.root.AggregateStats()
	return nil
}

// findFile returns the file entry at the given slash-separated path relative
// to e, or nil when the path does not resolve to a file entry.
func (e *Entry) findFile(relativePath string) *Entry {
	current := e
	parts := strings.Split(relativePath, "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		current = current.GetChild(part)
		if current == nil {
			return nil
		}
		if i == len(parts)-1 && !current.IsDir {
			return current
		}
	}
	return nil
}

// buildFromResult groups per-file stats by relative path and inserts them into
// the tree using the provided analysis root for path normalization.
func (t *Tree) buildFromResult(result provider.Result, absPath string) error {
	fileStats := make(map[string]map[string]CodeStats)

	for _, f := range result.Files {
		relativePath := normalizePath(absPath, f.Path)

		if _, ok := fileStats[relativePath]; !ok {
			fileStats[relativePath] = make(map[string]CodeStats)
		}
		fileStats[relativePath][f.Language] = CodeStats{
			Code:          f.Code,
			Comments:      f.Comments,
			Blanks:        f.Blanks,
			Complexity:    f.Complexity,
			MaxComplexity: f.Complexity,
		}
	}

	for filePath, stats := range fileStats {
		t.addFileToTree(t.root, filePath, stats)
	}
	return nil
}

func (t *Tree) addFileToTree(root *Entry, relativePath string, stats map[string]CodeStats) *Entry {
	parts := strings.Split(relativePath, "/")
	currentNode := root

	// Iterate through the directory parts of the path (excluding the final filename)
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if part == "" { // Ignore empty strings produced by splitting "//" etc.
			continue
		}

		childNode := currentNode.GetChild(part)
		if childNode == nil {
			// If the directory doesn't exist, create it. Use filepath.Join to ensure path separators match the current system.
			childPath := filepath.Join(currentNode.Path, part)
			childNode = NewDirEntry(childPath)
			currentNode.AddChild(childNode)
		}
		currentNode = childNode
	}

	// Add the file node
	if len(parts) > 0 {
		fileName := parts[len(parts)-1]
		if fileName != "" {
			filePath := filepath.Join(currentNode.Path, fileName)
			fileEntry := NewFileEntry(filePath, stats)
			currentNode.AddChild(fileEntry)
			return fileEntry
		}
	}
	return nil
}

// normalizePath converts a raw file path (absolute or relative) to a path
// relative to the analysis root. It handles slash normalization and removes
// leading "./" or "/" prefixes that tools like tokei may produce.
func normalizePath(root, raw string) string {
	root = filepath.ToSlash(filepath.Clean(root))
	raw = filepath.ToSlash(filepath.Clean(raw))

	if raw == root {
		return ""
	}

	prefix := root
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// If the raw path is already under the root (including Unix-style absolute
	// paths on Windows), trim the root directly.
	if strings.HasPrefix(raw, prefix) {
		rel := strings.TrimPrefix(raw, prefix)
		rel = strings.TrimPrefix(rel, "./")
		return rel
	}

	// Otherwise, if the path is relative, resolve it against the current
	// working directory and try again. This handles walkers that return paths
	// relative to the working directory while the analysis root is absolute.
	if !filepath.IsAbs(raw) {
		if abs, err := filepath.Abs(raw); err == nil {
			absRaw := filepath.ToSlash(filepath.Clean(abs))
			if absRaw == root {
				return ""
			}
			if strings.HasPrefix(absRaw, prefix) {
				rel := strings.TrimPrefix(absRaw, prefix)
				rel = strings.TrimPrefix(rel, "./")
				return rel
			}
		}
	}

	// No root prefix matched; still clean up any leading slash that may have
	// been produced by tools emitting double separators.
	rel := strings.TrimPrefix(raw, "/")
	rel = strings.TrimPrefix(rel, "./")
	return rel
}
