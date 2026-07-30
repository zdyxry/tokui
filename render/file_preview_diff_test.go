package render

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

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
	full := append([]string{"-C", dir, "-c", "user.email=test@example.com", "-c", "user.name=Test User", "-c", "core.autocrlf=false"}, args...)
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

// worktreeMode diffs the working tree against HEAD, like "tokui diff HEAD".
func worktreeMode(repo string) ModeInfo {
	return ModeInfo{
		Kind:     ModeDiff,
		Range:    "HEAD",
		RepoRoot: repo,
		DiffRev:  "HEAD",
	}
}

func TestFilePreviewDiffShowsColorizedDiff(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	repo := initPreviewRepo(t)
	fp := NewFilePreviewDiff(filepath.Join(repo, "main.go"), 80, 24, worktreeMode(repo), structure.Change{Kind: gitx.Modified})

	if fp.errorMsg != "" {
		t.Fatalf("expected no error, got %q", fp.errorMsg)
	}
	for _, want := range []string{"old version", "new version", "@@", "│"} {
		if !strings.Contains(fp.content, want) {
			t.Errorf("diff preview missing %q:\n%s", want, fp.content)
		}
	}
	if !strings.Contains(fp.content, "\x1b[") {
		t.Error("expected the diff to be colorized (ANSI escapes)")
	}
	if fp.diffRange != "HEAD" {
		t.Errorf("diffRange = %q, want %q", fp.diffRange, "HEAD")
	}
	if got := fp.View(); !strings.Contains(got, "Diff: main.go · HEAD") {
		t.Errorf("expected the diff title with the range label, got:\n%s", got)
	}
}

func TestStyleDiff(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	raw := "diff --git a/x.go b/x.go\nindex 111..222 100644\n--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-old\n+new\n ctx\n"
	styled := styleDiff(raw)

	lines := strings.Split(styled, "\n")
	if len(lines) != 8 {
		t.Fatalf("expected 8 lines, got %d:\n%s", len(lines), styled)
	}
	// Context lines pass through untouched.
	if lines[7] != " ctx" {
		t.Errorf("context line must stay unstyled, got %q", lines[7])
	}
	// Everything else carries an escape sequence.
	for i, line := range lines[:7] {
		if !strings.Contains(line, "\x1b[") {
			t.Errorf("line %d must be styled, got %q", i, line)
		}
	}
}

func TestFilePreviewDiffDeletedFile(t *testing.T) {
	repo := initPreviewRepo(t)
	if err := os.Remove(filepath.Join(repo, "main.go")); err != nil {
		t.Fatal(err)
	}

	fp := NewFilePreviewDiff(filepath.Join(repo, "main.go"), 80, 24, worktreeMode(repo), structure.Change{Kind: gitx.Deleted})

	if fp.errorMsg != "" {
		t.Fatalf("expected no error, got %q", fp.errorMsg)
	}
	if !strings.Contains(fp.content, "deleted file mode") || !strings.Contains(fp.content, "old version") {
		t.Errorf("expected the deletion diff, got:\n%s", fp.content)
	}
}

func TestFilePreviewDiffAddedFile(t *testing.T) {
	repo := initPreviewRepo(t)
	added := filepath.Join(repo, "added.go")
	if err := os.WriteFile(added, []byte("brand new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Untracked files are invisible to "git diff" (and to Numstat); an added
	// file in the change set always comes from the index or a commit range.
	git(t, repo, "add", "added.go")

	fp := NewFilePreviewDiff(added, 80, 24, worktreeMode(repo), structure.Change{Kind: gitx.Added})

	if fp.errorMsg != "" {
		t.Fatalf("expected no error, got %q", fp.errorMsg)
	}
	if !strings.Contains(fp.content, "new file mode") || !strings.Contains(fp.content, "brand new") {
		t.Errorf("expected the addition diff, got:\n%s", fp.content)
	}
}

func TestFilePreviewDiffRenameCoversBothPaths(t *testing.T) {
	repo := initPreviewRepo(t)
	// Restore the committed content so the rename is detected with high
	// similarity, then stage it (worktree renames need the index).
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("old version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "mv", "main.go", "renamed.go")

	fp := NewFilePreviewDiff(filepath.Join(repo, "renamed.go"), 80, 24, worktreeMode(repo),
		structure.Change{Kind: gitx.Renamed, OldPath: "main.go"})

	if fp.errorMsg != "" {
		t.Fatalf("expected no error, got %q", fp.errorMsg)
	}
	if !strings.Contains(fp.content, "rename from main.go") || !strings.Contains(fp.content, "rename to renamed.go") {
		t.Errorf("expected the rename diff, got:\n%s", fp.content)
	}
}

func TestFilePreviewDiffRange(t *testing.T) {
	repo := initPreviewRepo(t)
	// Commit the modification so the range targets two real refs.
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "update")

	mode := ModeInfo{
		Kind:     ModeDiff,
		Range:    "HEAD~1..HEAD",
		RepoRoot: repo,
		DiffRev:  "HEAD~1..HEAD",
	}
	fp := NewFilePreviewDiff(filepath.Join(repo, "main.go"), 80, 24, mode, structure.Change{Kind: gitx.Modified})

	if fp.errorMsg != "" {
		t.Fatalf("expected no error, got %q", fp.errorMsg)
	}
	if !strings.Contains(fp.content, "old version") || !strings.Contains(fp.content, "new version") {
		t.Errorf("expected the range diff, got:\n%s", fp.content)
	}
}

func TestFilePreviewDiffStagedAndUnstaged(t *testing.T) {
	repo := initPreviewRepo(t)
	// Stage the worktree modification: the index now differs from HEAD, the
	// worktree matches the index.
	git(t, repo, "add", "main.go")

	cached := ModeInfo{
		Kind:       ModeDiff,
		Range:      "--staged",
		RepoRoot:   repo,
		DiffCached: true,
	}
	fp := NewFilePreviewDiff(filepath.Join(repo, "main.go"), 80, 24, cached, structure.Change{Kind: gitx.Modified})
	if !strings.Contains(fp.content, "new version") {
		t.Errorf("expected the staged diff, got:\n%s", fp.content)
	}

	unstaged := ModeInfo{
		Kind:     ModeDiff,
		Range:    "worktree (unstaged)",
		RepoRoot: repo,
	}
	fp = NewFilePreviewDiff(filepath.Join(repo, "main.go"), 80, 24, unstaged, structure.Change{Kind: gitx.Modified})
	if !strings.Contains(fp.content, "No textual diff") {
		t.Errorf("expected the no-diff hint for a file without unstaged changes, got:\n%s", fp.content)
	}
}

func TestFilePreviewDiffTooLargeShowsFriendlyHint(t *testing.T) {
	repo := initPreviewRepo(t)
	big := filepath.Join(repo, "big.go")
	// A single >10MB text line produces a >10MB one-line diff without being
	// mistaken for a binary file.
	if err := os.WriteFile(big, bytes.Repeat([]byte("x"), (10<<20)+1), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", "big")

	mode := worktreeMode(repo)
	mode.DiffRev = "HEAD~1..HEAD"
	fp := NewFilePreviewDiff(big, 80, 24, mode, structure.Change{Kind: gitx.Added})

	if fp.errorMsg != "" {
		t.Fatalf("oversized diff must not surface a raw error, got %q", fp.errorMsg)
	}
	if !strings.Contains(fp.content, "Diff too large to preview") {
		t.Errorf("expected a friendly size hint, got %q", fp.content)
	}
}

func TestFilePreviewFullModeShowsContents(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	fp := NewFilePreview(p, 80, 24)
	if fp.diffRange != "" {
		t.Errorf("expected no diff range in full mode, got %q", fp.diffRange)
	}
	if fp.content != "hello" {
		t.Errorf("expected file contents in full mode, got %q", fp.content)
	}
}
