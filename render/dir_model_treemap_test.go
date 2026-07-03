package render

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/structure"
)

func TestTreemapKeyboardStaysAtTopLevel(t *testing.T) {
	dm := &DirModel{
		treemapMode: true,
		treemapBlocks: []treemapBlock{
			{entry: &structure.Entry{Path: "a"}, level: 0, topIdx: 0},
			{entry: &structure.Entry{Path: "a1"}, level: 1, topIdx: 0},
			{entry: &structure.Entry{Path: "a2"}, level: 1, topIdx: 0},
			{entry: &structure.Entry{Path: "b"}, level: 0, topIdx: 1},
			{entry: &structure.Entry{Path: "b1"}, level: 1, topIdx: 1},
			{entry: &structure.Entry{Path: "c"}, level: 0, topIdx: 2},
		},
		treemapSelected: 0,
	}

	dm.moveTreemapSelection(1)
	if dm.treemapSelected != 3 {
		t.Fatalf("expected selection to move to top-level b (index 3), got %d", dm.treemapSelected)
	}

	dm.moveTreemapSelection(1)
	if dm.treemapSelected != 5 {
		t.Fatalf("expected selection to move to top-level c (index 5), got %d", dm.treemapSelected)
	}

	dm.moveTreemapSelection(-1)
	if dm.treemapSelected != 3 {
		t.Fatalf("expected selection to move back to top-level b (index 3), got %d", dm.treemapSelected)
	}

	// Starting from a nested block, j/k should still move between top-level blocks.
	dm.treemapSelected = 1 // a1
	dm.moveTreemapSelection(1)
	if dm.treemapSelected != 3 {
		t.Fatalf("expected selection from nested block to jump to top-level b (index 3), got %d", dm.treemapSelected)
	}
}

func TestTreemapColorModeToggle(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.treemapMode = true
	dm.width = 100
	dm.height = 30
	dm.updateTableData()

	if dm.treemapColorByLang {
		t.Fatal("expected default directory color mode")
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if !dm.treemapColorByLang {
		t.Fatal("expected language color mode after pressing c")
	}
	if !strings.Contains(dm.View(), "Languages") {
		t.Fatal("expected legend to auto-show in language color mode")
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if dm.treemapColorByLang {
		t.Fatal("expected directory color mode after second c")
	}
	if strings.Contains(dm.View(), "Languages") {
		t.Fatal("expected legend to hide in directory color mode")
	}
}

func TestTreemapLegendAutoShow(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.treemapMode = true
	dm.treemapColorByLang = true
	dm.width = 100
	dm.height = 30
	dm.updateTableData()

	view := dm.View()
	if !strings.Contains(view, "Languages") {
		t.Fatalf("expected legend to auto-show in language color mode, got:\n%s", view)
	}
}

func TestTreemapViewHeight(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.treemapMode = true
	dm.treemapColorByLang = true
	dm.width = 100
	dm.height = 30
	dm.updateTableData()

	view := dm.View()
	got := lipgloss.Height(view)
	if got != dm.height {
		t.Fatalf("expected view height %d, got %d", dm.height, got)
	}
}

func TestCycleTreemapSize_WithSCC(t *testing.T) {
	dm := NewDirModel(
		NewCodeNavigation(structure.NewTree(structure.NewDirEntry("root"))),
		sccProviderInfo(),
		false,
		true,
	)

	order := []SortKey{SortByTotal, SortByComplexity}
	for i := 0; i < len(order)*2; i++ {
		expected := order[i%len(order)]
		if dm.treemapSizeKey != expected {
			t.Errorf("cycle %d: expected size key %q, got %q", i, expected, dm.treemapSizeKey)
		}
		dm.cycleTreemapSize()
	}
}

func TestCycleTreemapSize_WithTokei(t *testing.T) {
	dm := NewDirModel(
		NewCodeNavigation(structure.NewTree(structure.NewDirEntry("root"))),
		provider.Info{Name: "tokei", Capabilities: provider.CapLines},
		false,
		true,
	)

	for i := 0; i < 3; i++ {
		dm.cycleTreemapSize()
		if dm.treemapSizeKey != SortByTotal {
			t.Errorf("tokei provider should only support Total size, got %q", dm.treemapSizeKey)
		}
	}
}

func TestTreemapSizeFunc(t *testing.T) {
	root := structure.NewDirEntry("root")
	root.AddChild(structure.NewFileEntry("root/a.go", map[string]structure.CodeStats{
		"Go": {Code: 10, Comments: 2, Blanks: 3, Complexity: 5},
	}))
	root.AggregateStats()

	dm := NewDirModel(NewCodeNavigation(structure.NewTree(root)), sccProviderInfo(), false, true)

	dm.treemapSizeKey = SortByTotal
	if got := dm.treemapSizeFunc()(root.Child[0]); got != 15 {
		t.Errorf("Total size: expected 15, got %d", got)
	}

	dm.treemapSizeKey = SortByComplexity
	if got := dm.treemapSizeFunc()(root.Child[0]); got != 5 {
		t.Errorf("Complexity size: expected 5, got %d", got)
	}
}
