package render

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

	return &DirModel{
		nav:           &Navigation{entry: root},
		languages:     []string{"Go", "Python"},
		langFilterIdx: -1,
		selectedLangs: make(map[string]bool),
		sortState:     SortState{Key: SortByTotal, Desc: true},
	}
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
