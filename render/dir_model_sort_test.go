package render

import (
	"testing"

	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/structure"
)

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
	// Starting from SortByTotal, the cycle is Percent, Complexity,
	// Name, Languages, Code, Comments, Blanks, then wraps back to Total.
	order := []SortKey{SortByPercent, SortByComplexity, SortByName, SortByLanguages, SortByCode, SortByComments, SortByBlanks, SortByTotal}

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

func TestBuildChildComparator_Complexity(t *testing.T) {
	root := structure.NewDirEntry("root")
	root.AddChild(structure.NewFileEntry("root/a.go", map[string]structure.CodeStats{
		"Go": {Code: 10, Complexity: 5},
	}))
	root.AddChild(structure.NewFileEntry("root/b.go", map[string]structure.CodeStats{
		"Go": {Code: 10, Complexity: 3},
	}))
	root.AggregateStats()

	dm := NewDirModel(NewCodeNavigation(structure.NewTree(root)), sccProviderInfo(), false, false)

	dm.sortState = SortState{Key: SortByComplexity, Desc: true}
	cmp := dm.buildChildComparator()
	if cmp(root.Child[0], root.Child[1]) >= 0 {
		t.Error("expected a.go (complexity 5) to come before b.go (complexity 3)")
	}
}

func sccProviderInfo() provider.Info {
	return provider.Info{
		Name:         "scc",
		Capabilities: provider.CapLines | provider.CapComplexity,
	}
}
