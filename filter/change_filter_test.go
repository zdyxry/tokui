package filter

import (
	"testing"

	"github.com/zdyxry/tokui/gitx"
	"github.com/zdyxry/tokui/structure"
)

// buildChangeTree returns a directory entry aggregating the given file
// entries, mimicking what BuildFromDiff produces.
func buildChangeTree(files ...*structure.Entry) *structure.Entry {
	dir := structure.NewDirEntry("/repo")
	for _, f := range files {
		dir.AddChild(f)
	}
	dir.AggregateStats()
	return dir
}

func TestChangedFilterZeroChurnChanges(t *testing.T) {
	// A binary file and a pure rename both have zero Added/Deleted churn but
	// are part of the change set (Present). They — and the directory that
	// aggregates them — must survive the changed-only filter.
	binary := structure.NewFileEntry("/repo/blob.bin", map[string]structure.CodeStats{"Other": {}})
	binary.Change = structure.Change{Kind: gitx.Modified, Present: true}

	rename := structure.NewFileEntry("/repo/new.go", map[string]structure.CodeStats{"Go": {Code: 10}})
	rename.Change = structure.Change{Kind: gitx.Renamed, OldPath: "old.go", Present: true}

	unchanged := structure.NewFileEntry("/repo/unchanged.go", map[string]structure.CodeStats{"Go": {Code: 5}})

	dir := buildChangeTree(binary, rename, unchanged)

	cf := NewChangedFilter(true)
	for _, e := range []*structure.Entry{binary, rename, dir} {
		if !cf.Filter(e) {
			t.Errorf("expected %s to pass the changed-only filter (change %+v)", e.Path, e.Change)
		}
	}
	if cf.Filter(unchanged) {
		t.Errorf("expected unchanged file to be hidden, change %+v", unchanged.Change)
	}

	// Disabled filter passes everything.
	cf.Toggle()
	if !cf.Filter(unchanged) {
		t.Error("disabled filter must pass unchanged entries")
	}
}

func TestChangedFilterZeroChangeHidden(t *testing.T) {
	e := structure.NewFileEntry("/repo/a.go", map[string]structure.CodeStats{"Go": {Code: 1}})
	cf := NewChangedFilter(true)
	if cf.Filter(e) {
		t.Errorf("entry with zero Change must be hidden, change %+v", e.Change)
	}
}
