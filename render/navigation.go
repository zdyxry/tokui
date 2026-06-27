package render

import (
	"path/filepath"
	"strings"

	"github.com/zdyxry/tokui/structure"
)

// stackItem stores navigation state for a directory level
type stackItem struct {
	entry  *structure.Entry
	cursor int
}

type entryStack []*stackItem

func (e *entryStack) push(si *stackItem) {
	*e = append(*e, si)
}

func (e *entryStack) pop() *stackItem {
	if len(*e) == 0 {
		return nil
	}
	item := (*e)[len(*e)-1]
	*e = (*e)[:len(*e)-1]
	return item
}

func (e *entryStack) len() int {
	return len(*e)
}

// Navigation handles traversal through the code statistics tree
type Navigation struct {
	tree       *structure.Tree
	entry      *structure.Entry
	entryStack *entryStack
	cursor     int
}

func NewCodeNavigation(t *structure.Tree) *Navigation {
	return &Navigation{
		tree:       t,
		entry:      t.Root(),
		entryStack: &entryStack{},
	}
}

// Entry returns the current directory
func (n *Navigation) Entry() *structure.Entry {
	return n.entry
}

// ParentTotalLines returns the total lines of the parent directory for calculating child entry percentages.
// The langFilter parameter allows getting total lines based on the currently selected language.
func (n *Navigation) ParentTotalLines(langFilter string) int64 {
	minSize := int64(1) // Avoid division by zero
	if n.entry == nil {
		return minSize
	}
	return max(minSize, n.entry.GetStats(langFilter).Total())
}

func (n *Navigation) Up() {
	if n.entryStack.len() == 0 {
		return
	}
	lastItem := n.entryStack.pop()
	n.entry, n.cursor = lastItem.entry, lastItem.cursor
}

func (n *Navigation) Down(name string, parentCursor, childCursor int) {
	if len(name) == 0 {
		return
	}

	entry := n.entry.GetChild(name)
	if entry == nil || !entry.IsDir {
		return
	}

	n.entryStack.push(&stackItem{entry: n.entry, cursor: parentCursor})
	n.entry, n.cursor = entry, childCursor
}

// AbsPathFromSelectedRow returns absolute path from selected row, using column 1's hidden path
func (n *Navigation) AbsPathFromSelectedRow(selectedRow []string) string {
	if len(selectedRow) > 2 {
		return selectedRow[1]
	}

	// If unable to get it, fall back to building the path based on current entry and name
	if len(selectedRow) > 1 {
		return filepath.Join(n.Entry().Path, selectedRow[len(selectedRow)-1])
	}

	return ""
}

// NavigateToPath navigates to the parent directory of the given relative path
// and returns the target entry (file or directory) located at that path.
// The returned entry can be used by the caller to position the cursor.
// If relPath is empty or ".", the root entry is returned.
func (n *Navigation) NavigateToPath(relPath string) *structure.Entry {
	n.entry = n.tree.Root()
	n.entryStack = &entryStack{}
	n.cursor = 0

	relPath = strings.TrimSpace(relPath)
	relPath = strings.TrimPrefix(relPath, "./")
	if relPath == "" || relPath == "." {
		return n.entry
	}

	localPath := filepath.FromSlash(relPath)
	parts := strings.Split(localPath, string(filepath.Separator))

	for i, part := range parts {
		if part == "" {
			continue
		}

		if i == len(parts)-1 {
			// Last component can be a file or a directory.
			return n.entry.GetChild(part)
		}

		child := n.entry.GetChild(part)
		if child == nil || !child.IsDir {
			return nil
		}
		n.entryStack.push(&stackItem{entry: n.entry, cursor: 0})
		n.entry = child
		n.cursor = 0
	}

	return n.entry
}
