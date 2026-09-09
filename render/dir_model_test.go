package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zdyxry/tokui/filter"
	"github.com/zdyxry/tokui/provider"
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

	dm := NewDirModel(NewCodeNavigation(structure.NewTree(root)), provider.Info{Name: "test"}, false, false)
	dm.languages = []string{"Go", "Python"}
	dm.langFilterIdx = -1
	dm.selectedLangs = make(map[string]bool)
	dm.sortState = SortState{Key: SortByTotal, Desc: true}
	return dm
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

	dm := NewDirModel(NewCodeNavigation(structure.NewTree(root)), provider.Info{Name: "test"}, false, false)
	dm.languages = []string{"Go", "Python"}
	dm.langFilterIdx = -1
	dm.selectedLangs = make(map[string]bool)
	dm.sortState = SortState{Key: SortByTotal, Desc: true}
	return dm
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
	dm := NewDirModel(NewCodeNavigation(structure.NewTree(testDM.nav.Entry())), provider.Info{Name: "test"}, false, false)
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

func TestDirModelInputMode(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if dm.mode != INPUT {
		t.Fatalf("expected INPUT mode, got %v", dm.mode)
	}
	nf := dm.filters[filter.NameFilterID].(*filter.NameFilter)
	if !nf.IsEnabled() {
		t.Error("expected name filter to be enabled")
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if len(dm.dirsTable.Rows()) != 2 {
		t.Errorf("expected 2 rows matching 'o', got %d", len(dm.dirsTable.Rows()))
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if dm.mode != READY {
		t.Errorf("expected READY after escape, got %v", dm.mode)
	}
	if nf.IsEnabled() {
		t.Error("expected name filter to be disabled")
	}
	if len(dm.dirsTable.Rows()) != 3 {
		t.Errorf("expected 3 rows after clearing filter, got %d", len(dm.dirsTable.Rows()))
	}
}

func TestDirModelInputModeSlashTyped(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if dm.mode != INPUT {
		t.Fatalf("expected INPUT mode, got %v", dm.mode)
	}
	nf := dm.filters[filter.NameFilterID].(*filter.NameFilter)

	// Typing '/' again while already in INPUT mode should enter it as a filter character.
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !strings.Contains(nf.View(), "/") {
		t.Errorf("expected filter view to contain '/', got %q", nf.View())
	}
	if dm.mode != INPUT {
		t.Errorf("expected to stay in INPUT mode, got %v", dm.mode)
	}
}

func TestDirModelExitSearchMode(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if len(dm.dirsTable.Rows()) == 3 {
		t.Fatal("expected filter to reduce rows")
	}

	dm.ExitSearchMode()
	if dm.mode != READY {
		t.Errorf("expected READY, got %v", dm.mode)
	}
	nf := dm.filters[filter.NameFilterID].(*filter.NameFilter)
	if nf.IsEnabled() {
		t.Error("expected name filter to be disabled")
	}
	if len(dm.dirsTable.Rows()) != 3 {
		t.Errorf("expected 3 rows, got %d", len(dm.dirsTable.Rows()))
	}
}

func TestDirModelChartOverlay(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.width = 80
	dm.height = 24

	if dm.showCart {
		t.Fatal("expected chart to start hidden")
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	if !dm.showCart {
		t.Error("expected chart to be shown")
	}

	chart := dm.viewChart()
	if chart == "" {
		t.Error("expected non-empty chart view")
	}
	if !strings.Contains(chart, "Go") && !strings.Contains(chart, "Python") {
		t.Error("expected chart to contain language labels")
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	if dm.showCart {
		t.Error("expected chart to be hidden after toggle")
	}
}

func TestDirModelPreviewLifecycle(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})
	dm.width = 80
	dm.height = 24

	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(p, []byte("hello"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if dm.IsInPreviewMode() {
		t.Error("expected preview mode to start false")
	}

	dm.ShowFilePreview(structure.NewFileEntry(p, nil))
	if !dm.IsInPreviewMode() {
		t.Error("expected preview mode to be true")
	}
	if dm.mode != PREVIEW {
		t.Errorf("expected PREVIEW mode, got %v", dm.mode)
	}
	if dm.filePreview == nil {
		t.Fatal("expected filePreview to be created")
	}

	dm.ClosePreview()
	if dm.IsInPreviewMode() {
		t.Error("expected preview mode to be false after close")
	}
	if dm.mode != READY {
		t.Errorf("expected READY mode, got %v", dm.mode)
	}
	if dm.filePreview != nil {
		t.Error("expected filePreview to be cleared")
	}
}

func TestDirModelSortKeyBindings(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})

	initial := dm.sortState.Key
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if dm.sortState.Key == initial {
		t.Error("expected 's' to cycle sort column")
	}

	dm.sortState = SortState{Key: SortByTotal, Desc: true}
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if dm.sortState.Desc != false {
		t.Errorf("expected 'S' to toggle sort order, got Desc=%v", dm.sortState.Desc)
	}
	if dm.sortState.Key != SortByTotal {
		t.Errorf("expected sort key to stay %q, got %q", SortByTotal, dm.sortState.Key)
	}
}

func TestDirModelModeToggles(t *testing.T) {
	dm := newTestDirModel()
	dm.Update(ScanFinished{})

	if dm.treeMode {
		t.Fatal("expected tree mode to start disabled")
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if !dm.treeMode {
		t.Error("expected tree mode enabled")
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if !dm.treemapMode {
		t.Error("expected treemap mode enabled")
	}
	if dm.treeMode {
		t.Error("expected tree mode disabled when treemap enabled")
	}

	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if !dm.treeMode {
		t.Error("expected tree mode re-enabled")
	}
	if dm.treemapMode {
		t.Error("expected treemap mode disabled")
	}
}

func TestNewDirModel_DynamicColumns(t *testing.T) {
	root := structure.NewDirEntry("root")
	root.AggregateStats()

	dm := NewDirModel(NewCodeNavigation(structure.NewTree(root)), sccProviderInfo(), false, false)

	var got []SortKey
	for _, c := range dm.columns {
		got = append(got, c.SortKey)
	}
	want := []SortKey{
		SortByNone, SortByNone, SortByName, SortByLanguages,
		SortByCode, SortByComments, SortByBlanks, SortByTotal,
		SortByPercent, SortByComplexity,
	}
	if len(got) != len(want) {
		t.Fatalf("expected columns %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("column %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestDirModelTreeExpandCollapseAll(t *testing.T) {
	dm := newTestNestedDirModel()
	dm.treeMode = true
	dm.Update(ScanFinished{})

	subdir := dm.nav.Entry().GetChild("subdir")
	if subdir == nil {
		t.Fatal("expected fixture to contain subdir")
	}

	// Initially collapsed: only the direct children are visible.
	if subdir.Expanded {
		t.Fatal("expected subdir to start collapsed")
	}
	if got := len(dm.dirsTable.Rows()); got != 2 {
		t.Fatalf("expected 2 rows before expand, got %d", got)
	}

	// "}" expands every directory under the current navigation directory.
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}})
	if !dm.nav.Entry().Expanded || !subdir.Expanded {
		t.Error("expected '}' to set Expanded=true on all dirs under nav")
	}
	if got := len(dm.dirsTable.Rows()); got != 3 {
		t.Fatalf("expected 3 rows after expand all, got %d", got)
	}

	// Move the cursor onto the last (deepest) row so collapse must clamp it.
	dm.dirsTable.SetCursor(len(dm.dirsTable.Rows()) - 1)

	// "{" collapses every directory under the current navigation directory.
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'{'}})
	if dm.nav.Entry().Expanded || subdir.Expanded {
		t.Error("expected '{' to set Expanded=false on all dirs under nav")
	}
	if got := len(dm.dirsTable.Rows()); got != 2 {
		t.Fatalf("expected 2 rows after collapse all, got %d", got)
	}
	if cursor := dm.dirsTable.Cursor(); cursor >= len(dm.dirsTable.Rows()) {
		t.Errorf("expected cursor to be clamped after collapse, got %d", cursor)
	}
}

func TestDirModelEditFileWorksInTreeMode(t *testing.T) {
	dm := newTestDirModel()
	dm.treeMode = true
	dm.Update(ScanFinished{})

	// In tree mode, "e" on a file still opens it in the editor.
	dm.dirsTable.SetCursor(0)
	if entry := dm.SelectedEntry(); entry == nil || entry.IsDir {
		t.Fatal("expected cursor to be on a file")
	}
	_, cmd := dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd == nil {
		t.Fatal("expected 'e' in tree mode to open the file in the editor")
	}
}

func TestDirModelExpandCollapseAllNoopOutsideTreeMode(t *testing.T) {
	dm := newTestNestedDirModel()
	dm.Update(ScanFinished{})

	// Outside tree mode, "}", "{", "]" and "[" do nothing.
	for _, r := range []rune{'}', '{', ']', '['} {
		_, cmd := dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if cmd != nil {
			t.Fatalf("expected %q outside tree mode to be a no-op", r)
		}
		if entry := dm.nav.Entry(); entry.Expanded {
			t.Fatalf("expected %q outside tree mode not to change Expanded", r)
		}
	}
}

// newTestTreeShiftDirModel builds a fixture where expanding all inserts rows
// above the cursor row: dirA (Total 20) sorts before z.go (Total 10).
func newTestTreeShiftDirModel() *DirModel {
	root := structure.NewDirEntry("root")

	dirA := structure.NewDirEntry("root/dirA")
	root.AddChild(dirA)
	dirA.AddChild(structure.NewFileEntry("root/dirA/inner.go", map[string]structure.CodeStats{
		"Go": {Code: 20},
	}))

	root.AddChild(structure.NewFileEntry("root/z.go", map[string]structure.CodeStats{
		"Go": {Code: 10},
	}))

	root.AggregateStats()

	dm := NewDirModel(NewCodeNavigation(structure.NewTree(root)), provider.Info{Name: "test"}, true, false)
	dm.languages = []string{"Go"}
	dm.langFilterIdx = -1
	dm.selectedLangs = make(map[string]bool)
	dm.sortState = SortState{Key: SortByTotal, Desc: true}
	return dm
}

func TestDirModelTreeExpandCollapseAllKeepsCursorEntry(t *testing.T) {
	dm := newTestTreeShiftDirModel()
	dm.Update(ScanFinished{})

	// Rows sorted by Total desc: dirA, z.go. Put the cursor on z.go.
	dm.dirsTable.SetCursor(1)
	zFile := dm.nav.Entry().GetChild("z.go")
	if got := dm.SelectedEntry(); got != zFile {
		t.Fatalf("expected cursor on z.go, got %v", got)
	}

	// Expanding all inserts dirA's child above z.go; the cursor must follow.
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}})
	if got := len(dm.dirsTable.Rows()); got != 3 {
		t.Fatalf("expected 3 rows after expand all, got %d", got)
	}
	if got := dm.SelectedEntry(); got != zFile {
		t.Errorf("expected cursor to stay on z.go after expand all, got %v", got)
	}

	// Collapsing all removes the row again; the cursor must follow back.
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'{'}})
	if got := len(dm.dirsTable.Rows()); got != 2 {
		t.Fatalf("expected 2 rows after collapse all, got %d", got)
	}
	if got := dm.SelectedEntry(); got != zFile {
		t.Errorf("expected cursor to stay on z.go after collapse all, got %v", got)
	}
}

func TestDirModelTreeExpandCollapseSubtree(t *testing.T) {
	root := structure.NewDirEntry("root")

	dirA := structure.NewDirEntry("root/dirA")
	root.AddChild(dirA)
	dirA.AddChild(structure.NewFileEntry("root/dirA/inner.go", map[string]structure.CodeStats{
		"Go": {Code: 20},
	}))

	dirB := structure.NewDirEntry("root/dirB")
	root.AddChild(dirB)
	dirB.AddChild(structure.NewFileEntry("root/dirB/other.go", map[string]structure.CodeStats{
		"Go": {Code: 10},
	}))

	root.AggregateStats()

	dm := NewDirModel(NewCodeNavigation(structure.NewTree(root)), provider.Info{Name: "test"}, true, false)
	dm.languages = []string{"Go"}
	dm.langFilterIdx = -1
	dm.selectedLangs = make(map[string]bool)
	dm.sortState = SortState{Key: SortByTotal, Desc: true}
	dm.Update(ScanFinished{})

	// Both dirs collapsed: rows dirA, dirB; the cursor starts on dirA.
	if got := len(dm.dirsTable.Rows()); got != 2 {
		t.Fatalf("expected 2 rows, got %d", got)
	}
	if got := dm.SelectedEntry(); got != dirA {
		t.Fatalf("expected cursor on dirA, got %v", got)
	}

	// "]" recursively expands only the subtree under the cursor.
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if !dirA.Expanded {
		t.Error("expected ']' to expand dirA")
	}
	if dirB.Expanded {
		t.Error("expected ']' to leave dirB collapsed")
	}
	if got := len(dm.dirsTable.Rows()); got != 3 {
		t.Fatalf("expected 3 rows after subtree expand, got %d", got)
	}
	if got := dm.SelectedEntry(); got != dirA {
		t.Errorf("expected cursor to stay on dirA after subtree expand, got %v", got)
	}

	// "]"/"[" on a file are no-ops.
	dm.dirsTable.SetCursor(1) // inner.go
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if got := len(dm.dirsTable.Rows()); got != 3 {
		t.Fatalf("expected subtree keys on a file to change nothing, got %d rows", got)
	}

	// "[" recursively collapses the subtree under the cursor; the cursor
	// directory itself stays visible because its parent chain is expanded.
	dm.dirsTable.SetCursor(0)
	dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if dirA.Expanded {
		t.Error("expected '[' to collapse dirA")
	}
	if got := len(dm.dirsTable.Rows()); got != 2 {
		t.Fatalf("expected 2 rows after subtree collapse, got %d", got)
	}
	if got := dm.SelectedEntry(); got != dirA {
		t.Errorf("expected cursor to stay on dirA after subtree collapse, got %v", got)
	}
}
