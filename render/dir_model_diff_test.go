package render

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/zdyxry/tokui/filter"
	"github.com/zdyxry/tokui/gitx"
	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/structure"
)

// newTestDiffDirModel builds a Diff-mode DirModel from a small change set.
// Layout: changed files a.go (+120/-45), sub/c.go (+80/-230, net -150),
// unchanged.go (no churn), and old/deleted.go (deleted, no S2 stats).
func newTestDiffDirModel(t *testing.T) *DirModel {
	t.Helper()

	changes := []gitx.FileChange{
		{Path: "a.go", Added: 120, Deleted: 45, Kind: gitx.Modified},
		{Path: "sub/c.go", Added: 80, Deleted: 230, Kind: gitx.Modified},
		{Path: "old/deleted.go", Added: 0, Deleted: 60, Kind: gitx.Deleted},
	}
	s2 := provider.Result{Files: []provider.FileStats{
		{Path: "/repo/a.go", Language: "Go", Code: 200, Comments: 20, Blanks: 10},
		{Path: "/repo/sub/c.go", Language: "Go", Code: 100, Comments: 10, Blanks: 5},
		{Path: "/repo/unchanged.go", Language: "Go", Code: 999, Comments: 9, Blanks: 9},
	}}

	tree := structure.NewTree(nil)
	if err := tree.BuildFromDiff(changes, s2, "/repo"); err != nil {
		t.Fatalf("BuildFromDiff: %v", err)
	}

	info := provider.Info{
		Name:         "tokei",
		Version:      "12.1",
		Capabilities: provider.CapLines | provider.CapChurn,
	}
	mode := ModeInfo{Kind: ModeDiff, Range: "main...HEAD"}
	dm := NewDirModelWithMode(NewCodeNavigation(tree), info, mode, false, false)
	dm.width = 140
	dm.Update(ScanFinished{})
	return dm
}

func rowNames(dm *DirModel) []string {
	names := make([]string, 0, len(dm.tableEntries))
	for _, te := range dm.tableEntries {
		names = append(names, te.entry.Name())
	}
	return names
}

func TestNewDirModelWithMode_DiffColumns(t *testing.T) {
	dm := newTestDiffDirModel(t)

	var got []SortKey
	for _, c := range dm.columns {
		got = append(got, c.SortKey)
	}
	want := []SortKey{
		SortByNone, SortByNone, SortByName, SortByLanguages,
		SortByAdded, SortByDeleted, SortByDelta, SortByPercent, SortByCode, SortByTotal,
	}
	if len(got) != len(want) {
		t.Fatalf("expected columns %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("column %d: expected %q, got %q", i, want[i], got[i])
		}
	}

	// Default sort is |Δ| descending.
	if dm.sortState.Key != SortByDelta || !dm.sortState.Desc {
		t.Errorf("expected default sort Δ desc, got %q desc=%v", dm.sortState.Key, dm.sortState.Desc)
	}

	// The changed-only filter starts enabled.
	if !dm.changedOnly() {
		t.Error("expected changed-only filter to start enabled")
	}
}

func TestDiffModeChurnRowValues(t *testing.T) {
	dm := newTestDiffDirModel(t)

	// Find the row for a.go and check its churn cells.
	var row []string
	for i, te := range dm.tableEntries {
		if te.entry.Name() == "a.go" {
			row = dm.dirsTable.Rows()[i]
			break
		}
	}
	if row == nil {
		t.Fatal("expected a row for a.go")
	}

	// Columns: icon, path, name, lang, +, -, Δ, %, code, total.
	// Churn share: a.go 165 / root 535 = 30.8 %.
	want := []string{"", "", "a.go", "Go", "+120", "-45", "+75", "30.8 %", "200", "230"}
	for i := range want {
		if i < 2 {
			continue // icon and hidden path
		}
		if row[i] != want[i] {
			t.Errorf("cell %d: expected %q, got %q", i, want[i], row[i])
		}
	}

	// Directory rows aggregate churn: sub 310 / root 535 = 57.9 %.
	for i, te := range dm.tableEntries {
		if te.entry.Name() == "sub" {
			if got := dm.dirsTable.Rows()[i][7]; got != "57.9 %" {
				t.Errorf("sub %% cell: expected %q, got %q", "57.9 %", got)
			}
			break
		}
	}
}

func TestDiffModeChangedOnlyFilter(t *testing.T) {
	dm := newTestDiffDirModel(t)

	// Changed-only: unchanged.go is hidden, changed entries stay.
	for _, name := range rowNames(dm) {
		if name == "unchanged.go" {
			t.Error("unchanged.go must be hidden in changed-only mode")
		}
	}
	names := strings.Join(rowNames(dm), ",")
	for _, want := range []string{"a.go", "sub", "old"} {
		if !strings.Contains(names, want) {
			t.Errorf("expected %q in changed-only view, got %v", want, names)
		}
	}

	// Toggle "a" to show the full snapshot.
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

	// Toggle back.
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !dm.changedOnly() {
		t.Error("expected changed-only filter back on after second 'a'")
	}
}

func TestDiffModeUnchangedRowsFaint(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	dm := newTestDiffDirModel(t)
	// Show all files.
	dm.filters.ToggleFilter(filter.ChangedFilterID)
	dm.updateTableData()

	for i, te := range dm.tableEntries {
		row := dm.dirsTable.Rows()[i]
		if te.entry.Name() == "unchanged.go" {
			if !strings.Contains(row[2], "\x1b[2m") {
				t.Errorf("expected unchanged.go name cell to be faint, got %q", row[2])
			}
			if strings.Contains(row[1], "\x1b[") {
				t.Errorf("hidden path column must stay unstyled, got %q", row[1])
			}
		}
		if te.entry.Name() == "a.go" && strings.Contains(row[2], "\x1b[2m") {
			t.Errorf("changed row a.go must not be faint, got %q", row[2])
		}
	}
}

func TestDiffModeSortComparators(t *testing.T) {
	dm := newTestDiffDirModel(t)
	// Show all rows so unchanged.go participates in sorting.
	dm.filters.ToggleFilter(filter.ChangedFilterID)
	dm.updateTableData()

	cases := []struct {
		key  SortKey
		want []string // expected order, descending
	}{
		// Added: a.go 120 > sub 80 > old 0 = unchanged 0 (tie).
		{SortByAdded, []string{"a.go", "sub", "old", "unchanged.go"}},
		// Deleted: sub 230 > old 60 > a.go 45 > unchanged 0.
		{SortByDeleted, []string{"sub", "old", "a.go", "unchanged.go"}},
		// |Δ|: sub 150 > a.go 75 > old 60 > unchanged 0.
		{SortByDelta, []string{"sub", "a.go", "old", "unchanged.go"}},
		// Churn volume (% share): sub 310 > a.go 165 > old 60 > unchanged 0.
		{SortByPercent, []string{"sub", "a.go", "old", "unchanged.go"}},
	}

	for _, tc := range cases {
		dm.sortState = SortState{Key: tc.key, Desc: true}
		dm.updateTableData()
		got := rowNames(dm)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: expected %d rows, got %v", tc.key, len(tc.want), got)
		}
		limit := len(tc.want)
		if tc.key == SortByAdded {
			limit = 2 // the zero-churn tail order is unspecified
		}
		for i := 0; i < limit; i++ {
			if got[i] != tc.want[i] {
				t.Errorf("%s position %d: expected %q, got %q (full: %v)", tc.key, i, tc.want[i], got[i], got)
			}
		}
	}
}

func TestDiffModeCycleSortColumn(t *testing.T) {
	dm := newTestDiffDirModel(t)
	// Starting from Δ, the cycle is %, Code, Total, Name, +, -, then wraps to Δ.
	order := []SortKey{SortByPercent, SortByCode, SortByTotal, SortByName, SortByAdded, SortByDeleted, SortByDelta}

	for i := 0; i < len(order)*2; i++ {
		expected := order[i%len(order)]
		dm.cycleSortColumn()
		if dm.sortState.Key != expected {
			t.Errorf("cycle %d: expected key %q, got %q", i, expected, dm.sortState.Key)
		}
		if dm.sortState.Desc != defaultDescForSortKey(expected) {
			t.Errorf("cycle %d: unexpected default direction for %q", i, expected)
		}
	}
}

func TestDiffModeVisibleColumnsNarrow(t *testing.T) {
	dm := newTestDiffDirModel(t)
	// Sort by Name so the sort-column protection does not keep churn columns.
	dm.sortState = SortState{Key: SortByName, Desc: false}

	// Hiding order as the terminal narrows: Total, %, Code, Δ, -, +.
	cases := []struct {
		width  int
		hidden SortKey
	}{
		{119, SortByTotal},
		{114, SortByPercent},
		{104, SortByCode},
		{89, SortByDelta},
		{74, SortByDeleted},
		{59, SortByAdded},
	}
	for _, tc := range cases {
		dm.width = tc.width
		for _, c := range dm.visibleColumns() {
			if c.SortKey == tc.hidden {
				t.Errorf("width %d: expected %q hidden", tc.width, tc.hidden)
			}
		}
	}

	// Wide enough: everything visible.
	dm.width = 140
	if got := len(dm.visibleColumns()); got != len(dm.columns) {
		t.Errorf("width 140: expected %d columns, got %d", len(dm.columns), got)
	}

	// The active sort column is never hidden.
	dm.width = 59
	dm.sortState = SortState{Key: SortByAdded, Desc: true}
	found := false
	for _, c := range dm.visibleColumns() {
		if c.SortKey == SortByAdded {
			found = true
		}
	}
	if !found {
		t.Error("active sort column + must stay visible on narrow screens")
	}
}

func TestDiffModeDirsSummary(t *testing.T) {
	dm := newTestDiffDirModel(t)
	summary := dm.dirsSummary()

	for _, want := range []string{"RANGE", "main...HEAD", "3 files", "+200 / -335", "CHANGED", "CHURN", "DELTA", "tokei 12.1"} {
		if !strings.Contains(summary, want) {
			t.Errorf("expected summary to contain %q, got %q", want, summary)
		}
	}
}

func TestDiffModeDirsSummaryEmptyDiff(t *testing.T) {
	s2 := provider.Result{Files: []provider.FileStats{
		{Path: "/repo/a.go", Language: "Go", Code: 10},
	}}
	tree := structure.NewTree(nil)
	if err := tree.BuildFromDiff(nil, s2, "/repo"); err != nil {
		t.Fatalf("BuildFromDiff: %v", err)
	}
	info := provider.Info{Name: "tokei", Version: "12.1", Capabilities: provider.CapLines | provider.CapChurn}
	dm := NewDirModelWithMode(NewCodeNavigation(tree), info, ModeInfo{Kind: ModeDiff, Range: "main...HEAD"}, false, false)
	dm.width = 140
	dm.Update(ScanFinished{})

	summary := dm.dirsSummary()
	if !strings.Contains(summary, "no changes in main...HEAD") {
		t.Errorf("expected empty-diff hint, got %q", summary)
	}
}

func TestDiffModeDeletedFileRow(t *testing.T) {
	dm := newTestDiffDirModel(t)

	var deleted *structure.Entry
	for _, te := range dm.tableEntries {
		if te.entry.Name() == "old" {
			deleted = te.entry.GetChild("deleted.go")
		}
	}
	if deleted == nil {
		t.Fatal("expected old/deleted.go entry")
	}
	if deleted.Change.Kind != gitx.Deleted {
		t.Errorf("expected kind Deleted, got %v", deleted.Change.Kind)
	}
	if deleted.TotalStats != (structure.CodeStats{}) {
		t.Errorf("expected zeroed S2 stats for deleted file, got %+v", deleted.TotalStats)
	}
}

func TestFormatSigned(t *testing.T) {
	cases := []struct {
		n    int64
		neg  bool
		want string
	}{
		{120, false, "+120"},
		{45, true, "-45"},
		{0, false, "0"},
		{0, true, "0"},
		{-150, false, "-150"},
	}
	for _, tc := range cases {
		if got := formatSigned(tc.n, tc.neg); got != tc.want {
			t.Errorf("formatSigned(%d, %v) = %q, want %q", tc.n, tc.neg, got, tc.want)
		}
	}
}

// newTestBinaryDiffDirModel builds a Diff-mode model whose only change is a
// binary file with zero line churn.
func newTestBinaryDiffDirModel(t *testing.T) *DirModel {
	t.Helper()

	changes := []gitx.FileChange{
		{Path: "assets/logo.png", Added: 0, Deleted: 0, Kind: gitx.Modified},
	}
	s2 := provider.Result{Files: []provider.FileStats{
		{Path: "/repo/assets/logo.png", Language: "Image", Code: 1},
		{Path: "/repo/main.go", Language: "Go", Code: 100},
	}}
	tree := structure.NewTree(nil)
	if err := tree.BuildFromDiff(changes, s2, "/repo"); err != nil {
		t.Fatalf("BuildFromDiff: %v", err)
	}
	info := provider.Info{Name: "tokei", Version: "12.1", Capabilities: provider.CapLines | provider.CapChurn}
	dm := NewDirModelWithMode(NewCodeNavigation(tree), info, ModeInfo{Kind: ModeDiff, Range: "HEAD"}, false, false)
	dm.width = 140
	dm.Update(ScanFinished{})
	return dm
}

func TestDiffModeBinaryZeroChurnStaysVisible(t *testing.T) {
	dm := newTestBinaryDiffDirModel(t)

	// The changed-only view keeps the directory holding the binary change.
	found := false
	for _, name := range rowNames(dm) {
		if name == "assets" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected assets dir in changed-only view, got %v", rowNames(dm))
	}

	// The binary file counts towards "N files changed" despite 0/0 churn.
	if got := dm.changedFileCount(dm.nav.Entry()); got != 1 {
		t.Errorf("expected 1 changed file, got %d", got)
	}
	if summary := dm.changedSummary(); summary != "1 files" {
		t.Errorf("expected %q, got %q", "1 files", summary)
	}

	// The file row itself is visible inside the directory.
	dm.nav.Down("assets", 0, 0)
	dm.updateTableData()
	names := rowNames(dm)
	found = false
	for _, name := range names {
		if name == "logo.png" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected logo.png in changed-only view, got %v", names)
	}
}

func TestDiffModeEmptyDiffPercentColumn(t *testing.T) {
	s2 := provider.Result{Files: []provider.FileStats{
		{Path: "/repo/a.go", Language: "Go", Code: 10},
	}}
	tree := structure.NewTree(nil)
	if err := tree.BuildFromDiff(nil, s2, "/repo"); err != nil {
		t.Fatalf("BuildFromDiff: %v", err)
	}
	info := provider.Info{Name: "tokei", Version: "12.1", Capabilities: provider.CapLines | provider.CapChurn}
	dm := NewDirModelWithMode(NewCodeNavigation(tree), info, ModeInfo{Kind: ModeDiff, Range: "HEAD"}, false, false)
	dm.width = 140
	dm.Update(ScanFinished{})

	// Show all rows; with zero churn everywhere the % column renders 0.0 %.
	dm.filters.ToggleFilter(filter.ChangedFilterID)
	dm.updateTableData()

	for i, te := range dm.tableEntries {
		if te.entry.Name() == "a.go" {
			if got := dm.dirsTable.Rows()[i][7]; got != "0.0 %" {
				t.Errorf("expected %% cell %q, got %q", "0.0 %", got)
			}
			return
		}
	}
	t.Fatal("expected a row for a.go")
}

func TestDiffModeToggleChangedPreservesSelection(t *testing.T) {
	dm := newTestDiffDirModel(t)

	for i, te := range dm.tableEntries {
		if te.entry.Name() == "a.go" {
			dm.dirsTable.SetCursor(i)
			break
		}
	}
	before := dm.SelectedEntry()
	if before == nil || before.Name() != "a.go" {
		t.Fatalf("setup: expected a.go selected, got %v", before)
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if after := dm.SelectedEntry(); after != before {
		t.Errorf("expected selection to stay on a.go after 'a', got %v", after)
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if after := dm.SelectedEntry(); after != before {
		t.Errorf("expected selection to stay on a.go after second 'a', got %v", after)
	}
}

func TestDiffModeToggleChangedPreservesExpansion(t *testing.T) {
	dm := newTestDiffDirModel(t)
	dm.ToggleTreeMode()

	var sub *structure.Entry
	for _, te := range dm.tableEntries {
		if te.entry.Name() == "sub" {
			sub = te.entry
		}
	}
	if sub == nil {
		t.Fatal("expected sub in tree view")
	}
	sub.Expanded = true
	dm.updateTableData()

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !sub.Expanded {
		t.Error("expected expansion state to survive toggling changed-only")
	}
	// The expanded child stays visible in show-all mode.
	found := false
	for _, name := range rowNames(dm) {
		if name == "c.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected expanded child c.go in show-all tree view, got %v", rowNames(dm))
	}
}
