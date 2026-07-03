package render

import (
	"testing"

	"github.com/zdyxry/tokui/structure"
)

func TestVisibleColumns_Wide(t *testing.T) {
	dm := NewDirModel(NewCodeNavigation(structure.NewTree(structure.NewDirEntry("root"))), sccProviderInfo(), false, false)
	dm.width = 120
	dm.sortState = SortState{Key: SortByTotal, Desc: true}

	cols := dm.visibleColumns()
	if len(cols) != len(dm.columns) {
		t.Errorf("expected all %d columns at width 120, got %d", len(dm.columns), len(cols))
	}
}

func TestVisibleColumns_NarrowHidesOptional(t *testing.T) {
	dm := NewDirModel(NewCodeNavigation(structure.NewTree(structure.NewDirEntry("root"))), sccProviderInfo(), false, false)
	dm.width = 50
	dm.sortState = SortState{Key: SortByTotal, Desc: true}

	cols := dm.visibleColumns()
	for _, c := range cols {
		switch c.SortKey {
		case SortByComplexity, SortByLanguages, SortByComments, SortByBlanks:
			t.Errorf("expected %q to be hidden at width 50, but it was visible", c.SortKey)
		}
	}
}

func TestVisibleColumns_ActiveSortShownDespiteNarrow(t *testing.T) {
	dm := NewDirModel(NewCodeNavigation(structure.NewTree(structure.NewDirEntry("root"))), sccProviderInfo(), false, false)
	dm.width = 50
	dm.sortState = SortState{Key: SortByComplexity, Desc: true}

	cols := dm.visibleColumns()
	found := false
	for _, c := range cols {
		if c.SortKey == SortByComplexity {
			found = true
		}
	}
	if !found {
		t.Errorf("expected active sort column %q to remain visible at width 50", SortByComplexity)
	}
}

func TestBuildRow_WithComplexity(t *testing.T) {
	root := structure.NewDirEntry("root")
	root.AddChild(structure.NewFileEntry("root/a.go", map[string]structure.CodeStats{
		"Go": {Code: 20, Comments: 5, Blanks: 5, Complexity: 7},
	}))
	root.AggregateStats()

	dm := NewDirModel(NewCodeNavigation(structure.NewTree(root)), sccProviderInfo(), false, false)
	entry := root.Child[0]
	stats := entry.TotalStats

	row := dm.buildRow(dm.columns, entry, "a.go", "Go", stats, 100.0)
	if len(row) != len(dm.columns) {
		t.Fatalf("expected row length %d, got %d", len(dm.columns), len(row))
	}

	want := map[SortKey]string{
		SortByName:       "a.go",
		SortByLanguages:  "Go",
		SortByCode:       "20",
		SortByComments:   "5",
		SortByBlanks:     "5",
		SortByTotal:      "30",
		SortByPercent:    "100.00 %",
		SortByComplexity: "7",
	}
	for i, c := range dm.columns {
		if wantVal, ok := want[c.SortKey]; ok && row[i] != wantVal {
			t.Errorf("column %q: expected %q, got %q", c.SortKey, wantVal, row[i])
		}
	}
}

func TestPercentColumn_ContextAware(t *testing.T) {
	root := structure.NewDirEntry("root")
	root.AddChild(structure.NewFileEntry("root/a.go", map[string]structure.CodeStats{
		"Go": {Code: 10, Comments: 5, Blanks: 5, Complexity: 5},
	}))
	root.AddChild(structure.NewFileEntry("root/b.go", map[string]structure.CodeStats{
		"Go": {Code: 30, Comments: 10, Blanks: 10, Complexity: 15},
	}))
	root.AggregateStats()

	dm := NewDirModel(NewCodeNavigation(structure.NewTree(root)), sccProviderInfo(), false, false)
	dm.width = 120
	dm.height = 30

	tests := []struct {
		key  SortKey
		row0 string
		row1 string
	}{
		{SortByTotal, "71.43 %", "28.57 %"},
		{SortByComplexity, "75.00 %", "25.00 %"},
	}

	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			dm.sortState = SortState{Key: tt.key, Desc: true}
			dm.updateTableData()
			rows := dm.dirsTable.Rows()
			if len(rows) < 2 {
				t.Fatalf("expected 2 rows, got %d", len(rows))
			}
			// Find the percent column index.
			percentIdx := -1
			for i, c := range dm.columns {
				if c.SortKey == SortByPercent {
					percentIdx = i
					break
				}
			}
			if percentIdx < 0 {
				t.Fatal("percent column not found")
			}
			if got := rows[0][percentIdx]; got != tt.row0 {
				t.Errorf("row0 percent: expected %s, got %s", tt.row0, got)
			}
			if got := rows[1][percentIdx]; got != tt.row1 {
				t.Errorf("row1 percent: expected %s, got %s", tt.row1, got)
			}
		})
	}
}
