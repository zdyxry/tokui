package structure

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zdyxry/tokui/gitx"
	"github.com/zdyxry/tokui/provider"
)

// diffFixture returns a change set and the matching S2-side provider result.
// unchanged.go exists in S2 but not in the change set: it stays in the tree
// with a zero Change so the "show all" toggle can display it. old/legacy.rb
// is deleted and therefore absent from S2.
func diffFixture() (changes []gitx.FileChange, s2 provider.Result, repoRoot string) {
	changes = []gitx.FileChange{
		{Path: "src/main.go", Added: 120, Deleted: 45, Kind: gitx.Modified},
		{Path: "src/util/helper.go", Added: 80, Deleted: 10, Kind: gitx.Modified},
		{Path: "README.md", Added: 15, Deleted: 3, Kind: gitx.Modified},
		{Path: "new.py", Added: 200, Deleted: 0, Kind: gitx.Added},
		{Path: "old/legacy.rb", Added: 0, Deleted: 150, Kind: gitx.Deleted},
	}
	s2 = provider.Result{Files: []provider.FileStats{
		{Path: "/repo/src/main.go", Language: "Go", Code: 1000, Comments: 100, Blanks: 50, Complexity: 10},
		{Path: "/repo/src/util/helper.go", Language: "Go", Code: 300, Comments: 30, Blanks: 20, Complexity: 4},
		{Path: "/repo/README.md", Language: "Markdown", Code: 40, Comments: 0, Blanks: 10},
		{Path: "/repo/new.py", Language: "Python", Code: 200, Comments: 10, Blanks: 10, Complexity: 3},
		{Path: "/repo/unchanged.go", Language: "Go", Code: 999, Comments: 9, Blanks: 9},
	}}
	return changes, s2, "/repo"
}

func buildDiffTree(t *testing.T) *Tree {
	t.Helper()
	changes, s2, repoRoot := diffFixture()
	tree := NewTree(nil)
	if err := tree.BuildFromDiff(changes, s2, repoRoot); err != nil {
		t.Fatalf("BuildFromDiff failed: %v", err)
	}
	return tree
}

func TestBuildFromDiff_TreeStructure(t *testing.T) {
	tree := buildDiffTree(t)
	root := tree.Root()
	require.NotNil(t, root, "expected root entry")

	// The full S2 snapshot enters the tree; unchanged files carry a zero Change.
	unchanged := root.GetChild("unchanged.go")
	require.NotNil(t, unchanged, "unchanged.go must stay in the tree for the show-all toggle")
	if unchanged.Change != (Change{}) {
		t.Errorf("expected zero change for unchanged.go, got %+v", unchanged.Change)
	}

	mainGo := root.GetChild("src").GetChild("main.go")
	require.NotNil(t, mainGo, "expected src/main.go")
	if mainGo.TotalStats.Code != 1000 {
		t.Errorf("expected main.go code 1000 from S2, got %d", mainGo.TotalStats.Code)
	}
	if mainGo.StatsByLang["Go"].Complexity != 10 {
		t.Errorf("expected main.go complexity 10, got %d", mainGo.StatsByLang["Go"].Complexity)
	}

	helper := root.GetChild("src").GetChild("util").GetChild("helper.go")
	require.NotNil(t, helper, "expected src/util/helper.go")

	// Directory aggregation comes from the S2 stats of all files, changed or not.
	src := root.GetChild("src")
	require.NotNil(t, src, "expected src dir")
	wantSrc := CodeStats{Code: 1300, Comments: 130, Blanks: 70, Complexity: 14, MaxComplexity: 10}
	if src.TotalStats != wantSrc {
		t.Errorf("src total stats mismatch: got %+v, want %+v", src.TotalStats, wantSrc)
	}

	// Root sums the full S2 snapshot plus the zeroed deleted file.
	wantRoot := CodeStats{Code: 2539, Comments: 149, Blanks: 99, Complexity: 17, MaxComplexity: 10}
	if root.TotalStats != wantRoot {
		t.Errorf("root total stats mismatch: got %+v, want %+v", root.TotalStats, wantRoot)
	}
}

func TestBuildFromDiff_DeletedFile(t *testing.T) {
	tree := buildDiffTree(t)
	root := tree.Root()

	legacy := root.GetChild("old").GetChild("legacy.rb")
	require.NotNil(t, legacy, "expected old/legacy.rb")

	if legacy.Change.Kind != gitx.Deleted {
		t.Errorf("expected kind Deleted, got %v", legacy.Change.Kind)
	}
	if legacy.TotalStats != (CodeStats{}) {
		t.Errorf("expected zeroed stats for deleted file, got %+v", legacy.TotalStats)
	}
	langs := legacy.Languages()
	if len(langs) != 1 || langs[0] != "Ruby" {
		t.Errorf("expected language [Ruby] from extension, got %v", langs)
	}
	if legacy.Change.Deleted != 150 {
		t.Errorf("expected deleted churn 150, got %d", legacy.Change.Deleted)
	}
}

func TestBuildFromDiff_ChangeAggregation(t *testing.T) {
	tree := buildDiffTree(t)
	root := tree.Root()

	wantRoot := Change{Added: 415, Deleted: 208, Present: true}
	if root.Change != wantRoot {
		t.Errorf("root change mismatch: got %+v, want %+v", root.Change, wantRoot)
	}
	if root.Change.Delta() != 207 {
		t.Errorf("expected root delta 207, got %d", root.Change.Delta())
	}

	src := root.GetChild("src")
	require.NotNil(t, src, "expected src dir")
	wantSrc := Change{Added: 200, Deleted: 55, Present: true}
	if src.Change != wantSrc {
		t.Errorf("src change mismatch: got %+v, want %+v", src.Change, wantSrc)
	}

	util := src.GetChild("util")
	require.NotNil(t, util, "expected src/util dir")
	wantUtil := Change{Added: 80, Deleted: 10, Present: true}
	if util.Change != wantUtil {
		t.Errorf("util change mismatch: got %+v, want %+v", util.Change, wantUtil)
	}

	mainGo := src.GetChild("main.go")
	require.NotNil(t, mainGo, "expected src/main.go")
	if mainGo.Change.Kind != gitx.Modified {
		t.Errorf("expected kind Modified, got %v", mainGo.Change.Kind)
	}
	if mainGo.Change.Delta() != 75 {
		t.Errorf("expected main.go delta 75, got %d", mainGo.Change.Delta())
	}
}

func TestBuildFromDiff_ChangeByLang(t *testing.T) {
	tree := buildDiffTree(t)
	root := tree.Root()

	wantGo := Change{Added: 200, Deleted: 55, Present: true}
	if got := root.GetChange("Go"); got != wantGo {
		t.Errorf("root Go change mismatch: got %+v, want %+v", got, wantGo)
	}
	wantPython := Change{Added: 200, Deleted: 0, Present: true}
	if got := root.GetChange("Python"); got != wantPython {
		t.Errorf("root Python change mismatch: got %+v, want %+v", got, wantPython)
	}
	wantRuby := Change{Added: 0, Deleted: 150, Present: true}
	if got := root.GetChange("Ruby"); got != wantRuby {
		t.Errorf("root Ruby change mismatch: got %+v, want %+v", got, wantRuby)
	}
	if got := root.GetChange(""); got != root.Change {
		t.Errorf("empty filter should return total change, got %+v", got)
	}
	if got := root.GetChange("Missing"); got != (Change{}) {
		t.Errorf("missing language should return zero change, got %+v", got)
	}

	// Language filter on a file entry returns the file's own change when the
	// language matches.
	src := root.GetChild("src")
	mainGo := src.GetChild("main.go")
	if got := mainGo.GetChange("Go"); got != mainGo.Change {
		t.Errorf("file change with matching filter mismatch: got %+v, want %+v", got, mainGo.Change)
	}
	if got := mainGo.GetChange("Python"); got != (Change{}) {
		t.Errorf("file change with non-matching filter should be zero, got %+v", got)
	}

	// Directory-level per-language change.
	wantUtilGo := Change{Added: 80, Deleted: 10, Present: true}
	if got := src.GetChild("util").GetChange("Go"); got != wantUtilGo {
		t.Errorf("util Go change mismatch: got %+v, want %+v", got, wantUtilGo)
	}
}

func TestChangeDelta(t *testing.T) {
	c := Change{Added: 10, Deleted: 25}
	if c.Delta() != -15 {
		t.Errorf("expected delta -15, got %d", c.Delta())
	}

	c.Add(Change{Added: 5, Deleted: 5, PrevCode: 100, PrevComplexity: 3, Present: true})
	want := Change{Added: 15, Deleted: 30, PrevCode: 100, PrevComplexity: 3, Present: true}
	if c != want {
		t.Errorf("Add mismatch: got %+v, want %+v", c, want)
	}

	// Present is sticky once set.
	c.Add(Change{})
	if !c.Present {
		t.Errorf("Present must survive adding a zero change, got %+v", c)
	}
}

func TestBuildFromDiff_BinaryChangePresent(t *testing.T) {
	changes := []gitx.FileChange{
		{Path: "assets/logo.png", Added: 0, Deleted: 0, Kind: gitx.Modified},
	}
	s2 := provider.Result{Files: []provider.FileStats{
		{Path: "/repo/assets/logo.png", Language: "Binary", Code: 0},
	}}
	tree := NewTree(nil)
	if err := tree.BuildFromDiff(changes, s2, "/repo"); err != nil {
		t.Fatalf("BuildFromDiff failed: %v", err)
	}
	root := tree.Root()

	logo := root.GetChild("assets").GetChild("logo.png")
	require.NotNil(t, logo, "expected assets/logo.png")
	if !logo.Change.Present {
		t.Errorf("binary change must set Present, got %+v", logo.Change)
	}
	if logo.Change.Added != 0 || logo.Change.Deleted != 0 {
		t.Errorf("binary change must keep zero churn, got %+v", logo.Change)
	}
	// The zero-churn change must aggregate so the directory subtree survives
	// the changed-only filter.
	if assets := root.GetChild("assets"); !assets.Change.Present {
		t.Errorf("assets dir must aggregate Present, got %+v", assets.Change)
	}
	if !root.Change.Present {
		t.Errorf("root must aggregate Present, got %+v", root.Change)
	}
	if root.Change != (Change{Present: true}) {
		t.Errorf("root change mismatch: got %+v, want %+v", root.Change, Change{Present: true})
	}
}

func TestBuildFromDiff_RenameOldPath(t *testing.T) {
	changes := []gitx.FileChange{
		{Path: "new.go", OldPath: "old.go", Added: 0, Deleted: 0, Kind: gitx.Renamed},
	}
	s2 := provider.Result{Files: []provider.FileStats{
		{Path: "/repo/new.go", Language: "Go", Code: 100, Complexity: 2},
	}}
	tree := NewTree(nil)
	if err := tree.BuildFromDiff(changes, s2, "/repo"); err != nil {
		t.Fatalf("BuildFromDiff failed: %v", err)
	}

	entry := tree.Root().GetChild("new.go")
	require.NotNil(t, entry, "expected new.go")
	if entry.Change.Kind != gitx.Renamed {
		t.Errorf("expected kind Renamed, got %v", entry.Change.Kind)
	}
	if entry.Change.OldPath != "old.go" {
		t.Errorf("OldPath = %q, want %q", entry.Change.OldPath, "old.go")
	}
	if !entry.Change.Present {
		t.Errorf("pure rename (zero churn) must set Present, got %+v", entry.Change)
	}
}

func TestBuildFromDiff_MissingFromS2NotDeleted(t *testing.T) {
	// The provider skipped a modified file (e.g. its own ignore rules): the
	// entry is still added with its original Kind and churn, zeroed stats.
	changes := []gitx.FileChange{
		{Path: "vendor/dep.go", Added: 12, Deleted: 3, Kind: gitx.Modified},
	}
	s2 := provider.Result{Files: []provider.FileStats{
		{Path: "/repo/main.go", Language: "Go", Code: 10},
	}}
	tree := NewTree(nil)
	if err := tree.BuildFromDiff(changes, s2, "/repo"); err != nil {
		t.Fatalf("BuildFromDiff failed: %v", err)
	}

	dep := tree.Root().GetChild("vendor").GetChild("dep.go")
	require.NotNil(t, dep, "expected vendor/dep.go")
	if dep.Change.Kind != gitx.Modified {
		t.Errorf("S2-missing non-deleted file must keep kind Modified, got %v", dep.Change.Kind)
	}
	if dep.Change.Added != 12 || dep.Change.Deleted != 3 {
		t.Errorf("S2-missing non-deleted file must keep churn, got %+v", dep.Change)
	}
	if !dep.Change.Present {
		t.Errorf("S2-missing non-deleted file must set Present, got %+v", dep.Change)
	}
	if dep.TotalStats != (CodeStats{}) {
		t.Errorf("expected zeroed stats, got %+v", dep.TotalStats)
	}
}

func TestLanguageByExt(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "Go"},
		{"dir/script.PY", "Python"},
		{"a/b/component.tsx", "TypeScript"},
		{"config.yaml", "YAML"},
		{"Makefile", "Other"},
		{"noext", "Other"},
	}

	for _, tt := range tests {
		if got := languageByExt(tt.path); got != tt.want {
			t.Errorf("languageByExt(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
