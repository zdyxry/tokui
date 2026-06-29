package search

import (
	"path/filepath"
	"testing"

	"github.com/zdyxry/tokui/structure"
)

func buildTestTree() *structure.Entry {
	root := structure.NewDirEntry("/project")

	cmd := structure.NewDirEntry(filepath.Join("/project", "cmd"))
	root.AddChild(cmd)
	cmd.AddChild(structure.NewFileEntry(filepath.Join("/project", "cmd", "app.go"), map[string]structure.CodeStats{
		"Go": {Code: 10, Comments: 2, Blanks: 1},
	}))
	cmd.AddChild(structure.NewFileEntry(filepath.Join("/project", "cmd", "error.go"), map[string]structure.CodeStats{
		"Go": {Code: 5, Comments: 1, Blanks: 1},
	}))

	render := structure.NewDirEntry(filepath.Join("/project", "render"))
	root.AddChild(render)
	render.AddChild(structure.NewFileEntry(filepath.Join("/project", "render", "dir_model.go"), map[string]structure.CodeStats{
		"Go": {Code: 20, Comments: 5, Blanks: 3},
	}))

	return root
}

func TestBuildIndex(t *testing.T) {
	root := buildTestTree()
	idx := BuildIndex(root)

	if idx.Items() == 0 {
		t.Fatal("expected non-empty index")
	}

	paths := make(map[string]bool)
	for _, item := range idx.items {
		paths[item.Path] = true
	}

	expected := []string{".", "cmd", "cmd/app.go", "cmd/error.go", "render", "render/dir_model.go"}
	for _, p := range expected {
		if !paths[p] {
			t.Errorf("expected path %q in index", p)
		}
	}
}

func TestFind_Exact(t *testing.T) {
	root := buildTestTree()
	idx := BuildIndex(root)

	matches := idx.Find("dir_model.go")
	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}

	if matches[0].Item.Path != "render/dir_model.go" {
		t.Errorf("expected first match to be render/dir_model.go, got %q", matches[0].Item.Path)
	}
}

func TestFind_Fuzzy(t *testing.T) {
	root := buildTestTree()
	idx := BuildIndex(root)

	matches := idx.Find("cmdapp")
	found := false
	for _, m := range matches {
		if m.Item.Path == "cmd/app.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected fuzzy query 'cmdapp' to match cmd/app.go, got %v", matches)
	}
}

func TestFind_Directory(t *testing.T) {
	root := buildTestTree()
	idx := BuildIndex(root)

	matches := idx.Find("render")
	found := false
	for _, m := range matches {
		if m.Item.Path == "render" && m.Item.Entry.IsDir {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected query 'render' to match render directory, got %v", matches)
	}
}

func TestFind_EmptyQuery(t *testing.T) {
	root := buildTestTree()
	idx := BuildIndex(root)

	matches := idx.Find("")
	if len(matches) != 0 {
		t.Errorf("expected no matches for empty query, got %d", len(matches))
	}
}

func TestFind_HighlightIndexes(t *testing.T) {
	root := buildTestTree()
	idx := BuildIndex(root)

	matches := idx.Find("app")
	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}

	if len(matches[0].MatchedIndexes) == 0 {
		t.Error("expected non-empty matched indexes")
	}
}

func TestBuildIndex_Nil(t *testing.T) {
	idx := BuildIndex(nil)
	if idx == nil {
		t.Fatal("expected non-nil index")
	}
	if idx.Items() != 0 {
		t.Errorf("expected empty index, got %d items", idx.Items())
	}
}

func TestBuildIndex_DeeplyNested(t *testing.T) {
	root := structure.NewDirEntry("/project")
	a := structure.NewDirEntry(filepath.Join("/project", "a"))
	b := structure.NewDirEntry(filepath.Join("/project", "a", "b"))
	c := structure.NewDirEntry(filepath.Join("/project", "a", "b", "c"))
	root.AddChild(a)
	a.AddChild(b)
	b.AddChild(c)
	c.AddChild(structure.NewFileEntry(filepath.Join("/project", "a", "b", "c", "deep.go"), map[string]structure.CodeStats{
		"Go": {Code: 1},
	}))

	idx := BuildIndex(root)
	expected := []string{".", "a", "a/b", "a/b/c", "a/b/c/deep.go"}
	paths := make(map[string]bool)
	for _, item := range idx.items {
		paths[item.Path] = true
	}
	for _, p := range expected {
		if !paths[p] {
			t.Errorf("expected path %q in index", p)
		}
	}
}

func TestFind_NilIndex(t *testing.T) {
	var idx *Index
	if got := idx.Find("foo"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestFind_EmptyIndex(t *testing.T) {
	idx := &Index{}
	if got := idx.Find("foo"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestFind_WhitespaceOnly(t *testing.T) {
	root := buildTestTree()
	idx := BuildIndex(root)
	if got := idx.Find("   "); got != nil {
		t.Errorf("expected nil for whitespace-only query, got %v", got)
	}
}

func TestItems_NilIndex(t *testing.T) {
	var idx *Index
	if got := idx.Items(); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestFind_FuzzyMultipleResults(t *testing.T) {
	root := buildTestTree()
	idx := BuildIndex(root)

	matches := idx.Find("go")
	if len(matches) < 2 {
		t.Fatalf("expected multiple matches, got %d", len(matches))
	}

	for _, m := range matches {
		if len(m.MatchedIndexes) == 0 {
			t.Errorf("expected matched indexes for %q", m.Item.Path)
		}
	}
}
