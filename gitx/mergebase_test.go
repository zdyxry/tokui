package gitx_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/zdyxry/tokui/gitx"
)

// gitRevParse returns the commit sha a ref resolves to.
func gitRevParse(t *testing.T, repo, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

func TestMergeBase(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "main.go", []byte("a\n"))
	commitAll(t, repo, "base")
	git(t, repo, "tag", "base")

	git(t, repo, "checkout", "-b", "feature")
	writeFile(t, repo, "feature.go", []byte("f\n"))
	commitAll(t, repo, "feature work")

	git(t, repo, "checkout", "main")
	writeFile(t, repo, "main2.go", []byte("m\n"))
	commitAll(t, repo, "main work")

	base, err := gitx.MergeBase(repo, "main", "feature")
	if err != nil {
		t.Fatalf("MergeBase: %v", err)
	}
	want := gitRevParse(t, repo, "base")
	if base != want {
		t.Errorf("MergeBase = %q, want %q (the base tag commit)", base, want)
	}
}

func TestMergeBaseNoCommonAncestor(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.go", []byte("a\n"))
	commitAll(t, repo, "main work")

	git(t, repo, "checkout", "--orphan", "unrelated")
	git(t, repo, "rm", "-rf", ".")
	writeFile(t, repo, "b.go", []byte("b\n"))
	commitAll(t, repo, "orphan work")

	if _, err := gitx.MergeBase(repo, "main", "unrelated"); err == nil {
		t.Fatal("expected error for refs without a common ancestor")
	}
}

func TestMergeBaseRejectsOptions(t *testing.T) {
	repo := initRepo(t)
	if _, err := gitx.MergeBase(repo, "-HEAD", "HEAD"); err == nil {
		t.Fatal("expected error for ref starting with '-'")
	}
	if _, err := gitx.MergeBase(repo, "HEAD", "--all"); err == nil {
		t.Fatal("expected error for ref starting with '-'")
	}
}

func TestShowRange(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.go", []byte("a\n"))
	commitAll(t, repo, "first")
	writeFile(t, repo, "b.go", []byte("b\n"))
	commitAll(t, repo, "second")
	writeFile(t, repo, "c.go", []byte("c\n"))
	commitAll(t, repo, "third")

	rangeSpec, s1, s2, err := gitx.ShowRange(repo, "HEAD")
	if err != nil {
		t.Fatalf("ShowRange: %v", err)
	}
	if rangeSpec != "HEAD^..HEAD" || s1 != "HEAD^" || s2 != "HEAD" {
		t.Errorf("ShowRange(HEAD) = (%q, %q, %q), want (HEAD^..HEAD, HEAD^, HEAD)",
			rangeSpec, s1, s2)
	}

	// An explicit sha resolves the same way.
	sha := gitRevParse(t, repo, "HEAD~1")
	rangeSpec, s1, s2, err = gitx.ShowRange(repo, sha)
	if err != nil {
		t.Fatalf("ShowRange(sha): %v", err)
	}
	if rangeSpec != sha+"^.."+sha || s1 != sha+"^" || s2 != sha {
		t.Errorf("ShowRange(%s) = (%q, %q, %q)", sha, rangeSpec, s1, s2)
	}
}

func TestShowRangeRootCommit(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.go", []byte("a\n"))
	commitAll(t, repo, "root")

	const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	rangeSpec, s1, s2, err := gitx.ShowRange(repo, "HEAD")
	if err != nil {
		t.Fatalf("ShowRange: %v", err)
	}
	if rangeSpec != emptyTree+"..HEAD" || s1 != emptyTree || s2 != "HEAD" {
		t.Errorf("ShowRange(root) = (%q, %q, %q), want empty-tree base", rangeSpec, s1, s2)
	}

	// The range is usable: the root commit's churn is "everything added".
	changes, err := gitx.Numstat(repo, rangeSpec, false)
	if err != nil {
		t.Fatalf("Numstat(root range): %v", err)
	}
	if len(changes) != 1 || changes[0].Kind != gitx.Added {
		t.Errorf("expected a.go as Added, got %+v", changes)
	}
}

func TestShowRangeUnknownCommit(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.go", []byte("a\n"))
	commitAll(t, repo, "first")

	if _, _, _, err := gitx.ShowRange(repo, "no-such-commit"); err == nil {
		t.Fatal("expected error for unknown commit")
	}
	if _, _, _, err := gitx.ShowRange(repo, "-HEAD"); err == nil {
		t.Fatal("expected error for commit starting with '-'")
	}
}
