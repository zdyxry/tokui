package gitx_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/zdyxry/tokui/gitx"
)

// requireGit skips the test when no git binary is available.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
}

// initRepo creates a fresh git repository in a temporary directory.
func initRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	git(t, dir, "-c", "init.defaultBranch=main", "init")
	// CI runners may set core.autocrlf=true globally, which makes git
	// archive export CRLF line endings; pin it off for deterministic blobs.
	git(t, dir, "config", "core.autocrlf", "false")
	return dir
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

// writeFile writes content to a file inside the repo, creating parent
// directories as needed.
func writeFile(t *testing.T, repo, name string, content []byte) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, repo, msg string) {
	t.Helper()
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", msg)
}

func findChange(changes []gitx.FileChange, path string) *gitx.FileChange {
	for i := range changes {
		if changes[i].Path == path {
			return &changes[i]
		}
	}
	return nil
}

func TestNumstatModified(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "main.go", []byte("a\nb\nc\n"))
	commitAll(t, repo, "initial")
	writeFile(t, repo, "main.go", []byte("a\nX\nc\nd\n"))
	commitAll(t, repo, "update")

	changes, err := gitx.Numstat(repo, "HEAD~1..HEAD", false)
	if err != nil {
		t.Fatalf("Numstat: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("len(changes) = %d, want 1: %+v", len(changes), changes)
	}
	c := changes[0]
	if c.Path != "main.go" {
		t.Errorf("Path = %q, want %q", c.Path, "main.go")
	}
	if c.Kind != gitx.Modified {
		t.Errorf("Kind = %v, want Modified", c.Kind)
	}
	if c.Added != 2 || c.Deleted != 1 {
		t.Errorf("Added/Deleted = %d/%d, want 2/1", c.Added, c.Deleted)
	}
	if c.OldPath != "" {
		t.Errorf("OldPath = %q, want empty", c.OldPath)
	}
}

func TestNumstatAddedAndDeleted(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "keep.txt", []byte("keep\n"))
	writeFile(t, repo, "gone.txt", []byte("x\ny\nz\n"))
	commitAll(t, repo, "initial")
	writeFile(t, repo, "new.txt", []byte("1\n2\n"))
	if err := os.Remove(filepath.Join(repo, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	commitAll(t, repo, "add new, remove gone")

	changes, err := gitx.Numstat(repo, "HEAD~1..HEAD", false)
	if err != nil {
		t.Fatalf("Numstat: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("len(changes) = %d, want 2: %+v", len(changes), changes)
	}

	added := findChange(changes, "new.txt")
	if added == nil {
		t.Fatalf("new.txt not found in %+v", changes)
	}
	if added.Kind != gitx.Added {
		t.Errorf("new.txt Kind = %v, want Added", added.Kind)
	}
	if added.Added != 2 || added.Deleted != 0 {
		t.Errorf("new.txt Added/Deleted = %d/%d, want 2/0", added.Added, added.Deleted)
	}

	deleted := findChange(changes, "gone.txt")
	if deleted == nil {
		t.Fatalf("gone.txt not found in %+v", changes)
	}
	if deleted.Kind != gitx.Deleted {
		t.Errorf("gone.txt Kind = %v, want Deleted", deleted.Kind)
	}
	if deleted.Added != 0 || deleted.Deleted != 3 {
		t.Errorf("gone.txt Added/Deleted = %d/%d, want 0/3", deleted.Added, deleted.Deleted)
	}
}

func TestNumstatRename(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "old.txt", []byte("one\ntwo\nthree\nfour\nfive\n"))
	writeFile(t, repo, "sub/before.go", []byte("package sub\n\n// a\n// b\n// c\n// d\n"))
	commitAll(t, repo, "initial")
	git(t, repo, "mv", "old.txt", "new.txt")
	git(t, repo, "mv", "sub/before.go", "sub/after.go")
	commitAll(t, repo, "renames")

	changes, err := gitx.Numstat(repo, "HEAD~1..HEAD", false)
	if err != nil {
		t.Fatalf("Numstat: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("len(changes) = %d, want 2: %+v", len(changes), changes)
	}

	// Plain "old => new" form.
	plain := findChange(changes, "new.txt")
	if plain == nil {
		t.Fatalf("new.txt not found in %+v", changes)
	}
	if plain.Kind != gitx.Renamed {
		t.Errorf("new.txt Kind = %v, want Renamed", plain.Kind)
	}
	if plain.OldPath != "old.txt" {
		t.Errorf("new.txt OldPath = %q, want %q", plain.OldPath, "old.txt")
	}

	// Brace form "sub/{before.go => after.go}".
	braced := findChange(changes, "sub/after.go")
	if braced == nil {
		t.Fatalf("sub/after.go not found in %+v", changes)
	}
	if braced.Kind != gitx.Renamed {
		t.Errorf("sub/after.go Kind = %v, want Renamed", braced.Kind)
	}
	if braced.OldPath != "sub/before.go" {
		t.Errorf("sub/after.go OldPath = %q, want %q", braced.OldPath, "sub/before.go")
	}
}

func TestNumstatBinary(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "keep.txt", []byte("keep\n"))
	commitAll(t, repo, "initial")
	writeFile(t, repo, "blob.bin", []byte{0x00, 0x01, 0x02, 0x00, 0xff, 0x00})
	commitAll(t, repo, "add binary")

	changes, err := gitx.Numstat(repo, "HEAD~1..HEAD", false)
	if err != nil {
		t.Fatalf("Numstat: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("len(changes) = %d, want 1: %+v", len(changes), changes)
	}
	c := changes[0]
	if c.Path != "blob.bin" {
		t.Errorf("Path = %q, want %q", c.Path, "blob.bin")
	}
	if c.Kind != gitx.Added {
		t.Errorf("Kind = %v, want Added", c.Kind)
	}
	if c.Added != 0 || c.Deleted != 0 {
		t.Errorf("binary Added/Deleted = %d/%d, want 0/0", c.Added, c.Deleted)
	}
}

func TestNumstatEmptyDiff(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.txt", []byte("a\n"))
	commitAll(t, repo, "initial")

	changes, err := gitx.Numstat(repo, "HEAD..HEAD", false)
	if err != nil {
		t.Fatalf("Numstat: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("len(changes) = %d, want 0: %+v", len(changes), changes)
	}
}

func TestRepoRoot(t *testing.T) {
	repo := initRepo(t)
	want, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}

	root, err := gitx.RepoRoot(repo)
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	if root != want {
		t.Errorf("RepoRoot = %q, want %q", root, want)
	}

	sub := filepath.Join(repo, "sub", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err = gitx.RepoRoot(sub)
	if err != nil {
		t.Fatalf("RepoRoot(subdir): %v", err)
	}
	if root != want {
		t.Errorf("RepoRoot(subdir) = %q, want %q", root, want)
	}
}

func TestRepoRootNotARepo(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	_, err := gitx.RepoRoot(dir)
	if err == nil {
		t.Fatal("RepoRoot on non-git directory: expected error, got nil")
	}
	if !errors.Is(err, gitx.ErrNotGitRepo) {
		t.Errorf("errors.Is(err, ErrNotGitRepo) = false for %q", err)
	}
	if !strings.Contains(err.Error(), "not a git repository: "+dir) {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "not a git repository: "+dir)
	}
}

func TestArchive(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "main.go", []byte("package main\n"))
	writeFile(t, repo, "docs/readme.md", []byte("# readme\n"))
	commitAll(t, repo, "initial")

	// Archive resolves the ref in the current working directory.
	t.Chdir(repo)

	dir, cleanup, err := gitx.Archive("HEAD")
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatalf("read archived file: %v", err)
	}
	if string(content) != "package main\n" {
		t.Errorf("archived main.go = %q, want %q", content, "package main\n")
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "readme.md")); err != nil {
		t.Errorf("archived docs/readme.md: %v", err)
	}

	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("after cleanup, stat(%q) err = %v, want not-exist", dir, err)
	}
}

func TestShowFile(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "file.txt", []byte("v1\n"))
	commitAll(t, repo, "v1")
	writeFile(t, repo, "file.txt", []byte("v2\n"))
	commitAll(t, repo, "v2")

	old, err := gitx.ShowFile(repo, "HEAD~1", "file.txt")
	if err != nil {
		t.Fatalf("ShowFile HEAD~1: %v", err)
	}
	if string(old) != "v1\n" {
		t.Errorf("ShowFile HEAD~1 = %q, want %q", old, "v1\n")
	}

	current, err := gitx.ShowFile(repo, "HEAD", "file.txt")
	if err != nil {
		t.Fatalf("ShowFile HEAD: %v", err)
	}
	if string(current) != "v2\n" {
		t.Errorf("ShowFile HEAD = %q, want %q", current, "v2\n")
	}
}

func TestNumstatSubmoduleSkipped(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "main.go", []byte("package main\n"))
	commitAll(t, repo, "initial")

	// Fake a submodule by staging a gitlink entry directly.
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	sha := strings.TrimSpace(string(out))
	git(t, repo, "update-index", "--add", "--cacheinfo", "160000,"+sha+",sub")
	writeFile(t, repo, "main.go", []byte("package main\n\n// changed\n"))
	git(t, repo, "add", "main.go")
	git(t, repo, "commit", "-m", "add submodule and change main.go")

	changes, err := gitx.Numstat(repo, "HEAD~1..HEAD", false)
	if err != nil {
		t.Fatalf("Numstat: %v", err)
	}
	if findChange(changes, "sub") != nil {
		t.Errorf("submodule entry must be skipped, got %+v", changes)
	}
	main := findChange(changes, "main.go")
	if main == nil {
		t.Fatalf("main.go not found in %+v", changes)
	}
	if main.Kind != gitx.Modified {
		t.Errorf("main.go Kind = %v, want Modified", main.Kind)
	}
}

func TestNumstatDeletedBinary(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "blob.bin", []byte{0x00, 0x01, 0x02, 0x00, 0xff, 0x00})
	commitAll(t, repo, "add binary")
	if err := os.Remove(filepath.Join(repo, "blob.bin")); err != nil {
		t.Fatal(err)
	}
	commitAll(t, repo, "remove binary")

	changes, err := gitx.Numstat(repo, "HEAD~1..HEAD", false)
	if err != nil {
		t.Fatalf("Numstat: %v", err)
	}
	// A deleted binary shows "- -" in numstat and no longer exists in the
	// worktree: it must still be reported, not mistaken for a submodule.
	c := findChange(changes, "blob.bin")
	if c == nil {
		t.Fatalf("deleted binary blob.bin not found in %+v", changes)
	}
	if c.Kind != gitx.Deleted {
		t.Errorf("Kind = %v, want Deleted", c.Kind)
	}
	if c.Added != 0 || c.Deleted != 0 {
		t.Errorf("binary Added/Deleted = %d/%d, want 0/0", c.Added, c.Deleted)
	}
}

func TestNumstatSpecialFilenames(t *testing.T) {
	repo := initRepo(t)
	names := []string{"with space.txt", "日本語.txt"}
	if runtime.GOOS != "windows" {
		// ">" is not allowed in Windows file names.
		names = append([]string{"a => b.txt"}, names...)
	}
	for _, name := range names {
		writeFile(t, repo, name, []byte("one\ntwo\n"))
	}
	commitAll(t, repo, "initial")
	for _, name := range names {
		writeFile(t, repo, name, []byte("one\ntwo\nthree\n"))
	}
	commitAll(t, repo, "update")

	changes, err := gitx.Numstat(repo, "HEAD~1..HEAD", false)
	if err != nil {
		t.Fatalf("Numstat: %v", err)
	}
	if len(changes) != len(names) {
		t.Fatalf("len(changes) = %d, want %d: %+v", len(changes), len(names), changes)
	}
	for _, name := range names {
		c := findChange(changes, name)
		if c == nil {
			t.Errorf("%q not found in %+v", name, changes)
			continue
		}
		if c.Kind != gitx.Modified {
			t.Errorf("%q Kind = %v, want Modified (mis-parsed as rename?)", name, c.Kind)
		}
		if c.Added != 1 || c.Deleted != 0 {
			t.Errorf("%q Added/Deleted = %d/%d, want 1/0", name, c.Added, c.Deleted)
		}
	}
}

func TestLeadingDashRejected(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "file.txt", []byte("v1\n"))
	commitAll(t, repo, "v1")

	if _, err := gitx.Numstat(repo, "-HEAD", false); err == nil {
		t.Error("Numstat with leading-dash range: expected error, got nil")
	}
	if _, _, err := gitx.Archive("-HEAD"); err == nil {
		t.Error("Archive with leading-dash ref: expected error, got nil")
	}
	if _, err := gitx.ShowFile(repo, "-HEAD", "file.txt"); err == nil {
		t.Error("ShowFile with leading-dash ref: expected error, got nil")
	}
	if _, err := gitx.ShowFile(repo, "HEAD", "-file.txt"); err == nil {
		t.Error("ShowFile with leading-dash path: expected error, got nil")
	}
}

func TestShowFileMissing(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "file.txt", []byte("v1\n"))
	commitAll(t, repo, "v1")

	if _, err := gitx.ShowFile(repo, "HEAD", "no-such-file.txt"); err == nil {
		t.Error("ShowFile with missing path: expected error, got nil")
	}
	if _, err := gitx.ShowFile(repo, "refs/heads/no-such-ref", "file.txt"); err == nil {
		t.Error("ShowFile with bad ref: expected error, got nil")
	}
}

func TestShowFileTooLarge(t *testing.T) {
	repo := initRepo(t)
	big := bytes.Repeat([]byte("x"), (10<<20)+1)
	writeFile(t, repo, "big.bin", big)
	commitAll(t, repo, "add big file")

	_, err := gitx.ShowFile(repo, "HEAD", "big.bin")
	if err == nil {
		t.Fatal("ShowFile on >10MB file: expected error, got nil")
	}
	if !errors.Is(err, gitx.ErrFileTooLarge) {
		t.Errorf("errors.Is(err, ErrFileTooLarge) = false for %q", err)
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error = %q, want it to contain %q", err, "too large")
	}
}

func TestArchiveSymlink(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "main.go", []byte("package main\n"))
	if err := os.Symlink("main.go", filepath.Join(repo, "link.go")); err != nil {
		t.Fatal(err)
	}
	commitAll(t, repo, "initial")

	t.Chdir(repo)
	dir, cleanup, err := gitx.Archive("HEAD")
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	defer cleanup()

	target, err := os.Readlink(filepath.Join(dir, "link.go"))
	if err != nil {
		t.Fatalf("read archived symlink: %v", err)
	}
	if target != "main.go" {
		t.Errorf("symlink target = %q, want %q", target, "main.go")
	}
}

func TestArchiveSymlinkEscapeRejected(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "main.go", []byte("package main\n"))
	if err := os.Symlink("../../outside", filepath.Join(repo, "evil")); err != nil {
		t.Fatal(err)
	}
	commitAll(t, repo, "initial")

	t.Chdir(repo)
	_, _, err := gitx.Archive("HEAD")
	if err == nil {
		t.Fatal("Archive with escaping symlink: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "escapes") {
		t.Errorf("error = %q, want it to contain %q", err, "escapes")
	}
}

func TestNumstatUnstagedOnly(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "staged.txt", []byte("a\n"))
	writeFile(t, repo, "unstaged.txt", []byte("a\n"))
	commitAll(t, repo, "initial")

	// A staged change and a separate unstaged change.
	writeFile(t, repo, "staged.txt", []byte("a\nb\n"))
	git(t, repo, "add", "staged.txt")
	writeFile(t, repo, "unstaged.txt", []byte("a\nb\nc\n"))

	// Bare diff (no rev, not cached): worktree vs index — unstaged only.
	changes, err := gitx.Numstat(repo, "", false)
	if err != nil {
		t.Fatalf("Numstat: %v", err)
	}
	if len(changes) != 1 || changes[0].Path != "unstaged.txt" {
		t.Fatalf("expected only unstaged.txt, got %+v", changes)
	}
	if changes[0].Kind != gitx.Modified || changes[0].Added != 2 || changes[0].Deleted != 0 {
		t.Errorf("unstaged.txt mismatch: %+v", changes[0])
	}
}

func TestNumstatStagedOnly(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "staged.txt", []byte("a\n"))
	writeFile(t, repo, "unstaged.txt", []byte("a\n"))
	commitAll(t, repo, "initial")

	writeFile(t, repo, "staged.txt", []byte("a\nb\n"))
	git(t, repo, "add", "staged.txt")
	writeFile(t, repo, "unstaged.txt", []byte("a\nb\nc\n"))

	// "git diff --cached": index vs HEAD — staged only.
	changes, err := gitx.Numstat(repo, "", true)
	if err != nil {
		t.Fatalf("Numstat: %v", err)
	}
	if len(changes) != 1 || changes[0].Path != "staged.txt" {
		t.Fatalf("expected only staged.txt, got %+v", changes)
	}
	if changes[0].Added != 1 || changes[0].Deleted != 0 {
		t.Errorf("staged.txt mismatch: %+v", changes[0])
	}

	// "git diff --cached HEAD~0" (explicit rev) sees the same staged change.
	changes, err = gitx.Numstat(repo, "HEAD", true)
	if err != nil {
		t.Fatalf("Numstat with rev: %v", err)
	}
	if len(changes) != 1 || changes[0].Path != "staged.txt" {
		t.Fatalf("expected only staged.txt with explicit rev, got %+v", changes)
	}
}

func TestNumstatRevMatchingDirectoryFails(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "sub/file.txt", []byte("a\n"))
	commitAll(t, repo, "initial")

	// "sub" names a directory but no revision; the trailing "--" must force
	// revision interpretation and fail instead of degrading to a pathspec.
	if _, err := gitx.Numstat(repo, "sub", false); err == nil {
		t.Fatal("expected error for rev that only matches a directory name")
	}
}

func TestShowFileIndex(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "file.txt", []byte("committed\n"))
	commitAll(t, repo, "initial")

	// Stage a new version, then diverge the worktree from the index.
	writeFile(t, repo, "file.txt", []byte("staged\n"))
	git(t, repo, "add", "file.txt")
	writeFile(t, repo, "file.txt", []byte("worktree\n"))

	data, err := gitx.ShowFile(repo, "", "file.txt")
	if err != nil {
		t.Fatalf("ShowFile index: %v", err)
	}
	if string(data) != "staged\n" {
		t.Errorf("ShowFile index = %q, want the staged blob %q", data, "staged\n")
	}
}
