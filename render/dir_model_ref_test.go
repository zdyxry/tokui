package render

import (
	"strings"
	"testing"

	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/structure"
)

// newTestRefTree builds a plain snapshot tree for Ref/Full mode comparisons.
func newTestRefTree(t *testing.T) *structure.Tree {
	t.Helper()
	s2 := provider.Result{Files: []provider.FileStats{
		{Path: "/repo/a.go", Language: "Go", Code: 100, Comments: 10, Blanks: 5, Complexity: 3},
		{Path: "/repo/sub/b.py", Language: "Python", Code: 50, Comments: 5, Blanks: 5, Complexity: 2},
	}}
	tree := structure.NewTree(nil)
	if err := tree.BuildFromProviderResult(s2, "/repo"); err != nil {
		t.Fatalf("BuildFromProviderResult: %v", err)
	}
	return tree
}

func columnKeys(dm *DirModel) []SortKey {
	keys := make([]SortKey, 0, len(dm.columns))
	for _, c := range dm.columns {
		keys = append(keys, c.SortKey)
	}
	return keys
}

func TestRefModeColumnsMatchFullMode(t *testing.T) {
	info := provider.Info{
		Name:         "tokei",
		Version:      "12.1",
		Capabilities: provider.CapLines | provider.CapComplexity,
	}

	ref := NewDirModelWithMode(NewCodeNavigation(newTestRefTree(t)), info,
		ModeInfo{Kind: ModeRef, Range: "v1.0"}, false, false)
	full := NewDirModel(NewCodeNavigation(newTestRefTree(t)), info, false, false)

	got, want := columnKeys(ref), columnKeys(full)
	if len(got) != len(want) {
		t.Fatalf("expected Ref columns %v to match Full columns %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("column %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestRefModeDirsSummaryShowsRef(t *testing.T) {
	info := provider.Info{Name: "tokei", Version: "12.1", Capabilities: provider.CapLines}
	dm := NewDirModelWithMode(NewCodeNavigation(newTestRefTree(t)), info,
		ModeInfo{Kind: ModeRef, Range: "v1.0"}, false, false)
	dm.width = 140
	dm.Update(ScanFinished{})

	summary := dm.dirsSummary()
	for _, want := range []string{"REF", "v1.0"} {
		if !strings.Contains(summary, want) {
			t.Errorf("expected summary to contain %q, got %q", want, summary)
		}
	}
}
