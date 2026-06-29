// Package structure builds the in-memory file tree from a provider.Result and
// aggregates per-directory statistics.
package structure

import (
	"path/filepath"
	"strings"

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

func (t *Tree) addFileToTree(root *Entry, relativePath string, stats map[string]CodeStats) {
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
		}
	}
}

// normalizePath converts a raw file path (absolute or relative) to a path
// relative to the analysis root. It handles slash normalization and removes
// leading "./" or "/" prefixes that tools like tokei may produce.
func normalizePath(root, raw string) string {
	root = filepath.ToSlash(root)
	raw = filepath.ToSlash(raw)
	raw = strings.TrimPrefix(raw, "./")
	rel := strings.TrimPrefix(raw, root)
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.TrimPrefix(rel, "./")
	return rel
}
