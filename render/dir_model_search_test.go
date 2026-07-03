package render

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

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
	require.NotNil(t, subdir, "expected subdir to be visible under root")
	if !subdir.Expanded {
		t.Error("expected subdir to be expanded so the target file is visible")
	}

	target := subdir.GetChild("b.py")
	require.NotNil(t, target, "expected b.py to be visible under subdir")
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
