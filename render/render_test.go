package render

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestTreemapRightClickGoesUp(t *testing.T) {
	root := structure.NewDirEntry("/foo")
	sub := structure.NewDirEntry("/foo/sub")
	root.AddChild(sub)
	root.AggregateStats()

	nav := NewCodeNavigation(structure.NewTree(root))
	dm := NewDirModel(nav, provider.Info{Name: "test"}, false, true)
	vm := NewViewModel(nav, dm)

	// Drill into the child first so there is a parent directory to return to.
	nav.Down("sub", 0, 0)
	if vm.nav.Entry() != sub {
		t.Fatalf("setup: expected navigation at sub, got %v", vm.nav.Entry())
	}

	// A right-click in treemap mode should navigate back up to the parent.
	vm.Update(tea.MouseMsg{Button: tea.MouseButtonRight, Action: tea.MouseActionPress})

	if vm.nav.Entry() != root {
		t.Fatalf("expected navigation to return to root after right-click, got %v", vm.nav.Entry())
	}
}


func TestTreemapRightClickIgnoredDuringSearch(t *testing.T) {
	root := structure.NewDirEntry("/foo")
	sub := structure.NewDirEntry("/foo/sub")
	root.AddChild(sub)
	root.AggregateStats()

	nav := NewCodeNavigation(structure.NewTree(root))
	dm := NewDirModel(nav, provider.Info{Name: "test"}, false, true)
	vm := NewViewModel(nav, dm)

	// Drill into the child first so there is a parent directory to return to.
	nav.Down("sub", 0, 0)
	if vm.nav.Entry() != sub {
		t.Fatalf("setup: expected navigation at sub, got %v", vm.nav.Entry())
	}

	// Open the global search overlay.
	dm.openGlobalSearch()
	if dm.mode != SEARCH {
		t.Fatalf("setup: expected SEARCH mode, got %v", dm.mode)
	}

	// A right-click while the search overlay is open should not navigate.
	vm.Update(tea.MouseMsg{Button: tea.MouseButtonRight, Action: tea.MouseActionPress})

	if vm.nav.Entry() != sub {
		t.Fatalf("expected navigation to stay at sub while search is open, got %v", vm.nav.Entry())
	}
}
