package render

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/zdyxry/tokui/filter"
	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/structure"
)

// newTestCompareDirModel builds a Compare-mode DirModel. Layout: main.go grew
// (900 → 1000 code, 8 → 10 cmplx), new.py is added (480 code, 5 cmplx),
// old/legacy.rb is deleted (700 code, 12 cmplx in S1), unchanged.go is
// identical in both snapshots.
func newTestCompareDirModel(t *testing.T) *DirModel {
	t.Helper()

	s1 := provider.Result{Files: []provider.FileStats{
		{Path: "/repo/src/main.go", Language: "Go", Code: 900, Comments: 90, Blanks: 40, Complexity: 8},
		{Path: "/repo/old/legacy.rb", Language: "Ruby", Code: 700, Comments: 10, Blanks: 20, Complexity: 12},
		{Path: "/repo/unchanged.go", Language: "Go", Code: 500, Comments: 5, Blanks: 5, Complexity: 2},
	}}
	s2 := provider.Result{Files: []provider.FileStats{
		{Path: "/repo/src/main.go", Language: "Go", Code: 1000, Comments: 100, Blanks: 50, Complexity: 10},
		{Path: "/repo/new.py", Language: "Python", Code: 480, Comments: 20, Blanks: 10, Complexity: 5},
		{Path: "/repo/unchanged.go", Language: "Go", Code: 500, Comments: 5, Blanks: 5, Complexity: 2},
	}}

	tree := structure.NewTree(nil)
	if err := tree.BuildFromCompare(s1, s2, "/repo"); err != nil {
		t.Fatalf("BuildFromCompare: %v", err)
	}

	info := provider.Info{
		Name:         "scc",
		Version:      "3.x",
		Capabilities: provider.CapLines | provider.CapComplexity | provider.CapDelta,
	}
	mode := ModeInfo{Kind: ModeCompare, Range: "v1.0..v2.0"}
	dm := NewDirModelWithMode(NewCodeNavigation(tree), info, mode, false, false)
	dm.width = 140
	dm.Update(ScanFinished{})
	return dm
}

func TestCompareModeColumns(t *testing.T) {
	dm := newTestCompareDirModel(t)

	var got []SortKey
	for _, c := range dm.columns {
		got = append(got, c.SortKey)
	}
	want := []SortKey{
		SortByNone, SortByNone, SortByName,
		SortByCode, SortByDelta, SortByComplexity,
	}
	if len(got) != len(want) {
		t.Fatalf("expected columns %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("column %d: expected %q, got %q", i, want[i], got[i])
		}
	}

	// Default sort is |ΔCode| descending.
	if dm.sortState.Key != SortByDelta || !dm.sortState.Desc {
		t.Errorf("expected default sort ΔCode desc, got %q desc=%v", dm.sortState.Key, dm.sortState.Desc)
	}
	if !dm.changedOnly() {
		t.Error("expected changed-only filter to start enabled")
	}
}

func TestCompareModeRowValues(t *testing.T) {
	dm := newTestCompareDirModel(t)

	rows := map[string][]string{}
	collect := func() {
		rows = map[string][]string{}
		for i, te := range dm.tableEntries {
			rows[te.entry.Name()] = dm.dirsTable.Rows()[i]
		}
	}
	collect()

	// Columns: icon, path, name, codeCompare, ΔCode, ΔCmplx.
	newPy := rows["new.py"]
	if newPy == nil {
		t.Fatalf("expected a row for new.py, got %v", rowNames(dm))
	}
	if newPy[3] != "0 → 480" || newPy[4] != "+480" || newPy[5] != "+5" {
		t.Errorf("new.py cells mismatch: %q %q %q", newPy[3], newPy[4], newPy[5])
	}

	// Directory rows aggregate both sides.
	src := rows["src"]
	if src == nil {
		t.Fatal("expected a row for src")
	}
	if src[3] != "900 → 1,000" || src[4] != "+100" || src[5] != "+2" {
		t.Errorf("src cells mismatch: %q %q %q", src[3], src[4], src[5])
	}

	// The deleted file shows its S1 value on the left and carries the marker.
	dm.nav.Down("old", 0, 0)
	dm.updateTableData()
	collect()
	legacy := rows["legacy.rb"]
	if legacy == nil {
		t.Fatalf("expected a row for legacy.rb, got %v", rowNames(dm))
	}
	if legacy[2] != "legacy.rb (已删除)" {
		t.Errorf("expected deleted marker in name cell, got %q", legacy[2])
	}
	if legacy[3] != "700 → 0" || legacy[4] != "-700" || legacy[5] != "-12" {
		t.Errorf("legacy.rb cells mismatch: %q %q %q", legacy[3], legacy[4], legacy[5])
	}

	// Unchanged files are hidden by the changed-only filter.
	if _, ok := rows["unchanged.go"]; ok {
		t.Error("unchanged.go must be hidden in changed-only mode")
	}
}

func TestCompareModeToggleAll(t *testing.T) {
	dm := newTestCompareDirModel(t)

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if dm.changedOnly() {
		t.Error("expected changed-only filter off after 'a'")
	}
	found := false
	for _, name := range rowNames(dm) {
		if name == "unchanged.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unchanged.go in show-all view, got %v", rowNames(dm))
	}
}

func TestCompareModeSortCycle(t *testing.T) {
	dm := newTestCompareDirModel(t)
	// Starting from ΔCode, the cycle is ΔCmplx, Name, then wraps to ΔCode.
	order := []SortKey{SortByComplexity, SortByName, SortByDelta}
	for i := 0; i < len(order)*2; i++ {
		expected := order[i%len(order)]
		dm.cycleSortColumn()
		if dm.sortState.Key != expected {
			t.Errorf("cycle %d: expected key %q, got %q", i, expected, dm.sortState.Key)
		}
	}
}

func TestCompareModeSortComparators(t *testing.T) {
	dm := newTestCompareDirModel(t)
	dm.filters.ToggleFilter(filter.ChangedFilterID)
	dm.updateTableData()

	// |ΔCode| desc: old 700 > new.py 480 > src 100 > unchanged 0.
	dm.sortState = SortState{Key: SortByDelta, Desc: true}
	dm.updateTableData()
	want := []string{"old", "new.py", "src", "unchanged.go"}
	got := rowNames(dm)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ΔCode position %d: expected %q, got %q (full: %v)", i, want[i], got[i], got)
		}
	}

	// |ΔCmplx| desc: old 12 > new.py 5 > src 2 > unchanged 0.
	dm.sortState = SortState{Key: SortByComplexity, Desc: true}
	dm.updateTableData()
	want = []string{"old", "new.py", "src", "unchanged.go"}
	got = rowNames(dm)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ΔCmplx position %d: expected %q, got %q (full: %v)", i, want[i], got[i], got)
		}
	}
}

func TestCompareModeDirsSummary(t *testing.T) {
	dm := newTestCompareDirModel(t)
	summary := dm.dirsSummary()

	for _, want := range []string{"RANGE", "v1.0..v2.0", "3 files", "-120", "-5", "ΔCODE", "ΔCMPLX", "scc 3.x"} {
		if !strings.Contains(summary, want) {
			t.Errorf("expected summary to contain %q, got %q", want, summary)
		}
	}
}

func TestCompareModeVisibleColumnsNarrow(t *testing.T) {
	dm := newTestCompareDirModel(t)
	dm.sortState = SortState{Key: SortByName, Desc: false}

	// Wide: everything visible.
	dm.width = 140
	if got := len(dm.visibleColumns()); got != len(dm.columns) {
		t.Errorf("width 140: expected %d columns, got %d", len(dm.columns), got)
	}

	// ΔCmplx hides below 80, the compare column below 60.
	dm.width = 79
	for _, c := range dm.visibleColumns() {
		if c.SortKey == SortByComplexity {
			t.Error("width 79: expected ΔCmplx hidden")
		}
	}
	dm.width = 59
	for _, c := range dm.visibleColumns() {
		if c.SortKey == SortByCode || c.SortKey == SortByComplexity {
			t.Errorf("width 59: expected %q hidden", c.SortKey)
		}
	}
}

func TestCompareModeUnchangedRowsFaint(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	dm := newTestCompareDirModel(t)
	dm.filters.ToggleFilter(filter.ChangedFilterID)
	dm.updateTableData()

	for i, te := range dm.tableEntries {
		row := dm.dirsTable.Rows()[i]
		if te.entry.Name() == "unchanged.go" {
			if !strings.Contains(row[2], "\x1b[2m") {
				t.Errorf("expected unchanged.go name cell to be faint, got %q", row[2])
			}
		}
	}
}

func TestCompareModeParentRowCodeCellEmpty(t *testing.T) {
	dm := newTestCompareDirModel(t)
	dm.nav.Down("old", 0, 0)
	dm.updateTableData()

	if len(dm.tableEntries) == 0 || !dm.tableEntries[0].isParent {
		t.Fatal("expected the synthetic parent row first")
	}
	row := dm.dirsTable.Rows()[0]
	// Columns: icon, path, name, codeCompare, ΔCode, ΔCmplx. The S1 → S2 cell
	// is meaningless for ".." and stays empty.
	if got := row[3]; got != "" {
		t.Errorf("expected empty Code (S1 → S2) cell for the parent row, got %q", got)
	}
	if got := row[2]; got != ".." {
		t.Errorf("expected name cell %q, got %q", "..", got)
	}
}
