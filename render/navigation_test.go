package render

import (
	"testing"

	"github.com/zdyxry/tokui/structure"
)

func newTestTree() *structure.Tree {
	root := structure.NewDirEntry("root")
	root.AddChild(structure.NewFileEntry("root/a.go", map[string]structure.CodeStats{
		"Go": {Code: 10, Comments: 2, Blanks: 3},
	}))

	subdir := structure.NewDirEntry("root/subdir")
	root.AddChild(subdir)
	subdir.AddChild(structure.NewFileEntry("root/subdir/file.go", map[string]structure.CodeStats{
		"Go": {Code: 5, Comments: 1, Blanks: 1},
	}))

	nested := structure.NewDirEntry("root/subdir/nested")
	subdir.AddChild(nested)

	root.AggregateStats()
	return structure.NewTree(root)
}

func TestNewCodeNavigation(t *testing.T) {
	tree := newTestTree()
	nav := NewCodeNavigation(tree)

	if nav.Entry() != tree.Root() {
		t.Errorf("expected navigation entry to be the tree root")
	}
	if nav.entryStack.len() != 0 {
		t.Errorf("expected empty entry stack, got length %d", nav.entryStack.len())
	}
}

func TestNavigationEntry(t *testing.T) {
	tree := newTestTree()
	nav := NewCodeNavigation(tree)

	if got := nav.Entry(); got != tree.Root() {
		t.Errorf("Entry() returned unexpected entry")
	}
}

func TestNavigationParentTotalLines(t *testing.T) {
	t.Run("returns at least 1 for empty stats", func(t *testing.T) {
		tree := structure.NewTree(structure.NewDirEntry("root"))
		nav := NewCodeNavigation(tree)

		if got := nav.ParentTotalLines(""); got < 1 {
			t.Errorf("ParentTotalLines() = %d, want at least 1", got)
		}
	})

	t.Run("returns actual total when entry has stats", func(t *testing.T) {
		tree := newTestTree()
		nav := NewCodeNavigation(tree)

		want := int64(22) // a.go (15) + subdir/file.go (7)
		if got := nav.ParentTotalLines(""); got != want {
			t.Errorf("ParentTotalLines() = %d, want %d", got, want)
		}
	})
}

func TestNavigationUp(t *testing.T) {
	t.Run("no-op at root", func(t *testing.T) {
		tree := newTestTree()
		nav := NewCodeNavigation(tree)

		nav.Up()
		if nav.Entry() != tree.Root() {
			t.Errorf("Up() at root changed the current entry")
		}
	})

	t.Run("restores previous entry and cursor", func(t *testing.T) {
		tree := newTestTree()
		nav := NewCodeNavigation(tree)

		nav.Down("subdir", 7, 4)
		if nav.Entry().Path != "root/subdir" {
			t.Fatalf("expected to navigate into subdir, got %q", nav.Entry().Path)
		}

		nav.Up()
		if nav.Entry().Path != "root" {
			t.Errorf("expected to return to root, got %q", nav.Entry().Path)
		}
		if nav.cursor != 7 {
			t.Errorf("expected cursor to be restored to 7, got %d", nav.cursor)
		}
	})
}

func TestNavigationDown(t *testing.T) {
	tree := newTestTree()
	nav := NewCodeNavigation(tree)

	t.Run("no-op for empty name", func(t *testing.T) {
		before := nav.Entry()
		nav.Down("", 0, 0)
		if nav.Entry() != before {
			t.Errorf("Down(\"\") changed the current entry")
		}
	})

	t.Run("no-op for non-existent name", func(t *testing.T) {
		before := nav.Entry()
		nav.Down("missing", 0, 0)
		if nav.Entry() != before {
			t.Errorf("Down(\"missing\") changed the current entry")
		}
	})

	t.Run("no-op for file", func(t *testing.T) {
		before := nav.Entry()
		nav.Down("a.go", 0, 0)
		if nav.Entry() != before {
			t.Errorf("Down(\"a.go\") changed the current entry")
		}
	})

	t.Run("pushes current and sets new entry and cursor", func(t *testing.T) {
		nav.Down("subdir", 3, 5)
		if nav.Entry().Path != "root/subdir" {
			t.Errorf("expected entry root/subdir, got %q", nav.Entry().Path)
		}
		if nav.cursor != 5 {
			t.Errorf("expected cursor 5, got %d", nav.cursor)
		}
		if nav.entryStack.len() != 1 {
			t.Errorf("expected stack length 1, got %d", nav.entryStack.len())
		}
	})
}

func TestNavigationAbsPathFromSelectedRow(t *testing.T) {
	tree := newTestTree()
	nav := NewCodeNavigation(tree)

	t.Run("returns column 1 when present", func(t *testing.T) {
		got := nav.AbsPathFromSelectedRow([]string{"icon", "root/subdir/file.go", "file.go"})
		if got != "root/subdir/file.go" {
			t.Errorf("AbsPathFromSelectedRow() = %q, want %q", got, "root/subdir/file.go")
		}
	})

	t.Run("falls back to joining current path with name", func(t *testing.T) {
		got := nav.AbsPathFromSelectedRow([]string{"icon", "file.go"})
		want := "root/file.go"
		if got != want {
			t.Errorf("AbsPathFromSelectedRow() = %q, want %q", got, want)
		}
	})
}

func TestNavigationNavigateToPath(t *testing.T) {
	tree := newTestTree()

	t.Run("dot returns root", func(t *testing.T) {
		nav := NewCodeNavigation(tree)
		got := nav.NavigateToPath(".")
		if got != tree.Root() {
			t.Errorf("NavigateToPath(\".\") did not return root")
		}
	})

	t.Run("navigates to parent dir and returns file entry", func(t *testing.T) {
		nav := NewCodeNavigation(tree)
		got := nav.NavigateToPath("subdir/file.go")

		if nav.Entry().Path != "root/subdir" {
			t.Errorf("expected current entry root/subdir, got %q", nav.Entry().Path)
		}
		if got == nil || got.Path != "root/subdir/file.go" {
			t.Errorf("expected file entry root/subdir/file.go, got %v", got)
		}
	})

	t.Run("returns nil if intermediate path missing", func(t *testing.T) {
		nav := NewCodeNavigation(tree)
		got := nav.NavigateToPath("a/b/c")
		if got != nil {
			t.Errorf("expected nil for missing intermediate path, got %v", got)
		}
	})
}
