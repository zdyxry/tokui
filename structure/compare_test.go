package structure

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zdyxry/tokui/gitx"
	"github.com/zdyxry/tokui/provider"
)

// compareFixture returns S1/S2 results covering: a modified file, an added
// file (S2 only), a deleted file (S1 only) and an unchanged file.
func compareFixture() (s1, s2 provider.Result, root string) {
	s1 = provider.Result{Files: []provider.FileStats{
		{Path: "/repo/src/main.go", Language: "Go", Code: 900, Comments: 90, Blanks: 40, Complexity: 8},
		{Path: "/repo/old/legacy.rb", Language: "Ruby", Code: 700, Comments: 10, Blanks: 20, Complexity: 12},
		{Path: "/repo/unchanged.go", Language: "Go", Code: 500, Comments: 5, Blanks: 5, Complexity: 2},
	}}
	s2 = provider.Result{Files: []provider.FileStats{
		{Path: "/repo/src/main.go", Language: "Go", Code: 1000, Comments: 100, Blanks: 50, Complexity: 10},
		{Path: "/repo/new.py", Language: "Python", Code: 480, Comments: 20, Blanks: 10, Complexity: 5},
		{Path: "/repo/unchanged.go", Language: "Go", Code: 500, Comments: 5, Blanks: 5, Complexity: 2},
	}}
	return s1, s2, "/repo"
}

func buildCompareTree(t *testing.T) *Tree {
	t.Helper()
	s1, s2, root := compareFixture()
	tree := NewTree(nil)
	if err := tree.BuildFromCompare(s1, s2, root); err != nil {
		t.Fatalf("BuildFromCompare failed: %v", err)
	}
	return tree
}

func TestBuildFromCompare_TreeStructure(t *testing.T) {
	tree := buildCompareTree(t)
	root := tree.Root()
	require.NotNil(t, root, "expected root entry")

	// Union: modified, added, deleted and unchanged files are all present.
	mainGo := root.GetChild("src").GetChild("main.go")
	require.NotNil(t, mainGo, "expected src/main.go")
	if mainGo.TotalStats.Code != 1000 {
		t.Errorf("expected main.go S2 code 1000, got %d", mainGo.TotalStats.Code)
	}
	if mainGo.Change.PrevCode != 900 || mainGo.Change.PrevComplexity != 8 {
		t.Errorf("main.go prev mismatch: got %+v", mainGo.Change)
	}

	newPy := root.GetChild("new.py")
	require.NotNil(t, newPy, "expected new.py")
	if newPy.Change.PrevCode != 0 || newPy.Change.PrevComplexity != 0 {
		t.Errorf("added file must have zero prev values, got %+v", newPy.Change)
	}

	legacy := root.GetChild("old").GetChild("legacy.rb")
	require.NotNil(t, legacy, "expected old/legacy.rb")
	if legacy.Change.Kind != gitx.Deleted {
		t.Errorf("expected kind Deleted, got %v", legacy.Change.Kind)
	}
	if legacy.TotalStats != (CodeStats{}) {
		t.Errorf("expected zeroed S2 stats for deleted file, got %+v", legacy.TotalStats)
	}
	if legacy.Change.PrevCode != 700 || legacy.Change.PrevComplexity != 12 {
		t.Errorf("deleted file prev mismatch: got %+v", legacy.Change)
	}
	if langs := legacy.Languages(); len(langs) != 1 || langs[0] != "Ruby" {
		t.Errorf("expected language [Ruby] from S1, got %v", langs)
	}

	unchanged := root.GetChild("unchanged.go")
	require.NotNil(t, unchanged, "expected unchanged.go")
	if unchanged.Change.PrevCode != 500 || unchanged.TotalStats.Code != 500 {
		t.Errorf("unchanged file mismatch: stats %+v change %+v", unchanged.TotalStats, unchanged.Change)
	}
}

func TestBuildFromCompare_Aggregation(t *testing.T) {
	tree := buildCompareTree(t)
	root := tree.Root()

	// Prev* aggregate up the tree alongside CodeStats; Present is set because
	// the tree contains files from S1.
	wantPrev := Change{PrevCode: 2100, PrevComplexity: 22, Present: true}
	if root.Change != wantPrev {
		t.Errorf("root change mismatch: got %+v, want %+v", root.Change, wantPrev)
	}

	// ΔCode = 1980 - 2100 = -120, ΔCmplx = 17 - 22 = -5.
	if got := root.TotalStats.Code - root.Change.PrevCode; got != -120 {
		t.Errorf("expected root ΔCode -120, got %d", got)
	}
	if got := root.TotalStats.Complexity - root.Change.PrevComplexity; got != -5 {
		t.Errorf("expected root ΔCmplx -5, got %d", got)
	}

	src := root.GetChild("src")
	require.NotNil(t, src, "expected src dir")
	if src.Change.PrevCode != 900 || src.TotalStats.Code != 1000 {
		t.Errorf("src mismatch: stats %+v change %+v", src.TotalStats, src.Change)
	}
}

func TestBuildFromCompare_ChangeByLang(t *testing.T) {
	tree := buildCompareTree(t)
	root := tree.Root()

	// Per-language prev values follow the same filtering path as stats.
	if got := root.GetChange("Go").PrevCode; got != 1400 {
		t.Errorf("expected Go prev code 1400, got %d", got)
	}
	if got := root.GetChange("Ruby").PrevCode; got != 700 {
		t.Errorf("expected Ruby prev code 700 (deleted file), got %d", got)
	}
	if got := root.GetChange("Python").PrevCode; got != 0 {
		t.Errorf("expected Python prev code 0 (added file), got %d", got)
	}
	if got := root.GetChange("Missing"); got != (Change{}) {
		t.Errorf("missing language should return zero change, got %+v", got)
	}

	// Language-filtered Δ mirrors the stats side.
	goDelta := root.GetStats("Go").Code - root.GetChange("Go").PrevCode
	if goDelta != 100 {
		t.Errorf("expected Go ΔCode +100, got %d", goDelta)
	}
}
