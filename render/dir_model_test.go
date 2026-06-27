package render

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zdyxry/tokui/filter"
	"github.com/zdyxry/tokui/structure"
)

func TestColumnFmtName(t *testing.T) {
	col := Column{Title: "Total", SortKey: SortByTotal}

	if got := col.FmtName(SortState{Key: SortByCode, Desc: false}); got != "Total" {
		t.Errorf("expected 'Total', got %q", got)
	}
	if got := col.FmtName(SortState{Key: SortByTotal, Desc: false}); got != "Total ▲" {
		t.Errorf("expected 'Total ▲', got %q", got)
	}
	if got := col.FmtName(SortState{Key: SortByTotal, Desc: true}); got != "Total ▼" {
		t.Errorf("expected 'Total ▼', got %q", got)
	}
}

func TestSortStateDirectionArrow(t *testing.T) {
	if got := (SortState{Desc: false}).DirectionArrow(); got != "▲" {
		t.Errorf("expected '▲', got %q", got)
	}
	if got := (SortState{Desc: true}).DirectionArrow(); got != "▼" {
		t.Errorf("expected '▼', got %q", got)
	}
}

func TestDefaultDescForSortKey(t *testing.T) {
	for _, key := range []SortKey{SortByCode, SortByComments, SortByBlanks, SortByTotal, SortByPercent} {
		if !defaultDescForSortKey(key) {
			t.Errorf("expected numeric key %q to default to descending", key)
		}
	}
	for _, key := range []SortKey{SortByName, SortByLanguages} {
		if defaultDescForSortKey(key) {
			t.Errorf("expected text key %q to default to ascending", key)
		}
	}
}

func newTestDirModel() *DirModel {
	root := structure.NewDirEntry("root")

	// a.go: Go, Code=20, Comments=5, Blanks=5, Total=30
	root.AddChild(structure.NewFileEntry("root/a.go", map[string]structure.CodeStats{
		"Go": {Code: 20, Comments: 5, Blanks: 5},
	}))

	// b.py: Python, Code=10, Comments=2, Blanks=3, Total=15
	root.AddChild(structure.NewFileEntry("root/b.py", map[string]structure.CodeStats{
		"Python": {Code: 10, Comments: 2, Blanks: 3},
	}))

	// c.go: Go, Code=30, Comments=10, Blanks=10, Total=50
	root.AddChild(structure.NewFileEntry("root/c.go", map[string]structure.CodeStats{
		"Go": {Code: 30, Comments: 10, Blanks: 10},
	}))

	root.AggregateStats()

	dm := NewDirModel(NewCodeNavigation(structure.NewTree(root)), "", false, false)
	dm.languages = []string{"Go", "Python"}
	dm.langFilterIdx = -1
	dm.selectedLangs = make(map[string]bool)
	dm.sortState = SortState{Key: SortByTotal, Desc: true}
	return dm
}

func TestDirModelComparableStats(t *testing.T) {
	dm := newTestDirModel()
	root := dm.nav.Entry()

	t.Run("no filter uses total stats", func(t *testing.T) {
		got := dm.comparableStats(root.Child[0])
		want := structure.CodeStats{Code: 20, Comments: 5, Blanks: 5}
		if got != want {
			t.Errorf("a.go total stats: got %+v, want %+v", got, want)
		}
	})

	t.Run("single language filter", func(t *testing.T) {
		dm.langFilterIdx = 0                     // Go
		got := dm.comparableStats(root.Child[1]) // b.py has no Go stats
		want := structure.CodeStats{}
		if got != want {
			t.Errorf("b.py Go stats: got %+v, want %+v", got, want)
		}
	})

	t.Run("multi language filter aggregates", func(t *testing.T) {
		dm.langFilterIdx = -1
		dm.selectedLangs["Go"] = true
		dm.selectedLangs["Python"] = true
		got := dm.comparableStats(root)
		want := structure.CodeStats{Code: 60, Comments: 17, Blanks: 18}
		if got != want {
			t.Errorf("root aggregated stats: got %+v, want %+v", got, want)
		}
	})
}

func TestDirModelBuildChildComparator(t *testing.T) {
	t.Run("sort by name ascending", func(t *testing.T) {
		dm := newTestDirModel()
		dm.sortState = SortState{Key: SortByName, Desc: false}
		dm.nav.Entry().SortChildBy(dm.buildChildComparator())
		want := []string{"a.go", "b.py", "c.go"}
		for i, child := range dm.nav.Entry().Child {
			if child.Name() != want[i] {
				t.Errorf("position %d: expected %q, got %q", i, want[i], child.Name())
			}
		}
	})

	t.Run("sort by code descending", func(t *testing.T) {
		dm := newTestDirModel()
		dm.sortState = SortState{Key: SortByCode, Desc: true}
		dm.nav.Entry().SortChildBy(dm.buildChildComparator())
		want := []int64{30, 20, 10}
		for i, child := range dm.nav.Entry().Child {
			got := dm.comparableStats(child).Code
			if got != want[i] {
				t.Errorf("position %d: expected code %d, got %d", i, want[i], got)
			}
		}
	})

	t.Run("sort by total ascending", func(t *testing.T) {
		dm := newTestDirModel()
		dm.sortState = SortState{Key: SortByTotal, Desc: false}
		dm.nav.Entry().SortChildBy(dm.buildChildComparator())
		want := []int64{15, 30, 50}
		for i, child := range dm.nav.Entry().Child {
			got := dm.comparableStats(child).Total()
			if got != want[i] {
				t.Errorf("position %d: expected total %d, got %d", i, want[i], got)
			}
		}
	})

	t.Run("sort by percent descending", func(t *testing.T) {
		dm := newTestDirModel()
		dm.sortState = SortState{Key: SortByPercent, Desc: true}
		dm.nav.Entry().SortChildBy(dm.buildChildComparator())
		want := []int64{50, 30, 15}
		for i, child := range dm.nav.Entry().Child {
			got := dm.comparableStats(child).Total()
			if got != want[i] {
				t.Errorf("position %d: expected total %d, got %d", i, want[i], got)
			}
		}
	})

	t.Run("multi-language sort by code descending", func(t *testing.T) {
		dm := newTestDirModel()
		dm.selectedLangs["Go"] = true
		dm.selectedLangs["Python"] = true
		dm.sortState = SortState{Key: SortByCode, Desc: true}
		dm.nav.Entry().SortChildBy(dm.buildChildComparator())
		// a.go=20, c.go=30, b.py=10 when aggregated
		want := []int64{30, 20, 10}
		for i, child := range dm.nav.Entry().Child {
			got := dm.comparableStats(child).Code
			if got != want[i] {
				t.Errorf("position %d: expected code %d, got %d", i, want[i], got)
			}
		}
	})
}

func TestDirModelCycleSortColumn(t *testing.T) {
	dm := newTestDirModel()
	// newTestDirModel initializes sortState to SortByTotal, so the first cycle
	// moves to SortByPercent, then wraps back to SortByName.
	order := []SortKey{SortByPercent, SortByName, SortByLanguages, SortByCode, SortByComments, SortByBlanks, SortByTotal}

	for i := 0; i < len(order)*2; i++ {
		expected := order[i%len(order)]
		dm.cycleSortColumn()
		if dm.sortState.Key != expected {
			t.Errorf("cycle %d: expected key %q, got %q", i, expected, dm.sortState.Key)
		}
		if dm.sortState.Desc != defaultDescForSortKey(expected) {
			t.Errorf("cycle %d: expected default desc %v for %q", i, defaultDescForSortKey(expected), expected)
		}
	}
}

func TestDirModelToggleSortOrder(t *testing.T) {
	dm := newTestDirModel()
	dm.sortState = SortState{Key: SortByTotal, Desc: true}

	dm.toggleSortOrder()
	if dm.sortState.Desc != false {
		t.Errorf("expected Desc=false after toggle")
	}
	dm.toggleSortOrder()
	if dm.sortState.Desc != true {
		t.Errorf("expected Desc=true after second toggle")
	}
	if dm.sortState.Key != SortByTotal {
		t.Errorf("expected key to remain %q", SortByTotal)
	}
}

func TestDirModelGlobalSearchOpenClose(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})

	dm.openGlobalSearch()
	if dm.mode != SEARCH {
		t.Fatalf("expected SEARCH mode, got %v", dm.mode)
	}
	if dm.searchIndex == nil {
		t.Fatal("expected search index to be built")
	}

	dm.closeGlobalSearch()
	if dm.mode != READY {
		t.Fatalf("expected READY mode after close, got %v", dm.mode)
	}
}

func TestDirModelGlobalSearchFindsFile(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.openGlobalSearch()

	dm.searchInput.SetValue("b.py")
	dm.updateSearchQuery()

	if len(dm.searchMatches) == 0 {
		t.Fatalf("expected matches for 'b.py', got none")
	}
	if dm.searchMatches[0].Item.Path != "b.py" {
		t.Errorf("expected first match to be b.py, got %q", dm.searchMatches[0].Item.Path)
	}
}

func TestDirModelApplySearchResultDirectory(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.openGlobalSearch()

	// "root" is the top-level directory; selecting it should keep us at root.
	dm.searchInput.SetValue("root")
	dm.updateSearchQuery()
	dm.applySearchResult()

	if dm.mode != READY {
		t.Fatalf("expected READY mode after apply, got %v", dm.mode)
	}
	if dm.nav.Entry().Path != "root" {
		t.Errorf("expected nav at root, got %q", dm.nav.Entry().Path)
	}
}

func TestDirModelApplySearchResultFile(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.openGlobalSearch()

	dm.searchInput.SetValue("b.py")
	dm.updateSearchQuery()
	dm.applySearchResult()

	if dm.mode != READY {
		t.Fatalf("expected READY mode after apply, got %v", dm.mode)
	}
	if dm.nav.Entry().Path != "root" {
		t.Errorf("expected nav at parent directory root, got %q", dm.nav.Entry().Path)
	}

	idx := dm.findChildIndex(dm.nav.Entry().GetChild("b.py"))
	if idx < 0 {
		t.Fatal("expected b.py to be visible in the table")
	}
	if dm.dirsTable.Cursor() != idx {
		t.Errorf("expected cursor at b.py index %d, got %d", idx, dm.dirsTable.Cursor())
	}
}

func newTestNestedDirModel() *DirModel {
	root := structure.NewDirEntry("root")

	root.AddChild(structure.NewFileEntry("root/a.go", map[string]structure.CodeStats{
		"Go": {Code: 20, Comments: 5, Blanks: 5},
	}))

	subdir := structure.NewDirEntry("root/subdir")
	root.AddChild(subdir)
	subdir.AddChild(structure.NewFileEntry("root/subdir/b.py", map[string]structure.CodeStats{
		"Python": {Code: 10, Comments: 2, Blanks: 3},
	}))

	root.AggregateStats()

	dm := NewDirModel(NewCodeNavigation(structure.NewTree(root)), "", false, false)
	dm.languages = []string{"Go", "Python"}
	dm.langFilterIdx = -1
	dm.selectedLangs = make(map[string]bool)
	dm.sortState = SortState{Key: SortByTotal, Desc: true}
	return dm
}

func TestDirModelApplySearchResultFromTreeMode(t *testing.T) {
	dm := newTestNestedDirModel()
	dm.Update(ScanFinished{})
	dm.treeMode = true
	dm.openGlobalSearch()

	dm.searchInput.SetValue("b.py")
	dm.updateSearchQuery()
	dm.applySearchResult()

	if !dm.treeMode {
		t.Error("expected tree mode to remain active after global search jump")
	}
	if dm.treemapMode {
		t.Error("expected treemap mode to remain disabled after global search jump")
	}
	if dm.nav.Entry().Path != "root" {
		t.Errorf("expected tree root to stay at project root, got %q", dm.nav.Entry().Path)
	}
	subdir := dm.nav.Entry().GetChild("subdir")
	if subdir == nil {
		t.Fatal("expected subdir to be visible under root")
	}
	if !subdir.Expanded {
		t.Error("expected subdir to be expanded so the target file is visible")
	}

	target := subdir.GetChild("b.py")
	if target == nil {
		t.Fatal("expected b.py to be visible under subdir")
	}
	idx := dm.findChildIndex(target)
	if idx < 0 {
		t.Fatal("expected b.py to be visible in the tree table")
	}
	if dm.dirsTable.Cursor() != idx {
		t.Errorf("expected cursor on b.py index %d, got %d", idx, dm.dirsTable.Cursor())
	}
}

func TestDirModelApplySearchResultFromTreeModeDirectory(t *testing.T) {
	dm := newTestNestedDirModel()
	dm.Update(ScanFinished{})
	dm.treeMode = true
	dm.openGlobalSearch()

	dm.searchInput.SetValue("subdir")
	dm.updateSearchQuery()
	dm.applySearchResult()

	if !dm.treeMode {
		t.Error("expected tree mode to remain active after global search jump")
	}
	if dm.nav.Entry().Path != "root" {
		t.Errorf("expected nav at parent directory root, got %q", dm.nav.Entry().Path)
	}
	if !dm.nav.Entry().GetChild("subdir").Expanded {
		t.Error("expected subdir to be expanded after searching for it in tree mode")
	}

	idx := dm.findChildIndex(dm.nav.Entry().GetChild("subdir"))
	if idx < 0 {
		t.Fatal("expected subdir to be visible after search jump")
	}
	if dm.dirsTable.Cursor() != idx {
		t.Errorf("expected cursor on subdir index %d, got %d", idx, dm.dirsTable.Cursor())
	}
}

func TestDirModelApplySearchResultFromTreemapMode(t *testing.T) {
	dm := newTestNestedDirModel()
	dm.Update(ScanFinished{})
	dm.width = 80
	dm.height = 24
	dm.treemapMode = true
	dm.openGlobalSearch()

	dm.searchInput.SetValue("b.py")
	dm.updateSearchQuery()
	dm.applySearchResult()

	if !dm.treemapMode {
		t.Error("expected treemap mode to remain active after global search jump")
	}
	if dm.treeMode {
		t.Error("expected tree mode to remain disabled after global search jump")
	}
	if dm.nav.Entry().Path != "root/subdir" {
		t.Errorf("expected nav at parent directory root/subdir, got %q", dm.nav.Entry().Path)
	}

	_ = dm.View() // trigger treemap block layout

	idx := dm.findTreemapBlockIndex(dm.nav.Entry().GetChild("b.py"))
	if idx < 0 {
		t.Fatalf("expected b.py to be visible in treemap blocks after search jump, got %d blocks", len(dm.treemapBlocks))
	}
	if dm.treemapSelected != idx {
		t.Errorf("expected treemap selection on b.py index %d, got %d", idx, dm.treemapSelected)
	}
}

func TestDirModelApplySearchResultFromTreemapModeDirectory(t *testing.T) {
	dm := newTestNestedDirModel()
	dm.Update(ScanFinished{})
	dm.width = 80
	dm.height = 24
	dm.treemapMode = true
	dm.openGlobalSearch()

	dm.searchInput.SetValue("subdir")
	dm.updateSearchQuery()
	dm.applySearchResult()

	if !dm.treemapMode {
		t.Error("expected treemap mode to remain active after global search jump")
	}
	if dm.treeMode {
		t.Error("expected tree mode to remain disabled after global search jump")
	}
	if dm.nav.Entry().Path != "root/subdir" {
		t.Errorf("expected nav to enter subdir, got %q", dm.nav.Entry().Path)
	}

	_ = dm.View() // trigger treemap block layout

	// After entering subdir, the treemap should show its children (b.py).
	idx := dm.findTreemapBlockIndex(dm.nav.Entry().GetChild("b.py"))
	if idx < 0 {
		t.Fatalf("expected b.py to be visible in treemap blocks after jumping to subdir, got %d blocks", len(dm.treemapBlocks))
	}
}

func TestViewModelGlobalSearchJumpsToFile(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	vm := NewViewModel(dm.nav, dm)

	// Press Ctrl+P to open global search.
	_, cmd := vm.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if cmd != nil {
		t.Fatalf("expected Ctrl+P not to return a command, got %v", cmd)
	}
	if dm.mode != SEARCH {
		t.Fatalf("expected SEARCH mode after Ctrl+P, got %v", dm.mode)
	}

	// Type the query and update the model directly (DirModel handles typing).
	dm.searchInput.SetValue("b.py")
	dm.updateSearchQuery()

	// Press Enter to apply the search result.
	_, cmd = vm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("expected Enter not to return a command, got %v", cmd)
	}
	if dm.mode != READY {
		t.Fatalf("expected READY mode after applying search result, got %v", dm.mode)
	}
	if dm.nav.Entry().Path != "root" {
		t.Errorf("expected navigation at root, got %q", dm.nav.Entry().Path)
	}

	idx := dm.findChildIndex(dm.nav.Entry().GetChild("b.py"))
	if idx < 0 {
		t.Fatal("expected b.py to be visible after search jump")
	}
	if dm.dirsTable.Cursor() != idx {
		t.Errorf("expected cursor on b.py (index %d), got %d", idx, dm.dirsTable.Cursor())
	}
}

func TestViewModelPreviewQClosesPreviewWithoutQuitting(t *testing.T) {
	dm := newTestDirModel()
	dm.mode = PREVIEW
	dm.filePreview = &FilePreview{}
	vm := NewViewModel(nil, dm)

	_, cmd := vm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if cmd != nil {
		t.Fatalf("expected q in preview mode not to quit")
	}
	if dm.mode != READY {
		t.Fatalf("expected q to return to ready mode, got %v", dm.mode)
	}
	if dm.filePreview != nil {
		t.Fatalf("expected q to close preview")
	}
}

func TestViewModelInputQFiltersWithoutQuitting(t *testing.T) {
	testDM := newTestDirModel()
	dm := NewDirModel(NewCodeNavigation(structure.NewTree(testDM.nav.Entry())), "", false, false)
	dm.mode = INPUT
	dm.filters.ToggleFilter(filter.NameFilterID)
	vm := NewViewModel(nil, dm)

	_, cmd := vm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if cmd != nil {
		t.Fatalf("expected q in input mode not to quit")
	}
	if dm.mode != INPUT {
		t.Fatalf("expected q to keep input mode, got %v", dm.mode)
	}
	if got := len(dm.dirsTable.Rows()); got != 0 {
		t.Fatalf("expected q to be applied to the name filter, got %d rows", got)
	}
}

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

func TestByteIndexesToRuneIndexes(t *testing.T) {
	// "中" is 3 bytes; "a中b" has byte offsets 0,1,4,5 and rune offsets 0,1,2,3.
	s := "a中b"
	byteIdxs := []int{0, 1, 4, 5}
	want := []int{0, 1, 2, 3}

	got := byteIndexesToRuneIndexes(s, byteIdxs)
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("at index %d: expected %d, got %d", i, want[i], got[i])
		}
	}
}

func TestDirModelGlobalSearchPgDownNoMatches(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.openGlobalSearch()

	dm.searchInput.SetValue("nonexistent")
	dm.updateSearchQuery()

	if len(dm.searchMatches) != 0 {
		t.Fatalf("expected no matches, got %d", len(dm.searchMatches))
	}

	msg := tea.KeyMsg{Type: tea.KeyPgDown}
	dm.Update(msg)

	if dm.searchCursor != 0 {
		t.Errorf("expected cursor to stay at 0 with no matches, got %d", dm.searchCursor)
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
