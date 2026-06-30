package render

import (
	"testing"

	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/structure"
)

func TestTreemapDrillDownRejectsParentPath(t *testing.T) {
	// Simulate selecting a block whose path is the parent of the current
	// navigation path. filepath.Rel would produce "..", which must not be used
	// for navigation.
	root := structure.NewDirEntry("/foo/bar")
	root.AddChild(structure.NewDirEntry("/foo/bar/sub"))
	root.AggregateStats()

	nav := NewCodeNavigation(structure.NewTree(root))
	dm := NewDirModel(nav, provider.Info{Name: "test"}, false, true)
	dm.treemapBlocks = []treemapBlock{
		{entry: &structure.Entry{Path: "/foo", IsDir: true}, level: 0, topIdx: 0},
	}
	dm.treemapSelected = 0
	vm := NewViewModel(nav, dm)

	vm.treemapDrillDown()

	if vm.nav.Entry() != root {
		t.Fatalf("expected navigation to stay at root, got %v", vm.nav.Entry())
	}
	if dm.treemapSelected != 0 {
		t.Fatalf("expected treemap selection to remain unchanged, got %d", dm.treemapSelected)
	}
}

func TestTreemapDrillDownNavigatesIntoChild(t *testing.T) {
	root := structure.NewDirEntry("/foo")
	sub := structure.NewDirEntry("/foo/sub")
	root.AddChild(sub)
	root.AggregateStats()

	nav := NewCodeNavigation(structure.NewTree(root))
	dm := NewDirModel(nav, provider.Info{Name: "test"}, false, true)
	dm.treemapBlocks = []treemapBlock{
		{entry: sub, level: 0, topIdx: 0},
	}
	dm.treemapSelected = 0
	vm := NewViewModel(nav, dm)

	vm.treemapDrillDown()

	if vm.nav.Entry() != sub {
		t.Fatalf("expected navigation to move into sub, got %v", vm.nav.Entry())
	}
	if dm.treemapSelected != 0 {
		t.Fatalf("expected treemap selection to reset to 0, got %d", dm.treemapSelected)
	}
}
