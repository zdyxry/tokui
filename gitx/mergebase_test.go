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
