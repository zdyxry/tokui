package render

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zdyxry/tokui/gitx"
	"github.com/zdyxry/tokui/structure"
)

// requireGit skips the test when no git binary is available.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
}

// git runs a git command inside dir, failing the test on error.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.email=test@example.com", "-c", "user.name=Test User"}, args...)
	cmd := exec.Command("git", full...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// initPreviewRepo creates a git repo with one committed file that is then
// modified in the working tree.
func initPreviewRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	git(t, dir, "-c", "init.defaultBranch=main", "init")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("old version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("new version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func worktreeMode(repo string) ModeInfo {
	return ModeInfo{
		Kind:     ModeDiff,
		Range:    "HEAD",
		RepoRoot: repo,
		S1Ref:    "HEAD",
		S1Label:  "HEAD",
		S2Label:  "worktree",
	}
}

func TestFilePreviewDiffShowsS2ThenTogglesToS1(t *testing.T) {
	repo := initPreviewRepo(t)
	fp := NewFilePreviewDiff(filepath.Join(repo, "main.go"), 80, 24, worktreeMode(repo), structure.Change{Kind: gitx.Modified})

	// S2 (working tree) is shown first.
	if fp.showingS1 {
		t.Error("expected preview to start on the S2 version")
	}
	if fp.content != "new version\n" {
		t.Errorf("expected S2 content, got %q", fp.content)
	}
	if got := fp.versionLabel(); got != "S2: worktree" {
		t.Errorf("expected S2 title marker, got %q", got)
	}
	if !fp.CanToggleVersion() {
		t.Fatal("expected version toggle to be available")
	}

	// "v" switches to the S1 version loaded via git show.
	fp.ToggleVersion()
	if !fp.showingS1 {
		t.Error("expected S1 after toggle")
	}
	if fp.content != "old version\n" {
		t.Errorf("expected S1 content, got %q", fp.content)
	}
	if got := fp.versionLabel(); got != "S1: HEAD" {
		t.Errorf("expected S1 title marker, got %q", got)
	}

	// And back.
	fp.ToggleVersion()
	if fp.content != "new version\n" {
		t.Errorf("expected S2 content after second toggle, got %q", fp.content)
	}
}

func TestFilePreviewDiffDeletedFileFallsBackToS1(t *testing.T) {
	repo := initPreviewRepo(t)
	if err := os.Remove(filepath.Join(repo, "main.go")); err != nil {
		t.Fatal(err)
	}

	fp := NewFilePreviewDiff(filepath.Join(repo, "main.go"), 80, 24, worktreeMode(repo), structure.Change{Kind: gitx.Deleted})

	if !fp.s2Missing {
		t.Error("expected s2Missing for a deleted file")
	}
	if !fp.showingS1 {
		t.Error("expected deleted file to preview the S1 version directly")
	}
	if fp.content != "old version\n" {
		t.Errorf("expected S1 content for deleted file, got %q", fp.content)
	}
	if fp.CanToggleVersion() {
		t.Error("deleted file has no S2 version; toggle must be unavailable")
	}
	if got := fp.versionLabel(); got != "S1: HEAD" {
		t.Errorf("expected S1 title marker, got %q", got)
	}
}

func TestFilePreviewDiffHistoricalS2Ref(t *testing.T) {
	repo := initPreviewRepo(t)
	// Commit the modification so S2 can be a real ref.
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "update")

	mode := ModeInfo{
		Kind:     ModeDiff,
		Range:    "HEAD~1..HEAD",
		RepoRoot: repo,
		S1Ref:    "HEAD~1",
		S1Label:  "HEAD~1",
		S2Ref:    "HEAD",
		S2Label:  "HEAD",
	}
	fp := NewFilePreviewDiff(filepath.Join(repo, "main.go"), 80, 24, mode, structure.Change{Kind: gitx.Modified})

	if fp.content != "new version\n" {
		t.Errorf("expected S2 content from ref, got %q", fp.content)
	}
	fp.ToggleVersion()
	if fp.content != "old version\n" {
		t.Errorf("expected S1 content from ref, got %q", fp.content)
	}
}

func TestFilePreviewFullModeHasNoVersionLabel(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	fp := NewFilePreview(p, 80, 24)
	if got := fp.versionLabel(); got != "" {
		t.Errorf("expected no version label in full mode, got %q", got)
	}
	if fp.CanToggleVersion() {
		t.Error("version toggle must be unavailable in full mode")
	}
}

func TestFilePreviewDiffRenameUsesOldPathForS1(t *testing.T) {
	repo := initPreviewRepo(t)
	// Rename the modified file in the working tree: S1 knows it as main.go.
	if err := os.Rename(filepath.Join(repo, "main.go"), filepath.Join(repo, "renamed.go")); err != nil {
		t.Fatal(err)
	}

	fp := NewFilePreviewDiff(filepath.Join(repo, "renamed.go"), 80, 24, worktreeMode(repo),
		structure.Change{Kind: gitx.Renamed, OldPath: "main.go"})

	if fp.content != "new version\n" {
		t.Errorf("expected S2 content under the new path, got %q", fp.content)
	}
	fp.ToggleVersion()
	if fp.errorMsg != "" {
		t.Fatalf("expected no error for renamed S1 lookup, got %q", fp.errorMsg)
	}
	if fp.content != "old version\n" {
		t.Errorf("expected S1 content via the old path, got %q", fp.content)
	}
}

func TestFilePreviewDiffAddedFileShowsHintForS1(t *testing.T) {
	repo := initPreviewRepo(t)
	added := filepath.Join(repo, "added.go")
	if err := os.WriteFile(added, []byte("brand new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fp := NewFilePreviewDiff(added, 80, 24, worktreeMode(repo), structure.Change{Kind: gitx.Added})

	if fp.content != "brand new\n" {
		t.Errorf("expected S2 content for added file, got %q", fp.content)
	}
	fp.ToggleVersion()
	if fp.errorMsg != "" {
		t.Fatalf("added file must not surface a git error, got %q", fp.errorMsg)
	}
	if !strings.Contains(fp.content, "Not present in S1") || !strings.Contains(fp.content, "HEAD") {
		t.Errorf("expected a friendly added-file hint, got %q", fp.content)
	}
}

func TestFilePreviewDiffS2ErrorDoesNotFallbackUnlessDeleted(t *testing.T) {
	repo := initPreviewRepo(t)
	mode := worktreeMode(repo)
	mode.S2Ref = "HEAD"
	mode.S2Label = "HEAD"

	// untracked.go is not in HEAD; with Kind != Deleted the S2 failure must
	// surface as an error instead of falling back to S1.
	fp := NewFilePreviewDiff(filepath.Join(repo, "untracked.go"), 80, 24, mode,
		structure.Change{Kind: gitx.Modified})

	if fp.s2Missing {
		t.Error("non-deleted S2 read failure must not be treated as deleted")
	}
	if fp.showingS1 {
		t.Error("must stay on the S2 version when the read fails")
	}
	if fp.errorMsg == "" {
		t.Error("expected the S2 read error to be displayed")
	}
}

func TestFilePreviewDiffTooLargeShowsFriendlyHint(t *testing.T) {
	repo := initPreviewRepo(t)
	big := filepath.Join(repo, "big.go")
	if err := os.WriteFile(big, make([]byte, 10<<20+1), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "big")

	mode := worktreeMode(repo)
	mode.S2Ref = "HEAD"
	mode.S2Label = "HEAD"
	fp := NewFilePreviewDiff(big, 80, 24, mode, structure.Change{Kind: gitx.Modified})

	if fp.errorMsg != "" {
		t.Fatalf("oversized file must not surface a raw error, got %q", fp.errorMsg)
	}
	if !strings.Contains(fp.content, "File too large to preview") {
		t.Errorf("expected a friendly size hint, got %q", fp.content)
	}
}

func TestFilePreviewDiffIndexS1(t *testing.T) {
	repo := initPreviewRepo(t)
	// Stage the worktree modification, then diverge the worktree again so the
	// index holds a distinct "staged" version.
	git(t, repo, "add", "main.go")
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("diverged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mode := ModeInfo{
		Kind:     ModeDiff,
		Range:    "worktree (unstaged)",
		RepoRoot: repo,
		S1Label:  "index",
		S2Label:  "worktree",
	}
	fp := NewFilePreviewDiff(filepath.Join(repo, "main.go"), 80, 24, mode, structure.Change{Kind: gitx.Modified})

	if !fp.CanToggleVersion() {
		t.Fatal("expected version toggle to be available with the index as S1")
	}
	fp.ToggleVersion()
	if fp.errorMsg != "" {
		t.Fatalf("expected no error for index S1 lookup, got %q", fp.errorMsg)
	}
	if fp.content != "new version\n" {
		t.Errorf("expected the staged blob as S1 content, got %q", fp.content)
	}
	if got := fp.versionLabel(); got != "S1: index" {
		t.Errorf("expected S1 title marker, got %q", got)
	}
}
