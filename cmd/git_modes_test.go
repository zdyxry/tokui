package cmd

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zdyxry/tokui/gitx"
	"github.com/zdyxry/tokui/provider"
	"github.com/zdyxry/tokui/render"
	"github.com/zdyxry/tokui/structure"
)

// stubProvider analyzes a directory by counting newlines in each file, which
// is enough to exercise the git-mode flows without external binaries.
type stubProvider struct{}

func (stubProvider) Info() provider.Info {
	return provider.Info{Name: "stub", Capabilities: provider.CapLines}
}

func (stubProvider) Analyze(path string) (provider.Result, error) {
	var files []provider.FileStats
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files = append(files, provider.FileStats{
			Path:     p,
			Language: "Go",
			Code:     int64(bytes.Count(data, []byte("\n"))),
		})
		return nil
	})
	return provider.Result{Files: files}, err
}

func (stubProvider) ParseStdin([]byte) (provider.Result, error) {
	return provider.Result{}, nil
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.email=test@example.com", "-c", "user.name=Test User"}, args...)
	cmd := exec.Command("git", full...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeRepoFile(t *testing.T, repo, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// initCompareRepo creates a repo with tags v1 and v2: main.go grew from 2 to
// 5 lines, gone.go was deleted, new.go was added.
func initCompareRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	repo := t.TempDir()
	git(t, repo, "-c", "init.defaultBranch=main", "init")

	writeRepoFile(t, repo, "main.go", "a\nb\n")
	writeRepoFile(t, repo, "gone.go", "x\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "v1")
	git(t, repo, "tag", "v1")

	writeRepoFile(t, repo, "main.go", "a\nb\nc\nd\ne\n")
	writeRepoFile(t, repo, "new.go", "1\n2\n3\n")
	if err := os.Remove(filepath.Join(repo, "gone.go")); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "v2")
	git(t, repo, "tag", "v2")

	return repo
}

func TestRunCompareMode(t *testing.T) {
	repo := initCompareRepo(t)

	tree := structure.NewTree(nil)
	mode, err := runCompareMode(tree, stubProvider{}, repo, "v1..v2")
	if err != nil {
		t.Fatalf("runCompareMode: %v", err)
	}

	if mode.Kind != render.ModeCompare {
		t.Errorf("expected Compare mode, got %v", mode.Kind)
	}
	if mode.Range != "v1..v2" || mode.S1Ref != "v1" || mode.S2Ref != "v2" {
		t.Errorf("unexpected mode info: %+v", mode)
	}

	root := tree.Root()
	mainGo := root.GetChild("main.go")
	if mainGo == nil {
		t.Fatal("expected main.go in tree")
	}
	if mainGo.TotalStats.Code != 5 || mainGo.Change.PrevCode != 2 {
		t.Errorf("main.go mismatch: stats %+v change %+v", mainGo.TotalStats, mainGo.Change)
	}

	newGo := root.GetChild("new.go")
	if newGo == nil {
		t.Fatal("expected new.go in tree")
	}
	if newGo.Change.PrevCode != 0 {
		t.Errorf("added file must have zero prev code, got %+v", newGo.Change)
	}

	goneGo := root.GetChild("gone.go")
	if goneGo == nil {
		t.Fatal("expected gone.go in tree")
	}
	if goneGo.Change.Kind != gitx.Deleted || goneGo.Change.PrevCode != 1 {
		t.Errorf("deleted file mismatch: %+v", goneGo.Change)
	}
	if goneGo.TotalStats != (structure.CodeStats{}) {
		t.Errorf("expected zeroed S2 stats for deleted file, got %+v", goneGo.TotalStats)
	}
}

func TestRunCompareModeSingleRefRejected(t *testing.T) {
	repo := initCompareRepo(t)
	_, err := runCompareMode(structure.NewTree(nil), stubProvider{}, repo, "v1")
	if err == nil || !strings.Contains(err.Error(), "requires two refs") {
		t.Errorf("expected two-refs error, got %v", err)
	}
}

func TestRunCompareModeNotARepo(t *testing.T) {
	_, err := runCompareMode(structure.NewTree(nil), stubProvider{}, t.TempDir(), "v1..v2")
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected not-a-repo error, got %v", err)
	}
	if !strings.Contains(err.Error(), "remove --compare") {
		t.Errorf("expected full-mode hint, got %v", err)
	}
}

func TestRunRefMode(t *testing.T) {
	repo := initCompareRepo(t)

	tree := structure.NewTree(nil)
	mode, cleanup, err := runRefMode(tree, stubProvider{}, repo, "v1")
	if err != nil {
		t.Fatalf("runRefMode: %v", err)
	}
	defer cleanup()

	if mode.Kind != render.ModeRef || mode.Range != "v1" {
		t.Errorf("unexpected mode info: %+v", mode)
	}

	root := tree.Root()
	mainGo := root.GetChild("main.go")
	if mainGo == nil {
		t.Fatal("expected main.go in tree")
	}
	if mainGo.TotalStats.Code != 2 {
		t.Errorf("expected v1 snapshot code 2, got %d", mainGo.TotalStats.Code)
	}
	// gone.go exists in v1, new.go does not.
	if root.GetChild("gone.go") == nil {
		t.Error("expected gone.go in v1 snapshot")
	}
	if root.GetChild("new.go") != nil {
		t.Error("new.go must not be in the v1 snapshot")
	}

	// The archive stays readable until cleanup (previews read it).
	if _, err := os.Stat(mainGo.Path); err != nil {
		t.Errorf("expected archive file to exist before cleanup: %v", err)
	}
}

func TestRunRefModeBadRef(t *testing.T) {
	repo := initCompareRepo(t)
	_, _, err := runRefMode(structure.NewTree(nil), stubProvider{}, repo, "no-such-ref")
	if err == nil {
		t.Fatal("expected error for unknown ref")
	}
}

// gitRevParse returns the commit sha a ref resolves to in repo.
func gitRevParse(t *testing.T, repo, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

// initDiffRepo creates a repo where HEAD~1..HEAD touches both a top-level
// file (top.go, 1->2 lines) and a subdirectory file (sub/inner.go, 1->2
// lines).
func initDiffRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	repo := t.TempDir()
	git(t, repo, "-c", "init.defaultBranch=main", "init")

	writeRepoFile(t, repo, "top.go", "a\n")
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, repo, "sub/inner.go", "x\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "base")

	writeRepoFile(t, repo, "top.go", "a\nb\n")
	writeRepoFile(t, repo, "sub/inner.go", "x\ny\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "grow")

	return repo
}

func TestRunDiffMode(t *testing.T) {
	repo := initCompareRepo(t)

	tree := structure.NewTree(nil)
	mode, err := runDiffMode(tree, stubProvider{}, repo, "v1..v2")
	if err != nil {
		t.Fatalf("runDiffMode: %v", err)
	}

	if mode.Kind != render.ModeDiff {
		t.Errorf("expected Diff mode, got %v", mode.Kind)
	}
	if mode.Range != "v1..v2" || mode.S1Ref != "v1" || mode.S2Ref != "v2" {
		t.Errorf("unexpected mode info: %+v", mode)
	}
	if mode.RepoRoot == "" {
		t.Error("expected RepoRoot to be set for previews")
	}

	root := tree.Root()
	mainGo := root.GetChild("main.go")
	if mainGo == nil {
		t.Fatal("expected main.go in tree")
	}
	if mainGo.TotalStats.Code != 5 {
		t.Errorf("expected S2 code 5, got %d", mainGo.TotalStats.Code)
	}
	if mainGo.Change.Kind != gitx.Modified || mainGo.Change.Added != 3 || mainGo.Change.Deleted != 0 {
		t.Errorf("main.go churn mismatch: %+v", mainGo.Change)
	}

	goneGo := root.GetChild("gone.go")
	if goneGo == nil {
		t.Fatal("expected gone.go in tree")
	}
	if goneGo.Change.Kind != gitx.Deleted {
		t.Errorf("expected gone.go to be Deleted, got %+v", goneGo.Change)
	}
}

func TestRunDiffModeWorktree(t *testing.T) {
	repo := initCompareRepo(t)
	// An uncommitted worktree change on top of HEAD (v2).
	writeRepoFile(t, repo, "main.go", "a\nb\nc\nd\ne\nf\ng\n")

	tree := structure.NewTree(nil)
	mode, err := runDiffMode(tree, stubProvider{}, repo, "HEAD")
	if err != nil {
		t.Fatalf("runDiffMode: %v", err)
	}

	if mode.S1Ref != "HEAD" || mode.S2Ref != "" || mode.S2Label != "worktree" {
		t.Errorf("unexpected mode info: %+v", mode)
	}
	mainGo := tree.Root().GetChild("main.go")
	if mainGo == nil {
		t.Fatal("expected main.go in tree")
	}
	if mainGo.TotalStats.Code != 7 || mainGo.Change.Added != 2 {
		t.Errorf("expected worktree code 7 with +2 churn, got stats %+v change %+v",
			mainGo.TotalStats, mainGo.Change)
	}
}

func TestRunDiffModeThreeDotResolvesMergeBase(t *testing.T) {
	requireGit(t)
	repo := t.TempDir()
	git(t, repo, "-c", "init.defaultBranch=main", "init")

	writeRepoFile(t, repo, "main.go", "a\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "base")
	git(t, repo, "tag", "base")

	git(t, repo, "checkout", "-b", "feature")
	writeRepoFile(t, repo, "main.go", "a\nb\nc\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "feature work")

	git(t, repo, "checkout", "main")
	writeRepoFile(t, repo, "other.go", "o\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "main work")

	tree := structure.NewTree(nil)
	mode, err := runDiffMode(tree, stubProvider{}, repo, "main...feature")
	if err != nil {
		t.Fatalf("runDiffMode: %v", err)
	}

	// The S1 preview base must be the merge-base commit (what git diff
	// main...feature actually diffs against), not the main ref itself.
	want := gitRevParse(t, repo, "base")
	if mode.S1Ref != want {
		t.Errorf("S1Ref = %q, want merge-base %q", mode.S1Ref, want)
	}
	if mode.S1Label != "main" {
		t.Errorf("S1Label should stay the human-readable ref, got %q", mode.S1Label)
	}

	// The change set is base..feature: only main.go changed.
	root := tree.Root()
	if root.GetChild("other.go") != nil {
		t.Error("other.go only exists on main; it must not be in the diff tree")
	}
	mainGo := root.GetChild("main.go")
	if mainGo == nil {
		t.Fatal("expected main.go in tree")
	}
	if mainGo.TotalStats.Code != 3 || mainGo.Change.Added != 2 {
		t.Errorf("main.go mismatch: stats %+v change %+v", mainGo.TotalStats, mainGo.Change)
	}
}

func TestRunDiffModeNotARepo(t *testing.T) {
	_, err := runDiffMode(structure.NewTree(nil), stubProvider{}, t.TempDir(), "HEAD")
	if err == nil {
		t.Fatal("expected not-a-repo error")
	}
	if !errors.Is(err, gitx.ErrNotGitRepo) {
		t.Errorf("expected ErrNotGitRepo, got %v", err)
	}
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Errorf("expected CLIError, got %T", err)
	}
	if !strings.Contains(err.Error(), "remove --diff") {
		t.Errorf("expected full-mode hint, got %v", err)
	}
}

func TestRunDiffModeInvalidRange(t *testing.T) {
	repo := initCompareRepo(t)
	_, err := runDiffMode(structure.NewTree(nil), stubProvider{}, repo, "v1..no-such-ref")
	if err == nil {
		t.Fatal("expected error for unknown ref in range")
	}
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Errorf("expected CLIError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "tokui --diff") {
		t.Errorf("expected usage examples with the git error, got %v", err)
	}
}

func TestRunDiffModeScopedToSubdir(t *testing.T) {
	repo := initDiffRepo(t)

	tree := structure.NewTree(nil)
	_, err := runDiffMode(tree, stubProvider{}, filepath.Join(repo, "sub"), "HEAD~1..HEAD")
	if err != nil {
		t.Fatalf("runDiffMode: %v", err)
	}

	root := tree.Root()
	// The tree still holds the full S2 snapshot; only the change set is
	// scoped, so the out-of-scope file keeps zero, non-present Change.
	top := root.GetChild("top.go")
	if top == nil {
		t.Fatal("expected top.go in the S2 snapshot tree")
	}
	if top.Change.Present {
		t.Errorf("top.go is outside the subdir scope; its change must be filtered out, got %+v", top.Change)
	}
	sub := root.GetChild("sub")
	if sub == nil {
		t.Fatal("expected sub directory in tree")
	}
	inner := sub.GetChild("inner.go")
	if inner == nil {
		t.Fatal("expected sub/inner.go in tree")
	}
	if !inner.Change.Present || inner.Change.Added != 1 || inner.TotalStats.Code != 2 {
		t.Errorf("inner.go mismatch: stats %+v change %+v", inner.TotalStats, inner.Change)
	}
}

func TestScopeChangesToSubdir(t *testing.T) {
	changes := []gitx.FileChange{
		{Path: "sub/a.go"},
		{Path: "sub/deep/b.go"},
		{Path: "sub2/c.go"},
		{Path: "top.go"},
		{Path: "other/d.go", OldPath: "sub/d.go", Kind: gitx.Renamed},
	}
	repo := string(filepath.Separator) + "repo"

	t.Run("repo root keeps everything", func(t *testing.T) {
		got := scopeChangesToSubdir(changes, repo, repo)
		if len(got) != len(changes) {
			t.Errorf("expected %d changes, got %d", len(changes), len(got))
		}
	})

	t.Run("subdir filters by prefix", func(t *testing.T) {
		got := scopeChangesToSubdir(changes, repo, filepath.Join(repo, "sub"))
		var paths []string
		for _, c := range got {
			paths = append(paths, c.Path)
		}
		// sub2/c.go shares the "sub" prefix but not the "sub/" directory
		// prefix; top.go is outside; the rename from sub/d.go stays.
		want := []string{"sub/a.go", "sub/deep/b.go", "other/d.go"}
		if strings.Join(paths, ",") != strings.Join(want, ",") {
			t.Errorf("got %v, want %v", paths, want)
		}
	})

	t.Run("trailing slash and dot are normalized", func(t *testing.T) {
		for _, p := range []string{
			filepath.Join(repo, "sub") + string(filepath.Separator),
			filepath.Join(repo, ".", "sub"),
			filepath.Join(repo, "sub", "."),
		} {
			got := scopeChangesToSubdir(changes, repo, p)
			if len(got) != 3 {
				t.Errorf("path %q: expected 3 changes, got %d", p, len(got))
			}
		}
	})

	t.Run("path outside repo keeps everything", func(t *testing.T) {
		got := scopeChangesToSubdir(changes, repo, filepath.Join(repo, "..", "elsewhere"))
		if len(got) != len(changes) {
			t.Errorf("expected %d changes, got %d", len(changes), len(got))
		}
	})
}

func TestRunRefModeNotARepo(t *testing.T) {
	_, _, err := runRefMode(structure.NewTree(nil), stubProvider{}, t.TempDir(), "v1")
	if err == nil {
		t.Fatal("expected not-a-repo error")
	}
	if !errors.Is(err, gitx.ErrNotGitRepo) {
		t.Errorf("expected ErrNotGitRepo, got %v", err)
	}
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Errorf("expected CLIError, got %T", err)
	}
	if !strings.Contains(err.Error(), "remove --ref") {
		t.Errorf("expected full-mode hint, got %v", err)
	}
}

func TestRunCompareModeWorktreeLabel(t *testing.T) {
	repo := initCompareRepo(t)

	tree := structure.NewTree(nil)
	mode, err := runCompareMode(tree, stubProvider{}, repo, "v1..HEAD")
	if err != nil {
		t.Fatalf("runCompareMode: %v", err)
	}
	if mode.Range != "v1..HEAD (worktree)" {
		t.Errorf("expected worktree-annotated range, got %q", mode.Range)
	}
	if mode.S2Ref != "" || mode.S2Label != "worktree" {
		t.Errorf("unexpected S2 fields: %+v", mode)
	}
}
