// structure/tree.go
package structure

import (
	"path/filepath"
	"strings"

	"github.com/zdyxry/tokui/tokei"
)

type Tree struct {
	root *Entry
}

func NewTree(root *Entry) *Tree {
	return &Tree{root: root}
}

func (t *Tree) Root() *Entry {
	return t.root
}

func (t *Tree) SetRoot(root *Entry) {
	t.root = root
}

// BuildFromTokei builds the file tree from tokei's output for the given path
func (t *Tree) BuildFromTokei(path string) error {
	report, err := tokei.Analyze(path)
	if err != nil {
		return err
	}

	// 1. Get the absolute path of the analysis path for reliable prefix trimming.
	absPath, err := filepath.Abs(path)
	if err != nil {
		// If the path cannot be resolved, this is usually a serious issue and should stop here.
		return err
	}

	// 2. The root node uses the user-provided original path (which might be "." or an absolute path).
	//    This is to correctly display the user input in the UI status bar.
	t.root = NewDirEntry(path)

	fileStats := make(map[string]map[string]CodeStats)

	// 3. Collect statistics for all files and perform path normalization.
	for lang, stats := range report {
		if lang == "Total" { // "Total" is tokei's aggregate statistics, we calculate ourselves, so skip
			continue
		}
		for _, fileReport := range stats.Reports {
			// 4. *** Core logic: Path normalization ***
			//    Goal: Uniformly convert paths returned by tokei (which may be absolute or relative)
			//    to relative paths relative to the analysis root directory.

			// Convert both the tokei-returned file path and our absolute analysis path to use '/' separators for reliable comparison and trimming.
			tokeiFilePath := filepath.ToSlash(fileReport.Name)
			absAnalysisPath := filepath.ToSlash(absPath)

			// Trim the absolute path prefix to get the relative path.
			// For example, convert "/path/to/project/src/main.go" to "src/main.go".
			// `TrimPrefix` is case-sensitive, which is the correct behavior on most file systems.
			relativePath := strings.TrimPrefix(tokeiFilePath, absAnalysisPath)

			// Remove possible leading slash to ensure the path is purely relative.
			relativePath = strings.TrimPrefix(relativePath, "/")

			// Ignore the "./" prefix that tokei might produce when analyzing ".".
			relativePath = strings.TrimPrefix(relativePath, "./")

			// Now `relativePath` is always in the form like "src/main.go" or "README.md".

			if _, ok := fileStats[relativePath]; !ok {
				fileStats[relativePath] = make(map[string]CodeStats)
			}
			fileStats[relativePath][lang] = CodeStats{
				Code:     fileReport.Stats.Code,
				Comments: fileReport.Stats.Comments,
				Blanks:   fileReport.Stats.Blanks,
			}
		}
	}

	// 5. Iterate through all files and add them to the correct position in the tree.
	//    Here `filePath` is the `relativePath` we processed above.
	for filePath, stats := range fileStats {
		t.addFileToTree(t.root, filePath, stats)
	}

	// 6. Aggregate statistics for all directories.
	t.root.AggregateStats()

	return nil
}

func (t *Tree) BuildFromStdin() error {
	report, err := tokei.AnalyzeFromStdin()
	if err != nil {
		return err
	}

	// When reading from stdin, we don't have an analysis path, so use the current directory as the root node
	t.root = NewDirEntry(".")

	fileStats := make(map[string]map[string]CodeStats)

	// Collect statistics for all files
	for lang, stats := range report {
		if lang == "Total" { // "Total" is tokei's aggregate statistics, we calculate ourselves, so skip
			continue
		}
		for _, fileReport := range stats.Reports {
			// When reading from stdin, file paths should already be relative or absolute paths
			// We need to normalize them to relative paths
			filePath := fileReport.Name

			// Remove possible leading "./"
			filePath = strings.TrimPrefix(filePath, "./")

			// If it's an absolute path, try to extract the relative part
			if filepath.IsAbs(filePath) {
				// Try to get the current working directory
				if wd, err := filepath.Abs("."); err == nil {
					if rel, err := filepath.Rel(wd, filePath); err == nil {
						filePath = rel
					}
				}
			}

			if _, ok := fileStats[filePath]; !ok {
				fileStats[filePath] = make(map[string]CodeStats)
			}
			fileStats[filePath][lang] = CodeStats{
				Code:     fileReport.Stats.Code,
				Comments: fileReport.Stats.Comments,
				Blanks:   fileReport.Stats.Blanks,
			}
		}
	}

	// Iterate through all files and add them to the correct position in the tree
	for filePath, stats := range fileStats {
		t.addFileToTree(t.root, filePath, stats)
	}

	// Aggregate statistics for all directories
	t.root.AggregateStats()

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
